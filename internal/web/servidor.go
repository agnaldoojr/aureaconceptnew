// Package web é o servidor HTTP do sistema: sessão persistida no SQLite, CSRF, limite de login, controle de permissões por módulo/ação
// (internal/permissoes) e as telas. Substitui, no sistema novo, o Spring Security + Thymeleaf + PermissaoInterceptor do antigo.
package web

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	ativos "github.com/agnaldoojr/aureaconceptnew/web"

	"github.com/agnaldoojr/aureaconceptnew/internal/auth"
	"github.com/agnaldoojr/aureaconceptnew/internal/config"
	"github.com/agnaldoojr/aureaconceptnew/internal/db"
	"github.com/agnaldoojr/aureaconceptnew/internal/permissoes"
)

// Mensagens iguais às do AuthController do antigo.
const (
	msgLoginInvalido = "Login ou senha inválidos. Verifique seus dados."
	msgBloqueado     = "Muitas tentativas de login. Aguarde alguns minutos e tente novamente."
	msgSaiu          = "Você saiu do sistema com sucesso."
	msgExpirou       = "Sua sessão expirou. Faça login novamente."
)

// limite do corpo de um formulário (os formulários do sistema são pequenos; nada de upload)
const maxCorpo = 2 << 20

// Servidor reúne o que as telas precisam.
type Servidor struct {
	cfg     *config.Config
	banco   *db.Banco
	sessoes *Sessoes
	limite  *LimiteLogin
	est     *Estaticos
	paginas map[string]*template.Template
	log     *slog.Logger
	agora   func() time.Time

	mux       *http.ServeMux
	hashFalso string // BCrypt de uma senha qualquer, para gastar o mesmo tempo quando o login não existe
}

// Opcoes permite trocar o relógio nos testes.
type Opcoes struct {
	Agora func() time.Time
}

// Usuario é o usuário logado numa requisição.
type Usuario struct {
	ID    int64
	Login string
	Nome  string
	Auth  permissoes.Conjunto
}

type chaveCtx struct{}

type contexto struct {
	Sessao  *Sessao
	Usuario *Usuario
}

func ctxDe(r *http.Request) *contexto {
	c, _ := r.Context().Value(chaveCtx{}).(*contexto)
	return c
}

// Novo monta o servidor. Falha se algum template referencia um estático inexistente ou não compila.
func Novo(cfg *config.Config, banco *db.Banco, log *slog.Logger, op Opcoes) (*Servidor, error) {
	if log == nil {
		log = slog.Default()
	}
	agora := op.Agora
	if agora == nil {
		agora = time.Now
	}
	sub, err := fs.Sub(ativos.Estaticos, "static")
	if err != nil {
		return nil, err
	}
	est, err := NovosEstaticos(sub)
	if err != nil {
		return nil, fmt.Errorf("estáticos: %w", err)
	}
	falso, err := auth.GerarHash("senha-que-nunca-sera-usada")
	if err != nil {
		return nil, err
	}
	s := &Servidor{
		cfg: cfg, banco: banco, est: est, log: log, agora: agora, hashFalso: falso,
		sessoes: NovaSessoes(banco, agora),
		limite:  NovoLimiteLogin(cfg.LoginTentativas, cfg.LoginTentativasIP, cfg.LoginBloqueio, agora),
	}
	if err := s.carregarTemplates(); err != nil {
		return nil, err
	}
	s.mux = http.NewServeMux()
	s.rotas()
	return s, nil
}

// Sessoes expõe o armazenamento de sessões (o main usa para a limpeza periódica).
func (s *Servidor) Sessoes() *Sessoes { return s.sessoes }

func (s *Servidor) rotas() {
	s.mux.HandleFunc("GET /{$}", s.dashboard)
	s.mux.HandleFunc("GET /dashboard", s.dashboard)
	// as demais telas entram nas próximas fatias; até lá qualquer outro caminho (já autenticado e autorizado) é 404
	s.mux.HandleFunc("/", s.naoEncontrado)
}

func (s *Servidor) naoEncontrado(w http.ResponseWriter, r *http.Request) {
	http.NotFound(w, r)
}

// Handler é a entrada do servidor: cabeçalhos de segurança, rotas públicas e, para o resto, a cadeia
// sessão → login obrigatório → CSRF → permissão → tela.
func (s *Servidor) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				s.log.Error("falha inesperada", "caminho", r.URL.Path, "erro", rec)
				http.Error(w, "Erro interno. Tente novamente.", http.StatusInternalServerError)
			}
		}()
		r.Body = http.MaxBytesReader(w, r.Body, maxCorpo)
		cab := w.Header()
		cab.Set("X-Content-Type-Options", "nosniff")
		cab.Set("X-Frame-Options", "DENY")
		cab.Set("Referrer-Policy", "same-origin")
		if s.cfg.CookieSeguro {
			cab.Set("Strict-Transport-Security", "max-age=31536000")
		}

		caminho := r.URL.Path
		switch {
		case EEstatico(caminho):
			s.est.ServeHTTP(w, r)
			return
		case caminho == "/healthz":
			s.saude(w, r)
			return
		}
		// tudo o que muda por usuário não pode ficar em cache do navegador (o Spring também manda isto)
		cab.Set("Cache-Control", "no-cache, no-store, max-age=0, must-revalidate")
		cab.Set("Pragma", "no-cache")

		switch caminho {
		case "/login":
			s.publica(w, r, s.loginTela, s.loginEnvio)
			return
		case "/logout":
			s.logout(w, r)
			return
		}
		s.protegida(w, r)
	})
}

func (s *Servidor) saude(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.banco.R.PingContext(ctx); err != nil {
		http.Error(w, "banco indisponível", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("ok"))
}

// ───────────────────────────── sessão e cookie ─────────────────────────────

func (s *Servidor) cookieSessao(r *http.Request) string {
	if c, err := r.Cookie(NomeCookieSessao); err == nil {
		return c.Value
	}
	return ""
}

func (s *Servidor) gravarCookie(w http.ResponseWriter, sess *Sessao) {
	c := &http.Cookie{Name: NomeCookieSessao, Value: sess.ID, Path: "/", HttpOnly: true, Secure: s.cfg.CookieSeguro, SameSite: http.SameSiteLaxMode}
	if sess.ModoApp && sess.Autenticada() { // app instalado: o cookie sobrevive ao fechamento do app (SessaoApp do antigo)
		c.MaxAge = int(sess.Duracao.Seconds())
	}
	http.SetCookie(w, c)
}

func (s *Servidor) apagarCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: NomeCookieSessao, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: s.cfg.CookieSeguro, SameSite: http.SameSiteLaxMode})
}

// ipCliente devolve o IP usado no limite de login. Atrás do Caddy (PROXY_CONFIAVEL), vale o último valor de X-Forwarded-For.
func (s *Servidor) ipCliente(r *http.Request) string {
	if s.cfg.ProxyConfiavel {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			partes := strings.Split(xff, ",")
			if ip := strings.TrimSpace(partes[len(partes)-1]); ip != "" {
				return ip
			}
		}
	}
	h, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return h
}

// csrfValido confere o token do formulário (_csrf) ou do cabeçalho (X-CSRF-TOKEN), como o Spring.
func csrfValido(r *http.Request, esperado string) bool {
	if esperado == "" {
		return false
	}
	t := r.Header.Get("X-CSRF-TOKEN")
	if t == "" {
		t = r.PostFormValue("_csrf")
	}
	return t != "" && subtle.ConstantTimeCompare([]byte(t), []byte(esperado)) == 1
}

func metodoSeguro(m string) bool {
	return m == http.MethodGet || m == http.MethodHead || m == http.MethodOptions || m == http.MethodTrace
}

func negarCSRF(w http.ResponseWriter) {
	http.Error(w, "Acesso negado: o token de segurança da página é inválido ou expirou. Volte, recarregue a página e tente de novo.", http.StatusForbidden)
}

// ───────────────────────────── login e logout ─────────────────────────────

// publica trata /login: GET mostra o formulário (criando a sessão anônima do token CSRF), POST autentica.
func (s *Servidor) publica(w http.ResponseWriter, r *http.Request, get, post func(http.ResponseWriter, *http.Request, *Sessao)) {
	sess, _ := s.sessoes.Buscar(r.Context(), s.cookieSessao(r))
	switch r.Method {
	case http.MethodGet, http.MethodHead:
		if sess == nil || sess.Substituida {
			var err error
			if sess, err = s.sessoes.Criar(r.Context(), 0, false, 0); err != nil {
				s.erroInterno(w, err)
				return
			}
			s.gravarCookie(w, sess)
		}
		get(w, r, sess)
	case http.MethodPost:
		if sess == nil || sess.Substituida || !csrfValido(r, sess.CSRF) {
			negarCSRF(w)
			return
		}
		post(w, r, sess)
	default:
		w.Header().Set("Allow", "GET, HEAD, POST")
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
	}
}

func (s *Servidor) loginTela(w http.ResponseWriter, r *http.Request, sess *Sessao) {
	q := r.URL.Query()
	var erro, info string
	if q.Has("bloqueado") {
		erro = msgBloqueado
	}
	if q.Has("error") {
		erro = msgLoginInvalido
	}
	if q.Has("logout") {
		info = msgSaiu
	}
	if q.Has("expired") {
		info = msgExpirou
	}
	s.renderizar(w, "login.html", &Pagina{Erro: erro, Info: info, CSRF: sess.CSRF})
}

func (s *Servidor) loginEnvio(w http.ResponseWriter, r *http.Request, anonima *Sessao) {
	login := r.PostFormValue("login")
	senha := r.PostFormValue("senha")
	ip := s.ipCliente(r)

	if s.limite.Bloqueado(ip, login) {
		http.Redirect(w, r, "/login?bloqueado=true", http.StatusFound)
		return
	}
	u, ok := s.autenticar(r.Context(), login, senha)
	if !ok {
		s.limite.Falha(ip, login)
		http.Redirect(w, r, "/login?error=true", http.StatusFound)
		return
	}
	s.limite.Sucesso(ip, login)

	modoApp := r.PostFormValue("modoApp") == "1"
	duracao := s.cfg.SessaoWeb
	if modoApp {
		duracao = s.cfg.SessaoApp
	}
	nova, err := s.sessoes.Criar(r.Context(), u.ID, modoApp, duracao) // identificador novo a cada login (contra fixação de sessão)
	if err != nil {
		s.erroInterno(w, err)
		return
	}
	_ = s.sessoes.Encerrar(r.Context(), anonima.ID)
	s.gravarCookie(w, nova)
	http.Redirect(w, r, "/dashboard", http.StatusFound)
}

// autenticar confere login (sem diferenciar maiúscula/acento) e senha; usuário inativo não entra (RN-USR-02).
func (s *Servidor) autenticar(ctx context.Context, login, senha string) (*Usuario, bool) {
	var id int64
	var hash string
	var ativo int
	err := s.banco.R.QueryRowContext(ctx, `SELECT usu_codigo, usu_senha, usu_ativo FROM Usuarios WHERE usu_login = ?`, login).Scan(&id, &hash, &ativo)
	if errors.Is(err, sql.ErrNoRows) || login == "" {
		auth.ConferirSenha(s.hashFalso, senha) // gasta o mesmo tempo: não revela se o login existe
		return nil, false
	}
	if err != nil {
		s.log.Error("consulta de login", "erro", err)
		return nil, false
	}
	if !auth.ConferirSenha(hash, senha) || ativo != 1 {
		return nil, false
	}
	return &Usuario{ID: id}, true
}

func (s *Servidor) logout(w http.ResponseWriter, r *http.Request) {
	if id := s.cookieSessao(r); id != "" {
		if sess, _ := s.sessoes.Buscar(r.Context(), id); sess != nil {
			if r.Method == http.MethodPost && sess.Autenticada() && !csrfValido(r, sess.CSRF) {
				negarCSRF(w)
				return
			}
		}
		_ = s.sessoes.Encerrar(r.Context(), id)
	}
	s.apagarCookie(w)
	http.Redirect(w, r, "/login?logout=true", http.StatusFound)
}

// ───────────────────────────── área logada ─────────────────────────────

// carregarUsuario lê nome, situação e direitos A CADA requisição: uma mudança de permissão, ou a inativação do usuário, vale na hora
// (no antigo as autoridades ficavam congeladas na sessão até o próximo login; ver DECISOES T-21).
func (s *Servidor) carregarUsuario(ctx context.Context, id int64) (*Usuario, error) {
	var u Usuario
	var ativo int
	err := s.banco.R.QueryRowContext(ctx, `SELECT usu_codigo, usu_login, usu_nome, usu_ativo FROM Usuarios WHERE usu_codigo = ?`, id).Scan(&u.ID, &u.Login, &u.Nome, &ativo)
	if err != nil || ativo != 1 {
		return nil, err
	}
	rows, err := s.banco.R.QueryContext(ctx, `SELECT dusu_descricao, dusu_inclusao, dusu_exclusao, dusu_consulta, dusu_atualizacao FROM Direito_Usuario WHERE dusu_usu_codigo = ?`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var direitos []permissoes.Direito
	for rows.Next() {
		var d permissoes.Direito
		var inc, exc, con, atu int
		if err := rows.Scan(&d.Modulo, &inc, &exc, &con, &atu); err != nil {
			return nil, err
		}
		d.Inclusao, d.Exclusao, d.Consulta, d.Atualizacao = inc == 1, exc == 1, con == 1, atu == 1
		direitos = append(direitos, d)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	u.Auth = permissoes.ParaUsuario(u.Login, direitos)
	return &u, nil
}

func (s *Servidor) protegida(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sess, err := s.sessoes.Buscar(ctx, s.cookieSessao(r))
	if err != nil {
		s.erroInterno(w, err)
		return
	}
	if sess != nil && sess.Substituida { // outro login do mesmo usuário encerrou esta sessão (maximumSessions(1))
		_ = s.sessoes.Encerrar(ctx, sess.ID)
		s.apagarCookie(w)
		http.Redirect(w, r, "/login?expired=true", http.StatusFound)
		return
	}
	if !sess.Autenticada() {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	u, err := s.carregarUsuario(ctx, sess.UsuarioID)
	if err != nil {
		s.erroInterno(w, err)
		return
	}
	if u == nil { // usuário apagado ou inativado: a sessão não vale mais
		_ = s.sessoes.Encerrar(ctx, sess.ID)
		s.apagarCookie(w)
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	if !metodoSeguro(r.Method) && !csrfValido(r, sess.CSRF) {
		negarCSRF(w)
		return
	}

	// permissão por módulo/ação (PermissaoInterceptor)
	d := permissoes.Decidir(r.Method, r.URL.EscapedPath(), r.FormValue)
	if d.ExigeVendasTodos && !u.Auth.Pode("VENDAS_TODOS", permissoes.Consulta) {
		s.negar(w, r, sess, permissoes.MensagemSomaVendas)
		return
	}
	if d.Verifica && !u.Auth.Pode(d.Modulo, d.Acao) {
		s.negar(w, r, sess, permissoes.MensagemNegado(d.Modulo, d.Acao))
		return
	}

	r = r.WithContext(context.WithValue(ctx, chaveCtx{}, &contexto{Sessao: sess, Usuario: u}))
	s.mux.ServeHTTP(w, r)
}

// negar volta ao dashboard com a mensagem de uma só leitura.
func (s *Servidor) negar(w http.ResponseWriter, r *http.Request, sess *Sessao, mensagem string) {
	_ = s.sessoes.Flash(r.Context(), sess, "erro", mensagem)
	http.Redirect(w, r, "/dashboard", http.StatusFound)
}

func (s *Servidor) erroInterno(w http.ResponseWriter, err error) {
	s.log.Error("erro interno", "erro", err)
	http.Error(w, "Erro interno. Tente novamente.", http.StatusInternalServerError)
}

// ───────────────────────────── telas ─────────────────────────────

func (s *Servidor) dashboard(w http.ResponseWriter, r *http.Request) {
	s.tela(w, r, "dashboard.html", "Dashboard", "dashboard", nil)
}

// Pagina é o que os templates recebem.
type Pagina struct {
	Titulo, PaginaAtiva string
	Usuario             string
	Auth                permissoes.Conjunto
	Erro, Sucesso, Info string
	CSRF                string
	EntradaVendedor     bool
	Ano                 int
	Dados               any
}

// tela renderiza uma página com o layout base (menu por permissão, mensagens de uma só leitura, rodapé).
func (s *Servidor) tela(w http.ResponseWriter, r *http.Request, modelo, titulo, ativa string, dados any) {
	c := ctxDe(r)
	erro, sucesso, info, err := s.sessoes.LerFlash(r.Context(), c.Sessao)
	if err != nil {
		s.log.Warn("ler mensagens da sessão", "erro", err)
	}
	p := &Pagina{
		Titulo: titulo, PaginaAtiva: ativa, Usuario: c.Usuario.Login, Auth: c.Usuario.Auth,
		Erro: erro, Sucesso: sucesso, Info: info, CSRF: c.Sessao.CSRF,
		// o app instalado de quem só vê as próprias vendas e tem o app de vendas abre direto no Modo vendedor (ModoVendedor.entradaPadrao)
		EntradaVendedor: c.Usuario.Auth.Pode("APP_VENDAS", permissoes.Consulta) && !c.Usuario.Auth.Pode("VENDAS_TODOS", permissoes.Consulta),
		Ano:             s.agora().In(s.cfg.Local).Year(),
		Dados:           dados,
	}
	s.renderizar(w, modelo, p)
}

func (s *Servidor) renderizar(w http.ResponseWriter, modelo string, p *Pagina) {
	t, ok := s.paginas[modelo]
	if !ok {
		s.erroInterno(w, fmt.Errorf("template %q não existe", modelo))
		return
	}
	var buf strings.Builder
	nome := modelo
	if modelo != "login.html" {
		nome = "base.html"
	}
	if err := t.ExecuteTemplate(&buf, nome, p); err != nil {
		s.erroInterno(w, fmt.Errorf("template %s: %w", modelo, err))
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(buf.String()))
}

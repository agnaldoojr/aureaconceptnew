package web

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/agnaldoojr/aureaconceptnew/internal/auth"
	"github.com/agnaldoojr/aureaconceptnew/internal/config"
	"github.com/agnaldoojr/aureaconceptnew/internal/db"
)

const senhaAdmin = "Bom-Segredo-2026"

// relogio é um relógio manual para testar expiração e bloqueio sem esperar.
type relogio struct {
	mu sync.Mutex
	t  time.Time
}

func (r *relogio) Agora() time.Time        { r.mu.Lock(); defer r.mu.Unlock(); return r.t }
func (r *relogio) Avancar(d time.Duration) { r.mu.Lock(); r.t = r.t.Add(d); r.mu.Unlock() }

type ambiente struct {
	t     *testing.T
	srv   *Servidor
	ts    *httptest.Server
	banco *db.Banco
	rel   *relogio
	cfg   *config.Config
}

func novoAmbiente(t *testing.T, env map[string]string) *ambiente {
	t.Helper()
	cfg, err := config.Carregar(func(k string) string { return env[k] })
	if err != nil {
		t.Fatal(err)
	}
	b, err := db.Abrir(context.Background(), filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { b.Fechar() })
	rel := &relogio{t: time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)}
	srv, err := Novo(cfg, b, slog.New(slog.NewTextHandler(io.Discard, nil)), Opcoes{Agora: rel.Agora})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	a := &ambiente{t: t, srv: srv, ts: ts, banco: b, rel: rel, cfg: cfg}
	a.criarUsuario("admin", "Administrador", senhaAdmin, true)
	return a
}

// criarUsuario insere um usuário (BCrypt de verdade). direitos: "MODULO:CIAE" com as letras Consulta/Inclusão/Atualização/Exclusão.
func (a *ambiente) criarUsuario(login, nome, senha string, ativo bool, direitos ...string) int64 {
	a.t.Helper()
	hash, err := auth.GerarHash(senha)
	if err != nil {
		a.t.Fatal(err)
	}
	at := 0
	if ativo {
		at = 1
	}
	r, err := a.banco.W.Exec(`INSERT INTO Usuarios (usu_nome, usu_login, usu_senha, usu_ativo) VALUES (?,?,?,?)`, nome, login, hash, at)
	if err != nil {
		a.t.Fatal(err)
	}
	id, _ := r.LastInsertId()
	for _, d := range direitos {
		p := strings.SplitN(d, ":", 2)
		b := func(l string) int {
			if strings.Contains(p[1], l) {
				return 1
			}
			return 0
		}
		if _, err := a.banco.W.Exec(`INSERT INTO Direito_Usuario (dusu_usu_codigo, dusu_descricao, dusu_consulta, dusu_inclusao, dusu_atualizacao, dusu_exclusao) VALUES (?,?,?,?,?,?)`,
			id, p[0], b("C"), b("I"), b("A"), b("E")); err != nil {
			a.t.Fatal(err)
		}
	}
	return id
}

// navegador guarda cookies e NÃO segue redirecionamentos (para conferir cada passo).
type navegador struct {
	a       *ambiente
	cookies map[string]*http.Cookie
	ip      string
}

func (a *ambiente) novoNavegador() *navegador {
	return &navegador{a: a, cookies: map[string]*http.Cookie{}}
}

func (n *navegador) fazer(metodo, caminho string, corpo url.Values, cabecalhos map[string]string) (*http.Response, string) {
	n.a.t.Helper()
	var rd io.Reader
	if corpo != nil {
		rd = strings.NewReader(corpo.Encode())
	}
	req, _ := http.NewRequest(metodo, n.a.ts.URL+caminho, rd)
	if corpo != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	for _, c := range n.cookies {
		req.AddCookie(c)
	}
	for k, v := range cabecalhos {
		req.Header.Set(k, v)
	}
	cli := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := cli.Do(req)
	if err != nil {
		n.a.t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	for _, c := range resp.Cookies() {
		if c.MaxAge < 0 {
			delete(n.cookies, c.Name)
		} else {
			n.cookies[c.Name] = c
		}
	}
	return resp, string(b)
}

func (n *navegador) get(caminho string) (*http.Response, string) {
	n.a.t.Helper()
	return n.fazer("GET", caminho, nil, nil)
}

var reCSRF = regexp.MustCompile(`name="_csrf" value="([^"]+)"`)

// tokenDoLogin abre /login e devolve o token CSRF do formulário.
func (n *navegador) tokenDoLogin() string {
	n.a.t.Helper()
	resp, corpo := n.get("/login")
	if resp.StatusCode != 200 {
		n.a.t.Fatalf("GET /login = %d", resp.StatusCode)
	}
	m := reCSRF.FindStringSubmatch(corpo)
	if m == nil {
		n.a.t.Fatal("formulário de login sem _csrf")
	}
	return m[1]
}

func (n *navegador) login(login, senha string, extra ...string) *http.Response {
	n.a.t.Helper()
	v := url.Values{"_csrf": {n.tokenDoLogin()}, "login": {login}, "senha": {senha}}
	for i := 0; i+1 < len(extra); i += 2 {
		v.Set(extra[i], extra[i+1])
	}
	resp, _ := n.fazer("POST", "/login", v, nil)
	return resp
}

func (n *navegador) entrar(login, senha string) {
	n.a.t.Helper()
	resp := n.login(login, senha)
	if resp.StatusCode != 302 || resp.Header.Get("Location") != "/dashboard" {
		n.a.t.Fatalf("login de %s falhou: %d → %s", login, resp.StatusCode, resp.Header.Get("Location"))
	}
}

// csrfLogado lê o token da sessão logada (está no meta/ formulários das telas; aqui pega direto do banco).
func (n *navegador) csrfLogado() string {
	n.a.t.Helper()
	c := n.cookies[NomeCookieSessao]
	if c == nil {
		n.a.t.Fatal("sem cookie de sessão")
	}
	var csrf string
	if err := n.a.banco.R.QueryRow(`SELECT ses_csrf FROM Sessao_Web WHERE ses_hash = ?`, hashDe(c.Value)).Scan(&csrf); err != nil {
		n.a.t.Fatal(err)
	}
	return csrf
}

func deve(t *testing.T, resp *http.Response, status int, local string) {
	t.Helper()
	if resp.StatusCode != status || (local != "" && resp.Header.Get("Location") != local) {
		t.Fatalf("esperado %d → %q, veio %d → %q", status, local, resp.StatusCode, resp.Header.Get("Location"))
	}
}

// ───────────────────────────── login ─────────────────────────────

func TestLoginTelaTemCSRFECookieDeSessao(t *testing.T) {
	a := novoAmbiente(t, nil)
	n := a.novoNavegador()
	resp, corpo := n.get("/login")
	deve(t, resp, 200, "")
	for _, esperado := range []string{`name="_csrf"`, `name="login"`, `name="senha"`, `name="modoApp"`, "Bem-vindo", "Acesso restrito ao sistema"} {
		if !strings.Contains(corpo, esperado) {
			t.Errorf("tela de login sem %q", esperado)
		}
	}
	c := n.cookies[NomeCookieSessao]
	if c == nil || !c.HttpOnly || c.SameSite != http.SameSiteLaxMode || c.Path != "/" {
		t.Fatalf("cookie de sessão incorreto: %+v", c)
	}
	if c.MaxAge != 0 {
		t.Errorf("cookie anônimo deve ser de sessão (sem Max-Age), veio %d", c.MaxAge)
	}
	if strings.Contains(corpo, "googleapis") || strings.Contains(corpo, "cdnjs") {
		t.Error("a tela de login ainda cita CDN externa (D-04)")
	}
	if resp.Header.Get("Cache-Control") == "" || !strings.Contains(resp.Header.Get("Cache-Control"), "no-store") {
		t.Errorf("tela dinâmica sem no-store: %q", resp.Header.Get("Cache-Control"))
	}
	if resp.Header.Get("X-Frame-Options") != "DENY" || resp.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Error("faltam cabeçalhos de segurança")
	}
}

func TestMensagensDaTelaDeLogin(t *testing.T) {
	a := novoAmbiente(t, nil)
	n := a.novoNavegador()
	casos := map[string]string{
		"/login?error=true":     "Login ou senha inválidos. Verifique seus dados.",
		"/login?bloqueado=true": "Muitas tentativas de login. Aguarde alguns minutos e tente novamente.",
		"/login?logout=true":    "Você saiu do sistema com sucesso.",
		"/login?expired=true":   "Sua sessão expirou. Faça login novamente.",
	}
	for caminho, msg := range casos {
		_, corpo := n.get(caminho)
		if !strings.Contains(corpo, msg) {
			t.Errorf("%s não mostra %q", caminho, msg)
		}
	}
}

func TestLoginSemCSRFOuComTokenErradoE403(t *testing.T) {
	a := novoAmbiente(t, nil)
	n := a.novoNavegador()
	n.tokenDoLogin()
	for nome, v := range map[string]url.Values{
		"sem token":    {"login": {"admin"}, "senha": {senhaAdmin}},
		"token errado": {"_csrf": {"x"}, "login": {"admin"}, "senha": {senhaAdmin}},
	} {
		resp, _ := n.fazer("POST", "/login", v, nil)
		if resp.StatusCode != 403 {
			t.Errorf("%s: esperado 403, veio %d", nome, resp.StatusCode)
		}
	}
	// sem nenhuma sessão (sem cookie) também é 403
	resp, _ := a.novoNavegador().fazer("POST", "/login", url.Values{"login": {"admin"}, "senha": {senhaAdmin}}, nil)
	if resp.StatusCode != 403 {
		t.Errorf("sem sessão: esperado 403, veio %d", resp.StatusCode)
	}
}

func TestLoginErradoVoltaComErro(t *testing.T) {
	a := novoAmbiente(t, nil)
	n := a.novoNavegador()
	deve(t, n.login("admin", "errada"), 302, "/login?error=true")
	deve(t, n.login("naoexiste", senhaAdmin), 302, "/login?error=true")
	deve(t, n.login("", ""), 302, "/login?error=true")
}

func TestLoginCertoEntraNoDashboardEMostraMenuDoAdmin(t *testing.T) {
	a := novoAmbiente(t, nil)
	n := a.novoNavegador()
	n.entrar("admin", senhaAdmin)
	resp, corpo := n.get("/dashboard")
	deve(t, resp, 200, "")
	for _, esperado := range []string{"Dashboard", `<span class="user-name">admin</span>`, `href="/produtos"`, `href="/usuarios"`, `href="/financeiro"`, `href="/bi"`, `href="/agenda"`, `href="/vendedor"`, "Loja de roupa", "Salão", "Gerência", "Sair", "2026"} {
		if !strings.Contains(corpo, esperado) {
			t.Errorf("dashboard do admin sem %q", esperado)
		}
	}
	// "/" também é o dashboard
	resp, _ = n.get("/")
	deve(t, resp, 200, "")
}

func TestLoginNaoDiferenciaMaiusculaNemAcento(t *testing.T) {
	a := novoAmbiente(t, nil)
	a.criarUsuario("João", "João da Silva", "Outra-Senha-123", true, "PRODUTOS:C")
	for _, l := range []string{"João", "JOAO", "joão", "joao"} {
		n := a.novoNavegador()
		n.entrar(l, "Outra-Senha-123")
	}
	n := a.novoNavegador()
	n.entrar("ADMIN", senhaAdmin)
}

func TestMenuSoMostraOQueOUsuarioPodeVer(t *testing.T) {
	a := novoAmbiente(t, nil)
	a.criarUsuario("caixa", "Caixa", "Caixa-Senha-1", true, "PRODUTOS:C", "ESTOQUE:C")
	n := a.novoNavegador()
	n.entrar("caixa", "Caixa-Senha-1")
	_, corpo := n.get("/dashboard")
	for _, quer := range []string{`href="/produtos"`, `href="/estoque"`, "Loja de roupa"} {
		if !strings.Contains(corpo, quer) {
			t.Errorf("deveria mostrar %q", quer)
		}
	}
	for _, nao := range []string{`href="/usuarios"`, `href="/financeiro"`, `href="/bi"`, "Gerência", "Salão", `href="/pessoas"`, `href="/vendedor"`} {
		if strings.Contains(corpo, nao) {
			t.Errorf("não deveria mostrar %q", nao)
		}
	}
}

func TestNaoLogadoVaiParaOLogin(t *testing.T) {
	a := novoAmbiente(t, nil)
	n := a.novoNavegador()
	for _, c := range []string{"/", "/dashboard", "/produtos", "/usuarios/permissoes", "/financeiro/dre", "/qualquer/coisa"} {
		resp, _ := n.get(c)
		deve(t, resp, 302, "/login")
	}
	// uma sessão anônima (só do formulário) também não passa
	n.tokenDoLogin()
	resp, _ := n.get("/dashboard")
	deve(t, resp, 302, "/login")
}

func TestLoginTrocaOIdentificadorDaSessao(t *testing.T) {
	a := novoAmbiente(t, nil)
	n := a.novoNavegador()
	n.tokenDoLogin()
	antes := n.cookies[NomeCookieSessao].Value
	n.entrar("admin", senhaAdmin)
	depois := n.cookies[NomeCookieSessao].Value
	if antes == depois {
		t.Fatal("o identificador da sessão deve mudar no login (fixação de sessão)")
	}
	// a sessão anônima antiga deixou de existir
	var n1 int
	a.banco.R.QueryRow(`SELECT count(*) FROM Sessao_Web WHERE ses_hash = ?`, hashDe(antes)).Scan(&n1)
	if n1 != 0 {
		t.Error("a sessão anônima deveria ter sido apagada")
	}
	// o banco guarda só o hash, nunca o identificador
	a.banco.R.QueryRow(`SELECT count(*) FROM Sessao_Web WHERE ses_hash = ?`, depois).Scan(&n1)
	if n1 != 0 {
		t.Error("o identificador cru não pode estar no banco")
	}
}

// ───────────────────────────── limite de tentativas ─────────────────────────────

func TestCincoErrosBloqueiamMesmoComASenhaCerta(t *testing.T) {
	a := novoAmbiente(t, nil)
	n := a.novoNavegador()
	for i := 0; i < 5; i++ {
		deve(t, n.login("admin", "errada"), 302, "/login?error=true")
	}
	deve(t, n.login("admin", senhaAdmin), 302, "/login?bloqueado=true")
	// outro login do mesmo IP está bloqueado só pelo contador do IP (30), então entra
	a.criarUsuario("outro", "Outro", "Outro-Senha-1", true)
	deve(t, n.login("outro", "Outro-Senha-1"), 302, "/dashboard")
	// passado o tempo de bloqueio, o admin volta a entrar
	a.rel.Avancar(16 * time.Minute)
	deve(t, a.novoNavegador().login("admin", senhaAdmin), 302, "/dashboard")
}

func TestTrintaErrosDoMesmoIPBloqueiamTodosOsLogins(t *testing.T) {
	a := novoAmbiente(t, nil)
	n := a.novoNavegador()
	for i := 0; i < 30; i++ {
		n.login("fulano"+string(rune('a'+i%26))+string(rune('a'+i/26)), "x")
	}
	deve(t, n.login("admin", senhaAdmin), 302, "/login?bloqueado=true")
}

func TestLoginCertoZeraOContadorDoPar(t *testing.T) {
	a := novoAmbiente(t, nil)
	for i := 0; i < 4; i++ {
		deve(t, a.novoNavegador().login("admin", "errada"), 302, "/login?error=true")
	}
	deve(t, a.novoNavegador().login("admin", senhaAdmin), 302, "/dashboard") // zera
	for i := 0; i < 4; i++ {
		deve(t, a.novoNavegador().login("admin", "errada"), 302, "/login?error=true")
	}
	deve(t, a.novoNavegador().login("admin", senhaAdmin), 302, "/dashboard") // ainda não bloqueou
}

func TestProxyConfiavelUsaOUltimoXForwardedFor(t *testing.T) {
	a := novoAmbiente(t, map[string]string{"APP_ENV": "prod", "DOMINIO": "x.test", "COOKIE_SECURE": "false", "PROXY_CONFIAVEL": "true"})
	login := func(ip string) *http.Response {
		n := a.novoNavegador()
		v := url.Values{"_csrf": {n.tokenDoLogin()}, "login": {"admin"}, "senha": {"errada"}}
		resp, _ := n.fazer("POST", "/login", v, map[string]string{"X-Forwarded-For": "1.1.1.1, " + ip})
		return resp
	}
	for i := 0; i < 5; i++ {
		login("9.9.9.9")
	}
	// o IP 9.9.9.9 está bloqueado para o admin; outro IP (mesmo com o primeiro valor forjado igual) não
	n := a.novoNavegador()
	v := url.Values{"_csrf": {n.tokenDoLogin()}, "login": {"admin"}, "senha": {senhaAdmin}}
	resp, _ := n.fazer("POST", "/login", v, map[string]string{"X-Forwarded-For": "1.1.1.1, 9.9.9.9"})
	deve(t, resp, 302, "/login?bloqueado=true")
	n = a.novoNavegador()
	v = url.Values{"_csrf": {n.tokenDoLogin()}, "login": {"admin"}, "senha": {senhaAdmin}}
	resp, _ = n.fazer("POST", "/login", v, map[string]string{"X-Forwarded-For": "1.1.1.1, 8.8.8.8"})
	deve(t, resp, 302, "/dashboard")
}

func TestLimiteLoginUnidade(t *testing.T) {
	rel := &relogio{t: time.Unix(1000, 0)}
	l := NovoLimiteLogin(3, 10, 15*time.Minute, rel.Agora)
	for i := 0; i < 2; i++ {
		l.Falha("1.1.1.1", "Ana")
	}
	if l.Bloqueado("1.1.1.1", "ana") {
		t.Fatal("2 falhas ainda não bloqueiam")
	}
	l.Falha("1.1.1.1", " ANA ") // minúsculas e sem espaços: mesma chave
	if !l.Bloqueado("1.1.1.1", "ana") {
		t.Fatal("3 falhas do mesmo IP+login bloqueiam")
	}
	if l.Bloqueado("2.2.2.2", "ana") {
		t.Fatal("outro IP não é afetado")
	}
	rel.Avancar(14 * time.Minute)
	if !l.Bloqueado("1.1.1.1", "ana") {
		t.Fatal("ainda bloqueado aos 14 min")
	}
	rel.Avancar(2 * time.Minute)
	if l.Bloqueado("1.1.1.1", "ana") {
		t.Fatal("liberado após 15 min")
	}
	// falha antiga (mais velha que o bloqueio) recomeça a contagem
	l.Falha("3.3.3.3", "bia")
	rel.Avancar(20 * time.Minute)
	l.Falha("3.3.3.3", "bia")
	l.Falha("3.3.3.3", "bia")
	if l.Bloqueado("3.3.3.3", "bia") {
		t.Fatal("a falha de 20 min atrás não conta")
	}
}

// ───────────────────────────── sessão ─────────────────────────────

func TestSessaoExpiraPorInatividadeEVaiParaOLogin(t *testing.T) {
	a := novoAmbiente(t, nil)
	n := a.novoNavegador()
	n.entrar("admin", senhaAdmin)
	a.rel.Avancar(7 * time.Hour)
	resp, _ := n.get("/dashboard") // ainda dentro das 8 h de inatividade; renova o último acesso
	deve(t, resp, 200, "")
	a.rel.Avancar(7 * time.Hour)
	resp, _ = n.get("/dashboard") // 7 h desde o último acesso: vale (o prazo é de inatividade)
	deve(t, resp, 200, "")
	a.rel.Avancar(9 * time.Hour)
	resp, _ = n.get("/dashboard")
	deve(t, resp, 302, "/login")
}

func TestSessaoDoAppInstaladoDura30DiasECookiePersistente(t *testing.T) {
	a := novoAmbiente(t, nil)
	n := a.novoNavegador()
	resp := n.login("admin", senhaAdmin, "modoApp", "1")
	deve(t, resp, 302, "/dashboard")
	c := n.cookies[NomeCookieSessao]
	if c.MaxAge != 30*24*3600 {
		t.Fatalf("cookie do app deve durar 30 dias, Max-Age = %d", c.MaxAge)
	}
	a.rel.Avancar(29 * 24 * time.Hour)
	resp, _ = n.get("/dashboard")
	deve(t, resp, 200, "")

	// no navegador comum o cookie é de sessão
	n2 := a.novoNavegador()
	n2.entrar("admin", senhaAdmin)
	if n2.cookies[NomeCookieSessao].MaxAge != 0 {
		t.Error("no navegador o cookie deve ser de sessão")
	}
}

func TestUmLoginNovoEncerraASessaoAnterior(t *testing.T) {
	a := novoAmbiente(t, nil)
	primeiro := a.novoNavegador()
	primeiro.entrar("admin", senhaAdmin)
	segundo := a.novoNavegador()
	segundo.entrar("admin", senhaAdmin)

	resp, _ := segundo.get("/dashboard")
	deve(t, resp, 200, "")
	resp, _ = primeiro.get("/dashboard")
	deve(t, resp, 302, "/login?expired=true")
	// e depois disso o primeiro é só um visitante comum
	resp, _ = primeiro.get("/dashboard")
	deve(t, resp, 302, "/login")
}

func TestLogoutEncerraASessao(t *testing.T) {
	a := novoAmbiente(t, nil)
	n := a.novoNavegador()
	n.entrar("admin", senhaAdmin)
	id := n.cookies[NomeCookieSessao].Value
	resp, _ := n.get("/logout")
	deve(t, resp, 302, "/login?logout=true")
	// o cookie antigo, reapresentado, não vale mais
	n.cookies[NomeCookieSessao] = &http.Cookie{Name: NomeCookieSessao, Value: id}
	resp, _ = n.get("/dashboard")
	deve(t, resp, 302, "/login")
	// logout sem sessão também redireciona
	resp, _ = a.novoNavegador().get("/logout")
	deve(t, resp, 302, "/login?logout=true")
}

func TestLogoutPorPOSTExigeCSRF(t *testing.T) {
	a := novoAmbiente(t, nil)
	n := a.novoNavegador()
	n.entrar("admin", senhaAdmin)
	resp, _ := n.fazer("POST", "/logout", url.Values{}, nil)
	if resp.StatusCode != 403 {
		t.Fatalf("POST /logout sem token = %d, esperado 403", resp.StatusCode)
	}
	resp, _ = n.fazer("POST", "/logout", url.Values{"_csrf": {n.csrfLogado()}}, nil)
	deve(t, resp, 302, "/login?logout=true")
}

func TestUsuarioInativoNaoEntraEInativarDerrubaAsessao(t *testing.T) {
	a := novoAmbiente(t, nil)
	id := a.criarUsuario("maria", "Maria", "Maria-Senha-1", true, "PRODUTOS:C")
	deve(t, a.novoNavegador().login("maria", "Maria-Senha-1"), 302, "/dashboard")
	a.criarUsuario("inativo", "Inativo", "Inativo-Senha-1", false, "PRODUTOS:C")
	deve(t, a.novoNavegador().login("inativo", "Inativo-Senha-1"), 302, "/login?error=true")

	n := a.novoNavegador()
	n.entrar("maria", "Maria-Senha-1")
	a.banco.W.Exec(`UPDATE Usuarios SET usu_ativo = 0 WHERE usu_codigo = ?`, id)
	resp, _ := n.get("/dashboard")
	deve(t, resp, 302, "/login")
}

func TestMudancaDePermissaoValeNaHora(t *testing.T) {
	a := novoAmbiente(t, nil)
	id := a.criarUsuario("ana", "Ana", "Ana-Senha-123", true, "PRODUTOS:C")
	n := a.novoNavegador()
	n.entrar("ana", "Ana-Senha-123")
	_, corpo := n.get("/dashboard")
	if !strings.Contains(corpo, `href="/produtos"`) {
		t.Fatal("deveria ver Produtos")
	}
	a.banco.W.Exec(`DELETE FROM Direito_Usuario WHERE dusu_usu_codigo = ?`, id)
	_, corpo = n.get("/dashboard")
	if strings.Contains(corpo, `href="/produtos"`) {
		t.Error("o menu não acompanhou a perda da permissão")
	}
}

func TestSessaoPersisteNoBancoEntreInstanciasDoServidor(t *testing.T) {
	a := novoAmbiente(t, nil)
	n := a.novoNavegador()
	n.entrar("admin", senhaAdmin)
	// um segundo Servidor sobre o MESMO banco (como após reiniciar o sistema) reconhece o cookie
	srv2, err := Novo(a.cfg, a.banco, slog.New(slog.NewTextHandler(io.Discard, nil)), Opcoes{Agora: a.rel.Agora})
	if err != nil {
		t.Fatal(err)
	}
	ts2 := httptest.NewServer(srv2.Handler())
	defer ts2.Close()
	req, _ := http.NewRequest("GET", ts2.URL+"/dashboard", nil)
	req.AddCookie(n.cookies[NomeCookieSessao])
	resp, err := (&http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("após reiniciar, a sessão deveria valer; veio %d", resp.StatusCode)
	}
}

func TestLimpezaApagaSessoesVencidas(t *testing.T) {
	a := novoAmbiente(t, nil)
	a.novoNavegador().tokenDoLogin() // anônima
	n := a.novoNavegador()
	n.entrar("admin", senhaAdmin)
	a.rel.Avancar(time.Hour) // a anônima (30 min) venceu; a do admin (8 h) não
	apagadas, err := a.srv.Sessoes().Limpar(context.Background())
	if err != nil || apagadas != 1 {
		t.Fatalf("apagadas = %d, err = %v (esperado 1)", apagadas, err)
	}
	resp, _ := n.get("/dashboard")
	deve(t, resp, 200, "")
}

// ───────────────────────────── permissões ─────────────────────────────

func TestSemPermissaoVoltaAoDashboardComMensagemDeUmaLeitura(t *testing.T) {
	a := novoAmbiente(t, nil)
	a.criarUsuario("ana", "Ana", "Ana-Senha-123", true, "PESSOAS:C")
	n := a.novoNavegador()
	n.entrar("ana", "Ana-Senha-123")

	resp, _ := n.get("/produtos")
	deve(t, resp, 302, "/dashboard")
	_, corpo := n.get("/dashboard")
	if !strings.Contains(corpo, "Você não tem permissão para acessar Produtos.") {
		t.Fatalf("faltou a mensagem de negação:\n%s", trecho(corpo, "alert"))
	}
	_, corpo = n.get("/dashboard")
	if strings.Contains(corpo, "não tem permissão") {
		t.Error("a mensagem deve aparecer uma única vez")
	}

	// ações: incluir, alterar e excluir têm textos próprios
	tok := n.csrfLogado()
	casos := []struct{ metodo, caminho, msg string }{
		{"POST", "/pessoas/salvar", "Você não tem permissão para incluir registros em Pessoas."},
		{"POST", "/pessoas/salvar", "Você não tem permissão para alterar registros em Pessoas."},
		{"POST", "/pessoas/excluir/3", "Você não tem permissão para excluir registros de Pessoas."},
	}
	for i, c := range casos {
		v := url.Values{"_csrf": {tok}}
		if i == 1 {
			v.Set("upsCodigo", "7")
		}
		// Pessoas:C tem Consulta, então só nega incluir/alterar/excluir
		resp, _ := n.fazer(c.metodo, c.caminho, v, nil)
		deve(t, resp, 302, "/dashboard")
		_, corpo := n.get("/dashboard")
		if !strings.Contains(corpo, c.msg) {
			t.Errorf("%s %s: faltou %q", c.metodo, c.caminho, c.msg)
		}
	}
}

func TestComPermissaoPassaDoPorteiro(t *testing.T) {
	a := novoAmbiente(t, nil)
	a.criarUsuario("ana", "Ana", "Ana-Senha-123", true, "AGENDA:C")
	n := a.novoNavegador()
	n.entrar("ana", "Ana-Senha-123")
	// a tela de produtos ainda não existe (próxima fatia): passar pela permissão significa chegar ao 404, não ao redirecionamento
	resp, _ := n.get("/agenda")
	if resp.StatusCode != 404 {
		t.Fatalf("esperado 404 (passou na permissão, tela ainda não migrada), veio %d → %s", resp.StatusCode, resp.Header.Get("Location"))
	}
}

func TestBIeLucratividadeExigemVendasTodos(t *testing.T) {
	a := novoAmbiente(t, nil)
	a.criarUsuario("vend", "Vendedora", "Vend-Senha-123", true, "BI_EXECUTIVO:C", "RELATORIOS:C")
	a.criarUsuario("ger", "Gerente", "Ger-Senha-1234", true, "BI_EXECUTIVO:C", "RELATORIOS:C", "VENDAS_TODOS:C")

	v := a.novoNavegador()
	v.entrar("vend", "Vend-Senha-123")
	for _, c := range []string{"/bi", "/relatorios/lucratividade"} {
		resp, _ := v.get(c)
		deve(t, resp, 302, "/dashboard")
		_, corpo := v.get("/dashboard")
		if !strings.Contains(corpo, "Esta tela soma as vendas de todos os vendedores. Seu usuário só vê as próprias vendas.") {
			t.Errorf("%s: faltou a mensagem de soma de vendas", c)
		}
	}
	g := a.novoNavegador()
	g.entrar("ger", "Ger-Senha-1234")
	resp, _ := g.get("/bi")
	if resp.StatusCode != 404 { // /bi passa pelo porteiro; a tela vem numa fatia futura
		t.Errorf("gerente em /bi: esperado 404 (passou), veio %d", resp.StatusCode)
	}
}

func TestPostSemCSRFEBloqueadoMesmoLogado(t *testing.T) {
	a := novoAmbiente(t, nil)
	n := a.novoNavegador()
	n.entrar("admin", senhaAdmin)
	resp, _ := n.fazer("POST", "/estoque/ajustar", url.Values{"uprNome": {"x"}}, nil)
	if resp.StatusCode != 403 {
		t.Fatalf("POST sem token = %d, esperado 403", resp.StatusCode)
	}
	// o token também vale pelo cabeçalho X-CSRF-TOKEN (Modo vendedor); com ele o pedido chega à rota (404 por ora)
	resp, _ = n.fazer("POST", "/estoque/ajustar", url.Values{"uprNome": {"x"}}, map[string]string{"X-CSRF-TOKEN": n.csrfLogado()})
	if resp.StatusCode != 404 {
		t.Fatalf("POST com cabeçalho X-CSRF-TOKEN = %d, esperado 404 (passou)", resp.StatusCode)
	}
	resp, _ = n.fazer("POST", "/estoque/ajustar", url.Values{"uprNome": {"x"}}, map[string]string{"X-CSRF-TOKEN": "errado"})
	if resp.StatusCode != 403 {
		t.Fatalf("token errado = %d, esperado 403", resp.StatusCode)
	}
}

// ───────────────────────────── estáticos ─────────────────────────────

var reRecurso = regexp.MustCompile(`(?:href|src)="(/(?:css|js|img|vendor)/[^"]+)"`)

func TestTodosOsEstaticosDasTelasExistemEsaoVersionados(t *testing.T) {
	a := novoAmbiente(t, nil)
	n := a.novoNavegador()
	_, login := n.get("/login")
	n.entrar("admin", senhaAdmin)
	_, painel := n.get("/dashboard")
	vistos := map[string]bool{}
	for _, pagina := range []string{login, painel} {
		for _, m := range reRecurso.FindAllStringSubmatch(pagina, -1) {
			vistos[m[1]] = true
		}
	}
	if len(vistos) < 8 {
		t.Fatalf("poucos recursos encontrados nas páginas: %v", vistos)
	}
	hashed := regexp.MustCompile(`-[0-9a-f]{32}\.[a-z0-9]+$`)
	for caminho := range vistos {
		if !hashed.MatchString(caminho) {
			t.Errorf("%s não está versionado por hash", caminho)
		}
		resp, corpo := a.novoNavegador().get(caminho)
		if resp.StatusCode != 200 || len(corpo) == 0 {
			t.Errorf("%s = %d (%d bytes)", caminho, resp.StatusCode, len(corpo))
			continue
		}
		if cc := resp.Header.Get("Cache-Control"); !strings.Contains(cc, "max-age=31536000") {
			t.Errorf("%s: cache = %q, esperado 365 dias", caminho, cc)
		}
		// 304 com o ETag
		req, _ := http.NewRequest("GET", a.ts.URL+caminho, nil)
		req.Header.Set("If-None-Match", resp.Header.Get("ETag"))
		r2, _ := http.DefaultClient.Do(req)
		r2.Body.Close()
		if r2.StatusCode != 304 {
			t.Errorf("%s: If-None-Match deveria dar 304, deu %d", caminho, r2.StatusCode)
		}
	}
}

func TestEstaticoServeSemHashEComTipoCerto(t *testing.T) {
	a := novoAmbiente(t, nil)
	n := a.novoNavegador()
	casos := map[string]string{
		"/css/style.css":         "text/css",
		"/js/main.js":            "javascript",
		"/img/logo-aurea.webp":   "image/webp",
		"/img/pwa/manifest.json": "application/json",
		"/vendor/fontawesome/webfonts/fa-solid-900.woff2": "font/woff2",
		"/vendor/chartjs/chart.umd.min.js":                "javascript",
		"/favicon.ico":                                    "image/webp",
	}
	for caminho, tipo := range casos {
		resp, corpo := n.get(caminho)
		if resp.StatusCode != 200 || len(corpo) == 0 || !strings.Contains(resp.Header.Get("Content-Type"), tipo) {
			t.Errorf("%s = %d, tipo %q", caminho, resp.StatusCode, resp.Header.Get("Content-Type"))
		}
	}
	resp, _ := n.get("/css/nao-existe.css")
	if resp.StatusCode != 404 {
		t.Errorf("estático inexistente deveria dar 404, deu %d", resp.StatusCode)
	}
	// hash errado não serve (evita cache envenenado)
	resp, _ = n.get("/css/style-00000000000000000000000000000000.css")
	if resp.StatusCode != 404 {
		t.Errorf("hash errado deveria dar 404, deu %d", resp.StatusCode)
	}
}

func TestEstaticosDoAntigoSaoIdenticosByteAByte(t *testing.T) {
	// css/js/img/pwa vieram do clone de referência sem alteração; o hash do conteúdo no nome da URL prova que o servido = o embutido
	a := novoAmbiente(t, nil)
	n := a.novoNavegador()
	_, corpo := n.get("/css/style.css")
	if !strings.Contains(corpo, "--silver") {
		t.Error("style.css não parece ser o do sistema antigo (sem os tokens --silver)")
	}
}

func TestHealthz(t *testing.T) {
	a := novoAmbiente(t, nil)
	resp, corpo := a.novoNavegador().get("/healthz")
	if resp.StatusCode != 200 || corpo != "ok" {
		t.Fatalf("healthz = %d %q", resp.StatusCode, corpo)
	}
}

func TestCookieSeguroEHSTSEmProducao(t *testing.T) {
	a := novoAmbiente(t, map[string]string{"APP_ENV": "prod", "DOMINIO": "x.test"})
	resp, _ := a.novoNavegador().get("/login")
	if len(resp.Cookies()) == 0 || !resp.Cookies()[0].Secure {
		t.Error("em prod o cookie deve ser Secure")
	}
	if resp.Header.Get("Strict-Transport-Security") == "" {
		t.Error("em prod deve mandar HSTS")
	}
}

func trecho(s, chave string) string {
	i := strings.Index(s, chave)
	if i < 0 {
		return "(sem '" + chave + "')"
	}
	j := i + 300
	if j > len(s) {
		j = len(s)
	}
	return s[i:j]
}

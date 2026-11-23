package web

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/agnaldoojr/aureaconceptnew/internal/cadastro"
	"github.com/agnaldoojr/aureaconceptnew/internal/permissoes"
)

func (s *Servidor) rotasUsuarios() {
	s.mux.HandleFunc("GET /usuarios", s.usuariosLista)
	s.mux.HandleFunc("GET /usuarios/novo", s.usuariosNovo)
	s.mux.HandleFunc("GET /usuarios/editar/{id}", s.usuariosEditar)
	s.mux.HandleFunc("POST /usuarios/salvar", s.usuariosSalvar)
	s.mux.HandleFunc("POST /usuarios/status/{id}", s.usuariosStatus)
	s.mux.HandleFunc("POST /usuarios/excluir/{id}", s.usuariosExcluir)
	s.mux.HandleFunc("GET /usuarios/permissoes/{id}", s.permissoesTela)
	s.mux.HandleFunc("POST /usuarios/permissoes/{id}/perfil", s.permissoesPerfil)
	s.mux.HandleFunc("POST /usuarios/permissoes/{id}", s.permissoesSalvar)
}

// Também aqui o menu lateral não destaca "Usuários" (mesma causa de Pessoas; ver paginaAtivaPessoas).
const paginaAtivaUsuarios = ""

func (s *Servidor) usuariosLista(w http.ResponseWriter, r *http.Request) {
	lista, err := s.cad.ListarUsuarios(r.Context())
	if err != nil {
		s.erroInterno(w, err)
		return
	}
	s.tela(w, r, "usuarios/lista.html", "Usuários", paginaAtivaUsuarios, map[string]any{"Usuarios": lista})
}

type perfilOpcao struct{ Codigo, Nome, Descricao string }

func perfisParaTela() []perfilOpcao {
	var l []perfilOpcao
	for _, p := range permissoes.Perfis {
		l = append(l, perfilOpcao{p.Codigo, p.Nome, p.Descricao})
	}
	return l
}

// usuarioView é o usuário como o formulário o mostra.
type usuarioView struct {
	Codigo, Nome, Login string
	Ativo               bool
	VendedorSel         int64 // 0 = nenhum
}

func (s *Servidor) usuariosForm(w http.ResponseWriter, r *http.Request, titulo string, v usuarioView, erroSenha string) {
	vend, err := s.cad.VendedoresAtivos(r.Context())
	if err != nil {
		s.erroInterno(w, err)
		return
	}
	s.tela(w, r, "usuarios/form.html", titulo, paginaAtivaUsuarios, map[string]any{
		"Usuario": v, "Novo": v.Codigo == "", "Perfis": perfisParaTela(), "Vendedores": vend, "ErroSenha": erroSenha,
	})
}

func (s *Servidor) usuariosNovo(w http.ResponseWriter, r *http.Request) {
	s.usuariosForm(w, r, "Novo Usuário", usuarioView{Ativo: true}, "")
}

func (s *Servidor) usuariosEditar(w http.ResponseWriter, r *http.Request) {
	id, ok := idDoCaminho(r)
	if !ok {
		pedidoRuim(w)
		return
	}
	u, err := s.cad.BuscarUsuario(r.Context(), id)
	if err != nil {
		s.erroInterno(w, err)
		return
	}
	if u == nil {
		s.flash(r, "erro", "Usuário não encontrado.")
		s.redirecionar(w, r, "/usuarios")
		return
	}
	s.usuariosForm(w, r, "Editar Usuário", usuarioView{Codigo: strconv.FormatInt(u.Codigo, 10), Nome: u.Nome, Login: u.Login, Ativo: u.Ativo, VendedorSel: zeroSeNil(u.VendedorID)}, "")
}

func (s *Servidor) usuariosSalvar(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		pedidoRuim(w)
		return
	}
	cod, _, ok := inteiroOpcional(r.PostFormValue("usuCodigo"))
	if !ok {
		pedidoRuim(w)
		return
	}
	vend, temVend, ok := inteiroOpcional(r.PostFormValue("vendedorId"))
	if !ok {
		pedidoRuim(w)
		return
	}
	u := cadastro.Usuario{Codigo: cod, Nome: r.PostFormValue("usuNome"), Login: r.PostFormValue("usuLogin"), Ativo: caixa(r, "usuAtivo", true)}
	v := usuarioView{Nome: u.Nome, Login: u.Login, Ativo: u.Ativo}
	if cod != 0 {
		v.Codigo = strconv.FormatInt(cod, 10)
	}
	novaSenha := r.PostFormValue("novaSenha")

	// o antigo volta ao formulário (sem mensagem nos campos) e, para usuário novo, avisa só da senha
	if len(cadastro.ValidarUsuario(&u)) > 0 || caixaRuim(r, "usuAtivo") {
		s.usuariosForm(w, r, "Usuário", v, "")
		return
	}
	if cod == 0 && emBranco(novaSenha) {
		s.usuariosForm(w, r, "Usuário", v, "Senha é obrigatória para novos usuários.")
		return
	}
	var vendPtr *int64
	if temVend {
		vendPtr = &vend
	}
	if _, err := s.cad.SalvarUsuario(r.Context(), u, novaSenha, vendPtr, r.PostFormValue("perfil")); err != nil {
		s.flash(r, "erro", "Erro: "+s.mensagemDeErro(err, "não foi possível salvar."))
	} else {
		s.flash(r, "sucesso", "Usuário salvo com sucesso!")
	}
	s.redirecionar(w, r, "/usuarios")
}

// emBranco é o `isBlank()` do Java: só espaços (ou vazio).
func emBranco(s string) bool {
	for _, r := range s {
		if r > ' ' && r != 0x85 && r != 0xa0 {
			return false
		}
	}
	return true
}

func (s *Servidor) usuariosStatus(w http.ResponseWriter, r *http.Request) {
	id, ok := idDoCaminho(r)
	if !ok {
		pedidoRuim(w)
		return
	}
	ativo, ok := booleano(r.PostFormValue("ativo"))
	if !ok {
		pedidoRuim(w)
		return
	}
	if err := s.cad.AlterarStatusUsuario(r.Context(), id, ativo); err != nil {
		var re cadastro.ErroRegra
		if !errors.As(err, &re) {
			s.erroInterno(w, err)
			return
		}
		s.flash(r, "erro", string(re))
	} else {
		s.flash(r, "sucesso", "Status do usuário atualizado!")
	}
	s.redirecionar(w, r, "/usuarios")
}

func (s *Servidor) usuariosExcluir(w http.ResponseWriter, r *http.Request) {
	id, ok := idDoCaminho(r)
	if !ok {
		pedidoRuim(w)
		return
	}
	if err := s.cad.ExcluirUsuario(r.Context(), id); err != nil {
		var re cadastro.ErroRegra
		if errors.As(err, &re) {
			s.flash(r, "erro", string(re))
		} else {
			s.log.Info("usuário não excluído", "codigo", id, "erro", err)
			s.flash(r, "erro", "Não foi possível excluir.")
		}
	} else {
		s.flash(r, "sucesso", "Usuário excluído!")
	}
	s.redirecionar(w, r, "/usuarios")
}

// ───────────────────────────── permissões ─────────────────────────────

type linhaPermissao struct {
	Modulo, Rotulo, Icone                     string
	Inclusao, Exclusao, Consulta, Atualizacao bool
	TemInclusao, TemExclusao, TemAtualizacao  bool // colunas que o módulo oferece (ver o template do antigo)
}

func (s *Servidor) permissoesTela(w http.ResponseWriter, r *http.Request) {
	id, ok := idDoCaminho(r)
	if !ok {
		pedidoRuim(w)
		return
	}
	ctx := r.Context()
	u, err := s.cad.BuscarUsuario(ctx, id)
	if err != nil {
		s.erroInterno(w, err)
		return
	}
	if u == nil {
		s.flash(r, "erro", "Usuário não encontrado.")
		s.redirecionar(w, r, "/usuarios")
		return
	}
	direitos, err := s.cad.PermissoesDoUsuario(ctx, id)
	if err != nil {
		s.erroInterno(w, err)
		return
	}
	var linhas []linhaPermissao
	for _, d := range direitos {
		somenteConsulta := d.Modulo == "VENDAS_TODOS" || d.Modulo == "COMISSOES" || d.Modulo == "AGENDA_TODOS"
		linhas = append(linhas, linhaPermissao{
			Modulo: d.Modulo, Rotulo: permissoes.Rotulo(d.Modulo), Icone: permissoes.Icone(d.Modulo),
			Inclusao: d.Inclusao, Exclusao: d.Exclusao, Consulta: d.Consulta, Atualizacao: d.Atualizacao,
			TemInclusao: !somenteConsulta, TemExclusao: !somenteConsulta && d.Modulo != "METAS", TemAtualizacao: !somenteConsulta && d.Modulo != "METAS",
		})
	}
	s.tela(w, r, "permissoes/index.html", "Permissões — "+u.Nome, paginaAtivaUsuarios, map[string]any{
		"Usuario": u, "Linhas": linhas, "Perfis": perfisParaTela(), "UsuarioAdmin": permissoes.EhAdmin(u.Login),
	})
}

func (s *Servidor) permissoesPerfil(w http.ResponseWriter, r *http.Request) {
	id, ok := idDoCaminho(r)
	if !ok {
		pedidoRuim(w)
		return
	}
	if err := r.ParseForm(); err != nil {
		pedidoRuim(w)
		return
	}
	vals, tem := r.PostForm["perfil"]
	if !tem || len(vals) == 0 { // @RequestParam obrigatório: sem ele o Spring responde 400
		pedidoRuim(w)
		return
	}
	if err := s.cad.AplicarPerfil(r.Context(), id, vals[0]); err != nil {
		s.flash(r, "erro", "Não foi possível aplicar o perfil: "+s.mensagemDeErro(err, "erro interno."))
	} else {
		s.flash(r, "sucesso", "Perfil aplicado! Valem na hora para o usuário. Ajuste abaixo o que precisar.")
	}
	s.redirecionar(w, r, "/usuarios/permissoes/"+strconv.FormatInt(id, 10))
}

func (s *Servidor) permissoesSalvar(w http.ResponseWriter, r *http.Request) {
	id, ok := idDoCaminho(r)
	if !ok {
		pedidoRuim(w)
		return
	}
	if err := r.ParseForm(); err != nil {
		pedidoRuim(w)
		return
	}
	marcadas := map[string]bool{}
	for k := range r.PostForm {
		marcadas[k] = true
	}
	if err := s.cad.SalvarPermissoes(r.Context(), id, marcadas); err != nil {
		s.flash(r, "erro", "Não foi possível salvar as permissões: "+s.mensagemDeErro(err, "erro interno."))
	} else {
		s.flash(r, "sucesso", "Permissões salvas! Valem na hora para o usuário.")
	}
	s.redirecionar(w, r, "/usuarios/permissoes/"+strconv.FormatInt(id, 10))
}

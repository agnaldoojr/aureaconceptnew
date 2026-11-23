package web

import (
	"net/http"

	"github.com/agnaldoojr/aureaconceptnew/internal/cadastro"
)

func (s *Servidor) rotasUnidades() {
	s.mux.HandleFunc("GET /unidades", s.unidadesLista)
	s.mux.HandleFunc("GET /unidades/novo", s.unidadesNova)
	s.mux.HandleFunc("GET /unidades/editar/{id}", s.unidadesEditar)
	s.mux.HandleFunc("POST /unidades/salvar", s.unidadesSalvar)
	s.mux.HandleFunc("POST /unidades/status/{id}", s.unidadesStatus)
	s.mux.HandleFunc("POST /unidades/excluir/{id}", s.unidadesExcluir)
}

func (s *Servidor) unidadesLista(w http.ResponseWriter, r *http.Request) {
	lista, err := s.cad.ListarUnidades(r.Context())
	if err != nil {
		s.erroInterno(w, err)
		return
	}
	s.tela(w, r, "unidades/lista.html", "Unidades de medida", "unidades", map[string]any{"Unidades": lista})
}

func (s *Servidor) unidadesForm(w http.ResponseWriter, r *http.Request, u cadastro.Unidade, emUso int64, titulo string) {
	s.tela(w, r, "unidades/form.html", titulo, "unidades", map[string]any{"Unidade": u, "EmUso": emUso})
}

func (s *Servidor) unidadesNova(w http.ResponseWriter, r *http.Request) {
	s.unidadesForm(w, r, cadastro.Unidade{Ativo: true}, 0, "Nova unidade de medida")
}

func (s *Servidor) unidadesEditar(w http.ResponseWriter, r *http.Request) {
	id, ok := idDoCaminho(r)
	if !ok {
		pedidoRuim(w)
		return
	}
	u, err := s.cad.BuscarUnidade(r.Context(), id)
	if err != nil {
		s.erroInterno(w, err)
		return
	}
	if u == nil {
		s.flash(r, "erro", "Unidade de medida não encontrada.")
		s.redirecionar(w, r, "/unidades")
		return
	}
	s.unidadesForm(w, r, u.Unidade, u.Uso, "Editar unidade de medida")
}

func (s *Servidor) unidadesSalvar(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		pedidoRuim(w)
		return
	}
	cod, _, ok := inteiroOpcional(r.PostFormValue("undCodigo"))
	if !ok {
		pedidoRuim(w)
		return
	}
	if caixaRuim(r, "undAtivo") { // no antigo este formulário não trata erro de ligação: o Spring responde 400
		pedidoRuim(w)
		return
	}
	u := cadastro.Unidade{Codigo: cod, Sigla: r.PostFormValue("undSigla"), Descricao: r.PostFormValue("undDescricao"), Ativo: caixa(r, "undAtivo", true)}
	if err := s.cad.SalvarUnidade(r.Context(), u); err != nil {
		s.flash(r, "erro", s.mensagemDeErro(err, "Não foi possível salvar a unidade."))
		if cod == 0 {
			s.redirecionar(w, r, "/unidades/novo")
		} else {
			s.redirecionar(w, r, "/unidades/editar/"+r.PostFormValue("undCodigo"))
		}
		return
	}
	s.flash(r, "sucesso", "Unidade de medida salva!")
	s.redirecionar(w, r, "/unidades")
}

func (s *Servidor) unidadesStatus(w http.ResponseWriter, r *http.Request) {
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
	if err := s.cad.AlterarStatusUnidade(r.Context(), id, ativo); err != nil {
		s.erroInterno(w, err)
		return
	}
	s.flash(r, "sucesso", "Status atualizado!")
	s.redirecionar(w, r, "/unidades")
}

func (s *Servidor) unidadesExcluir(w http.ResponseWriter, r *http.Request) {
	id, ok := idDoCaminho(r)
	if !ok {
		pedidoRuim(w)
		return
	}
	if err := s.cad.ExcluirUnidade(r.Context(), id); err != nil {
		s.flash(r, "erro", s.mensagemDeErro(err, "Não foi possível excluir a unidade."))
	} else {
		s.flash(r, "sucesso", "Unidade de medida excluída!")
	}
	s.redirecionar(w, r, "/unidades")
}

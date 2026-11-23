package web

import (
	"net/http"
	"strconv"

	"github.com/agnaldoojr/aureaconceptnew/internal/cadastro"
)

func (s *Servidor) rotasPessoas() {
	s.mux.HandleFunc("GET /pessoas", s.pessoasLista)
	s.mux.HandleFunc("GET /pessoas/nova", s.pessoasNova)
	s.mux.HandleFunc("GET /pessoas/editar/{id}", s.pessoasEditar)
	s.mux.HandleFunc("POST /pessoas/salvar", s.pessoasSalvar)
	s.mux.HandleFunc("POST /pessoas/excluir/{id}", s.pessoasExcluir)
}

// hoje é a data local (AAAA-MM-DD) no fuso configurado.
func (s *Servidor) hoje() string { return s.agora().In(s.cfg.Local).Format("2006-01-02") }

// Obs.: nas telas de Pessoas o menu lateral NÃO destaca "Pessoas" (no antigo o `activePage` era definido num bloco que fechava
// na hora e nunca chegava ao layout); a tela fica igual à do antigo.
const paginaAtivaPessoas = ""

func (s *Servidor) pessoasLista(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	busca, tipo := q.Get("busca"), q.Get("tipo")
	seg, temSeg, ok := inteiroOpcional(q.Get("segmento"))
	if !ok {
		pedidoRuim(w)
		return
	}
	var segPtr *int64
	if temSeg {
		segPtr = &seg
	}
	pessoas, err := s.cad.ListarPessoas(r.Context(), busca, tipo, segPtr)
	if err != nil {
		s.erroInterno(w, err)
		return
	}
	segmentos, err := s.cad.SegmentosAtivos(r.Context())
	if err != nil {
		s.erroInterno(w, err)
		return
	}
	s.tela(w, r, "pessoas/lista.html", "Pessoas", paginaAtivaPessoas, map[string]any{
		"Pessoas": pessoas, "Busca": busca, "Tipo": tipo, "SegmentoSel": seg, "Segmentos": segmentos,
	})
}

func (s *Servidor) pessoasForm(w http.ResponseWriter, r *http.Request, titulo string, p *cadastro.Pessoa, marcados []int64, erros map[string]string) {
	segmentos, err := s.cad.SegmentosAtivos(r.Context())
	if err != nil {
		s.erroInterno(w, err)
		return
	}
	m := map[int64]bool{}
	for _, id := range marcados {
		m[id] = true
	}
	if erros == nil {
		erros = map[string]string{}
	}
	s.tela(w, r, "pessoas/form.html", titulo, paginaAtivaPessoas, map[string]any{
		"Pessoa": p, "Segmentos": segmentos, "Marcados": m, "Erros": erros, "Hoje": s.hoje(),
	})
}

func (s *Servidor) pessoasNova(w http.ResponseWriter, r *http.Request) {
	s.pessoasForm(w, r, "Nova Pessoa", &cadastro.Pessoa{Ativo: true}, nil, nil)
}

func (s *Servidor) pessoasEditar(w http.ResponseWriter, r *http.Request) {
	id, ok := idDoCaminho(r)
	if !ok {
		pedidoRuim(w)
		return
	}
	p, err := s.cad.BuscarPessoa(r.Context(), id)
	if err != nil {
		s.erroInterno(w, err)
		return
	}
	if p == nil {
		s.flash(r, "erro", "Pessoa não encontrada.")
		s.redirecionar(w, r, "/pessoas")
		return
	}
	s.pessoasForm(w, r, "Editar Pessoa", p, p.SegmentoIDs(), nil)
}

func (s *Servidor) pessoasSalvar(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		pedidoRuim(w)
		return
	}
	cod, _, ok := inteiroOpcional(r.PostFormValue("upsCodigo"))
	if !ok {
		pedidoRuim(w)
		return
	}
	var segIDs []int64
	for _, v := range r.PostForm["segmentoIds"] {
		n, err := strconv.ParseInt(v, 10, 32)
		if err != nil {
			pedidoRuim(w)
			return
		}
		segIDs = append(segIDs, n)
	}
	p := &cadastro.Pessoa{
		Codigo: cod, Nome: r.PostFormValue("upsNome"), CPF: r.PostFormValue("upsCpf"), Telefone: r.PostFormValue("upsTelefone"),
		Cliente: caixa(r, "upsCliente", false), Fornecedor: caixa(r, "upsFornecedor", false),
		Vendedor: caixa(r, "upsVendedor", false), Funcionario: caixa(r, "upsFuncionario", false), Ativo: caixa(r, "upsAtivo", true),
		Nascimento: r.PostFormValue("upsNascimento"),
	}

	erros := cadastro.ValidarPessoa(p, s.hoje())
	if caixaRuim(r, "upsCliente", "upsFornecedor", "upsVendedor", "upsFuncionario", "upsAtivo") {
		erros = append(erros, cadastro.ErroCampo{Campo: "upsAtivo"}) // erro de ligação: volta ao formulário sem texto
	}
	mensagens := map[string]string{}
	if !cadastro.ValidarNascimento(p.Nascimento) {
		// o antigo mostraria o texto técnico do Spring ("Failed to convert…"); aqui uma frase clara (D-18)
		erros = append(erros, cadastro.ErroCampo{Campo: "upsNascimento", Mensagem: "Data de nascimento inválida"})
	}
	if len(erros) > 0 {
		for _, e := range erros {
			if e.Mensagem != "" {
				mensagens[e.Campo] = e.Mensagem
			}
		}
		s.pessoasForm(w, r, "Pessoa", p, segIDs, mensagens) // volta ao formulário com o que foi digitado
		return
	}

	if _, err := s.cad.SalvarPessoa(r.Context(), p, segIDs); err != nil {
		s.flash(r, "erro", "Erro: "+s.mensagemDeErro(err, "não foi possível salvar."))
	} else {
		s.flash(r, "sucesso", "Pessoa salva com sucesso!")
	}
	s.redirecionar(w, r, "/pessoas")
}

func (s *Servidor) pessoasExcluir(w http.ResponseWriter, r *http.Request) {
	id, ok := idDoCaminho(r)
	if !ok {
		pedidoRuim(w)
		return
	}
	if err := s.cad.ExcluirPessoa(r.Context(), id); err != nil {
		s.log.Info("pessoa não excluída", "codigo", id, "erro", err)
		s.flash(r, "erro", "Não foi possível excluir. Verifique documentos vinculados.")
	} else {
		s.flash(r, "sucesso", "Pessoa excluída!")
	}
	s.redirecionar(w, r, "/pessoas")
}

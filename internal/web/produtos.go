package web

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/shopspring/decimal"

	"github.com/agnaldoojr/aureaconceptnew/internal/cadastro"
)

func (s *Servidor) rotasProdutos() {
	s.mux.HandleFunc("GET /produtos", s.produtosLista)
	s.mux.HandleFunc("GET /produtos/novo", s.produtosNovo)
	s.mux.HandleFunc("GET /produtos/editar/{id}", s.produtosEditar)
	s.mux.HandleFunc("POST /produtos/salvar", s.produtosSalvar)
	s.mux.HandleFunc("POST /produtos/excluir/{id}", s.produtosExcluir)
	s.mux.HandleFunc("POST /produtos/status/{id}", s.produtosStatus)
}

// As telas de Produtos também não destacam o item do menu (mesma causa de Pessoas; ver paginaAtivaPessoas).
const paginaAtivaProdutos = ""

// ───────────────────────────── lista ─────────────────────────────

type produtoItem struct {
	Codigo      int64
	Nome        string
	Selo        string // "Serviço · 45 min · comissão 40,00%"; vazio se não for serviço
	Complemento string
	Und         string
	Preco       string
	Ativo       bool
}

func (s *Servidor) produtosLista(w http.ResponseWriter, r *http.Request) {
	busca, tipo := r.URL.Query().Get("busca"), r.URL.Query().Get("tipo")
	lista, err := s.cad.ListarProdutos(r.Context(), busca, tipo)
	if err != nil {
		s.erroInterno(w, err)
		return
	}
	itens := make([]produtoItem, 0, len(lista))
	for _, p := range lista {
		it := produtoItem{Codigo: p.Codigo, Nome: p.Nome, Complemento: p.Complemento, Und: p.Und, Preco: moeda(p.PrecoCent), Ativo: p.Ativo}
		if p.Servico {
			it.Selo = "Serviço"
			if p.DuracaoMin != nil {
				it.Selo += fmt.Sprintf(" · %d min", *p.DuracaoMin)
			}
			if p.ComissaoCent != nil {
				it.Selo += " · comissão " + decimalBR(*p.ComissaoCent, 2) + "%"
			}
		}
		itens = append(itens, it)
	}
	s.tela(w, r, "produtos/lista.html", "Produtos", paginaAtivaProdutos, map[string]any{"Produtos": itens, "Busca": busca, "Tipo": tipo})
}

// ───────────────────────────── formulário ─────────────────────────────

// produtoView é o produto como o formulário o mostra: textos prontos (o que veio do banco ou o que foi digitado).
type produtoView struct {
	Codigo, Nome, Complemento, Und, Preco, Comissao, Duracao string
	Ativo, Servico                                           bool
	CatVenda, CatCompra, Segmento                            int64 // 0 = nenhum
	UndVazio                                                 bool  // a unidade veio do formulário em branco ("" e não ausente): o antigo marca "Selecione..."
}

type linhaView struct{ Sigla, Fator, Preco string }

func zeroSeNil(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}

func viewDoProduto(p *cadastro.Produto) produtoView {
	v := produtoView{
		Codigo: strconv.FormatInt(p.Codigo, 10), Nome: p.Nome, Complemento: p.Complemento, Und: p.Und, Preco: escala(p.PrecoCent, 2),
		Ativo: p.Ativo, Servico: p.Servico, CatVenda: zeroSeNil(p.CatVenda), CatCompra: zeroSeNil(p.CatCompra), Segmento: zeroSeNil(p.SegmentoID),
	}
	if p.ComissaoCent != nil {
		v.Comissao = escala(*p.ComissaoCent, 2)
	}
	if p.DuracaoMin != nil {
		v.Duracao = strconv.FormatInt(*p.DuracaoMin, 10)
	}
	return v
}

func (s *Servidor) produtosForm(w http.ResponseWriter, r *http.Request, titulo string, v produtoView, linhas []linhaView, erros map[string]string, erroDireto string) {
	ctx := r.Context()
	extras := make([]string, 0, len(linhas))
	for _, l := range linhas {
		extras = append(extras, l.Sigla)
	}
	unidades, err := s.cad.UnidadesParaFormulario(ctx, v.Und, extras)
	if err != nil {
		s.erroInterno(w, err)
		return
	}
	receita, err := s.cad.CategoriasAtivas(ctx, "RECEITA")
	if err != nil {
		s.erroInterno(w, err)
		return
	}
	despesa, err := s.cad.CategoriasAtivas(ctx, "DESPESA")
	if err != nil {
		s.erroInterno(w, err)
		return
	}
	segmentos, err := s.cad.SegmentosAtivos(ctx)
	if err != nil {
		s.erroInterno(w, err)
		return
	}
	if erros == nil {
		erros = map[string]string{}
	}
	s.telaComErro(w, r, "produtos/form.html", titulo, paginaAtivaProdutos, map[string]any{
		"Produto": v, "Linhas": linhas, "Unidades": unidades, "CatReceita": receita, "CatDespesa": despesa, "Segmentos": segmentos, "Erros": erros,
	}, erroDireto)
}

func (s *Servidor) produtosNovo(w http.ResponseWriter, r *http.Request) {
	s.produtosForm(w, r, "Novo Produto", produtoView{Ativo: true}, nil, nil, "")
}

func (s *Servidor) produtosEditar(w http.ResponseWriter, r *http.Request) {
	id, ok := idDoCaminho(r)
	if !ok {
		pedidoRuim(w)
		return
	}
	p, err := s.cad.BuscarProduto(r.Context(), id)
	if err != nil {
		s.erroInterno(w, err)
		return
	}
	if p == nil {
		s.flash(r, "erro", "Produto não encontrado.")
		s.redirecionar(w, r, "/produtos")
		return
	}
	extras, err := s.cad.UnidadesDoProduto(r.Context(), id)
	if err != nil {
		s.erroInterno(w, err)
		return
	}
	linhas := make([]linhaView, 0, len(extras))
	for _, u := range extras {
		l := linhaView{Sigla: u.Sigla, Fator: escala(u.FatorMicro, 6)}
		if u.PrecoCent != nil {
			l.Preco = escala(*u.PrecoCent, 2)
		}
		linhas = append(linhas, l)
	}
	s.produtosForm(w, r, "Editar Produto", viewDoProduto(p), linhas, nil, "")
}

// ───────────────────────────── salvar ─────────────────────────────

// decimalOpcional lê um número opcional do formulário (vazio = ausente). ok=false se não for número.
func decimalOpcional(texto string) (d *decimal.Decimal, ok bool) {
	t := strings.TrimSpace(texto)
	if t == "" {
		return nil, true
	}
	v, err := decimal.NewFromString(t)
	if err != nil {
		return nil, false
	}
	return &v, true
}

// textoDigitado reexibe o que foi digitado como o Java reexibiria um BigDecimal; se não for número, o texto cru.
func textoDigitado(t string) string {
	if v, ok := textoBigDecimal(t); ok {
		return v
	}
	return strings.TrimSpace(t)
}

func centavos(d decimal.Decimal) int64 { return d.Shift(2).Round(0).IntPart() }

func (s *Servidor) produtosSalvar(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		pedidoRuim(w)
		return
	}
	f := r.PostForm
	cod, _, ok := inteiroOpcional(r.PostFormValue("uprCodigo"))
	if !ok {
		pedidoRuim(w)
		return
	}
	segmento, temSeg, ok := inteiroOpcional(r.PostFormValue("segmentoId"))
	if !ok {
		pedidoRuim(w)
		return
	}

	// colunas das unidades extras (o antigo as junta pela posição)
	siglas, fatores, precos := f["unSiglas"], f["unFatores"], f["unPrecos"]
	n := max(len(siglas), len(fatores), len(precos))
	item := func(l []string, i int) string {
		if i < len(l) {
			return l[i]
		}
		return ""
	}
	var linhas []cadastro.LinhaUnidade
	var paraTela []linhaView
	for i := 0; i < n; i++ {
		fat, ok1 := decimalOpcional(item(fatores, i))
		pre, ok2 := decimalOpcional(item(precos, i))
		if !ok1 || !ok2 { // o Spring responde 400 quando um número da lista não converte
			pedidoRuim(w)
			return
		}
		l := cadastro.LinhaUnidade{Sigla: item(siglas, i), Fator: fat, Preco: pre}
		linhas = append(linhas, l)
		if !(strings.TrimSpace(l.Sigla) == "" && fat == nil && pre == nil) {
			v := linhaView{Sigla: strings.TrimSpace(l.Sigla)}
			if fat != nil {
				v.Fator = textoDigitado(item(fatores, i))
			}
			if pre != nil {
				v.Preco = textoDigitado(item(precos, i))
			}
			paraTela = append(paraTela, v)
		}
	}

	// o produto digitado
	p := &cadastro.Produto{
		Codigo: cod, Nome: r.PostFormValue("uprNome"), Complemento: r.PostFormValue("uprComplemento"), Und: r.PostFormValue("uprUnd"),
		Ativo: caixa(r, "uprAtivo", true), Servico: caixa(r, "uprServico", false),
	}
	v := produtoView{
		Nome: p.Nome, Complemento: p.Complemento, Und: p.Und, Ativo: p.Ativo, Servico: p.Servico,
		Preco: textoDigitado(r.PostFormValue("uprPreco")), Comissao: textoDigitado(r.PostFormValue("uprComissaoPerc")),
		Duracao: strings.TrimSpace(r.PostFormValue("uprDuracaoMin")),
	}
	if _, veio := f["uprUnd"]; veio && p.Und == "" {
		v.UndVazio = true
	}
	if cod != 0 {
		v.Codigo = strconv.FormatInt(cod, 10)
	}
	var erros []cadastro.ErroCampo
	if preco, ok := decimalOpcional(r.PostFormValue("uprPreco")); !ok {
		erros = append(erros, cadastro.ErroCampo{Campo: "uprPreco", Mensagem: "Valor inválido"})
	} else if preco != nil {
		p.PrecoInformado = true
		switch {
		case preco.LessThan(decimal.New(1, -2)): // @DecimalMin("0.01") vale sobre o número digitado, antes de arredondar
			p.PrecoCent = 0
		case preco.GreaterThan(decimal.New(1, 15)):
			erros = append(erros, cadastro.ErroCampo{Campo: "uprPreco", Mensagem: "Valor inválido"})
		default:
			p.PrecoCent = centavos(*preco)
		}
	}
	comissao, ok := decimalOpcional(r.PostFormValue("uprComissaoPerc"))
	if !ok {
		erros = append(erros, cadastro.ErroCampo{Campo: "uprComissaoPerc", Mensagem: "Valor inválido"})
	}
	var duracao *int64
	if t := strings.TrimSpace(r.PostFormValue("uprDuracaoMin")); t != "" {
		d, err := strconv.ParseInt(t, 10, 32)
		if err != nil {
			erros = append(erros, cadastro.ErroCampo{Campo: "uprDuracaoMin", Mensagem: "Valor inválido"})
		} else {
			duracao = &d
		}
	}
	for _, c := range []struct {
		campo string
		dest  **int64
		view  *int64
	}{{"uprCatVenda", &p.CatVenda, &v.CatVenda}, {"uprCatCompra", &p.CatCompra, &v.CatCompra}} {
		n, tem, ok := inteiroOpcional(r.PostFormValue(c.campo))
		if !ok {
			erros = append(erros, cadastro.ErroCampo{Campo: c.campo}) // sem texto na tela, como no antigo
		} else if tem {
			*c.dest = &n
			*c.view = n
		}
	}
	if comissao != nil {
		c := centavos(*comissao)
		p.ComissaoCent = &c
	}
	p.DuracaoMin = duracao
	erros = append(erros, cadastro.ValidarProduto(p, comissao, duracao)...)
	if caixaRuim(r, "uprAtivo", "uprServico") {
		erros = append(erros, cadastro.ErroCampo{Campo: "uprAtivo"}) // erro de ligação: volta ao formulário sem texto
	}

	titulo := "Editar Produto"
	if cod == 0 {
		titulo = "Novo Produto"
	}
	if len(erros) > 0 {
		msgs := map[string]string{}
		for _, e := range erros {
			if e.Mensagem != "" {
				msgs[e.Campo] = e.Mensagem
			}
		}
		// no antigo o segmento só entra no objeto depois da validação: ao voltar daqui ele não vem selecionado
		s.produtosForm(w, r, titulo, v, paraTela, msgs, "")
		return
	}

	ctx := r.Context()
	var segPtr *int64
	if temSeg {
		segPtr = &segmento
	}
	if err := s.cad.ResolverSegmento(ctx, segPtr); err != nil {
		var re cadastro.ErroRegra
		if errors.As(err, &re) {
			s.produtosForm(w, r, titulo, v, paraTela, nil, string(re))
			return
		}
		s.erroInterno(w, err)
		return
	}
	p.SegmentoID = segPtr
	v.Segmento = zeroSeNil(segPtr)

	if idGravado, err := s.cad.SalvarProduto(ctx, p, linhas); err != nil {
		var re cadastro.ErroRegra
		if errors.As(err, &re) {
			// regra de negócio: volta ao formulário com os dados digitados (o segmento já estava escolhido)
			// o que o antigo já tinha alterado no objeto antes de falhar: a unidade com a grafia do cadastro e, em produto
			// (que não é serviço), a comissão e a duração zeradas
			v.Und = p.Und
			if cod == 0 && idGravado != 0 {
				// produto novo que falhou depois de gravado: o antigo já tinha o código e chama o formulário de "Editar Produto"
				titulo, v.Codigo = "Editar Produto", strconv.FormatInt(idGravado, 10)
			}
			if p.ComissaoCent == nil {
				v.Comissao = ""
			}
			if p.DuracaoMin == nil {
				v.Duracao = ""
			}
			s.produtosForm(w, r, titulo, v, paraTela, nil, string(re))
			return
		}
		s.log.Error("salvar produto", "erro", err)
		s.flash(r, "erro", "Erro ao salvar produto: não foi possível gravar.")
		s.redirecionar(w, r, "/produtos")
		return
	}
	s.flash(r, "sucesso", "Produto salvo com sucesso!")
	s.redirecionar(w, r, "/produtos")
}

// ───────────────────────────── excluir e status ─────────────────────────────

func (s *Servidor) produtosExcluir(w http.ResponseWriter, r *http.Request) {
	id, ok := idDoCaminho(r)
	if !ok {
		pedidoRuim(w)
		return
	}
	if err := s.cad.ExcluirProduto(r.Context(), id); err != nil {
		if !errors.Is(err, cadastro.ErroProdutoEmUso) {
			s.log.Error("excluir produto", "erro", err)
		}
		s.flash(r, "erro", "Não foi possível excluir o produto. Verifique se há documentos vinculados.")
	} else {
		s.flash(r, "sucesso", "Produto excluído com sucesso!")
	}
	s.redirecionar(w, r, "/produtos")
}

func (s *Servidor) produtosStatus(w http.ResponseWriter, r *http.Request) {
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
	if err := s.cad.AlterarStatusProduto(r.Context(), id, ativo); err != nil {
		var re cadastro.ErroRegra
		if errors.As(err, &re) {
			s.flash(r, "erro", string(re))
			s.redirecionar(w, r, "/produtos/editar/"+strconv.FormatInt(id, 10))
			return
		}
		s.erroInterno(w, err)
		return
	}
	s.flash(r, "sucesso", "Status do produto atualizado!")
	s.redirecionar(w, r, "/produtos")
}

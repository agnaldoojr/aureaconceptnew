package web

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/shopspring/decimal"

	"github.com/agnaldoojr/aureaconceptnew/internal/cadastro"
	"github.com/agnaldoojr/aureaconceptnew/internal/documentos"
	"github.com/agnaldoojr/aureaconceptnew/internal/dominio"
	"github.com/agnaldoojr/aureaconceptnew/internal/financeiro"
	"github.com/agnaldoojr/aureaconceptnew/internal/msg"
	"github.com/agnaldoojr/aureaconceptnew/internal/permissoes"
)

const msgSoPropriasVendas = "Você só pode ver e alterar os documentos em que é o vendedor."

func (s *Servidor) rotasDocumentos() {
	s.mux.HandleFunc("GET /documentos", s.documentosLista)
	s.mux.HandleFunc("GET /documentos/novo", s.documentosNovo)
	s.mux.HandleFunc("GET /documentos/editar/{id}", s.documentosEditar)
	s.mux.HandleFunc("GET /documentos/visualizar/{id}", s.documentosVisualizar)
	s.mux.HandleFunc("POST /documentos/salvar", s.documentosSalvar)
	s.mux.HandleFunc("POST /documentos/excluir/{id}", s.documentosExcluir)
}

// As telas de Documentos não destacam o item do menu no antigo (mesma causa de Pessoas; ver paginaAtivaPessoas).
const paginaAtivaDocumentos = ""

// ───────────────────────────── escopo de vendas (EscopoVendas do antigo) ─────────────────────────────

// veTodasAsVendas: com VENDAS_TODOS (Consulta) — e o admin — vê as vendas de todos os vendedores.
func veTodasAsVendas(u *Usuario) bool { return u.Auth.Pode("VENDAS_TODOS", permissoes.Consulta) }

// vendedorRestrito é nulo sem restrição; senão o vendedor do usuário (ou -1 se ele não tem vendedor vinculado: não casa com ninguém).
func vendedorRestrito(u *Usuario) *int64 {
	if veTodasAsVendas(u) {
		return nil
	}
	if u.VendedorID != nil {
		return u.VendedorID
	}
	nenhum := int64(-1)
	return &nenhum
}

// podeVerDocumento: sem restrição vê qualquer um; com restrição, só os documentos do próprio vendedor.
func podeVerDocumento(u *Usuario, d *documentos.Documento) bool {
	v := vendedorRestrito(u)
	if v == nil {
		return true
	}
	return d != nil && d.VendedorID != nil && *d.VendedorID == *v
}

// ───────────────────────────── lista ─────────────────────────────

type docLista struct {
	Codigo                       int64
	Tipo, Data, Pessoa, Vendedor string
	Usuario, Valor               string
	Vermelho                     bool // venda e saída; compra e entrada ficam verdes
}

func dataBRDe(iso string) string {
	if len(iso) < 10 {
		return ""
	}
	return iso[8:10] + "/" + iso[5:7] + "/" + iso[0:4]
}

func vendedorOuTraco(nome string, tem bool) string {
	if tem {
		return nome
	}
	return "—"
}

// dataOpcional lê uma data AAAA-MM-DD opcional (vazio = ausente). ok=false se vier algo que não é data.
func dataOpcional(v string) (string, bool) {
	if v == "" {
		return "", true
	}
	if _, err := time.Parse("2006-01-02", v); err != nil {
		return "", false
	}
	return v, true
}

func (s *Servidor) documentosLista(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var tipo *string
	if q.Has("tipo") {
		t := q.Get("tipo")
		tipo = &t
	}
	inicio, ok1 := dataOpcional(q.Get("inicio"))
	fim, ok2 := dataOpcional(q.Get("fim"))
	if !ok1 || !ok2 {
		pedidoRuim(w)
		return
	}
	u := ctxDe(r).Usuario
	docs, err := s.docs.Filtrar(r.Context(), documentos.Filtro{Tipo: tipo, Inicio: inicio, Fim: fim, Vendedor: vendedorRestrito(u)})
	if err != nil {
		s.erroInterno(w, err)
		return
	}
	linhas := make([]docLista, 0, len(docs))
	for _, d := range docs {
		linhas = append(linhas, docLista{
			Codigo: d.Codigo, Tipo: d.Tipo, Data: dataBRDe(d.Data), Pessoa: d.PessoaNome, Vendedor: vendedorOuTraco(d.VendedorNome, d.VendedorID != nil),
			Usuario: d.UsuarioNome, Valor: "R$ " + dominio.FormatarBR64(d.ValorCent, 2, 2), Vermelho: d.Tipo == "VENDA" || d.Tipo == "SAIDA",
		})
	}
	tipoTxt := ""
	if tipo != nil {
		tipoTxt = *tipo
	}
	s.tela(w, r, "documentos/lista.html", "Documentos", paginaAtivaDocumentos, map[string]any{"Documentos": linhas, "Tipo": tipoTxt, "Inicio": inicio, "Fim": fim})
}

// ───────────────────────────── visualizar ─────────────────────────────

type itemVista struct {
	N                                          int
	Produto, Profissional, Comissao, Und, Base string
	TemProf, TemComissao, TemBase              bool
	Qtd, Valor, Total                          string
}

func (s *Servidor) documentosVisualizar(w http.ResponseWriter, r *http.Request) {
	id, ok := idDoCaminho(r)
	if !ok {
		pedidoRuim(w)
		return
	}
	d, err := s.docs.Buscar(r.Context(), id)
	if err != nil {
		s.erroInterno(w, err)
		return
	}
	if d == nil || !podeVerDocumento(ctxDe(r).Usuario, d) {
		s.flash(r, "erro", "Documento não encontrado.")
		s.redirecionar(w, r, "/documentos")
		return
	}
	var itens []itemVista
	for i, it := range d.Itens {
		v := itemVista{N: i + 1, Produto: it.ProdutoNome, Und: it.Sigla, Qtd: dominio.FormatarBR64(it.QtdMil, 3, 3),
			Valor: "R$ " + dominio.FormatarBR64(it.ValorCent, 2, 2), Total: "R$ " + dominio.FormatarBR64(it.TotalCent, 2, 2)}
		if it.ProfissionalID != nil {
			v.TemProf, v.Profissional = true, it.ProfissionalNome
			if it.ComissaoPercCent != nil {
				v.TemComissao = true
				v.Comissao = "· comissão " + dominio.FormatarBR64(*it.ComissaoPercCent, 2, 2) + "% (R$ " + dominio.FormatarBR64(it.ComissaoCent(), 2, 2) + ")"
			}
		}
		if it.FatorMicro != 1_000_000 {
			v.TemBase = true
			v.Base = "= " + dominio.FormatarBR64(mil(it.QtdBase()), 3, 3) + " " + it.ProdutoUnd + " no estoque"
		}
		itens = append(itens, v)
	}
	s.tela(w, r, "documentos/visualizar.html", "Documento #"+strconv.FormatInt(id, 10), paginaAtivaDocumentos, map[string]any{
		"Doc": d, "Data": dataBRDe(d.Data), "Vermelho": d.Tipo == "VENDA" || d.Tipo == "SAIDA", "Total": "R$ " + dominio.FormatarBR64(d.ValorCent, 2, 2),
		"TemVendedor": d.VendedorID != nil, "Itens": itens,
	})
}

func mil(d decimal.Decimal) int64 { return d.Shift(3).Round(0).IntPart() }

// ───────────────────────────── formulário ─────────────────────────────

type docView struct {
	Codigo, Tipo, Data, Classificacao   string
	PessoaSel, SegmentoSel, VendedorSel int64
}

type itemForm struct {
	ProdutoSel int64
	Sigla      string
	Qtd, Valor string
	ProfSel    int64
	Total      string
}

type finForm struct {
	Existe         bool
	FpgSel         int64
	Parcelas       string
	PrimeiroVenc   string
	CatSel         int64
	TemBaixaManual bool
}

func (s *Servidor) documentosForm(w http.ResponseWriter, r *http.Request, titulo string, d docView, itens []itemForm, fin finForm) {
	ctx := r.Context()
	u := ctxDe(r).Usuario
	pessoas, err := s.cad.PessoasAtivas(ctx)
	if err == nil {
		var produtos []cadastro.ProdutoOpcao
		if produtos, err = s.cad.ProdutosAtivos(ctx); err == nil {
			var receber, pagar []cadastro.FormaOpcao
			if receber, err = s.cad.FormasAtivas(ctx, "RECEBER"); err == nil {
				if pagar, err = s.cad.FormasAtivas(ctx, "PAGAR"); err == nil {
					var cats []cadastro.CategoriaComTipo
					if cats, err = s.cad.CategoriasAtivasTodas(ctx); err == nil {
						var vend []cadastro.VendedorOpcao
						if vend, err = s.cad.VendedoresAtivos(ctx); err == nil {
							var segs []cadastro.Segmento
							if segs, err = s.cad.SegmentosAtivos(ctx); err == nil {
								s.tela(w, r, "documentos/form.html", titulo, paginaAtivaDocumentos, map[string]any{
									"Doc": d, "Itens": itens, "Fin": fin, "Pessoas": pessoas, "Produtos": produtos,
									"Formas": append(receber, pagar...), "Categorias": cats, "Vendedores": vend, "Segmentos": segs,
									"VendedorTravado": !veTodasAsVendas(u), "Hoje": s.hoje(),
								})
								return
							}
						}
					}
				}
			}
		}
	}
	s.erroInterno(w, err)
}

func (s *Servidor) finFormDe(r *http.Request, docID int64) (finForm, error) {
	res, err := financeiro.ResumoDoDocumento(r.Context(), s.banco.R, docID)
	if err != nil || res == nil {
		return finForm{Parcelas: "1"}, err
	}
	f := finForm{Existe: true, Parcelas: strconv.Itoa(res.Parcelas), PrimeiroVenc: res.PrimeiroVenc, CatSel: res.CatCodigo, TemBaixaManual: res.TemBaixaManual}
	if res.FpgCodigo != nil {
		f.FpgSel = *res.FpgCodigo
	}
	return f, nil
}

func (s *Servidor) documentosNovo(w http.ResponseWriter, r *http.Request) {
	u := ctxDe(r).Usuario
	d := docView{Data: s.hoje(), VendedorSel: zeroSeNil(u.VendedorID)}
	// (a venda vinda da agenda, com ?agendamentoId=, entra junto com o módulo da Agenda)
	s.documentosForm(w, r, "Novo Documento", d, nil, finForm{Parcelas: "1"})
}

func (s *Servidor) documentosEditar(w http.ResponseWriter, r *http.Request) {
	id, ok := idDoCaminho(r)
	if !ok {
		pedidoRuim(w)
		return
	}
	doc, err := s.docs.Buscar(r.Context(), id)
	if err != nil {
		s.erroInterno(w, err)
		return
	}
	if doc == nil || !podeVerDocumento(ctxDe(r).Usuario, doc) {
		s.flash(r, "erro", "Documento não encontrado.")
		s.redirecionar(w, r, "/documentos")
		return
	}
	fin, err := s.finFormDe(r, id)
	if err != nil {
		s.erroInterno(w, err)
		return
	}
	v := docView{Codigo: strconv.FormatInt(id, 10), Tipo: doc.Tipo, Data: doc.Data, Classificacao: doc.Classificacao, PessoaSel: doc.PessoaID, SegmentoSel: zeroSeNil(doc.SegmentoID), VendedorSel: zeroSeNil(doc.VendedorID)}
	var itens []itemForm
	for _, it := range doc.Itens {
		itens = append(itens, itemForm{ProdutoSel: it.ProdutoID, Sigla: it.Sigla, Qtd: dominio.Escala(it.QtdMil, 3), Valor: dominio.Escala(it.ValorCent, 2),
			ProfSel: zeroSeNil(it.ProfissionalID), Total: "R$ " + dominio.FormatarBR64(it.TotalCent, 2, 2)})
	}
	s.documentosForm(w, r, "Editar Documento #"+strconv.FormatInt(id, 10), v, itens, fin)
}

// ───────────────────────────── salvar ─────────────────────────────

func decimalLista(valores []string) ([]*decimal.Decimal, bool) {
	out := make([]*decimal.Decimal, len(valores))
	for i, v := range valores {
		d, ok := decimalOpcional(v)
		if !ok {
			return nil, false
		}
		out[i] = d
	}
	return out, true
}

func (s *Servidor) documentosSalvar(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		pedidoRuim(w)
		return
	}
	f := r.PostForm
	ctx := r.Context()
	u := ctxDe(r).Usuario

	cod, _, ok := inteiroOpcional(r.PostFormValue("doctoCodigo"))
	if !ok {
		pedidoRuim(w)
		return
	}
	data, ok := dataOpcional(r.PostFormValue("doctoData"))
	if !ok {
		pedidoRuim(w)
		return
	}
	pessoa, _, ok := inteiroOpcional(r.PostFormValue("pessoa.upsCodigo"))
	if !ok {
		pedidoRuim(w)
		return
	}
	// listas obrigatórias: sem elas o Spring responde 400
	for _, k := range []string{"produtoIds", "quantidades", "valores"} {
		if _, tem := f[k]; !tem {
			pedidoRuim(w)
			return
		}
	}
	prodIDs := make([]*int64, len(f["produtoIds"]))
	for i, v := range f["produtoIds"] {
		n, tem, ok := inteiroOpcional(v)
		if !ok {
			pedidoRuim(w)
			return
		}
		if tem {
			prodIDs[i] = &n
		}
	}
	qtds, ok1 := decimalLista(f["quantidades"])
	vals, ok2 := decimalLista(f["valores"])
	if !ok1 || !ok2 {
		pedidoRuim(w)
		return
	}
	profIDs := make([]*int64, len(f["profissionalIds"]))
	for i, v := range f["profissionalIds"] {
		n, tem, ok := inteiroOpcional(v)
		if !ok {
			pedidoRuim(w)
			return
		}
		if tem {
			profIDs[i] = &n
		}
	}
	var fpg, parc, cat, vendedorID, segID *int64
	for _, c := range []struct {
		campo string
		dest  **int64
	}{{"fpgCodigo", &fpg}, {"parcelas", &parc}, {"catCodigo", &cat}, {"vendedorId", &vendedorID}, {"segmentoId", &segID}} {
		n, tem, ok := inteiroOpcional(r.PostFormValue(c.campo))
		if !ok {
			pedidoRuim(w)
			return
		}
		if tem {
			v := n
			*c.dest = &v
		}
	}
	primeiroVenc, ok := dataOpcional(r.PostFormValue("primeiroVencimento"))
	if !ok {
		pedidoRuim(w)
		return
	}

	novo := cod == 0
	falha := func(m string) {
		s.flash(r, "erro", m)
		if novo {
			s.redirecionar(w, r, "/documentos/novo")
		} else {
			s.redirecionar(w, r, "/documentos/editar/"+strconv.FormatInt(cod, 10))
		}
	}
	erroTexto := func(err error) string {
		var re msg.Erro
		if errors.As(err, &re) {
			return string(re)
		}
		s.log.Error("salvar documento", "erro", err)
		return "Não foi possível gravar o documento."
	}

	// segmento escolhido na tela; em branco, o serviço usa o do produto de maior valor
	if err := s.cad.ResolverSegmento(ctx, segID); err != nil {
		falha(erroTexto(err))
		return
	}
	veTodas := veTodasAsVendas(u)
	if !novo && !veTodas { // quem só vê as próprias vendas não altera o documento de outro vendedor
		atual, err := s.docs.Buscar(ctx, cod)
		if err != nil {
			s.erroInterno(w, err)
			return
		}
		if !podeVerDocumento(u, atual) {
			falha(msgSoPropriasVendas)
			return
		}
	}
	var vendedor *int64
	if veTodas {
		// vendedor escolhido na tela (opcional): precisa ser uma pessoa cadastrada como vendedor
		if vendedorID != nil {
			ehVend, err := s.cad.EhVendedor(ctx, *vendedorID)
			if err != nil {
				s.erroInterno(w, err)
				return
			}
			if !ehVend {
				falha("O vendedor escolhido não é válido.")
				return
			}
			vendedor = vendedorID
		}
	} else { // só as próprias vendas: o documento sai sempre no nome do vendedor do usuário
		if u.VendedorID == nil {
			falha("Seu usuário não tem vendedor vinculado. Peça ao administrador para vincular um vendedor ao seu usuário.")
			return
		}
		vendedor = u.VendedorID
	}

	e := documentos.Entrada{Codigo: cod, Tipo: r.PostFormValue("doctoTipo"), Data: data, VendedorID: vendedor, Classificacao: r.PostFormValue("doctoClassificacao"), SegmentoID: segID}
	if pessoa != 0 {
		e.PessoaID = pessoa
	}
	if novo {
		e.UsuarioID = u.ID
	}
	for i, pid := range prodIDs {
		if pid == nil {
			continue
		}
		prod, err := s.docs.BuscarProduto(ctx, *pid)
		if err != nil {
			s.erroInterno(w, err)
			return
		}
		if prod == nil { // produto que não existe: o antigo ignora o item
			continue
		}
		if i >= len(qtds) || i >= len(vals) || qtds[i] == nil || vals[i] == nil {
			pedidoRuim(w)
			return
		}
		sigla := ""
		if us := f["unidades"]; i < len(us) {
			sigla = us[i]
		}
		canonica, fator, err := s.docs.ResolverUnidade(ctx, *prod, sigla)
		if err != nil {
			falha(erroTexto(err))
			return
		}
		it := documentos.ItemEntrada{Produto: *prod, Qtd: *qtds[i], Valor: *vals[i], Sigla: canonica, Fator: fator}
		if prod.Servico {
			if !veTodas { // quem só vê as próprias vendas é sempre o profissional dos seus serviços
				it.ProfissionalID = vendedor
			} else if i < len(profIDs) && profIDs[i] != nil {
				ehVend, err := s.cad.EhVendedor(ctx, *profIDs[i])
				if err != nil {
					s.erroInterno(w, err)
					return
				}
				if !ehVend {
					falha("O profissional escolhido não é válido.")
					return
				}
				it.ProfissionalID = profIDs[i]
			}
		}
		e.Itens = append(e.Itens, it)
	}

	params := &financeiro.ParamsDocumento{FpgCodigo: fpg, Parcelas: parc, PrimeiroVenc: primeiroVenc, CatCodigo: cat}
	if novo {
		if _, err := s.docs.Salvar(ctx, e, params); err != nil {
			falha(erroTexto(err))
			return
		}
		s.flash(r, "sucesso", "Documento lançado com sucesso!")
	} else {
		if err := s.docs.Atualizar(ctx, e, params, u.ID); err != nil {
			falha(erroTexto(err))
			return
		}
		s.flash(r, "sucesso", "Documento atualizado com sucesso!")
	}
	s.redirecionar(w, r, "/documentos")
}

func (s *Servidor) documentosExcluir(w http.ResponseWriter, r *http.Request) {
	id, ok := idDoCaminho(r)
	if !ok {
		pedidoRuim(w)
		return
	}
	ctx := r.Context()
	erro := func(m string) {
		s.flash(r, "erro", "Erro ao excluir: "+m)
		s.redirecionar(w, r, "/documentos")
	}
	alvo, err := s.docs.Buscar(ctx, id)
	if err != nil {
		s.erroInterno(w, err)
		return
	}
	if alvo != nil && !podeVerDocumento(ctxDe(r).Usuario, alvo) {
		erro(msgSoPropriasVendas)
		return
	}
	if err := s.docs.Excluir(ctx, id); err != nil {
		var re msg.Erro
		if errors.As(err, &re) {
			erro(string(re))
		} else {
			s.log.Error("excluir documento", "erro", err)
			erro("não foi possível excluir.")
		}
		return
	}
	s.flash(r, "sucesso", "Documento excluído e estoque revertido!")
	s.redirecionar(w, r, "/documentos")
}

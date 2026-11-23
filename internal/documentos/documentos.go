// Package documentos tem as regras dos documentos (COMPRA, VENDA, ENTRADA, SAIDA) do sistema antigo (DocumentoService): validações,
// movimento de estoque, segmento e geração/refazimento dos títulos, tudo NUMA transação (RN-DOC-01 a RN-DOC-08).
// Quantidade em milésimos, valores em centavos, fator em milionésimos; as contas usam decimais exatos como o BigDecimal do Java
// (o total do item e do documento só são arredondados ao gravar, ver DUVIDAS D-05).
package documentos

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/agnaldoojr/aureaconceptnew/internal/db"
	"github.com/agnaldoojr/aureaconceptnew/internal/dominio"
	"github.com/agnaldoojr/aureaconceptnew/internal/estoque"
	"github.com/agnaldoojr/aureaconceptnew/internal/financeiro"
	"github.com/agnaldoojr/aureaconceptnew/internal/msg"
)

// Servico acessa os documentos. Agora é o relógio (para docto_criado_em, no fuso da loja); pode ser nil (time.Now).
type Servico struct {
	B     *db.Banco
	Agora func() time.Time
}

func (s *Servico) agora() time.Time {
	if s.Agora != nil {
		return s.Agora()
	}
	return time.Now()
}

// Produto é o que um item precisa saber do produto escolhido.
type Produto struct {
	ID           int64
	Nome, Und    string
	Servico      bool
	ComissaoCent *int64 // centésimos de %
	SegmentoID   *int64
	CatVenda     *int64
	CatCompra    *int64
}

// ItemEntrada é um item vindo do formulário (valores digitados, sem arredondar).
type ItemEntrada struct {
	Produto        Produto
	Qtd, Valor     decimal.Decimal
	Sigla          string          // unidade escolhida já resolvida (canônica)
	Fator          decimal.Decimal // quantas unidades base cabem em 1 unidade do item
	ProfissionalID *int64
}

// Entrada é o documento vindo do formulário.
type Entrada struct {
	Codigo        int64 // 0 = novo
	Tipo          string
	Data          string // AAAA-MM-DD
	PessoaID      int64
	VendedorID    *int64
	Classificacao string
	SegmentoID    *int64 // escolhido na tela (nulo = herdar do produto de maior valor)
	Itens         []ItemEntrada
	UsuarioID     int64 // só vale em documento novo
}

// Item é um item gravado.
type Item struct {
	Codigo           int64
	ProdutoID        int64
	ProdutoNome      string
	ProdutoUnd       string // unidade base atual do produto
	ProdutoServico   bool
	Sigla            string
	QtdMil           int64
	ValorCent        int64
	TotalCent        int64
	FatorMicro       int64
	ProfissionalID   *int64
	ProfissionalNome string
	ComissaoPercCent *int64
}

// QtdBase é a quantidade na unidade base: qtd × fator, 3 casas HALF_UP.
func (i Item) QtdBase() decimal.Decimal {
	return dominio.QtdBase(decimal.New(i.QtdMil, -3), decimal.New(i.FatorMicro, -6))
}

// ComissaoCent é a comissão do item em centavos: total × % ÷ 100, HALF_UP; zero sem percentual.
func (i Item) ComissaoCent() int64 {
	if i.ComissaoPercCent == nil || *i.ComissaoPercCent <= 0 {
		return 0
	}
	p := decimal.New(*i.ComissaoPercCent, -2)
	return dominio.Comissao(decimal.New(i.TotalCent, -2), &p).Shift(2).Round(0).IntPart()
}

// Documento é um documento gravado.
type Documento struct {
	Codigo        int64
	Tipo          string
	Data          string
	ValorCent     int64
	PessoaID      int64
	PessoaNome    string
	UsuarioID     int64
	UsuarioNome   string
	CriadoEm      string
	VendedorID    *int64
	VendedorNome  string
	Classificacao string
	SegmentoID    *int64
	Itens         []Item
}

// Financeiro diz se o documento gera título (compra e venda).
func (d *Documento) Financeiro() bool { return d.Tipo == "COMPRA" || d.Tipo == "VENDA" }

// Entrada diz se o documento soma ao estoque (compra e entrada).
func (d *Documento) EhEntrada() bool { return d.Tipo == "COMPRA" || d.Tipo == "ENTRADA" }

func tipoEntrada(t string) bool    { return t == "COMPRA" || t == "ENTRADA" }
func tipoSaida(t string) bool      { return t == "VENDA" || t == "SAIDA" }
func tipoFinanceiro(t string) bool { return t == "COMPRA" || t == "VENDA" }

// ───────────────────────────── leitura ─────────────────────────────

const colunasDoc = `d.docto_codigo, d.docto_tipo, d.docto_data, d.docto_valor, d.docto_ups_codigo, p.ups_nome, d.docto_usu_codigo, u.usu_nome, d.docto_criado_em,
	d.docto_vendedor, COALESCE(v.ups_nome,''), COALESCE(d.docto_classificacao,''), d.docto_seg_codigo`

const juncoesDoc = ` FROM Documento d JOIN Pessoas p ON p.ups_codigo = d.docto_ups_codigo JOIN Usuarios u ON u.usu_codigo = d.docto_usu_codigo LEFT JOIN Pessoas v ON v.ups_codigo = d.docto_vendedor`

func lerDoc(sc interface{ Scan(...any) error }) (*Documento, error) {
	var d Documento
	var vend, seg sql.NullInt64
	if err := sc.Scan(&d.Codigo, &d.Tipo, &d.Data, &d.ValorCent, &d.PessoaID, &d.PessoaNome, &d.UsuarioID, &d.UsuarioNome, &d.CriadoEm, &vend, &d.VendedorNome, &d.Classificacao, &seg); err != nil {
		return nil, err
	}
	if vend.Valid {
		d.VendedorID = &vend.Int64
	}
	if seg.Valid {
		d.SegmentoID = &seg.Int64
	}
	return &d, nil
}

func itensDoDocumento(ctx context.Context, x db.Executor, doc int64) ([]Item, error) {
	rows, err := x.QueryContext(ctx, `SELECT i.itemdocto_codigo, i.itemdocto_upr_codigo, pr.upr_nome, pr.upr_und, pr.upr_servico, i.itemdocto_upr_und, i.itemdocto_qtd, i.itemdocto_valor,
		i.itemdocto_total, i.itemdocto_fator, i.itemdocto_prof_codigo, COALESCE(pf.ups_nome,''), i.itemdocto_comissao_perc
		FROM Itens_Documento i JOIN Produto pr ON pr.upr_codigo = i.itemdocto_upr_codigo LEFT JOIN Pessoas pf ON pf.ups_codigo = i.itemdocto_prof_codigo
		WHERE i.itemdocto_docto_codigo = ? ORDER BY i.itemdocto_codigo`, doc)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var itens []Item
	for rows.Next() {
		var it Item
		var serv int
		var prof, com sql.NullInt64
		if err := rows.Scan(&it.Codigo, &it.ProdutoID, &it.ProdutoNome, &it.ProdutoUnd, &serv, &it.Sigla, &it.QtdMil, &it.ValorCent, &it.TotalCent, &it.FatorMicro, &prof, &it.ProfissionalNome, &com); err != nil {
			return nil, err
		}
		it.ProdutoServico = serv == 1
		if prof.Valid {
			it.ProfissionalID = &prof.Int64
		}
		if com.Valid {
			it.ComissaoPercCent = &com.Int64
		}
		itens = append(itens, it)
	}
	return itens, rows.Err()
}

// Buscar devolve o documento com os itens; (nil, nil) se não existe.
func (s *Servico) Buscar(ctx context.Context, id int64) (*Documento, error) {
	return buscar(ctx, s.B.R, id)
}

func buscar(ctx context.Context, x db.Executor, id int64) (*Documento, error) {
	d, err := lerDoc(x.QueryRowContext(ctx, `SELECT `+colunasDoc+juncoesDoc+` WHERE d.docto_codigo = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	d.Itens, err = itensDoDocumento(ctx, x, id)
	return d, err
}

// Filtro é a busca da lista de documentos. Campos vazios (nil) não filtram.
type Filtro struct {
	Tipo     *string // "" também filtra (e não casa com nada), como o `:tipo IS NULL OR tipo = :tipo` do antigo
	Inicio   string
	Fim      string
	Vendedor *int64 // só os documentos desse vendedor; -1 = nenhum
}

// Filtrar lista os documentos, do mais recente para o mais antigo.
func (s *Servico) Filtrar(ctx context.Context, f Filtro) ([]*Documento, error) {
	q := `SELECT ` + colunasDoc + juncoesDoc
	var onde []string
	var args []any
	if f.Tipo != nil {
		onde = append(onde, `d.docto_tipo = ?`)
		args = append(args, *f.Tipo)
	}
	if f.Inicio != "" {
		onde = append(onde, `d.docto_data >= ?`)
		args = append(args, f.Inicio)
	}
	if f.Fim != "" {
		onde = append(onde, `d.docto_data <= ?`)
		args = append(args, f.Fim)
	}
	if f.Vendedor != nil {
		onde = append(onde, `d.docto_vendedor = ?`)
		args = append(args, *f.Vendedor)
	}
	if len(onde) > 0 {
		q += ` WHERE ` + strings.Join(onde, ` AND `)
	}
	rows, err := s.B.R.QueryContext(ctx, q+` ORDER BY d.docto_data DESC, d.docto_codigo DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var lista []*Documento
	for rows.Next() {
		d, err := lerDoc(rows)
		if err != nil {
			return nil, err
		}
		lista = append(lista, d)
	}
	return lista, rows.Err()
}

// BuscarProduto carrega o produto para um item; (nil, nil) se não existe.
func (s *Servico) BuscarProduto(ctx context.Context, id int64) (*Produto, error) {
	var p Produto
	var serv int
	var com, seg, cv, cc sql.NullInt64
	err := s.B.R.QueryRowContext(ctx, `SELECT upr_codigo, upr_nome, upr_und, upr_servico, upr_comissao_perc, upr_seg_codigo, upr_cat_venda, upr_cat_compra FROM Produto WHERE upr_codigo = ?`, id).
		Scan(&p.ID, &p.Nome, &p.Und, &serv, &com, &seg, &cv, &cc)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	p.Servico = serv == 1
	for _, par := range []struct {
		src sql.NullInt64
		dst **int64
	}{{com, &p.ComissaoCent}, {seg, &p.SegmentoID}, {cv, &p.CatVenda}, {cc, &p.CatCompra}} {
		if par.src.Valid {
			v := par.src.Int64
			*par.dst = &v
		}
	}
	return &p, nil
}

// ResolverUnidade escolhe a unidade do item: em branco ou igual à base = base (fator 1); senão a unidade extra do produto (sem diferenciar
// maiúscula/acento). Devolve a sigla como está cadastrada e o fator.
func (s *Servico) ResolverUnidade(ctx context.Context, p Produto, sigla string) (string, decimal.Decimal, error) {
	t := strings.TrimSpace(sigla)
	if t == "" || strings.EqualFold(t, p.Und) {
		return p.Und, decimal.NewFromInt(1), nil
	}
	var cad string
	var fator int64
	err := s.B.R.QueryRowContext(ctx, `SELECT pun_sigla, pun_fator FROM Produto_Unidade WHERE pun_upr_codigo = ? AND pun_sigla = ?`, p.ID, t).Scan(&cad, &fator)
	if errors.Is(err, sql.ErrNoRows) {
		return "", decimal.Zero, msg.Novo("O produto '%s' não tem a unidade '%s'. Cadastre-a em Produtos ou escolha outra.", p.Nome, t)
	}
	if err != nil {
		return "", decimal.Zero, err
	}
	return cad, decimal.New(fator, -6), nil
}

// ───────────────────────────── regras ─────────────────────────────

var classificacoes = map[string]bool{"AJUSTE": true, "TRANSFERENCIA": true, "QUEBRA": true}

// validarFinanceiro: compra/venda exigem a forma de pagamento e zeram a classificação; entrada/saída exigem a classificação.
func validarFinanceiro(e *Entrada, p *financeiro.ParamsDocumento) error {
	if tipoFinanceiro(e.Tipo) {
		e.Classificacao = ""
		if p == nil || p.FpgCodigo == nil {
			return msg.Novo("Selecione a forma de pagamento do documento.")
		}
		return nil
	}
	c := strings.ToUpper(strings.TrimSpace(e.Classificacao))
	if !classificacoes[c] {
		nome := "saída"
		if e.Tipo == "ENTRADA" {
			nome = "entrada"
		}
		return msg.Novo("Informe a classificação da %s de estoque: ajuste, transferência ou quebra.", nome)
	}
	if c == "QUEBRA" && e.Tipo != "SAIDA" {
		return msg.Novo("Quebra só pode ser lançada em uma saída.")
	}
	e.Classificacao = c
	return nil
}

// itemPreparado é o item depois de prePersist e prepararServicos.
type itemPreparado struct {
	ItemEntrada
	total        decimal.Decimal // qtd × valor, sem arredondar
	comissao     *int64          // centésimos de %
	profissional *int64
}

func (p itemPreparado) qtdBase() decimal.Decimal   { return dominio.QtdBase(p.Qtd, p.Fator) }
func (p itemPreparado) valorBase() decimal.Decimal { return dominio.ValorBase(p.Valor, p.Fator) }

// prepararServicos aplica as regras do salão (RN-DOC-05): serviço só em compra/venda; na venda, serviço com comissão exige o
// profissional e guarda o percentual do momento; item que não é serviço vendido fica sem profissional e sem comissão.
func prepararServicos(tipo string, itens []ItemEntrada) ([]itemPreparado, error) {
	venda := tipo == "VENDA"
	out := make([]itemPreparado, 0, len(itens))
	for _, it := range itens {
		pr := itemPreparado{ItemEntrada: it, total: dominio.TotalItem(it.Qtd, it.Valor)}
		servico := it.Produto.Servico
		if servico && !tipoFinanceiro(tipo) {
			quem := "uma saída"
			if tipo == "ENTRADA" {
				quem = "uma entrada"
			}
			return nil, msg.Novo("'%s' é um serviço e não tem estoque: lance-o em uma venda, não em %s de estoque.", it.Produto.Nome, quem)
		}
		if !(servico && venda) {
			out = append(out, pr)
			continue
		}
		if c := it.Produto.ComissaoCent; c != nil && *c > 0 {
			pr.comissao = c
			if it.ProfissionalID == nil {
				return nil, msg.Novo("Informe o profissional do serviço '%s' (ele tem comissão de %s%%).", it.Produto.Nome, decimal.New(*c, -2).String())
			}
		}
		pr.profissional = it.ProfissionalID
		out = append(out, pr)
	}
	return out, nil
}

// segmentoDosItens é o segmento do item de MAIOR valor (qtd × valor) entre os produtos que têm segmento; empate: o que apareceu primeiro.
func segmentoDosItens(itens []itemPreparado) *int64 {
	var ordem []int64
	soma := map[int64]decimal.Decimal{}
	for _, it := range itens {
		s := it.Produto.SegmentoID
		if s == nil {
			continue
		}
		if _, ja := soma[*s]; !ja {
			ordem = append(ordem, *s)
		}
		soma[*s] = soma[*s].Add(it.total)
	}
	var melhor *int64
	for i := range ordem {
		if melhor == nil || soma[ordem[i]].GreaterThan(soma[*melhor]) {
			melhor = &ordem[i]
		}
	}
	return melhor
}

func totalDocumento(itens []itemPreparado) decimal.Decimal {
	t := make([]decimal.Decimal, len(itens))
	for i, it := range itens {
		t[i] = it.total
	}
	return dominio.TotalDocumento(t)
}

func somaExata(itens []itemPreparado) decimal.Decimal {
	s := decimal.Zero
	for _, it := range itens {
		s = s.Add(it.total)
	}
	return s
}

func cent(d decimal.Decimal) int64  { return d.Shift(2).Round(0).IntPart() }
func mil(d decimal.Decimal) int64   { return d.Shift(3).Round(0).IntPart() }
func micro(d decimal.Decimal) int64 { return d.Shift(6).Round(0).IntPart() }

func est(p Produto) estoque.Produto {
	return estoque.Produto{ID: p.ID, Nome: p.Nome, Servico: p.Servico}
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// aplicarEstoque movimenta o estoque de cada item no sentido do tipo do documento.
func aplicarEstoque(ctx context.Context, x db.Executor, tipo string, itens []itemPreparado) error {
	for _, it := range itens {
		var err error
		if tipoEntrada(tipo) {
			err = estoque.Entrada(ctx, x, est(it.Produto), it.qtdBase(), it.valorBase())
		} else {
			err = estoque.Saida(ctx, x, est(it.Produto), it.qtdBase())
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func paraFinanceiro(docID int64, e Entrada, itens []itemPreparado, valorCent int64, seg *int64) financeiro.Documento {
	fd := financeiro.Documento{Codigo: docID, Tipo: e.Tipo, Data: e.Data, ValorCent: valorCent, PessoaID: e.PessoaID, SegmentoID: seg}
	for _, it := range itens {
		fd.Itens = append(fd.Itens, financeiro.ItemParaCategoria{CatVenda: it.Produto.CatVenda, CatCompra: it.Produto.CatCompra, Total: it.total})
	}
	return fd
}

func inserirItens(ctx context.Context, x db.Executor, r db.Rastro, doc int64, itens []itemPreparado) error {
	for _, it := range itens {
		sigla := it.Sigla
		if strings.TrimSpace(sigla) == "" {
			sigla = it.Produto.Und
		}
		if _, err := db.Inserir(ctx, x, r, "Itens_Documento", `INSERT INTO Itens_Documento (itemdocto_docto_codigo, itemdocto_upr_codigo, itemdocto_upr_und, itemdocto_qtd, itemdocto_valor, itemdocto_total,
			itemdocto_fator, itemdocto_prof_codigo, itemdocto_comissao_perc) VALUES (?,?,?,?,?,?,?,?,?)`,
			doc, it.Produto.ID, sigla, mil(it.Qtd), cent(it.Valor), cent(it.total), micro(it.Fator), it.profissional, it.comissao); err != nil {
			return err
		}
	}
	return nil
}

func (s *Servico) criadoEm() string { return s.agora().Format("2006-01-02 15:04:05") }

// Salvar lança um documento novo (DocumentoService.salvar): valida, grava, movimenta o estoque e gera os títulos, na mesma transação.
// Se falhar depois de gravar, tudo é desfeito, mas os códigos gerados ficam queimados, como o IDENTITY do SQL Server. Devolve o código.
func (s *Servico) Salvar(ctx context.Context, e Entrada, p *financeiro.ParamsDocumento) (int64, error) {
	r := db.Rastro{}
	id, err := s.salvar(ctx, e, p, r)
	if err != nil {
		s.B.QueimarCodigos(ctx, r)
	}
	return id, err
}

func (s *Servico) salvar(ctx context.Context, e Entrada, p *financeiro.ParamsDocumento, r db.Rastro) (int64, error) {
	if len(e.Itens) == 0 {
		return 0, msg.Novo("O documento deve ter pelo menos um item.")
	}
	if err := validarFinanceiro(&e, p); err != nil {
		return 0, err
	}
	itens, err := prepararServicos(e.Tipo, e.Itens)
	if err != nil {
		return 0, err
	}
	seg := e.SegmentoID
	if seg == nil {
		seg = segmentoDosItens(itens)
	}
	total := totalDocumento(itens)

	tx, err := s.B.W.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	if tipoSaida(e.Tipo) { // saldo de cada item, um a um (a soma de itens do mesmo produto só falha mais adiante, como no antigo)
		for _, it := range itens {
			l, err := estoque.Buscar(ctx, tx, it.Produto.ID)
			if err != nil {
				return 0, err
			}
			if l != nil && l.Qtd < mil(it.qtdBase()) {
				return 0, msg.Novo("Estoque insuficiente para '%s'. Disponível: %s | Solicitado: %s", it.Produto.Nome, dominio.Escala(l.Qtd, 3), dominio.Escala(mil(it.qtdBase()), 3))
			}
		}
	}

	var classif any
	if e.Classificacao != "" {
		classif = e.Classificacao
	}
	docID, err := db.Inserir(ctx, tx, r, "Documento", `INSERT INTO Documento (docto_tipo, docto_data, docto_valor, docto_ups_codigo, docto_usu_codigo, docto_criado_em, docto_classificacao, docto_vendedor, docto_seg_codigo)
		VALUES (?,?,?,?,?,?,?,?,?)`, e.Tipo, e.Data, cent(total), e.PessoaID, e.UsuarioID, s.criadoEm(), classif, e.VendedorID, seg)
	if err != nil {
		return 0, err
	}
	if err := inserirItens(ctx, tx, r, docID, itens); err != nil {
		return docID, err
	}
	if err := aplicarEstoque(ctx, tx, e.Tipo, itens); err != nil {
		return docID, err
	}
	if err := financeiro.GerarTitulosDocumento(ctx, tx, r, paraFinanceiro(docID, e, itens, cent(total), seg), p, e.UsuarioID, somaExata(itens)); err != nil {
		return docID, err
	}
	return docID, tx.Commit()
}

// Atualizar edita um documento (DocumentoService.atualizar): refaz o financeiro, reverte o estoque do que estava gravado e aplica o novo.
func (s *Servico) Atualizar(ctx context.Context, e Entrada, p *financeiro.ParamsDocumento, usuarioID int64) error {
	r := db.Rastro{}
	err := s.atualizar(ctx, e, p, usuarioID, r)
	if err != nil {
		s.B.QueimarCodigos(ctx, r)
	}
	return err
}

func (s *Servico) atualizar(ctx context.Context, e Entrada, p *financeiro.ParamsDocumento, usuarioID int64, r db.Rastro) error {
	if e.Codigo == 0 {
		return msg.Novo("Documento sem ID — use salvar() para novos.")
	}
	if len(e.Itens) == 0 {
		return msg.Novo("O documento deve ter pelo menos um item.")
	}
	if err := validarFinanceiro(&e, p); err != nil {
		return err
	}
	tx, err := s.B.W.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	original, err := buscar(ctx, tx, e.Codigo)
	if err != nil {
		return err
	}
	if original == nil {
		return msg.Novo("Documento não encontrado.")
	}
	if err := financeiro.RemoverTitulosDocumento(ctx, tx, original.Codigo); err != nil {
		return err
	}
	if original.EhEntrada() {
		for _, it := range original.Itens {
			l, err := estoque.Buscar(ctx, tx, it.ProdutoID)
			if err != nil {
				return err
			}
			if l != nil && l.Qtd < mil(it.QtdBase()) {
				return msg.Novo("Não é possível editar: o estoque de '%s' já foi consumido (saldo: %s, precisaria reverter: %s).", it.ProdutoNome, dominio.Escala(l.Qtd, 3), dominio.Escala(mil(it.QtdBase()), 3))
			}
		}
	}
	for _, it := range original.Itens { // reverte o efeito antigo
		pr := estoque.Produto{ID: it.ProdutoID, Nome: it.ProdutoNome, Servico: it.ProdutoServico}
		if original.EhEntrada() {
			err = estoque.Saida(ctx, tx, pr, it.QtdBase())
		} else {
			err = estoque.EntradaSemAlterarCusto(ctx, tx, pr, it.QtdBase())
		}
		if err != nil {
			return err
		}
	}

	itens, err := prepararServicos(e.Tipo, e.Itens)
	if err != nil {
		return err
	}
	seg := e.SegmentoID
	if seg == nil {
		seg = segmentoDosItens(itens)
	}
	total := totalDocumento(itens)
	if tipoSaida(e.Tipo) {
		for _, it := range itens {
			l, err := estoque.Buscar(ctx, tx, it.Produto.ID)
			if err != nil {
				return err
			}
			if l != nil && l.Qtd < mil(it.qtdBase()) {
				return msg.Novo("Estoque insuficiente para '%s'.", it.Produto.Nome)
			}
		}
	}

	var classif any
	if e.Classificacao != "" {
		classif = e.Classificacao
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM Itens_Documento WHERE itemdocto_docto_codigo = ?`, original.Codigo); err != nil {
		return err
	}
	if err := inserirItens(ctx, tx, r, original.Codigo, itens); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE Documento SET docto_tipo = ?, docto_classificacao = ?, docto_vendedor = ?, docto_seg_codigo = ?, docto_data = ?, docto_ups_codigo = ?, docto_valor = ? WHERE docto_codigo = ?`,
		e.Tipo, classif, e.VendedorID, seg, e.Data, e.PessoaID, cent(total), original.Codigo); err != nil {
		return err
	}
	if err := aplicarEstoque(ctx, tx, e.Tipo, itens); err != nil {
		return err
	}
	if err := financeiro.GerarTitulosDocumento(ctx, tx, r, paraFinanceiro(original.Codigo, e, itens, cent(total), seg), p, usuarioID, somaExata(itens)); err != nil {
		return err
	}
	return tx.Commit()
}

// Excluir apaga o documento e reverte o estoque e o financeiro (DocumentoService.excluir).
func (s *Servico) Excluir(ctx context.Context, id int64) error {
	tx, err := s.B.W.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	doc, err := buscar(ctx, tx, id)
	if err != nil {
		return err
	}
	if doc == nil {
		return msg.Novo("Documento não encontrado.")
	}
	if err := financeiro.RemoverTitulosDocumento(ctx, tx, id); err != nil {
		return err
	}
	if doc.EhEntrada() {
		for _, it := range doc.Itens {
			l, err := estoque.Buscar(ctx, tx, it.ProdutoID)
			if err != nil {
				return err
			}
			if l != nil && l.Qtd < mil(it.QtdBase()) {
				return msg.Novo("Não é possível excluir: o estoque de '%s' já foi consumido (saldo atual: %s, precisaria reverter: %s).", it.ProdutoNome, dominio.Escala(l.Qtd, 3), dominio.Escala(mil(it.QtdBase()), 3))
			}
		}
	}
	for _, it := range doc.Itens {
		pr := estoque.Produto{ID: it.ProdutoID, Nome: it.ProdutoNome, Servico: it.ProdutoServico}
		if doc.EhEntrada() {
			err = estoque.Saida(ctx, tx, pr, it.QtdBase())
		} else {
			err = estoque.EntradaSemAlterarCusto(ctx, tx, pr, it.QtdBase())
		}
		if err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM Itens_Documento WHERE itemdocto_docto_codigo = ?`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM Documento WHERE docto_codigo = ?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

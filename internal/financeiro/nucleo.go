// Package financeiro tem o núcleo do financeiro que os documentos usam: gerar os títulos de uma venda (a receber) ou compra
// (a pagar), baixar automaticamente, descontar a taxa do cartão e desfazer tudo ao editar ou excluir o documento
// (FinanceiroService do sistema antigo). Valores em centavos; tudo roda na transação de quem chama.
package financeiro

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/agnaldoojr/aureaconceptnew/internal/db"
	"github.com/agnaldoojr/aureaconceptnew/internal/dominio"
	"github.com/agnaldoojr/aureaconceptnew/internal/msg"
)

// ParamsDocumento é o que a tela do documento informa sobre o financeiro.
type ParamsDocumento struct {
	FpgCodigo    *int64
	Parcelas     *int64
	PrimeiroVenc string // AAAA-MM-DD ou vazio
	CatCodigo    *int64
}

// ItemParaCategoria é o que a escolha da categoria precisa de cada item do documento.
type ItemParaCategoria struct {
	CatVenda, CatCompra *int64
	Total               decimal.Decimal // qtd × valor, sem arredondar
}

// Documento é o que a geração de títulos precisa saber do documento já gravado.
type Documento struct {
	Codigo     int64
	Tipo       string // "VENDA" ou "COMPRA"
	Data       string // AAAA-MM-DD
	ValorCent  int64
	PessoaID   int64
	SegmentoID *int64
	Itens      []ItemParaCategoria
}

type forma struct {
	codigo, tipoTitulo                      int64
	nome, natureza                          string
	ativo, permiteParcelas, baixaAutomatica bool
	maxParcelas, diasVenc, intervalo        int64
	taxaCent                                int64 // centésimos de %
	contaPadrao                             *int64
	nomeBaixa                               *string
}

func lerForma(ctx context.Context, x db.Executor, id int64) (*forma, error) {
	var f forma
	var ativo, parc, auto int
	var conta sql.NullInt64
	var nomeBaixa sql.NullString
	err := x.QueryRowContext(ctx, `SELECT f.fpg_codigo, f.fpg_nome, t.tpt_natureza, f.fpg_tpt_codigo, f.fpg_ativo, f.fpg_permite_parcelas, f.fpg_max_parcelas, f.fpg_taxa_percentual,
		f.fpg_dias_vencimento, f.fpg_intervalo_parcelas, f.fpg_baixa_automatica, f.fpg_cfn_codigo, f.fpg_nome_baixa
		FROM Forma_Pagamento f JOIN Tipo_Titulo t ON t.tpt_codigo = f.fpg_tpt_codigo WHERE f.fpg_codigo = ?`, id).
		Scan(&f.codigo, &f.nome, &f.natureza, &f.tipoTitulo, &ativo, &parc, &f.maxParcelas, &f.taxaCent, &f.diasVenc, &f.intervalo, &auto, &conta, &nomeBaixa)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	f.ativo, f.permiteParcelas, f.baixaAutomatica = ativo == 1, parc == 1, auto == 1
	if conta.Valid {
		f.contaPadrao = &conta.Int64
	}
	if nomeBaixa.Valid {
		f.nomeBaixa = &nomeBaixa.String
	}
	return &f, nil
}

const layoutData = "2006-01-02"

func somarDias(data string, dias int64) string {
	t, err := time.Parse(layoutData, data)
	if err != nil {
		return data
	}
	return t.AddDate(0, 0, int(dias)).Format(layoutData)
}

func primeiroDoMes(data string) string {
	if len(data) >= 7 {
		return data[:7] + "-01"
	}
	return data
}

func cent(v int64) decimal.Decimal { return decimal.New(v, -2) }

// GerarTitulosDocumento gera os títulos de uma venda ou compra já gravada: uma ou mais parcelas, com baixa automática quando a
// forma de pagamento pede (RN-FIN-01, RN-FIN-02, RN-FIN-04). Os códigos gerados vão para o rastro (pode ser nil).
// totalExato é a soma qtd × valor dos itens SEM arredondar: o antigo divide esse número (não o gravado) em parcelas (RN-FIN-02).
func GerarTitulosDocumento(ctx context.Context, x db.Executor, r db.Rastro, d Documento, p *ParamsDocumento, usuarioID int64, totalExato decimal.Decimal) error {
	if d.Tipo != "VENDA" && d.Tipo != "COMPRA" {
		return nil
	}
	if p == nil || p.FpgCodigo == nil {
		return msg.Novo("Selecione a forma de pagamento do documento.")
	}
	venda := d.Tipo == "VENDA"
	nat := "PAGAR"
	if venda {
		nat = "RECEBER"
	}
	f, err := lerForma(ctx, x, *p.FpgCodigo)
	if err != nil {
		return err
	}
	if f == nil {
		return msg.Novo("Forma de pagamento não encontrada.")
	}
	if !f.ativo {
		return msg.Novo("A forma de pagamento '%s' está inativa.", f.nome)
	}
	if f.natureza != nat {
		tipo := "pagamento"
		if venda {
			tipo = "recebimento"
		}
		return msg.Novo("A forma '%s' não é de %s.", f.nome, tipo)
	}
	auto := f.baixaAutomatica
	if auto && f.contaPadrao == nil {
		return msg.Novo("A forma '%s' baixa automaticamente, mas não tem conta de destino.", f.nome)
	}
	n := int64(1)
	if p.Parcelas != nil {
		n = *p.Parcelas
	}
	if !f.permiteParcelas || auto {
		n = 1
	} else if n < 1 || n > f.maxParcelas {
		return msg.Novo("A forma '%s' permite de 1 a %d parcelas.", f.nome, f.maxParcelas)
	}

	cat, err := escolherCategoria(ctx, x, d, nat, p.CatCodigo)
	if err != nil {
		return err
	}
	parcelas := dominio.Parcelas(totalExato, int(n))
	base := d.Data
	if !auto {
		if p.PrimeiroVenc != "" {
			base = p.PrimeiroVenc
		} else {
			base = somarDias(d.Data, f.diasVenc)
		}
	}
	var nome string
	if err := x.QueryRowContext(ctx, `SELECT ups_nome FROM Pessoas WHERE ups_codigo = ?`, d.PessoaID).Scan(&nome); errors.Is(err, sql.ErrNoRows) {
		return msg.Novo("Pessoa do documento não encontrada.")
	} else if err != nil {
		return err
	}
	rotulo := "Compra #"
	if venda {
		rotulo = "Venda #"
	}
	descricao := fmt.Sprintf("%s%d — %s", rotulo, d.Codigo, nome)

	for i := int64(0); i < n; i++ {
		valor := parcelas[i].Shift(2).Round(0).IntPart()
		titID, err := db.Inserir(ctx, x, r, "Titulo", `INSERT INTO Titulo (tit_natureza, tit_origem, tit_docto_codigo, tit_ups_codigo, tit_cat_codigo, tit_fpg_codigo, tit_tpt_codigo,
			tit_descricao, tit_parcela, tit_total_parcelas, tit_emissao, tit_competencia, tit_vencimento, tit_valor, tit_usu_codigo, tit_seg_codigo)
			VALUES (?, 'DOCUMENTO', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			nat, d.Codigo, d.PessoaID, cat, f.codigo, f.tipoTitulo, descricao, i+1, n, d.Data, primeiroDoMes(d.Data), somarDias(base, i*f.intervalo), valor, usuarioID, d.SegmentoID)
		if err != nil {
			return err
		}
		if auto {
			obs := ""
			if f.nomeBaixa != nil {
				obs = *f.nomeBaixa
			}
			t := titulo{codigo: titID, natureza: nat, valor: valor, descricao: descricao, parcela: i + 1, total: n, competencia: primeiroDoMes(d.Data), segmento: d.SegmentoID}
			if err := baixarInterno(ctx, x, r, &t, d.Data, *f.contaPadrao, f, valor, 0, 0, 0, obs, true, usuarioID); err != nil {
				return err
			}
		}
	}
	return nil
}

// escolherCategoria: a escolhida na tela; senão a do cadastro dos itens com MAIOR soma de total (empate: a de menor código); senão a do sistema.
func escolherCategoria(ctx context.Context, x db.Executor, d Documento, nat string, escolhida *int64) (int64, error) {
	if escolhida != nil {
		var id int64
		if err := x.QueryRowContext(ctx, `SELECT cat_codigo FROM Categoria_Financeira WHERE cat_codigo = ?`, *escolhida).Scan(&id); errors.Is(err, sql.ErrNoRows) {
			return 0, msg.Novo("Categoria não encontrada.")
		} else if err != nil {
			return 0, err
		}
		return id, nil
	}
	soma := map[int64]decimal.Decimal{}
	for _, it := range d.Itens {
		c := it.CatCompra
		if nat == "RECEBER" {
			c = it.CatVenda
		}
		if c != nil {
			soma[*c] = soma[*c].Add(it.Total)
		}
	}
	var melhor int64
	var achou bool
	for c, v := range soma {
		if !achou || v.GreaterThan(soma[melhor]) || (v.Equal(soma[melhor]) && c < melhor) {
			melhor, achou = c, true
		}
	}
	if achou {
		var id int64
		if err := x.QueryRowContext(ctx, `SELECT cat_codigo FROM Categoria_Financeira WHERE cat_codigo = ?`, melhor).Scan(&id); err == nil {
			return id, nil
		} else if !errors.Is(err, sql.ErrNoRows) {
			return 0, err
		}
	}
	sis := "COMPRA_MERCADORIA"
	if nat == "RECEBER" {
		sis = "VENDA_PRODUTOS"
	}
	return categoriaDoSistema(ctx, x, sis)
}

func categoriaDoSistema(ctx context.Context, x db.Executor, codigo string) (int64, error) {
	var id int64
	err := x.QueryRowContext(ctx, `SELECT cat_codigo FROM Categoria_Financeira WHERE cat_codigo_sistema = ?`, codigo).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, msg.Novo("Categoria do sistema '%s' não encontrada.", codigo)
	}
	return id, err
}

// titulo é o que a baixa precisa do título em andamento.
type titulo struct {
	codigo         int64
	natureza       string
	valor, baixado int64
	descricao      string
	parcela, total int64
	competencia    string
	segmento       *int64
}

func (t *titulo) saldo() int64 { return t.valor - t.baixado }

func status(valor, baixado int64) string {
	return string(dominio.StatusPorBaixa(cent(valor), cent(baixado)))
}

// baixarInterno lança a baixa: valida, grava a baixa e o movimento no extrato, atualiza o título e, para título a receber com forma de
// taxa, cria o título de taxa já baixado (RN-BAIXA-01, RN-BAIXA-02). conta é o código da conta; f pode ser nil.
func baixarInterno(ctx context.Context, x db.Executor, r db.Rastro, t *titulo, data string, conta int64, f *forma, valor, juros, multa, desconto int64,
	obs string, automatica bool, usuarioID int64) error {
	if valor <= 0 {
		return msg.Novo("Informe um valor maior que zero.")
	}
	if valor > t.saldo() {
		return msg.Novo("O valor da baixa (R$ %s) é maior que o saldo do título (R$ %s).", dominio.Escala(valor, 2), dominio.Escala(t.saldo(), 2))
	}
	if juros < 0 || multa < 0 || desconto < 0 {
		return msg.Novo("Juros, multa e desconto não podem ser negativos.")
	}
	if desconto > valor+juros+multa {
		return msg.Novo("O desconto não pode ser maior que o valor da baixa.")
	}
	obsTrim := strings.TrimSpace(obs)
	var obsSQL, fpg any
	if obsTrim != "" {
		obsSQL = obsTrim
	}
	if f != nil {
		fpg = f.codigo
	}
	bxID, err := db.Inserir(ctx, x, r, "Baixa", `INSERT INTO Baixa (bx_tit_codigo, bx_data, bx_cfn_codigo, bx_fpg_codigo, bx_valor, bx_juros, bx_multa, bx_desconto, bx_observacao, bx_automatica, bx_usu_codigo)
		VALUES (?,?,?,?,?,?,?,?,?,?,?)`, t.codigo, data, conta, fpg, valor, juros, multa, desconto, obsSQL, b2i(automatica), usuarioID)
	if err != nil {
		return err
	}
	if mov := valor + juros + multa - desconto; mov > 0 {
		tipo := "SAIDA"
		padrao := "Pagamento"
		if t.natureza == "RECEBER" {
			tipo, padrao = "ENTRADA", "Recebimento"
		}
		nome := padrao
		switch {
		case obsTrim != "" && !automatica:
			nome = obsTrim
		case f != nil && f.nomeBaixa != nil:
			nome = *f.nomeBaixa
		}
		desc := nome + " — " + t.descricao
		if t.total > 1 {
			desc += fmt.Sprintf(" (%d/%d)", t.parcela, t.total)
		}
		if _, err := db.Inserir(ctx, x, r, "Movimento_Conta", `INSERT INTO Movimento_Conta (mov_cfn_codigo, mov_data, mov_tipo, mov_valor, mov_descricao, mov_bx_codigo) VALUES (?,?,?,?,?,?)`,
			conta, data, tipo, mov, desc, bxID); err != nil {
			return err
		}
	}
	t.baixado += valor
	if _, err := x.ExecContext(ctx, `UPDATE Titulo SET tit_valor_baixado = ?, tit_status = ? WHERE tit_codigo = ?`, t.baixado, status(t.valor, t.baixado), t.codigo); err != nil {
		return err
	}
	if t.natureza == "RECEBER" && f != nil && f.taxaCent > 0 {
		taxa := dominio.TaxaCartao(cent(valor), decimal.New(f.taxaCent, -2)).Shift(2).Round(0).IntPart()
		if taxa > 0 {
			if err := criarTaxa(ctx, x, r, t, bxID, f, conta, data, taxa, usuarioID); err != nil {
				return err
			}
			if _, err := x.ExecContext(ctx, `UPDATE Baixa SET bx_taxa = ? WHERE bx_codigo = ?`, taxa, bxID); err != nil {
				return err
			}
		}
	}
	return nil
}

// criarTaxa lança a taxa de cartão como despesa própria (título a pagar, origem TAXA), já baixada na mesma conta.
func criarTaxa(ctx context.Context, x db.Executor, r db.Rastro, origem *titulo, baixaOrigem int64, f *forma, conta int64, data string, taxa, usuarioID int64) error {
	cat, err := categoriaDoSistema(ctx, x, "TAXA_CARTAO")
	if err != nil {
		return err
	}
	descricao := fmt.Sprintf("Taxa %s — título #%d", f.nome, origem.codigo)
	id, err := db.Inserir(ctx, x, r, "Titulo", `INSERT INTO Titulo (tit_natureza, tit_origem, tit_cat_codigo, tit_descricao, tit_emissao, tit_competencia, tit_vencimento, tit_valor, tit_baixa_origem, tit_usu_codigo, tit_seg_codigo)
		VALUES ('PAGAR', 'TAXA', ?, ?, ?, ?, ?, ?, ?, ?, ?)`, cat, descricao, data, origem.competencia, data, taxa, baixaOrigem, usuarioID, origem.segmento)
	if err != nil {
		return err
	}
	tx := titulo{codigo: id, natureza: "PAGAR", valor: taxa, descricao: descricao, parcela: 1, total: 1, competencia: origem.competencia, segmento: origem.segmento}
	return baixarInterno(ctx, x, r, &tx, data, conta, nil, taxa, 0, 0, 0, "Taxa automática — "+f.nome, true, usuarioID)
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// ───────────────────────────── desfazer ─────────────────────────────

// RemoverTitulosDocumento apaga os títulos de um documento (ao editá-lo ou excluí-lo). Baixas automáticas somem sozinhas; baixa lançada
// à mão ou movimento já conciliado bloqueiam (RN-FIN-05).
func RemoverTitulosDocumento(ctx context.Context, x db.Executor, doctoID int64) error {
	ids, err := idsDeTitulosDoDocumento(ctx, x, doctoID)
	if err != nil {
		return err
	}
	for _, tid := range ids {
		baixas, err := baixasAtivas(ctx, x, tid)
		if err != nil {
			return err
		}
		for _, b := range baixas {
			if !b.automatica {
				return msg.Novo("O documento tem baixa lançada manualmente no título #%d. Estorne a baixa no Financeiro antes de alterar ou excluir o documento.", tid)
			}
			if err := verificarSemConciliacao(ctx, x, b.codigo); err != nil {
				return err
			}
		}
	}
	for _, tid := range ids {
		if err := purgarTitulo(ctx, x, tid); err != nil {
			return err
		}
	}
	return nil
}

type baixaLinha struct {
	codigo     int64
	automatica bool
}

func idsDeTitulosDoDocumento(ctx context.Context, x db.Executor, doctoID int64) ([]int64, error) {
	return colunaInt(ctx, x, `SELECT tit_codigo FROM Titulo WHERE tit_docto_codigo = ? ORDER BY tit_parcela, tit_codigo`, doctoID)
}

func colunaInt(ctx context.Context, x db.Executor, q string, args ...any) ([]int64, error) {
	rows, err := x.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var v int64
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func baixasAtivas(ctx context.Context, x db.Executor, titID int64) ([]baixaLinha, error) {
	rows, err := x.QueryContext(ctx, `SELECT bx_codigo, bx_automatica FROM Baixa WHERE bx_tit_codigo = ? AND bx_estornada = 0 ORDER BY bx_codigo`, titID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []baixaLinha
	for rows.Next() {
		var b baixaLinha
		var a int
		if err := rows.Scan(&b.codigo, &a); err != nil {
			return nil, err
		}
		b.automatica = a == 1
		out = append(out, b)
	}
	return out, rows.Err()
}

func verificarSemConciliacao(ctx context.Context, x db.Executor, bxID int64) error {
	var conc int
	if err := x.QueryRowContext(ctx, `SELECT count(*) FROM Movimento_Conta WHERE mov_bx_codigo = ? AND mov_conciliado = 1`, bxID).Scan(&conc); err != nil {
		return err
	}
	if conc > 0 {
		return msg.Novo("O lançamento da baixa #%d já foi conciliado no extrato. Desmarque a conciliação antes.", bxID)
	}
	filhos, err := colunaInt(ctx, x, `SELECT tit_codigo FROM Titulo WHERE tit_baixa_origem = ?`, bxID)
	if err != nil {
		return err
	}
	for _, f := range filhos {
		bs, err := baixasAtivas(ctx, x, f)
		if err != nil {
			return err
		}
		for _, b := range bs {
			if err := verificarSemConciliacao(ctx, x, b.codigo); err != nil {
				return err
			}
		}
	}
	return nil
}

// purgarTitulo apaga o título, suas baixas, os movimentos delas e os títulos de taxa filhos.
func purgarTitulo(ctx context.Context, x db.Executor, titID int64) error {
	baixas, err := colunaInt(ctx, x, `SELECT bx_codigo FROM Baixa WHERE bx_tit_codigo = ? ORDER BY bx_data DESC, bx_codigo DESC`, titID)
	if err != nil {
		return err
	}
	for _, bx := range baixas {
		filhos, err := colunaInt(ctx, x, `SELECT tit_codigo FROM Titulo WHERE tit_baixa_origem = ?`, bx)
		if err != nil {
			return err
		}
		for _, f := range filhos {
			if err := purgarTitulo(ctx, x, f); err != nil {
				return err
			}
		}
		if _, err := x.ExecContext(ctx, `DELETE FROM Movimento_Conta WHERE mov_bx_codigo = ?`, bx); err != nil {
			return err
		}
		if _, err := x.ExecContext(ctx, `DELETE FROM Baixa WHERE bx_codigo = ?`, bx); err != nil {
			return err
		}
	}
	_, err = x.ExecContext(ctx, `DELETE FROM Titulo WHERE tit_codigo = ?`, titID)
	return err
}

// ───────────────────────────── resumo para a tela ─────────────────────────────

// Resumo é o que já existe no financeiro de um documento (para preencher a tela ao editar).
type Resumo struct {
	FpgCodigo      *int64
	Parcelas       int
	PrimeiroVenc   string
	CatCodigo      int64
	TemBaixaManual bool
}

// ResumoDoDocumento devolve nil se o documento não tem títulos.
func ResumoDoDocumento(ctx context.Context, x db.Executor, doctoID int64) (*Resumo, error) {
	ids, err := idsDeTitulosDoDocumento(ctx, x, doctoID)
	if err != nil || len(ids) == 0 {
		return nil, err
	}
	var rs Resumo
	var fpg sql.NullInt64
	if err := x.QueryRowContext(ctx, `SELECT tit_fpg_codigo, tit_vencimento, tit_cat_codigo FROM Titulo WHERE tit_codigo = ?`, ids[0]).Scan(&fpg, &rs.PrimeiroVenc, &rs.CatCodigo); err != nil {
		return nil, err
	}
	if fpg.Valid {
		rs.FpgCodigo = &fpg.Int64
	}
	rs.Parcelas = len(ids)
	for _, id := range ids {
		bs, err := baixasAtivas(ctx, x, id)
		if err != nil {
			return nil, err
		}
		for _, b := range bs {
			if !b.automatica {
				rs.TemBaixaManual = true
			}
		}
	}
	return &rs, nil
}

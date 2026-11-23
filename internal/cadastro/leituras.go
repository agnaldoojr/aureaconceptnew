package cadastro

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"

	"github.com/shopspring/decimal"

	"github.com/agnaldoojr/aureaconceptnew/internal/dominio"
)

// PessoaOpcao é uma pessoa nas listas de escolha.
type PessoaOpcao struct {
	ID   int64
	Nome string
}

// PessoasAtivas devolve as pessoas ativas na ordem do código (todas, não só vendedores).
func (r *Repo) PessoasAtivas(ctx context.Context) ([]PessoaOpcao, error) {
	rows, err := r.B.R.QueryContext(ctx, `SELECT ups_codigo, ups_nome FROM Pessoas WHERE ups_ativo = 1 ORDER BY ups_codigo`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var l []PessoaOpcao
	for rows.Next() {
		var p PessoaOpcao
		if err := rows.Scan(&p.ID, &p.Nome); err != nil {
			return nil, err
		}
		l = append(l, p)
	}
	return l, rows.Err()
}

// ProdutoOpcao é um produto ativo na escolha de itens do documento.
type ProdutoOpcao struct {
	ID           int64
	Nome, Und    string
	PrecoCent    int64
	Servico      bool
	UnidadesJSON string // [{"sigla":"CX","fator":12.000000,"preco":100.00}] — as unidades extras com o preço sugerido
}

// ProdutosAtivos devolve os produtos ativos na ordem do código, com o JSON das unidades extras de cada um.
func (r *Repo) ProdutosAtivos(ctx context.Context) ([]ProdutoOpcao, error) {
	extras := map[int64][]string{}
	rows, err := r.B.R.QueryContext(ctx, `SELECT u.pun_upr_codigo, u.pun_sigla, u.pun_fator, u.pun_preco, p.upr_preco FROM Produto_Unidade u JOIN Produto p ON p.upr_codigo = u.pun_upr_codigo ORDER BY u.pun_sigla`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var prod, fator, precoProd int64
		var sigla string
		var preco *int64
		if err := rows.Scan(&prod, &sigla, &fator, &preco, &precoProd); err != nil {
			rows.Close()
			return nil, err
		}
		precoTxt := ""
		if preco != nil {
			precoTxt = dominio.Escala(*preco, 2)
		} else {
			sug := dominio.PrecoSugeridoNaUnidade(decimal.New(precoProd, -2), decimal.New(fator, -6))
			precoTxt = dominio.Escala(sug.Shift(2).Round(0).IntPart(), 2)
		}
		s, _ := json.Marshal(sigla)
		extras[prod] = append(extras[prod], `{"sigla":`+string(s)+`,"fator":`+dominio.Escala(fator, 6)+`,"preco":`+precoTxt+`}`)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	prows, err := r.B.R.QueryContext(ctx, `SELECT upr_codigo, upr_nome, upr_und, upr_preco, upr_servico FROM Produto WHERE upr_ativo = 1 ORDER BY upr_codigo`)
	if err != nil {
		return nil, err
	}
	defer prows.Close()
	var l []ProdutoOpcao
	for prows.Next() {
		var p ProdutoOpcao
		var serv int
		if err := prows.Scan(&p.ID, &p.Nome, &p.Und, &p.PrecoCent, &serv); err != nil {
			return nil, err
		}
		p.Servico = serv == 1
		p.UnidadesJSON = "[" + strings.Join(extras[p.ID], ",") + "]"
		l = append(l, p)
	}
	return l, prows.Err()
}

// FormaOpcao é uma forma de pagamento ativa, com o que a tela do documento usa para se ajustar.
type FormaOpcao struct {
	ID              int64
	Nome, Natureza  string
	PermiteParcelas bool
	MaxParcelas     int64
	DiasVenc        int64
	Intervalo       int64
	BaixaAutomatica bool
	TaxaCent        int64
	Conta           string
	NomeBaixa       string
	TemNomeBaixa    bool
}

// FormasAtivas devolve as formas ativas da natureza ("RECEBER" ou "PAGAR"), por nome.
func (r *Repo) FormasAtivas(ctx context.Context, natureza string) ([]FormaOpcao, error) {
	rows, err := r.B.R.QueryContext(ctx, `SELECT f.fpg_codigo, f.fpg_nome, t.tpt_natureza, f.fpg_permite_parcelas, f.fpg_max_parcelas, f.fpg_dias_vencimento, f.fpg_intervalo_parcelas,
		f.fpg_baixa_automatica, f.fpg_taxa_percentual, COALESCE(c.cfn_nome,''), f.fpg_nome_baixa
		FROM Forma_Pagamento f JOIN Tipo_Titulo t ON t.tpt_codigo = f.fpg_tpt_codigo LEFT JOIN Conta_Financeira c ON c.cfn_codigo = f.fpg_cfn_codigo
		WHERE f.fpg_ativo = 1 AND t.tpt_natureza = ? ORDER BY f.fpg_nome`, natureza)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var l []FormaOpcao
	for rows.Next() {
		var f FormaOpcao
		var parc, auto int
		var nb *string
		if err := rows.Scan(&f.ID, &f.Nome, &f.Natureza, &parc, &f.MaxParcelas, &f.DiasVenc, &f.Intervalo, &auto, &f.TaxaCent, &f.Conta, &nb); err != nil {
			return nil, err
		}
		f.PermiteParcelas, f.BaixaAutomatica = parc == 1, auto == 1
		if nb != nil {
			f.NomeBaixa, f.TemNomeBaixa = *nb, true
		}
		l = append(l, f)
	}
	return l, rows.Err()
}

// CategoriaComTipo é uma categoria ativa com o rótulo do tipo ("Receita" ou "Despesa").
type CategoriaComTipo struct {
	ID   int64
	Nome string
	Tipo string
}

// CategoriasAtivasTodas devolve todas as categorias ativas, por nome.
func (r *Repo) CategoriasAtivasTodas(ctx context.Context) ([]CategoriaComTipo, error) {
	rows, err := r.B.R.QueryContext(ctx, `SELECT cat_codigo, cat_nome, cat_tipo FROM Categoria_Financeira WHERE cat_ativo = 1 ORDER BY cat_nome`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var l []CategoriaComTipo
	for rows.Next() {
		var c CategoriaComTipo
		var t string
		if err := rows.Scan(&c.ID, &c.Nome, &t); err != nil {
			return nil, err
		}
		c.Tipo = "Despesa"
		if t == "RECEITA" {
			c.Tipo = "Receita"
		}
		l = append(l, c)
	}
	return l, rows.Err()
}

// EhVendedor diz se a pessoa existe e está marcada como vendedor (ativa ou não).
func (r *Repo) EhVendedor(ctx context.Context, id int64) (bool, error) {
	var v int
	err := r.B.R.QueryRowContext(ctx, `SELECT ups_vendedor FROM Pessoas WHERE ups_codigo = ?`, id).Scan(&v)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return v == 1, nil
}

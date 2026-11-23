package cadastro

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/shopspring/decimal"
)

// Produto é um produto ou serviço. Valores em inteiros escalados, como no banco: preço em centavos, comissão em centésimos de %.
type Produto struct {
	Codigo         int64
	Nome           string
	Complemento    string
	Und            string
	PrecoCent      int64 // centavos; 0 quando ainda não informado (veja PrecoInformado)
	Ativo          bool
	CatVenda       *int64 // categoria financeira (receita)
	CatCompra      *int64 // categoria financeira (despesa)
	SegmentoID     *int64
	Servico        bool
	ComissaoCent   *int64 // centésimos de % (4000 = 40,00 %)
	DuracaoMin     *int64
	PrecoInformado bool
}

// UnidadeExtra é uma unidade adicional do produto com o fator de conversão para a base.
type UnidadeExtra struct {
	Sigla      string
	FatorMicro int64  // milionésimos: 12 = 12.000.000
	PrecoCent  *int64 // centavos; nulo = preço do produto × fator
}

// LinhaUnidade é uma linha digitada no formulário (antes de validar). Nulo = campo em branco.
type LinhaUnidade struct {
	Sigla string
	Fator *decimal.Decimal
	Preco *decimal.Decimal
}

func (l LinhaUnidade) vazia() bool {
	return strings.TrimSpace(l.Sigla) == "" && l.Fator == nil && l.Preco == nil
}

// ───────────────────────────── leitura ─────────────────────────────

const colunasProduto = `p.upr_codigo, p.upr_nome, COALESCE(p.upr_complemento,''), p.upr_und, p.upr_preco, p.upr_ativo, p.upr_cat_venda, p.upr_cat_compra, p.upr_seg_codigo, p.upr_servico, p.upr_comissao_perc, p.upr_duracao_min`

func lerProduto(sc interface{ Scan(...any) error }) (*Produto, error) {
	var p Produto
	var ativo, serv int
	var cv, cc, seg, com, dur sql.NullInt64
	if err := sc.Scan(&p.Codigo, &p.Nome, &p.Complemento, &p.Und, &p.PrecoCent, &ativo, &cv, &cc, &seg, &serv, &com, &dur); err != nil {
		return nil, err
	}
	p.Ativo, p.Servico, p.PrecoInformado = ativo == 1, serv == 1, true
	p.CatVenda, p.CatCompra, p.SegmentoID = ptr(cv), ptr(cc), ptr(seg)
	p.ComissaoCent, p.DuracaoMin = ptr(com), ptr(dur)
	return &p, nil
}

func ptr(n sql.NullInt64) *int64 {
	if !n.Valid {
		return nil
	}
	v := n.Int64
	return &v
}

// ListarProdutos faz a busca da tela. Com texto: só produtos ATIVOS cujo nome contém o texto (sem diferenciar maiúscula/acento;
// % e _ valem como letras comuns, como nas consultas "Containing" do Spring Data). Sem texto: todos. Depois filtra por tipo
// ("servico" ou "produto"). Ordem do código.
func (r *Repo) ListarProdutos(ctx context.Context, busca, tipo string) ([]*Produto, error) {
	q := `SELECT ` + colunasProduto + ` FROM Produto p`
	var args []any
	var onde []string
	if strings.TrimSpace(busca) != "" {
		onde = append(onde, `p.upr_ativo = 1 AND instr(aurea_norm(p.upr_nome), aurea_norm(?)) > 0`)
		args = append(args, busca)
	}
	switch tipo {
	case "servico":
		onde = append(onde, `p.upr_servico = 1`)
	case "produto":
		onde = append(onde, `p.upr_servico = 0`)
	}
	if len(onde) > 0 {
		q += ` WHERE ` + strings.Join(onde, ` AND `)
	}
	rows, err := r.B.R.QueryContext(ctx, q+` ORDER BY p.upr_codigo`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var lista []*Produto
	for rows.Next() {
		p, err := lerProduto(rows)
		if err != nil {
			return nil, err
		}
		lista = append(lista, p)
	}
	return lista, rows.Err()
}

// BuscarProduto devolve o produto; (nil, nil) se não existe.
func (r *Repo) BuscarProduto(ctx context.Context, id int64) (*Produto, error) {
	p, err := lerProduto(r.B.R.QueryRowContext(ctx, `SELECT `+colunasProduto+` FROM Produto p WHERE p.upr_codigo = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return p, err
}

// UnidadesDoProduto devolve as unidades extras, por sigla.
func (r *Repo) UnidadesDoProduto(ctx context.Context, produto int64) ([]UnidadeExtra, error) {
	rows, err := r.B.R.QueryContext(ctx, `SELECT pun_sigla, pun_fator, pun_preco FROM Produto_Unidade WHERE pun_upr_codigo = ? ORDER BY pun_sigla`, produto)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var lista []UnidadeExtra
	for rows.Next() {
		var u UnidadeExtra
		var preco sql.NullInt64
		if err := rows.Scan(&u.Sigla, &u.FatorMicro, &preco); err != nil {
			return nil, err
		}
		u.PrecoCent = ptr(preco)
		lista = append(lista, u)
	}
	return lista, rows.Err()
}

// UnidadesParaFormulario são as opções de unidade do formulário: as ativas (por sigla) mais a unidade que o produto já tem e as
// extras que já usa, mesmo inativas ou sem cadastro ("sem cadastro").
func (r *Repo) UnidadesParaFormulario(ctx context.Context, atual string, extras []string) ([]Unidade, error) {
	rows, err := r.B.R.QueryContext(ctx, `SELECT und_codigo, und_sigla, und_descricao, und_ativo FROM Unidade_Medida WHERE und_ativo = 1 ORDER BY und_sigla`)
	if err != nil {
		return nil, err
	}
	var lista []Unidade
	for rows.Next() {
		var u Unidade
		var a int
		if err := rows.Scan(&u.Codigo, &u.Sigla, &u.Descricao, &a); err != nil {
			rows.Close()
			return nil, err
		}
		u.Ativo = a == 1
		lista = append(lista, u)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	manter := append([]string{atual}, extras...)
	for _, s := range manter {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		achou := false
		for _, u := range lista {
			if strings.EqualFold(u.Sigla, s) {
				achou = true
				break
			}
		}
		if achou {
			continue
		}
		var u Unidade
		var a int
		err := r.B.R.QueryRowContext(ctx, `SELECT und_codigo, und_sigla, und_descricao, und_ativo FROM Unidade_Medida WHERE und_sigla = ?`, s).Scan(&u.Codigo, &u.Sigla, &u.Descricao, &a)
		if errors.Is(err, sql.ErrNoRows) {
			u = Unidade{Sigla: s, Descricao: "sem cadastro"}
		} else if err != nil {
			return nil, err
		} else {
			u.Ativo = a == 1
		}
		lista = append(lista, u)
	}
	return lista, nil
}

// Categoria é uma categoria financeira (só o que o formulário de produto precisa).
type Categoria struct {
	Codigo int64
	Nome   string
}

// CategoriasAtivas devolve as categorias ativas do tipo ("RECEITA" ou "DESPESA"), por nome.
func (r *Repo) CategoriasAtivas(ctx context.Context, tipo string) ([]Categoria, error) {
	rows, err := r.B.R.QueryContext(ctx, `SELECT cat_codigo, cat_nome FROM Categoria_Financeira WHERE cat_ativo = 1 AND cat_tipo = ? ORDER BY cat_nome`, tipo)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var lista []Categoria
	for rows.Next() {
		var c Categoria
		if err := rows.Scan(&c.Codigo, &c.Nome); err != nil {
			return nil, err
		}
		lista = append(lista, c)
	}
	return lista, rows.Err()
}

// ResolverSegmento converte o código da tela em segmento. Nulo = sem segmento. Desconhecido ou inativo é erro.
func (r *Repo) ResolverSegmento(ctx context.Context, id *int64) error {
	if id == nil {
		return nil
	}
	var nome string
	var ativo int
	err := r.B.R.QueryRowContext(ctx, `SELECT seg_nome, seg_ativo FROM Segmento WHERE seg_codigo = ?`, *id).Scan(&nome, &ativo)
	if errors.Is(err, sql.ErrNoRows) {
		return regra("Segmento não encontrado.")
	}
	if err != nil {
		return err
	}
	if ativo != 1 {
		return regra("O segmento \"%s\" está inativo.", nome)
	}
	return nil
}

// ───────────────────────────── validação de campos (anotações de Produto.java) ─────────────────────────────

// ValidarProduto aplica @NotBlank/@Size/@DecimalMin/@Min/@Max do modelo. Os campos sem texto na tela (complemento, unidade > 10)
// devolvem erro sem mensagem, como no antigo.
func ValidarProduto(p *Produto, comissao *decimal.Decimal, duracao *int64) []ErroCampo {
	var erros []ErroCampo
	if strings.TrimSpace(p.Nome) == "" {
		erros = append(erros, ErroCampo{"uprNome", "Nome do produto é obrigatório"})
	} else if utf8.RuneCountInString(p.Nome) > 150 {
		erros = append(erros, ErroCampo{"uprNome", "Nome deve ter no máximo 150 caracteres"})
	}
	if utf8.RuneCountInString(p.Complemento) > 255 {
		erros = append(erros, ErroCampo{"uprComplemento", ""})
	}
	if strings.TrimSpace(p.Und) == "" {
		erros = append(erros, ErroCampo{"uprUnd", "Unidade é obrigatória"})
	} else if utf8.RuneCountInString(p.Und) > 10 {
		erros = append(erros, ErroCampo{"uprUnd", ""})
	}
	if !p.PrecoInformado {
		erros = append(erros, ErroCampo{"uprPreco", "Preço é obrigatório"})
	} else if p.PrecoCent < 1 {
		erros = append(erros, ErroCampo{"uprPreco", "Preço deve ser maior que zero"})
	}
	if comissao != nil {
		if comissao.IsNegative() {
			erros = append(erros, ErroCampo{"uprComissaoPerc", "A comissão não pode ser negativa"})
		} else if comissao.GreaterThan(decimal.NewFromInt(100)) {
			erros = append(erros, ErroCampo{"uprComissaoPerc", "A comissão não pode passar de 100%"})
		}
	}
	if duracao != nil {
		if *duracao < 1 {
			erros = append(erros, ErroCampo{"uprDuracaoMin", "A duração deve ser de pelo menos 1 minuto"})
		} else if *duracao > 1440 {
			erros = append(erros, ErroCampo{"uprDuracaoMin", "A duração não pode passar de 24 horas"})
		}
	}
	return erros
}

// ───────────────────────────── gravação ─────────────────────────────

const fatorMaximo = 1_000_000

// qtdTexto escreve milésimos como o antigo (`stripTrailingZeros().toPlainString()`): 10000 → "10", 2500 → "2.5".
func qtdTexto(milesimos int64) string {
	return decimal.New(milesimos, -3).String()
}

// SalvarProduto grava o produto e as unidades extras numa só transação (como o @Transactional do antigo): qualquer erro desfaz tudo.
// Mensagens e ordem das verificações são as do ProdutoService/ProdutoUnidadeService. Devolve o código gravado.
func (r *Repo) salvarProduto(ctx context.Context, p *Produto, linhas []LinhaUnidade) (int64, error) {
	tx, err := r.B.W.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	novo := p.Codigo == 0
	var atual string
	existe := false
	if !novo {
		err := tx.QueryRowContext(ctx, `SELECT upr_und FROM Produto WHERE upr_codigo = ?`, p.Codigo).Scan(&atual)
		if err == nil {
			existe = true
		} else if !errors.Is(err, sql.ErrNoRows) {
			return 0, err
		}
	}

	temLinhas := false
	for _, l := range linhas {
		if !l.vazia() {
			temLinhas = true
		}
	}
	// 1) com unidades extras no formulário, a base não pode mudar
	if temLinhas && existe && strings.TrimSpace(p.Und) != "" && !strings.EqualFold(atual, strings.TrimSpace(p.Und)) {
		return 0, regra("Para trocar a unidade base (%s), remova antes as unidades adicionais do produto: os fatores de conversão valem em relação à unidade base.", atual)
	}

	// 2) a unidade precisa estar cadastrada (e ativa, a menos que o produto já a tenha)
	base, err := validarUnidadeParaProduto(ctx, tx, p.Und, ternario(existe, atual, ""), existe)
	if err != nil {
		return 0, err
	}
	p.Und = base

	// 3) serviço: sem estoque, só com saldo zero; produto: sem comissão e duração
	var estoqueQtd sql.NullInt64
	temEstoque := false
	if existe {
		if err := tx.QueryRowContext(ctx, `SELECT est_qtd FROM Estoque WHERE est_upr_codigo = ?`, p.Codigo).Scan(&estoqueQtd); err == nil {
			temEstoque = true
		} else if !errors.Is(err, sql.ErrNoRows) {
			return 0, err
		}
	}
	if p.Servico {
		if temEstoque && estoqueQtd.Int64 != 0 {
			return 0, regra("Este produto ainda tem %s em estoque. Zere o estoque (saída ou ajuste) antes de transformá-lo em serviço.", qtdTexto(estoqueQtd.Int64))
		}
	} else {
		p.ComissaoCent, p.DuracaoMin = nil, nil
	}

	// 4) grava o produto
	id := p.Codigo
	if novo || !existe {
		res, err := tx.ExecContext(ctx, `INSERT INTO Produto (upr_nome, upr_complemento, upr_und, upr_preco, upr_ativo, upr_cat_venda, upr_cat_compra, upr_seg_codigo, upr_servico, upr_comissao_perc, upr_duracao_min) VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
			p.Nome, p.Complemento, p.Und, p.PrecoCent, b2i(p.Ativo), p.CatVenda, p.CatCompra, p.SegmentoID, b2i(p.Servico), p.ComissaoCent, p.DuracaoMin)
		if err != nil {
			return 0, fmt.Errorf("gravar produto: %w", err)
		}
		if id, err = res.LastInsertId(); err != nil {
			return 0, err
		}
	} else {
		if _, err := tx.ExecContext(ctx, `UPDATE Produto SET upr_nome=?, upr_complemento=?, upr_und=?, upr_preco=?, upr_ativo=?, upr_cat_venda=?, upr_cat_compra=?, upr_seg_codigo=?, upr_servico=?, upr_comissao_perc=?, upr_duracao_min=? WHERE upr_codigo=?`,
			p.Nome, p.Complemento, p.Und, p.PrecoCent, b2i(p.Ativo), p.CatVenda, p.CatCompra, p.SegmentoID, b2i(p.Servico), p.ComissaoCent, p.DuracaoMin, id); err != nil {
			return 0, fmt.Errorf("gravar produto: %w", err)
		}
	}
	// a linha de estoque acompanha: serviço não tem; produto sempre tem (0/0 quando nasce)
	if p.Servico {
		if _, err := tx.ExecContext(ctx, `DELETE FROM Estoque WHERE est_upr_codigo = ?`, id); err != nil {
			return id, err
		}
	} else if !temEstoque {
		if _, err := tx.ExecContext(ctx, `INSERT INTO Estoque (est_upr_codigo, est_qtd, est_valor_custo) VALUES (?,0,0)`, id); err != nil {
			return id, err
		}
	}

	// 5) unidades extras
	if err := substituirUnidades(ctx, tx, id, p.Und, linhas); err != nil {
		return id, err
	}
	return id, tx.Commit()
}

func ternario(c bool, a, b string) string {
	if c {
		return a
	}
	return b
}

type consultor interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

// validarUnidadeParaProduto confere a unidade escolhida e devolve a sigla como está cadastrada (UnidadeMedidaService.validarParaProduto).
func validarUnidadeParaProduto(ctx context.Context, q consultor, sigla, siglaAtual string, temAtual bool) (string, error) {
	s := strings.TrimSpace(sigla)
	var cad string
	var ativo int
	err := q.QueryRowContext(ctx, `SELECT und_sigla, und_ativo FROM Unidade_Medida WHERE und_sigla = ?`, s).Scan(&cad, &ativo)
	if errors.Is(err, sql.ErrNoRows) {
		return "", regra("A unidade de medida '%s' não está cadastrada.", s)
	}
	if err != nil {
		return "", err
	}
	mantida := temAtual && strings.EqualFold(strings.TrimSpace(siglaAtual), s)
	if ativo != 1 && !mantida {
		return "", regra("A unidade de medida '%s' está inativa.", cad)
	}
	return cad, nil
}

// substituirUnidades troca todas as unidades extras do produto pelas linhas informadas (ProdutoUnidadeService.substituir).
func substituirUnidades(ctx context.Context, tx consultor, produto int64, base string, linhas []LinhaUnidade) error {
	rows, err := tx.QueryContext(ctx, `SELECT pun_sigla FROM Produto_Unidade WHERE pun_upr_codigo = ?`, produto)
	if err != nil {
		return err
	}
	jaVinculadas := map[string]bool{}
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			rows.Close()
			return err
		}
		jaVinculadas[strings.ToUpper(s)] = true
	}
	rows.Close()

	type nova struct {
		sigla string
		fator int64
		preco *int64
	}
	var novas []nova
	vistas := map[string]bool{}
	for _, l := range linhas {
		if l.vazia() {
			continue
		}
		if strings.TrimSpace(l.Sigla) == "" {
			return regra("Escolha a unidade em todas as linhas de unidades adicionais.")
		}
		pedida := strings.TrimSpace(l.Sigla)
		sigla, err := validarUnidadeParaProduto(ctx, tx, pedida, pedida, jaVinculadas[strings.ToUpper(pedida)])
		if err != nil {
			return err
		}
		if strings.EqualFold(sigla, base) {
			return regra("A unidade '%s' já é a unidade base do produto.", sigla)
		}
		chave := strings.ToUpper(sigla)
		if vistas[chave] {
			return regra("A unidade '%s' está repetida.", sigla)
		}
		vistas[chave] = true
		if l.Fator == nil || l.Fator.Sign() <= 0 {
			return regra("Informe quantas %s cabem em 1 %s (maior que zero).", base, sigla)
		}
		if l.Fator.GreaterThan(decimal.NewFromInt(fatorMaximo)) {
			return regra("O fator de %s é grande demais (máximo 1.000.000).", sigla)
		}
		if l.Preco != nil && l.Preco.Sign() <= 0 {
			return regra("O preço de %s deve ser maior que zero (ou deixe em branco).", sigla)
		}
		fator := l.Fator.Shift(6).Round(0).IntPart() // 6 casas, HALF_UP
		if fator < 1 {
			return regra("O fator de %s é pequeno demais (mínimo 0,000001).", sigla)
		}
		n := nova{sigla: sigla, fator: fator}
		if l.Preco != nil {
			c := l.Preco.Shift(2).Round(0).IntPart()
			if c < 1 {
				return regra("O preço de %s deve ser maior que zero (ou deixe em branco).", sigla)
			}
			n.preco = &c
		}
		novas = append(novas, n)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM Produto_Unidade WHERE pun_upr_codigo = ?`, produto); err != nil {
		return err
	}
	for _, n := range novas {
		if _, err := tx.ExecContext(ctx, `INSERT INTO Produto_Unidade (pun_upr_codigo, pun_sigla, pun_fator, pun_preco) VALUES (?,?,?,?)`, produto, n.sigla, n.fator, n.preco); err != nil {
			return fmt.Errorf("gravar unidade extra: %w", err)
		}
	}
	return nil
}

// AlterarStatusProduto ativa ou inativa. Ativar exige preço de pelo menos 0,01 (a mesma regra do cadastro).
// Código inexistente não faz nada. O erro de preço é ErroRegra: a tela volta à edição do produto.
func (r *Repo) AlterarStatusProduto(ctx context.Context, id int64, ativo bool) error {
	tx, err := r.B.W.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var nome string
	var preco int64
	err = tx.QueryRowContext(ctx, `SELECT upr_nome, upr_preco FROM Produto WHERE upr_codigo = ?`, id).Scan(&nome, &preco)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if ativo && preco < 1 {
		return regra("Defina o preço do produto \"%s\" (maior que zero) antes de ativar: clique em Editar.", nome)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE Produto SET upr_ativo = ? WHERE upr_codigo = ?`, b2i(ativo), id); err != nil {
		return err
	}
	return tx.Commit()
}

// ErroProdutoEmUso é devolvido quando o produto tem documentos (ou outro vínculo) e não pode ser excluído.
var ErroProdutoEmUso = errors.New("produto em uso")

// ExcluirProduto apaga o produto, a linha de estoque zerada e as unidades extras (decisão D-03: o antigo falhava sempre aqui,
// porque nunca removia a linha de Estoque). Com documentos, ou com saldo em estoque, nada é apagado e devolve ErroProdutoEmUso.
// Código inexistente não faz nada (como o deleteById do antigo).
func (r *Repo) ExcluirProduto(ctx context.Context, id int64) error {
	tx, err := r.B.W.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var docs, saldo int64
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM Itens_Documento WHERE itemdocto_upr_codigo = ?`, id).Scan(&docs); err != nil {
		return err
	}
	if docs > 0 {
		return ErroProdutoEmUso
	}
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE((SELECT est_qtd FROM Estoque WHERE est_upr_codigo = ?), 0)`, id).Scan(&saldo); err != nil {
		return err
	}
	if saldo != 0 {
		return ErroProdutoEmUso
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM Produto_Unidade WHERE pun_upr_codigo = ?`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM Estoque WHERE est_upr_codigo = ?`, id); err != nil {
		return err
	}
	// outros vínculos (ex.: serviço usado numa agenda) são barrados pelas chaves estrangeiras
	if _, err := tx.ExecContext(ctx, `DELETE FROM Produto WHERE upr_codigo = ?`, id); err != nil {
		return fmt.Errorf("%w: %v", ErroProdutoEmUso, err)
	}
	return tx.Commit()
}

// SalvarProduto grava o produto e as unidades extras (veja salvarProduto). Devolve o código gravado.
//
// Se um produto NOVO falha depois de inserido (erro nas unidades extras), tudo é desfeito, mas o código gerado é devolvido junto
// com o erro e fica "queimado", como a coluna IDENTITY do SQL Server do antigo: a tela volta mostrando esse código (o antigo
// chama o formulário de "Editar Produto") e o próximo produto criado usa o seguinte.
func (r *Repo) SalvarProduto(ctx context.Context, p *Produto, linhas []LinhaUnidade) (int64, error) {
	novo := p.Codigo == 0
	id, err := r.salvarProduto(ctx, p, linhas)
	if err != nil && novo && id != 0 {
		r.queimarCodigo(ctx, "Produto", id)
	}
	return id, err
}

// queimarCodigo faz o próximo AUTOINCREMENT da tabela começar depois de id (ver db.Banco.QueimarCodigo).
func (r *Repo) queimarCodigo(ctx context.Context, tabela string, id int64) {
	r.B.QueimarCodigo(ctx, tabela, id)
}

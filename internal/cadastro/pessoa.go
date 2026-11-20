package cadastro

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// Segmento é um segmento do negócio (Loja de roupa, Salão).
type Segmento struct {
	Codigo int64
	Nome   string
	Ativo  bool
}

// Pessoa é um cadastro de cliente/fornecedor/vendedor/funcionário.
type Pessoa struct {
	Codigo      int64
	Nome        string
	CPF         string
	Telefone    string
	Cliente     bool
	Fornecedor  bool
	Vendedor    bool
	Funcionario bool
	Ativo       bool
	Nascimento  string // AAAA-MM-DD ou vazio
	Segmentos   []Segmento
}

// SegmentoIDs devolve os códigos dos segmentos da pessoa.
func (p *Pessoa) SegmentoIDs() []int64 {
	ids := make([]int64, 0, len(p.Segmentos))
	for _, s := range p.Segmentos {
		ids = append(ids, s.Codigo)
	}
	return ids
}

// TemSegmento diz se a pessoa pertence ao segmento.
func (p *Pessoa) TemSegmento(id int64) bool {
	for _, s := range p.Segmentos {
		if s.Codigo == id {
			return true
		}
	}
	return false
}

// ErroCPFDuplicado é o caso "CPF/CNPJ já usado por outra pessoa" (a API do app devolve como conflito, 409).
type ErroCPFDuplicado struct{ ErroRegra }

// Digitos mantém só os algarismos (como o `replaceAll("\\D", "")` do antigo).
func Digitos(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ErroCampo é um erro de validação de um campo do formulário (a tela volta com a mensagem ao lado do campo).
type ErroCampo struct {
	Campo    string
	Mensagem string // vazia = erro sem texto na tela (o antigo não mostra mensagem para estes campos)
}

// ValidarPessoa aplica as anotações de Pessoa.java (@NotBlank, @Size, @PastOrPresent). hoje é a data local de hoje (AAAA-MM-DD).
// Devolve os erros por campo (vazio = válida).
func ValidarPessoa(p *Pessoa, hoje string) []ErroCampo {
	var erros []ErroCampo
	if strings.TrimSpace(p.Nome) == "" {
		erros = append(erros, ErroCampo{"upsNome", "Nome é obrigatório"})
	} else if utf8.RuneCountInString(p.Nome) > 150 {
		erros = append(erros, ErroCampo{"upsNome", "Nome deve ter no máximo 150 caracteres"})
	}
	if utf8.RuneCountInString(p.CPF) > 20 {
		erros = append(erros, ErroCampo{"upsCpf", ""}) // "CPF/CNPJ inválido" existe no modelo, mas a tela não o exibe
	}
	if utf8.RuneCountInString(p.Telefone) > 20 {
		erros = append(erros, ErroCampo{"upsTelefone", ""})
	}
	if p.Nascimento != "" && p.Nascimento > hoje {
		erros = append(erros, ErroCampo{"upsNascimento", "Data de nascimento não pode ser futura"})
	}
	return erros
}

// ValidarNascimento diz se o texto é uma data AAAA-MM-DD válida (vazio vale).
func ValidarNascimento(s string) bool {
	if s == "" {
		return true
	}
	_, err := time.Parse("2006-01-02", s)
	return err == nil
}

const colunasPessoa = `p.ups_codigo, p.ups_nome, COALESCE(p.ups_cpf,''), COALESCE(p.ups_telefone,''), p.ups_cliente, p.ups_fornecedor, p.ups_vendedor, p.ups_funcionario, p.ups_ativo, COALESCE(p.ups_nascimento,'')`

func lerPessoa(sc interface{ Scan(...any) error }) (*Pessoa, error) {
	var p Pessoa
	var cli, forn, vend, func_, ativo int
	if err := sc.Scan(&p.Codigo, &p.Nome, &p.CPF, &p.Telefone, &cli, &forn, &vend, &func_, &ativo, &p.Nascimento); err != nil {
		return nil, err
	}
	p.Cliente, p.Fornecedor, p.Vendedor, p.Funcionario, p.Ativo = cli == 1, forn == 1, vend == 1, func_ == 1, ativo == 1
	return &p, nil
}

// ListarPessoas faz a busca da tela: texto (nome sem acento/maiúscula, CPF/CNPJ ou telefone), tipo e segmento; filtro vazio é ignorado.
// A ordem é a do código, como o SQL Server devolvia no antigo.
func (r *Repo) ListarPessoas(ctx context.Context, busca, tipo string, segmento *int64) ([]*Pessoa, error) {
	var onde []string
	var args []any
	if busca != "" && strings.TrimSpace(busca) != "" { // Java: termo em branco = todos
		onde = append(onde, `(aurea_norm(p.ups_nome) LIKE '%' || aurea_norm(?) || '%' OR p.ups_cpf LIKE '%' || ? || '%' OR p.ups_telefone LIKE '%' || ? || '%')`)
		args = append(args, busca, busca, busca)
	}
	switch strings.ToLower(strings.TrimSpace(tipo)) {
	case "cliente":
		onde = append(onde, `p.ups_cliente = 1`)
	case "fornecedor":
		onde = append(onde, `p.ups_fornecedor = 1`)
	case "vendedor":
		onde = append(onde, `p.ups_vendedor = 1`)
	case "funcionario":
		onde = append(onde, `p.ups_funcionario = 1`)
	}
	if segmento != nil {
		onde = append(onde, `EXISTS (SELECT 1 FROM Pessoa_Segmento s WHERE s.ps_ups_codigo = p.ups_codigo AND s.ps_seg_codigo = ?)`)
		args = append(args, *segmento)
	}
	q := `SELECT ` + colunasPessoa + ` FROM Pessoas p`
	if len(onde) > 0 {
		q += ` WHERE ` + strings.Join(onde, ` AND `)
	}
	q += ` ORDER BY p.ups_codigo`
	rows, err := r.B.R.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var lista []*Pessoa
	for rows.Next() {
		p, err := lerPessoa(rows)
		if err != nil {
			return nil, err
		}
		lista = append(lista, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return lista, r.carregarSegmentos(ctx, lista)
}

// carregarSegmentos preenche Segmentos de cada pessoa (ordem do código do segmento).
func (r *Repo) carregarSegmentos(ctx context.Context, pessoas []*Pessoa) error {
	if len(pessoas) == 0 {
		return nil
	}
	rows, err := r.B.R.QueryContext(ctx, `SELECT ps.ps_ups_codigo, s.seg_codigo, s.seg_nome, s.seg_ativo FROM Pessoa_Segmento ps JOIN Segmento s ON s.seg_codigo = ps.ps_seg_codigo ORDER BY ps.ps_ups_codigo, s.seg_codigo`)
	if err != nil {
		return err
	}
	defer rows.Close()
	por := map[int64]*Pessoa{}
	for _, p := range pessoas {
		por[p.Codigo] = p
	}
	for rows.Next() {
		var pid int64
		var s Segmento
		var ativo int
		if err := rows.Scan(&pid, &s.Codigo, &s.Nome, &ativo); err != nil {
			return err
		}
		s.Ativo = ativo == 1
		if p := por[pid]; p != nil {
			p.Segmentos = append(p.Segmentos, s)
		}
	}
	return rows.Err()
}

// BuscarPessoa devolve a pessoa com seus segmentos; (nil, nil) se não existe.
func (r *Repo) BuscarPessoa(ctx context.Context, id int64) (*Pessoa, error) {
	p, err := lerPessoa(r.B.R.QueryRowContext(ctx, `SELECT `+colunasPessoa+` FROM Pessoas p WHERE p.ups_codigo = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return p, r.carregarSegmentos(ctx, []*Pessoa{p})
}

// SegmentosAtivos devolve os segmentos ativos por nome.
func (r *Repo) SegmentosAtivos(ctx context.Context) ([]Segmento, error) {
	rows, err := r.B.R.QueryContext(ctx, `SELECT seg_codigo, seg_nome, seg_ativo FROM Segmento WHERE seg_ativo = 1 ORDER BY seg_nome`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var lista []Segmento
	for rows.Next() {
		var s Segmento
		var a int
		if err := rows.Scan(&s.Codigo, &s.Nome, &a); err != nil {
			return nil, err
		}
		s.Ativo = a == 1
		lista = append(lista, s)
	}
	return lista, rows.Err()
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// SalvarPessoa grava a pessoa trocando os segmentos pelos informados (RN-PESSOA): segmento inativo só é aceito se a pessoa já o tinha;
// CPF/CNPJ repetido (comparando só os dígitos) barra, salvo se a pessoa já tinha esse mesmo número. Devolve o código gravado.
func (r *Repo) SalvarPessoa(ctx context.Context, p *Pessoa, segmentoIDs []int64) (int64, error) {
	tx, err := r.B.W.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	jaTinha := map[int64]bool{}
	var cpfAntigo string
	existe := false
	if p.Codigo != 0 {
		var a sql.NullString
		err := tx.QueryRowContext(ctx, `SELECT ups_cpf FROM Pessoas WHERE ups_codigo = ?`, p.Codigo).Scan(&a)
		if err == nil {
			existe, cpfAntigo = true, a.String
			rows, err := tx.QueryContext(ctx, `SELECT ps_seg_codigo FROM Pessoa_Segmento WHERE ps_ups_codigo = ?`, p.Codigo)
			if err != nil {
				return 0, err
			}
			for rows.Next() {
				var id int64
				if err := rows.Scan(&id); err != nil {
					rows.Close()
					return 0, err
				}
				jaTinha[id] = true
			}
			rows.Close()
		} else if !errors.Is(err, sql.ErrNoRows) {
			return 0, err
		}
	}

	var novos []int64
	visto := map[int64]bool{}
	for _, id := range segmentoIDs {
		var nome string
		var ativo int
		if err := tx.QueryRowContext(ctx, `SELECT seg_nome, seg_ativo FROM Segmento WHERE seg_codigo = ?`, id).Scan(&nome, &ativo); errors.Is(err, sql.ErrNoRows) {
			return 0, regra("Segmento não encontrado.")
		} else if err != nil {
			return 0, err
		}
		if ativo != 1 && !jaTinha[id] {
			return 0, regra("O segmento \"%s\" está inativo.", nome)
		}
		if !visto[id] {
			visto[id] = true
			novos = append(novos, id)
		}
	}

	// CPF/CNPJ único comparando só dígitos
	if d := Digitos(p.CPF); d != "" && !(existe && d == Digitos(cpfAntigo)) {
		var n int
		err := tx.QueryRowContext(ctx, `SELECT count(*) FROM Pessoas WHERE ups_codigo <> ? AND REPLACE(REPLACE(REPLACE(REPLACE(ups_cpf,'.',''),'-',''),'/',''),' ','') = ?`, p.Codigo, d).Scan(&n)
		if err != nil {
			return 0, err
		}
		if n > 0 {
			return 0, ErroCPFDuplicado{regra("Já existe uma pessoa cadastrada com este CPF/CNPJ.").(ErroRegra)}
		}
	}

	var nasc any
	if p.Nascimento != "" {
		nasc = p.Nascimento
	}
	id := p.Codigo
	if p.Codigo == 0 || !existe {
		// pessoa nova (o antigo, com código que não existe mais, também inseria uma nova)
		res, err := tx.ExecContext(ctx, `INSERT INTO Pessoas (ups_nome, ups_cpf, ups_telefone, ups_cliente, ups_fornecedor, ups_vendedor, ups_funcionario, ups_ativo, ups_nascimento) VALUES (?,?,?,?,?,?,?,?,?)`,
			p.Nome, p.CPF, p.Telefone, b2i(p.Cliente), b2i(p.Fornecedor), b2i(p.Vendedor), b2i(p.Funcionario), b2i(p.Ativo), nasc)
		if err != nil {
			return 0, err
		}
		if id, err = res.LastInsertId(); err != nil {
			return 0, err
		}
	} else {
		if _, err := tx.ExecContext(ctx, `UPDATE Pessoas SET ups_nome=?, ups_cpf=?, ups_telefone=?, ups_cliente=?, ups_fornecedor=?, ups_vendedor=?, ups_funcionario=?, ups_ativo=?, ups_nascimento=? WHERE ups_codigo=?`,
			p.Nome, p.CPF, p.Telefone, b2i(p.Cliente), b2i(p.Fornecedor), b2i(p.Vendedor), b2i(p.Funcionario), b2i(p.Ativo), nasc, p.Codigo); err != nil {
			return 0, err
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM Pessoa_Segmento WHERE ps_ups_codigo = ?`, id); err != nil {
		return 0, err
	}
	for _, s := range novos {
		if _, err := tx.ExecContext(ctx, `INSERT INTO Pessoa_Segmento (ps_ups_codigo, ps_seg_codigo) VALUES (?,?)`, id, s); err != nil {
			return 0, err
		}
	}
	return id, tx.Commit()
}

// TelefoneJaCadastrado diz se OUTRA pessoa tem este telefone (ignorando a máscara). Só serve para avisar.
func (r *Repo) TelefoneJaCadastrado(ctx context.Context, telefone string, ignorar int64) (bool, error) {
	d := Digitos(telefone)
	if d == "" {
		return false, nil
	}
	var n int
	err := r.B.R.QueryRowContext(ctx, `SELECT count(*) FROM Pessoas WHERE ups_codigo <> ? AND REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(ups_telefone,'(',''),')',''),'-',''),' ',''),'.','') = ?`, ignorar, d).Scan(&n)
	return n > 0, err
}

// ExcluirPessoa apaga a pessoa e seus vínculos de segmento. Falha (e nada é apagado) se houver documentos, títulos, usuário,
// agenda etc. ligados a ela. Código inexistente não faz nada (como o deleteById do antigo).
func (r *Repo) ExcluirPessoa(ctx context.Context, id int64) error {
	tx, err := r.B.W.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM Pessoa_Segmento WHERE ps_ups_codigo = ?`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM Pessoas WHERE ups_codigo = ?`, id); err != nil {
		return fmt.Errorf("excluir pessoa: %w", err)
	}
	return tx.Commit()
}

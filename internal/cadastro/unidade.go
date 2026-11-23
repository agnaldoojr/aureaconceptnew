// Package cadastro tem as regras dos cadastros (unidades de medida, pessoas, ...), escritas a partir dos serviços do sistema antigo
// (UnidadeMedidaService, PessoaService). As mensagens de erro de regra são as do antigo, prontas para a tela.
package cadastro

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/agnaldoojr/aureaconceptnew/internal/db"
	"github.com/agnaldoojr/aureaconceptnew/internal/msg"
)

// ErroRegra é o erro de regra de negócio compartilhado (msg.Erro): a mensagem vai direto para o usuário (flash "erro").
type ErroRegra = msg.Erro

func regra(f string, a ...any) error { return msg.Novo(f, a...) }

// Repo reúne o acesso ao banco dos cadastros: leituras no pool de leitura, gravações na conexão de escrita (em transação).
type Repo struct{ B *db.Banco }

// Unidade é uma unidade de medida.
type Unidade struct {
	Codigo    int64
	Sigla     string
	Descricao string
	Ativo     bool
}

// UnidadeComUso é a linha da lista: a unidade e quantos produtos a usam (como base ou como unidade extra).
type UnidadeComUso struct {
	Unidade
	Uso int64
}

const sqlUso = `(SELECT count(*) FROM Produto p WHERE p.upr_und = u.und_sigla COLLATE AUREA_CIAI)
	+ (SELECT count(*) FROM Produto_Unidade x WHERE x.pun_sigla = u.und_sigla COLLATE AUREA_CIAI)`

// ListarUnidades devolve todas, por sigla (sem diferenciar maiúscula/acento), com o uso de cada uma.
func (r *Repo) ListarUnidades(ctx context.Context) ([]UnidadeComUso, error) {
	rows, err := r.B.R.QueryContext(ctx, `SELECT u.und_codigo, u.und_sigla, u.und_descricao, u.und_ativo, `+sqlUso+` FROM Unidade_Medida u ORDER BY u.und_sigla`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var lista []UnidadeComUso
	for rows.Next() {
		var x UnidadeComUso
		var ativo int
		if err := rows.Scan(&x.Codigo, &x.Sigla, &x.Descricao, &ativo, &x.Uso); err != nil {
			return nil, err
		}
		x.Ativo = ativo == 1
		lista = append(lista, x)
	}
	return lista, rows.Err()
}

// BuscarUnidade devolve a unidade e o seu uso; (nil, 0, nil) se não existe.
func (r *Repo) BuscarUnidade(ctx context.Context, id int64) (*UnidadeComUso, error) {
	var x UnidadeComUso
	var ativo int
	err := r.B.R.QueryRowContext(ctx, `SELECT u.und_codigo, u.und_sigla, u.und_descricao, u.und_ativo, `+sqlUso+` FROM Unidade_Medida u WHERE u.und_codigo = ?`, id).
		Scan(&x.Codigo, &x.Sigla, &x.Descricao, &ativo, &x.Uso)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	x.Ativo = ativo == 1
	return &x, nil
}

func usoDaSigla(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, sigla string) (int64, error) {
	var n int64
	err := q.QueryRowContext(ctx, `SELECT (SELECT count(*) FROM Produto WHERE upr_und = ?1 COLLATE AUREA_CIAI) + (SELECT count(*) FROM Produto_Unidade WHERE pun_sigla = ?1 COLLATE AUREA_CIAI)`, sigla).Scan(&n)
	return n, err
}

// SalvarUnidade valida e grava (codigo 0 = nova). As regras e as mensagens são as do UnidadeMedidaService (RN-UND-01).
func (r *Repo) SalvarUnidade(ctx context.Context, f Unidade) error {
	sigla := strings.ToUpper(strings.TrimSpace(f.Sigla))
	descricao := strings.TrimSpace(f.Descricao)
	switch {
	case sigla == "":
		return regra("Informe a sigla da unidade.")
	case utf8.RuneCountInString(sigla) > 10:
		return regra("A sigla pode ter no máximo 10 caracteres.")
	case strings.IndexFunc(sigla, unicode.IsSpace) >= 0:
		return regra("A sigla não pode ter espaços.")
	case descricao == "":
		return regra("Informe a descrição da unidade.")
	case utf8.RuneCountInString(descricao) > 60:
		return regra("A descrição pode ter no máximo 60 caracteres.")
	}

	tx, err := r.B.W.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var outroID int64
	var outraSigla string
	err = tx.QueryRowContext(ctx, `SELECT und_codigo, und_sigla FROM Unidade_Medida WHERE und_sigla = ?`, sigla).Scan(&outroID, &outraSigla)
	if err == nil && outroID != f.Codigo {
		return regra("Já existe a unidade '%s'.", outraSigla)
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	ativo := 0
	if f.Ativo {
		ativo = 1
	}
	if f.Codigo == 0 {
		if _, err := tx.ExecContext(ctx, `INSERT INTO Unidade_Medida (und_sigla, und_descricao, und_ativo) VALUES (?,?,?)`, sigla, descricao, ativo); err != nil {
			return err
		}
		return tx.Commit()
	}
	var atual string
	if err := tx.QueryRowContext(ctx, `SELECT und_sigla FROM Unidade_Medida WHERE und_codigo = ?`, f.Codigo).Scan(&atual); errors.Is(err, sql.ErrNoRows) {
		return regra("Unidade de medida não encontrada.")
	} else if err != nil {
		return err
	}
	if !strings.EqualFold(atual, sigla) {
		uso, err := usoDaSigla(ctx, tx, atual)
		if err != nil {
			return err
		}
		if uso > 0 {
			return regra("A sigla '%s' é usada por %d produto(s) e não pode ser trocada. Altere só a descrição ou crie uma unidade nova.", atual, uso)
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE Unidade_Medida SET und_sigla = ?, und_descricao = ?, und_ativo = ? WHERE und_codigo = ?`, sigla, descricao, ativo, f.Codigo); err != nil {
		return err
	}
	return tx.Commit()
}

// AlterarStatusUnidade ativa/inativa; id inexistente não faz nada (como o antigo).
func (r *Repo) AlterarStatusUnidade(ctx context.Context, id int64, ativo bool) error {
	v := 0
	if ativo {
		v = 1
	}
	_, err := r.B.W.ExecContext(ctx, `UPDATE Unidade_Medida SET und_ativo = ? WHERE und_codigo = ?`, v, id)
	return err
}

// ExcluirUnidade apaga uma unidade que nenhum produto usa.
func (r *Repo) ExcluirUnidade(ctx context.Context, id int64) error {
	tx, err := r.B.W.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var sigla string
	if err := tx.QueryRowContext(ctx, `SELECT und_sigla FROM Unidade_Medida WHERE und_codigo = ?`, id).Scan(&sigla); errors.Is(err, sql.ErrNoRows) {
		return regra("Unidade de medida não encontrada.")
	} else if err != nil {
		return err
	}
	uso, err := usoDaSigla(ctx, tx, sigla)
	if err != nil {
		return err
	}
	if uso > 0 {
		return regra("Não é possível excluir '%s': %d produto(s) usam esta unidade. Inative-a para ela deixar de ser oferecida.", sigla, uso)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM Unidade_Medida WHERE und_codigo = ?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

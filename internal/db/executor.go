package db

import (
	"context"
	"database/sql"
	"errors"
)

// Executor é o que as regras de negócio precisam de um banco: *sql.Tx e *sql.DB atendem.
type Executor interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// Rastro anota o maior código gerado por tabela dentro de uma transação. Se a transação for desfeita, QueimarCodigos faz o
// contador de cada tabela seguir adiante, como o IDENTITY do SQL Server do sistema antigo (que não volta atrás num rollback).
type Rastro map[string]int64

// Anotar guarda o código gerado numa inserção.
func (r Rastro) Anotar(tabela string, id int64) {
	if id > r[tabela] {
		r[tabela] = id
	}
}

// QueimarCodigo faz o próximo AUTOINCREMENT da tabela começar depois de id (o SQLite desfaz o contador junto com a transação).
func (b *Banco) QueimarCodigo(ctx context.Context, tabela string, id int64) {
	res, err := b.W.ExecContext(ctx, `UPDATE sqlite_sequence SET seq = ? WHERE name = ? AND seq < ?`, id, tabela, id)
	if err != nil {
		return
	}
	if n, _ := res.RowsAffected(); n > 0 {
		return
	}
	var existe int
	if err := b.W.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_sequence WHERE name = ?`, tabela).Scan(&existe); err == nil && existe == 0 {
		_, _ = b.W.ExecContext(ctx, `INSERT INTO sqlite_sequence (name, seq) VALUES (?, ?)`, tabela, id)
	}
}

// QueimarCodigos queima todos os códigos anotados no rastro (chamar DEPOIS do rollback).
func (b *Banco) QueimarCodigos(ctx context.Context, r Rastro) {
	for tabela, id := range r {
		b.QueimarCodigo(ctx, tabela, id)
	}
}

// Inserir executa um INSERT e anota o código gerado no rastro (que pode ser nil).
func Inserir(ctx context.Context, x Executor, r Rastro, tabela, query string, args ...any) (int64, error) {
	res, err := x.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	if r != nil {
		r.Anotar(tabela, id)
	}
	return id, nil
}

// ErrNaoEncontrado é devolvido por leituras de uma linha que não existe.
var ErrNaoEncontrado = errors.New("não encontrado")

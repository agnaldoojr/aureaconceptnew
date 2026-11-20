// Package db abre o SQLite (modernc.org/sqlite, sem CGO) com as regras do projeto (docs/DECISOES.md T-11, T-12):
// WAL, busy_timeout, foreign_keys, UMA conexão de escrita, pool só de leitura, collation AUREA_CIAI e migrations goose embutidas.
package db

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"sync"

	"github.com/pressly/goose/v3"
	"modernc.org/sqlite"

	"github.com/agnaldoojr/aureaconceptnew/migrations"
)

var registrar sync.Once

// registrarExtensoes registra (uma vez por processo) a collation e a função de normalização usadas pelo esquema.
func registrarExtensoes() error {
	var erro error
	registrar.Do(func() {
		if erro = sqlite.RegisterCollationUtf8("AUREA_CIAI", compararCIAI); erro != nil {
			return
		}
		erro = sqlite.RegisterDeterministicScalarFunction("aurea_norm", 1, func(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			if args[0] == nil {
				return nil, nil
			}
			switch v := args[0].(type) {
			case string:
				return Normalizar(v), nil
			case []byte:
				return Normalizar(string(v)), nil
			default:
				return fmt.Sprint(v), nil
			}
		})
	})
	return erro
}

// Banco agrupa a conexão de escrita (única) e o pool de leitura.
type Banco struct {
	W *sql.DB // escrita: MaxOpenConns(1)
	R *sql.DB // leitura: várias conexões somente leitura
}

// Fechar fecha as duas conexões.
func (b *Banco) Fechar() error {
	e1 := b.R.Close()
	e2 := b.W.Close()
	if e2 != nil {
		return e2
	}
	return e1
}

func dsn(caminho string, somenteLeitura bool) string {
	p := filepath.ToSlash(caminho)
	if !strings.HasPrefix(p, "/") && len(p) > 1 && p[1] == ':' { // C:/... no Windows
		p = "/" + p
	}
	q := url.Values{}
	q.Add("_pragma", "busy_timeout(5000)")
	q.Add("_pragma", "foreign_keys(1)")
	q.Add("_pragma", "synchronous(NORMAL)")
	if somenteLeitura {
		q.Set("mode", "ro")
	} else {
		q.Add("_pragma", "journal_mode(WAL)")
		q.Set("_txlock", "immediate")
	}
	return "file:" + p + "?" + q.Encode()
}

// Abrir abre (criando, se preciso) o banco em caminho e aplica as migrations pendentes.
func Abrir(ctx context.Context, caminho string) (*Banco, error) {
	if err := registrarExtensoes(); err != nil {
		return nil, fmt.Errorf("registrar extensões do SQLite: %w", err)
	}
	w, err := sql.Open("sqlite", dsn(caminho, false))
	if err != nil {
		return nil, err
	}
	w.SetMaxOpenConns(1)
	w.SetMaxIdleConns(1)
	if err := w.PingContext(ctx); err != nil {
		w.Close()
		return nil, fmt.Errorf("abrir %s: %w", caminho, err)
	}
	if err := Migrar(w); err != nil {
		w.Close()
		return nil, err
	}
	r, err := sql.Open("sqlite", dsn(caminho, true))
	if err != nil {
		w.Close()
		return nil, err
	}
	r.SetMaxOpenConns(4)
	if err := r.PingContext(ctx); err != nil {
		r.Close()
		w.Close()
		return nil, fmt.Errorf("abrir leitura de %s: %w", caminho, err)
	}
	return &Banco{W: w, R: r}, nil
}

// Migrar aplica as migrations embutidas pendentes (roda no boot). A versão fica na tabela goose_db_version.
func Migrar(w *sql.DB) error {
	goose.SetBaseFS(migrations.FS)
	goose.SetLogger(goose.NopLogger())
	if err := goose.SetDialect("sqlite3"); err != nil {
		return err
	}
	if err := goose.Up(w, "."); err != nil {
		return fmt.Errorf("migrations: %w", err)
	}
	return nil
}

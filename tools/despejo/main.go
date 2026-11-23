// Comando despejo: imprime as tabelas do SQLite (as de e2e/despejo-tabelas.json) em linhas "col|col|col" na MESMA forma que o
// e2e/despejo.mjs tira do SQL Server do sistema antigo, para o teste golden comparar o estado final do banco dos dois sistemas.
// SÓ PARA DESENVOLVIMENTO. Abre o arquivo somente para leitura.
//
//	go run ./tools/despejo -db caminho/aurea.db -tabelas e2e/despejo-tabelas.json
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/agnaldoojr/aureaconceptnew/internal/db"
)

func main() {
	caminho := flag.String("db", "", "arquivo do banco SQLite")
	tabelas := flag.String("tabelas", "e2e/despejo-tabelas.json", "especificação das tabelas")
	flag.Parse()
	if err := executar(*caminho, *tabelas); err != nil {
		fmt.Fprintln(os.Stderr, "erro:", err)
		os.Exit(1)
	}
}

func executar(caminho, espec string) error {
	b, err := os.ReadFile(espec)
	if err != nil {
		return err
	}
	var mapa map[string]json.RawMessage
	if err := json.Unmarshal(b, &mapa); err != nil {
		return err
	}
	banco, err := db.Abrir(context.Background(), caminho)
	if err != nil {
		return err
	}
	defer banco.Fechar()
	nomes := make([]string, 0, len(mapa))
	for n := range mapa {
		if !strings.HasPrefix(n, "_") {
			nomes = append(nomes, n)
		}
	}
	sort.Strings(nomes)
	for _, n := range nomes {
		var cols []string
		var onde, ordem string
		if err := json.Unmarshal(mapa[n], &cols); err != nil { // forma com filtro: {"onde": "...", "cols": [...]}
			var obj struct {
				Onde  string   `json:"onde"`
				Ordem string   `json:"ordem"`
				Cols  []string `json:"cols"`
			}
			if err := json.Unmarshal(mapa[n], &obj); err != nil {
				return err
			}
			cols, onde, ordem = obj.Cols, obj.Onde, obj.Ordem
		}
		if err := despejar(banco.R, n, cols, onde, ordem); err != nil {
			return err
		}
	}
	return nil
}

func despejar(r *sql.DB, tabela string, cols []string, onde, ordem string) error {
	if ordem == "" {
		ordem = strings.SplitN(cols[0], ":", 2)[0]
	}
	nomes := make([]string, len(cols))
	for i, c := range cols {
		nomes[i] = strings.SplitN(c, ":", 2)[0]
	}
	filtro := ""
	if onde != "" {
		filtro = " WHERE " + onde
	}
	rows, err := r.Query(fmt.Sprintf("SELECT %s FROM %s%s ORDER BY %s", strings.Join(nomes, ", "), tabela, filtro, ordem))
	if err != nil {
		return fmt.Errorf("%s: %w", tabela, err)
	}
	defer rows.Close()
	fmt.Printf("## %s\n", tabela)
	for rows.Next() {
		vals := make([]sql.NullString, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return err
		}
		partes := make([]string, len(cols))
		for i, v := range vals {
			if !v.Valid {
				partes[i] = "NULL"
			} else {
				partes[i] = v.String
			}
		}
		fmt.Println(strings.Join(partes, "|"))
	}
	return rows.Err()
}

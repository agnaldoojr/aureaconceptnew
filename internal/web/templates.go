package web

import (
	"fmt"
	"html/template"
	"io/fs"
	"path"
	"strings"

	ativos "github.com/agnaldoojr/aureaconceptnew/web"

	"github.com/agnaldoojr/aureaconceptnew/internal/permissoes"
)

// funcoes são as funções disponíveis nos templates.
func (s *Servidor) funcoes() template.FuncMap {
	return template.FuncMap{
		// asset("/css/style.css") → "/css/style-<hash>.css" (INF-08)
		"asset": s.est.URL,
		// pode(.Auth, "PRODUTOS_CONSULTA") = hasAuthority do Spring Security
		"pode": func(c permissoes.Conjunto, autoridade string) bool { return c[autoridade] },
		// podeAlgum(.Auth, "A", "B") = hasAnyAuthority
		"podeAlgum": func(c permissoes.Conjunto, autoridades ...string) bool {
			for _, a := range autoridades {
				if c[a] {
					return true
				}
			}
			return false
		},
	}
}

// carregarTemplates compila cada página junto com o layout base. login.html é independente (não usa o layout).

func (s *Servidor) carregarTemplates() error {
	s.paginas = map[string]*template.Template{}
	entradas, err := fs.ReadDir(ativos.Templates, "templates")
	if err != nil {
		return err
	}
	for _, e := range entradas {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".html") {
			continue
		}
		arquivos := []string{path.Join("templates", e.Name())}
		if e.Name() != "login.html" {
			arquivos = []string{"templates/layout/base.html", path.Join("templates", e.Name())}
		}
		t, err := template.New(e.Name()).Funcs(s.funcoes()).ParseFS(ativos.Templates, arquivos...)
		if err != nil {
			return fmt.Errorf("template %s: %w", e.Name(), err)
		}
		s.paginas[e.Name()] = t
	}
	return nil
}

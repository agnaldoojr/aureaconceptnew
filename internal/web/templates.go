package web

import (
	"fmt"
	"html/template"
	"io/fs"
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
		// dataBR("2026-11-20") = "20/11/2026"; texto vazio ou inválido volta vazio
		"dataBR": func(iso string) string {
			if len(iso) < 10 || iso[4] != '-' || iso[7] != '-' {
				return ""
			}
			return iso[8:10] + "/" + iso[5:7] + "/" + iso[0:4]
		},
		"contains": strings.Contains,
	}
}

// carregarTemplates compila cada página junto com o layout base (login.html é independente). As páginas ficam em
// web/templates/<área>/<nome>.html e são chamadas por esse caminho (ex.: "pessoas/lista.html").
func (s *Servidor) carregarTemplates() error {
	s.paginas = map[string]*template.Template{}
	return fs.WalkDir(ativos.Templates, "templates", func(caminho string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(caminho, ".html") || strings.HasPrefix(caminho, "templates/layout/") {
			return err
		}
		nome := strings.TrimPrefix(caminho, "templates/")
		arquivos := []string{caminho}
		if nome != "login.html" {
			arquivos = []string{"templates/layout/base.html", caminho}
		}
		t, err := template.New(nome).Funcs(s.funcoes()).ParseFS(ativos.Templates, arquivos...)
		if err != nil {
			return fmt.Errorf("template %s: %w", nome, err)
		}
		s.paginas[nome] = t
		return nil
	})
}

// Package ativos embute no binário os templates (html/template) e os arquivos estáticos do sistema:
// CSS, JS, imagens e PWA copiados do sistema antigo SEM alteração, e as bibliotecas locais (vendor: Font Awesome, Chart.js, fontes).
package ativos

import "embed"

// Templates guarda web/templates.
//
//go:embed templates
var Templates embed.FS

// Estaticos guarda web/static.
//
//go:embed static
var Estaticos embed.FS

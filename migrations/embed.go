// Package migrations embute os arquivos SQL versionados no binário (goose).
package migrations

import "embed"

// FS contém os arquivos *.sql desta pasta.
//
//go:embed *.sql
var FS embed.FS

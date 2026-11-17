// Package migrations embebe las migraciones SQL de goose en el binario.
package migrations

import "embed"

// FS contiene los archivos *.sql de migración.
//
//go:embed *.sql
var FS embed.FS

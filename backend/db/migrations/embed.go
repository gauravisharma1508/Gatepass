// Package migrations embeds the goose SQL migrations (one ordered sequence,
// file names carry the owning module).
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS

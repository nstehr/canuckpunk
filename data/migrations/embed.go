// Package migrations carries the schema so the migrate binary is
// self-contained.
package migrations

import "embed"

// Embed holds every goose migration, applied in filename order.
//
//go:embed *.sql
var Embed embed.FS

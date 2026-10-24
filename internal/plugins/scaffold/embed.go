package scaffold

import (
	"embed"
	"io/fs"
)

// embeddedTrees holds the starter projects of both languages: go/ and ts/ are
// each a project root. Every file carries a .tmpl suffix, because go:embed
// refuses a directory that holds a go.mod and the Go files would otherwise be
// compiled with the CLI. The all: prefix keeps the files whose names start with a
// dot.
//
//go:embed all:templates
var embeddedTrees embed.FS

// Embedded returns the starter projects that are part of the CLI: go/ and ts/ at
// the root of the file system, laid out the way Create reads a template tree.
func Embedded() fs.FS {
	// The directory is embedded above, so it exists and Sub cannot fail.
	sub, _ := fs.Sub(embeddedTrees, "templates")

	return sub
}

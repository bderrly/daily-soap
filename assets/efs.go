// Package assets provides embedded filesystem access to HTML templates and
// static files.
package assets

import (
	"embed"
	"io/fs"
)

//go:embed "html" "static"
var files embed.FS

var (
	// HTMLFiles contains the embedded HTML template files.
	HTMLFiles = sub(files, "html")
	// StaticFiles contains the embedded static asset files (CSS, JS, images).
	StaticFiles = sub(files, "static")
)

func sub(f embed.FS, dir string) fs.FS {
	sub, err := fs.Sub(f, dir)
	if err != nil {
		panic(err)
	}
	return sub
}

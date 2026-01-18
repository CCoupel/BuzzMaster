package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var webFS embed.FS

// GetEmbeddedFS returns the embedded web filesystem rooted at dist/
// Returns nil, false if no files are embedded
func GetEmbeddedFS() (fs.FS, bool) {
	// Check if embedded files exist
	entries, err := webFS.ReadDir("dist")
	if err != nil || len(entries) == 0 {
		return nil, false
	}

	subFS, err := fs.Sub(webFS, "dist")
	if err != nil {
		return nil, false
	}
	return subFS, true
}

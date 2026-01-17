package main

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var embeddedWebFS embed.FS

// GetEmbeddedWebFS returns the embedded web filesystem rooted at dist/
func GetEmbeddedWebFS() (fs.FS, bool) {
	// Check if embedded files exist
	entries, err := embeddedWebFS.ReadDir("dist")
	if err != nil || len(entries) == 0 {
		return nil, false
	}

	subFS, err := fs.Sub(embeddedWebFS, "dist")
	if err != nil {
		return nil, false
	}
	return subFS, true
}

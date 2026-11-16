// Package web embeds the built React app (web/dist is produced by `npm run build`).
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// FS returns the built frontend rooted at dist/.
func FS() fs.FS {
	f, err := fs.Sub(dist, "dist")
	if err != nil {
		return nil
	}
	return f
}

// Package assets embeds the trained policies and the precomputed benchmark.
package assets

import "embed"

//go:embed *.json
var FS embed.FS

func Read(name string) ([]byte, error) { return FS.ReadFile(name) }

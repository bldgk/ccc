// Package registry holds the shipped entries: one TOML file per tool.
package registry

import "embed"

// Files is this directory as it was at build time. Only the .toml files are entries.
//
//go:embed *
var Files embed.FS

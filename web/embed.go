// Package web embeds the built browser client (run `make web` first).
package web

import "embed"

// Dist holds the built client.
//
//go:embed all:dist
var Dist embed.FS

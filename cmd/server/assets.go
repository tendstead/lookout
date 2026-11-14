// SPDX-License-Identifier: Elastic-2.0
package main

import (
	"embed"
	"io/fs"
)

//go:embed all:static
var assets embed.FS

func frontendAssets() fs.FS {
	f, err := fs.Sub(assets, "static")
	if err != nil {
		panic(err)
	}
	return f
}

// Package web embeds the BuildFlow workbench so the executable is completely
// self contained: no Node.js, no build step and no files to install alongside
// the binary.
package web

import (
	"embed"
	"io/fs"
)

//go:embed index.html app.css app.js
var files embed.FS

// Assets returns the embedded web file system rooted at the interface files.
func Assets() fs.FS { return files }

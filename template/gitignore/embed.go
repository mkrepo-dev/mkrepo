package gitignore

import "embed"

//go:embed *.gitignore
var FS embed.FS

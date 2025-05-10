package docker

import "embed"

//go:embed *.Dockerfile.tmpl *.dockerignore
var FS embed.FS

package license

import "embed"

//go:embed config.yaml
var LicenseConfig []byte

//go:embed *.txt
var FS embed.FS

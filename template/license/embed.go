package license

import "embed"

//go:embed config.yaml
var LicenseConfig string

//go:embed *.txt
var FS embed.FS

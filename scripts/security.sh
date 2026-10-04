#!/usr/bin/env bash
set -euo pipefail

"$(dirname "$0")/build.sh"

go run golang.org/x/vuln/cmd/govulncheck@latest -mode binary bin/mkrepo

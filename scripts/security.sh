#!/usr/bin/env bash
set -euo pipefail

go run golang.org/x/vuln/cmd/govulncheck@latest -mode binary bin/mkrepo

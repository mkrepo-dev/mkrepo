#!/usr/bin/env bash
#set -euo pipefail

#source "$(dirname "$0")/vars.sh"
MODULE=github.com/mkrepo-dev/mkrepo

CGO_ENABLED=0 GOOS=linux go build -o bin/mkrepo -trimpath -ldflags="-buildid= -X $MODULE/internal.revision=$REVISION -X $MODULE/internal.version=$VERSION -X $MODULE/internal.buildDatetime=$DATETIME" ./cmd/mkrepo

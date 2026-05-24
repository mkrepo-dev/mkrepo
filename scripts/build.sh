#!/usr/bin/env bash

source "$(dirname "$0")/vars.sh"

CGO_ENABLED=0 GOOS=linux go build -o bin/$1 -trimpath -ldflags="-buildid= -X $MODULE/internal.revision=$REVISION -X $MODULE/internal.version=$VERSION -X $MODULE/internal.buildDatetime=$DATETIME" ./cmd/mkrepo

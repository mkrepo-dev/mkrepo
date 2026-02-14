#!/usr/bin/env bash

: "${MODULE:=github.com/mkrepo-dev/mkrepo}"
: "${DATETIME:=$(date -u +'%Y-%m-%dT%H:%M:%SZ')}"
: "${VERSION:=$(git describe --tags --abbrev=0 | sed 's/^v//')}"
: "${REVISION:=$(git describe --always --dirty --abbrev=7 --exclude='*')}"

: "${REGISTRY:=ghcr.io}"
: "${IMAGE:="$REGISTRY/mkrepo-dev/mkrepo"}"
: "${IMAGE_REF:="$IMAGE:$VERSION-$REVISION"}"

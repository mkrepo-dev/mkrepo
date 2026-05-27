#!/usr/bin/env bash

: "${MODULE:=github.com/mkrepo-dev/mkrepo}"
: "${VERSION:=$(git describe --tags --abbrev=0 | sed 's/^v//')}"
: "${REVISION:=$(git describe --always --dirty --abbrev=7 --exclude='*')}"

: "${REGISTRY:=ghcr.io}"
: "${IMAGE:="$REGISTRY/mkrepo-dev/mkrepo"}"
: "${IMAGE_REF:="$IMAGE:$VERSION-$REVISION"}"

if [ -n "$(git status --porcelain)" ]; then
    DATETIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
else
    DATETIME=$(git log -1 --format="%cI" --date iso HEAD | { read -r input && date -u -d "$input" +"%Y-%m-%dT%H:%M:%SZ"; } )
fi

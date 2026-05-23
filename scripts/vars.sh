#!/usr/bin/env bash

: "${MODULE:=github.com/mkrepo-dev/mkrepo}"
: "${DATETIME:=$(date -u +'%Y-%m-%dT%H:%M:%SZ')}"
: "${VERSION:=$(git describe --tags --abbrev=0 | sed 's/^v//')}"
: "${REVISION:=$(git describe --always --dirty --abbrev=7 --exclude='*')}"

: "${REGISTRY:=ghcr.io}"
: "${IMAGE:="$REGISTRY/mkrepo-dev/mkrepo"}"
: "${IMAGE_REF:="$IMAGE:$VERSION-$REVISION"}"

: "${DATETIME:=$( [ -z "$(git status --porcelain)" ] && git log -1 --format="%cI" --date=utc HEAD | sed 's/+00:00$/Z/' || date -u +"%Y-%m-%dT%H:%M:%SZ" )}"

echo $DATETIME

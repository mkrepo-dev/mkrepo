#!/usr/bin/env bash
set -euo pipefail

source "$(dirname "$0")/vars.sh"

docker push $IMAGE:latest $IMAGE_REF

if git describe --exact-match --tags >/dev/null 2>&1; then
  docker push $IMAGE:$VERSION
fi

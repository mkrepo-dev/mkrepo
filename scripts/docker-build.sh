#!/usr/bin/env bash
set -euo pipefail

source "$(dirname "$0")/vars.sh"

docker build \
  --build-arg VERSION=$VERSION \
  --build-arg REVISION=$REVISION \
  --build-arg BUILD_DATETIME=$DATETIME \
  --build-arg IMAGE_REF=$IMAGE_REF \
  -t $IMAGE:latest \
  -t $IMAGE_REF .

echo "Docker image built: $IMAGE:latest, $IMAGE_REF"

if git describe --exact-match --tags >/dev/null 2>&1; then
  docker tag $IMAGE_REF $IMAGE:$VERSION
  echo "Docker release image tagged as: $IMAGE:$VERSION"
fi

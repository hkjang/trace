#!/usr/bin/env sh
set -eu

requested_version="${1:-v$(tr -d '[:space:]' < VERSION)}"
case "$requested_version" in
  v*) release_version="$requested_version" ;;
  *) release_version="v$requested_version" ;;
esac
plain_version="${release_version#v}"
commit="$(git rev-parse --short HEAD 2>/dev/null || printf unknown)"
build_time="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
image="trace:$release_version"
archive="dist/trace-$release_version.tar.gz"

mkdir -p dist
docker build \
  --build-arg "VERSION=$plain_version" \
  --build-arg "COMMIT=$commit" \
  --build-arg "BUILD_TIME=$build_time" \
  --tag "$image" .
docker image save "$image" | gzip -9 > "$archive"
printf '%s\n' "$archive"

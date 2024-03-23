#!/bin/bash
set -e
cd $(dirname "$0")
START_TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
echo Starting build at $START_TIMESTAMP

echo Testing...
go test ./...

set +e
echo "Detecting version..."
if [[ $(git status --porcelain) ]]; then
  echo "  uncommitted changes; leaving version blank"
else
  TAG_VERSION=$(git describe --tags | grep ^v[0-9]\\+\\.[0-9]\\+\\.[0-9]\\+\$)
  if [ "$TAG_VERSION" != "" ]; then
    echo "  found version from git tag: $TAG_VERSION"
  else
    echo "  no git version tag found"
  fi
fi
set -e

echo Building...
go build -ldflags="-X \"main.Version=$TAG_VERSION\" -X \"main.BuildTimestamp=$START_TIMESTAMP\"" -o ./output/ .

echo "Done"

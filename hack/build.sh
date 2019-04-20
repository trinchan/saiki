#!/usr/bin/env bash
set -o nounset
set -o pipefail

VERSION=0.0.1

SHA=$(git rev-parse --short HEAD)
DIRTY=$(git status --porcelain 2> /dev/null)
if [[ -z "${DIRTY}" ]]; then
    BUILD_STATE=clean
else
    BUILD_STATE=dirty
fi
LDFLAGS="-X github.com/trinchan/saiki/pkg/build.Version=${VERSION}"
LDFLAGS="${LDFLAGS} -X github.com/trinchan/saiki/pkg/build.SHA=${SHA}"
LDFLAGS="${LDFLAGS} -X github.com/trinchan/saiki/pkg/build.BuildState=${BUILD_STATE}"

GOMODULE111=on go build \
 -mod=vendor \
 -installsuffix "static" \
 -ldflags "${LDFLAGS}"

#!/usr/bin/env bash
set -o nounset
set -o pipefail

UNFORMATTED=$(find . -name '*.go' ! -path '*/vendor/*' -exec gofmt -s -d {} \+)
if [ -n "$UNFORMATTED" ]; then
	echo "Run gofmt on the following files and try again."
	echo "$UNFORMATTED";
	exit 1;
fi;
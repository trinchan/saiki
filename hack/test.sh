#!/usr/bin/env bash

set -o nounset
set -o pipefail

go test -v -mod=vendor -race ./...

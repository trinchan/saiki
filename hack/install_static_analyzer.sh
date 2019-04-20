#!/usr/bin/env bash
set -o nounset
set -o pipefail

curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | sh -s -- -b $(go env GOPATH)/bin v1.16.0
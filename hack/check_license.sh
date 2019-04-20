#!/usr/bin/env bash
set -o nounset
set -o pipefail

UNLICENSED=()
for GOFILE in $(find . -name '*.go' ! -path '*/vendor/*')
do
    if ! head -n 2 $GOFILE | grep -q "Copyright"; then
        UNLICENSED+=($(echo $GOFILE | sed "s|^\./||"))
    fi
done

if [ ${#UNLICENSED[@]} -gt 0 ]; then
    printf '%s does not have a license header. Run make license to add it.\n' "${UNLICENSED[@]}"
    exit 1
fi
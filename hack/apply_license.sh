#!/usr/bin/env bash
set -o nounset
set -o pipefail

TEMP=$(mktemp)
trap "rm -f $TEMP; exit 1" 0 1 2 3 13 15

YEAR=$(date +"%Y")
read -d '' HEADER << Header
/*
Copyright $YEAR The saiki Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
Header

for GOFILE in $(find . -name '*.go' ! -path '*/vendor/*')
do
    if head -n 2 $GOFILE | grep -q "Copyright"; then
        continue
    fi
    echo "Applying license to: $GOFILE"
    {
    echo "$HEADER"
    echo ""
    cat $GOFILE
    } >| $TEMP
    mv $TEMP $GOFILE
done

rm -f $TEMP
trap 0
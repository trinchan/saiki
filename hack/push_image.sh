#!/usr/bin/env bash
set -o nounset
set -o pipefail

echo "$DOCKER_PASSWORD" | docker login -u "$DOCKER_USERNAME" --password-stdin registry.example.com
export REPO=trinchan/saiki
export COMMIT=${TRAVIS_COMMIT::8}
export TAG=`if [ "$TRAVIS_BRANCH" == "master" ]; then echo "latest"; else echo $TRAVIS_TAG ; fi`
docker build -f ../Dockerfile -t $REPO:$COMMIT ..
docker tag $REPO:$COMMIT $REPO:$TAG
docker push $REPO
#!/usr/bin/env bash
set -o nounset
set -o pipefail

export REGISTRY=quay.io
docker login -u="$DOCKER_USERNAME" -p="$DOCKER_PASSWORD" $REGISTRY
export REPO=$TRAVIS_REPO_SLUG
export COMMIT=${TRAVIS_COMMIT::8}
docker build -f Dockerfile -t $REPO:$COMMIT .
docker tag $REPO:$COMMIT $REGISTRY/$REPO:$TRAVIS_TAG
docker push $REGISTRY/$REPO:$TRAVIS_TAG
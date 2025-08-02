#!/bin/bash
set -e
VERSION=0.3.1
IMAGE=maintainer64/srl-labs-clabernetes
# Build launcher
docker build --no-cache -t $IMAGE-launcher:$VERSION -t $IMAGE-launcher:latest -f ./build/launcher.Dockerfile . --build-arg VERSION=$VERSION
docker push $IMAGE-launcher:$VERSION
docker push $IMAGE-launcher:latest
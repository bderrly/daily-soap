#!/bin/bash
# Build the container image and optionally push it to a repository.
#
# You should likely set VERSION when running this script.
# I recommend Scalable Calendar Versioning (https://scalver.org/).
# Example:
#   VERSION=0.20260101.0 ./hack/container_build.bash

set -eu

# Google Artifact Repository (GAR)
# Ensure the following key=value pairs are defined in .container_build.env at the root of the project repository:
#   - GAR_URL
#   - GAR_PROJECT
#   - GAR_REPO
#   - GAR_PKG
if [[ ! -f .container_build.env ]]; then
    >&2 echo ".container_build.env file not found, will not proceed"
    exit 1
fi
source .container_build.env

VERSION=${VERSION:=latest}

# Container label
FULL_LABEL="${GAR_URL}/${GAR_PROJECT}/${GAR_REPO}/${GAR_PKG}:${VERSION}"

podman build -t "${FULL_LABEL}" .

shopt -s nocasematch
if [[ ${PUSH_CONTAINER:-false} =~ ^(t(rue)?|y(es)?)$ ]]; then
    podman push "${FULL_LABEL}"
fi
shopt -u nocasematch

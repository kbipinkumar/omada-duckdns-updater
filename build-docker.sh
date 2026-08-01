#!/bin/bash
set -e

GIT_TAG=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
IMAGE_NAME="omada-duckdns-updater"

echo "Building Docker image ${IMAGE_NAME}:${GIT_TAG}..."

docker build \
    --build-arg VERSION="${GIT_TAG}" \
    -t "${IMAGE_NAME}:${GIT_TAG}" \
    .

# Optionally tag as latest if it's a clean tagged release
if [[ "${GIT_TAG}" != "dev" && "${GIT_TAG}" != *"-dirty"* ]]; then
    echo "Tagging as latest..."
    docker tag "${IMAGE_NAME}:${GIT_TAG}" "${IMAGE_NAME}:latest"
fi

echo "Docker build complete!"

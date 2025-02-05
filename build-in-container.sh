#!/bin/bash

# This script uses the latest Ubuntu 24.04 container to build the project with GoReleaser.  It emulates the GitHub Actions environment as closely as possible.
# Before submitting a PR for release/build. please run this script to ensure your PR will pass the build.

# Name of the container
CONTAINER_NAME="goreleaser-build"

# Directory where built binaries will be available
OUTPUT_DIR="$(pwd)/dist"

export GIT_STATE=$(if git diff-index --quiet HEAD --; then echo 'clean'; else echo 'dirty'; fi)
export BUILD_HOST=$(hostname)
export GO_VERSION=$(go version | awk '{print $3}')
export BUILD_USER=$(whoami)

# Start a new disposable Ubuntu 24.04 container with the current directory mounted
${CONTAINER_CMD:-docker} run --rm -it \
    --name "$CONTAINER_NAME" \
    -v "$(pwd)":/workspace \
    -w /workspace \
    ubuntu:24.04 bash -c "

    # Suppress timezone prompts
    export DEBIAN_FRONTEND=noninteractive
    export TZ=UTC


    # Update package lists and install dependencies
    apt update && apt install -y curl git gcc g++ make \
        gcc-aarch64-linux-gnu binutils-aarch64-linux-gnu \
        libc6-dev-arm64-cross software-properties-common

    # Install Go (match GitHub runner version)
    curl -fsSL https://golang.org/dl/go1.21.5.linux-amd64.tar.gz | tar -C /usr/local -xz
    export PATH=\$PATH:/usr/local/go/bin
    go version  # Verify Go installation

    # Set GOPATH and update PATH to include Go binaries
    export GOPATH=\$(go env GOPATH)
    export PATH=\$PATH:\$GOPATH/bin
    echo \"GOPATH: \$GOPATH\" && echo \"PATH: \$PATH\"``

    # Install Goreleaser
    curl -sL https://github.com/goreleaser/goreleaser/releases/latest/download/goreleaser_Linux_x86_64.tar.gz | tar -xz -C /usr/local/bin
    goreleaser --version  # Verify Goreleaser installation

    # Set Build Environment Variables
    export GIT_STATE="$GIT_STATE"
    export BUILD_HOST="$BUILD_HOST"
    export BUILD_USER="$BUILD_USER"
    export GO_VERSION=$(go version | awk '{print $3}')

    # Run Goreleaser
    goreleaser build --snapshot --clean
"

# Notify user of success
echo "✅ Build complete! Check the output in: $OUTPUT_DIR"


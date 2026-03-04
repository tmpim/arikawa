#!/usr/bin/env bash
# ABOUTME: Builds prebuilt libdave.a for all target platforms.
# ABOUTME: Run this script when updating the libdave submodule.
#
# Prerequisites:
#   macOS: brew install cmake ninja openssl
#   Linux builds: Docker with QEMU support (Docker Desktop covers this)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ARIKAWA_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

echo "==> Ensuring submodule is initialised..."
git -C "$ARIKAWA_ROOT" submodule update --init --recursive voice/dave/libdave

echo "==> Copying public header..."
mkdir -p "$SCRIPT_DIR/include/dave"
cp "$SCRIPT_DIR/libdave/cpp/includes/dave/dave.h" \
   "$SCRIPT_DIR/include/dave/dave.h"

# ---------------------------------------------------------------------------
# darwin/arm64 — native build (must run on Apple Silicon)
# ---------------------------------------------------------------------------
build_darwin_arm64() {
    echo "==> Building darwin/arm64 (native)..."
    make -C "$SCRIPT_DIR"
    mkdir -p "$SCRIPT_DIR/lib/darwin_arm64"
    cp "$SCRIPT_DIR/libdave/cpp/build/install/lib/libdave.a" \
       "$SCRIPT_DIR/lib/darwin_arm64/libdave.a"
    echo "==> darwin/arm64 done."
}

# ---------------------------------------------------------------------------
# linux — Docker build, cleans artifacts between platforms
# ---------------------------------------------------------------------------
build_linux() {
    local arch="$1"   # docker arch: amd64 or arm64
    local goarch="$2" # Go arch: amd64 or arm64
    local platform="linux/$arch"

    echo "==> Building $platform (Docker)..."

    # Clean previous build dir so the Linux build starts fresh.
    rm -rf "$SCRIPT_DIR/libdave/cpp/build" \
           "$SCRIPT_DIR/libdave/cpp/vcpkg"

    docker run --rm \
        --platform "$platform" \
        -v "$ARIKAWA_ROOT:/arikawa" \
        -w /arikawa/voice/dave \
        alpine:3 \
        sh -c "
            set -e
            apk add --no-cache \
                cmake ninja-build clang compiler-rt git pkgconf \
                curl zip unzip tar make ca-certificates \
                linux-headers musl-dev perl
            export PATH=/usr/lib/ninja-build/bin:\$PATH
            export CMAKE_POLICY_VERSION_MINIMUM=3.5
            export CC=clang CXX=clang++
            git config --global --add safe.directory /arikawa
            make
        "

    mkdir -p "$SCRIPT_DIR/lib/linux_$goarch"
    cp "$SCRIPT_DIR/libdave/cpp/build/install/lib/libdave.a" \
       "$SCRIPT_DIR/lib/linux_$goarch/libdave.a"

    echo "==> $platform done."
}

# Build macOS first (native), then Linux (Docker cleans the build dir).
build_darwin_arm64
build_linux amd64 amd64
build_linux arm64 arm64

echo ""
echo "All platforms built:"
ls -lh "$SCRIPT_DIR/lib/"*/libdave.a
ls -lh "$SCRIPT_DIR/include/dave/dave.h"
echo ""
echo "Next: git add voice/dave/lib voice/dave/include voice/dave/build-libdave.sh"
echo "      git commit -m 'build(dave): update prebuilt libdave binaries'"

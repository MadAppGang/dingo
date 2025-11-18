#!/bin/bash
# Build Dingo binaries for multiple platforms

set -e

VERSION="v0.3.0"
BUILD_DIR="release/${VERSION}"

echo "🐕 Building Dingo ${VERSION} for multiple platforms..."

# Create release directory
mkdir -p "${BUILD_DIR}"

# Build for different platforms
platforms=(
    "linux/amd64"
    "linux/arm64"
    "darwin/amd64"
    "darwin/arm64"
    "windows/amd64"
)

for platform in "${platforms[@]}"; do
    IFS='/' read -r GOOS GOARCH <<< "$platform"

    output_name="dingo-${VERSION}-${GOOS}-${GOARCH}"
    if [ "$GOOS" = "windows" ]; then
        output_name="${output_name}.exe"
    fi

    echo "  Building ${GOOS}/${GOARCH}..."

    GOOS=$GOOS GOARCH=$GOARCH go build \
        -ldflags="-X 'main.Version=${VERSION}' -X 'main.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)'" \
        -o "${BUILD_DIR}/${output_name}" \
        ./cmd/dingo

    echo "  ✓ ${output_name}"
done

echo ""
echo "✅ Build complete! Binaries in: ${BUILD_DIR}/"
echo ""
ls -lh "${BUILD_DIR}/"

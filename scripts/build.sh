#!/bin/bash
set -e

echo ""
echo "============================================================"
echo "  Any2Claude (Go) - Build (zero dependencies)"
echo "============================================================"
echo ""

# Navigate to project root
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "${SCRIPT_DIR}/.."
PROJECT_ROOT="$(pwd)"
BUILD_PKG="./cmd/any2claude"

# Check Go
if ! command -v go &> /dev/null; then
    echo "[ERROR] Go not found."
    echo ""
    if [[ "$OSTYPE" == "darwin"* ]]; then
        echo "  Install via Homebrew:  brew install go"
        echo "  Or download from:      https://go.dev/dl/"
    else
        echo "  Install via package manager:"
        echo "    Ubuntu/Debian:  sudo apt install golang-go"
        echo "    Fedora:         sudo dnf install golang"
        echo "    Arch:           sudo pacman -S go"
        echo "  Or download from:  https://go.dev/dl/"
    fi
    exit 1
fi
echo "[OK] Go $(go version | awk '{print $3}')"

# Detect OS
OS=$(uname -s)
ARCH=$(uname -m)
OUTPUT="Any2Claude"

echo "[OK] Platform: ${OS} / ${ARCH}"
echo ""
echo "Building executable (no network required)..."

go build -ldflags="-s -w" -o "${OUTPUT}" ${BUILD_PKG}

# Get file size (cross-platform)
if [[ "$OS" == "Darwin" ]]; then
    SIZE=$(stat -f%z "${OUTPUT}")
else
    SIZE=$(stat -c%s "${OUTPUT}")
fi

echo ""
echo "[OK] Build success!"
echo ""
echo "  Output: ${PROJECT_ROOT}/${OUTPUT}  (${SIZE} bytes)"
echo ""
echo "  Usage:"
echo "    ./Any2Claude"
echo "    Dashboard: http://127.0.0.1:8090"
echo "    Press Ctrl+C to quit."
echo ""

if [[ "$OS" == "Darwin" ]]; then
    echo "  Tip: Run in background:"
    echo "    nohup ./Any2Claude &"
    echo ""
fi

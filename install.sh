#!/usr/bin/env bash
set -euo pipefail

APP="commitai"
PREFIX="${PREFIX:-$HOME/.local}"
INSTALL_DIR="$PREFIX/bin"

cd "$(dirname "${BASH_SOURCE[0]}")"

if [[ "${1:-}" == "uninstall" ]]; then
    rm -f "$INSTALL_DIR/$APP"
    echo "Removed $INSTALL_DIR/$APP"
    echo "Your config in ~/.config/commitai was left untouched."
    exit 0
fi

echo ""
echo "  commitai installer"
echo ""

command -v go  >/dev/null 2>&1 || { echo "Error: Go is not installed (https://go.dev/dl/)."; exit 1; }
command -v git >/dev/null 2>&1 || { echo "Error: Git is not installed."; exit 1; }

VERSION="$(git describe --tags --always --dirty 2>/dev/null || echo dev)"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

echo "Building commitai $VERSION..."
go build -trimpath -ldflags "-s -w -X main.version=$VERSION" -o "$TMP/$APP" ./cmd/commitai

mkdir -p "$INSTALL_DIR"
install -m 0755 "$TMP/$APP" "$INSTALL_DIR/$APP"

echo ""
echo "Installed to $INSTALL_DIR/$APP"
echo ""
echo "Next:"
echo "    git add <files> && commitai"
echo "    commitai -h"
echo ""

case ":$PATH:" in
    *":$INSTALL_DIR:"*) ;;
    *)
        echo "Add this to your shell configuration (~/.bashrc, ~/.zshrc, or fish_add_path):"
        echo "    export PATH=\"$INSTALL_DIR:\$PATH\""
        echo ""
        ;;
esac
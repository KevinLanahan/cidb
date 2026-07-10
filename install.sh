#!/bin/sh
set -e

REPO="KevinLanahan/Lokal"
VERSION="v0.1.4"
INSTALL_DIR="/usr/local/bin"
BIN_NAME="lokal"

# Detect OS and arch
OS="$(uname -s)"
ARCH="$(uname -m)"

case "$OS" in
  Darwin)
    case "$ARCH" in
      arm64)  ASSET="lokal-mac-apple-silicon" ;;
      x86_64) ASSET="lokal-mac-intel" ;;
      *)      echo "Unsupported Mac architecture: $ARCH" && exit 1 ;;
    esac
    ;;
  Linux)
    case "$ARCH" in
      x86_64) ASSET="lokal-linux" ;;
      *)      echo "Unsupported Linux architecture: $ARCH" && exit 1 ;;
    esac
    ;;
  *)
    echo "Unsupported OS: $OS"
    echo "Download manually from https://github.com/$REPO/releases"
    exit 1
    ;;
esac

URL="https://github.com/$REPO/releases/download/$VERSION/$ASSET"

echo "Downloading lokal $VERSION ($ASSET)..."
curl -fsSL "$URL" -o "/tmp/$BIN_NAME"
chmod +x "/tmp/$BIN_NAME"

echo "Installing to $INSTALL_DIR/$BIN_NAME (may require sudo)..."
if [ -w "$INSTALL_DIR" ]; then
  mv "/tmp/$BIN_NAME" "$INSTALL_DIR/$BIN_NAME"
else
  sudo mv "/tmp/$BIN_NAME" "$INSTALL_DIR/$BIN_NAME"
fi

echo ""
echo "  lokal installed successfully!"
echo "  Run: lokal --help"
echo ""

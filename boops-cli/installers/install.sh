#!/usr/bin/env sh
set -eu

REPO="${BOOPS_REPO:-BooyahDev/BoopsDB}"
VERSION="${BOOPS_VERSION:-latest}"
INSTALL_DIR="${BOOPS_INSTALL_DIR:-}"

need_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "error: $1 is required" >&2
    exit 1
  fi
}

detect_os() {
  case "$(uname -s)" in
    Linux) echo "linux" ;;
    Darwin) echo "darwin" ;;
    *) echo "error: unsupported OS: $(uname -s)" >&2; exit 1 ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64) echo "amd64" ;;
    i386|i686) echo "386" ;;
    arm64|aarch64) echo "arm64" ;;
    armv7l|armv7) echo "armv7" ;;
    *) echo "error: unsupported architecture: $(uname -m)" >&2; exit 1 ;;
  esac
}

download() {
  url="$1"
  out="$2"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$url" -o "$out"
  elif command -v wget >/dev/null 2>&1; then
    wget -q "$url" -O "$out"
  else
    echo "error: curl or wget is required" >&2
    exit 1
  fi
}

default_install_dir() {
  if [ -w /usr/local/bin ]; then
    echo "/usr/local/bin"
  else
    echo "$HOME/.local/bin"
  fi
}

need_cmd uname
need_cmd tar
need_cmd mkdir

os="$(detect_os)"
arch="$(detect_arch)"
asset="boops-${os}-${arch}.tar.gz"

if [ "$VERSION" = "latest" ]; then
  url="https://github.com/${REPO}/releases/latest/download/${asset}"
else
  url="https://github.com/${REPO}/releases/download/${VERSION}/${asset}"
fi

if [ -z "$INSTALL_DIR" ]; then
  INSTALL_DIR="$(default_install_dir)"
fi

tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT INT TERM

archive="$tmp_dir/$asset"
echo "Downloading $url"
download "$url" "$archive"

tar -xzf "$archive" -C "$tmp_dir"
mkdir -p "$INSTALL_DIR"
install -m 0755 "$tmp_dir/boops" "$INSTALL_DIR/boops"

echo "Installed boops to $INSTALL_DIR/boops"
case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *) echo "Add $INSTALL_DIR to PATH to run: boops" ;;
esac

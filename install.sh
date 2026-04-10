#!/usr/bin/env bash
set -euo pipefail

REPO="LFroesch/scout"
BINARY_NAME="scout"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"

error() {
  echo "Error: $1" >&2
  exit 1
}

warn() {
  echo "Warning: $1" >&2
}

detect_platform() {
  local os arch

  case "$(uname -s)" in
    Linux*) os="linux" ;;
    Darwin*) os="darwin" ;;
    MINGW*|MSYS*|CYGWIN*) os="windows" ;;
    *) error "Unsupported OS: $(uname -s)" ;;
  esac

  case "$(uname -m)" in
    x86_64|amd64) arch="amd64" ;;
    aarch64|arm64) arch="arm64" ;;
    *) error "Unsupported architecture: $(uname -m)" ;;
  esac

  echo "${os}-${arch}"
}

get_latest_version() {
  curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest"     | sed -n 's/.*"tag_name": "\([^"]*\)".*/\1/p'     | head -n1
}

verify_checksum() {
  local file="$1"
  local checksums_file="$2"
  local expected

  expected="$(awk -v name="$(basename "$file")" '$2 == name { print $1 }' "$checksums_file")"
  if [ -z "$expected" ]; then
    warn "No checksum entry found for $(basename "$file"); skipping verification"
    return 0
  fi

  if command -v sha256sum >/dev/null 2>&1; then
    local actual
    actual="$(sha256sum "$file" | awk '{print $1}')"
    [ "$actual" = "$expected" ] || error "Checksum verification failed"
  elif command -v shasum >/dev/null 2>&1; then
    local actual
    actual="$(shasum -a 256 "$file" | awk '{print $1}')"
    [ "$actual" = "$expected" ] || error "Checksum verification failed"
  else
    warn "No SHA256 tool found; skipping checksum verification"
  fi
}

main() {
  local platform version binary_file base_url tmp_bin tmp_checksums

  platform="$(detect_platform)"
  version="${VERSION:-$(get_latest_version)}"
  [ -n "$version" ] || error "Unable to resolve release version"

  if [[ "$platform" == windows* ]]; then
    binary_file="${BINARY_NAME}-${platform}.exe"
  else
    binary_file="${BINARY_NAME}-${platform}"
  fi

  base_url="https://github.com/${REPO}/releases/download/${version}"

  tmp_dir="$(mktemp -d)"
  trap 'rm -rf "$tmp_dir"' EXIT

  tmp_bin="$tmp_dir/$binary_file"
  tmp_checksums="$tmp_dir/checksums.txt"

  curl -fsSL "${base_url}/${binary_file}" -o "$tmp_bin" || error "Failed to download ${binary_file}"
  if curl -fsSL "${base_url}/checksums.txt" -o "$tmp_checksums"; then
    verify_checksum "$tmp_bin" "$tmp_checksums"
  else
    warn "checksums.txt not found; skipping checksum verification"
  fi

  mkdir -p "$INSTALL_DIR"
  install -m 0755 "$tmp_bin" "$INSTALL_DIR/$BINARY_NAME"

  echo "Installed $BINARY_NAME to $INSTALL_DIR/$BINARY_NAME"
  if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
    warn "$INSTALL_DIR is not in PATH"
    warn "Add this to your shell config: export PATH=\"$PATH:$INSTALL_DIR\""
  fi
}

main

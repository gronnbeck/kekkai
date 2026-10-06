#!/usr/bin/env bash
set -euo pipefail

BIN_DIR="${BIN_DIR:-$HOME/.local/bin}"
REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

step() { printf '\n==> %s\n' "$*"; }
die() { printf 'error: %s\n' "$*" >&2; exit 1; }

step "Checking platform"
[[ "$(uname -s)" == Darwin ]] || die "Apple container runs on macOS only"
[[ "$(uname -m)" == arm64 ]] || die "Apple container needs Apple silicon"
macos_major="$(sw_vers -productVersion | cut -d. -f1)"
(( macos_major >= 26 )) || die "Apple container needs macOS 26 or later (found $(sw_vers -productVersion))"

step "Checking Homebrew"
if ! command -v brew >/dev/null; then
  /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
  eval "$(/opt/homebrew/bin/brew shellenv)"
fi

for formula in container go git; do
  if command -v "$formula" >/dev/null; then
    echo "$formula: $(command -v "$formula")"
  else
    step "Installing $formula"
    brew install "$formula"
  fi
done

step "Starting the container system service"
if container system status >/dev/null 2>&1; then
  echo "already running"
else
  container system start --enable-kernel-install || container system start
fi

step "Building kekkai into $BIN_DIR"
mkdir -p "$BIN_DIR"
(cd "$REPO_DIR" && go build -o "$BIN_DIR/kekkai" .)

case ":$PATH:" in
  *":$BIN_DIR:"*) ;;
  *) printf '\n%s is not on PATH. Add this to your shell profile:\n  export PATH="%s:$PATH"\n' "$BIN_DIR" "$BIN_DIR" ;;
esac

step "Done"
echo "Try: kekkai --kekkai-dry-run -p hi"

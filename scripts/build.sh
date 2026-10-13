#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
mkdir -p dist
version="$(tr -d '\n' < VERSION)"
go build -trimpath -ldflags "-X main.version=$version" -o dist/photo-organizer ./cmd/photo-organizer
if [[ "$(uname -s)" == "Darwin" ]]; then
  command -v clang >/dev/null || { echo 'Xcode Command Line Tools required: xcode-select --install' >&2; exit 1; }
  clang -fobjc-arc -mmacosx-version-min=12.0 -framework Foundation -framework ImageCaptureCore -framework CoreFoundation \
    -o dist/photo-organizer-import native/macos/import.m
else
  echo 'Skipping macOS ImageCaptureCore helper: this host is not macOS' >&2
fi

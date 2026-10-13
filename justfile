# Photo Organizer v0.3.0 (Go + macOS ImageCaptureCore USB importer)
set shell := ["bash", "-eu", "-c"]
source := env_var_or_default("HOME", ".") + "/Pictures/iPhone-Import"
destination := env_var_or_default("HOME", ".") + "/Pictures/iPhone-Archive"
import_folder := env_var_or_default("HOME", ".") + "/Pictures/iPhone-Import"
binary := "dist/photo-organizer"

default:
    @just --list

check:
    @go version
    @exiftool -ver
    @if [ "$(uname -s)" = Darwin ]; then xcrun clang --version | head -1; fi

deps:
    @command -v brew >/dev/null || { echo 'Homebrew required: https://brew.sh'; exit 1; }
    @command -v exiftool >/dev/null || brew install exiftool

build:
    ./scripts/build.sh

test:
    go test ./...
    go vet ./...

install:
    update-cli setup

ci:
    update-cli setup workflow ci

help-cli: build
    ./{{binary}} --help

# List media on a connected, trusted, unlocked iPhone; no writes.
import-dry-run: build
    ./{{binary}} import --destination "{{import_folder}}" --dry-run

# Copy iPhone photos and videos into a staging/import directory; keep originals.
import: build
    ./{{binary}} import --destination "{{import_folder}}"

# Dangerous: delete ONLY after batch copies pass local size checks.
# Requires two explicit confirmation flags and disables itself with iCloud Photos.
import-delete: build
    ./{{binary}} import --destination "{{import_folder}}" --delete-after-import --yes-delete

dry-run: build
    ./{{binary}} --source "{{source}}" --destination "{{destination}}" --dry-run

copy: build
    ./{{binary}} --source "{{source}}" --destination "{{destination}}" --copy

move: build
    ./{{binary}} --source "{{source}}" --destination "{{destination}}"

dry-run-separate: build
    ./{{binary}} --source "{{source}}" --destination "{{destination}}" --live-photos separate --dry-run

copy-separate: build
    ./{{binary}} --source "{{source}}" --destination "{{destination}}" --live-photos separate --copy

copy-skip-live: build
    ./{{binary}} --source "{{source}}" --destination "{{destination}}" --live-photos skip --copy

clean:
    rm -rf dist

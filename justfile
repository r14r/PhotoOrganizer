# iPhone Photo Organizer (Go)
set shell := ["bash", "-eu", "-c"]
source := env_var_or_default("HOME", ".") + "/Pictures/iPhone-Import"
destination := env_var_or_default("HOME", ".") + "/Pictures/iPhone-Archive"
binary := "dist/photo-organizer"

default:
    @just --list

check:
    @go version
    @exiftool -ver

deps:
    @command -v brew >/dev/null || { echo 'Homebrew required: https://brew.sh'; exit 1; }
    @command -v exiftool >/dev/null || brew install exiftool

# Local Go build, independent of update-cli.
build:
    mkdir -p dist
    go build -trimpath -ldflags "-X main.version=$(cat VERSION)" -o {{binary}} ./cmd/photo-organizer

test:
    go test ./...
    go vet ./...

# Install with the project update-cli.yaml manifest.
install:
    update-cli setup

ci:
    update-cli setup workflow ci

help-cli: build
    ./{{binary}} --help

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

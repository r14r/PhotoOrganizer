# Photo Organizer (Go + macOS iPhone USB Import)

Version **0.3.0**. Imports originals from a USB-connected iPhone via Apple's ImageCaptureCore, then optionally organizes them into date folders. No Python dependency.

## Requirements

- macOS 12+ and Xcode Command Line Tools (`xcode-select --install`) for compiling the USB importer
- Go 1.21+; ExifTool (`brew install exiftool`) for date-based organization
- `update-cli` for setup/install (`update-cli.yaml`); optionally `just` (`brew install just`)
- Connect and unlock the iPhone; accept **Trust This Computer** on first connection; close Image Capture/Photos if they hold the camera session

## Install

```sh
just deps
just test
just build    # builds dist/photo-organizer AND dist/photo-organizer-import on macOS
just install  # update-cli setup; installs both executables into /usr/local/bin
```

Alternatively run `update-cli setup` directly. The `/usr/local/bin` destination must be writable; installation does not invoke `sudo`. When running on Linux, only the Go CLI is built; USB import is supported **only on macOS**. If using Apple Silicon, `/opt/homebrew/bin` may be more convenient; edit the manifest if needed.

## iPhone -> Mac transfer

```sh
# List photos/video on a USB-connected iPhone, without modifying anything
photo-organizer import --destination "$HOME/Pictures/iPhone-Import" --dry-run

# Copy all available originals, preserve originals on iPhone (default)
photo-organizer import --destination "$HOME/Pictures/iPhone-Import"

# WARNING: also delete imported originals from iPhone after the entire batch verifies
photo-organizer import \
  --destination "$HOME/Pictures/iPhone-Import" \
  --delete-after-import --yes-delete

# With multiple cameras, select a specific one
photo-organizer import --device 'iPhone' --destination "$HOME/Pictures/iPhone-Import"
```

The import step copies originals to one flat folder. It retains paired Live Photo HEIC/MOV files and ordinary videos. `--dry-run` only lists files. Existing names are never overwritten: byte-identical files are reused (checked using SHA-256); different files get a `__N` suffix. A hidden `.photo-organizer-import-<UUID>` directory stages downloads before verification and movement into the output. Failed downloads can leave the staging folder for diagnosis.

**Deletion safeguards:** Deletion is **off by default**, requires `--delete-after-import` **and** `--yes-delete`, is forbidden in dry-run mode, and refuses to run when the iPhone reports iCloud Photos enabled. Deletion is requested only **after the entire enumerated batch downloads successfully and each local file's byte length matches the device's reported size**. The existing-file case additionally verifies SHA-256. Failed or partial imports never initiate deletion. Apple's camera API can still refuse deletion depending on device settings. Do not treat length checks as cryptographic verification against device originals; keep an independent backup before enabling deletion. The tool cannot restore deleted originals.

## Organize imported files by original capture date

```sh
# Preview
photo-organizer --source "$HOME/Pictures/iPhone-Import" --destination "$HOME/Pictures/iPhone-Archive" --live-photos separate --dry-run

# Copy without deleting the staged originals
photo-organizer --source "$HOME/Pictures/iPhone-Import" --destination "$HOME/Pictures/iPhone-Archive" --live-photos separate --copy
```

The existing `--live-photos keep|separate|skip`, `--format '%Y/%m'`, `--fallback mtime`, `--log`, and copy/move behavior remains available. The USB `import` and filesystem organizer are separate commands.

## Just recipes

`just`, `just check`, `just deps`, `just test`, `just build`, `just install`, `just ci`, `just import-dry-run`, `just import`, `just import-delete`, `just dry-run`, `just copy`, `just move`, `just dry-run-separate`, `just copy-separate`, `just copy-skip-live`, `just clean`.

## Project layout

```text
cmd/photo-organizer/main.go        Go CLI entry point
internal/importer/                 Go USB transport launcher and safety validation
native/macos/import.m              Native ImageCaptureCore transport for macOS
internal/organizer/                EXIF/QuickTime date organization, Live Photos
scripts/build.sh                   Go and macOS native build
update-cli.yaml                    build, test and install both binaries
justfile
VERSION
```

**Compatibility note:** Go tests and builds can run on Linux. The Objective-C ImageCaptureCore helper and real USB transfer/delete workflows require validation on a physical Mac and iPhone; they are not hardware-tested by this project. If macOS rejects iPhone access, import using the built-in **Image Capture / Digitale Bilder** application instead.

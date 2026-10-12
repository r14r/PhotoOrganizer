# iPhone Photo Organizer (Go)

A standalone Go CLI to organize original iPhone photo/video files by capture date. **Version 0.2.0.** The former Python implementation has been replaced; only ExifTool is required at runtime.

## Requirements

- macOS Intel or Apple Silicon (Go builds also work on Linux)
- Go 1.21+ to compile
- ExifTool (`brew install exiftool`) to read HEIC/EXIF/QuickTime metadata
- [`update-cli` by r14r](https://r14r.github.io/update-cli/) for automated build/test/install using `update-cli.yaml`
- Optional: `just` for developer aliases (`brew install just`)

## Build and install through update-cli

From the project root:

```sh
update-cli setup                # workflow setup -> install, including build and tests
update-cli setup workflow ci    # test, vet, build and verify without installing
```

The manifest installs the binary to `/usr/local/bin/photo-organizer`. This directory may need elevated write permissions; run in a suitably provisioned environment. You may also use `just install`.

For a local build without installing:

```sh
just build
./dist/photo-organizer --version
```

Equivalent direct Go build:

```sh
mkdir -p dist
go build -trimpath -ldflags "-X main.version=$(cat VERSION)" -o dist/photo-organizer ./cmd/photo-organizer
```

## Import photos

1. Connect iPhone to iMac using USB.
2. Open **Image Capture / Digitale Bilder**.
3. Import originals into `~/Pictures/iPhone-Import`.
4. Preserve the import directory until checking the generated archive.

## Run

Preview:

```sh
photo-organizer --source ~/Pictures/iPhone-Import --destination ~/Pictures/iPhone-Archive --live-photos separate --dry-run
```

Copy while keeping the imported originals:

```sh
photo-organizer --source ~/Pictures/iPhone-Import --destination ~/Pictures/iPhone-Archive --live-photos separate --copy --log ~/Pictures/photo-organizer.jsonl
```

Use `--live-photos keep` (default), `separate` (LivePhotos/ and Videos/ subfolders) or `skip` (ignore identified Live Photo MOV files); `--format '%Y/%m'` to organize by month; and `--fallback mtime` only when capture metadata is unavailable. **Without `--copy`, files are moved.** `--dry-run` never writes or moves files.

The Live Photo detector prefers Apple content identifiers, falling back to equal basenames plus a known short video duration. Identification cannot be guaranteed with all iPhone exports. The CLI does not modify image content and avoids overwriting occupied paths. Duplicates are identified by SHA-256 when the prospective target filename already exists. Video timezone metadata can be ambiguous. Check your original files before deleting anything from the iPhone.

## Developer commands

`just check`, `just deps`, `just build`, `just test`, `just install`, `just ci`, `just dry-run`, `just copy`, `just move`, `just dry-run-separate`, `just copy-separate`, `just copy-skip-live`, `just clean`.

## Layout

```text
cmd/photo-organizer/main.go       CLI entry point
internal/organizer/organizer.go   scanning, metadata, pairing, copy/move
internal/organizer/organizer_test.go
update-cli.yaml                   schema v2 setup/CI/install
justfile
VERSION
```

package main

import (
	"flag"
	"fmt"
	"github.com/r14r/photo-organizer/internal/importer"
	"github.com/r14r/photo-organizer/internal/organizer"
	"os"
)

var version = "dev"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "import" {
		importCommand(os.Args[2:])
		return
	}
	if len(os.Args) > 1 && (os.Args[1] == "help" || os.Args[1] == "--help") {
		fmt.Println("Usage: photo-organizer [legacy organizer flags] | import [options]\n\nImport directly from a connected iPhone: photo-organizer import --help")
		if os.Args[1] == "help" {
			return
		}
	}
	var opts organizer.Options
	flag.StringVar(&opts.Source, "source", "", "Input directory, recursively scanned")
	flag.StringVar(&opts.Destination, "destination", "", "Archive directory")
	flag.StringVar(&opts.Format, "format", "%Y/%Y-%m-%d", "Date folder pattern")
	flag.StringVar(&opts.Fallback, "fallback", "skip", "Missing metadata: skip or mtime")
	flag.StringVar(&opts.LivePhotos, "live-photos", "keep", "Live Photo MOV files: keep, separate or skip")
	flag.StringVar(&opts.ExifTool, "exiftool", "exiftool", "ExifTool executable path")
	flag.StringVar(&opts.Log, "log", "", "Write JSONL operation log")
	flag.BoolVar(&opts.DryRun, "dry-run", false, "Preview without changing files")
	flag.BoolVar(&opts.Copy, "copy", false, "Copy instead of moving")
	v := flag.Bool("version", false, "Print version")
	flag.Parse()
	if *v {
		fmt.Println(version)
		return
	}
	if opts.Source == "" || opts.Destination == "" {
		flag.Usage()
		os.Exit(2)
	}
	if e := organizer.Run(opts); e != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", e)
		os.Exit(1)
	}
}

func importCommand(args []string) {
	flags := flag.NewFlagSet("import", flag.ExitOnError)
	var opts importer.Options
	flags.StringVar(&opts.Destination, "destination", "", "Folder to receive originals from iPhone")
	flags.StringVar(&opts.Device, "device", "", "Connected device name (optional)")
	flags.BoolVar(&opts.DryRun, "dry-run", false, "List device media without copying or deleting")
	flags.BoolVar(&opts.DeleteAfterImport, "delete-after-import", false, "Delete from iPhone ONLY after all files verified locally")
	flags.BoolVar(&opts.ConfirmDelete, "yes-delete", false, "Mandatory second confirmation for iPhone deletion")
	flags.Parse(args)
	if err := importer.Run(opts, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		os.Exit(1)
	}
}

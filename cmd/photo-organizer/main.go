package main

import (
 "flag"
 "fmt"
 "github.com/r14r/photo-organizer/internal/organizer"
 "os"
)

var version = "dev"

func main() {
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
 if *v { fmt.Println(version); return }
 if opts.Source == "" || opts.Destination == "" { flag.Usage(); os.Exit(2) }
 if e := organizer.Run(opts); e != nil { fmt.Fprintln(os.Stderr, "ERROR:", e); os.Exit(1) }
}

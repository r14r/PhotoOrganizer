package organizer

import (
 "crypto/sha256"
 "encoding/hex"
 "encoding/json"
 "errors"
 "fmt"
 "io"
 "io/fs"
 "os"
 "os/exec"
 "path/filepath"
 "regexp"
 "sort"
 "strconv"
 "strings"
 "time"
)

var extensions = map[string]bool{".jpg": true, ".jpeg": true, ".heic": true, ".heif": true, ".png": true, ".dng": true, ".tif": true, ".tiff": true, ".mov": true, ".mp4": true, ".m4v": true, ".aae": true}
var dateKeys = []string{"SubSecDateTimeOriginal", "DateTimeOriginal", "CreationDate", "MediaCreateDate", "TrackCreateDate", "CreateDate"}
var dateRE = regexp.MustCompile(`^(\d{4})[:-]?(\d{2})[:-]?(\d{2})[ T](\d{2}):(\d{2}):(\d{2})`)
var durRE = regexp.MustCompile(`^\d+(?:\.\d+)?`)

type Options struct {
 Source, Destination, Format, Fallback, LivePhotos, ExifTool, Log string
 DryRun, Copy bool
 Output io.Writer
}
type Result struct { Operation, Source, Destination, DateSource, Error string `json:"-"` }
type logRecord struct {
 Operation string `json:"operation"`
 Source string `json:"source"`
 Destination *string `json:"destination"`
 DateSource string `json:"date_source"`
 Error string `json:"error,omitempty"`
}
type selectedDate struct { Date time.Time; Source string; Valid bool }
type fileEntry struct { Path string; Record map[string]any; Date selectedDate }

func isVideo(p string) bool {
 switch strings.ToLower(filepath.Ext(p)) {
 case ".mov", ".mp4", ".m4v": return true
 }
 return false
}
func isImage(p string) bool {
 return extensions[strings.ToLower(filepath.Ext(p))] && !isVideo(p) && strings.ToLower(filepath.Ext(p)) != ".aae"
}
func ParseDate(s string) (time.Time, bool) {
 m := dateRE.FindStringSubmatch(s)
 if m == nil { return time.Time{}, false }
 parts := make([]int, 6)
 for i := 1; i <= 6; i++ { parts[i-1], _ = strconv.Atoi(m[i]) }
 t := time.Date(parts[0], time.Month(parts[1]), parts[2], parts[3], parts[4], parts[5], 0, time.UTC)
 return t, t.Year() == parts[0] && int(t.Month()) == parts[1] && t.Day() == parts[2] && t.Hour() == parts[3] && t.Minute() == parts[4] && t.Second() == parts[5]
}
func field(record map[string]any, names ...string) string {
 for _, name := range names {
  keys := make([]string, 0, len(record))
  for k := range record { keys = append(keys, k) }
  sort.Strings(keys)
  for _, k := range keys {
   if k == name || strings.HasSuffix(k, ":"+name) {
    if s, ok := record[k].(string); ok && s != "" { return s }
   }
  }
 }
 return ""
}
func dateFor(record map[string]any, path, fallback string) selectedDate {
 for _, key := range dateKeys {
  if t, ok := ParseDate(field(record, key)); ok && t.Year() >= 1971 { return selectedDate{t, key, true} }
 }
 if fallback == "mtime" {
  if info, err := os.Stat(path); err == nil { return selectedDate{info.ModTime(), "filesystem mtime", true} }
 }
 return selectedDate{Source: "no capture metadata"}
}
func duration(record map[string]any) (float64, bool) {
 s := strings.TrimSpace(field(record, "Duration", "MediaDuration", "TrackDuration"))
 if s == "" { return 0, false }
 if strings.Contains(s, ":") {
  p := strings.Split(s, ":")
  if len(p) == 3 {
   a, e1 := strconv.ParseFloat(p[0], 64)
   b, e2 := strconv.ParseFloat(p[1], 64)
   c, e3 := strconv.ParseFloat(p[2], 64)
   if e1 == nil && e2 == nil && e3 == nil { return a*3600 + b*60 + c, true }
  }
 }
 if m := durRE.FindString(s); m != "" { v, e := strconv.ParseFloat(m, 64); return v, e == nil }
 return 0, false
}
func metadata(paths []string, executable string) (map[string]map[string]any, error) {
 result := map[string]map[string]any{}
 for start := 0; start < len(paths); start += 200 {
  end := start + 200
  if end > len(paths) { end = len(paths) }
  args := []string{"-json", "-G1", "-api", "QuickTimeUTC=0", "--"}
  args = append(args, paths[start:end]...)
  cmd := exec.Command(executable, args...)
  data, err := cmd.Output()
  if err != nil {
   var ee *exec.ExitError
   if !errors.As(err, &ee) || ee.ExitCode() != 1 { return nil, fmt.Errorf("exiftool: %w", err) }
  }
  var records []map[string]any
  if e := json.Unmarshal(data, &records); e != nil { return nil, fmt.Errorf("decode exiftool JSON: %w", e) }
  for _, rec := range records {
   if s, ok := rec["SourceFile"].(string); ok { abs, _ := filepath.Abs(s); result[abs] = rec }
  }
 }
 return result, nil
}
func inventory(source, dest string) ([]string, error) {
 paths := []string{}
 err := filepath.WalkDir(source, func(p string, d fs.DirEntry, err error) error {
  if err != nil { return err }
  if d.IsDir() {
   if p != source && (p == dest || strings.HasPrefix(d.Name(), ".")) { return filepath.SkipDir }
   return nil
  }
  if d.Type()&os.ModeSymlink != 0 { return nil }
  if extensions[strings.ToLower(filepath.Ext(p))] { paths = append(paths, p) }
  return nil
 })
 sort.Strings(paths)
 return paths, err
}
func pairs(files []*fileEntry) map[string]string {
 pairs := map[string]string{}
 byID := map[string][]string{}
 for _, f := range files {
  if isImage(f.Path) {
   if id := field(f.Record, "ContentIdentifier", "MediaGroupUUID"); id != "" { byID[id] = append(byID[id], f.Path) }
  }
 }
 for _, f := range files {
  if !isVideo(f.Path) { continue }
  if id := field(f.Record, "ContentIdentifier", "MediaGroupUUID"); id != "" {
   if len(byID[id]) == 1 { pairs[f.Path] = byID[id][0] }
   continue
  }
  var matches []string
  for _, candidate := range files {
   if isImage(candidate.Path) && filepath.Dir(candidate.Path) == filepath.Dir(f.Path) && strings.EqualFold(strings.TrimSuffix(filepath.Base(candidate.Path), filepath.Ext(candidate.Path)), strings.TrimSuffix(filepath.Base(f.Path), filepath.Ext(f.Path))) && field(candidate.Record, "ContentIdentifier", "MediaGroupUUID") == "" {
    matches = append(matches, candidate.Path)
   }
  }
  if n, ok := duration(f.Record); len(matches) == 1 && ok && n > 0 && n <= 5 { pairs[f.Path] = matches[0] }
 }
 return pairs
}
func folder(t time.Time, format string) (string, error) {
 replacements := strings.NewReplacer("%Y", t.Format("2006"), "%m", t.Format("01"), "%d", t.Format("02"), "%H", t.Format("15"), "%M", t.Format("04"), "%S", t.Format("05"), "%%", "%")
 if strings.Contains(format, "%") {
  for i := 0; i < len(format); i++ {
   if format[i] == '%' {
    if i+1 >= len(format) || !strings.ContainsRune("YmdHMS%", rune(format[i+1])) { return "", fmt.Errorf("unsupported date format token in %q", format) }
    i++
   }
  }
 }
 p := filepath.Clean(replacements.Replace(format))
 if filepath.IsAbs(p) || p == "." || p == ".." || strings.HasPrefix(p, ".."+string(os.PathSeparator)) { return "", fmt.Errorf("unsafe date folder %q", p) }
 return p, nil
}
func hashFile(path string) (string, error) {
 f, e := os.Open(path)
 if e != nil { return "", e }
 defer f.Close()
 h := sha256.New()
 if _, e = io.Copy(h, f); e != nil { return "", e }
 return hex.EncodeToString(h.Sum(nil)), nil
}
func target(base, src string, reserved map[string]string, hashes map[string]string) (string, string, error) {
 stem := strings.TrimSuffix(filepath.Base(src), filepath.Ext(src))
 ext := filepath.Ext(src)
 hash := ""
 for i := 0; i < 100000; i++ {
  name := filepath.Base(src)
  if i > 0 { name = fmt.Sprintf("%s__%d%s", stem, i, ext) }
  p := filepath.Join(base, name)
  known, booked := reserved[p]
  _, err := os.Stat(p)
  exists := err == nil
  if err != nil && !os.IsNotExist(err) { return "", "", err }
  if !booked && !exists { return p, "move", nil }
  if hash == "" {
   hash, err = hashFile(src)
   if err != nil { return "", "", err }
  }
  if exists && !booked {
   known = hashes[p]
   if known == "" {
    known, err = hashFile(p)
    if err != nil { return "", "", err }
    hashes[p] = known
   }
  }
  if known == hash { return "", "duplicate", nil }
 }
 return "", "", fmt.Errorf("too many filename collisions: %s", src)
}
func copyFile(src, dst string, info fs.FileInfo) error {
 in, e := os.Open(src)
 if e != nil { return e }
 defer in.Close()
 out, e := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
 if e != nil { return e }
 success := false
 defer func() { out.Close(); if !success { os.Remove(dst) } }()
 if _, e = io.Copy(out, in); e != nil { return e }
 if e = out.Sync(); e != nil { return e }
 if e = out.Close(); e != nil { return e }
 _ = os.Chtimes(dst, info.ModTime(), info.ModTime())
 success = true
 return nil
}
func moveFile(src, dst string) error {
 if e := os.Rename(src, dst); e == nil { return nil }
 info, e := os.Stat(src)
 if e != nil { return e }
 if e = copyFile(src, dst, info); e != nil { return e }
 return os.Remove(src)
}
func Run(o Options) error {
 if o.Output == nil { o.Output = os.Stdout }
 if o.Format == "" { o.Format = "%Y/%Y-%m-%d" }
 if o.Fallback == "" { o.Fallback = "skip" }
 if o.LivePhotos == "" { o.LivePhotos = "keep" }
 if o.ExifTool == "" { o.ExifTool = "exiftool" }
 if o.Fallback != "skip" && o.Fallback != "mtime" { return fmt.Errorf("invalid fallback %q", o.Fallback) }
 if o.LivePhotos != "keep" && o.LivePhotos != "separate" && o.LivePhotos != "skip" { return fmt.Errorf("invalid live photo option %q", o.LivePhotos) }
 if o.Source == "" || o.Destination == "" { return errors.New("source and destination required") }
 src, e := filepath.Abs(o.Source)
 if e != nil { return e }
 dst, e := filepath.Abs(o.Destination)
 if e != nil { return e }
 src = filepath.Clean(src); dst = filepath.Clean(dst)
 if src == dst { return errors.New("source and destination must differ") }
 if _, e = os.Stat(src); e != nil { return e }
 if e = validateFolder(o.Format); e != nil { return e }
 if _, e = exec.LookPath(o.ExifTool); e != nil { return fmt.Errorf("ExifTool not found: brew install exiftool: %w", e) }
 paths, e := inventory(src, dst)
 if e != nil { return e }
 status := "EXECUTE"
 if o.DryRun { status = "DRY RUN" }
 fmt.Fprintf(o.Output, "Found %d supported files. %s\n", len(paths), status)
 if len(paths) == 0 { return nil }
 records, e := metadata(paths, o.ExifTool)
 if e != nil { return e }
 entries := make([]*fileEntry, 0, len(paths))
 byPath := map[string]*fileEntry{}
 for _, p := range paths {
  rec := records[p]
  if rec == nil { rec = map[string]any{} }
  ent := &fileEntry{p, rec, dateFor(rec, p, o.Fallback)}
  entries = append(entries, ent); byPath[p] = ent
 }
 live := pairs(entries)
 for video, image := range live {
  if d := byPath[image].Date; d.Valid { byPath[video].Date = selectedDate{d.Date, "paired with Live Photo image", true} }
 }
 for _, f := range entries {
  if strings.ToLower(filepath.Ext(f.Path)) != ".aae" { continue }
  var matches []*fileEntry
  for _, img := range entries {
   if filepath.Dir(f.Path) == filepath.Dir(img.Path) && isImage(img.Path) && strings.EqualFold(strings.TrimSuffix(filepath.Base(f.Path), filepath.Ext(f.Path)), strings.TrimSuffix(filepath.Base(img.Path), filepath.Ext(img.Path))) { matches = append(matches, img) }
  }
  if len(matches) == 1 && matches[0].Date.Valid { f.Date = selectedDate{matches[0].Date.Date, "paired AAE", true} }
 }
 var log *os.File
 if o.Log != "" && !o.DryRun {
  logPath, e := filepath.Abs(o.Log)
  if e != nil { return e }
  if e = os.MkdirAll(filepath.Dir(logPath), 0755); e != nil { return e }
  log, e = os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
  if e != nil { return e }
  defer log.Close()
 }
 reserved := map[string]string{}
 hashes := map[string]string{}
 totals := map[string]int{}
 failed := false
 for _, f := range entries {
  op := ""
  var dest *string
  if _, yes := live[f.Path]; o.LivePhotos == "skip" && yes { op = "skip_live_photo" } else if !f.Date.Valid {
   op = "skip_no_date"
  } else {
   relative, e := folder(f.Date.Date, o.Format)
   if e != nil { return e }
   dir := filepath.Join(dst, relative)
   if o.LivePhotos == "separate" {
    if _, yes := live[f.Path]; yes { dir = filepath.Join(dir, "LivePhotos") } else if isVideo(f.Path) { dir = filepath.Join(dir, "Videos") }
   }
   if filepath.Clean(filepath.Dir(f.Path)) == dir {
    op = "already_organized"; s := f.Path; dest = &s
   } else {
    p, operation, e := target(dir, f.Path, reserved, hashes)
    if e != nil { return e }
    op = operation
    if p != "" {
     s := p; dest = &s
     h, e := hashFile(f.Path)
     if e != nil { return e }
     reserved[p] = h
    }
   }
  }
  totals[op]++
  suffix := ""
  if dest != nil { suffix = " -> " + *dest }
  fmt.Fprintf(o.Output, "%17s | %s%s [%s]\n", op, f.Path, suffix, f.Date.Source)
  record := logRecord{Operation: op, Source: f.Path, Destination: dest, DateSource: f.Date.Source}
  if !o.DryRun && op == "move" && dest != nil {
   e := os.MkdirAll(filepath.Dir(*dest), 0755)
   if e == nil {
    if o.Copy {
     info, statErr := os.Stat(f.Path)
     if statErr != nil { e = statErr } else { e = copyFile(f.Path, *dest, info) }
    } else { e = moveFile(f.Path, *dest) }
   }
   if e != nil {
    failed = true; totals["error"]++; record.Operation = "error"; record.Error = e.Error()
    fmt.Fprintf(os.Stderr, "ERROR: %s: %v\n", f.Path, e)
   } else if o.Copy { record.Operation = "copied" } else { record.Operation = "moved" }
  }
  if log != nil {
   data, _ := json.Marshal(record)
   if _, e := log.Write(append(data, '\n')); e != nil { return e }
  }
 }
 keys := make([]string, 0, len(totals))
 for k := range totals { keys = append(keys, k) }
 sort.Strings(keys)
 summary := []string{}
 for _, k := range keys { summary = append(summary, fmt.Sprintf("%s=%d", k, totals[k])) }
 fmt.Fprintln(o.Output, "\nSummary: "+strings.Join(summary, ", "))
 if failed { return errors.New("some file operations failed") }
 return nil
}
func validateFolder(pattern string) error {
 _, e := folder(time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC), pattern)
 return e
}

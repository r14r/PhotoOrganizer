package organizer

import (
 "os"
 "path/filepath"
 "strings"
 "testing"
 "time"
)

func TestParseDate(t *testing.T) {
 for _, s := range []string{"2026:10:09 13:14:15", "2026-10-09 13:14:15"} {
  d, ok := ParseDate(s)
  if !ok || d.Year() != 2026 || d.Day() != 9 { t.Fatalf("invalid %s", s) }
 }
 if _, ok := ParseDate("2026:02:30 12:00:00"); ok { t.Fatal("accepted invalid day") }
}
func TestDatePriority(t *testing.T) {
 rec := map[string]any{"QuickTime:CreateDate": "2020:01:01 01:00:00", "EXIF:DateTimeOriginal": "2026:09:18 10:20:30"}
 d := dateFor(rec, "", "skip")
 if !d.Valid || d.Date.Year() != 2026 { t.Fatalf("unexpected date: %+v", d) }
}
func TestLivePairs(t *testing.T) {
 items := []*fileEntry{
  {Path: "/photos/IMG_1.HEIC", Record: map[string]any{"MakerNotes:ContentIdentifier": "abc"}},
  {Path: "/photos/IMG_1.MOV", Record: map[string]any{"QuickTime:ContentIdentifier": "abc"}},
  {Path: "/photos/IMG_2.HEIC", Record: map[string]any{}},
  {Path: "/photos/IMG_2.MOV", Record: map[string]any{"QuickTime:Duration": "3.1"}},
  {Path: "/photos/IMG_3.MOV", Record: map[string]any{}},
 }
 p := pairs(items)
 if len(p) != 2 || p["/photos/IMG_1.MOV"] != "/photos/IMG_1.HEIC" || p["/photos/IMG_2.MOV"] != "/photos/IMG_2.HEIC" { t.Fatalf("unexpected pairs: %v", p) }
}
func TestTargetCollision(t *testing.T) {
 root := t.TempDir(); base := filepath.Join(root, "archive"); os.MkdirAll(base, 0755)
 first := filepath.Join(root, "A.HEIC"); second := filepath.Join(root, "B.HEIC")
 os.WriteFile(first, []byte("one"), 0644); os.WriteFile(second, []byte("two"), 0644)
 os.WriteFile(filepath.Join(base, "A.HEIC"), []byte("other"), 0644)
 p, op, e := target(base, first, map[string]string{}, map[string]string{})
 if e != nil || op != "move" || !strings.HasSuffix(p, "A__1.HEIC") { t.Fatalf("collision: %s %s %v", p, op, e) }
 _ = second
 os.WriteFile(filepath.Join(base, "A.HEIC"), []byte("one"), 0644)
 p, op, e = target(base, first, map[string]string{}, map[string]string{})
 if e != nil || p != "" || op != "duplicate" { t.Fatalf("duplicate: %s %s %v", p, op, e) }
}
func TestSafeFolder(t *testing.T) {
 d := time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC)
 p, e := folder(d, "%Y/%Y-%m-%d")
 if e != nil || p != "2026/2026-10-09" { t.Fatalf("folder %s %v", p, e) }
 for _, format := range []string{"../%Y", "/%Y", "%Q"} {
  if _, e = folder(d, format); e == nil { t.Fatalf("accepted unsafe %s", format) }
 }
}
func TestCopyDoesNotOverwrite(t *testing.T) {
 root := t.TempDir(); a := filepath.Join(root, "src"); b := filepath.Join(root, "dst")
 os.WriteFile(a, []byte("original"), 0644); os.WriteFile(b, []byte("existing"), 0644)
 info, _ := os.Stat(a)
 if e := copyFile(a, b, info); e == nil { t.Fatal("overwrote destination") }
 data, _ := os.ReadFile(b)
 if string(data) != "existing" { t.Fatal("destination changed") }
}

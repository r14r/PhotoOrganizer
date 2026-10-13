package importer

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDeleteGuard(t *testing.T) {
	cases := []struct {
		o     Options
		valid bool
	}{
		{Options{Destination: "/tmp/photos"}, true},
		{Options{Destination: "/tmp/photos", DeleteAfterImport: true}, false},
		{Options{Destination: "/tmp/photos", DeleteAfterImport: true, ConfirmDelete: true}, true},
		{Options{Destination: "/tmp/photos", DeleteAfterImport: true, ConfirmDelete: true, DryRun: true}, false},
		{Options{Destination: "/tmp/photos", ConfirmDelete: true}, false},
		{Options{}, false},
	}
	for _, c := range cases {
		if (Validate(c.o) == nil) != c.valid {
			t.Fatalf("validation %+v", c.o)
		}
	}
}
func TestHelperPath(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "photo-organizer")
	if _, e := HelperPath(binary); e == nil {
		t.Fatal("missing helper accepted")
	}
	helper := filepath.Join(dir, "photo-organizer-import")
	os.WriteFile(helper, []byte("#!/bin/sh\n"), 0755)
	if p, e := HelperPath(binary); e != nil || p != helper {
		t.Fatalf("helper %s %v", p, e)
	}
}
func TestRunHelperArguments(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX script")
	}
	dir := t.TempDir()
	script := filepath.Join(dir, "mock.sh")
	if e := os.WriteFile(script, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\"\n"), 0755); e != nil {
		t.Fatal(e)
	}
	var output bytes.Buffer
	o := Options{Destination: dir, Device: "My iPhone", DeleteAfterImport: true, ConfirmDelete: true}
	if e := RunHelper(o, script, &output, &output); e != nil {
		t.Fatal(e)
	}
	got := output.String()
	for _, expect := range []string{"--destination", dir, "--device", "My iPhone", "--delete-after-import", "--yes-delete"} {
		if !strings.Contains(got, expect) {
			t.Fatalf("missing %s in %s", expect, got)
		}
	}
	output.Reset()
	o = Options{Destination: dir, DryRun: true}
	if e := RunHelper(o, script, &output, &output); e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(output.String(), "--list") || strings.Contains(output.String(), "--yes-delete") {
		t.Fatal(output.String())
	}
}

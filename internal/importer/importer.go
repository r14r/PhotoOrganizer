// Package importer launches the macOS ImageCaptureCore transport helper.
// The organizer's filesystem import is intentionally independent of iOS APIs.
package importer

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

type Options struct {
	Destination       string
	Device            string
	DryRun            bool
	DeleteAfterImport bool
	ConfirmDelete     bool
}

func Validate(o Options) error {
	if o.Destination == "" {
		return errors.New("--destination is required")
	}
	if o.ConfirmDelete && !o.DeleteAfterImport {
		return errors.New("--yes-delete requires --delete-after-import")
	}
	if o.DeleteAfterImport && !o.ConfirmDelete {
		return errors.New("deleting originals requires both --delete-after-import and --yes-delete")
	}
	if o.DryRun && o.DeleteAfterImport {
		return errors.New("--dry-run and --delete-after-import cannot be combined")
	}
	return nil
}

// HelperPath locates the signed-or-locally-built transport next to the Go binary.
// This deliberately avoids PATH search for a helper permitted to delete photos.
func HelperPath(binary string) (string, error) {
	path := filepath.Join(filepath.Dir(binary), "photo-organizer-import")
	info, err := os.Stat(path)
	if err != nil || info.IsDir() || info.Mode()&0111 == 0 {
		return "", fmt.Errorf("macOS import helper missing or not executable at %s; run 'just build' or 'update-cli setup'", path)
	}
	return path, nil
}

func Run(o Options, out, stderr io.Writer) error {
	if err := Validate(o); err != nil {
		return err
	}
	if runtime.GOOS != "darwin" {
		return errors.New("USB iPhone import requires macOS (ImageCaptureCore)")
	}
	binary, err := os.Executable()
	if err != nil {
		return err
	}
	helper, err := HelperPath(binary)
	if err != nil {
		return err
	}
	return RunHelper(o, helper, out, stderr)
}

// RunHelper is separated for integration testing with a mock transport.
func RunHelper(o Options, helper string, out, stderr io.Writer) error {
	if err := Validate(o); err != nil {
		return err
	}
	dir, err := filepath.Abs(o.Destination)
	if err != nil {
		return err
	}
	args := []string{"--destination", dir}
	if o.Device != "" {
		args = append(args, "--device", o.Device)
	}
	if o.DryRun {
		args = append(args, "--list")
	}
	if o.DeleteAfterImport {
		args = append(args, "--delete-after-import", "--yes-delete")
	}
	cmd := exec.Command(helper, args...)
	cmd.Stdout, cmd.Stderr = out, stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("iPhone import failed: %w", err)
	}
	return nil
}

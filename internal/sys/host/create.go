//go:build darwin || linux

package host

import (
	"errors"
	"os"
	"path/filepath"
)

// CreateFile writes a new file. O_EXCL makes the kernel refuse a path that is taken, so
// a file that appeared between a check and this call is never replaced.
func (h *Host) CreateFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		// The file is this call's own, so a half-written one is taken away again.
		return errors.Join(err, file.Close(), os.Remove(path))
	}
	return file.Close()
}

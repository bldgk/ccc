//go:build darwin || linux

package host

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// The fewest levels below the root at which a directory may be removed.
const minRemovalDepth = 2

// RemoveDir deletes a directory tree. Removal cannot be undone, so every guard of
// checkRemoval has to hold first. The guard "the path is exactly what detection resolved"
// is split: the caller confirms the resolution, and this side accepts the path only in
// the exact form a resolution produces, never a form it would have to reinterpret.
func (h *Host) RemoveDir(path string) error {
	if err := h.checkRemoval(path); err != nil {
		return err
	}
	err := os.RemoveAll(path)
	if err == nil || !errors.Is(err, fs.ErrPermission) {
		return err
	}
	if werr := addWritePermission(path); werr != nil {
		return errors.Join(err, werr)
	}
	return os.RemoveAll(path)
}

func (h *Host) checkRemoval(path string) error {
	refuse := func(reason string) error {
		return fmt.Errorf("refusing to remove %q: %s", path, reason)
	}
	if !filepath.IsAbs(path) {
		return refuse("the path is not absolute")
	}
	if slices.Contains(strings.Split(path, string(filepath.Separator)), "..") {
		return refuse("the path contains ..")
	}
	if filepath.Clean(path) != path {
		return refuse("the path is not in its clean form")
	}
	home, err := h.home()
	if err != nil {
		return refuse("the home directory is unknown: " + err.Error())
	}
	if !filepath.IsAbs(home) {
		return refuse("the home directory is unknown: " + home + " is not absolute")
	}
	home = filepath.Clean(home)
	if err := CheckLocation(path, home); err != nil {
		return refuse(err.Error())
	}

	info, err := os.Lstat(path)
	if err != nil {
		return refuse(err.Error())
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return refuse("the path is a symbolic link")
	}
	if !info.IsDir() {
		return refuse("the path is not a directory")
	}

	// A symbolic link in a parent can make a harmless-looking path be the home directory
	// or something above it, so the location is checked once more as the kernel sees it.
	realPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return refuse(err.Error())
	}
	realHome, err := filepath.EvalSymlinks(home)
	if err != nil {
		return refuse("the home directory is unknown: " + err.Error())
	}
	for _, p := range []string{path, realPath} {
		for _, hm := range []string{home, realHome} {
			if err := CheckLocation(p, hm); err != nil {
				return refuse(err.Error())
			}
		}
	}
	return nil
}

// CheckLocation is the part of the removal guards that depends only on where a clean
// absolute path lies: it refuses the root, the home directory, a directory that contains
// the home directory, and a path too close to the root.
func CheckLocation(path, home string) error {
	root := string(filepath.Separator)
	if path == root {
		return errors.New("the path is the root")
	}
	if path == home {
		return errors.New("the path is the home directory")
	}
	if strings.HasPrefix(home, path+root) {
		return errors.New("the path contains the home directory")
	}
	depth := len(strings.Split(strings.TrimPrefix(path, root), root))
	if depth < minRemovalDepth {
		return fmt.Errorf("the path is less than %d levels below the root", minRemovalDepth)
	}
	return nil
}

// Only directories get the permission: deleting a file needs write access to the
// directory that holds it, and changing a file's mode would also change every other
// hard link to that file outside the tree.
func addWritePermission(root string) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		const ownerWrite = 0o200
		if info.Mode().Perm()&ownerWrite != 0 {
			return nil
		}
		return os.Chmod(path, info.Mode().Perm()|ownerWrite)
	})
}

//go:build darwin || linux

package host

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
)

// The unit of syscall.Stat_t.Blocks on both supported systems, whatever the block size
// of the file system is.
const statBlockSize = 512

type inode struct {
	dev uint64
	ino uint64
}

type usage struct {
	ctx   context.Context
	seen  map[inode]struct{}
	total int64
}

func (h *Host) DiskUsage(ctx context.Context, root string) (int64, error) {
	info, err := os.Lstat(root)
	if err != nil {
		return 0, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return 0, fmt.Errorf("%s is a symbolic link and is not followed", root)
	}
	if !info.IsDir() {
		return 0, fmt.Errorf("%s is not a directory", root)
	}
	u := &usage{ctx: ctx, seen: map[inode]struct{}{}}
	if err := u.add(root, info); err != nil {
		return 0, err
	}
	if err := u.walk(root); err != nil {
		return 0, err
	}
	return u.total, nil
}

func (u *usage) walk(dir string) error {
	if err := u.ctx.Err(); err != nil {
		return err
	}
	items, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, item := range items {
		path := filepath.Join(dir, item.Name())
		info, err := item.Info()
		// A file that was deleted between listing and looking at it takes no space.
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if err := u.add(path, info); err != nil {
			return err
		}
		if info.IsDir() {
			err := u.walk(path)
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (u *usage) add(path string, info fs.FileInfo) error {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("%s: the allocated size is not available", path)
	}
	if !info.IsDir() && st.Nlink > 1 {
		key := inode{dev: uint64(st.Dev), ino: uint64(st.Ino)}
		if _, counted := u.seen[key]; counted {
			return nil
		}
		u.seen[key] = struct{}{}
	}
	u.total += int64(st.Blocks) * statBlockSize
	return nil
}

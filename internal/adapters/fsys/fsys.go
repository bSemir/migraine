// Package fsys implements ports.MigrationFS on the local filesystem.
package fsys

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/bSemir/migraine/internal/core/domain"
)

// FS is the os-backed ports.MigrationFS.
type FS struct{}

// New returns a filesystem adapter rooted at the process working
// directory.
func New() FS {
	return FS{}
}

// List returns the base names of the regular files directly inside dir.
func (FS) List(ctx context.Context, dir string) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%w: %s (use --dir to point at it)", domain.ErrDirNotFound, dir)
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", dir, err)
	}
	var names []string
	for _, entry := range entries {
		if !entry.IsDir() {
			names = append(names, entry.Name())
		}
	}
	return names, nil
}

// Rename moves oldPath to newPath. It is a plain os.Rename rather than
// "git mv": git detects renames from content on its own, and this way
// untracked migrations work too.
func (FS) Rename(ctx context.Context, oldPath, newPath string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return os.Rename(oldPath, newPath)
}

// Exists reports whether path exists, without following symlinks.
func (FS) Exists(ctx context.Context, path string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	_, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

// Package ports declares the interfaces the core needs from the outside
// world. Adapters implement them; the app and planner only see these.
package ports

import (
	"context"

	"github.com/bSemir/migraine/internal/core/domain"
)

// GitRepo reads migration state from git history.
type GitRepo interface {
	// ResolveBase returns the ref to compare against. An empty request
	// means "main, falling back to master". It fails with
	// domain.ErrNotGitRepo or domain.ErrBaseNotFound.
	ResolveBase(ctx context.Context, requested string) (string, error)
	// ListFiles returns the base names of the files directly inside dir
	// as of ref. A dir that does not exist at ref yields no files.
	ListFiles(ctx context.Context, ref, dir string) ([]string, error)
}

// MigrationFS is the working-tree view of the migrations directory.
type MigrationFS interface {
	// List returns the base names of the regular files in dir. It fails
	// with domain.ErrDirNotFound if dir does not exist.
	List(ctx context.Context, dir string) ([]string, error)
	// Rename moves oldPath to newPath.
	Rename(ctx context.Context, oldPath, newPath string) error
	// Exists reports whether path exists.
	Exists(ctx context.Context, path string) (bool, error)
}

// NamingScheme translates between filenames and migration parts. It is
// the extension point for other tools (goose, sql-migrate, ...).
type NamingScheme interface {
	// Parse splits a base name into its parts. It returns
	// domain.ErrNotMigration for names outside the scheme and
	// domain.ErrTimestampVersion for timestamp-versioned names.
	Parse(filename string) (domain.Part, error)
	// Format is the inverse of Parse.
	Format(part domain.Part) string
}

package domain

import "errors"

// Sentinel errors. Adapters and use cases wrap these with context; the
// CLI matches them with errors.Is to choose an exit code.
var (
	// ErrNotGitRepo means the working directory is not inside a git
	// repository.
	ErrNotGitRepo = errors.New("not a git repository")
	// ErrBaseNotFound means the base ref does not resolve to a commit.
	ErrBaseNotFound = errors.New("base ref not found")
	// ErrDirNotFound means the migrations directory does not exist.
	ErrDirNotFound = errors.New("migrations directory not found")
	// ErrNotMigration means a filename does not follow the naming scheme.
	ErrNotMigration = errors.New("not a migration filename")
	// ErrTimestampVersion means a filename uses a timestamp instead of a
	// sequence number.
	ErrTimestampVersion = errors.New("timestamp versions not supported in v0.1")
	// ErrTargetExists means a rename would overwrite an existing file.
	ErrTargetExists = errors.New("rename target already exists")
)

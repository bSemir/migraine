// Package domain holds the tool-agnostic vocabulary of migraine:
// migrations, the problems found between two sets of them, and the
// rename plan that resolves those problems.
package domain

// Part is what a naming scheme extracts from a single migration filename.
type Part struct {
	// Version is the sequence number encoded in the filename.
	Version int
	// Width is the number of digits the version was written with,
	// including leading zeros.
	Width int
	// Title is the descriptive name between the version and the suffix.
	Title string
	// Suffix is everything after the title, e.g. ".up.sql".
	Suffix string
}

// Key identifies a migration independently of its files and padding.
type Key struct {
	Version int
	Title   string
}

// Migration is one logical migration: every file sharing a version and
// title, typically an up/down pair.
type Migration struct {
	// Version is the sequence number of the migration.
	Version int
	// Title is the descriptive name of the migration.
	Title string
	// Width is the zero-padding width its files use on disk.
	Width int
	// Files are the base names of the files that make up the migration.
	Files []string
}

// Key returns the identity of the migration.
func (m Migration) Key() Key {
	return Key{Version: m.Version, Title: m.Title}
}

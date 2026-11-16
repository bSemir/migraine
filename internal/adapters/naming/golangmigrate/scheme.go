// Package golangmigrate implements the golang-migrate naming scheme:
// {version}_{title}.{up|down}.sql with a zero-padded sequential version.
package golangmigrate

import (
	"fmt"
	"regexp"
	"strconv"

	"github.com/bSemir/migraine/internal/core/domain"
)

// timestampDigits is the shortest version treated as a timestamp. Unix
// seconds have 10 digits and YYYYMMDDHHMMSS has 14, while no realistic
// sequence reaches a billion migrations.
const timestampDigits = 10

var (
	leadingDigits = regexp.MustCompile(`^(\d+)_`)
	migrationName = regexp.MustCompile(`^(\d+)_(.+)(\.(?:up|down)\.sql)$`)
)

// Scheme is the golang-migrate ports.NamingScheme.
type Scheme struct{}

// New returns the golang-migrate naming scheme.
func New() Scheme {
	return Scheme{}
}

// Parse splits a filename such as 000042_add_users.up.sql into its parts.
func (Scheme) Parse(filename string) (domain.Part, error) {
	// Timestamps are checked before the full pattern so that files like
	// 20240101120000_x.sql are refused loudly instead of being skipped
	// as "not a migration" and silently left out of the analysis.
	if m := leadingDigits.FindStringSubmatch(filename); m != nil && len(m[1]) >= timestampDigits {
		return domain.Part{}, domain.ErrTimestampVersion
	}
	m := migrationName.FindStringSubmatch(filename)
	if m == nil {
		return domain.Part{}, domain.ErrNotMigration
	}
	version, err := strconv.Atoi(m[1])
	if err != nil {
		return domain.Part{}, fmt.Errorf("%w: %w", domain.ErrNotMigration, err)
	}
	return domain.Part{Version: version, Width: len(m[1]), Title: m[2], Suffix: m[3]}, nil
}

// Format renders a part back into a filename, zero-padding the version
// to the part's width.
func (Scheme) Format(p domain.Part) string {
	return fmt.Sprintf("%0*d_%s%s", p.Width, p.Version, p.Title, p.Suffix)
}

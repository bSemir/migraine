// Package planner contains the pure decision logic of migraine: grouping
// files into migrations, detecting problems and building a rename plan.
// Nothing in here performs I/O.
package planner

import (
	"cmp"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/bSemir/migraine/internal/core/domain"
	"github.com/bSemir/migraine/internal/core/ports"
)

// Compare orders migrations by version, then title.
func Compare(a, b domain.Migration) int {
	return cmp.Or(cmp.Compare(a.Version, b.Version), strings.Compare(a.Title, b.Title))
}

// Group parses filenames and bundles those sharing a version and title
// into one migration. Names outside the scheme are returned as ignored;
// any other parse failure (such as a timestamp version) aborts.
func Group(files []string, scheme ports.NamingScheme) (migrations []domain.Migration, ignored []string, err error) {
	byKey := map[domain.Key]domain.Migration{}
	for _, file := range files {
		part, err := scheme.Parse(file)
		if errors.Is(err, domain.ErrNotMigration) {
			ignored = append(ignored, file)
			continue
		}
		if err != nil {
			return nil, nil, fmt.Errorf("%q: %w", file, err)
		}
		key := domain.Key{Version: part.Version, Title: part.Title}
		m := byKey[key]
		m.Version, m.Title = part.Version, part.Title
		m.Width = max(m.Width, part.Width)
		m.Files = append(m.Files, file)
		byKey[key] = m
	}
	migrations = slices.SortedFunc(maps.Values(byKey), Compare)
	for _, m := range migrations {
		// Reverse order puts ".up" before ".down", which is how people
		// name the pair and how the output reads best.
		slices.SortFunc(m.Files, func(a, b string) int { return strings.Compare(b, a) })
	}
	return migrations, ignored, nil
}

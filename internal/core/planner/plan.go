package planner

import (
	"fmt"
	"slices"
	"strconv"

	"github.com/bSemir/migraine/internal/core/domain"
	"github.com/bSemir/migraine/internal/core/ports"
)

// BuildPlan renumbers the branch-local migrations so they follow the
// base branch's newest version, preserving their relative order.
// Migrations that already sit at their assigned number are left out.
func BuildPlan(base, ours []domain.Migration, scheme ports.NamingScheme) (domain.RenamePlan, error) {
	sorted := slices.SortedFunc(slices.Values(ours), Compare)
	next := BaseMax(base) + 1

	var plan domain.RenamePlan
	for i, from := range sorted {
		if from.Version == next+i {
			continue
		}
		to, err := renumber(from, next+i, scheme)
		if err != nil {
			return domain.RenamePlan{}, err
		}
		plan.Moves = append(plan.Moves, domain.Move{From: from, To: to})
	}
	return plan, nil
}

// renumber returns m at a new version. Padding is kept, and only grows
// when the new version no longer fits (999 -> 1000).
func renumber(m domain.Migration, version int, scheme ports.NamingScheme) (domain.Migration, error) {
	to := domain.Migration{
		Version: version,
		Title:   m.Title,
		Width:   max(m.Width, len(strconv.Itoa(version))),
		Files:   make([]string, len(m.Files)),
	}
	for i, file := range m.Files {
		part, err := scheme.Parse(file)
		if err != nil {
			return domain.Migration{}, fmt.Errorf("renumber %q: %w", file, err)
		}
		part.Version, part.Width = to.Version, to.Width
		to.Files[i] = scheme.Format(part)
	}
	return to, nil
}

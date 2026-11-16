package planner

import (
	"cmp"
	"slices"

	"github.com/bSemir/migraine/internal/core/domain"
)

// BaseMax returns the highest version on the base branch, or 0 if the
// base has no migrations.
func BaseMax(base []domain.Migration) int {
	if len(base) == 0 {
		return 0
	}
	newest := slices.MaxFunc(base, func(a, b domain.Migration) int {
		return cmp.Compare(a.Version, b.Version)
	})
	return newest.Version
}

// Ours returns the working-tree migrations that the base branch does not
// have. Identity is version+title rather than filename, so adding a
// missing .down.sql to a migration that is already on base does not turn
// that migration into "ours" and split the pair.
func Ours(base, working []domain.Migration) []domain.Migration {
	onBase := func(m domain.Migration) bool {
		return slices.ContainsFunc(base, func(b domain.Migration) bool { return b.Key() == m.Key() })
	}
	return slices.DeleteFunc(slices.Clone(working), onBase)
}

// DetectProblems reports every branch-local migration that collides with
// the base branch, sorts at or below its newest version, or shares a
// version with another local migration. Results are ordered by
// (version, title) of the offending migration.
func DetectProblems(base, ours []domain.Migration) []domain.Collision {
	sorted := slices.SortedFunc(slices.Values(ours), Compare)
	baseMax := BaseMax(base)

	var problems []domain.Collision
	for i, m := range sorted {
		sameVersion := func(o domain.Migration) bool { return o.Version == m.Version }

		if rivals := filter(base, sameVersion); len(rivals) > 0 {
			problems = append(problems, domain.Collision{Kind: domain.CollidesWithBase, Ours: m, Others: rivals})
		} else if m.Version <= baseMax {
			problems = append(problems, domain.Collision{Kind: domain.BelowBaseMax, Ours: m})
		}

		// A duplicated version is reported once, on the first migration
		// of its group, instead of once per pair.
		firstOfVersion := i == 0 || sorted[i-1].Version != m.Version
		if dups := filter(sorted[i+1:], sameVersion); firstOfVersion && len(dups) > 0 {
			problems = append(problems, domain.Collision{Kind: domain.LocalDuplicate, Ours: m, Others: dups})
		}
	}
	return problems
}

func filter(migrations []domain.Migration, keep func(domain.Migration) bool) []domain.Migration {
	return slices.DeleteFunc(slices.Clone(migrations), func(m domain.Migration) bool { return !keep(m) })
}

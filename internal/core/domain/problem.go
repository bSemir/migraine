package domain

// ProblemKind classifies what is wrong with a branch-local migration.
type ProblemKind int

const (
	// CollidesWithBase means the base branch already uses this version
	// for a different migration.
	CollidesWithBase ProblemKind = iota
	// BelowBaseMax means the version is unused on the base branch but
	// sorts at or before its newest migration, so it would be applied
	// out of order.
	BelowBaseMax
	// LocalDuplicate means two branch-local migrations share a version.
	LocalDuplicate
)

// Collision is a single problem found by the planner.
type Collision struct {
	// Kind says what sort of problem this is.
	Kind ProblemKind
	// Ours is the branch-local migration at fault.
	Ours Migration
	// Others are the migrations it clashes with: base migrations for
	// CollidesWithBase, local ones for LocalDuplicate, none otherwise.
	Others []Migration
}

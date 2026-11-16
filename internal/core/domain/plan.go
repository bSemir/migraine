package domain

// Rename moves a single file. Both names are base names inside the
// migrations directory.
type Rename struct {
	From string
	To   string
}

// Move renumbers one migration. To.Files is parallel to From.Files, so
// the files of a migration always travel together.
type Move struct {
	From Migration
	To   Migration
}

// Widened reports whether the new version needed more digits than the
// existing zero-padding provides.
func (m Move) Widened() bool {
	return m.To.Width > m.From.Width
}

// Renames lists the file renames that carry out the move.
func (m Move) Renames() []Rename {
	renames := make([]Rename, len(m.From.Files))
	for i, from := range m.From.Files {
		renames[i] = Rename{From: from, To: m.To.Files[i]}
	}
	return renames
}

// RenamePlan is the ordered set of moves that resolves every collision.
type RenamePlan struct {
	Moves []Move
}

// Empty reports whether the plan has nothing to do.
func (p RenamePlan) Empty() bool {
	return len(p.Moves) == 0
}

// Renames flattens the plan into individual file renames.
func (p RenamePlan) Renames() []Rename {
	var renames []Rename
	for _, m := range p.Moves {
		renames = append(renames, m.Renames()...)
	}
	return renames
}

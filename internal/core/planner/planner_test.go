package planner_test

import (
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/bSemir/migraine/internal/adapters/naming/golangmigrate"
	"github.com/bSemir/migraine/internal/core/domain"
	"github.com/bSemir/migraine/internal/core/planner"
)

var scheme = golangmigrate.New()

// group builds migrations from filenames, failing the test on bad input.
func group(t *testing.T, files ...string) []domain.Migration {
	t.Helper()
	migrations, ignored, err := planner.Group(files, scheme)
	if err != nil || len(ignored) > 0 {
		t.Fatalf("Group(%v): ignored=%v err=%v", files, ignored, err)
	}
	return migrations
}

// pair returns the up and down filenames of a 6-digit migration.
func pair(version int, title string) []string {
	return []string{
		fmt.Sprintf("%06d_%s.up.sql", version, title),
		fmt.Sprintf("%06d_%s.down.sql", version, title),
	}
}

func files(lists ...[]string) []string {
	return slices.Concat(lists...)
}

func TestGroup(t *testing.T) {
	got, ignored, err := planner.Group([]string{
		"000002_add_users.down.sql",
		"README.md",
		"000001_init.up.sql",
		"000002_add_users.up.sql",
		"000002_add_orders.up.sql",
	}, scheme)
	if err != nil {
		t.Fatal(err)
	}
	want := []domain.Migration{
		{Version: 1, Title: "init", Width: 6, Files: []string{"000001_init.up.sql"}},
		{Version: 2, Title: "add_orders", Width: 6, Files: []string{"000002_add_orders.up.sql"}},
		{Version: 2, Title: "add_users", Width: 6, Files: []string{"000002_add_users.up.sql", "000002_add_users.down.sql"}},
	}
	if !slices.EqualFunc(got, want, equal) {
		t.Errorf("migrations = %+v\nwant %+v", got, want)
	}
	if !slices.Equal(ignored, []string{"README.md"}) {
		t.Errorf("ignored = %v", ignored)
	}
}

func TestGroupRejectsTimestamps(t *testing.T) {
	_, _, err := planner.Group([]string{"000001_init.up.sql", "20240101120000_x.sql"}, scheme)
	if !errors.Is(err, domain.ErrTimestampVersion) {
		t.Fatalf("err = %v, want ErrTimestampVersion", err)
	}
}

func TestBaseMax(t *testing.T) {
	if got := planner.BaseMax(nil); got != 0 {
		t.Errorf("BaseMax(nil) = %d, want 0", got)
	}
	base := group(t, "000003_c.up.sql", "000010_j.up.sql", "000007_g.up.sql")
	if got := planner.BaseMax(base); got != 10 {
		t.Errorf("BaseMax = %d, want 10", got)
	}
}

func TestOurs(t *testing.T) {
	base := group(t, "000001_init.up.sql", "000002_add_users.up.sql")
	working := group(t, files(
		[]string{"000001_init.up.sql"},
		// A down file added to a migration that is already on base
		// must not make it ours.
		pair(2, "add_users"),
		pair(2, "add_orders"),
	)...)
	got := planner.Ours(base, working)
	if len(got) != 1 || got[0].Title != "add_orders" {
		t.Errorf("Ours = %+v, want only add_orders", got)
	}
}

func TestDetectProblemsAndBuildPlan(t *testing.T) {
	type problem struct {
		kind   domain.ProblemKind
		ours   string
		others []string
	}
	tests := []struct {
		name         string
		base         []string
		ours         []string
		wantProblems []problem
		wantRenames  []string
		wantWidened  bool
	}{
		{
			name: "no collision",
			base: pair(1, "init"),
			ours: pair(2, "add_orders"),
		},
		{
			name:         "exact collision",
			base:         files(pair(1, "init"), pair(2, "add_users")),
			ours:         pair(2, "add_orders"),
			wantProblems: []problem{{domain.CollidesWithBase, "add_orders", []string{"add_users"}}},
			wantRenames: []string{
				"000002_add_orders.up.sql -> 000003_add_orders.up.sql",
				"000002_add_orders.down.sql -> 000003_add_orders.down.sql",
			},
		},
		{
			name:         "below base max without exact collision",
			base:         []string{"000001_init.up.sql", "000003_add_users.up.sql"},
			ours:         []string{"000002_add_orders.up.sql"},
			wantProblems: []problem{{domain.BelowBaseMax, "add_orders", nil}},
			wantRenames:  []string{"000002_add_orders.up.sql -> 000004_add_orders.up.sql"},
		},
		{
			name: "multiple ours keep their order",
			base: []string{"000001_init.up.sql", "000002_a.up.sql", "000003_b.up.sql"},
			ours: []string{"000003_zebra.up.sql", "000002_yak.up.sql", "000004_ant.up.sql"},
			wantProblems: []problem{
				{domain.CollidesWithBase, "yak", []string{"a"}},
				{domain.CollidesWithBase, "zebra", []string{"b"}},
			},
			wantRenames: []string{
				"000002_yak.up.sql -> 000004_yak.up.sql",
				"000003_zebra.up.sql -> 000005_zebra.up.sql",
				"000004_ant.up.sql -> 000006_ant.up.sql",
			},
		},
		{
			name:         "chain where a target is another source",
			base:         []string{"000001_init.up.sql", "000002_a.up.sql"},
			ours:         []string{"000002_x.up.sql", "000003_y.up.sql"},
			wantProblems: []problem{{domain.CollidesWithBase, "x", []string{"a"}}},
			wantRenames: []string{
				"000002_x.up.sql -> 000003_x.up.sql",
				"000003_y.up.sql -> 000004_y.up.sql",
			},
		},
		{
			name:         "up only",
			base:         files(pair(1, "init"), pair(2, "add_users")),
			ours:         []string{"000002_add_orders.up.sql"},
			wantProblems: []problem{{domain.CollidesWithBase, "add_orders", []string{"add_users"}}},
			wantRenames:  []string{"000002_add_orders.up.sql -> 000003_add_orders.up.sql"},
		},
		{
			name:         "local duplicate versions",
			base:         pair(1, "init"),
			ours:         files(pair(2, "beta"), pair(2, "alpha"), pair(2, "gamma")),
			wantProblems: []problem{{domain.LocalDuplicate, "alpha", []string{"beta", "gamma"}}},
			wantRenames: []string{
				"000002_beta.up.sql -> 000003_beta.up.sql",
				"000002_beta.down.sql -> 000003_beta.down.sql",
				"000002_gamma.up.sql -> 000004_gamma.up.sql",
				"000002_gamma.down.sql -> 000004_gamma.down.sql",
			},
		},
		{
			name: "local duplicate that also collides with base",
			base: []string{"000001_init.up.sql"},
			ours: []string{"000001_a.up.sql", "000001_b.up.sql"},
			wantProblems: []problem{
				{domain.CollidesWithBase, "a", []string{"init"}},
				{domain.LocalDuplicate, "a", []string{"b"}},
				{domain.CollidesWithBase, "b", []string{"init"}},
			},
			wantRenames: []string{
				"000001_a.up.sql -> 000002_a.up.sql",
				"000001_b.up.sql -> 000003_b.up.sql",
			},
		},
		{
			name:         "padding preserved",
			base:         []string{"001_init.up.sql", "002_add_users.up.sql"},
			ours:         []string{"002_add_orders.up.sql"},
			wantProblems: []problem{{domain.CollidesWithBase, "add_orders", []string{"add_users"}}},
			wantRenames:  []string{"002_add_orders.up.sql -> 003_add_orders.up.sql"},
		},
		{
			name:         "padding widens at 999 -> 1000",
			base:         []string{"999_last.up.sql"},
			ours:         []string{"999_mine.up.sql", "999_mine.down.sql"},
			wantProblems: []problem{{domain.CollidesWithBase, "mine", []string{"last"}}},
			wantRenames: []string{
				"999_mine.up.sql -> 1000_mine.up.sql",
				"999_mine.down.sql -> 1000_mine.down.sql",
			},
			wantWidened: true,
		},
		{
			name: "empty base, nothing wrong",
			ours: files(pair(1, "init"), pair(2, "add_users")),
		},
		{
			name:         "empty base, local duplicate",
			ours:         []string{"000001_a.up.sql", "000001_b.up.sql"},
			wantProblems: []problem{{domain.LocalDuplicate, "a", []string{"b"}}},
			wantRenames:  []string{"000001_b.up.sql -> 000002_b.up.sql"},
		},
		{
			name: "nothing ours",
			base: pair(1, "init"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base, ours := group(t, tt.base...), group(t, tt.ours...)

			var gotProblems []problem
			for _, c := range planner.DetectProblems(base, ours) {
				gotProblems = append(gotProblems, problem{c.Kind, c.Ours.Title, titles(c.Others)})
			}
			if !slices.EqualFunc(gotProblems, tt.wantProblems, func(a, b problem) bool {
				return a.kind == b.kind && a.ours == b.ours && slices.Equal(a.others, b.others)
			}) {
				t.Errorf("problems = %+v\nwant %+v", gotProblems, tt.wantProblems)
			}

			plan, err := planner.BuildPlan(base, ours, scheme)
			if err != nil {
				t.Fatalf("BuildPlan: %v", err)
			}
			var gotRenames []string
			for _, r := range plan.Renames() {
				gotRenames = append(gotRenames, r.From+" -> "+r.To)
			}
			if !slices.Equal(gotRenames, tt.wantRenames) {
				t.Errorf("renames = %q\nwant %q", gotRenames, tt.wantRenames)
			}
			if plan.Empty() != (len(tt.wantRenames) == 0) {
				t.Errorf("Empty() = %v with %d renames", plan.Empty(), len(tt.wantRenames))
			}
			widened := slices.ContainsFunc(plan.Moves, domain.Move.Widened)
			if widened != tt.wantWidened {
				t.Errorf("widened = %v, want %v", widened, tt.wantWidened)
			}
		})
	}
}

// brokenScheme parses nothing, to exercise BuildPlan's error path.
type brokenScheme struct{ golangmigrate.Scheme }

func (brokenScheme) Parse(string) (domain.Part, error) {
	return domain.Part{}, domain.ErrNotMigration
}

func TestBuildPlanReportsUnparseableFiles(t *testing.T) {
	ours := []domain.Migration{{Version: 1, Title: "x", Width: 6, Files: []string{"000001_x.up.sql"}}}
	base := []domain.Migration{{Version: 1, Title: "init", Width: 6}}
	_, err := planner.BuildPlan(base, ours, brokenScheme{})
	if !errors.Is(err, domain.ErrNotMigration) {
		t.Fatalf("err = %v, want ErrNotMigration", err)
	}
}

func titles(migrations []domain.Migration) []string {
	var out []string
	for _, m := range migrations {
		out = append(out, m.Title)
	}
	return out
}

func equal(a, b domain.Migration) bool {
	return a.Key() == b.Key() && a.Width == b.Width && slices.Equal(a.Files, b.Files)
}

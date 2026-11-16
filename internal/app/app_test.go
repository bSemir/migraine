package app_test

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/bSemir/migraine/internal/adapters/naming/golangmigrate"
	"github.com/bSemir/migraine/internal/app"
	"github.com/bSemir/migraine/internal/core/domain"
)

const dir = "migrations"

var errBoom = errors.New("boom")

// fakeGit serves a fixed base branch.
type fakeGit struct {
	files      []string
	resolveErr error
	listErr    error
}

func (g fakeGit) ResolveBase(context.Context, string) (string, error) {
	return "main", g.resolveErr
}

func (g fakeGit) ListFiles(context.Context, string, string) ([]string, error) {
	return g.files, g.listErr
}

// memFS is an in-memory MigrationFS holding file names inside dir.
type memFS struct {
	files     []string
	listErr   error
	existsErr error
	// failRename makes the nth Rename call (1-based) fail; every call
	// from failAlways onward fails too, when set.
	failRename int
	failAlways int
	renames    int
}

func (f *memFS) List(context.Context, string) ([]string, error) {
	return slices.Clone(f.files), f.listErr
}

func (f *memFS) Exists(_ context.Context, path string) (bool, error) {
	return slices.Contains(f.files, filepath.Base(path)), f.existsErr
}

func (f *memFS) Rename(_ context.Context, oldPath, newPath string) error {
	f.renames++
	if f.renames == f.failRename || (f.failAlways > 0 && f.renames >= f.failAlways) {
		return errBoom
	}
	i := slices.Index(f.files, filepath.Base(oldPath))
	if i < 0 {
		return errors.New("no such file: " + oldPath)
	}
	f.files[i] = filepath.Base(newPath)
	return nil
}

func (f *memFS) sorted() []string {
	return slices.Sorted(slices.Values(f.files))
}

var (
	baseFiles = []string{
		"000001_init.up.sql", "000001_init.down.sql",
		"000002_add_users.up.sql", "000002_add_users.down.sql",
	}
	collidingTree = append(slices.Clone(baseFiles),
		"000002_add_orders.up.sql", "000002_add_orders.down.sql")
	fixedTree = []string{
		"000001_init.down.sql", "000001_init.up.sql",
		"000002_add_users.down.sql", "000002_add_users.up.sql",
		"000003_add_orders.down.sql", "000003_add_orders.up.sql",
	}
)

func newApp(git fakeGit, fs *memFS) *app.App {
	return app.New(git, fs, golangmigrate.New())
}

func TestCheckClean(t *testing.T) {
	fs := &memFS{files: append(slices.Clone(baseFiles), "000003_add_orders.up.sql", "README.md")}
	report, err := newApp(fakeGit{files: baseFiles}, fs).Check(t.Context(), app.Options{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Collisions) != 0 || !report.Plan.Empty() {
		t.Errorf("want a clean report, got %+v", report)
	}
	if report.Base != "main" || report.BaseMax != 2 {
		t.Errorf("base = %q max %d, want main max 2", report.Base, report.BaseMax)
	}
	if !slices.Equal(report.Ignored, []string{"README.md"}) {
		t.Errorf("ignored = %v", report.Ignored)
	}
}

func TestCheckLeavesGapsAlone(t *testing.T) {
	fs := &memFS{files: append(slices.Clone(baseFiles), "000007_add_orders.up.sql")}
	report, err := newApp(fakeGit{files: baseFiles}, fs).Check(t.Context(), app.Options{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Collisions) != 0 || !report.Plan.Empty() {
		t.Errorf("a gap is not a problem, got %+v", report)
	}
}

func TestCheckReportsCollisionWithoutTouchingFiles(t *testing.T) {
	fs := &memFS{files: slices.Clone(collidingTree)}
	report, err := newApp(fakeGit{files: baseFiles}, fs).Check(t.Context(), app.Options{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Collisions) != 1 || report.Collisions[0].Kind != domain.CollidesWithBase {
		t.Fatalf("collisions = %+v", report.Collisions)
	}
	if got := report.Collisions[0].Ours.Title; got != "add_orders" {
		t.Errorf("colliding migration = %q, want add_orders", got)
	}
	if len(report.Plan.Moves) != 1 || report.Plan.Moves[0].To.Version != 3 {
		t.Errorf("plan = %+v, want add_orders -> 3", report.Plan)
	}
	if report.Applied || fs.renames != 0 {
		t.Errorf("check must not rename (applied=%v renames=%d)", report.Applied, fs.renames)
	}
}

func TestFix(t *testing.T) {
	fs := &memFS{files: slices.Clone(collidingTree)}
	report, err := newApp(fakeGit{files: baseFiles}, fs).Fix(t.Context(), app.Options{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if !report.Applied {
		t.Error("Applied = false")
	}
	if got := fs.sorted(); !slices.Equal(got, fixedTree) {
		t.Errorf("files = %v\nwant %v", got, fixedTree)
	}
}

func TestFixDryRunChangesNothing(t *testing.T) {
	fs := &memFS{files: slices.Clone(collidingTree)}
	report, err := newApp(fakeGit{files: baseFiles}, fs).Fix(t.Context(), app.Options{Dir: dir, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if report.Applied || fs.renames != 0 {
		t.Errorf("dry run renamed files (applied=%v renames=%d)", report.Applied, fs.renames)
	}
	if report.Plan.Empty() {
		t.Error("dry run must still return the plan")
	}
}

func TestFixNothingToDo(t *testing.T) {
	fs := &memFS{files: slices.Clone(baseFiles)}
	report, err := newApp(fakeGit{files: baseFiles}, fs).Fix(t.Context(), app.Options{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if report.Applied || fs.renames != 0 {
		t.Errorf("nothing to fix, but applied=%v renames=%d", report.Applied, fs.renames)
	}
}

func TestFixRefusesToOverwrite(t *testing.T) {
	// A temp file left behind by a killed run occupies the name phase
	// one wants to use.
	fs := &memFS{files: append(slices.Clone(collidingTree), ".migraine-0.tmp")}
	report, err := newApp(fakeGit{files: baseFiles}, fs).Fix(t.Context(), app.Options{Dir: dir})
	if !errors.Is(err, domain.ErrTargetExists) {
		t.Fatalf("err = %v, want ErrTargetExists", err)
	}
	if report.Applied || fs.renames != 0 {
		t.Errorf("validation must come before any rename (renames=%d)", fs.renames)
	}
	if report.Plan.Empty() {
		t.Error("the report should still carry the plan that could not be applied")
	}
}

func TestFixRollsBackInMemory(t *testing.T) {
	// Four renames are planned (2 files x 2 phases); fail each in turn.
	for failAt := 1; failAt <= 4; failAt++ {
		fs := &memFS{files: slices.Clone(collidingTree), failRename: failAt}
		_, err := newApp(fakeGit{files: baseFiles}, fs).Fix(t.Context(), app.Options{Dir: dir})
		if !errors.Is(err, errBoom) {
			t.Fatalf("failAt=%d: err = %v, want boom", failAt, err)
		}
		if got, want := fs.sorted(), slices.Sorted(slices.Values(collidingTree)); !slices.Equal(got, want) {
			t.Errorf("failAt=%d: files = %v\nwant original %v", failAt, got, want)
		}
	}
}

func TestFixReportsFailedRollback(t *testing.T) {
	fs := &memFS{files: slices.Clone(collidingTree), failAlways: 3}
	_, err := newApp(fakeGit{files: baseFiles}, fs).Fix(t.Context(), app.Options{Dir: dir})
	if err == nil || !strings.Contains(err.Error(), "roll back") {
		t.Fatalf("err = %v, want it to mention the failed rollback", err)
	}
}

func TestErrorsAreWrappedWithContext(t *testing.T) {
	tests := []struct {
		name    string
		git     fakeGit
		fs      *memFS
		wantErr error
		wantMsg string
	}{
		{
			name:    "base not found",
			git:     fakeGit{resolveErr: domain.ErrBaseNotFound},
			fs:      &memFS{},
			wantErr: domain.ErrBaseNotFound,
			wantMsg: "resolve base",
		},
		{
			name:    "dir missing",
			git:     fakeGit{},
			fs:      &memFS{listErr: domain.ErrDirNotFound},
			wantErr: domain.ErrDirNotFound,
			wantMsg: "list working tree migrations",
		},
		{
			name:    "git listing fails",
			git:     fakeGit{listErr: errBoom},
			fs:      &memFS{},
			wantErr: errBoom,
			wantMsg: "list base migrations",
		},
		{
			name:    "timestamp in working tree",
			git:     fakeGit{files: baseFiles},
			fs:      &memFS{files: []string{"20240101120000_x.sql"}},
			wantErr: domain.ErrTimestampVersion,
			wantMsg: `"20240101120000_x.sql"`,
		},
		{
			name:    "timestamp on base",
			git:     fakeGit{files: []string{"20240101120000_x.up.sql"}},
			fs:      &memFS{},
			wantErr: domain.ErrTimestampVersion,
			wantMsg: "read base migrations",
		},
		{
			name:    "existence check fails",
			git:     fakeGit{files: baseFiles},
			fs:      &memFS{files: slices.Clone(collidingTree), existsErr: errBoom},
			wantErr: errBoom,
			wantMsg: "apply renames: check",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := newApp(tt.git, tt.fs).Fix(t.Context(), app.Options{Dir: dir})
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantMsg) {
				t.Errorf("err = %q, want it to contain %q", err, tt.wantMsg)
			}
		})
	}
}

// failingScheme parses once per file during grouping, then starts
// failing, which is the only way BuildPlan can error.
type failingScheme struct {
	golangmigrate.Scheme
	budget *int
}

func (s failingScheme) Parse(name string) (domain.Part, error) {
	if *s.budget--; *s.budget < 0 {
		return domain.Part{}, errBoom
	}
	return s.Scheme.Parse(name)
}

func TestPlanFailureIsReported(t *testing.T) {
	budget := len(baseFiles) + len(collidingTree)
	a := app.New(fakeGit{files: baseFiles}, &memFS{files: slices.Clone(collidingTree)}, failingScheme{budget: &budget})
	_, err := a.Check(t.Context(), app.Options{Dir: dir})
	if !errors.Is(err, errBoom) || !strings.Contains(err.Error(), "build plan") {
		t.Fatalf("err = %v, want a build plan failure", err)
	}
}

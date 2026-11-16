package app_test

import (
	"context"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"testing"

	"github.com/bSemir/migraine/internal/adapters/fsys"
	"github.com/bSemir/migraine/internal/adapters/naming/golangmigrate"
	"github.com/bSemir/migraine/internal/app"
)

// flakyFS is the real filesystem adapter with one injected failure: the
// nth Rename call fails, optionally cancelling the context first to
// mimic Ctrl-C arriving mid-apply.
type flakyFS struct {
	fsys.FS
	failAt int
	cancel context.CancelFunc
	calls  int
}

func (f *flakyFS) Rename(ctx context.Context, oldPath, newPath string) error {
	f.calls++
	if f.calls == f.failAt {
		if f.cancel != nil {
			f.cancel()
		}
		return errBoom
	}
	return f.FS.Rename(ctx, oldPath, newPath)
}

// seed writes files named after their own content, so a rename that
// lands on the wrong file is visible in the snapshot.
func seed(t *testing.T, names ...string) string {
	t.Helper()
	root := t.TempDir()
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(root, name), []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// snapshot maps each file name in root to its content.
func snapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{}
	for _, entry := range entries {
		content, err := os.ReadFile(filepath.Join(root, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		files[entry.Name()] = string(content)
	}
	return files
}

// chainBase and chainTree produce the plan 2->3, 3->4: the target of the
// first move is the source of the second.
var (
	chainBase = []string{"000001_init.up.sql", "000002_a.up.sql"}
	chainTree = []string{
		"000001_init.up.sql", "000002_a.up.sql",
		"000002_x.up.sql", "000002_x.down.sql",
		"000003_y.up.sql", "000003_y.down.sql",
	}
)

func TestFixOnDiskChainDoesNotClobber(t *testing.T) {
	root := seed(t, chainTree...)
	a := app.New(fakeGit{files: chainBase}, fsys.New(), golangmigrate.New())

	if _, err := a.Fix(t.Context(), app.Options{Dir: root}); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"000001_init.up.sql": "000001_init.up.sql",
		"000002_a.up.sql":    "000002_a.up.sql",
		"000003_x.up.sql":    "000002_x.up.sql",
		"000003_x.down.sql":  "000002_x.down.sql",
		"000004_y.up.sql":    "000003_y.up.sql",
		"000004_y.down.sql":  "000003_y.down.sql",
	}
	if got := snapshot(t, root); !maps.Equal(got, want) {
		t.Errorf("after fix:\n got %v\nwant %v", got, want)
	}
}

func TestFixOnDiskRollsBackAfterMidApplyFailure(t *testing.T) {
	// Four files, two phases: eight renames. Fail at every position.
	for failAt := 1; failAt <= 8; failAt++ {
		for _, cancelled := range []bool{false, true} {
			root := seed(t, chainTree...)
			before := snapshot(t, root)

			ctx, cancel := context.WithCancel(t.Context())
			fs := &flakyFS{failAt: failAt}
			if cancelled {
				fs.cancel = cancel
			}
			a := app.New(fakeGit{files: chainBase}, fs, golangmigrate.New())
			report, err := a.Fix(ctx, app.Options{Dir: root})
			cancel()

			if !errors.Is(err, errBoom) || report.Applied {
				t.Fatalf("failAt=%d: err = %v applied = %v, want boom and not applied", failAt, err, report.Applied)
			}
			if after := snapshot(t, root); !maps.Equal(after, before) {
				t.Errorf("failAt=%d cancelled=%v: directory not restored:\n got %v\nwant %v",
					failAt, cancelled, after, before)
			}
		}
	}
}

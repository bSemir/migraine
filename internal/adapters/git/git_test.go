package git_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"github.com/bSemir/migraine/internal/adapters/git"
	"github.com/bSemir/migraine/internal/core/domain"
)

// repo is a throwaway git repository for one test.
type repo struct {
	t   *testing.T
	dir string
}

// newRepo creates an empty repository on the given initial branch. The
// identity and config environment make it independent of the machine's
// git setup, so it behaves the same in CI.
func newRepo(t *testing.T, branch string) repo {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	for key, value := range map[string]string{
		"GIT_AUTHOR_NAME":     "Test",
		"GIT_AUTHOR_EMAIL":    "test@example.com",
		"GIT_COMMITTER_NAME":  "Test",
		"GIT_COMMITTER_EMAIL": "test@example.com",
		"GIT_CONFIG_GLOBAL":   os.DevNull,
		"GIT_CONFIG_SYSTEM":   os.DevNull,
	} {
		t.Setenv(key, value)
	}
	r := repo{t: t, dir: t.TempDir()}
	r.git("init", "--quiet", "--initial-branch", branch)
	return r
}

func (r repo) git(args ...string) {
	r.t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = r.dir
	if out, err := cmd.CombinedOutput(); err != nil {
		r.t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func (r repo) commit(message string, files ...string) {
	r.t.Helper()
	for _, file := range files {
		full := filepath.Join(r.dir, file)
		if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
			r.t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("-- "+file+"\n"), 0o600); err != nil {
			r.t.Fatal(err)
		}
	}
	r.git("add", ".")
	r.git("commit", "--quiet", "--message", message)
}

func TestResolveBase(t *testing.T) {
	tests := []struct {
		name      string
		branch    string
		requested string
		want      string
		wantErr   error
	}{
		{name: "defaults to main", branch: "main", want: "main"},
		{name: "falls back to master", branch: "master", want: "master"},
		{name: "explicit branch", branch: "main", requested: "release", want: "release"},
		{name: "explicit sha-like ref", branch: "main", requested: "HEAD", want: "HEAD"},
		{name: "no default branch", branch: "trunk", wantErr: domain.ErrBaseNotFound},
		{name: "explicit missing ref has no fallback", branch: "master", requested: "main", wantErr: domain.ErrBaseNotFound},
		{name: "option-like ref", branch: "main", requested: "--all", wantErr: domain.ErrBaseNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newRepo(t, tt.branch)
			r.commit("init", "migrations/000001_init.up.sql")
			r.git("branch", "release")

			got, err := git.New(r.dir).ResolveBase(t.Context(), tt.requested)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("base = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveBaseRemoteTrackingRef(t *testing.T) {
	upstream := newRepo(t, "main")
	upstream.commit("init", "migrations/000001_init.up.sql")

	clone := repo{t: t, dir: t.TempDir()}
	clone.git("clone", "--quiet", upstream.dir, ".")
	clone.git("checkout", "--quiet", "-b", "feature")
	clone.git("branch", "--quiet", "-D", "main")

	g := git.New(clone.dir)
	if _, err := g.ResolveBase(t.Context(), ""); !errors.Is(err, domain.ErrBaseNotFound) {
		t.Fatalf("default base err = %v, want ErrBaseNotFound", err)
	}
	got, err := g.ResolveBase(t.Context(), "origin/main")
	if err != nil || got != "origin/main" {
		t.Fatalf("ResolveBase(origin/main) = %q, %v", got, err)
	}
}

func TestResolveBaseOutsideRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	// Stop git from discovering a repository above the temp dir.
	dir := t.TempDir()
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(dir))

	_, err := git.New(dir).ResolveBase(t.Context(), "")
	if !errors.Is(err, domain.ErrNotGitRepo) {
		t.Fatalf("err = %v, want ErrNotGitRepo", err)
	}
}

func TestResolveBaseWithoutGitBinary(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, err := git.New(t.TempDir()).ResolveBase(t.Context(), "")
	if err == nil || errors.Is(err, domain.ErrNotGitRepo) {
		t.Fatalf("err = %v, want a failure to start git", err)
	}
}

func TestListFiles(t *testing.T) {
	r := newRepo(t, "main")
	r.commit("init",
		"migrations/000001_init.up.sql",
		"migrations/000001_init.down.sql",
		"migrations/archive/000000_old.up.sql",
		"migrations/has space.sql",
		"other/unrelated.sql",
	)
	r.git("checkout", "--quiet", "-b", "feature")
	r.commit("orders", "migrations/000002_add_orders.up.sql")
	// Uncommitted work must not leak into what a ref reports.
	if err := os.WriteFile(filepath.Join(r.dir, "migrations", "000003_wip.up.sql"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	g := git.New(r.dir)

	tests := []struct {
		name string
		ref  string
		dir  string
		want []string
	}{
		{"base branch", "main", "migrations", []string{"000001_init.down.sql", "000001_init.up.sql", "archive", "has space.sql"}},
		{"trailing slash", "main", "migrations/", []string{"000001_init.down.sql", "000001_init.up.sql", "archive", "has space.sql"}},
		{"other branch", "feature", "migrations", []string{"000001_init.down.sql", "000001_init.up.sql", "000002_add_orders.up.sql", "archive", "has space.sql"}},
		{"nested dir", "main", "migrations/archive", []string{"000000_old.up.sql"}},
		{"dir missing at ref", "main", "db/migrations", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := g.ListFiles(t.Context(), tt.ref, tt.dir)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("ListFiles = %q\nwant %q", got, tt.want)
			}
		})
	}
}

func TestListFilesUnknownRef(t *testing.T) {
	r := newRepo(t, "main")
	r.commit("init", "migrations/000001_init.up.sql")
	if _, err := git.New(r.dir).ListFiles(t.Context(), "no-such-ref", "migrations"); err == nil {
		t.Fatal("want an error for an unknown ref")
	}
}

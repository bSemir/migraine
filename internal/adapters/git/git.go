// Package git implements ports.GitRepo by shelling out to the git
// binary, which keeps migraine dependency-free and consistent with
// whatever git version and configuration the user already has.
package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/bSemir/migraine/internal/core/domain"
)

// defaultBases are tried in order when no base ref is requested.
var defaultBases = []string{"main", "master"}

// Repo is the exec-backed ports.GitRepo.
type Repo struct {
	dir string
}

// New returns a Repo that runs git in dir. An empty dir means the
// process working directory.
func New(dir string) *Repo {
	return &Repo{dir: dir}
}

// ResolveBase returns the ref to compare against: requested if given,
// otherwise main, falling back to master.
func (r *Repo) ResolveBase(ctx context.Context, requested string) (string, error) {
	if _, err := r.run(ctx, "rev-parse", "--git-dir"); err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			// git itself could not be started; don't mislabel that.
			return "", err
		}
		return "", fmt.Errorf("%w (run migraine from inside a repository)", domain.ErrNotGitRepo)
	}

	candidates := defaultBases
	if requested != "" {
		candidates = []string{requested}
	}
	i := slices.IndexFunc(candidates, func(ref string) bool { return r.isCommit(ctx, ref) })
	if i < 0 {
		return "", fmt.Errorf("%w: %s (try --base origin/main)",
			domain.ErrBaseNotFound, strings.Join(candidates, " or "))
	}
	return candidates[i], nil
}

// ListFiles returns the base names of the entries directly inside dir
// at ref. A dir that does not exist at ref yields an empty list: a
// branch that introduces the first migrations is a valid state.
func (r *Repo) ListFiles(ctx context.Context, ref, dir string) ([]string, error) {
	// The trailing slash makes ls-tree list the directory's children
	// instead of the directory entry itself.
	pathspec := strings.TrimSuffix(filepath.ToSlash(dir), "/") + "/"
	// -z avoids git's quoting of unusual filenames.
	out, err := r.run(ctx, "ls-tree", "-z", "--name-only", ref, "--", pathspec)
	if err != nil {
		return nil, err
	}
	var names []string
	for entry := range strings.SplitSeq(out, "\x00") {
		if entry != "" {
			names = append(names, path.Base(entry))
		}
	}
	return names, nil
}

func (r *Repo) isCommit(ctx context.Context, ref string) bool {
	// A ref starting with "-" would be read by git as an option.
	if strings.HasPrefix(ref, "-") {
		return false
	}
	_, err := r.run(ctx, "rev-parse", "--verify", "--quiet", ref+"^{commit}")
	return err == nil
}

func (r *Repo) run(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = r.dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}
	return string(out), nil
}

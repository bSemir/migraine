package app

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"

	"github.com/bSemir/migraine/internal/core/domain"
)

// step is one filesystem rename, with full paths.
type step struct {
	from string
	to   string
}

// apply carries out the renames. Nothing is touched unless every
// destination is free, and a failure part-way restores the original
// names.
func (a *App) apply(ctx context.Context, dir string, renames []domain.Rename) error {
	steps := twoPhase(dir, renames)
	if err := a.checkFree(ctx, dir, renames, steps); err != nil {
		return err
	}
	for i, s := range steps {
		if err := a.fs.Rename(ctx, s.from, s.to); err != nil {
			err = fmt.Errorf("rename %s -> %s: %w", s.from, s.to, err)
			return errors.Join(err, a.rollback(ctx, steps[:i]))
		}
	}
	return nil
}

// twoPhase orders the work as "every source to a unique temporary name,
// then every temporary name to its target". Renaming directly would
// break on chains: with 2->3 and 3->4 in one plan, the first rename
// would overwrite the file the second one still has to move.
func twoPhase(dir string, renames []domain.Rename) []step {
	park := make([]step, len(renames))
	place := make([]step, len(renames))
	for i, r := range renames {
		tmp := filepath.Join(dir, fmt.Sprintf(".migraine-%d.tmp", i))
		park[i] = step{from: filepath.Join(dir, r.From), to: tmp}
		place[i] = step{from: tmp, to: filepath.Join(dir, r.To)}
	}
	return slices.Concat(park, place)
}

// checkFree fails if any step would land on an existing file. Sources of
// the plan are exempt because phase one moves them out of the way first.
func (a *App) checkFree(ctx context.Context, dir string, renames []domain.Rename, steps []step) error {
	for _, s := range steps {
		isSource := slices.ContainsFunc(renames, func(r domain.Rename) bool {
			return filepath.Join(dir, r.From) == s.to
		})
		if isSource {
			continue
		}
		exists, err := a.fs.Exists(ctx, s.to)
		if err != nil {
			return fmt.Errorf("check %s: %w", s.to, err)
		}
		if exists {
			return fmt.Errorf("%w: %s", domain.ErrTargetExists, s.to)
		}
	}
	return nil
}

// rollback undoes completed steps in reverse order.
func (a *App) rollback(ctx context.Context, done []step) error {
	// The failure may itself be a cancellation (Ctrl-C mid-apply);
	// undoing must still run, or the directory is left half-renamed.
	ctx = context.WithoutCancel(ctx)
	var errs []error
	for _, s := range slices.Backward(done) {
		if err := a.fs.Rename(ctx, s.to, s.from); err != nil {
			errs = append(errs, fmt.Errorf("roll back %s -> %s: %w", s.to, s.from, err))
		}
	}
	return errors.Join(errs...)
}

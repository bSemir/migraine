// Package app implements migraine's use cases, Check and Fix, by
// orchestrating the ports and the pure planner. It never prints; callers
// render the returned Report.
package app

import (
	"context"
	"fmt"

	"github.com/bSemir/migraine/internal/core/domain"
	"github.com/bSemir/migraine/internal/core/planner"
	"github.com/bSemir/migraine/internal/core/ports"
)

// Options are the inputs shared by Check and Fix.
type Options struct {
	// Dir is the migrations directory, relative to the working directory.
	Dir string
	// Base is the ref to compare against. Empty means main, then master.
	Base string
	// DryRun makes Fix build the plan without renaming anything.
	DryRun bool
}

// Report is the outcome of a use case.
type Report struct {
	// Base is the ref the working tree was compared against.
	Base string
	// BaseMax is the highest migration version on Base.
	BaseMax int
	// Collisions are the problems found; empty means the tree is clean.
	Collisions []domain.Collision
	// Plan is the renumbering that resolves Collisions.
	Plan domain.RenamePlan
	// Ignored are files in Dir that do not follow the naming scheme.
	Ignored []string
	// Applied reports whether Plan was carried out on disk.
	Applied bool
}

// App wires the ports together.
type App struct {
	git    ports.GitRepo
	fs     ports.MigrationFS
	scheme ports.NamingScheme
}

// New returns an App using the given adapters.
func New(git ports.GitRepo, fs ports.MigrationFS, scheme ports.NamingScheme) *App {
	return &App{git: git, fs: fs, scheme: scheme}
}

// Check compares the working tree against the base ref and reports
// collisions together with the plan that would fix them.
func (a *App) Check(ctx context.Context, opts Options) (Report, error) {
	return a.analyze(ctx, opts)
}

// Fix runs Check and then applies the plan, unless opts.DryRun is set.
// On failure the report still describes what was found.
func (a *App) Fix(ctx context.Context, opts Options) (Report, error) {
	report, err := a.analyze(ctx, opts)
	if err != nil || opts.DryRun || report.Plan.Empty() {
		return report, err
	}
	if err := a.apply(ctx, opts.Dir, report.Plan.Renames()); err != nil {
		return report, fmt.Errorf("apply renames: %w", err)
	}
	report.Applied = true
	return report, nil
}

func (a *App) analyze(ctx context.Context, opts Options) (Report, error) {
	ref, err := a.git.ResolveBase(ctx, opts.Base)
	if err != nil {
		return Report{}, fmt.Errorf("resolve base: %w", err)
	}
	workingFiles, err := a.fs.List(ctx, opts.Dir)
	if err != nil {
		return Report{}, fmt.Errorf("list working tree migrations: %w", err)
	}
	baseFiles, err := a.git.ListFiles(ctx, ref, opts.Dir)
	if err != nil {
		return Report{}, fmt.Errorf("list base migrations: %w", err)
	}

	// Stray files on the base branch are not this branch's concern, so
	// only the working tree's ignored files are reported.
	base, _, err := planner.Group(baseFiles, a.scheme)
	if err != nil {
		return Report{}, fmt.Errorf("read base migrations: %w", err)
	}
	working, ignored, err := planner.Group(workingFiles, a.scheme)
	if err != nil {
		return Report{}, fmt.Errorf("read working tree migrations: %w", err)
	}

	report := Report{Base: ref, BaseMax: planner.BaseMax(base), Ignored: ignored}
	ours := planner.Ours(base, working)
	report.Collisions = planner.DetectProblems(base, ours)
	if len(report.Collisions) == 0 {
		// Gaps and other harmless oddities are left alone: migraine
		// only renumbers when something is actually broken.
		return report, nil
	}
	report.Plan, err = planner.BuildPlan(base, ours, a.scheme)
	if err != nil {
		return Report{}, fmt.Errorf("build plan: %w", err)
	}
	return report, nil
}

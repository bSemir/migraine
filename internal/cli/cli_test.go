package cli_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/bSemir/migraine/internal/app"
	"github.com/bSemir/migraine/internal/cli"
	"github.com/bSemir/migraine/internal/core/domain"
)

// stubService returns a canned report and records what it was asked.
type stubService struct {
	report app.Report
	err    error
	called string
	opts   app.Options
}

func (s *stubService) Check(_ context.Context, opts app.Options) (app.Report, error) {
	s.called, s.opts = "check", opts
	return s.report, s.err
}

func (s *stubService) Fix(_ context.Context, opts app.Options) (app.Report, error) {
	s.called, s.opts = "fix", opts
	report := s.report
	report.Applied = s.err == nil && !opts.DryRun && !report.Plan.Empty()
	return report, s.err
}

func migration(version int, title string, width int, suffixes ...string) domain.Migration {
	m := domain.Migration{Version: version, Title: title, Width: width}
	for _, suffix := range suffixes {
		m.Files = append(m.Files, fmt.Sprintf("%0*d_%s%s", width, version, title, suffix))
	}
	return m
}

func pairOf(version int, title string) domain.Migration {
	return migration(version, title, 6, ".up.sql", ".down.sql")
}

func collisionReport() app.Report {
	orders := pairOf(2, "add_orders")
	return app.Report{
		Base:       "main",
		BaseMax:    2,
		Collisions: []domain.Collision{{Kind: domain.CollidesWithBase, Ours: orders, Others: []domain.Migration{pairOf(2, "add_users")}}},
		Plan:       domain.RenamePlan{Moves: []domain.Move{{From: orders, To: pairOf(3, "add_orders")}}},
	}
}

func run(svc *stubService, args ...string) (code int, stdout, stderr string) {
	var out, errOut bytes.Buffer
	code = cli.Run(context.Background(), args, &out, &errOut, svc)
	return code, out.String(), errOut.String()
}

func TestOutput(t *testing.T) {
	const collides = "✗ 000002 collides: add_orders (yours) vs add_users (main)\n"
	const move = "000002_add_orders.{up,down}.sql -> 000003_add_orders.{up,down}.sql\n"

	x, y := migration(2, "x", 6, ".up.sql"), migration(3, "y", 6, ".up.sql")
	a, b := migration(7, "a", 3, ".up.sql"), migration(7, "b", 3, ".up.sql")

	tests := []struct {
		name       string
		args       []string
		report     app.Report
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		{
			name:       "check clean",
			args:       []string{"check"},
			report:     app.Report{Base: "main"},
			wantCode:   cli.ExitOK,
			wantStdout: "✓ no collisions\n",
		},
		{
			name:       "check collision",
			args:       []string{"check"},
			report:     collisionReport(),
			wantCode:   cli.ExitProblems,
			wantStdout: collides + "  fix: " + move + "run `migraine fix` to renumber\n",
		},
		{
			name:       "fix",
			args:       []string{"fix"},
			report:     collisionReport(),
			wantCode:   cli.ExitOK,
			wantStdout: collides + "  renamed: " + move + "✓ renumbered 1 migration\n",
		},
		{
			name:       "fix dry run",
			args:       []string{"fix", "--dry-run"},
			report:     collisionReport(),
			wantCode:   cli.ExitOK,
			wantStdout: collides + "  fix: " + move + "dry run: no files changed\n",
		},
		{
			name:       "fix clean",
			args:       []string{"fix"},
			report:     app.Report{Base: "main"},
			wantCode:   cli.ExitOK,
			wantStdout: "✓ no collisions\n",
		},
		{
			name: "out of order, with a later migration shifted along",
			args: []string{"fix"},
			report: app.Report{
				Base:       "origin/main",
				BaseMax:    4,
				Collisions: []domain.Collision{{Kind: domain.BelowBaseMax, Ours: x}},
				Plan: domain.RenamePlan{Moves: []domain.Move{
					{From: x, To: migration(5, "x", 6, ".up.sql")},
					{From: y, To: migration(6, "y", 6, ".up.sql")},
				}},
			},
			wantCode: cli.ExitOK,
			wantStdout: "✗ 000002 out of order: x (yours) sorts before origin/main's latest, 000004\n" +
				"  renamed: 000002_x.up.sql -> 000005_x.up.sql\n" +
				"  renamed: 000003_y.up.sql -> 000006_y.up.sql\n" +
				"✓ renumbered 2 migrations\n",
		},
		{
			name: "local duplicate lists the move of the other migration",
			args: []string{"check"},
			report: app.Report{
				Base:       "main",
				BaseMax:    6,
				Collisions: []domain.Collision{{Kind: domain.LocalDuplicate, Ours: a, Others: []domain.Migration{b}}},
				Plan:       domain.RenamePlan{Moves: []domain.Move{{From: b, To: migration(8, "b", 3, ".up.sql")}}},
			},
			wantCode: cli.ExitProblems,
			wantStdout: "✗ 007 duplicated: a vs b (both yours)\n" +
				"  fix: 007_b.up.sql -> 008_b.up.sql\n" +
				"run `migraine fix` to renumber\n",
		},
		{
			name: "warnings go to stderr",
			args: []string{"check"},
			report: app.Report{
				Base:       "main",
				BaseMax:    999,
				Ignored:    []string{"README.md"},
				Collisions: []domain.Collision{{Kind: domain.CollidesWithBase, Ours: migration(999, "mine", 3, ".up.sql", ".down.sql"), Others: []domain.Migration{migration(999, "last", 3, ".up.sql")}}},
				Plan: domain.RenamePlan{Moves: []domain.Move{{
					From: migration(999, "mine", 3, ".up.sql", ".down.sql"),
					To:   migration(1000, "mine", 4, ".up.sql", ".down.sql"),
				}}},
			},
			wantCode: cli.ExitProblems,
			wantStdout: "✗ 999 collides: mine (yours) vs last (main)\n" +
				"  fix: 999_mine.{up,down}.sql -> 1000_mine.{up,down}.sql\n" +
				"run `migraine fix` to renumber\n",
			wantStderr: "warning: ignoring \"README.md\": not a migration filename\n" +
				"warning: 1000 does not fit in 3 digits; widening 1000_mine.{up,down}.sql\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, stdout, stderr := run(&stubService{report: tt.report}, tt.args...)
			if code != tt.wantCode {
				t.Errorf("exit code = %d, want %d", code, tt.wantCode)
			}
			if stdout != tt.wantStdout {
				t.Errorf("stdout:\n%s\nwant:\n%s", stdout, tt.wantStdout)
			}
			if stderr != tt.wantStderr {
				t.Errorf("stderr:\n%s\nwant:\n%s", stderr, tt.wantStderr)
			}
		})
	}
}

func TestFlagsReachTheService(t *testing.T) {
	svc := &stubService{}
	code, _, _ := run(svc, "fix", "--dir", "db/migrations", "--base=origin/main", "--dry-run", "--no-color")
	if code != cli.ExitOK {
		t.Fatalf("exit code = %d", code)
	}
	want := app.Options{Dir: "db/migrations", Base: "origin/main", DryRun: true}
	if svc.called != "fix" || svc.opts != want {
		t.Errorf("called %s with %+v, want fix with %+v", svc.called, svc.opts, want)
	}

	run(svc, "check")
	if want := (app.Options{Dir: "migrations"}); svc.called != "check" || svc.opts != want {
		t.Errorf("called %s with %+v, want check with defaults %+v", svc.called, svc.opts, want)
	}
}

func TestErrorsMapToExitCodes(t *testing.T) {
	tests := []struct {
		err      error
		wantCode int
	}{
		{fmt.Errorf("resolve base: %w", domain.ErrNotGitRepo), cli.ExitError},
		{fmt.Errorf("resolve base: %w", domain.ErrBaseNotFound), cli.ExitError},
		{fmt.Errorf("list: %w", domain.ErrDirNotFound), cli.ExitError},
		{fmt.Errorf("read: %w", domain.ErrTimestampVersion), cli.ExitError},
		{errors.New("disk on fire"), cli.ExitError},
		{fmt.Errorf("apply renames: %w", domain.ErrTargetExists), cli.ExitProblems},
	}
	for _, tt := range tests {
		code, stdout, stderr := run(&stubService{err: tt.err}, "fix")
		if code != tt.wantCode {
			t.Errorf("%v: exit code = %d, want %d", tt.err, code, tt.wantCode)
		}
		if want := "migraine: " + tt.err.Error() + "\n"; stderr != want || stdout != "" {
			t.Errorf("stdout = %q stderr = %q, want only %q on stderr", stdout, stderr, want)
		}
	}
}

func TestUsageErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantMsg string
	}{
		{"no command", nil, "missing command"},
		{"unknown command", []string{"repair"}, `unknown command "repair"`},
		{"unknown flag", []string{"check", "--force"}, "flag provided but not defined"},
		{"dry-run is fix only", []string{"check", "--dry-run"}, "flag provided but not defined"},
		{"stray argument", []string{"fix", "migrations"}, `unexpected argument "migrations"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &stubService{}
			code, stdout, stderr := run(svc, tt.args...)
			if code != cli.ExitError || svc.called != "" {
				t.Errorf("exit code = %d, service called = %q; want 2 and no call", code, svc.called)
			}
			if stdout != "" || !strings.Contains(stderr, tt.wantMsg) || !strings.Contains(stderr, "Usage:") {
				t.Errorf("stdout = %q\nstderr = %q\nwant %q plus usage on stderr", stdout, stderr, tt.wantMsg)
			}
		})
	}
}

func TestHelp(t *testing.T) {
	for _, args := range [][]string{{"help"}, {"--help"}, {"-h"}, {"check", "-h"}, {"fix", "--help"}} {
		code, stdout, stderr := run(&stubService{}, args...)
		if code != cli.ExitOK || !strings.HasPrefix(stdout, "migraine fixes") || stderr != "" {
			t.Errorf("%v: code = %d stdout = %q stderr = %q", args, code, stdout, stderr)
		}
	}
}

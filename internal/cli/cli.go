// Package cli is the command-line front end: it parses arguments, runs
// a use case and turns the result into output and an exit code.
package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/bSemir/migraine/internal/app"
	"github.com/bSemir/migraine/internal/core/domain"
)

// Exit codes returned by Run.
const (
	// ExitOK means the tree is clean or was fixed.
	ExitOK = 0
	// ExitProblems means check found problems, or fix could not safely
	// resolve them.
	ExitProblems = 1
	// ExitError means a usage or runtime error.
	ExitError = 2
)

const usage = `migraine fixes colliding SQL migration numbers after git merges and rebases.

Usage:
  migraine check [flags]   report collisions and out-of-order migrations (exit 1 if any)
  migraine fix [flags]     renumber this branch's migrations to follow the base branch

Flags:
  --dir string    migrations directory (default "migrations")
  --base string   base ref to compare against (default main, falling back to master)
  --no-color      disable colored output
  --dry-run       fix only: print the plan without renaming anything
`

// Service is what the CLI needs from the application layer.
type Service interface {
	Check(ctx context.Context, opts app.Options) (app.Report, error)
	Fix(ctx context.Context, opts app.Options) (app.Report, error)
}

// command is a fully parsed invocation.
type command struct {
	name    string
	opts    app.Options
	noColor bool
}

// Run executes migraine with the given arguments (without the program
// name) and returns the process exit code.
func Run(ctx context.Context, args []string, stdout, stderr io.Writer, svc Service) int {
	cmd, err := parse(args)
	if errors.Is(err, flag.ErrHelp) {
		_, _ = fmt.Fprint(stdout, usage)
		return ExitOK
	}
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "migraine: %v\n\n%s", err, usage)
		return ExitError
	}

	run := svc.Check
	if cmd.name == "fix" {
		run = svc.Fix
	}
	report, err := run(ctx, cmd.opts)

	out := printer{out: stdout, errOut: stderr, color: !cmd.noColor && isTerminal(stdout)}
	out.warnings(report)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "migraine: %v\n", err)
		return exitCodeFor(err)
	}
	out.report(cmd, report)
	if cmd.name == "check" && len(report.Collisions) > 0 {
		return ExitProblems
	}
	return ExitOK
}

func parse(args []string) (command, error) {
	if len(args) == 0 {
		return command{}, errors.New("missing command")
	}
	cmd := command{name: args[0]}
	flags := flag.NewFlagSet(cmd.name, flag.ContinueOnError)
	// Errors and usage are printed by Run, in one consistent format.
	flags.SetOutput(io.Discard)
	flags.StringVar(&cmd.opts.Dir, "dir", "migrations", "")
	flags.StringVar(&cmd.opts.Base, "base", "", "")
	flags.BoolVar(&cmd.noColor, "no-color", false, "")

	switch cmd.name {
	case "check":
	case "fix":
		flags.BoolVar(&cmd.opts.DryRun, "dry-run", false, "")
	case "help", "-h", "-help", "--help":
		return command{}, flag.ErrHelp
	default:
		return command{}, fmt.Errorf("unknown command %q", cmd.name)
	}

	if err := flags.Parse(args[1:]); err != nil {
		return command{}, err
	}
	if flags.NArg() > 0 {
		return command{}, fmt.Errorf("unexpected argument %q", flags.Arg(0))
	}
	return cmd, nil
}

// exitCodeFor maps a use-case error to an exit code. Only "the fix is
// known but unsafe to apply" counts as unresolved problems; everything
// else is a runtime error.
func exitCodeFor(err error) int {
	if errors.Is(err, domain.ErrTargetExists) {
		return ExitProblems
	}
	return ExitError
}

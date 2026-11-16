package main

import (
	"errors"
	"os"
	"os/exec"
	"strconv"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
)

// TestMain lets the test binary double as the migraine executable, so
// the scripts run the real main function without a separate build.
func TestMain(m *testing.M) {
	testscript.Main(m, map[string]func(){"migraine": main})
}

func TestScripts(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir:                "testdata/script",
		RequireUniqueNames: true,
		Setup: func(env *testscript.Env) error {
			// A fixed identity and no user or system config make git
			// behave the same on a laptop and on a bare CI runner.
			env.Setenv("GIT_AUTHOR_NAME", "Test")
			env.Setenv("GIT_AUTHOR_EMAIL", "test@example.com")
			env.Setenv("GIT_COMMITTER_NAME", "Test")
			env.Setenv("GIT_COMMITTER_EMAIL", "test@example.com")
			env.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
			env.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
			// The work dir may sit below a repository (TMPDIR inside a
			// checkout); never let git discover one above it.
			env.Setenv("GIT_CEILING_DIRECTORIES", env.WorkDir)
			return nil
		},
		Cmds: map[string]func(*testscript.TestScript, bool, []string){
			"exitcode": exitCode,
		},
	})
}

// exitCode implements "exitcode N command [args...]": it runs the
// command and fails unless it exits with exactly N. The built-in "!"
// only distinguishes zero from non-zero, but migraine's contract
// separates 1 (problems found) from 2 (error). stdout and stderr stay
// available to the following script lines.
func exitCode(ts *testscript.TestScript, neg bool, args []string) {
	if neg || len(args) < 2 {
		ts.Fatalf("usage: exitcode N command [args...]")
	}
	want, err := strconv.Atoi(args[0])
	ts.Check(err)

	got := 0
	var exitErr *exec.ExitError
	if err := ts.Exec(args[1], args[2:]...); errors.As(err, &exitErr) {
		got = exitErr.ExitCode()
	} else {
		ts.Check(err)
	}
	if got != want {
		ts.Fatalf("%s exited with code %d, want %d", args[1], got, want)
	}
}

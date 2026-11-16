// Command migraine detects and fixes colliding SQL migration numbers
// after git merges and rebases.
package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/bSemir/migraine/internal/adapters/fsys"
	"github.com/bSemir/migraine/internal/adapters/git"
	"github.com/bSemir/migraine/internal/adapters/naming/golangmigrate"
	"github.com/bSemir/migraine/internal/app"
	"github.com/bSemir/migraine/internal/cli"
)

func main() {
	os.Exit(run())
}

// run is separate from main so the deferred signal cleanup happens
// before os.Exit, which skips defers.
func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// Wiring only: the golang-migrate scheme is the one supported today;
	// other tools plug in here as another ports.NamingScheme.
	service := app.New(git.New(""), fsys.New(), golangmigrate.New())
	return cli.Run(ctx, os.Args[1:], os.Stdout, os.Stderr, service)
}

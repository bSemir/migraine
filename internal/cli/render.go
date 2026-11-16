package cli

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/bSemir/migraine/internal/app"
	"github.com/bSemir/migraine/internal/core/domain"
)

// ANSI styles. They are applied only when color is enabled.
const (
	red   = "\x1b[31m"
	green = "\x1b[32m"
	dim   = "\x1b[2m"
	reset = "\x1b[0m"
)

// printer renders reports for humans.
type printer struct {
	out    io.Writer
	errOut io.Writer
	color  bool
}

func (p printer) style(code, text string) string {
	if !p.color {
		return text
	}
	return code + text + reset
}

// println writes one line to stdout. Write errors are dropped on
// purpose: there is nowhere left to report a broken stdout.
func (p printer) println(format string, args ...any) {
	_, _ = fmt.Fprintf(p.out, format+"\n", args...)
}

// warnings goes to stderr so that stdout stays clean for the report.
func (p printer) warnings(report app.Report) {
	for _, name := range report.Ignored {
		_, _ = fmt.Fprintf(p.errOut, "warning: ignoring %q: not a migration filename\n", name)
	}
	for _, move := range report.Plan.Moves {
		if move.Widened() {
			_, _ = fmt.Fprintf(p.errOut, "warning: %d does not fit in %d digits; widening %s\n",
				move.To.Version, move.From.Width, braces(move.To.Files))
		}
	}
}

func (p printer) report(cmd command, report app.Report) {
	if len(report.Collisions) == 0 {
		p.println("%s no collisions", p.style(green, "✓"))
		return
	}

	label := "fix"
	if report.Applied {
		label = "renamed"
	}
	pending := slices.Clone(report.Plan.Moves)
	for _, c := range report.Collisions {
		p.println("%s %s", p.style(red, "✗"), describe(c, report))
		// Each move is listed once, under the first problem it settles.
		for _, m := range append([]domain.Migration{c.Ours}, c.Others...) {
			i := slices.IndexFunc(pending, func(mv domain.Move) bool { return mv.From.Key() == m.Key() })
			if i >= 0 {
				p.move(label, pending[i])
				pending = slices.Delete(pending, i, i+1)
			}
		}
	}
	// Moves without a problem of their own: later migrations shifted
	// up to keep the branch's order.
	for _, move := range pending {
		p.move(label, move)
	}

	n := len(report.Plan.Moves)
	switch {
	case report.Applied:
		p.println("%s renumbered %d %s", p.style(green, "✓"), n, plural(n, "migration"))
	case cmd.opts.DryRun:
		p.println("dry run: no files changed")
	default:
		p.println("run `migraine fix` to renumber")
	}
}

func (p printer) move(label string, move domain.Move) {
	p.println("  %s", p.style(dim, fmt.Sprintf("%s: %s -> %s", label, braces(move.From.Files), braces(move.To.Files))))
}

// describe states one problem in a sentence.
func describe(c domain.Collision, report app.Report) string {
	version := fmt.Sprintf("%0*d", c.Ours.Width, c.Ours.Version)
	switch c.Kind {
	case domain.CollidesWithBase:
		return fmt.Sprintf("%s collides: %s (yours) vs %s (%s)", version, c.Ours.Title, titles(c.Others), report.Base)
	case domain.LocalDuplicate:
		return fmt.Sprintf("%s duplicated: %s vs %s (both yours)", version, c.Ours.Title, titles(c.Others))
	default:
		return fmt.Sprintf("%s out of order: %s (yours) sorts before %s's latest, %0*d",
			version, c.Ours.Title, report.Base, c.Ours.Width, report.BaseMax)
	}
}

func titles(migrations []domain.Migration) string {
	names := make([]string, len(migrations))
	for i, m := range migrations {
		names[i] = m.Title
	}
	return strings.Join(names, ", ")
}

// braces collapses the files of one migration into shell brace
// notation: x.up.sql and x.down.sql become x.{up,down}.sql.
func braces(files []string) string {
	if len(files) == 1 {
		return files[0]
	}
	prefix, suffix := files[0], files[0]
	for _, f := range files[1:] {
		prefix = commonPrefix(prefix, f)
	}
	for _, f := range files {
		// The suffix is measured on what the prefix leaves over, so
		// the two can never overlap.
		suffix = commonSuffix(suffix, f[len(prefix):])
	}
	middles := make([]string, len(files))
	for i, f := range files {
		middles[i] = f[len(prefix) : len(f)-len(suffix)]
	}
	return prefix + "{" + strings.Join(middles, ",") + "}" + suffix
}

func commonPrefix(a, b string) string {
	n := 0
	for n < len(a) && n < len(b) && a[n] == b[n] {
		n++
	}
	return a[:n]
}

func commonSuffix(a, b string) string {
	n := 0
	for n < len(a) && n < len(b) && a[len(a)-1-n] == b[len(b)-1-n] {
		n++
	}
	return a[len(a)-n:]
}

func plural(n int, word string) string {
	if n == 1 {
		return word
	}
	return word + "s"
}

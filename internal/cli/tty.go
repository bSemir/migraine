package cli

import (
	"io"
	"os"
)

// isTerminal reports whether w is an interactive terminal. Pipes, files
// and in-memory buffers are not, which keeps CI logs and redirected
// output free of escape codes.
func isTerminal(w io.Writer) bool {
	file, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

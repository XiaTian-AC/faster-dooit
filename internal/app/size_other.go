//go:build !windows

package app

import (
	"os"

	"github.com/charmbracelet/x/term"
)

// terminalSize returns the visible terminal size. On Unix the controlling
// terminal is queried via the stdout (or stdin) file descriptor.
func terminalSize() (int, int, bool) {
	for _, fd := range []uintptr{os.Stdout.Fd(), os.Stdin.Fd()} {
		if w, h, err := term.GetSize(fd); err == nil {
			return w, h, true
		}
	}
	return 0, 0, false
}

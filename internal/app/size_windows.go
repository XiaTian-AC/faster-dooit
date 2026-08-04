//go:build windows

package app

import (
	"os"

	"github.com/charmbracelet/x/term"
)

// terminalSize returns the visible terminal size. On Windows the stdout
// handle can be a pipe under ConPTY (Windows Terminal / wezterm), so we open
// the real console output handle CONOUT$ and fall back to stdout/stdin.
func terminalSize() (int, int, bool) {
	if f, err := os.OpenFile("CONOUT$", os.O_WRONLY, 0); err == nil {
		if w, h, err2 := term.GetSize(f.Fd()); err2 == nil {
			f.Close()
			return w, h, true
		}
		f.Close()
	}
	for _, fd := range []uintptr{os.Stdout.Fd(), os.Stdin.Fd()} {
		if w, h, err := term.GetSize(fd); err == nil {
			return w, h, true
		}
	}
	return 0, 0, false
}

//go:build windows

package term

import (
	"os"

	"golang.org/x/sys/windows"
)

// enableVT enables ANSI escape sequence processing on a Windows console
// output handle so remote PTY output renders correctly.
func enableVT(f *os.File) error {
	h := windows.Handle(f.Fd())
	var mode uint32
	if err := windows.GetConsoleMode(h, &mode); err != nil {
		return err
	}
	return windows.SetConsoleMode(h, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING)
}

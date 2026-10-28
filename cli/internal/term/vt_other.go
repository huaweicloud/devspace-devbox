//go:build !windows

package term

import "os"

// enableVT is a no-op on Unix terminals, which process ANSI escapes by default.
func enableVT(f *os.File) error { return nil }

//go:build !windows

package store

import (
	"os"
	"syscall"
)

// NudgeInstance asks an already-running sjournal process (pid) to bring its
// window to the front, by sending it SIGUSR1. Best-effort: any error is
// ignored. Used when a second launch is blocked by the single-instance lock
// so double-clicking the launcher behaves like a normal app.
func NudgeInstance(pid int) {
	if proc, err := os.FindProcess(pid); err == nil {
		_ = proc.Signal(syscall.SIGUSR1)
	}
}

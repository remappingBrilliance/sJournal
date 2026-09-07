package store

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

const lockFileName = ".lock"

// ErrLocked is returned by AcquireLock when another live process already
// holds the lock on this notes folder.
type ErrLocked struct{ PID int }

func (e *ErrLocked) Error() string {
	return fmt.Sprintf("notes folder is already locked by running process (pid %d)", e.PID)
}

// Lock represents this process's hold on a notes folder.
type Lock struct {
	path string
}

// AcquireLock takes the single-instance lock on dir. If a stale lock (owning
// PID no longer running) is found, it's cleaned up and acquisition proceeds.
func AcquireLock(dir string) (*Lock, error) {
	path := filepath.Join(dir, lockFileName)

	if data, err := os.ReadFile(path); err == nil {
		if pid, ok := parsePID(data); ok && pidRunning(pid) {
			return nil, &ErrLocked{PID: pid}
		}
		// stale lock: fall through and overwrite it.
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
		return nil, err
	}
	return &Lock{path: path}, nil
}

// Release removes the lock file. Safe to call once when the app shuts down
// cleanly.
func (l *Lock) Release() error {
	return os.Remove(l.path)
}

func parsePID(data []byte) (int, bool) {
	s := strings.TrimSpace(string(data))
	pid, err := strconv.Atoi(s)
	if err != nil {
		return 0, false
	}
	return pid, true
}

func pidRunning(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Unix, FindProcess always succeeds; signal 0 checks liveness/permission
	// without actually sending a signal.
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}

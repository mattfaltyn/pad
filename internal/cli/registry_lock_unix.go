//go:build unix

package cli

import (
	"os"
	"path/filepath"
	"syscall"
)

func lockRegistry(dir string) (unlock func(), err error) {
	f, err := os.OpenFile(filepath.Join(dir, "installations.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		_ = f.Close()
		return nil, err
	}
	return func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
	}, nil
}

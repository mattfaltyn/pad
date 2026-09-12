//go:build darwin

package cli

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

func pidIsSelfOrAncestor(pid int) (bool, error) {
	if pid <= 0 {
		return false, nil
	}
	current := os.Getpid()
	for steps := 0; current > 0 && steps < 1024; steps++ {
		if current == pid {
			return true, nil
		}
		proc, err := unix.SysctlKinfoProc("kern.proc.pid", current)
		if err != nil {
			if errors.Is(err, unix.ESRCH) {
				return false, os.ErrNotExist
			}
			return false, err
		}
		parent := int(proc.Proc.P_oppid)
		if parent == current {
			return false, nil
		}
		current = parent
	}
	if current > 0 {
		return false, fmt.Errorf("process ancestry exceeded 1024 levels")
	}
	return false, nil
}

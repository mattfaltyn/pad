//go:build darwin

package cli

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

const darwinZombieState = 5

func procStartToken(pid int) (string, bool) {
	token, err := procStartTokenErr(pid)
	return token, err == nil
}

func procStartTokenErr(pid int) (string, error) {
	proc, err := unix.SysctlKinfoProc("kern.proc.pid", pid)
	if err != nil {
		if errors.Is(err, unix.ESRCH) {
			return "", os.ErrNotExist
		}
		return "", err
	}
	if proc.Proc.P_pid == 0 {
		return "", os.ErrNotExist
	}
	if proc.Proc.P_stat == darwinZombieState {
		return "", errProcZombie
	}
	started := proc.Proc.P_starttime
	return fmt.Sprintf("%d:%d", started.Sec, started.Usec), nil
}

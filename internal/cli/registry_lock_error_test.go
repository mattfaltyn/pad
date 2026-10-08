package cli

import (
	"errors"
	"io/fs"
	"strings"
	"syscall"
	"testing"
)

func TestLockErrorExplainsPermissionDenied(t *testing.T) {
	raw := &fs.PathError{Op: "open", Path: "/home/x/.pad/installations.lock", Err: syscall.EPERM}

	err := lockError(raw)

	if !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("lockError must keep the permission error in the chain: %v", err)
	}
	if !strings.Contains(err.Error(), "was not updated") {
		t.Fatalf("lockError should say the registry was not updated: %v", err)
	}
}

func TestLockErrorKeepsOtherFailuresUnchanged(t *testing.T) {
	err := lockError(errors.New("disk full"))

	if strings.Contains(err.Error(), "was not updated") {
		t.Fatalf("non-permission failures must not get the permission hint: %v", err)
	}
	if err.Error() != "lock registry: disk full" {
		t.Fatalf("unexpected message: %q", err.Error())
	}
}

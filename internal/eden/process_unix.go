//go:build !windows

package eden

import (
	"errors"
	"syscall"
)

func profileProcessAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

//go:build !windows

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"errors"
	"os"
	"syscall"
)

func pidRunning(pid int) (bool, error) {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false, err
	}
	defer proc.Release()
	err = proc.Signal(syscall.Signal(0))
	if errors.Is(err, os.ErrProcessDone) {
		return false, nil
	}
	if err != nil && !errors.Is(err, os.ErrPermission) {
		return false, err
	}
	return true, nil
}

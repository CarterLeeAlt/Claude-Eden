//go:build !windows

package localsession

import "syscall"

const connectionRefused = syscall.ECONNREFUSED

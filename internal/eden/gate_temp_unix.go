//go:build !windows

package eden

import "os"

// TMPDIR must leave room for the random directory and AF_UNIX socket name.
func gateTempDir() string { return os.TempDir() }

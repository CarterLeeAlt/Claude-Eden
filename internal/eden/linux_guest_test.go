package eden

import (
	"runtime"
	"testing"
)

func requireLinuxGuest(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("executes Linux guest scripts with Linux tools, POSIX paths and process semantics")
	}
}

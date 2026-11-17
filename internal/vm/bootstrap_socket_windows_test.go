package vm

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/sylvinus/agent-vm/internal/eden"
	"github.com/sylvinus/agent-vm/internal/paths"
	"github.com/sylvinus/agent-vm/internal/vmname"
)

func TestWindowsBootstrapUsesActualOpenSSHSocketBudget(t *testing.T) {
	// Windows path syntax only: read-only name lookup, never a write target.
	temporary := `C:\` + strings.Repeat("x", 65)
	home := filepath.Join(temporary, "v", "state", "lima")
	managed := eden.InstanceName(filepath.Join(temporary, "v"), filepath.Join(temporary, "downloads"))
	for _, name := range []string{vmname.Building, vmname.Template, managed} {
		socket := filepath.Join(home, name, paths.SSHControlSocketMaxName)
		if len(socket) >= paths.UnixSocketPathMax {
			t.Fatalf("bootstrap %q needs %d socket bytes, maximum %d", name, len(socket), paths.UnixSocketPathMax-1)
		}
	}
	limit := vmname.HomeLimit(home, true)
	if len(vmname.Building) > int(limit) || len(vmname.Template) > int(limit) {
		t.Fatalf("fixed bootstrap names exceed the computed name budget: %d", limit)
	}
}

package vm

import (
	"testing"

	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
	"github.com/lima-vm/lima/v2/pkg/osutil"
	"github.com/sylvinus/agent-vm/internal/paths"
)

func TestSocketBudgetMatchesLimaTransport(t *testing.T) {
	if paths.UnixSocketPathMax != osutil.UnixPathMax {
		t.Fatalf("host sockaddr_un capacity %d differs from Lima %d", paths.UnixSocketPathMax, osutil.UnixPathMax)
	}
	if paths.SSHControlSocketMaxName != filenames.LongestSock {
		t.Fatalf("OpenSSH control socket budget %q differs from Lima %q", paths.SSHControlSocketMaxName, filenames.LongestSock)
	}
}

package vm

import (
	"slices"
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
	for _, name := range []string{filenames.QMPSock, filenames.SerialSock, filenames.SerialPCISock, filenames.SerialVirtioSock, filenames.GuestAgentSock, filenames.HostAgentSock, filenames.SwtpmSock} {
		if len(name) > len(filenames.LongestSock) {
			t.Fatalf("socket budget does not cover %q", name)
		}
	}
	if !slices.Contains(filenames.SkipOnClone, filenames.SSHSock) {
		t.Fatal("cloning must not copy the SSH control endpoint")
	}
}

//go:build !windows

package bashref

import (
	"path/filepath"
	"testing"
)

func TestPathsNormalizeWorkspaceTemporaryRootsBeforeRepository(t *testing.T) {
	temporary := filepath.Join(Root(), ".tmp")
	t.Setenv("TMPDIR", temporary)
	p := newPaths()
	for _, tc := range []struct{ input, want string }{
		{Root() + "/internal", "{ROOT}/internal"},
		{"HOME=" + temporary + "/TestFixture123/001", "HOME={T0}/001"},
		{temporary + "/TestFixture123/002 " + Root() + "/agent-vm.sh", "{T0}/002 {ROOT}/agent-vm.sh"},
		{temporary + "/TestOther456/001 " + temporary + "/TestFixture123/003", "{T1}/001 {T0}/003"},
	} {
		actual := p.out(tc.input)
		if actual != tc.want {
			t.Fatalf("normalized %q as %q, want %q", tc.input, actual, tc.want)
		}
		if restored := p.in(actual); restored != tc.input {
			t.Fatalf("round trip changed %q to %q", tc.input, restored)
		}
	}
}

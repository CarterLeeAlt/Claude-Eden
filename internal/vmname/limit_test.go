package vmname

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/sylvinus/agent-vm/internal/paths"
)

func TestHomeLimitSocketBoundary(t *testing.T) {
	for _, size := range []int{1, 10, 75, 76, 77, 150} {
		homeBytes := paths.UnixSocketPathMax - 1 - 2 - len(paths.SSHControlSocketMaxName) - size
		if homeBytes < 0 {
			homeBytes = 0
		}
		home := strings.Repeat("x", homeBytes)
		want := min(76, paths.UnixSocketPathMax-1-2-len(paths.SSHControlSocketMaxName)-len(home))
		if got := HomeLimit(home, true); got != Limit(want) {
			t.Fatalf("%d-byte home: limit %d, want %d", len(home), got, want)
		}
		if size >= 10 && want < 76 {
			name := HomeLimit(home, true).Name(strings.Repeat("project", 30))
			if len(home)+1+len(name)+1+len(paths.SSHControlSocketMaxName) != paths.UnixSocketPathMax-1 {
				t.Fatal("name failed to reserve exactly one terminating NUL")
			}
		}
	}
	if got := HomeLimit(strings.Repeat("长", paths.UnixSocketPathMax), true); got != 1 {
		t.Fatalf("oversized UTF-8 home: %d", got)
	}
	root := t.TempDir()
	home := filepath.Join(root, "not-created", "lima")
	if HomeLimit(home, false) != HomeLimit(home, true) {
		t.Fatal("nonexistent descendants changed an ordinary physical home budget")
	}
}

package env

import (
	"os"
	"runtime"
	"testing"

	"github.com/sylvinus/agent-vm/internal/paths"
)

// Exercise Windows drive syntax with every filesystem write inside the fixture.
func TestProjectRelWindows(t *testing.T) {
	old := paths.Windows
	paths.Windows = func() bool { return true }
	t.Cleanup(func() { paths.Windows = old })
	dir := t.TempDir()
	t.Chdir(dir)
	root := "C:"
	if runtime.GOOS == "windows" {
		root = paths.Host(dir)
	}
	project := root + "/p"
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("p", root+"/link"); err != nil {
		t.Fatal(err)
	}
	for target, want := range map[string]string{project + "/.env": ".env", root + "/link/.env": ".env", root + "/link/sub/x": "sub/x"} {
		if rel, ok := ProjectRel(project, target); !ok || rel != want {
			t.Errorf("ProjectRel(%q, %q) = %q, %v", project, target, rel, ok)
		}
	}
	if _, ok := ProjectRel(project, root+"/other/.env"); ok {
		t.Error("the sibling directory is not in the project")
	}
}

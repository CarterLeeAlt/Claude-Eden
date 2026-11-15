package mounts

import (
	"bytes"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/sylvinus/agent-vm/internal/paths"
)

// A repository's own folder is found up to the drive, the drive's root
// excepted, as "/" is elsewhere.
func TestWindowsGitDirShare(t *testing.T) {
	old := paths.Windows
	paths.Windows = func() bool { return true }
	t.Cleanup(func() { paths.Windows = old })
	root := windowsFixtureRoot(t)
	for _, d := range []string{"proj", "bare/objects", "bare/refs", "bare/sub"} {
		if err := os.MkdirAll(root+"/"+d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// On POSIX the simulated drive root is a private relative directory.
	if runtime.GOOS != "windows" {
		for _, d := range []string{"objects", "refs"} {
			if err := os.MkdirAll(root+"/"+d, 0o755); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.WriteFile(root+"/HEAD", nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(root+"/bare/HEAD", nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if why, bad := GitDirShare(paths.Root(root)); bad {
		t.Errorf("the drive root taken for a repository: %s", why)
	}
	if why, bad := GitDirShare(root + "/proj"); bad {
		t.Errorf("the root taken for a repository: %s", why)
	}
	if why, bad := GitDirShare(root + "/bare/sub"); !bad || !strings.Contains(why, "("+root+"/bare)") {
		t.Errorf("inside a bare repository: %q %v", why, bad)
	}
}

// windowsFixtureRoot uses an actual private path on Windows and a relative
// simulated drive on POSIX; neither case creates anything at a real drive root.
func windowsFixtureRoot(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Chdir(dir)
	if runtime.GOOS == "windows" {
		return paths.Host(dir)
	}
	return "C:"
}

// The volumes file and the shares use Windows path syntax on every host.
func TestWindowsVolumes(t *testing.T) {
	old := paths.Windows
	paths.Windows = func() bool { return true }
	t.Cleanup(func() { paths.Windows = old })
	root := windowsFixtureRoot(t)
	fixture := strings.NewReplacer("C:/", root+"/", `C:\`, strings.ReplaceAll(root, "/", `\`)+`\`, "/c/", paths.Guest(root)+"/").Replace
	for _, d := range []string{"C:/Users/me/proj", "C:/data", "C:/data2", "C:/data3", "C:/data4", "C:/data5", "C:/data6", "C:/st"} {
		if err := os.MkdirAll(fixture(d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(fixture("C:/f.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := hideDrives(`C:/data:/mnt/d:rw:C:/work/*`); got != "C"+driveColon+"/data:/mnt/d:rw:C"+driveColon+"/work/*" {
		t.Errorf("hideDrives: %q", got)
	}
	if got := hideDrives(`C:\data:x`); got != "C"+driveColon+`\data:x` {
		t.Errorf(`hideDrives(C:\data): %q`, got)
	}

	vols := strings.Join([]string{
		"C:/data",                              // its own path in the VM: /c/data
		`C:\data2:/mnt/d2:rw`,                  // backslashes
		"/c/data3",                             // 0.2's Git Bash spelling
		"C:/data4:sub:ro:C:/Users/me/*",        // a filter with a drive, matched
		"C:/data5:/mnt/x:ro:C:/other/*",        // not matched
		"C:/data6:/mnt/d6:ro:/c/Users/me/proj", // 0.2's filter spelling, matched
		"C:/data6:C:/x",                        // a destination with a drive: refused
		"C:/f.txt",                             // a single file, bound at /c/f.txt
		"C:/Users",                             // at /c/Users: over the project, refused
	}, "\n") + "\n"
	if err := os.WriteFile("vols", []byte(fixture(vols)), 0o644); err != nil {
		t.Fatal(err)
	}
	var warn bytes.Buffer
	es := Entries("vols", fixture("C:/Users/me/proj"), fixture("C:/Users/me"), nil, &warn)
	var got []string
	for _, e := range es {
		got = append(got, e.Src+">"+e.Dst+">"+e.Mode)
	}
	want := fixture("C:/data>>ro C:/data2>/mnt/d2>rw C:/data3>>ro C:/data4>sub>ro C:/data6>/mnt/d6>ro C:/f.txt>>ro")
	if strings.Join(got, " ") != want {
		t.Errorf("entries:\n got  %s\n want %s\n%s", strings.Join(got, " "), want, warn.String())
	}
	if !strings.Contains(warn.String(), fixture("'C:/data6:C:/x'")) || !strings.Contains(warn.String(), fixture("'C:/Users' (from ~/.agent-vm/volumes) would be mounted at /c/Users, which covers the project")) ||
		strings.Count(warn.String(), "Warning") != 2 {
		t.Errorf("warnings: %s", warn.String())
	}

	ms, files := Build(Shares{VM: "v", Dir: fixture("C:/Users/me/proj"), Writable: true, Entries: es, StateDir: fixture("C:/st"), Warn: &warn})
	var mounts []string
	for _, m := range ms[:len(ms)-1] {
		mounts = append(mounts, m.Location+">"+m.MountPoint)
	}
	if want := fixture("C:/Users/me/proj>/c/Users/me/proj C:/data>/c/data C:/data2>/mnt/d2 C:/data3>/c/data3 C:/data4>/c/Users/me/proj/sub C:/data6>/mnt/d6"); strings.Join(mounts, " ") != want {
		t.Errorf("mounts:\n got  %s\n want %s", strings.Join(mounts, " "), want)
	}
	if len(files) != 1 || files[0].BindDst != fixture("/c/f.txt") || !strings.HasPrefix(files[0].Staging, fixture("C:/st/file-mounts/v/")) || ms[len(ms)-1].MountPoint != "/tmp/.agent-vm-file-mounts/0" {
		t.Errorf("single file: %+v %+v", files, ms[len(ms)-1])
	}
}

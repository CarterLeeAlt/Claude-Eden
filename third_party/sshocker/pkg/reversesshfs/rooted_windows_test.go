package reversesshfs

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"
	"unsafe"

	"github.com/pkg/sftp"
	"golang.org/x/sys/windows"
)

func TestRootedWindowsRenameAndLinkNameLengths(t *testing.T) {
	operations := []struct {
		name          string
		apply         func(*sftp.Client, string, string) error
		retainsSource bool
	}{
		{"rename", (*sftp.Client).Rename, false},
		{"replace", (*sftp.Client).PosixRename, false},
		{"link", (*sftp.Client).Link, true},
	}
	for _, operation := range operations {
		for _, name := range []string{"a", "ab", "abc", "字", "🙂", strings.Repeat("x", 255)} {
			t.Run(fmt.Sprintf("%s/%d-units-%s", operation.name, len(utf16.Encode([]rune(name))), name[:min(len(name), 8)]), func(t *testing.T) {
				c, root := setupRooted(t, false)
				source, destination := filepath.Join(root, "src", "main.go"), filepath.Join(root, "src", name)
				if err := operation.apply(c, source, destination); err != nil {
					t.Fatalf("%s to %q (%d UTF-16 code units): %v", operation.name, name, len(utf16.Encode([]rune(name))), err)
				}
				if data, err := os.ReadFile(destination); err != nil || string(data) != "orig" {
					t.Fatalf("destination changed: %q %v", data, err)
				}
				if operation.retainsSource {
					src, err := os.Stat(source)
					if err != nil {
						t.Fatal(err)
					}
					dst, err := os.Stat(destination)
					if err != nil || !os.SameFile(src, dst) {
						t.Fatalf("link did not preserve file identity: %v", err)
					}
				} else if _, err := os.Stat(source); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("rename retained source: %v", err)
				}
				assertUnchanged(t, root)
			})
		}
	}
}

func TestWindowsSetNameUsesCompleteInformationBuffer(t *testing.T) {
	for _, class := range []uint32{windows.FileRenameInformation, fileRenameInformationEx, fileLinkInformation} {
		t.Run(fmt.Sprint(class), func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "source"), []byte("kept"), 0o600); err != nil {
				t.Fatal(err)
			}
			sys, err := openRootedSys(root)
			if err != nil {
				t.Fatal(err)
			}
			defer sys.close()
			source, err := ntCreate(sys.rootHandle, "source", windows.DELETE|windows.FILE_WRITE_ATTRIBUTES|windows.FILE_READ_ATTRIBUTES, 0, windows.FILE_OPEN, windows.FILE_NON_DIRECTORY_FILE, windows.OBJ_DONT_REPARSE)
			if err != nil {
				t.Fatal(err)
			}
			err = setName(source, class, 0, sys.rootHandle, "a")
			closeErr := windows.CloseHandle(source)
			if err != nil {
				t.Fatalf("class=%d, struct=%d bytes, name-end=%d bytes: %T %v", class, unsafe.Sizeof(fileNameInformation{}), unsafe.Offsetof(fileNameInformation{}.FileName)+2, err, err)
			}
			if closeErr != nil {
				t.Fatal(closeErr)
			}
			if data, err := os.ReadFile(filepath.Join(root, "a")); err != nil || string(data) != "kept" {
				t.Fatalf("destination: %q %v", data, err)
			}
		})
	}
}

func TestRootedWindowsReplaceShortOpenTarget(t *testing.T) {
	c, root := setupRooted(t, false)
	source, target := filepath.Join(root, "src", "main.go"), filepath.Join(root, "src", "字")
	old, err := c.Create(target)
	if err != nil {
		t.Fatal(err)
	}
	defer old.Close()
	if _, err := old.Write([]byte("prior")); err != nil {
		t.Fatal(err)
	}
	for _, operation := range []func(string, string) error{c.Rename, c.Link} {
		if err := operation(source, target); err == nil {
			t.Fatal("non-replacing operation overwrote an existing target")
		}
		if data, err := os.ReadFile(target); err != nil || string(data) != "prior" {
			t.Fatalf("existing target changed: %q %v", data, err)
		}
	}
	if err := c.PosixRename(source, target); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(target); err != nil || string(data) != "orig" {
		t.Fatalf("atomic replacement: %q %v", data, err)
	}
	previous := make([]byte, 5)
	if n, err := old.ReadAt(previous, 0); err != nil || n != 5 || string(previous) != "prior" {
		t.Fatalf("open target lost its original file: %q %v", previous, err)
	}
	if _, err := os.Stat(source); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("renamed source still exists: %v", err)
	}
	assertUnchanged(t, root)
}

func ctime(t *testing.T, p string) int64 {
	t.Helper()
	f, err := os.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var info fileBasicInformation
	if err := windows.GetFileInformationByHandleEx(windows.Handle(f.Fd()), windows.FileBasicInfo,
		(*byte)(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		t.Fatal(err)
	}
	return info.ChangeTime
}

func TestSlashPath(t *testing.T) {
	for in, want := range map[string]string{
		`C:\Users\foo`:       "/C:/Users/foo",
		`C:/Users/foo`:       "/C:/Users/foo",
		`/C:/Users/foo/`:     "/C:/Users/foo",
		`C:\`:                "/C:",
		`\\server\share\foo`: "/server/share/foo",
	} {
		if got := slashPath(in); got != want {
			t.Errorf("slashPath(%q) = %q, want %q", in, got, want)
		}
	}
	for in, want := range map[string]string{
		"C:/Users/foo/bar":  "/C:/Users/foo/bar",
		"/C:/Users/foo/bar": "/C:/Users/foo/bar",
		"bar":               "/C:/Users/foo/bar",
		".":                 "/C:/Users/foo",
	} {
		if got := realPath("/C:/Users/foo", in); got != want {
			t.Errorf("realPath(%q) = %q, want %q", in, got, want)
		}
	}
}

func shortBase(t *testing.T, p string) string {
	t.Helper()
	p16, err := windows.UTF16PtrFromString(p)
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]uint16, windows.MAX_PATH)
	n, err := windows.GetShortPathName(p16, &buf[0], uint32(len(buf)))
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Base(windows.UTF16ToString(buf[:n]))
}

// TestRootedDeniesShortNames writes through the 8.3 short name of .git (CVE-2019-1353),
// which passes the lexical check.
func TestRootedDeniesShortNames(t *testing.T) {
	type env struct {
		c         *sftp.Client
		root, git string // git is the short name of .git
		gitfile   string // the short name of src/.git
	}
	p := func(e env, s string) string { return filepath.Join(e.root, s) }
	open := func(e env, s string, flags int) error {
		f, err := e.c.OpenFile(p(e, s), flags)
		if err == nil {
			f.Close()
		}
		return err
	}
	attempts := []struct {
		name string
		f    func(e env) error
	}{
		{"create hook", func(e env) error { return open(e, e.git+"/hooks/pre-commit", os.O_RDWR|os.O_CREATE|os.O_TRUNC) }},
		{"open config for write", func(e env) error { return open(e, e.git+"/config", os.O_WRONLY) }},
		{"truncate on open", func(e env) error { return open(e, e.git+"/config", os.O_WRONLY|os.O_TRUNC) }},
		{"remove config", func(e env) error { return e.c.Remove(p(e, e.git+"/config")) }},
		{"rmdir hooks", func(e env) error { return e.c.RemoveDirectory(p(e, e.git+"/hooks")) }},
		{"rename .git away", func(e env) error { return e.c.PosixRename(p(e, e.git), p(e, "old")) }},
		{"rename into .git", func(e env) error { return e.c.PosixRename(p(e, "src/main.go"), p(e, e.git+"/hooks/x")) }},
		{"hardlink into hooks", func(e env) error { return e.c.Link(p(e, "src/main.go"), p(e, e.git+"/hooks/x")) }},
		{"chmod config", func(e env) error { return e.c.Chmod(p(e, e.git+"/config"), 0o444) }},
		{"truncate config", func(e env) error { return e.c.Truncate(p(e, e.git+"/config"), 0) }},
		{"write gitfile", func(e env) error { return open(e, "src/"+e.gitfile, os.O_WRONLY|os.O_TRUNC) }},
		{"replace gitfile", func(e env) error { return e.c.PosixRename(p(e, "src/main.go"), p(e, "src/"+e.gitfile)) }},
		{"remove gitfile", func(e env) error { return e.c.Remove(p(e, "src/"+e.gitfile)) }},
	}
	for _, a := range attempts {
		t.Run(a.name, func(t *testing.T) {
			c, root := setupRooted(t, false)
			gitfileLong := filepath.Join(root, "src", ".git")
			if err := os.WriteFile(gitfileLong, []byte("gitdir: x"), 0o644); err != nil {
				t.Fatal(err)
			}
			e := env{c: c, root: root, git: shortBase(t, filepath.Join(root, ".git")), gitfile: shortBase(t, gitfileLong)}
			if strings.EqualFold(e.git, ".git") || strings.EqualFold(e.gitfile, ".git") {
				t.Skip("8.3 names are not created on this volume")
			}
			if err := a.f(e); err == nil {
				t.Error("expected an error")
			}
			assertUnchanged(t, root)
			if b, err := os.ReadFile(gitfileLong); err != nil || string(b) != "gitdir: x" {
				t.Fatalf("src/.git changed: %q, %v", b, err)
			}
		})
	}
}

func TestRootedDeniesWindowsNames(t *testing.T) {
	junction := func(t *testing.T, root string) {
		t.Helper()
		// Unlike a symlink, a junction needs no privilege.
		out, err := exec.Command("cmd", "/c", "mklink", "/J", filepath.Join(root, "jn"), filepath.Join(root, ".git")).CombinedOutput()
		if err != nil {
			t.Fatalf("mklink: %v: %s", err, out)
		}
	}
	attempts := []struct {
		name  string
		setup func(t *testing.T, root string)
		path  string
	}{
		{"stream of .git", nil, ".git::$INDEX_ALLOCATION/hooks/pre-commit"},
		{"stream of a file", nil, "src/main.go:x"},
		{"backslashes", nil, `src\..\.git\hooks\pre-commit`},
		{"via junction", junction, "jn/hooks/pre-commit"},
		{"via junction to a file", junction, "jn/config"},
	}
	for _, a := range attempts {
		t.Run(a.name, func(t *testing.T) {
			c, root := setupRooted(t, false)
			if a.setup != nil {
				a.setup(t, root)
			}
			if f, err := c.OpenFile(root+`\`+a.path, os.O_RDWR|os.O_CREATE|os.O_TRUNC); err == nil {
				f.Close()
				t.Error("expected an error")
			}
			assertUnchanged(t, root)
			if fis, err := os.ReadDir(filepath.Join(root, "src")); err != nil || len(fis) != 1 {
				t.Fatalf("src entries changed: %v, %v", fis, err)
			}
		})
	}
}

// Names NtCreateFile creates but Win32 tools cannot open or delete.
func TestWritableNameWin32(t *testing.T) {
	for _, rel := range []string{"src/a.", "src/a ", "a./b", "NUL", "src/nul.txt", "aux", "Con .txt", "COM1", "lpt9.log", "COM¹", "CONIN$"} {
		if writableName(rel) {
			t.Errorf("%q: writable", rel)
		}
	}
	for _, rel := range []string{".", "src/main.go", "src/.a", "console", "COM10", "COMx", "nulx", "src/a.b"} {
		if !writableName(rel) {
			t.Errorf("%q: not writable", rel)
		}
	}
}

func TestRootedSymlinkUnsupported(t *testing.T) {
	c, root := setupRooted(t, false)
	if err := c.Symlink("main.go", filepath.Join(root, "src", "link")); err == nil {
		t.Fatal("expected an error")
	}
	if _, err := os.Lstat(filepath.Join(root, "src", "link")); err == nil {
		t.Fatal("the symlink was created")
	}
}

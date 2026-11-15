package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixtureSource(t *testing.T, source string) string {
	t.Helper()
	file := filepath.Join(t.TempDir(), "fixture_test.go")
	if err := os.WriteFile(file, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	return file
}

func TestMutationPaths(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		paths      []string
	}{
		{"original env accident", `t.Chdir(t.TempDir()); os.MkdirAll("C:/p",0755); os.Symlink("p","C:/link")`, []string{"C:/p", "C:/link"}},
		{"original mounts accident", `for _,d:=range []string{"C:/data","C:/data2","C:/data3","C:/data4","C:/data5","C:/data6","C:/st"} { os.MkdirAll(d,0755) }`, []string{"C:/data", "C:/data2", "C:/data3", "C:/data4", "C:/data5", "C:/data6", "C:/st"}},
		{"raw backslashes", "os.WriteFile(`C:\\outside\\file`,nil,0600)", []string{`C:\outside\file`}},
		{"Unix absolute path", `os.MkdirAll("/outside/data",0700)`, []string{"/outside/data"}},
		{"variable assignment constant concat", `const root="C:"; dir:=root; dir += "/outside"; file:=dir+"/file"; os.WriteFile(file,nil,0600)`, []string{"C:/outside/file"}},
		{"filepath join", `root:="C:/outside"; file:=filepath.Join(root,"nested","file"); os.Create(file)`, []string{"C:/outside/nested/file"}},
		{"absolute prefix with unknown suffix", `os.Create("C:/outside/"+dynamicName())`, []string{"C:/outside/<unknown>"}},
		{"joined loop", `for _,dir:=range []string{"C:/data","C:/data2"} { os.RemoveAll(filepath.Join(dir,"nested")) }`, []string{"C:/data/nested", "C:/data2/nested"}},
		{"renames check both ends", `os.Rename("C:/old","D:/new")`, []string{"C:/old", "D:/new"}},
		{"links check both ends", `os.Link("C:/source",filepath.Join(t.TempDir(),"copy")); os.Symlink("/outside/target","relative-link")`, []string{"C:/source", "/outside/target"}},
		{"permissions and removal", `os.Chmod("C:/private",0600); os.Chown("/outside",1,1); os.Remove("D:/retained")`, []string{"C:/private", "/outside", "D:/retained"}},
		{"read only open", `os.OpenFile("C:/existing",os.O_RDONLY,0)`, nil},
		{"write open flags", `flags:=os.O_RDWR|os.O_CREATE; os.OpenFile("C:/new",flags,0600)`, []string{"C:/new"}},
		{"file handle mutation", `f,_:=os.Open("C:/existing"); f.Chmod(0600); f.WriteString("data")`, []string{"C:/existing", "C:/existing"}},
		{"pure path parsing", `p:="C:/data"; filepath.Join(p,"child"); strings.Contains(p,"data"); if p=="/etc/test" { t.Fatal(p) }`, nil},
		{"private temp writes", `root:=t.TempDir(); os.MkdirAll(filepath.Join(root,"data"),0700); os.WriteFile(root+"/file",nil,0600); os.RemoveAll(root); f,_:=os.CreateTemp(root,"sample"); defer os.Remove(f.Name())`, nil},
		{"isolated temp environment", `root:=os.TempDir(); os.MkdirAll(filepath.Join(root,"data"),0700); os.MkdirTemp("","fixture"); os.CreateTemp("","fixture")`, nil},
		{"temp path escape", `os.Create(filepath.Join(t.TempDir(),"..","outside"))`, []string{"<test-temp>/../outside"}},
		{"closure cleanup", `p:="C:/outside"; t.Cleanup(func(){ os.RemoveAll(p) })`, []string{"C:/outside"}},
		{"subtest body", `t.Run("write",func(t *testing.T){ os.Create("C:/outside") })`, []string{"C:/outside"}},
		{"unknown branch union", `p:="C:/first"; if condition() { p="D:/second" }; os.Create(p)`, []string{"C:/first", "D:/second"}},
		{"conditional lists", `dirs:=[]string{"C:/first"}; if condition() { dirs=[]string{"D:/second"} }; for _,p:=range dirs { os.Create(p) }`, []string{"C:/first", "D:/second"}},
		{"test case fields", `for _,tc:=range []struct{ path string }{{"C:/one"},{"C:/two"}} { os.Create(tc.path) }`, []string{"C:/one", "C:/two"}},
		{"known helper call", `write("C:/outside")`, []string{"C:/outside"}},
		{"shadowed import", `os:=fake(); os.Create("C:/not-a-filesystem-call")`, nil},
		{"mutation alias", `mkdir:=os.MkdirAll; mkdir("C:/outside",0700)`, []string{"C:/outside"}},
		{"field assignment", `state:=struct{root string}{t.TempDir()}; state.root=filepath.Join(t.TempDir(),"nested"); os.Create(state.root+"-sibling")`, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := "package fixture\nimport(\"os\";\"testing\";\"path/filepath\";\"strings\")\nfunc write(p string){ os.Create(p) }\nfunc TestFixture(t *testing.T){" + tc.body + "}\n"
			file := fixtureSource(t, source)
			got, err := scan([]string{file})
			if err != nil {
				t.Fatal(err)
			}
			if len(got.findings) != len(tc.paths) {
				t.Fatalf("findings=%+v, want %v", got.findings, tc.paths)
			}
			remaining := append([]string(nil), tc.paths...)
			for _, f := range got.findings {
				if f.file != file || f.line < 3 {
					t.Fatalf("missing source location: %+v", f)
				}
				found := -1
				for i, p := range remaining {
					if p == f.target {
						found = i
						break
					}
				}
				if found < 0 {
					t.Fatalf("unexpected path: %+v", f)
				}
				remaining = append(remaining[:found], remaining[found+1:]...)
			}
		})
	}
}

func TestTemporaryDriveMapping(t *testing.T) {
	source := `package fixture
import("os";"testing";"runtime";"strings";"github.com/sylvinus/agent-vm/internal/paths")
func windowsFixtureRoot(t *testing.T) string {
 dir:=t.TempDir(); t.Chdir(dir)
 if runtime.GOOS=="windows" { return paths.Host(dir) }
 return "C:"
}
func TestFixture(t *testing.T) {
 old:=paths.Windows
 paths.Windows=func()bool{return true}
 t.Cleanup(func(){paths.Windows=old})
 root:=windowsFixtureRoot(t)
 fixture:=strings.NewReplacer("C:/",root+"/","C:\\",strings.ReplaceAll(root,"/","\\")+"\\","/c/",paths.Guest(root)+"/").Replace
 for _,d:=range []string{"C:/Users/me/proj","C:/data","C:/data2","C:/st"} { os.MkdirAll(fixture(d),0755) }
 os.WriteFile(fixture("C:/f.txt"),nil,0600)
 os.MkdirAll(root+"/p",0755)
 os.Symlink("p",root+"/link")
}`
	got, err := scan([]string{fixtureSource(t, source)})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.findings) != 0 {
		t.Fatalf("private mapping rejected: %+v", got.findings)
	}
	if len(got.unresolved) != 0 {
		t.Fatalf("private mapping was not proved: %v", got.unresolved)
	}
}

func TestImportsGlobalsAndUnknownPaths(t *testing.T) {
	file := fixtureSource(t, `package fixture
import(fs "os"; . "io/ioutil"; "testing")
const base="C:/outside"
func TestFixture(t *testing.T){ fs.Create(base); WriteFile("/etc/outside",nil,0600); fs.Create(dynamicPath()) }
`)
	got, err := scan([]string{file})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.findings) != 2 || len(got.unresolved) != 1 {
		t.Fatalf("aliases or unknown accounting: %+v", got)
	}
	var output, diagnostic bytes.Buffer
	if code := run([]string{file}, &output, &diagnostic); code != 1 {
		t.Fatalf("exit=%d", code)
	}
	if !strings.Contains(diagnostic.String(), file+":4:") || !strings.Contains(diagnostic.String(), `"C:/outside"`) || !strings.Contains(output.String(), "not an OS sandbox") {
		t.Fatal(output.String(), diagnostic.String())
	}
}

func TestRepositoryTraversalAndParseFailure(t *testing.T) {
	root := t.TempDir()
	for _, directory := range []string{"internal/fixture", "third_party/dependency/pkg", ".git", ".tmp", "_output", "node_modules", "bin", "obj", "dist", "build", "target"} {
		dir := filepath.Join(root, filepath.FromSlash(directory))
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "write_test.go"), []byte(`package p; import "os"; func TestWrite(){os.Create("C:/outside")}`), 0600); err != nil {
			t.Fatal(err)
		}
	}
	got, err := scan([]string{root})
	if err != nil {
		t.Fatal(err)
	}
	if got.files != 2 || len(got.findings) != 2 {
		t.Fatalf("traversal: %+v", got)
	}
	bad := filepath.Join(root, "syntax_test.go")
	if err := os.WriteFile(bad, []byte("package broken; func"), 0600); err != nil {
		t.Fatal(err)
	}
	var output, diagnostic bytes.Buffer
	if code := run([]string{bad}, &output, &diagnostic); code != 2 {
		t.Fatalf("parse failure exit=%d", code)
	}
}

func TestTargetBuildConstraints(t *testing.T) {
	root := t.TempDir()
	for name, source := range map[string]string{
		"write_windows_test.go":     "package p; import \"os\"; func TestWindows(){os.Create(`C:\\outside`)}",
		"write_linux_arm64_test.go": "package p; import \"os\"; func TestARM(){os.Create(\"/outside/arm\")}",
		"syntax_unix_test.go":       "//go:build !windows\n\npackage p; import \"os\"; func TestRelativeDrive(){os.Create(\"C:/relative-on-unix\")}",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
	}
	got, err := scan([]string{root})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.findings) != 2 {
		t.Fatalf("target-aware scan: %+v", got.findings)
	}
}

func TestTemporaryKernelPTYException(t *testing.T) {
	const open = `master,_:=os.OpenFile("/dev/ptmx",os.O_RDWR|unix.O_NOCTTY,0); defer master.Close();`
	for _, tc := range []struct {
		name, filename, build, body string
		unsafe                      int
	}{
		{"Linux filename", "pty_linux_test.go", "", open, 0},
		{"Linux architecture", "pty_linux_arm64_test.go", "", open, 0},
		{"Linux tag", "pty_test.go", "linux", open, 0},
		{"Linux tag expression", "pty_test.go", "linux && (amd64 || arm64)", open, 0},
		{"Linux read only", "pty_linux_test.go", "", `os.OpenFile("/dev/ptmx",os.O_RDONLY,0)`, 0},
		{"Linux without noctty", "pty_linux_test.go", "", `os.OpenFile("/dev/ptmx",os.O_RDWR,0)`, 0},
		{"portable test", "pty_test.go", "", open, 1},
		{"not Windows is not Linux", "pty_test.go", "!windows", open, 1},
		{"Unix is not Linux", "pty_test.go", "unix", open, 1},
		{"OS tag disjunction", "pty_test.go", "linux || darwin", open, 1},
		{"misleading filename", "pty_linux_fixture_test.go", "", open, 1},
		{"portable read only", "pty_test.go", "", `os.OpenFile("/dev/ptmx",os.O_RDONLY,0)`, 1},
		{"ordinary file", "pty_linux_test.go", "", `os.OpenFile("/tmp/ordinary",os.O_RDWR|unix.O_NOCTTY,0)`, 1},
		{"prefix is not exact", "pty_linux_test.go", "", `os.OpenFile("/dev/ptmx-other",os.O_RDWR,0)`, 1},
		{"child is not exact", "pty_linux_test.go", "", `os.OpenFile("/dev/ptmx/file",os.O_RDWR,0)`, 1},
		{"create", "pty_linux_test.go", "", `os.OpenFile("/dev/ptmx",os.O_RDWR|os.O_CREATE,0600)`, 1},
		{"truncate", "pty_linux_test.go", "", `os.OpenFile("/dev/ptmx",os.O_RDWR|os.O_TRUNC,0)`, 1},
		{"read only create", "pty_linux_test.go", "", `os.OpenFile("/dev/ptmx",os.O_CREATE,0600)`, 1},
		{"append", "pty_linux_test.go", "", `os.OpenFile("/dev/ptmx",os.O_RDWR|os.O_APPEND,0)`, 1},
		{"exclusive", "pty_linux_test.go", "", `os.OpenFile("/dev/ptmx",os.O_RDWR|os.O_EXCL,0)`, 1},
		{"unknown flags", "pty_linux_test.go", "", `os.OpenFile("/dev/ptmx",flags(),0)`, 1},
		{"unknown flag bits", "pty_linux_test.go", "", `os.OpenFile("/dev/ptmx",os.O_RDWR|flags(),0)`, 1},
		{"conditional flags", "pty_linux_test.go", "", `flags:=os.O_RDWR; if condition(){flags=os.O_RDWR|os.O_TRUNC}; os.OpenFile("/dev/ptmx",flags,0)`, 1},
		{"conditional unrecognized flag", "pty_linux_test.go", "", `flags:=os.O_RDWR; if condition(){flags=unix.O_UNRECOGNIZED}; os.OpenFile("/dev/ptmx",flags,0)`, 1},
		{"syscall has no exception", "pty_linux_test.go", "", `unix.Open("/dev/ptmx",unix.O_RDWR|unix.O_NOCTTY,0)`, 1},
		{"remove", "pty_linux_test.go", "", open + `os.Remove("/dev/ptmx")`, 1},
		{"remove all", "pty_linux_test.go", "", open + `os.RemoveAll("/dev/ptmx")`, 1},
		{"rename source", "pty_linux_test.go", "", open + `os.Rename("/dev/ptmx",filepath.Join(t.TempDir(),"device"))`, 1},
		{"rename target", "pty_linux_test.go", "", open + `os.Rename(filepath.Join(t.TempDir(),"device"),"/dev/ptmx")`, 1},
		{"chmod", "pty_linux_test.go", "", open + `os.Chmod("/dev/ptmx",0600)`, 1},
		{"handle chmod", "pty_linux_test.go", "", open + `master.Chmod(0600)`, 1},
		{"handle truncate", "pty_linux_test.go", "", open + `master.Truncate(0)`, 1},
		{"handle name removal", "pty_linux_test.go", "", open + `os.Remove(master.Name())`, 1},
		{"unrelated mutation same file", "pty_linux_test.go", "", open + `os.WriteFile("/tmp/ordinary",nil,0600)`, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := "package fixture\nimport(\"os\";\"testing\";\"path/filepath\";\"golang.org/x/sys/unix\")\nfunc TestPTY(t *testing.T){" + tc.body + "}\n"
			if tc.build != "" {
				source = "//go:build " + tc.build + "\n\n" + source
			}
			file := filepath.Join(t.TempDir(), tc.filename)
			if err := os.WriteFile(file, []byte(source), 0600); err != nil {
				t.Fatal(err)
			}
			got, err := scan([]string{file})
			if err != nil {
				t.Fatal(err)
			}
			if len(got.findings) != tc.unsafe {
				t.Fatalf("findings=%+v, want %d unsafe paths", got.findings, tc.unsafe)
			}
		})
	}
}

func TestUnresolvedParentEvidence(t *testing.T) {
	file := fixtureSource(t, `package fixture
import("os";"testing";"path/filepath")
func TestPaths(t *testing.T) {
 os.Create(filepath.Join(os.Getenv("FIXTURE_ROOT"),"child"))
 os.Create(filepath.Join(t.TempDir(),dynamicName()))
 os.Create(filepath.Join("relative-root",dynamicName()))
}`)
	var output, diagnostic bytes.Buffer
	if code := run([]string{"-unresolved", file}, &output, &diagnostic); code != 0 {
		t.Fatalf("exit=%d: %s", code, diagnostic.String())
	}
	for _, expected := range []string{"3 unresolved mutation sites", "parent-unknown: 1", "temp-prefix-partial: 1", "relative-prefix-partial: 1", `os.Getenv(\"FIXTURE_ROOT\")`, "<unknown>/child", "<test-temp>/<unknown>"} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("missing %q in %s", expected, output.String())
		}
	}
}

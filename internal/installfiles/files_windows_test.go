package installfiles

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func writeFixture(t *testing.T, name, content string) {
	t.Helper()
	if e := os.MkdirAll(filepath.Dir(name), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(name, []byte(content), 0600); e != nil {
		t.Fatal(e)
	}
}
func text(t *testing.T, name string) string {
	t.Helper()
	b, e := os.ReadFile(name)
	if e != nil {
		t.Fatal(e)
	}
	return string(b)
}
func requireText(t *testing.T, name, want string) {
	t.Helper()
	if got := text(t, name); got != want {
		t.Fatalf("%s: got %q, want %q", name, got, want)
	}
}
func digest(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }
func app(t *testing.T, root, version string) {
	t.Helper()
	m := Manifest{Version: 1}
	for name, content := range map[string]string{IdentityName: Identity, "program.txt": version, "resources/customizable.txt": "default-" + version, "Uninstall.exe": "uninstaller-" + version} {
		writeFixture(t, filepath.Join(root, filepath.FromSlash(name)), content)
		m.Files = append(m.Files, File{name, digest(content)})
	}
	if e := writeJSON(filepath.Join(root, ManifestName), m, false); e != nil {
		t.Fatal(e)
	}
}
func run(t *testing.T, o Options) {
	t.Helper()
	if e := Run(context.Background(), o); e != nil {
		t.Fatalf("%s: %v", o.Operation, e)
	}
}
func commit(t *testing.T, old, stage, receipt string, keep bool) {
	t.Helper()
	backup := old + "-backup"
	if keep {
		run(t, Options{Operation: "detach", Source: old, Target: backup, Receipt: receipt})
		run(t, Options{Operation: "move", Source: backup, Target: stage, Receipt: receipt})
		run(t, Options{Operation: "attach", Source: stage, Target: old})
	} else {
		if e := os.Rename(old, backup); e != nil {
			t.Fatal(e)
		}
		run(t, Options{Operation: "move", Source: backup, Target: stage, Receipt: receipt})
		if e := os.Rename(stage, old); e != nil {
			t.Fatal(e)
		}
	}
	run(t, Options{Operation: "cleanup", Source: backup, Target: old, Receipt: receipt})
	if exists(backup) || exists(receipt) {
		t.Fatal("finished transaction retained backup or receipt")
	}
}
func preservedTexts(t *testing.T, root string) map[string]int {
	t.Helper()
	result := map[string]int{}
	e := filepath.WalkDir(filepath.Join(root, "Eden-preserved-files"), func(name string, entry os.DirEntry, e error) error {
		if errors.Is(e, os.ErrNotExist) {
			return nil
		}
		if e != nil {
			return e
		}
		if entry.Type()&os.ModeSymlink != 0 {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !entry.IsDir() {
			result[text(t, name)]++
		}
		return nil
	})
	if e != nil {
		t.Fatal(e)
	}
	return result
}

func TestManifestValidation(t *testing.T) {
	root := t.TempDir()
	name := filepath.Join(root, ManifestName)
	for _, relative := range []string{"", "/absolute", "../outside", "a/../b", "a//b", "a/./b", `a\b`, "a:stream", "CON", "a./b", "x/COM1.txt", "a\x00b"} {
		if ValidRelative(relative) {
			t.Errorf("accepted invalid relative: %q", relative)
		}
	}
	for _, relative := range []string{"Eden.exe", "resources/program.dll", "my-project/项目.txt"} {
		if !ValidRelative(relative) {
			t.Errorf("rejected safe path %q", relative)
		}
	}
	for _, body := range []string{`{broken`, `{"version":2,"files":[]}`, `{"version":1,"files":[{"path":"../escape","sha256":"` + digest("x") + `"}]}`, `{"version":1,"files":[{"path":"a","sha256":"bad"}]}`, `{"version":1,"files":[{"path":"a","sha256":"` + digest("x") + `"},{"path":"A","sha256":"` + digest("x") + `"}]}`} {
		writeFixture(t, name, body)
		if len(ReadManifest(name)) != 0 {
			t.Fatalf("invalid manifest grants ownership: %s", body)
		}
	}
}

func TestInstallationOwnershipLifecycle(t *testing.T) {
	for _, manifestState := range []string{"current", "missing", "invalid"} {
		t.Run(manifestState, func(t *testing.T) {
			root := t.TempDir()
			old, stage, receipt := filepath.Join(root, "installed"), filepath.Join(root, "staged"), filepath.Join(root, "receipt.json")
			app(t, old, "old")
			app(t, stage, "new")
			if manifestState == "missing" {
				if e := os.Remove(filepath.Join(old, ManifestName)); e != nil {
					t.Fatal(e)
				}
			}
			if manifestState == "invalid" {
				writeFixture(t, filepath.Join(old, ManifestName), "{broken")
			}
			writeFixture(t, filepath.Join(old, "my-project", "work.txt"), "project data")
			writeFixture(t, filepath.Join(old, "resources", "customizable.txt"), "user customization")
			writeFixture(t, filepath.Join(old, "EdenData", "state", "disk"), "persistent disk")
			if e := os.Mkdir(filepath.Join(old, "empty-project"), 0700); e != nil {
				t.Fatal(e)
			}
			run(t, Options{Operation: "prepare", Source: old, Target: stage, Receipt: receipt, Backup: old + "-backup"})
			requireText(t, filepath.Join(old, "program.txt"), "old")
			commit(t, old, stage, receipt, false)
			requireText(t, filepath.Join(old, "program.txt"), "new")
			requireText(t, filepath.Join(old, "my-project", "work.txt"), "project data")
			requireText(t, filepath.Join(old, "EdenData", "state", "disk"), "persistent disk")
			if !exists(filepath.Join(old, "empty-project")) {
				t.Fatal("empty user directory lost")
			}
			if preservedTexts(t, old)["user customization"] != 1 {
				t.Fatal("modified program content not retained exactly once")
			}
			embedded := filepath.Join(root, "embedded.json")
			writeFixture(t, embedded, text(t, filepath.Join(old, ManifestName)))
			writeFixture(t, filepath.Join(old, "resources", "customizable.txt"), "new customization")
			run(t, Options{Operation: "uninstall", Target: old, Manifest: embedded})
			if exists(filepath.Join(old, "program.txt")) {
				t.Fatal("unchanged program was not removed")
			}
			requireText(t, filepath.Join(old, "resources", "customizable.txt"), "new customization")
			requireText(t, filepath.Join(old, "my-project", "work.txt"), "project data")
			requireText(t, filepath.Join(old, "EdenData", "state", "disk"), "persistent disk")
			if !exists(filepath.Join(old, IdentityName)) {
				t.Fatal("data-bearing installation identity lost")
			}
			app(t, stage, "reinstalled")
			run(t, Options{Operation: "prepare", Source: old, Target: stage, Receipt: receipt, Backup: old + "-backup"})
			commit(t, old, stage, receipt, false)
			requireText(t, filepath.Join(old, "my-project", "work.txt"), "project data")
		})
	}
}

func TestFirstInstallAndUnmodifiedUninstall(t *testing.T) {
	root := t.TempDir()
	target, stage, receipt := filepath.Join(root, "target"), filepath.Join(root, "stage"), filepath.Join(root, "receipt.json")
	if e := os.Mkdir(target, 0700); e != nil {
		t.Fatal(e)
	}
	app(t, stage, "fresh")
	run(t, Options{Operation: "prepare", Source: target, Target: stage, Receipt: receipt})
	r, e := readReceipt(receipt)
	if e != nil {
		t.Fatal(e)
	}
	if r.BackupPath != "" || len(r.Moves) != 0 {
		t.Fatal("first install invented backup or personal moves")
	}
	if e = os.Remove(target); e != nil {
		t.Fatal(e)
	}
	if e = os.Rename(stage, target); e != nil {
		t.Fatal(e)
	}
	run(t, Options{Operation: "complete", Target: target, Receipt: receipt})
	embedded := filepath.Join(root, "embedded.json")
	writeFixture(t, embedded, text(t, filepath.Join(target, ManifestName)))
	run(t, Options{Operation: "uninstall", Target: target, Manifest: embedded})
	if exists(target) {
		t.Fatal("unmodified install was not fully removed")
	}
}

func TestConcurrentChangesPreserved(t *testing.T) {
	root := t.TempDir()
	old, stage, receipt := filepath.Join(root, "installed"), filepath.Join(root, "staged"), filepath.Join(root, "receipt.json")
	app(t, old, "old")
	app(t, stage, "new")
	writeFixture(t, filepath.Join(old, "project.txt"), "first")
	run(t, Options{Operation: "prepare", Source: old, Target: stage, Receipt: receipt, Backup: old + "-backup"})
	writeFixture(t, filepath.Join(old, "project.txt"), "late edit")
	writeFixture(t, filepath.Join(old, "new-project.txt"), "created during install")
	writeFixture(t, filepath.Join(old, "program.txt"), "late program edit")
	if e := os.Mkdir(filepath.Join(old, "late-empty-project"), 0700); e != nil {
		t.Fatal(e)
	}
	commit(t, old, stage, receipt, false)
	requireText(t, filepath.Join(old, "project.txt"), "late edit")
	p := preservedTexts(t, old)
	if p["created during install"] != 1 || p["late program edit"] != 1 {
		t.Fatal("concurrent files not retained")
	}
	found := false
	_ = filepath.WalkDir(filepath.Join(old, "Eden-preserved-files"), func(name string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.Name() == "late-empty-project" {
			found = true
		}
		return nil
	})
	if !found {
		t.Fatal("late empty project directory lost")
	}
}

func TestMoveFailureReversesPersonalFiles(t *testing.T) {
	root := t.TempDir()
	old, stage, receipt := filepath.Join(root, "installed"), filepath.Join(root, "staged"), filepath.Join(root, "receipt.json")
	app(t, old, "old")
	app(t, stage, "new")
	for _, p := range []string{"first-project", "second-project"} {
		writeFixture(t, filepath.Join(old, p, "work.txt"), p)
	}
	run(t, Options{Operation: "prepare", Source: old, Target: stage, Receipt: receipt, Backup: old + "-backup"})
	r, e := readReceipt(receipt)
	if e != nil {
		t.Fatal(e)
	}
	last := r.Moves[len(r.Moves)-1]
	backup := old + "-backup"
	if e = os.Rename(old, backup); e != nil {
		t.Fatal(e)
	}
	if e = os.MkdirAll(filepath.Join(stage, filepath.FromSlash(last.Destination)), 0700); e != nil {
		t.Fatal(e)
	}
	if e = Run(context.Background(), Options{Operation: "move", Source: backup, Target: stage, Receipt: receipt}); e == nil {
		t.Fatal("expected destination collision")
	}
	run(t, Options{Operation: "restore", Source: stage, Target: backup, Receipt: receipt})
	if e = os.Rename(backup, old); e != nil {
		t.Fatal(e)
	}
	for _, p := range []string{"first-project", "second-project"} {
		requireText(t, filepath.Join(old, p, "work.txt"), p)
	}
	run(t, Options{Operation: "complete", Target: old, Receipt: receipt})
}

func TestKeptDataAndDetachFailure(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(fmtBool(fail), func(t *testing.T) {
			root := t.TempDir()
			old, stage, receipt := filepath.Join(root, "installed"), filepath.Join(root, "staged"), filepath.Join(root, "receipt.json")
			app(t, old, "old")
			app(t, stage, "new")
			diskPath := filepath.Join(old, "EdenData", "vm", "disk.qcow2")
			writeFixture(t, diskPath, "persistent disk")
			writeFixture(t, filepath.Join(old, "my-project", "work.txt"), "project")
			disk, e := OpenReadLocked(diskPath)
			if e != nil {
				t.Fatal(e)
			}
			defer disk.Close()
			run(t, Options{Operation: "prepare", Source: old, Target: stage, Receipt: receipt, Backup: old + "-backup", KeepInPlace: []string{"EdenData"}})
			r, e := readReceipt(receipt)
			if e != nil {
				t.Fatal(e)
			}
			for _, m := range r.Moves {
				if strings.HasPrefix(m.Path, "EdenData") {
					t.Fatal("kept disk scheduled for move")
				}
			}
			if fail {
				held, e := OpenReadLocked(filepath.Join(old, "program.txt"))
				if e != nil {
					t.Fatal(e)
				}
				diagnostic := filepath.Join(root, "diagnostic.json")
				e = Run(context.Background(), Options{Operation: "detach", Source: old, Target: old + "-backup", Receipt: receipt, Diagnostics: diagnostic})
				held.Close()
				if e == nil {
					t.Fatal("locked program did not block detach")
				}
				if exists(old + "-backup") {
					t.Fatal("failed detach left backup")
				}
				requireText(t, filepath.Join(old, "program.txt"), "old")
				requireText(t, filepath.Join(old, "my-project", "work.txt"), "project")
				var d struct {
					Blocked string `json:"blocked"`
				}
				if e = json.Unmarshal([]byte(text(t, diagnostic)), &d); e != nil {
					t.Fatal(e)
				}
				if d.Blocked != "program" || strings.Contains(text(t, diagnostic), root) {
					t.Fatal("unsafe or incorrect diagnostic")
				}
				run(t, Options{Operation: "complete", Target: old, Receipt: receipt})
			} else {
				commit(t, old, stage, receipt, true)
				requireText(t, filepath.Join(old, "program.txt"), "new")
			}
			requireText(t, diskPath, "persistent disk")
		})
	}
}
func fmtBool(v bool) string {
	if v {
		return "failure"
	}
	return "success"
}

func TestTypeConflictAndLegacyRollback(t *testing.T) {
	t.Run("type-conflict", func(t *testing.T) {
		root := t.TempDir()
		old, stage, receipt := filepath.Join(root, "installed"), filepath.Join(root, "staged"), filepath.Join(root, "receipt.json")
		app(t, old, "old")
		app(t, stage, "new")
		if e := os.Remove(filepath.Join(old, ManifestName)); e != nil {
			t.Fatal(e)
		}
		writeFixture(t, filepath.Join(old, "obsolete.dll"), "old library")
		writeFixture(t, filepath.Join(old, "resources", "new-tool", "data.txt"), "user directory")
		writeFixture(t, filepath.Join(stage, "resources", "new-tool"), "new program")
		run(t, Options{Operation: "prepare", Source: old, Target: stage, Receipt: receipt, Backup: old + "-backup"})
		commit(t, old, stage, receipt, false)
		if exists(filepath.Join(old, "obsolete.dll")) {
			t.Fatal("unknown library remained in loader path")
		}
		requireText(t, filepath.Join(old, "resources", "new-tool"), "new program")
		p := preservedTexts(t, old)
		if p["old library"] != 1 || p["user directory"] != 1 {
			t.Fatal("conflicting bytes lost")
		}
	})
	t.Run("legacy-rollback", func(t *testing.T) {
		root := t.TempDir()
		old, stage, receipt := filepath.Join(root, "installed"), filepath.Join(root, "restored"), filepath.Join(root, "receipt.json")
		app(t, stage, "old program")
		if e := os.Remove(filepath.Join(stage, ManifestName)); e != nil {
			t.Fatal(e)
		}
		writeFixture(t, filepath.Join(stage, "my-project", "work.txt"), "before attempted update")
		writeFixture(t, filepath.Join(stage, "personal.txt"), "old personal")
		app(t, old, "failed new program")
		writeFixture(t, filepath.Join(old, "my-project", "work.txt"), "latest project")
		writeFixture(t, filepath.Join(old, "personal.txt"), "latest personal")
		run(t, Options{Operation: "prepare", Source: old, Target: stage, Receipt: receipt, Backup: old + "-backup", LegacyTarget: true})
		commit(t, old, stage, receipt, false)
		requireText(t, filepath.Join(old, "program.txt"), "old program")
		requireText(t, filepath.Join(old, "my-project", "work.txt"), "latest project")
		requireText(t, filepath.Join(old, "personal.txt"), "latest personal")
		p := preservedTexts(t, old)
		if p["before attempted update"] != 1 || p["old personal"] != 1 {
			t.Fatal("old personal versions lost")
		}
	})
}

func TestVerifiedHandleDeletionRace(t *testing.T) {
	root := t.TempDir()
	original, replaced := filepath.Join(root, "program.dll"), filepath.Join(root, "original-renamed.dll")
	writeFixture(t, original, "verified old program")
	f, e := OpenVerified(root, "program.dll")
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	h, e := Hash(f)
	if e != nil {
		t.Fatal(e)
	}
	if h != digest("verified old program") {
		t.Fatal("incorrect hash")
	}
	if e = os.Rename(original, replaced); e != nil {
		t.Fatal(e)
	}
	writeFixture(t, original, "new personal data")
	if e = DeleteVerified(f); e != nil {
		t.Fatal(e)
	}
	if e = f.Close(); e != nil {
		t.Fatal(e)
	}
	requireText(t, original, "new personal data")
	if exists(replaced) {
		t.Fatal("verified object was not deleted")
	}
}

func makeJunction(t *testing.T, link, target string) {
	t.Helper()
	if e := os.Mkdir(link, 0700); e != nil {
		t.Fatal(e)
	}
	p, e := windows.UTF16PtrFromString(link)
	if e != nil {
		t.Fatal(e)
	}
	h, e := windows.CreateFile(p, windows.GENERIC_WRITE, 0, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if e != nil {
		t.Fatal(e)
	}
	defer windows.CloseHandle(h)
	substitute, e := windows.UTF16FromString(`\??\` + target)
	if e != nil {
		t.Fatal(e)
	}
	printName, e := windows.UTF16FromString(target)
	if e != nil {
		t.Fatal(e)
	}
	buf := make([]byte, 16+2*(len(substitute)+len(printName)))
	binary.LittleEndian.PutUint32(buf, windows.IO_REPARSE_TAG_MOUNT_POINT)
	binary.LittleEndian.PutUint16(buf[4:], uint16(len(buf)-8))
	binary.LittleEndian.PutUint16(buf[10:], uint16(2*(len(substitute)-1)))
	binary.LittleEndian.PutUint16(buf[12:], uint16(2*len(substitute)))
	binary.LittleEndian.PutUint16(buf[14:], uint16(2*(len(printName)-1)))
	for i, v := range append(substitute, printName...) {
		binary.LittleEndian.PutUint16(buf[16+2*i:], v)
	}
	var returned uint32
	if e = windows.DeviceIoControl(h, windows.FSCTL_SET_REPARSE_POINT, (*byte)(unsafe.Pointer(&buf[0])), uint32(len(buf)), nil, 0, &returned, nil); e != nil {
		t.Fatal(e)
	}
}

func TestReparseTraversalProtection(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(root, "outside")
	writeFixture(t, filepath.Join(outside, "owned.dll"), "external target")
	installed := filepath.Join(root, "installed")
	app(t, installed, "old")
	embedded := filepath.Join(root, "embedded.json")
	writeFixture(t, embedded, text(t, filepath.Join(installed, ManifestName)))
	resources := filepath.Join(installed, "resources")
	if e := os.Remove(filepath.Join(resources, "customizable.txt")); e != nil {
		t.Fatal(e)
	}
	if e := os.Remove(resources); e != nil {
		t.Fatal(e)
	}
	makeJunction(t, resources, outside)
	if f, e := OpenVerified(installed, "resources/owned.dll"); e == nil {
		f.Close()
		t.Fatal("verified open followed junction ancestor")
	}
	run(t, Options{Operation: "uninstall", Target: installed, Manifest: embedded})
	requireText(t, filepath.Join(outside, "owned.dll"), "external target")
	if link, e := isReparse(resources); e != nil || !link {
		t.Fatal("uninstall changed personal junction", e)
	}
	actual := filepath.Join(root, "actual")
	app(t, actual, "old")
	linked := filepath.Join(root, "linked-installation")
	makeJunction(t, linked, actual)
	stage := filepath.Join(root, "stage")
	app(t, stage, "new")
	receipt := filepath.Join(root, "receipt.json")
	if e := Run(context.Background(), Options{Operation: "prepare", Source: linked, Target: stage, Receipt: receipt}); e == nil {
		t.Fatal("accepted linked install root")
	}
	if exists(receipt) {
		t.Fatal("published receipt for linked root")
	}
	requireText(t, filepath.Join(actual, "program.txt"), "old")
}

func TestJunctionIdentityPreservedByUpdateAndRollback(t *testing.T) {
	root := t.TempDir()
	old, stage, receipt := filepath.Join(root, "installed"), filepath.Join(root, "stage"), filepath.Join(root, "receipt.json")
	app(t, old, "old")
	app(t, stage, "new")
	outside := filepath.Join(root, "outside")
	writeFixture(t, filepath.Join(outside, "work.txt"), "external project")
	makeJunction(t, filepath.Join(old, "linked-project"), outside)
	run(t, Options{Operation: "prepare", Source: old, Target: stage, Receipt: receipt, Backup: old + "-backup"})
	commit(t, old, stage, receipt, false)
	if link, e := isReparse(filepath.Join(old, "linked-project")); e != nil || !link {
		t.Fatal("link identity lost", e)
	}
	requireText(t, filepath.Join(old, "linked-project", "work.txt"), "external project")
	embedded := filepath.Join(root, "embedded.json")
	writeFixture(t, embedded, text(t, filepath.Join(old, ManifestName)))
	run(t, Options{Operation: "uninstall", Target: old, Manifest: embedded})
	requireText(t, filepath.Join(outside, "work.txt"), "external project")
	if link, e := isReparse(filepath.Join(old, "linked-project")); e != nil || !link {
		t.Fatal("uninstall removed project junction", e)
	}
}

func TestCancellationAndInvalidReceipt(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	app(t, target, "new")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	receipt := filepath.Join(root, "receipt.json")
	if e := Run(ctx, Options{Operation: "prepare", Target: target, Receipt: receipt}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	if exists(receipt) {
		t.Fatal("canceled preparation wrote receipt")
	}
	for _, op := range []string{"move", "restore", "cleanup", "complete", "detach"} {
		writeFixture(t, receipt, `{"kind":"eden-installation","version":1,"moves":[{"path":"../outside","destination":"safe","state":"prepared"}]}`)
		if e := Run(context.Background(), Options{Operation: op, Target: target, Source: filepath.Join(root, "source"), Receipt: receipt}); e == nil {
			t.Fatalf("%s accepted traversal receipt", op)
		}
	}
}

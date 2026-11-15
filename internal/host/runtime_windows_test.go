package host

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"golang.org/x/sys/windows"
)

func TestSystemPowerShellLoadsOwnModulesAfterAnIntermediary(t *testing.T) {
	directory := t.TempDir()
	modules := filepath.Join(directory, "modules")
	module := filepath.Join(modules, "Microsoft.PowerShell.Utility")
	if err := os.MkdirAll(module, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(module, "Microsoft.PowerShell.Utility.psd1"), []byte("@{ ModuleVersion='1.0'; PowerShellVersion='7.0'; RootModule='fixture.psm1' }"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(module, "fixture.psm1"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PSModulePath", modules)
	file := filepath.Join(directory, "module-path-fixture.txt")
	content := []byte("native module lookup")
	if err := os.WriteFile(file, content, 0600); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(directory, "read-hash.ps1")
	if err := os.WriteFile(script, []byte("param([string]$Path)\n$ErrorActionPreference='Stop'\n[Console]::Write((Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash)\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cmd, err := SystemPowerShellCommand(t.Context(), "-NoProfile", "-NonInteractive", "-File", script, "-Path", file)
	if err != nil {
		t.Fatal(err)
	}
	HideCommandWindow(cmd)
	output, err := cmd.CombinedOutput()
	if err != nil || !strings.EqualFold(strings.TrimSpace(string(output)), fmt.Sprintf("%x", sha256.Sum256(content))) {
		t.Fatalf("Windows PowerShell module loading failed: %v\n%s", err, output)
	}
	if os.Getenv("PSModulePath") != modules {
		t.Fatal("child environment changed the caller's module configuration")
	}
}

func TestHideCommandWindowPreservesProcessAttributes(t *testing.T) {
	attributes := &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_PROCESS_GROUP}
	cmd := &exec.Cmd{SysProcAttr: attributes}
	HideCommandWindow(cmd)
	if cmd.SysProcAttr != attributes || !attributes.HideWindow || attributes.CreationFlags != windows.CREATE_NEW_PROCESS_GROUP|windows.CREATE_NO_WINDOW {
		t.Fatalf("background command attributes changed: %+v", cmd.SysProcAttr)
	}
}

func TestHiddenCommandHasNoConsoleAndCapturesOutput(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(executable, "-test.run=^TestHiddenCommandChild$")
	cmd.Env = append(os.Environ(), "EDEN_HIDDEN_COMMAND_TEST=1")
	HideCommandWindow(cmd)
	output, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(output), "console=0") || !strings.Contains(string(output), "stderr-captured") {
		t.Fatalf("hidden child: %s, %v", output, err)
	}
}

func TestHiddenCommandChild(t *testing.T) {
	if os.Getenv("EDEN_HIDDEN_COMMAND_TEST") != "1" {
		return
	}
	window, _, _ := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetConsoleWindow").Call()
	fmt.Printf("console=%d\n", window)
	fmt.Fprintln(os.Stderr, "stderr-captured")
}

package desktop

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestProductionUISelfTestRejectsHostBeforeCreatingFiles(t *testing.T) {
	t.Setenv("EDEN_NATIVE_SELF_TEST", "")
	for _, scenario := range []string{"ui-session", "ui-settings", "ui-status", "ui-input", "ui-clipboard", "ui-unknown"} {
		t.Run(scenario, func(t *testing.T) {
			output := filepath.Join(t.TempDir(), "not-created")
			_, err := RunNativeStartupSelfTest(output, scenario, Options{})
			if err == nil || !strings.Contains(err.Error(), "EDEN_NATIVE_SELF_TEST=vm") {
				t.Fatalf("host was not rejected by the VM gate: %v", err)
			}
			if _, err := os.Stat(output); !os.IsNotExist(err) {
				t.Fatalf("VM gate created an output directory: %v", err)
			}
		})
	}
}

func TestOrdinaryShellDoesNotEnableSelfTestBrowserArguments(t *testing.T) {
	t.Setenv("WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS", "")
	shell, err := New(Options{BackendExecutable: filepath.Join(t.TempDir(), "eden.exe"), ConfigPath: filepath.Join(t.TempDir(), "config.json")})
	if err != nil {
		t.Fatal(err)
	}
	defer shell.cancel()
	if len(shell.selfTestBrowserArgs) != 0 {
		t.Fatal("ordinary shell enabled self-test CDP")
	}
}

func TestAdditionalBrowserArgumentsUseTheWebViewEnvironment(t *testing.T) {
	t.Setenv("WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS", "--remote-debugging-port=1234 --remote-debugging-address=127.0.0.1")
	if got, want := additionalBrowserArgsFromEnvironment(), []string{"--remote-debugging-port=1234", "--remote-debugging-address=127.0.0.1"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("browser arguments = %#v, want %#v", got, want)
	}
}

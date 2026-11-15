package agentvm

import (
	"os/exec"
	"runtime"
	"testing"
)

// tests/setup-script.sh: the setup script's blocks, run against stubs.
func TestSetupScript(t *testing.T) {
	runScript(t, "tests/setup-script.sh")
}

func runScript(t *testing.T, script string) {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("runs the Linux guest setup script")
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("no bash")
	}
	out, err := exec.Command(bash, script).CombinedOutput()
	if err != nil {
		t.Errorf("%v\n%s", err, out)
	}
}

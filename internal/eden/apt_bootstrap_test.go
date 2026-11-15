package eden

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestAPTBootstrapUpgradeFailureAndRetry(t *testing.T) {
	requireLinuxGuest(t)
	for _, mode := range []string{"current", "upgrade", "download-failure", "stale-candidate", "unconfigured", "no-utils", "debian-12", "debian-14", "ubuntu", "missing-version"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			version := "3.0.3"
			if mode == "current" {
				version = "3.0.4"
			}
			os.WriteFile(filepath.Join(dir, "version"), []byte(version), 0600)
			release := "ID=debian\nVERSION_ID=13\n"
			rejected := map[string]string{
				"debian-12": "ID=debian\nVERSION_ID=12\n", "debian-14": "ID=debian\nVERSION_ID=14\n",
				"ubuntu": "ID=ubuntu\nVERSION_ID=24.04\n", "missing-version": "ID=debian\n",
			}
			if unsupported, ok := rejected[mode]; ok {
				release = unsupported
			}
			if err := os.WriteFile(filepath.Join(dir, "os-release"), []byte(release), 0600); err != nil {
				t.Fatal(err)
			}
			os.WriteFile(filepath.Join(dir, "sentinel"), []byte("personal data stays"), 0600)
			commands := map[string]string{
				"sudo": "#!/bin/sh\nexec \"$@\"\n",
				"dpkg-query": `#!/bin/sh
case "$*" in
 *apt-utils*)
  case "$MODE" in no-utils) exit 1;; unconfigured) echo 'install ok unpacked';; *) echo 'install ok installed';; esac;;
 *) cat "$FIXTURE/version";;
esac
`,
				"dpkg": `#!/bin/sh
[ "$2" != 3.0.3 ]
`,
				"apt-get": `#!/bin/sh
printf '%s\n' "$*" >> "$FIXTURE/requests"
case "$*" in
 *Acquire::Retries=0*) ;; *) exit 92;;
esac
case "$*" in
 *'install '*)
  [ "$MODE" != download-failure ] || exit 100
  [ "$MODE" = stale-candidate ] || echo 3.0.4 > "$FIXTURE/version";;
esac
`,
			}
			for name, body := range commands {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0755); err != nil {
					t.Fatal(err)
				}
			}
			script := strings.ReplaceAll(APTBootstrapScript, ". /etc/os-release", ". \"$FIXTURE/os-release\"")
			run := func() ([]byte, error) {
				cmd := exec.Command("sh", "-c", script+"\nprintf continued >> \"$FIXTURE/continued\"\n")
				cmd.Env = append(os.Environ(), "PATH="+dir+":"+os.Getenv("PATH"), "FIXTURE="+dir, "MODE="+mode, "TMPDIR="+dir)
				return cmd.CombinedOutput()
			}
			out, err := run()
			requests, _ := os.ReadFile(filepath.Join(dir, "requests"))
			if _, unsupported := rejected[mode]; unsupported {
				if err == nil || !strings.Contains(string(out), "requires Debian 13") || len(requests) != 0 {
					t.Fatalf("unsupported system reached installation: %v %s %s", err, out, requests)
				}
				if _, err := os.Stat(filepath.Join(dir, "continued")); !os.IsNotExist(err) {
					t.Fatal("continued after unsupported system")
				}
				return
			}
			fail := mode == "download-failure" || mode == "stale-candidate"
			if fail {
				if err == nil {
					t.Fatalf("failure continued: %s", out)
				}
				if _, err := os.Stat(filepath.Join(dir, "continued")); !os.IsNotExist(err) {
					t.Fatal("continued after failed bootstrap")
				}
				mode = "upgrade"
				if out, err := run(); err != nil {
					t.Fatalf("retry failed: %v %s", err, out)
				}
			} else if err != nil {
				t.Fatalf("%v %s", err, out)
			}
			if mode == "current" && len(requests) != 0 {
				t.Fatal("current version used network")
			}
			if strings.Contains(string(requests), "apt-utils") && (mode == "unconfigured" || mode == "no-utils") {
				t.Fatal("upgraded uninstalled apt-utils")
			}
			if data, _ := os.ReadFile(filepath.Join(dir, "sentinel")); string(data) != "personal data stays" {
				t.Fatal("personal data changed")
			}
		})
	}
}

func TestGuestInstallEntrypointsCheckAPTFirst(t *testing.T) {
	for name, script := range map[string]string{"lima": DebianPrepareScript, "services": GuestSetup, "start": DefaultConfig().StartScript(), "tools": ToolsUpdateScript()} {
		gate := strings.Index(script, "WORKBENCH_APT_BOOTSTRAP")
		install := strings.Index(script, "apt-get install")
		if gate < 0 || (install >= 0 && gate > install) {
			t.Fatalf("%s installs before APT check", name)
		}
	}
	tools := ToolsUpdateScript()
	configure := strings.Index(tools, "dpkg --configure -a")
	bootstrap := strings.Index(tools, "WORKBENCH_APT_BOOTSTRAP")
	update := strings.Index(tools, "apt-get update")
	if configure < 0 || bootstrap < 0 || configure > bootstrap || update < bootstrap {
		t.Fatal("tool updates must recover dpkg before APT bootstrap and dependency updates")
	}
}

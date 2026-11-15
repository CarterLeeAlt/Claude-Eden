package eden

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDebianMirrorFailureFallbackAndPreference(t *testing.T) {
	requireLinuxGuest(t)
	customPriorities := "https://mirrors.nju.edu.cn/debian\tpriority:1\nhttps://mirrors.ustc.edu.cn/debian\tpriority:2\nhttps://deb.debian.org/debian\tpriority:3\n"
	for _, mode := range []string{"reachable", "tls-failure", "official-failure", "nju-failure", "official", "custom", "custom-priorities", "reboot", "invalid"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			mirrors := filepath.Join(dir, "mirrors")
			if err := os.MkdirAll(mirrors, 0755); err != nil {
				t.Fatal(err)
			}
			list := filepath.Join(mirrors, "debian.list")
			content := "https://deb.debian.org/debian\n"
			if mode == "custom" {
				content = "https://custom.invalid/debian\n"
			}
			if mode == "custom-priorities" {
				content = customPriorities
			}
			if mode == "reboot" {
				content = "# Workbench managed Debian mirrors\nhttps://deb.debian.org/debian\tpriority:1\n"
			}
			if err := os.WriteFile(list, []byte(content), 0644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(mirrors, "debian-security.list"), []byte("https://deb.debian.org/debian-security\n"), 0644); err != nil {
				t.Fatal(err)
			}
			preference := filepath.Join(dir, "preference")
			if mode == "official" || mode == "invalid" {
				if err := os.WriteFile(preference, []byte(mode+"\n"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			for name, body := range map[string]string{
				"sudo": "#!/bin/sh\nexec \"$@\"\n",
				"apt-helper": `#!/bin/sh
printf '%s\n' "$*" >> "$FIXTURE/probes"
[ "$MODE" != tls-failure ] || exit 1
case "$*" in
 *deb.debian.org*) [ "$MODE" != nju-failure ] && [ "$MODE" != official-failure ] || exit 1;;
 *mirrors.nju*) [ "$MODE" != nju-failure ] || exit 1;;
esac
exit 0
`,
				"timeout": "#!/bin/sh\nshift\nexec \"$@\"\n",
			} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0755); err != nil {
					t.Fatal(err)
				}
			}
			script := strings.ReplaceAll(DebianMirrorsScript, "/etc/apt/mirrors", mirrors)
			script = strings.ReplaceAll(script, "/etc/workbench/debian-mirror", preference)
			script = strings.ReplaceAll(script, "/usr/lib/apt/apt-helper", filepath.Join(dir, "apt-helper"))
			run := func() ([]byte, error) {
				cmd := exec.Command("sh", "-c", script)
				cmd.Env = append(os.Environ(), "PATH="+dir+":"+os.Getenv("PATH"), "FIXTURE="+dir, "MODE="+mode, "TMPDIR="+dir)
				return cmd.CombinedOutput()
			}
			out, err := run()
			if mode == "invalid" {
				if err == nil {
					t.Fatal("invalid preference accepted")
				}
				return
			}
			if err != nil {
				t.Fatalf("%v %s", err, out)
			}
			data, err := os.ReadFile(list)
			if err != nil {
				t.Fatal(err)
			}
			got := string(data)
			switch mode {
			case "custom", "custom-priorities":
				if got != content {
					t.Fatal("custom source overwritten")
				}
				probes, err := os.ReadFile(filepath.Join(dir, "probes"))
				if err != nil || strings.Contains(string(probes), "/debian/dists/") {
					t.Fatalf("custom repository was probed: %s, %v", probes, err)
				}
			case "official", "tls-failure", "reachable", "reboot":
				if strings.Contains(got, "mirrors.") || !strings.Contains(got, "deb.debian.org") {
					t.Fatal(got)
				}
			case "nju-failure":
				if strings.Contains(got, "mirrors.nju") || !strings.Contains(got, "mirrors.ustc") {
					t.Fatal(got)
				}
			default:
				if !strings.Contains(got, "mirrors.nju") {
					t.Fatal(got)
				}
			}
			if mode == "official" {
				probes, err := os.ReadFile(filepath.Join(dir, "probes"))
				if err != nil && !os.IsNotExist(err) {
					t.Fatal(err)
				}
				if len(probes) > 0 {
					t.Fatal("official preference contacted mirrors")
				}
			}
			if mode == "reachable" {
				probes, err := os.ReadFile(filepath.Join(dir, "probes"))
				if err != nil || !strings.Contains(string(probes), "/dists/trixie/InRelease") || !strings.Contains(string(probes), "/dists/trixie-security/InRelease") {
					t.Fatalf("wrong distribution probes: %s, %v", probes, err)
				}
				if strings.Contains(string(probes), "mirrors.") {
					t.Fatal("reachable official source still contacted domestic mirrors")
				}
			}
			if out, err := run(); err != nil {
				t.Fatalf("repeat: %v %s", err, out)
			}
			after, err := os.ReadFile(list)
			if err != nil {
				t.Fatal(err)
			}
			if string(after) != got {
				t.Fatal("preference changed on repeated setup")
			}
		})
	}
}

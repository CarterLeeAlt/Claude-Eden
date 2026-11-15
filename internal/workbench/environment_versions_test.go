package workbench

import (
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/sylvinus/agent-vm/internal/eden"
)

func TestGuestConnectionRequiresSuccessfulMatchingManifest(t *testing.T) {
	required := eden.RequiredGuestManifest()
	for _, name := range []string{"current", "missing", "old-revision", "old-protocol", "broken"} {
		t.Run(name, func(t *testing.T) {
			manifest := required
			if name == "old-revision" {
				manifest.Revision = "previous"
			}
			if name == "old-protocol" {
				manifest.Protocol++
			}
			encoded, _ := json.Marshal(manifest)
			if name == "missing" {
				encoded = []byte("null")
			}
			if name == "broken" {
				encoded = []byte("{")
			}
			credentials, current, err := parseGuestConnection(append([]byte("{\"token\":\"guest-test\"}\npassword\n"), encoded...))
			if name == "current" {
				if err != nil || current == nil || credentials.Token != "guest-test" || credentials.VNCPassword != "password" {
					t.Fatal(current, err)
				}
			} else if !errors.Is(err, errGuestUpdateRequired) {
				t.Fatalf("stale guest accepted: %v", err)
			}
		})
	}
}

func grantedToolUpdateServer(t *testing.T) (*Server, *eden.EgressGate) {
	t.Helper()
	s, gate := interfaceMonitorServer(t)
	grantTestGate(t, gate)
	s.ready = true
	required := eden.RequiredGuestManifest()
	s.guestManifest = &required
	s.setJobForTest(Job{ID: "tools", Kind: "environment-tools", Running: true})
	s.publishGate(gate.Status())
	return s, gate
}

func grantTestGate(t *testing.T, gate *eden.EgressGate) {
	t.Helper()
	state := gate.Status()
	report := eden.SafetyReport{Policy: eden.SafetyPolicyVersion, ID: "tool-update-test", RunID: state.RunID, CheckID: state.CheckID, Configuration: state.Configuration, CreatedAt: time.Now()}
	for _, id := range []string{"egress_policy", "proxy_transport", "dns_transport", "guest_settings", "browser_settings", "filesystem_boundary", "management_boundary", "fail_closed", "udp_policy", "ipv6_policy", "exit_consistency"} {
		report.Checks = append(report.Checks, eden.SafetyCheck{ID: id, Title: id, Critical: true, State: eden.CheckPassed, Evidence: "synthetic isolated fixture", CheckedAt: report.CreatedAt})
	}
	if err := gate.Grant(report); err != nil {
		t.Fatal(err)
	}
}

func TestToolUpdateLeaseHonorsPauseStopAndConfigurationAtRenewal(t *testing.T) {
	for _, jobKind := range []string{"environment-tools", "claude-desktop-install"} {
		t.Run(jobKind, func(t *testing.T) {
			for _, condition := range []string{"allowed", "paused", "stop", "configuration-changed"} {
				t.Run(condition, func(t *testing.T) {
					s, gate := grantedToolUpdateServer(t)
					s.setJobForTest(Job{ID: "tools", Kind: jobKind, Running: true})
					client, err := s.gateClient()
					if err != nil {
						t.Fatal(err)
					}
					before := gate.Status()
					// Windows can grant and renew within the same wall-clock tick.
					// Advance past the grant tick before requiring a later deadline.
					for !time.Now().Add(eden.GateLease).After(before.Until) {
						time.Sleep(time.Millisecond)
					}
					switch condition {
					case "paused":
						if err := s.pauseNetworkNow("test user pause"); err != nil {
							t.Fatal(err)
						}
					case "stop":
						s.setJobForTest(Job{ID: "stop", Kind: "stop", Running: true})
					case "configuration-changed":
						cfg, err := loadConfig(s.configPath)
						if err != nil {
							t.Fatal(err)
						}
						cfg.Timezone = "Etc/UTC"
						if cfg.Fingerprint() == before.Configuration {
							cfg.Timezone = "Asia/Tokyo"
						}
						if err := writePrivate(s.configPath, cfg); err != nil {
							t.Fatal(err)
						}
					}
					s.finishSafetyMonitor(context.Background(), client, before, "192.0.2.1", "")
					after := gate.Status()
					switch condition {
					case "allowed":
						if after.Mode != eden.GateOpen || !after.Until.After(before.Until) {
							t.Fatal("tool update lease was not renewed")
						}
					case "paused", "configuration-changed":
						if after.Mode != eden.GateClosed {
							t.Fatal("denied network was renewed")
						}
					case "stop":
						if after.Until != before.Until {
							t.Fatal("stopping job allowed a lease renewal")
						}
					}
				})
			}
		})
	}
}

func TestToolUpdateDoesNotRestorePausedOrExpiredNetworking(t *testing.T) {
	for _, paused := range []bool{true, false} {
		s, gate := grantedToolUpdateServer(t)
		if paused {
			if err := s.pauseNetworkNow("manual"); err != nil {
				t.Fatal(err)
			}
		} else {
			gate.Close("expired")
		}
		s.mu.Lock()
		allowed := s.toolsNetworkAllowedLocked()
		s.mu.Unlock()
		if !allowed {
			t.Fatal("business network pause blocked maintenance")
		}
		if gate.Status().Mode != eden.GateClosed {
			t.Fatal("tool update restored closed network")
		}
	}
}

func TestEnvironmentVersionRoutesRequireHostSession(t *testing.T) {
	s := testServer(t)
	for _, path := range []string{"/api/environment/versions", "/api/environment/update", "/api/environment/tools"} {
		method := "POST"
		if strings.HasSuffix(path, "versions") {
			method = "GET"
		}
		if w := request(s, method, path, s.origin, "{}", false); w.Code != 401 {
			t.Fatalf("untrusted version action accepted: %d", w.Code)
		}
	}
	w := request(s, "GET", "/api/environment/versions", s.origin, "", true)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"status":"stopped"`) {
		t.Fatal(w.Code, w.Body.String())
	}
}

func TestToolVersionOutputSeparatesCodeServerInitializationLogs(t *testing.T) {
	const version = "4.145.0 2cb3ad3e258891fbb51001ba465c2348d5021644 with Code 1.145.0"
	const notice = "[2026-11-15T06:14:29.107Z] info  Wrote default config file to /home/developer/.config/code-server/config.yaml"
	for _, tc := range []struct{ name, output, want string }{
		{"code_server", version + "\n", version},
		{"code_server", notice + "\n" + version + "\n", version},
		{"code_server", notice + "\r\n" + version + "\r\n", version},
		{"code_server", notice + "\n", ""},
		{"code_server", "[info] " + version + "\n", ""},
		{"code_server", "", ""},
		{"gh", "gh version 2.103.3 (2026-10-08)\nhttps://github.com/cli/cli/releases\n", "gh version 2.103.3 (2026-10-08)"},
	} {
		if got := toolVersionOutput(tc.name, tc.output); got != tc.want {
			t.Errorf("%s output %q: got %q, want %q", tc.name, tc.output, got, tc.want)
		}
	}
}

func TestGuestToolVersionsExecutesCollectorAndPreservesVersionAfterNotices(t *testing.T) {
	name := "python3"
	if runtime.GOOS == "windows" {
		name = "python"
	}
	python, err := exec.LookPath(name)
	if err != nil {
		t.Skip("Python interpreter unavailable")
	}
	const version = "4.145.0 2cb3ad3e258891fbb51001ba465c2348d5021644 with Code 1.145.0"
	const notice = "[info] Wrote default config file"
	// Run the actual embedded program; replace only guest filesystem reads and
	// external commands so this test cannot invoke tools or touch host accounts.
	prelude := `import os,subprocess
from pathlib import Path
Path.read_text=lambda self: '{"revision":"fixture"}'
Path.home=classmethod(lambda cls: Path('/fixture-home'))
os.path.isfile=lambda path: False
def command(args, **kwargs):
 assert args[-1]=='--version' and kwargs['capture_output'] and kwargs['text'] and kwargs['timeout']==4
 assert kwargs['env']['DISABLE_AUTOUPDATER']=='1'
 name=str(args[0]).replace('\\','/').rsplit('/',1)[-1]
 if name=='code-server': return subprocess.CompletedProcess(args,0,'[info] Wrote default config file\n4.145.0 2cb3ad3e258891fbb51001ba465c2348d5021644 with Code 1.145.0\n','')
 if name=='gh': return subprocess.CompletedProcess(args,0,'gh version 2.103.3\nrelease link\n','')
 if name=='git': return subprocess.CompletedProcess(args,1,'must not become a version','')
 if name=='claude': raise subprocess.TimeoutExpired(args,4)
 if name=='mise': raise FileNotFoundError('missing fixture tool')
 assert str(args[0])=='/usr/bin/chromium'
 return subprocess.CompletedProcess(args,0,'x'*20000,'')
subprocess.run=command
`
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, python, "-I", "-c", prelude+guestToolVersionsScript)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("guest version collector failed: %v\n%s", err, output)
	}
	var snapshot struct {
		Manifest map[string]string `json:"manifest"`
		Tools    map[string]string `json:"tools"`
	}
	if err := json.Unmarshal(output, &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.Manifest["revision"] != "fixture" || len(snapshot.Tools) != 6 {
		t.Fatalf("incomplete guest snapshot: %s", output)
	}
	if snapshot.Tools["code_server"] != notice+"\n"+version+"\n" || toolVersionOutput("code_server", snapshot.Tools["code_server"]) != version {
		t.Fatal("first-run notice hid the actual code-server version")
	}
	if toolVersionOutput("gh", snapshot.Tools["gh"]) != "gh version 2.103.3" || len(snapshot.Tools["chromium"]) != 16384 {
		t.Fatal("tool output parsing or collection limit changed")
	}
	for _, name := range []string{"git", "claude", "mise"} {
		if snapshot.Tools[name] != "" {
			t.Fatalf("failed tool %s was reported as installed", name)
		}
	}
}

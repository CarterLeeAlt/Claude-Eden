package eden

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestGuestDisplayReadiness(t *testing.T) {
	if !strings.Contains(guestServices, "ExecStartPost=/usr/bin/python3 /usr/local/lib/workbench-wait-services.py --display :77") {
		t.Fatal("dependent services must wait for the X server to accept connections")
	}
	interpreter := "python3"
	if runtime.GOOS == "windows" {
		interpreter = "python"
	}
	python, err := exec.LookPath(interpreter)
	if err != nil {
		t.Skip("Python unavailable")
	}
	dir := t.TempDir()
	script, check := filepath.Join(dir, "wait.py"), filepath.Join(dir, "check.py")
	if err := os.WriteFile(script, []byte(guestWaitServices), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(check, []byte(`import runpy, subprocess, sys
from unittest.mock import patch
module = runpy.run_path(sys.argv[1])
wait = module['wait_display']
with patch('time.sleep'):
    with patch('subprocess.run', side_effect=[subprocess.CompletedProcess([], 1, stderr='not ready'), subprocess.TimeoutExpired('xrandr', 2), subprocess.CompletedProcess([], 0)]) as query:
        wait(':177')
        assert query.call_count == 3
        assert query.call_args.args[0] == ['/usr/bin/xrandr', '--display', ':177', '--query']
    with patch('subprocess.run', side_effect=FileNotFoundError('xrandr missing')):
        try:
            wait(':177')
        except FileNotFoundError:
            pass
        else:
            raise AssertionError('Missing dependency must propagate')
    with patch('subprocess.run', side_effect=KeyboardInterrupt):
        try:
            wait(':177')
        except KeyboardInterrupt:
            pass
        else:
            raise AssertionError('Cancellation must propagate')
check = module['check_services']
audio = module['wait_audio']
attempts = 0
def audio_probe(command, **kwargs):
    global attempts
    if command[0] == 'pactl':
        attempts += 1
        return subprocess.CompletedProcess(command, 0 if attempts > 60 else 1)
    return subprocess.CompletedProcess(command, 0, stdout='Id=workbench-audio.service\nLoadState=loaded\nActiveState=activating\nSubState=start-post\nResult=success\n')
with patch('time.sleep'), patch('subprocess.run', side_effect=audio_probe):
    audio()
    assert attempts == 61
with patch('subprocess.run', side_effect=[subprocess.CompletedProcess([], 1), subprocess.CompletedProcess([], 0, stdout='Id=workbench-audio.service\nLoadState=loaded\nActiveState=failed\nResult=exit-code\n')]):
    try:
        audio()
    except SystemExit as error:
        assert 'workbench-audio.service' in str(error)
    else:
        raise AssertionError('Audio service failure must propagate')
with patch('subprocess.run', side_effect=KeyboardInterrupt):
    try:
        audio()
    except KeyboardInterrupt:
        pass
    else:
        raise AssertionError('Audio readiness must remain cancellable')
for state in ['active', 'activating', 'failed', 'inactive']:
    output = 'Id=workbench-browser.service\nLoadState=loaded\nActiveState=' + state + '\nResult=exit-code\n'
    with patch('subprocess.run', return_value=subprocess.CompletedProcess([], 0, stdout=output)):
        try:
            check(['workbench-browser.service'])
        except SystemExit as error:
            assert state in ('failed', 'inactive') and 'workbench-browser.service' in str(error)
        else:
            assert state in ('active', 'activating')
for extra in ['LoadState=not-found\nActiveState=inactive', 'LoadState=loaded\nActiveState=activating\nSubState=auto-restart']:
    with patch('subprocess.run', return_value=subprocess.CompletedProcess([], 0, stdout='Id=workbench-browser.service\n' + extra)):
        try:
            check(['workbench-browser.service'])
        except SystemExit:
            pass
        else:
            raise AssertionError('Missing or failed service must fail startup')
`), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	if output, err := exec.CommandContext(ctx, python, check, script).CombinedOutput(); err != nil {
		t.Fatalf("display readiness: %v %s", err, output)
	}
}

func TestGuestReadinessWaitsForAuthenticatedBrowser(t *testing.T) {
	interpreter := "python3"
	if runtime.GOOS == "windows" {
		interpreter = "python"
	}
	python, err := exec.LookPath(interpreter)
	if err != nil {
		t.Skip("Python unavailable")
	}
	var browserRequests, proxyRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/browser" {
			_, _ = fmt.Fprint(w, "ready")
			return
		}
		if r.Header.Get("Authorization") != "Bearer fixture-token" {
			w.WriteHeader(401)
			return
		}
		if browserRequests.Add(1) < 3 {
			w.WriteHeader(503)
			return
		}
		_, _ = fmt.Fprint(w, `{"ready":true}`)
	}))
	defer server.Close()
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { proxyRequests.Add(1); w.WriteHeader(502) }))
	defer proxy.Close()
	dir := t.TempDir()
	script, config := filepath.Join(dir, "wait.py"), filepath.Join(dir, "browser.json")
	if err := os.WriteFile(script, []byte(guestWaitServices), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config, []byte(`{"token":"fixture-token"}`), 0600); err != nil {
		t.Fatal(err)
	}
	endpoints, _ := json.Marshal([][]any{{server.URL + "/editor", false}, {server.URL + "/vnc", false}, {server.URL + "/browser", true}})
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, python, "-c", `import runpy,sys,json;runpy.run_path(sys.argv[1])['wait'](sys.argv[2],json.loads(sys.argv[3]))`, script, config, string(endpoints))
	cmd.Env = append(os.Environ(), "http_proxy="+proxy.URL, "HTTP_PROXY="+proxy.URL, "no_proxy=", "NO_PROXY=")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("readiness: %v %s", err, output)
	}
	if browserRequests.Load() < 3 || proxyRequests.Load() != 0 {
		t.Fatalf("premature or proxied readiness: browser=%d proxy=%d", browserRequests.Load(), proxyRequests.Load())
	}
}

func TestGuestReadinessReportsProtocolFailure(t *testing.T) {
	interpreter := "python3"
	if runtime.GOOS == "windows" {
		interpreter = "python"
	}
	python, err := exec.LookPath(interpreter)
	if err != nil {
		t.Skip("Python unavailable")
	}
	dir := t.TempDir()
	script, config := filepath.Join(dir, "wait.py"), filepath.Join(dir, "browser.json")
	if err := os.WriteFile(script, []byte(guestWaitServices), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config, []byte(`{"token":"fixture-token"}`), 0600); err != nil {
		t.Fatal(err)
	}
	for _, status := range []int{http.StatusUnauthorized, http.StatusNotFound, http.StatusInternalServerError, http.StatusOK} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
				fmt.Fprint(w, "invalid protocol response")
			}))
			defer server.Close()
			endpoints, _ := json.Marshal([][]any{{server.URL, true}})
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, python, "-c", `import runpy,sys,json;runpy.run_path(sys.argv[1])['wait'](sys.argv[2],json.loads(sys.argv[3]))`, script, config, string(endpoints))
			output, err := cmd.CombinedOutput()
			if err == nil || ctx.Err() != nil {
				t.Fatalf("explicit HTTP/protocol failure kept waiting: %v %s", err, output)
			}
			if status != http.StatusOK && !strings.Contains(string(output), fmt.Sprintf("HTTP %d", status)) {
				t.Fatalf("missing HTTP failure detail: %s", output)
			}
		})
	}
}

func TestGuestEditorReadinessDoesNotReadBrowserCredentials(t *testing.T) {
	interpreter := "python3"
	if runtime.GOOS == "windows" {
		interpreter = "python"
	}
	python, err := exec.LookPath(interpreter)
	if err != nil {
		t.Fatal(err)
	}
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.Header.Get("Authorization") != "" {
			t.Error("editor health probe received browser credentials")
		}
		_, _ = fmt.Fprint(w, `{"status":"alive"}`)
	}))
	defer server.Close()
	dir := t.TempDir()
	script, check := filepath.Join(dir, "wait.py"), filepath.Join(dir, "check.py")
	if err := os.WriteFile(script, []byte(guestWaitServices), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(check, []byte(`import runpy, sys
wait = runpy.run_path(sys.argv[1])['wait']
wait(sys.argv[2], ((sys.argv[3], False),))
`), 0600); err != nil {
		t.Fatal(err)
	}
	// The editor runs as its own user and cannot read browser-control.json.
	// A missing file and a directory both reject reads on all test platforms.
	for _, config := range []string{filepath.Join(dir, "not-created.json"), dir} {
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		output, err := exec.CommandContext(ctx, python, check, script, config, server.URL).CombinedOutput()
		cancel()
		if err != nil {
			t.Fatalf("editor readiness read unrelated credentials: %v %s", err, output)
		}
	}
	if requests.Load() != 2 {
		t.Fatalf("editor health endpoint received %d probes", requests.Load())
	}
}

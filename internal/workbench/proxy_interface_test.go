package workbench

import (
	"context"
	"errors"
	"net"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sylvinus/agent-vm/internal/eden"
	"github.com/sylvinus/agent-vm/internal/managedproxy"
)

func interfaceMonitorServer(t *testing.T) (*Server, *eden.EgressGate) {
	t.Helper()
	s := testServer(t)
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	cfg, _ := loadConfig(s.configPath)
	p, _ := managedproxy.Load(s.profile)
	p.Port = listener.Addr().(*net.TCPAddr).Port
	if err = p.Save(s.profile); err != nil {
		t.Fatal(err)
	}
	cfg.ProxyURL = p.Endpoint()
	if err := writePrivate(s.configPath, cfg); err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := os.MkdirAll(filepath.Join(s.profile, "proxy"), 0700); err != nil {
		t.Fatal(err)
	}
	data := []byte("interface-name: en0\nmixed-port: " + strconv.Itoa(port) + "\n")
	if err := os.WriteFile(filepath.Join(s.profile, "proxy", "config.yaml"), data, 0600); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(s.profile, "state", "lima", eden.InstanceName(s.profile, s.environmentDir()))
	if err := writePrivate(filepath.Join(dir, "eden-running.json"), map[string]int{"pid": os.Getpid()}); err != nil {
		t.Fatal(err)
	}
	gate, err := eden.NewEgressGate(cfg)
	if err != nil {
		t.Fatal(err)
	}
	stop, err := eden.StartGateControl(s.configPath, dir, gate)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(stop)
	if err := gate.BeginCheck(); err != nil {
		t.Fatal(err)
	}
	return s, gate
}

func TestInterfaceRecoveryRevokesGateAndPreservesManualPause(t *testing.T) {
	previous := physicalProxyInterface
	previousValidate := validateProxyCore
	t.Cleanup(func() { physicalProxyInterface, validateProxyCore = previous, previousValidate })
	validateProxyCore = func(ctx context.Context, _ string, _ managedproxy.Store) error {
		<-ctx.Done()
		return ctx.Err()
	}
	for _, tc := range []struct {
		name, next   string
		paused, lost bool
	}{
		{"Wi-Fi to Ethernet", "en7", false, false},
		{"Ethernet while paused", "en7", true, false},
		{"same interface after outage", "en0", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, gate := interfaceMonitorServer(t)
			s.networkPaused, s.proxyNetworkLost = tc.paused, tc.lost
			physicalProxyInterface = func(context.Context) (string, error) { return tc.next, nil }
			// Hold recovery in cancellable validation, before a core can restart.
			// Revocation is synchronous; stop cancels the admitted recovery job.
			s.monitorProxyInterface()
			s.mu.Lock()
			queued, paused := s.jobs.snapshot().Kind == "proxy-network" && s.jobs.snapshot().Running, s.networkPaused
			s.mu.Unlock()
			closed := gate.Status().Mode == eden.GateClosed
			finished := make(chan struct{})
			s.launch(httptest.NewRecorder(), "stop", func() error { close(finished); return nil })
			select {
			case <-finished:
			case <-time.After(time.Second):
				t.Fatal("stop could not cancel network recovery")
			}
			s.op.Lock()
			s.op.Unlock()
			if !queued || !closed || paused != tc.paused {
				t.Fatalf("queued=%v closed=%v paused=%v; wanted pause=%v", queued, closed, paused, tc.paused)
			}
		})
	}
}

func TestInterfaceObservationDoesNotApplyToChangedConfigurationOrJob(t *testing.T) {
	previous := physicalProxyInterface
	t.Cleanup(func() { physicalProxyInterface = previous })
	for _, change := range []string{"configuration", "job", "shutdown"} {
		t.Run(change, func(t *testing.T) {
			s := testServer(t)
			started, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
			physicalProxyInterface = func(context.Context) (string, error) {
				close(started)
				<-release
				return "", errors.New("old interface observation failed")
			}
			go func() { defer close(done); s.monitorProxyInterface() }()
			<-started
			s.op.Lock()
			switch change {
			case "configuration":
				cfg, err := loadConfig(s.configPath)
				if err != nil {
					t.Error(err)
				} else {
					cfg.ProxyRevision = "new-selection"
					if err := writePrivate(s.configPath, cfg); err != nil {
						t.Error(err)
					}
				}
			case "job":
				s.setJobForTest(Job{ID: "completed-stop", Kind: "stop"})
			case "shutdown":
				s.beginClose()
			}
			s.op.Unlock()
			close(release)
			<-done
			if s.proxyNetworkLost || s.safety.monitorError != "" || s.busy() {
				t.Fatal("stale observation changed the current network state")
			}
		})
	}
}

func TestMissingPhysicalInterfaceClosesGateWithoutUnboundFallback(t *testing.T) {
	s, gate := interfaceMonitorServer(t)
	previous := physicalProxyInterface
	t.Cleanup(func() { physicalProxyInterface = previous })
	physicalProxyInterface = func(context.Context) (string, error) { return "", errors.New("physical network unavailable") }
	s.monitorProxyInterface()
	if gate.Status().Mode != eden.GateClosed || s.networkPaused || !s.proxyNetworkLost || s.jobs.snapshot().Running || s.currentNetworkErrorLocked() == "" {
		t.Fatal("missing uplink did not preserve offline recovery state")
	}
	data, _ := os.ReadFile(filepath.Join(s.profile, "proxy", "config.yaml"))
	if !strings.Contains(string(data), "interface-name: en0") {
		t.Fatal("missing uplink removed the existing binding")
	}
}

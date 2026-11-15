package workbench

import (
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSetupProgressParsesComponentNamesBeforeLogRedaction(t *testing.T) {
	var m jobManager
	var op sync.Mutex
	_, err := m.submit(&op, "setup", 0, nil, nil, func() error {
		e := m.execution()
		e.setSetupProgress(&SetupProgress{Phase: "image", System: "debian-13"})
		for _, part := range []string{"EDEN_GUEST_BYTES debian-13 ", "download 25 100\n"} {
			_, _ = e.Write([]byte(part))
		}
		p := m.snapshot().SetupProgress
		if p.Operation != "download" || p.Received == nil || *p.Received != 25 || p.Total != 100 {
			t.Errorf("host component progress lost: %+v", p)
		}
		_, _ = e.Write([]byte("WORKBENCH_TOOL_BYTES chromium-common.deb 5 10\n"))
		p = m.snapshot().SetupProgress
		if p.Phase != "packages" || p.Received == nil || *p.Received != 5 || p.Total != 10 {
			t.Errorf("tool component progress lost: %+v", p)
		}
		_, _ = e.Write([]byte("connect error: static.example-provider.io:1337\n"))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	awaitJobIdle(t, &m)
	log := m.snapshot().Log
	if strings.Contains(log, "static.example-provider.io") || strings.Contains(log, "1337") || strings.Contains(log, "EDEN_") || strings.Contains(log, "WORKBENCH_") || !strings.Contains(log, "connect error:") {
		t.Fatalf("job log leaked raw output or lost error evidence: %q", log)
	}
}

// Child pipes may split a progress line across writes. A fallback mirror may
// restart from zero, and an absent Content-Length must stay indeterminate.
func TestSetupProgressFromChildOutput(t *testing.T) {
	var m jobManager
	var op sync.Mutex
	var points []*SetupProgress
	_, err := m.submit(&op, "start", 0, nil, nil, func() error {
		e := m.execution()
		e.setSetupProgress(&SetupProgress{Phase: "image"})
		for _, part := range []string{"EDEN_IMAGE_BYTES 25", " 100\n", "EDEN_IMAGE_BYTES 0 -1\n", "EDEN_IMAGE_BYTES 123 5\n", "EDEN_IMAGE_READY\n", "Installing Claude Code...\n"} {
			_, _ = e.Write([]byte(part))
			points = append(points, m.snapshot().SetupProgress)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	awaitJobIdle(t, &m)
	if len(points) != 6 || points[0].Received != nil || points[1].Received == nil || *points[1].Received != 25 || points[1].Total != 100 || points[2].Received == nil || *points[2].Received != 0 || points[2].Total != 0 || points[3] != points[2] || points[4].Received != nil || points[5].Phase != "tools" {
		t.Fatalf("bad setup progress sequence: %+v", points)
	}
}

func TestLinuxInstallProgressKeepsSystemAndSeparatesVerification(t *testing.T) {
	var m jobManager
	var op sync.Mutex
	_, err := m.submit(&op, "guest-install", 0, nil, nil, func() error {
		e := m.execution()
		e.setSetupProgress(&SetupProgress{Phase: "image", System: "debian-13"})
		_, _ = e.Write([]byte("EDEN_GUEST_BYTES debian-13 download 25"))
		if m.snapshot().SetupProgress.Received != nil {
			t.Error("partial line became progress")
		}
		_, _ = e.Write([]byte(" 100\n"))
		m.mu.Lock()
		e.progressUpdated = time.Now().Add(-2 * time.Second)
		m.mu.Unlock()
		_, _ = e.Write([]byte("EDEN_GUEST_BYTES debian-13 download 75 100\n"))
		p := m.snapshot().SetupProgress
		if p.BytesPerSecond < 24 || p.BytesPerSecond > 26 || *p.Received != 75 {
			t.Errorf("wrong measured rate: %+v", p)
		}
		_, _ = e.Write([]byte("EDEN_GUEST_BYTES debian-13 verify 0 100\n"))
		p = m.snapshot().SetupProgress
		if p.Operation != "verify" || p.BytesPerSecond != 0 || *p.Received != 0 {
			t.Errorf("download leaked into verification: %+v", p)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	awaitJobIdle(t, &m)
	for _, line := range []string{"EDEN_GUEST_BYTES unknown download 2 10", "EDEN_GUEST_BYTES debian-13 download 11 10", "EDEN_GUEST_BYTES debian-13 verify -1 10", "EDEN_GUEST_BYTES debian-13 anything 1 10", "EDEN_GUEST_BYTES debian-12 download 1 10", "EDEN_GUEST_BYTES ubuntu-22.04 download 1 10"} {
		if p := parseSetupProgress(line); p != nil {
			t.Errorf("invalid progress accepted: %s", line)
		}
	}
	for _, system := range []string{"debian-13"} {
		p := parseSetupProgress("EDEN_GUEST_BYTES " + system + " download 1 10")
		if p == nil || p.Total != 10 || p.System != system {
			t.Errorf("missing shared progress for %s", system)
		}
	}
}

func TestToolProgressUsesOnlyKnownPackageNames(t *testing.T) {
	p := parseSetupProgress("WORKBENCH_TOOL_BYTES claude-code-linux-x64.tgz 4980736")
	if p == nil || p.Phase != "tools" || p.Received == nil || *p.Received != 4980736 || p.Total != 0 {
		t.Fatalf("expected current bytes without a guessed total, got %+v", p)
	}
	if p := parseSetupProgress("WORKBENCH_TOOL_BYTES untrusted 12"); p != nil {
		t.Fatalf("unexpected package name entered UI: %+v", p)
	}
	if p := parseSetupProgress("WORKBENCH_TOOL_BYTES chromium-common.deb 5 10"); p == nil || p.Total != 10 || *p.Received != 5 {
		t.Fatalf("missing Chromium progress from signed index: %+v", p)
	}
	if p := parseSetupProgress("WORKBENCH_TOOL_BYTES chromium.deb 12 10"); p != nil {
		t.Fatalf("excess bytes entered UI: %+v", p)
	}
}

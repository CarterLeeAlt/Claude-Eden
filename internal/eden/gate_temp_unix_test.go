//go:build !windows

package eden

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGateControlUsesTMPDIR(t *testing.T) {
	// Keep the actual socket short while retaining the caller's isolated root.
	root, err := os.MkdirTemp(os.TempDir(), "gate-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(root); err != nil {
			t.Error(err)
		}
	})
	instance := t.TempDir()
	t.Setenv("TMPDIR", root)
	gate, err := NewEgressGate(DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	stop, err := StartGateControl(filepath.Join(instance, "config.json"), instance, gate)
	if err != nil {
		t.Fatalf("use a shorter workspace TMPDIR for Unix sockets: %v", err)
	}
	t.Cleanup(stop)
	client, err := OpenGateClient(instance)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(filepath.Dir(client.record.Socket)) != root {
		t.Fatalf("socket escaped TMPDIR: %s", client.record.Socket)
	}
	info, err := os.Stat(client.record.Socket)
	if err != nil || info.Mode()&os.ModeSocket == 0 || info.Mode().Perm() != 0600 {
		t.Fatalf("expected a private Unix socket: %v %v", info, err)
	}
	state, err := client.Status(context.Background())
	if err != nil || state.RunID != gate.Status().RunID {
		t.Fatalf("real gate control request failed: %+v %v", state, err)
	}
	stop()
	if _, err := os.Stat(filepath.Dir(client.record.Socket)); !os.IsNotExist(err) {
		t.Fatalf("gate runtime survived stop: %v", err)
	}
}

func TestGateControlLongTMPDIRReturnsSocketError(t *testing.T) {
	instance := t.TempDir()
	root := filepath.Join(instance, strings.Repeat("x", 120))
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", root)
	gate, err := NewEgressGate(DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	stop, err := StartGateControl(filepath.Join(instance, "config.json"), instance, gate)
	if stop != nil {
		stop()
	}
	var socketError *net.OpError
	if !errors.As(err, &socketError) || socketError.Net != "unix" || socketError.Addr == nil || !strings.HasPrefix(socketError.Addr.String(), root+string(os.PathSeparator)) {
		t.Fatalf("long TMPDIR must return its original socket error without fallback: %v", err)
	}
	if !strings.Contains(err.Error(), "TMPDIR") || !strings.Contains(err.Error(), socketError.Error()) {
		t.Fatalf("socket error lost its cause or TMPDIR guidance: %v", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatalf("failed gate left runtime files: %v %v", entries, err)
	}
	if _, err := os.Stat(filepath.Join(instance, gateRecordName)); !os.IsNotExist(err) {
		t.Fatalf("failed gate published a control record: %v", err)
	}
}

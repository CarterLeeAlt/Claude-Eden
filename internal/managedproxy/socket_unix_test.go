//go:build !windows

package managedproxy

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestProxyControlSocketUsesTMPDIR(t *testing.T) {
	root, err := os.MkdirTemp(os.TempDir(), "proxy-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(root); err != nil {
			t.Error(err)
		}
	})
	profile := t.TempDir()
	t.Setenv("TMPDIR", root)
	socket := Socket(profile)
	if filepath.Dir(filepath.Dir(socket)) != root || Socket(filepath.Join(profile, "other")) == socket {
		t.Fatalf("socket root or profile isolation is wrong: %s", socket)
	}
	if err := os.Mkdir(filepath.Dir(socket), 0700); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatalf("use a shorter workspace TMPDIR for Unix sockets: %v", err)
	}
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/version" {
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"version":"workspace-socket"}`)
	})}
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	t.Cleanup(func() {
		if err := server.Close(); err != nil {
			t.Error(err)
		}
		if err := <-done; !errors.Is(err, http.ErrServerClosed) {
			t.Error(err)
		}
	})
	var reply struct{ Version string }
	if err := API(context.Background(), profile, "GET", "/version", nil, &reply); err != nil || reply.Version != "workspace-socket" {
		t.Fatalf("proxy API did not use the actual TMPDIR socket: %+v %v", reply, err)
	}
}

func TestProxyControlLongTMPDIRReturnsSocketError(t *testing.T) {
	profile := t.TempDir()
	root := filepath.Join(profile, strings.Repeat("x", 120))
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", root)
	var reply any
	err := API(context.Background(), profile, "GET", "/version", nil, &reply)
	var socketError *net.OpError
	if !errors.As(err, &socketError) || socketError.Net != "unix" || socketError.Addr == nil || socketError.Addr.String() != Socket(profile) {
		t.Fatalf("long TMPDIR must preserve the original socket error without fallback: %v", err)
	}
	var errno syscall.Errno
	if !errors.As(err, &errno) || !strings.Contains(err.Error(), errno.Error()) || !strings.Contains(err.Error(), "TMPDIR") || strings.Contains(err.Error(), root) {
		t.Fatalf("socket error lost its cause, guidance or path redaction: %v", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatalf("failed proxy request created runtime files: %v %v", entries, err)
	}
}

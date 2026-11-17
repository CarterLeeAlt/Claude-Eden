package workbench

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestLocalControllerRestartWaitsForFreshRecordOnReusedPortAndPID(t *testing.T) {
	s := testServer(t)
	var err error
	s.identity, err = currentController()
	if err != nil {
		t.Fatal(err)
	}
	s.instanceID = strings.Repeat("c", 64)
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		s.handler().ServeHTTP(w, r)
	}))
	defer server.Close()
	u, _ := url.Parse(server.URL)
	port, _ := strconv.Atoi(u.Port())
	s.origin = "http://localhost:" + u.Port()
	oldToken := strings.Repeat("d", 64)
	// Simulate an abnormal exit: both the listener's port and its old PID
	// are reused, so PID liveness alone cannot identify the retired record.
	if err := writePrivate(filepath.Join(s.profile, "control.json"), controlFile{Port: port, Token: oldToken, PID: os.Getpid(), Controller: s.identity}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	d := &desktopLocal{profile: s.profile}
	type result struct {
		c    *controllerClient
		view desktopLocalReply
		err  error
	}
	done := make(chan result, 1)
	go func() { c, view, err := d.waitLocalController(ctx, oldToken, true, nil); done <- result{c, view, err} }()
	select {
	case got := <-done:
		if got.c != nil {
			got.c.close()
		}
		t.Fatalf("restart used the stale record before publication: %v", got.err)
	case <-time.After(250 * time.Millisecond):
	}
	if requests.Load() != 0 {
		t.Fatal("retired token sent to the reused listener")
	}
	if err := writePrivate(filepath.Join(s.profile, "control.json"), controlFile{Port: port, Token: s.token, PID: os.Getpid(), Controller: s.identity}); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-done:
		if got.c != nil {
			defer got.c.close()
		}
		if got.err != nil || got.view.Identity != s.identity || got.c.record.Token != s.token {
			t.Fatalf("fresh controller rejected: %+v %v", got.view, got.err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
}

func TestLocalControllerRestartWaitEndsOnCancellationOrProcessExit(t *testing.T) {
	for _, outcome := range []string{"cancel", "exit"} {
		t.Run(outcome, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			d := &desktopLocal{profile: t.TempDir()}
			exited := make(chan error, 1)
			want := errors.New("fixture exited")
			if outcome == "cancel" {
				cancel()
				want = context.Canceled
			} else {
				exited <- want
			}
			c, _, err := d.waitLocalController(ctx, "", true, exited)
			if c != nil {
				c.close()
				t.Fatal("unpublished controller connected")
			}
			if !errors.Is(err, want) {
				t.Fatalf("wait lost terminal reason: %v", err)
			}
		})
	}
}

func TestLocalControllerRestartStillRejectsFreshUnauthenticatedRecord(t *testing.T) {
	profile := t.TempDir()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Launcher refused", http.StatusUnauthorized)
	}))
	defer server.Close()
	u, _ := url.Parse(server.URL)
	port, _ := strconv.Atoi(u.Port())
	if err := writePrivate(filepath.Join(profile, "control.json"), controlFile{Port: port, Token: strings.Repeat("a", 64), PID: os.Getpid()}); err != nil {
		t.Fatal(err)
	}
	d := &desktopLocal{profile: profile}
	c, _, err := d.waitLocalController(t.Context(), strings.Repeat("b", 64), true, nil)
	if c != nil {
		c.close()
		t.Fatal("unauthenticated service connected")
	}
	if err == nil || !strings.Contains(err.Error(), "HTTP 401") {
		t.Fatalf("authentication refusal was not terminal: %v", err)
	}
}

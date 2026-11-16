package workbench

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestEnvironmentHealthSamplingAllowsProjectOpen(t *testing.T) {
	s := testServer(t)
	s.ready = true
	s.credentials.Token = "health-test"
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s.projectShares = map[string]*projectShare{root: {done: make(chan struct{})}}
	started, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	s.browserControl = browserClientForTest(t, s, browserTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/health" || r.Header.Get("Authorization") != "Bearer health-test" {
			t.Errorf("unexpected health request: %s", r.URL.Path)
		}
		close(started)
		select {
		case <-release:
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"ok":true}`))}, nil
		case <-r.Context().Done():
			return nil, r.Context().Err()
		}
	}))
	ctx, cancel := context.WithCancel(t.Context())
	defer func() { cancel(); close(release); <-done }()
	go func() { defer close(done); s.monitor(ctx) }()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("health sample did not start")
	}
	body, err := json.Marshal(map[string]any{"workspace": root, "add": true, "activate": true})
	if err != nil {
		t.Fatal(err)
	}
	w := request(s, "POST", "/api/projects/open", s.origin, string(body), true)
	if w.Code != http.StatusOK || s.workspace() != root {
		t.Fatalf("health sampling rejected project open: %d %s", w.Code, w.Body.String())
	}
}

func TestEnvironmentHealthResultRespectsAttachmentAndLifecycle(t *testing.T) {
	for _, state := range []string{"healthy", "unavailable", "reattached", "job", "closing"} {
		t.Run(state, func(t *testing.T) {
			s := testServer(t)
			s.ready = true
			s.credentials.Token = "unchanged-token"
			started, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
			ctx, cancel := context.WithCancel(t.Context())
			s.browserControl = browserClientForTest(t, s, browserTransport(func(r *http.Request) (*http.Response, error) {
				close(started)
				select {
				case <-release:
				case <-r.Context().Done():
				}
				cancel()
				status := http.StatusServiceUnavailable
				if state == "healthy" {
					status = http.StatusOK
				}
				return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(`{}`))}, nil
			}))
			go func() { defer close(done); s.monitor(ctx) }()
			defer func() { cancel(); <-done }()
			select {
			case <-started:
			case <-time.After(3 * time.Second):
				close(release)
				t.Fatal("health sample did not start")
			}
			s.mu.Lock()
			switch state {
			case "reattached":
				// Successful attach publishes a new generation even when guest
				// credentials survive a component restart.
				s.environmentGeneration++
			case "job":
				s.setJobForTest(Job{ID: "new-operation", Running: true})
			}
			s.mu.Unlock()
			if state == "closing" {
				s.beginClose()
			}
			close(release)
			select {
			case <-done:
			case <-time.After(3 * time.Second):
				t.Fatal("health result did not finish")
			}
			if s.ready != (state != "unavailable") {
				t.Fatalf("health result changed readiness across %s: %v", state, s.ready)
			}
		})
	}
}

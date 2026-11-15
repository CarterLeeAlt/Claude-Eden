package workbench

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sylvinus/agent-vm/internal/localsession"
)

type gatewayLocalFixture struct {
	endpoint                         *httptest.Server
	connections, suspended, commands atomic.Int32
	status                           DesktopLocalStatus
	release                          func(DesktopLocalStatus, bool) error
	connect                          func(context.Context) (*localsession.Session, error)
}

func gatewayFixture(t *testing.T) (*Server, *gatewayLocalFixture) {
	t.Helper()
	s := testServer(t)
	f := &gatewayLocalFixture{status: DesktopLocalStatus{Known: true, Available: true}}
	f.endpoint = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/account/suspend":
			var input map[string]any
			if r.Header.Get("Content-Type") != "application/json" || json.NewDecoder(r.Body).Decode(&input) != nil {
				http.Error(w, "invalid account request", 400)
				return
			}
			f.suspended.Add(1)
		default:
			f.commands.Add(1)
		}
		_, _ = w.Write([]byte(`{"upstream":"local-controller"}`))
	}))
	t.Cleanup(f.endpoint.Close)
	s.gateway = newDesktopGateway(f)
	t.Cleanup(s.gateway.close)
	return s, f
}

func (f *gatewayLocalFixture) Connect(ctx context.Context) (*localsession.Session, error) {
	f.connections.Add(1)
	if f.connect != nil {
		return f.connect(ctx)
	}
	return &localsession.Session{Origin: f.endpoint.URL, Client: f.endpoint.Client(), Transport: f.endpoint.Client().Transport, Context: ctx, Manifest: localsession.Manifest{Platform: "windows", InstanceID: strings.Repeat("a", 64)}}, nil
}

func (f *gatewayLocalFixture) Status(context.Context) (DesktopLocalStatus, error) {
	return f.status, nil
}

func (f *gatewayLocalFixture) Release(_ context.Context, expected DesktopLocalStatus, stop bool) error {
	if f.release == nil {
		return errors.New("unexpected release")
	}
	return f.release(expected, stop)
}

func candidateState(t *testing.T, d *desktopGateway, want string) *desktopCandidate {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		c := d.context().Candidate
		if c != nil && c.Connection.State == want {
			return c
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("candidate did not reach %s: %+v", want, d.context().Candidate)
	return nil
}

func gatewayRequest(s *Server, path, generation string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("GET", s.origin+path, nil)
	r.AddCookie(&http.Cookie{Name: s.cookieName(), Value: s.session})
	if generation != "" {
		r.Header.Set(connectionHeader, generation)
	}
	w := httptest.NewRecorder()
	s.handler().ServeHTTP(w, r)
	return w
}

func chooseLocal(t *testing.T, d *desktopGateway, id string) string {
	t.Helper()
	c, err := d.prepare(switchRequest{ConnectionID: id, Mode: "local"})
	if err != nil {
		t.Fatal(err)
	}
	candidateState(t, d, "connected")
	if err = d.commit(context.Background(), switchRequest{ConnectionID: id, CandidateID: c.ID}); err != nil {
		t.Fatal(err)
	}
	return c.ID
}

func TestDesktopLocalGenerationAndDisconnectedAdmission(t *testing.T) {
	s, f := gatewayFixture(t)
	d := s.gateway.desktop
	none := d.context().ConnectionID
	if c := d.context(); c.Mode != "none" || none == "" || f.connections.Load() != 0 {
		t.Fatal("cold shell started a controller", c)
	}
	if w := gatewayRequest(s, "/api/account", none); w.Code != 409 {
		t.Fatal("unselected view exposed a workload API", w.Code)
	}
	local := chooseLocal(t, d, none)
	if w := gatewayRequest(s, "/api/status", local); w.Code != 200 || !strings.Contains(w.Body.String(), "local-controller") {
		t.Fatal("selected view did not reach its controller", w.Code, w.Body.String())
	}
	for _, mode := range []string{"remote", "local", ""} {
		if _, err := d.prepare(switchRequest{ConnectionID: local, Mode: mode}); err == nil {
			t.Fatalf("invalid or duplicate candidate %q accepted", mode)
		}
	}
	if err := d.disconnect(context.Background(), switchRequest{ConnectionID: local}); err != nil {
		t.Fatal(err)
	}
	second := chooseLocal(t, d, d.context().ConnectionID)
	if second == local || f.suspended.Load() != 1 {
		t.Fatal("reconnect reused a generation or did not suspend the old view")
	}
	for _, stale := range []string{"", none, "local", local} {
		if w := gatewayRequest(s, "/api/status", stale); w.Code != 409 {
			t.Fatalf("stale ID %q reached the workload: %d", stale, w.Code)
		}
	}
	if err := d.disconnect(context.Background(), switchRequest{ConnectionID: local}); err == nil {
		t.Fatal("stale disconnect released the new view")
	}
	if w := gatewayRequest(s, "/api/status", second); w.Code != 200 {
		t.Fatal("current connection rejected", w.Code)
	}
}

func TestDesktopLocalCandidateFailureCancellationAndRetry(t *testing.T) {
	s, f := gatewayFixture(t)
	d := s.gateway.desktop
	id := d.context().ConnectionID
	f.connect = func(context.Context) (*localsession.Session, error) { return nil, errors.New("controller unavailable") }
	failed, err := d.prepare(switchRequest{ConnectionID: id, Mode: "local"})
	if err != nil {
		t.Fatal(err)
	}
	candidateState(t, d, "failed")
	if err := d.commit(context.Background(), switchRequest{ConnectionID: id, CandidateID: failed.ID}); err == nil {
		t.Fatal("failed candidate committed")
	}
	if err := d.cancelCandidate(switchRequest{ConnectionID: id, CandidateID: failed.ID}); err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	f.connect = func(ctx context.Context) (*localsession.Session, error) {
		close(started)
		<-ctx.Done()
		return nil, ctx.Err()
	}
	pending, err := d.prepare(switchRequest{ConnectionID: id, Mode: "local"})
	if err != nil {
		t.Fatal(err)
	}
	<-started
	if err := d.cancelCandidate(switchRequest{ConnectionID: id, CandidateID: pending.ID}); err != nil {
		t.Fatal(err)
	}
	if d.context().Candidate != nil || d.context().Mode != "none" {
		t.Fatal("cancel retained a pending view")
	}
	f.connect = nil
	current := chooseLocal(t, d, id)
	if w := gatewayRequest(s, "/api/status", current); w.Code != 200 {
		t.Fatal(w.Code)
	}
}

func TestDesktopLocalStopPinsControllerAndRestoresAdmissionAfterFailure(t *testing.T) {
	s, f := gatewayFixture(t)
	d := s.gateway.desktop
	id := chooseLocal(t, d, d.context().ConnectionID)
	f.status = DesktopLocalStatus{Available: true, Known: true, Running: true, ServiceRunning: true, LiveTerminals: 1, ControllerID: "current-controller"}
	in := switchRequest{ConnectionID: id, LocalControllerID: "obsolete-controller"}
	if err := d.stopLocalCard(context.Background(), in); err == nil {
		t.Fatal("stale controller stopped replacement")
	}
	in.LocalControllerID = f.status.ControllerID
	f.release = func(observed DesktopLocalStatus, stop bool) error {
		if !stop || observed.ControllerID != f.status.ControllerID {
			t.Fatal("wrong stop target")
		}
		if w := gatewayRequest(s, "/api/status", id); w.Code != 409 {
			t.Fatal("releasing view admitted new work")
		}
		return errDesktopLocalLocked
	}
	if err := d.stopLocalCard(context.Background(), in); !errors.Is(err, errDesktopLocalLocked) {
		t.Fatal(err)
	}
	if w := gatewayRequest(s, "/api/status", id); w.Code != 200 {
		t.Fatal("failed stop left view frozen")
	}
	f.release = func(DesktopLocalStatus, bool) error { return nil }
	if err := d.stopLocalCard(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	if d.context().Mode != "none" || d.context().ConnectionID == id {
		t.Fatal("successful stop retained the released generation")
	}
}

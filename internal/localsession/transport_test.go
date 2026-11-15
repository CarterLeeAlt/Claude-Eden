package localsession

import (
	"context"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func controllerFixture(t *testing.T, handler http.Handler) *Session {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("private")
		if err != nil || cookie.Value != "controller-proof" || r.Header.Get("Origin") != "http://"+r.Host || r.Header.Get("Authorization") != "" || r.Header.Get("Proxy-Authorization") != "" {
			t.Error("controller authentication or origin boundary changed")
			http.Error(w, "invalid session", http.StatusForbidden)
			return
		}
		if _, err := r.Cookie("desktop"); err == nil {
			t.Error("forwarded desktop cookie")
		}
		handler.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)
	u, _ := url.Parse(server.URL)
	u.Host = "localhost:" + u.Port()
	jar, _ := cookiejar.New(nil)
	jar.SetCookies(u, []*http.Cookie{{Name: "private", Value: "controller-proof", Path: "/"}})
	s, err := New(context.Background(), u.String(), Manifest{}, jar)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestControllerHTTPWebSocketSSEAndCancellation(t *testing.T) {
	sseEnded := make(chan struct{})
	s := controllerFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/check":
			io.WriteString(w, "authenticated")
		case "/stream":
			w.Header().Set("Content-Type", "text/event-stream")
			io.WriteString(w, "data: ready\n\n")
			w.(http.Flusher).Flush()
			<-r.Context().Done()
			close(sseEnded)
		case "/ws":
			connection, err := (&websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return r.Header.Get("Origin") == "http://"+r.Host }}).Upgrade(w, r, nil)
			if err != nil {
				return
			}
			defer connection.Close()
			kind, data, err := connection.ReadMessage()
			if err == nil {
				_ = connection.WriteMessage(kind, data)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	req, _ := http.NewRequest("GET", s.Origin+"/check", nil)
	req.Header.Set("Cookie", "desktop=desktop-secret")
	req.Header.Set("Authorization", "Bearer desktop-secret")
	req.Header.Set("Proxy-Authorization", "Basic desktop-secret")
	response, err := s.Client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if response.StatusCode != 200 || string(body) != "authenticated" {
		t.Fatalf("HTTP %d %q", response.StatusCode, body)
	}
	u, _ := url.Parse(s.Origin)
	proxy := httputil.NewSingleHostReverseProxy(u)
	proxy.Transport = s.Transport
	gateway := httptest.NewServer(proxy)
	defer gateway.Close()
	ws, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(gateway.URL, "http")+"/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Close()
	if err := ws.WriteMessage(websocket.TextMessage, []byte("hello")); err != nil {
		t.Fatal(err)
	}
	_, message, err := ws.ReadMessage()
	if err != nil || string(message) != "hello" {
		t.Fatalf("WebSocket %q %v", message, err)
	}
	stream, err := s.Client.Get(s.Origin + "/stream")
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Body.Close()
	if _, err := io.ReadFull(stream.Body, make([]byte, len("data: ready\n\n"))); err != nil {
		t.Fatal(err)
	}
	s.Close()
	select {
	case <-sseEnded:
	case <-time.After(3 * time.Second):
		t.Fatal("SSE outlived the view")
	}
	if _, err := s.Client.Get(s.Origin + "/check"); err == nil {
		t.Fatal("closed view accepted a request")
	}
}

func TestControllerRejectsOriginRedirectAndMutationReplay(t *testing.T) {
	var mutations atomic.Int64
	s := controllerFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/redirect":
			http.Redirect(w, r, "http://public.example/private", 302)
		case "/mutation":
			mutations.Add(1)
			if r.Header.Get("Idempotency-Key") != "" {
				t.Error("retained replay hint")
			}
			conn, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				t.Error(err)
				return
			}
			conn.Close()
		default:
			io.WriteString(w, "ok")
		}
	}))
	for _, origin := range []string{"http://public.example/check", s.Origin + "/redirect"} {
		if _, err := s.Client.Get(origin); err == nil {
			t.Fatalf("accepted %s", origin)
		}
	}
	req, _ := http.NewRequest("POST", s.Origin+"/mutation", strings.NewReader("operation"))
	req.Header.Set("Idempotency-Key", "do-not-replay")
	if _, err := s.Client.Do(req); err == nil {
		t.Fatal("missing mutation response accepted")
	}
	if mutations.Load() != 1 {
		t.Fatal("mutation replayed", mutations.Load())
	}
}

func TestExpiredControllerSessionInvalidatesView(t *testing.T) {
	var calls atomic.Int64
	s := controllerFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); w.WriteHeader(http.StatusUnauthorized) }))
	for range 2 {
		if _, err := s.Client.Get(s.Origin + "/expired"); err == nil {
			t.Fatal("expired session accepted")
		}
	}
	if calls.Load() != 1 || s.Context.Err() == nil {
		t.Fatal("expired view remained usable")
	}
}

func TestControllerRequiresLocalOriginAndExistingCookie(t *testing.T) {
	u, _ := url.Parse("http://localhost:12345")
	jar, _ := cookiejar.New(nil)
	jar.SetCookies(u, []*http.Cookie{{Name: "private", Value: "proof", Path: "/"}})
	for _, origin := range []string{"https://localhost:12345", "http://127.0.0.1:12345", "http://public.example:12345", "http://localhost", "http://localhost:12345/path", "http://localhost:12345?x=1", "http://name@localhost:12345"} {
		if s, err := New(context.Background(), origin, Manifest{}, jar); err == nil {
			s.Close()
			t.Fatalf("accepted %s", origin)
		}
	}
	empty, _ := cookiejar.New(nil)
	for _, cookies := range []http.CookieJar{nil, empty} {
		if s, err := New(context.Background(), u.String(), Manifest{}, cookies); err == nil {
			s.Close()
			t.Fatal("unauthenticated view accepted")
		}
	}
}

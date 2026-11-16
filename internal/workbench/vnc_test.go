package workbench

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// A short native popup capture can hold the serialized RFB reader. Mouse
// bursts during it must arrive in order once capture finishes, not disconnect.
func TestVNCInputBurstSurvivesBriefGuestCapture(t *testing.T) {
	s := testServer(t)
	remote, peer := net.Pipe()
	finished := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(finished)
		s.streamVNC(w, r, remote, remote, func() { _ = remote.Close() }, func(bool) error { return nil })
	}))
	s.origin = server.URL
	defer func() { remote.Close(); peer.Close(); server.Close() }()
	ws, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), http.Header{"Origin": {server.URL}})
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Close()
	want := make([]byte, 0, 120)
	for i := byte(0); i < 20; i++ {
		packet := []byte{5, 0, 0, i, 0, i}
		want = append(want, packet...)
		if err := ws.WriteMessage(websocket.BinaryMessage, packet); err != nil {
			t.Fatal(err)
		}
	}
	// The previous four-message / one-second queue failed this condition.
	select {
	case <-finished:
		t.Fatal("display disconnected during a brief capture")
	case <-time.After(1300 * time.Millisecond):
	}
	peer.SetReadDeadline(time.Now().Add(3 * time.Second))
	got := make([]byte, len(want))
	if _, err := io.ReadFull(peer, got); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("input was dropped or reordered")
	}
	ws.Close()
	select {
	case <-finished:
	case <-time.After(3 * time.Second):
		t.Fatal("disconnect did not cancel transport")
	}
}

type observedVNCTransport struct {
	net.Conn
	readEntered, readExited   chan struct{}
	writeEntered, writeExited chan struct{}
	readOnce, writeOnce       sync.Once
}

func (c *observedVNCTransport) Read(data []byte) (int, error) {
	c.readOnce.Do(func() { close(c.readEntered) })
	n, err := c.Conn.Read(data)
	if err != nil {
		close(c.readExited)
	}
	return n, err
}

func (c *observedVNCTransport) Write(data []byte) (int, error) {
	c.writeOnce.Do(func() { close(c.writeEntered) })
	n, err := c.Conn.Write(data)
	if err != nil {
		close(c.writeExited)
	}
	return n, err
}

func TestVNCClosesBlockedTransportAndWaitsForReader(t *testing.T) {
	for _, condition := range []string{"cancel-read", "cancel-write", "disconnect-read", "disconnect-write"} {
		t.Run(condition, func(t *testing.T) {
			s := testServer(t)
			ctx, cancel := context.WithCancel(context.Background())
			remote, peer := net.Pipe()
			transport := &observedVNCTransport{Conn: remote, readEntered: make(chan struct{}), readExited: make(chan struct{}), writeEntered: make(chan struct{}), writeExited: make(chan struct{})}
			finished := make(chan struct{})
			var closed atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer close(finished)
				s.streamVNC(w, r.WithContext(ctx), transport, transport, func() {
					closed.Add(1)
					_ = remote.Close()
				}, func(bool) error { return nil })
			}))
			s.origin = server.URL
			defer func() { cancel(); _ = remote.Close(); _ = peer.Close(); server.Close() }()
			ws, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), http.Header{"Origin": {server.URL}})
			if err != nil {
				t.Fatal(err)
			}
			defer ws.Close()
			awaitJobSignal(t, transport.readEntered)
			writeBlocked := strings.HasSuffix(condition, "write")
			if writeBlocked {
				if err := ws.WriteMessage(websocket.BinaryMessage, []byte("client input")); err != nil {
					t.Fatal(err)
				}
				// The peer never reads: writing to the remote transport cannot finish.
				awaitJobSignal(t, transport.writeEntered)
			}
			if strings.HasPrefix(condition, "cancel") {
				cancel()
			} else {
				_ = ws.Close()
			}
			select {
			case <-finished:
			case <-time.After(3 * time.Second):
				t.Fatal("VNC handler survived cancellation or client disconnection with blocked transport I/O")
			}
			if closed.Load() != 1 {
				t.Fatalf("transport closed %d times", closed.Load())
			}
			select {
			case <-transport.readExited:
			default:
				t.Fatal("VNC handler returned before the remote reader exited")
			}
			if writeBlocked {
				select {
				case <-transport.writeExited:
				default:
					t.Fatal("VNC handler returned before the remote writer exited")
				}
			}
		})
	}
}

func TestVNCRetainedConnectionReleasesAndRestoresDisplay(t *testing.T) {
	s := testServer(t)
	var demand displayViewDemand
	changes := make(chan bool, 8)
	remote, peer := net.Pipe()
	finished := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(finished)
		view, err := demand.connect(func(show bool) error { changes <- show; return nil })
		if err != nil {
			t.Error(err)
			return
		}
		defer view.close()
		s.streamVNC(w, r, remote, remote, func() { _ = remote.Close() }, view.setVisible)
	}))
	s.origin = server.URL
	defer func() { remote.Close(); peer.Close(); server.Close() }()
	ws, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), http.Header{"Origin": {server.URL}})
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Close()
	waitChange := func(want bool) {
		t.Helper()
		select {
		case got := <-changes:
			if got != want {
				t.Fatalf("display visibility = %t, want %t", got, want)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("display visibility did not change")
		}
	}
	waitChange(true)
	for range 3 {
		if err := ws.WriteJSON(map[string]any{"type": "visibility", "visible": false}); err != nil {
			t.Fatal(err)
		}
		waitChange(false)
		demand.mu.Lock()
		connections, viewers := demand.connections, demand.viewers
		demand.mu.Unlock()
		if connections != 1 || viewers != 0 {
			t.Fatalf("hidden connection/demand = %d/%d", connections, viewers)
		}
		if err := ws.WriteJSON(map[string]any{"type": "visibility", "visible": true}); err != nil {
			t.Fatal(err)
		}
		if err := ws.WriteMessage(websocket.BinaryMessage, []byte("input")); err != nil {
			t.Fatal(err)
		}
		waitChange(true)
		peer.SetReadDeadline(time.Now().Add(2 * time.Second))
		data := make([]byte, 5)
		if _, err := io.ReadFull(peer, data); err != nil || string(data) != "input" {
			t.Fatalf("resumed input: %q, %v", data, err)
		}
	}
	ws.Close()
	awaitJobSignal(t, finished)
	demand.mu.Lock()
	connections, viewers := demand.connections, demand.viewers
	demand.mu.Unlock()
	if connections != 0 || viewers != 0 {
		t.Fatalf("closed connection leaked demand: %d/%d", connections, viewers)
	}
	waitChange(false)
}

func TestVNCRejectsInvalidOrFailedVisibilityControl(t *testing.T) {
	for _, message := range []string{`{`, `{"type":"visibility"}`, `{"type":"visibility","visible":"false"}`, `{"type":"other","visible":false}`, `{"type":"visibility","visible":true}`} {
		t.Run(message, func(t *testing.T) {
			s := testServer(t)
			remote, peer := net.Pipe()
			finished := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer close(finished)
				s.streamVNC(w, r, remote, remote, func() { _ = remote.Close() }, func(bool) error { return errors.New("display unavailable") })
			}))
			s.origin = server.URL
			defer func() { remote.Close(); peer.Close(); server.Close() }()
			ws, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), http.Header{"Origin": {server.URL}})
			if err != nil {
				t.Fatal(err)
			}
			defer ws.Close()
			if err := ws.WriteMessage(websocket.TextMessage, []byte(message)); err != nil {
				t.Fatal(err)
			}
			awaitJobSignal(t, finished)
		})
	}
}

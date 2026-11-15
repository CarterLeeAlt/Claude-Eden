// Package localsession authenticates desktop views of the local controller.
package localsession

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// New attaches a desktop view after its private launch ticket has been
// exchanged. Cookies, origin and stream cancellation remain bound to this
// controller. Closing a session releases the view, not the workload.
func New(ctx context.Context, origin string, manifest Manifest, jar http.CookieJar) (*Session, error) {
	u, err := url.Parse(origin)
	if err != nil || u.Scheme != "http" || u.Hostname() != "localhost" || u.Port() == "" || u.User != nil || origin != "http://"+u.Host || jar == nil || len(jar.Cookies(u)) == 0 {
		return nil, errors.New("本机控制服务会话无效")
	}
	endpoint := net.JoinHostPort("127.0.0.1", u.Port())
	dial := &net.Dialer{Timeout: 5 * time.Second}
	life, cancel := context.WithCancel(ctx)
	base := &http.Transport{MaxIdleConnsPerHost: 16, IdleConnTimeout: time.Minute, ResponseHeaderTimeout: 30 * time.Second, DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		if network != "tcp" || address != u.Host {
			return nil, errors.New("本机服务地址已改变")
		}
		conn, err := dial.DialContext(ctx, "tcp4", endpoint)
		if errors.Is(err, connectionRefused) {
			cancel()
		}
		return conn, err
	}}
	bound := &sessionTransport{base: base, jar: jar, origin: origin, ctx: life}
	s := &Session{Manifest: manifest, Origin: origin, Context: life, Transport: bound}
	var once sync.Once
	s.close = func() error {
		once.Do(func() { cancel(); base.CloseIdleConnections() })
		return nil
	}
	bound.invalidate = s.Close
	s.Client = &http.Client{Transport: bound, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("本机服务重定向已拒绝") }}
	context.AfterFunc(life, func() { _ = s.Close() })
	return s, nil
}

type Manifest struct {
	APIProtocol  int             `json:"api_protocol"`
	InstanceID   string          `json:"instance_id"`
	Version      string          `json:"version"`
	Platform     string          `json:"platform"`
	Runtime      string          `json:"runtime"`
	Capabilities map[string]bool `json:"capabilities"`
}

type State struct {
	State        string    `json:"state"`
	ConnectionID string    `json:"connection_id,omitempty"`
	InstanceID   string    `json:"instance_id,omitempty"`
	Phase        string    `json:"phase,omitempty"`
	Message      string    `json:"message,omitempty"`
	ErrorCode    string    `json:"error_code,omitempty"`
	Manifest     *Manifest `json:"manifest,omitempty"`
}

// Session provides authenticated HTTP, WebSocket and SSE access to one
// controller. Its cancellation never extends to the controller's own lifetime.
type Session struct {
	Manifest  Manifest
	Client    *http.Client
	Transport http.RoundTripper
	Context   context.Context
	Origin    string
	close     func() error
}

func (s *Session) Close() error {
	if s == nil || s.close == nil {
		return nil
	}
	return s.close()
}

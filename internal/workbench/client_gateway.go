package workbench

import (
	"context"
	"errors"
	"mime"
	"net/http"
	"net/http/httputil"
	"net/url"
	"runtime"
	"strings"

	"github.com/sylvinus/agent-vm/internal/localsession"
)

const connectionHeader = "X-Eden-Connection"

type clientGateway struct {
	desktop *desktopGateway
}

func newClientGateway() *clientGateway { return &clientGateway{} }

func (g *clientGateway) close() {
	if g.desktop != nil {
		g.desktop.close()
	}
}

type ClientContext struct {
	DesktopGateway bool               `json:"desktop_gateway,omitempty"`
	Candidate      *desktopCandidate  `json:"candidate,omitempty"`
	Mode           string             `json:"mode"`
	ClientPlatform string             `json:"client_platform"`
	ConnectionID   string             `json:"connection_id"`
	State          localsession.State `json:"connection"`
	InstanceID     string             `json:"instance_id,omitempty"`
	ServerPlatform string             `json:"server_platform"`
	Name           string             `json:"name,omitempty"`
}

func (g *clientGateway) context() ClientContext {
	if g.desktop != nil {
		return g.desktop.context()
	}
	return ClientContext{Mode: "local", ConnectionID: "local", ClientPlatform: runtime.GOOS, ServerPlatform: runtime.GOOS, State: localsession.State{State: "connected"}}
}

func (s *Server) clientRequest(w http.ResponseWriter, r *http.Request) {
	g := s.gateway
	if g == nil {
		http.Error(w, "本机连接服务不可用", http.StatusServiceUnavailable)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/client/")
	if path == "browser-extensions" && (r.Method == http.MethodGet || r.Method == http.MethodPost) {
		s.localBrowserExtensions(w, r)
		return
	}
	if g.desktop != nil && g.desktop.handle(path, w, r) {
		return
	}
	switch {
	case r.Method == http.MethodGet && path == "context":
		jsonReply(w, g.context())
	case r.Method == http.MethodGet && path == "connection":
		jsonReply(w, g.context().State)
	default:
		http.NotFound(w, r)
	}
}

func requestConnection(r *http.Request) string {
	if value := r.Header.Get(connectionHeader); value != "" {
		return value
	}
	return r.URL.Query().Get("connection")
}

// routeController follows local Host/Origin/session authentication. Only
// business APIs cross into the authenticated workload controller; the desktop
// listener owns its launch, update, native-input and diagnostics entry points.
func (s *Server) routeController(w http.ResponseWriter, r *http.Request) bool {
	if s.gateway == nil || !strings.HasPrefix(r.URL.Path, "/api/") {
		return false
	}
	if strings.HasPrefix(r.URL.Path, "/api/client/") {
		s.clientRequest(w, r)
		return true
	}
	switch r.URL.Path {
	case "/api/session", "/api/launch", "/api/handoff", "/api/diagnostics/events":
		return false
	case "/api/controller/manifest":
		if s.gateway.desktop != nil {
			http.NotFound(w, r)
			return true
		}
		return false
	}
	if s.gateway.desktop == nil {
		if id := requestConnection(r); id != "" && id != "local" {
			http.Error(w, "连接已经变化，请重新打开当前工作台", http.StatusConflict)
			return true
		}
		return false
	}
	session, err := s.gateway.desktop.acquire(requestConnection(r))
	if err != nil {
		http.Error(w, "本机连接不可用或已变化，请重新连接工作台", http.StatusConflict)
		return true
	}
	if r.URL.Path == "/api/diagnostics/export" {
		if !s.diagnosticMu.TryLock() {
			http.Error(w, "正在导出另一份诊断包，请稍候", http.StatusConflict)
			return true
		}
		defer s.diagnosticMu.Unlock()
	}
	target, err := url.Parse(session.Origin)
	if err != nil {
		http.Error(w, "本机入口无效", http.StatusBadGateway)
		return true
	}
	ctx, cancel := context.WithCancel(r.Context())
	stop := context.AfterFunc(session.Context, cancel)
	defer func() { stop(); cancel() }()
	proxy := &httputil.ReverseProxy{
		Transport:     session.Transport,
		FlushInterval: -1,
		Rewrite: func(p *httputil.ProxyRequest) {
			p.SetURL(target)
			p.Out.Host = target.Host
			p.Out.Header.Del(connectionHeader)
			query := p.Out.URL.Query()
			query.Del("connection")
			p.Out.URL.RawQuery = query.Encode()
		},
		ModifyResponse: func(response *http.Response) error {
			response.Header.Del("Set-Cookie")
			response.Header.Del("Content-Security-Policy")
			response.Header.Del("Access-Control-Allow-Origin")
			response.Header.Set("X-Content-Type-Options", "nosniff")
			if response.StatusCode == http.StatusOK && response.Request.URL.Path == "/api/diagnostics/export" && strings.HasPrefix(response.Header.Get("Content-Type"), "application/zip") {
				return s.addDesktopDiagnosticResponse(response)
			}
			if response.StatusCode >= 300 && response.StatusCode < 400 {
				return errors.New("controller redirects are refused")
			}
			if downloadID := strings.TrimSuffix(strings.TrimPrefix(response.Request.URL.Path, "/api/browser/files/downloads/"), "/content"); strings.HasSuffix(response.Request.URL.Path, "/content") && browserFileID.MatchString(downloadID) {
				name := "download"
				_, parameters, err := mime.ParseMediaType(response.Header.Get("Content-Disposition"))
				if err == nil && browserDownloadName(parameters["filename"]) {
					name = parameters["filename"]
				}
				response.Header.Set("Content-Type", "application/octet-stream")
				response.Header.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
				return nil
			}
			if response.StatusCode != http.StatusSwitchingProtocols && response.StatusCode != http.StatusNoContent {
				contentType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
				if err != nil {
					return errors.New("controller response has no supported content type")
				}
				switch contentType {
				case "application/json", "text/plain", "text/event-stream", "application/octet-stream", "application/zip", "image/png", "image/jpeg", "image/webp":
				default:
					return errors.New("controller active content is refused")
				}
			}
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, _ error) {
			http.Error(w, "本机连接已中断；已提交的操作请重连后核对", http.StatusBadGateway)
		},
	}
	proxy.ServeHTTP(w, r.WithContext(ctx))
	return true
}

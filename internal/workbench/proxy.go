package workbench

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/sylvinus/agent-vm/internal/eden"
	"github.com/sylvinus/agent-vm/internal/managedproxy"
)

var (
	validateProxyCore = managedproxy.Validate
	startProxyCore    = managedproxy.Start
)

type proxyInput struct {
	NodeID string          `json:"node_id,omitempty"`
	Node   *proxyNodeInput `json:"node,omitempty"`
	Port   int             `json:"port,omitempty"`
}

type proxyNodeInput struct {
	Name                 string   `json:"name"`
	Type                 string   `json:"type"`
	Server               string   `json:"server"`
	Port                 int      `json:"port"`
	Username             string   `json:"username,omitempty"`
	AuthMethod           string   `json:"auth_method,omitempty"`
	Password             *string  `json:"password,omitempty"`
	PrivateKey           *string  `json:"private_key,omitempty"`
	PrivateKeyPassphrase *string  `json:"private_key_passphrase,omitempty"`
	HostKey              []string `json:"host_key,omitempty"`
}

// Omitted secrets keep the saved value only for the same protocol/authentication.
// Empty strings explicitly clear them. Responses never contain these secrets.
func (v proxyNodeInput) node(previous managedproxy.Node) managedproxy.Node {
	n := managedproxy.Node{ID: previous.ID, Name: strings.TrimSpace(v.Name), Type: v.Type,
		Server: strings.TrimSpace(v.Server), Port: v.Port, Username: strings.TrimSpace(v.Username),
		AuthMethod: v.AuthMethod, HostKey: append([]string(nil), v.HostKey...)}
	if previous.Type == n.Type && previous.AuthMethod == n.AuthMethod {
		n.Password, n.PrivateKey, n.PrivateKeyPassphrase = previous.Password, previous.PrivateKey, previous.PrivateKeyPassphrase
	}
	if v.Password != nil {
		n.Password = *v.Password
	}
	if v.PrivateKey != nil {
		n.PrivateKey = *v.PrivateKey
	}
	if v.PrivateKeyPassphrase != nil {
		n.PrivateKeyPassphrase = *v.PrivateKeyPassphrase
	}
	return n
}

func (s *Server) proxyStatus(w http.ResponseWriter, r *http.Request) {
	p, err := managedproxy.Load(s.profile)
	if err != nil {
		requestError(w, err)
		return
	}
	cfg, err := loadConfig(s.configPath)
	if err != nil {
		requestError(w, err)
		return
	}
	nodes := []any{}
	for _, n := range p.Nodes {
		nodes = append(nodes, map[string]any{"id": n.ID, "name": n.Name, "type": n.Type,
			"server": n.Server, "port": n.Port, "username": n.Username, "auth_method": n.AuthMethod,
			"host_key": n.HostKey, "has_password": n.Password != "", "has_private_key": n.PrivateKey != "",
			"has_private_key_passphrase": n.PrivateKeyPassphrase != ""})
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	_, appliedErr := managedproxy.Applied(s.profile, cfg.ProxyURL, cfg.ProxyRevision)
	active := appliedErr == nil
	running := managedproxy.Running(ctx, s.profile, p)
	state, label := "unconfigured", "尚未配置"
	_, selected := p.Find(p.Selected)
	if selected {
		state, label = "unapplied", "尚未应用"
	}
	if active {
		state, label = "stopped", "代理已停止"
		if running {
			state, label = "prepared", "待启动环境"
			if s.vmRunning() {
				state, label = "paused", "外网未放行"
				if client, e := s.gateClient(); e == nil {
					if gate, e := client.Status(ctx); e == nil {
						if gate.Mode == eden.GateChecking {
							state, label = "checking", "正在检查"
						} else if gate.Mode == eden.GateOpen && gate.Report != nil && gate.Report.Authorizes(gate.RunID, cfg.Fingerprint(), time.Now()) == nil && time.Now().Before(gate.Until) {
							state, label = "ready", "已联网"
						}
					}
				}
			}
		}
	}
	var version struct {
		Version string `json:"version"`
	}
	if running {
		_ = managedproxy.API(ctx, s.profile, "GET", "/version", nil, &version)
	}
	_, coreErr := managedproxy.CorePath()
	jsonReply(w, map[string]any{"nodes": nodes, "selected": p.Selected, "port": p.Port, "running": running, "active": active, "state": state, "state_label": label, "network_open": state == "ready", "core_available": coreErr == nil, "version": version.Version, "revision": p.Revision, "bootstrap_dns": strings.Join(managedproxy.BootstrapDNSURLs(), "、"), "udp_blocked": true, "ipv6_blocked": true})
}
func (s *Server) proxyMutation(w http.ResponseWriter, r *http.Request) {
	action := r.PathValue("action")
	allowed := map[string]bool{"add": true, "edit": true, "delete": true, "select": true, "restart": true, "stop": true}
	if !allowed[action] {
		http.NotFound(w, r)
		return
	}
	var input proxyInput
	if !readJSON(w, r, &input, 768<<10) {
		return
	}
	s.mu.Lock()
	pauseVersion := s.safety.pauseVersion
	s.mu.Unlock()
	s.launch(w, "proxy", func() error { return s.changeProxy(action, input, &pauseVersion) })
}
func (s *Server) changeProxy(action string, in proxyInput, resumeVersion *uint64) error {
	p, err := managedproxy.Load(s.profile)
	if err != nil {
		return err
	}
	cfg, err := loadConfig(s.configPath)
	if err != nil {
		return err
	}
	ctx := s.operationContext()
	if action == "stop" {
		if err = s.closeProxyGate("内置代理已停止，外网保持关闭"); err != nil {
			return err
		}
		return managedproxy.Stop(s.profile)
	}
	if action == "select" || action == "restart" {
		if action == "select" {
			if _, ok := p.Find(in.NodeID); !ok {
				return errors.New("所选节点已不存在，请刷新列表")
			}
			p.Selected = in.NodeID
		}
		if in.Port != 0 {
			if in.Port < 1024 || in.Port > 65535 {
				return errors.New("代理端口应为 1024–65535")
			}
			p.Port = in.Port
		}
		return s.applyProxy(p, cfg, resumeVersion, action != "restart")
	}
	index := slices.IndexFunc(p.Nodes, func(n managedproxy.Node) bool { return n.ID == in.NodeID })
	if action == "delete" {
		if index < 0 {
			return errors.New("代理节点不存在")
		}
		if p.Selected == in.NodeID {
			if err := s.closeProxyGate("当前节点已移除，外网保持关闭"); err != nil {
				return err
			}
			if err := managedproxy.Stop(s.profile); err != nil {
				return err
			}
			p.Selected, p.Revision = "", ""
		}
		p.Nodes = append(p.Nodes[:index], p.Nodes[index+1:]...)
		delete(p.ExitPolicies, in.NodeID)
		return p.Save(s.profile)
	}
	if (action != "add" && action != "edit") || in.Node == nil || action == "add" && in.NodeID != "" {
		return errors.New("代理节点操作无效")
	}
	previous := managedproxy.Node{ID: managedproxy.Token()[:24]}
	if action == "edit" {
		if index < 0 {
			return errors.New("代理节点不存在")
		}
		previous = p.Nodes[index]
	}
	node := in.Node.node(previous)
	if err := p.SetNode(node); err != nil {
		return err
	}
	candidate := p
	if candidate.Selected == "" {
		candidate.Selected = node.ID
	}
	if err := validateProxyCore(ctx, s.profile, candidate); err != nil {
		return err
	}
	if action == "edit" && node.ID == p.Selected && cfg.ProxyRevision != "" && cfg.ProxyURL == p.Endpoint() {
		// Editing the active node revalidates its route but preserves a manual pause.
		return s.applyProxy(p, cfg, nil, false)
	}
	s.setMessage("节点已保存；选择节点并应用代理后生效")
	return p.Save(s.profile)
}
func (s *Server) closeProxyGate(reason string) error {
	if !s.vmRunning() {
		return nil
	}
	return s.revokeNetworkNow(reason, false)
}

func (s *Server) activateProxy(p managedproxy.Store, old eden.Config) (returned error) {
	// The explicit start operation already requested networking. A pause made
	// after that request must survive any core recovery within the start.
	return s.applyProxy(p, old, nil, false)
}

func (s *Server) applyProxy(p managedproxy.Store, old eden.Config, resumeVersion *uint64, selectExit bool) (returned error) {
	ctx := s.operationContext()
	previous, err := managedproxy.Load(s.profile)
	if err != nil {
		return err
	}
	s.setMessage("正在验证节点配置…")
	if err := validateProxyCore(ctx, s.profile, p); err != nil {
		return err
	}
	// Changing the endpoint requires an environment restart; node-only switches
	// keep its SSH sessions and VM alive and revoke the gate synchronously.
	restartVM := s.vmRunning() && old.ProxyURL != p.Endpoint()
	if err := s.closeProxyGate("代理节点切换中，旧连接及检测授权已关闭"); err != nil {
		return err
	}
	if restartVM {
		if err := s.stopEnvironment(); err != nil {
			return err
		}
	}
	s.setMessage("正在应用内置代理节点…")
	if err := managedproxy.Stop(s.profile); err != nil {
		return err
	}
	committed := false
	defer func() {
		if returned != nil && !committed {
			_ = previous.Save(s.profile)
			_ = writePrivate(s.configPath, old)
		}
	}()
	p.Revision = managedproxy.Token()
	cfg := old
	cfg.ExitPolicy = selectProxyExitPolicy(&p, previous, old.ExitPolicy, selectExit)
	if err := p.Save(s.profile); err != nil {
		return err
	}
	cfg.ProxyURL = p.Endpoint()
	cfg.ProxyRevision = p.Revision
	if err := writePrivate(s.configPath, cfg); err != nil {
		return err
	}
	if err := startProxyCore(ctx, s.profile, p); err != nil {
		return err
	}
	committed = true
	if resumeVersion != nil {
		if err := s.resumeNetworkIntentAt(*resumeVersion); err != nil {
			return err
		}
	}
	s.mu.Lock()
	// Only a user request may resume networking, and a later pause wins even
	// when it arrived while node validation or core startup was pending.
	paused := s.networkPaused
	s.exitProbeError = ""
	s.locationError = ""
	s.location = nil
	s.safety.Report = nil
	s.mu.Unlock()
	if s.vmRunning() {
		client, err := s.gateClient()
		if err != nil {
			return err
		}
		state, err := client.SyncProxyRevision(ctx)
		if err != nil {
			return err
		}
		s.publishGate(state)
		if paused {
			return nil
		}
		s.setMessage("节点已应用，正在重新检查环境出口…")
		return s.runSafetyCheck()
	}
	if restartVM {
		return s.startEnvironment(false)
	}
	s.setMessage("节点已应用；启动 Claude Eden 环境后完成联网检查")
	return nil
}

func (s *Server) proxyDelay(w http.ResponseWriter, r *http.Request) {
	var in proxyInput
	if !readJSON(w, r, &in) {
		return
	}
	p, err := managedproxy.Load(s.profile)
	if err != nil {
		requestError(w, err)
		return
	}
	if _, ok := p.Find(in.NodeID); !ok {
		requestError(w, errors.New("节点不存在"))
		return
	}
	var v struct {
		Delay int `json:"delay"`
	}
	err = managedproxy.API(r.Context(), s.profile, "GET", "/proxies/"+url.PathEscape(in.NodeID)+"/delay?timeout=5000&url="+url.QueryEscape("https://www.gstatic.com/generate_204"), nil, &v)
	if err != nil {
		requestError(w, errors.New("节点未载入核心或测速失败；请先应用节点以载入当前列表"))
		return
	}
	jsonReply(w, v)
}
func (s *Server) proxyConnections(w http.ResponseWriter, r *http.Request) {
	var response struct {
		Upload      int64 `json:"uploadTotal"`
		Download    int64 `json:"downloadTotal"`
		Connections []struct {
			ID       string `json:"id"`
			Upload   int64  `json:"upload"`
			Download int64  `json:"download"`
			Metadata struct {
				Network string `json:"network"`
				Host    string `json:"host"`
				IP      string `json:"destinationIP"`
				Port    string `json:"destinationPort"`
			} `json:"metadata"`
		} `json:"connections"`
	}
	if err := managedproxy.API(r.Context(), s.profile, "GET", "/connections", nil, &response); err != nil {
		jsonReply(w, map[string]any{"connections": []any{}, "uploadTotal": 0, "downloadTotal": 0})
		return
	}
	items := []any{}
	for i, c := range response.Connections {
		if i >= 100 {
			break
		}
		host := c.Metadata.Host
		if host == "" {
			host = c.Metadata.IP
		}
		items = append(items, map[string]any{"id": c.ID, "host": host, "port": c.Metadata.Port, "network": c.Metadata.Network, "upload": c.Upload, "download": c.Download})
	}
	jsonReply(w, map[string]any{"connections": items, "uploadTotal": response.Upload, "downloadTotal": response.Download})
}
func (s *Server) proxyLogs(w http.ResponseWriter, r *http.Request) {
	p, err := managedproxy.Load(s.profile)
	if err != nil {
		requestError(w, err)
		return
	}
	// Whitelisted lifecycle messages avoid publishing upstream raw log lines,
	// which can contain credentials and host paths.
	lines := []string{"客机公网 UDP、IPv6、ICMP：阻断", "业务 DNS：经选定节点转发", "代理基础设施 DNS：" + strings.Join(managedproxy.BootstrapDNSURLs(), "、") + "（主机网络，并行查询）"}
	if managedproxy.Running(r.Context(), s.profile, p) {
		lines = append(lines, "内置核心运行中；已读回当前节点选择")
	} else {
		lines = append(lines, "内置核心未就绪；不会直连回退")
	}
	s.mu.Lock()
	if issue := s.currentNetworkErrorLocked(); issue != "" {
		lines = append(lines, issue)
	}
	s.mu.Unlock()
	jsonReply(w, map[string]any{"lines": lines})
}
func (s *Server) checkProxyHealth() {
	job := s.jobs.snapshot()
	if job.Running {
		return
	}
	cfg, configErr := loadConfig(s.configPath)
	if !s.vmRunning() {
		return
	}
	p, err := managedproxy.Load(s.profile)
	ctx, cancel := context.WithTimeout(s.lifetimeContext(), 2*time.Second)
	defer cancel()
	if configErr == nil && err == nil && cfg.RequireManagedProxy(s.profile) == nil && managedproxy.Running(ctx, s.profile, p) {
		return
	}
	// The health request can overlap a new start, stop or proxy selection. A
	// stale observation must not revoke that operation's network state.
	if !s.op.TryLock() {
		return
	}
	defer s.op.Unlock()
	current, currentErr := loadConfig(s.configPath)
	nextJob := s.jobs.snapshot()
	if nextJob.Running || nextJob.ID != job.ID || s.lifetimeContext().Err() != nil || (configErr == nil) != (currentErr == nil) || (currentErr == nil && current != cfg) {
		return
	}
	if currentErr != nil {
		_ = s.closeProxyGate("代理配置无法读取，外网保持关闭")
		return
	}
	_ = s.closeProxyGate("内置代理已中断或节点状态变化，请重试应用节点")
	s.mu.Lock()
	s.safety.monitorError = "内置代理未就绪；外网保持关闭"
	s.mu.Unlock()
}

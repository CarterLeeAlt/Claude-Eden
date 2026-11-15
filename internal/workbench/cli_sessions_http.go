package workbench

import (
	"errors"
	"net/http"
	"os"
	"strconv"
)

func (s *Server) cliSessionRoutes(mux routeRegistrar) {
	mux.HandleFunc("GET /api/cli/sessions", s.cliSessionList)
	mux.HandleFunc("GET /api/cli/sessions/{id}", s.cliSessionView)
	mux.HandleFunc("POST /api/cli/sessions/{id}/open", s.cliOpenTerminal)
	mux.HandleFunc("POST /api/cli/sessions/{id}/metadata", s.cliSessionMetadata)
	mux.HandleFunc("POST /api/cli/sessions/{id}/account", s.cliSessionAccount)
	mux.HandleFunc("POST /api/cli/sessions/{id}/archive", s.cliArchiveSession)
	mux.HandleFunc("POST /api/cli/sessions/{id}/restore", s.cliRestoreSession)
	mux.HandleFunc("POST /api/cli/sessions/{id}/purge", s.cliPurgeSession)
}

func (s *Server) cliOpenTerminal(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Automatic bool `json:"automatic"`
	}
	if !readJSON(w, r, &input) {
		return
	}
	s.cliConnectMu.Lock()
	defer s.cliConnectMu.Unlock()
	view, ok := s.authorizedCLISession(w, r)
	if !ok {
		return
	}
	s.cliSessions.mu.Lock()
	record, err := s.cliSessions.record(s.profile, view.ID)
	s.cliSessions.mu.Unlock()
	if err != nil {
		http.Error(w, "无法读取会话", 500)
		return
	}
	existing := s.terminals.get(record.TerminalID)
	needsTerminal := existing == nil || !existing.ownsConversation()
	if needsTerminal {
		s.mu.Lock()
		launcher := claudeTerminalCommand(s.preferences)
		s.mu.Unlock()
		if !sharedCLILauncher(launcher) || record.Launcher != "" && launcher != record.Launcher {
			http.Error(w, "主机启动命令与原会话不一致，请恢复原配置后重试", 422)
			return
		}
	}
	if (record.Resume || record.Connection != "") && needsTerminal {
		var state struct {
			Running      bool   `json:"running"`
			ChannelReady bool   `json:"channel_ready"`
			Error        string `json:"error"`
		}
		mode := "active"
		if input.Automatic {
			if err := s.terminals.canStart(); err != nil {
				http.Error(w, err.Error(), 409)
				return
			}
			mode = "handoff"
		}
		query := map[string]any{"cli_id": record.CLIID, "config": record.GuestConfig, "workspace": record.GuestWorkspace}
		if err := s.runCLIChannel(r.Context(), mode, query, &state); err != nil {
			http.Error(w, err.Error(), 503)
			return
		}
		if state.Error != "" {
			http.Error(w, state.Error, 409)
			return
		}
		if state.Running {
			if input.Automatic {
				if state.ChannelReady {
					var result struct {
						OK bool `json:"ok"`
					}
					resume := map[string]any{"cli_id": record.CLIID, "connection": record.Connection, "action": "resume"}
					if err := s.runCLIChannel(r.Context(), "rpc", resume, &result); err != nil || !result.OK {
						http.Error(w, "会话恢复暂未完成，请重试", 503)
						return
					}
				}
				view.ConnectionWaiting = true
				jsonReply(w, view)
				return
			}
			http.Error(w, "原会话仍在电脑终端运行，请先退出其中的 Claude Code，再接续同一段历史", 409)
			return
		}
	}
	terminal, err := s.createTerminal(terminalRequest{Workspace: view.Workspace, Kind: "claude", sharedID: view.ID, requestContext: r.Context()})
	if err != nil {
		http.Error(w, err.Error(), 409)
		return
	}
	view.DesktopTerminalID = terminal.id
	if input.Automatic {
		view, err = s.terminalConversationView(r.Context(), view.ID)
		if err != nil {
			http.Error(w, err.Error(), 503)
			return
		}
	}
	jsonReply(w, view)
}

func (s *Server) cliSessionList(w http.ResponseWriter, r *http.Request) {
	paths, err := s.projectPaths()
	if err != nil {
		http.Error(w, "无法读取已授权的项目", 500)
		return
	}
	granted := map[string]bool{}
	for _, path := range paths {
		granted[path] = true
	}
	archived := r.URL.Query().Get("archived") == "true"
	sessions, err := s.cliSessions.summaries(s.profile, granted, archived)
	if err != nil {
		http.Error(w, "无法读取主机上的 CLI 会话记录", 500)
		return
	}
	var notice string
	if err := s.discoverTerminalConversations(r.Context(), paths); err != nil {
		notice = err.Error()
	} else {
		sessions, err = s.cliSessions.summaries(s.profile, granted, archived)
		if err != nil {
			http.Error(w, "无法读取主机上的 CLI 会话记录", 500)
			return
		}
	}
	s.mu.Lock()
	ready := s.ready && !s.closing
	s.mu.Unlock()
	if !ready || notice != "" {
		for i := range sessions {
			sessions[i].Active = false
		}
	}
	jsonReply(w, map[string]any{"sessions": sessions, "notice": notice})
}

func (s *Server) authorizedCLISession(w http.ResponseWriter, r *http.Request) (cliSession, bool) {
	view, terminal, ok := s.authorizedCLISessionRecord(w, r)
	if !ok {
		return view, false
	}
	if view.Archived {
		http.Error(w, "此会话已归档，请先在设置中的已归档会话撤销归档", http.StatusGone)
		return view, false
	}
	if terminal {
		var err error
		view, err = s.terminalConversationView(r.Context(), view.ID)
		if err != nil {
			http.Error(w, err.Error(), 503)
			return view, false
		}
	}
	return view, true
}

// Metadata reads do not require a reachable guest or a live terminal.
func (s *Server) authorizedCLISessionRecord(w http.ResponseWriter, r *http.Request) (cliSession, bool, bool) {
	id := r.PathValue("id")
	m := &s.cliSessions
	m.mu.Lock()
	record, err := m.record(s.profile, id)
	var view cliSession
	if err == nil {
		if record.Deleted {
			err = os.ErrNotExist
		}
		view = record.snapshot()
	}
	m.mu.Unlock()
	if errors.Is(err, os.ErrNotExist) {
		http.NotFound(w, r)
		return view, false, false
	}
	if err != nil {
		http.Error(w, "无法读取主机上的 CLI 会话记录", 500)
		return view, false, false
	}
	if _, err := s.projectWorkspace(view.Workspace); err != nil {
		http.Error(w, "项目未授权或已不可用", 403)
		return view, false, false
	}
	return view, record.Terminal, true
}

func (s *Server) cliSessionView(w http.ResponseWriter, r *http.Request) {
	view, ok := s.authorizedCLISession(w, r)
	if !ok {
		return
	}
	if value := r.URL.Query().Get("after"); value != "" {
		revision, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			http.Error(w, "会话版本无效", 400)
			return
		}
		epoch := r.URL.Query().Get("epoch")
		if epoch != "" && !cliSessionID(epoch) {
			http.Error(w, "服务标识无效", 400)
			return
		}
		if revision == view.Revision && (epoch == "" || epoch == view.Generation) {
			w.WriteHeader(http.StatusNoContent)
			return
		}
	}
	jsonReply(w, view)
}

package workbench

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sylvinus/agent-vm/internal/cliconversation"
	"github.com/sylvinus/agent-vm/internal/host"
)

type cliSessionSummary struct {
	AccountID     string    `json:"account_id,omitempty"`
	AccountLabel  string    `json:"account_label,omitempty"`
	Generation    string    `json:"generation"`
	ID            string    `json:"id"`
	Workspace     string    `json:"workspace"`
	Title         string    `json:"title"`
	CustomTitle   string    `json:"custom_title,omitempty"`
	Pinned        bool      `json:"pinned"`
	Active        bool      `json:"active"`
	Running       bool      `json:"running"`
	ClaudeDesktop bool      `json:"claude_desktop,omitempty"`
	MetadataSet   bool      `json:"metadata_set"`
	Deleted       bool      `json:"deleted,omitempty"`
	Archived      bool      `json:"archived,omitempty"`
	Purged        bool      `json:"purged,omitempty"`
	Deleting      bool      `json:"deleting,omitempty"`
	Revision      uint64    `json:"revision"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type cliSessionTurn struct {
	Tools       []cliconversation.ToolActivity `json:"tools,omitempty"`
	ID          string                         `json:"id"`
	Prompt      string                         `json:"prompt"`
	Text        string                         `json:"text"`
	Status      string                         `json:"status"`
	Error       string                         `json:"error,omitempty"`
	DeniedTools []string                       `json:"denied_tools,omitempty"`
	Truncated   bool                           `json:"truncated,omitempty"`
	Approval    *cliSessionApproval            `json:"approval,omitempty"`
}

// Approval is transcript data only; decisions remain in the desktop terminal.
type cliSessionApproval struct {
	ID      string          `json:"id"`
	Tool    string          `json:"tool"`
	Input   json.RawMessage `json:"input"`
	Preview bool            `json:"preview,omitempty"`
}

type cliSession struct {
	SwitchPolicy   string            `json:"switch_policy,omitempty"`
	AccountFailure string            `json:"account_failure,omitempty"`
	AccountSwitch  *claudeSwitchView `json:"account_switch,omitempty"`
	cliSessionSummary
	Turns             []cliSessionTurn `json:"turns"`
	CanSend           *bool            `json:"can_send,omitempty"`
	ConnectionNotice  string           `json:"connection_notice,omitempty"`
	ConnectionWaiting bool             `json:"connection_waiting,omitempty"`
	HistoryTruncated  bool             `json:"history_truncated,omitempty"`
	TerminalAvailable bool             `json:"terminal_available,omitempty"`
	DesktopTerminalID string           `json:"desktop_terminal_id,omitempty"`
}

type cliSessionRecord struct {
	cliSession
	CLIID            string               `json:"cli_id"`
	Launcher         string               `json:"launcher,omitempty"`
	Account          claudeSelection      `json:"account"`
	Resume           bool                 `json:"resume"`
	Terminal         bool                 `json:"terminal,omitempty"`
	GuestConfig      string               `json:"guest_config,omitempty"`
	GuestWorkspace   string               `json:"guest_workspace,omitempty"`
	Connection       string               `json:"connection,omitempty"`
	TerminalID       string               `json:"terminal_id,omitempty"`
	TerminalRunning  bool                 `json:"terminal_running,omitempty"`
	Switch           *claudeAccountSwitch `json:"switch,omitempty"`
	Executions       []claudeExecution    `json:"executions,omitempty"`
	ResumeTranscript string               `json:"resume_transcript,omitempty"`
}

type claudeExecution struct {
	CLIID   string          `json:"cli_id"`
	Account claudeSelection `json:"account"`
	Config  string          `json:"config"`
}

type claudeAccountSwitch struct {
	ID          string          `json:"id"`
	Phase       string          `json:"phase"`
	Source      claudeExecution `json:"source"`
	Target      claudeSelection `json:"target"`
	TargetID    string          `json:"target_cli_id"`
	TargetLabel string          `json:"target_label"`
	Connection  string          `json:"connection"`
	Error       string          `json:"error,omitempty"`
	Continue    bool            `json:"continue"`
}

type claudeSwitchView struct {
	ID          string `json:"id"`
	Phase       string `json:"phase"`
	TargetLabel string `json:"target_label"`
	Error       string `json:"error,omitempty"`
}

type cliSessionStore struct {
	mu         sync.Mutex
	catalog    map[string]cliSessionSummary
	running    map[string]bool // Fresh guest process observation, never restored from disk.
	desktop    map[string]bool // Claude Desktop ownership from the same live observation.
	generation string
}

func (m *cliSessionStore) epochLocked() string {
	if m.generation == "" {
		m.generation = uuid.NewString()
	}
	return m.generation
}

func cliSessionID(id string) bool {
	v, err := uuid.Parse(id)
	return err == nil && v.Version() == 4 && v.Variant() == uuid.RFC4122 && v.String() == id
}

func validCLIUUID(id string) bool {
	v, err := uuid.Parse(id)
	return err == nil && v.String() == id
}

func cliSessionRecordPath(profile, id string) string {
	return filepath.Join(profile, "cli-sessions", id+".json")
}

func (record *cliSessionRecord) changed() {
	record.Revision++
	record.UpdatedAt = time.Now().UTC()
}

func (record *cliSessionRecord) summary() cliSessionSummary {
	view := record.cliSessionSummary
	view.AccountID = record.Account.ID
	return view
}

func (record *cliSessionRecord) snapshot() cliSession {
	view := record.cliSession
	view.AccountID = record.Account.ID
	if sw := record.Switch; sw != nil {
		view.AccountSwitch = &claudeSwitchView{ID: sw.ID, Phase: sw.Phase, TargetLabel: sw.TargetLabel, Error: sw.Error}
	}
	view.TerminalAvailable = record.Terminal
	view.DesktopTerminalID = record.TerminalID
	if record.CustomTitle != "" {
		view.Title = record.CustomTitle
	}
	view.Turns = append([]cliSessionTurn{}, record.Turns...)
	return view
}

// Caller holds the store lock. The guest transcript owns task execution; loading
// this catalog never launches or replays a task.
func (m *cliSessionStore) record(profile, id string) (*cliSessionRecord, error) {
	if !cliSessionID(id) {
		return nil, os.ErrNotExist
	}
	path := cliSessionRecordPath(profile, id)
	if err := host.CheckPrivateFile(path); err != nil {
		return nil, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.Size() > 4<<20 {
		return nil, errors.New("CLI 会话记录过大")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var record cliSessionRecord
	if json.Unmarshal(b, &record) != nil || record.ID != id || !validCLIUUID(record.CLIID) || record.Workspace == "" || record.Turns == nil {
		return nil, errors.New("CLI 会话记录无效，请从备份恢复")
	}
	record.Generation = m.epochLocked()
	return &record, nil
}

func (m *cliSessionStore) summaries(profile string, granted map[string]bool, archived bool) ([]cliSessionSummary, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	list := []cliSessionSummary{}
	if m.catalog == nil {
		entries, err := os.ReadDir(filepath.Join(profile, "cli-sessions"))
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		catalog := make(map[string]cliSessionSummary, len(entries))
		for _, entry := range entries {
			id, ok := strings.CutSuffix(entry.Name(), ".json")
			if !ok || !cliSessionID(id) {
				continue
			}
			record, err := m.record(profile, id)
			if err != nil {
				return nil, err
			}
			catalog[id] = record.summary()
		}
		m.catalog = catalog
	}
	for id, summary := range m.catalog {
		summary.Running = m.running[id]
		summary.ClaudeDesktop = summary.Running && m.desktop[id]
		if granted[summary.Workspace] && !summary.Deleted && summary.Archived == archived {
			if summary.CustomTitle != "" {
				summary.Title = summary.CustomTitle
			}
			list = append(list, summary)
		}
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].Pinned != list[j].Pinned {
			return list[i].Pinned
		}
		return list[i].UpdatedAt.After(list[j].UpdatedAt)
	})
	return list, nil
}

func (m *cliSessionStore) create(profile, workspace, id string) (cliSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	record, err := m.record(profile, id)
	if err == nil {
		if record.Deleted || record.Archived {
			return cliSession{}, errors.New("此会话已归档或删除，请从已归档会话恢复或新建会话")
		}
		if record.Workspace != workspace {
			return cliSession{}, errors.New("创建请求已用于其他项目")
		}
		return record.snapshot(), nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return cliSession{}, err
	}
	if !cliSessionID(id) {
		return cliSession{}, errors.New("CLI 会话编号无效")
	}
	record = &cliSessionRecord{cliSession: cliSession{cliSessionSummary: cliSessionSummary{ID: id, Workspace: workspace, Title: "新会话"}, Turns: []cliSessionTurn{}}, CLIID: uuid.NewString(), Terminal: true}
	record.Generation = m.epochLocked()
	record.changed()
	err = writePrivate(cliSessionRecordPath(profile, id), record)
	if err == nil && m.catalog != nil {
		m.catalog[id] = record.summary()
	}
	return record.snapshot(), err
}

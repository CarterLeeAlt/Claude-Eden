// Package desktop owns the Windows presentation process. VM lifecycle and file
// transfer remain in the independently running eden controller.
package desktop

import "github.com/sylvinus/agent-vm/internal/desktopupdate"

type ConnectionRequest struct {
	ConnectionID string `json:"connection_id"`
}

type FileSelection struct {
	FileSelection string `json:"fileSelection,omitempty"`
	Count         int    `json:"count,omitempty"`
	Canceled      bool   `json:"canceled,omitempty"`
}

type ClipboardContent struct {
	Revision      uint32  `json:"revision"`
	Text          *string `json:"text,omitempty"`
	PNG           string  `json:"png,omitempty"`
	Files         string  `json:"files,omitempty"`
	FileSelection string  `json:"fileSelection,omitempty"`
	Count         int     `json:"count,omitempty"`
	Superseded    bool    `json:"superseded,omitempty"`
}

type ClipboardWriteRequest struct {
	ConnectionID string  `json:"connection_id"`
	Revision     *uint32 `json:"revision,omitempty"`
	Text         *string `json:"text,omitempty"`
	PNG          string  `json:"png,omitempty"`
	Files        string  `json:"files,omitempty"`
}

type ClipboardReleaseRequest struct {
	ConnectionID string `json:"connection_id"`
	Files        string `json:"files"`
}

type UploadRequest struct {
	ConnectionID string `json:"connection_id"`
	Target       string `json:"target"`
	ID           string `json:"id"`
	Selection    string `json:"selection"`
}

type FileActionRequest struct {
	ConnectionID string `json:"connection_id"`
	ID           string `json:"id"`
}

type DownloadRequest struct {
	ConnectionID string `json:"connection_id"`
	ID           string `json:"id"`
	Operation    string `json:"operation"`
	Name         string `json:"name"`
}

// FileResult is the controller's public file-operation result; host paths never
// serve as authority in subsequent frontend requests.
type FileResult struct {
	OK         bool     `json:"ok,omitempty"`
	Handled    bool     `json:"handled,omitempty"`
	Status     string   `json:"status,omitempty"`
	Message    string   `json:"message,omitempty"`
	Canceled   bool     `json:"canceled,omitempty"`
	Path       string   `json:"path,omitempty"`
	Name       string   `json:"name,omitempty"`
	Size       int64    `json:"size,omitempty"`
	Count      int      `json:"count,omitempty"`
	Paths      []string `json:"paths,omitempty"`
	ID         string   `json:"id,omitempty"`
	InstanceID string   `json:"instance_id,omitempty"`
	SavedAt    string   `json:"saved_at,omitempty"`
}

type ThemeExportRequest struct {
	Text     string `json:"text"`
	Filename string `json:"filename"`
}

type AppearanceRequest struct {
	Background string `json:"background"`
	Foreground string `json:"foreground"`
}

type SessionResult struct {
	URL string `json:"url"`
}
type DeviceStatus struct {
	Supported bool `json:"supported"`
}

type UpdateState = desktopupdate.State

type RemoteInputRequest struct {
	Owner       string  `json:"owner"`
	Operation   string  `json:"operation"`
	X           float64 `json:"x"`
	Y           float64 `json:"y"`
	Height      float64 `json:"height"`
	ReturnFocus bool    `json:"returnFocus"`
}

type RemoteInputResult struct {
	Accepted      bool `json:"accepted"`
	ReturnedFocus bool `json:"returnedFocus"`
}

type InputEvent struct {
	Owner            string `json:"owner"`
	Kind             string `json:"kind"`
	Text             string `json:"text,omitempty"`
	SelectionStart   int    `json:"selectionStart"`
	SelectionEnd     int    `json:"selectionEnd"`
	Key              string `json:"key,omitempty"`
	Code             string `json:"code,omitempty"`
	TextKey          bool   `json:"textKey"`
	Repeat           bool   `json:"repeat"`
	CtrlKey          bool   `json:"ctrlKey"`
	AltKey           bool   `json:"altKey"`
	ShiftKey         bool   `json:"shiftKey"`
	MetaKey          bool   `json:"metaKey"`
	ModifierCapsLock bool   `json:"modifierCapsLock"`
}

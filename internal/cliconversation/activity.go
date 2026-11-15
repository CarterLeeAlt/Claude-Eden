package cliconversation

// ToolActivity is a display-only summary. Command bodies, file contents and
// tool outputs stay in the CLI transcript; decisions remain in the terminal.
type ToolActivity struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Detail string `json:"detail,omitempty"`
	Status string `json:"status"`
}

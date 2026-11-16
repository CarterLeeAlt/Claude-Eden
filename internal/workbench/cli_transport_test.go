package workbench

import (
	"testing"
	"unicode/utf16"

	"github.com/google/uuid"
)

func TestCLIChannelCommandFitsWindows(t *testing.T) {
	// Leave room in CreateProcessW's 32767 UTF-16 limit for SSH paths and the
	// login-shell quoting around this command.
	command := cliChannelCommand("launch", `{"id":"`+uuid.NewString()+`","prefix":["claude"]}`)
	size := len(utf16.Encode([]rune(command)))
	t.Logf("embedded command: %d UTF-16 units", size)
	if size > 16000 {
		t.Fatalf("channel command uses %d UTF-16 units", size)
	}
}

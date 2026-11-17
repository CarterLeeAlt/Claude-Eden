package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/sylvinus/agent-vm/internal/diagnostics"
	"github.com/wailsapp/wails/v3/pkg/application"
)

const smokeDiagnosticsPrefix = "eden-smoke-diagnostics:"

func (s *Shell) runtimeReplied(request application.RuntimeRequest, err error) {
	if s.options.SmokeOutput == "" || err != nil || !s.ready.Load() || request.Object != 0 || request.Method != 0 || request.Args == nil {
		return
	}
	var call application.CallOptions
	if request.Args.ToStruct(&call) != nil {
		return
	}
	// This is the frozen generated WorkbenchReady ID, not an alternate binding
	// dispatcher. The smoke snapshot must be queued after its real response so
	// promise reactions cannot arrive behind a premature "clean page" snapshot.
	if call.MethodName == "" && call.MethodID == 1132524461 || call.MethodName == "github.com/sylvinus/agent-vm/internal/desktop.DesktopService.WorkbenchReady" {
		s.completeSmoke(true, "")
	}
}

type smokePageError struct {
	Type    string   `json:"type"`
	Source  string   `json:"source,omitempty"`
	Message string   `json:"message,omitempty"`
	Stack   []string `json:"stack,omitempty"`
}

// One collector spans all documents in a smoke run. Snapshots include the full
// bounded document buffer, so concurrent raw-message dispatch cannot lose an
// error merely because its earlier notification arrives after the final ack.
type smokeDiagnostics struct {
	mu       sync.Mutex
	seen     map[string]int
	errors   []smokePageError
	snapshot string
	done     chan struct{}
}

func (d *smokeDiagnostics) receive(message string, origin *application.OriginInfo, port int) []smokePageError {
	if !strings.HasPrefix(message, smokeDiagnosticsPrefix) || len(message) > 256<<10 || !trustedWorkbenchSource(origin, port) {
		return nil
	}
	var value struct {
		Document string           `json:"document"`
		Snapshot string           `json:"snapshot"`
		Errors   []smokePageError `json:"errors"`
	}
	if json.Unmarshal([]byte(strings.TrimPrefix(message, smokeDiagnosticsPrefix)), &value) != nil || validID(value.Document) != nil || len(value.Errors) > 8 {
		return nil
	}
	for _, item := range value.Errors {
		if item.Type != "error" && item.Type != "module-load" && item.Type != "unhandledrejection" {
			return nil
		}
		if utf8.RuneCountInString(item.Source) > 768 || utf8.RuneCountInString(item.Message) > 768 || len(item.Stack) > 6 {
			return nil
		}
		for _, line := range item.Stack {
			if utf8.RuneCountInString(line) > 768 {
				return nil
			}
		}
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.seen == nil {
		d.seen = make(map[string]int)
	}
	start := d.seen[value.Document]
	var fresh []smokePageError
	for index := start; index < len(value.Errors) && len(d.errors) < 8; index++ {
		item := value.Errors[index]
		item.Source = redactMessage(item.Source)
		if !strings.HasPrefix(item.Source, "/") || strings.HasPrefix(item.Source, "//") {
			item.Source = ""
		}
		item.Message = redactMessage(diagnostics.Redact(item.Message))
		for index, line := range item.Stack {
			item.Stack[index] = redactMessage(diagnostics.Redact(line))
		}
		d.errors = append(d.errors, item)
		fresh = append(fresh, item)
		d.seen[value.Document] = index + 1
	}
	if value.Snapshot == d.snapshot && d.done != nil {
		close(d.done)
		d.done = nil
	}
	return fresh
}

func (d *smokeDiagnostics) beginSnapshot() (string, <-chan struct{}) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.snapshot, d.done = uuid.NewString(), make(chan struct{})
	return d.snapshot, d.done
}

func (d *smokeDiagnostics) pageErrors() []smokePageError {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]smokePageError{}, d.errors...)
}

func (s *Shell) captureSmokePage(ctx context.Context) error {
	return s.capturePageDiagnostics(ctx, false)
}

func (s *Shell) capturePageDiagnostics(ctx context.Context, startup bool) error {
	if s.window == nil {
		return errors.New("无法收集工作台 JavaScript 错误：窗口尚未创建")
	}
	id, done := s.smokeDiagnostics.beginSnapshot()
	function := "__edenSmokeSnapshot"
	if startup {
		function = "__edenStartupSnapshot"
	}
	s.window.ExecJS(fmt.Sprintf("window.%s?.(%q)", function, id))
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("工作台 JavaScript 错误收集未得到页面确认：%w", ctx.Err())
	}
}

// Installed before any page script, not at WindowRuntimeReady. Normal startup
// uses the same trusted diagnostics as smoke, without exporting DOM or rejection
// objects. Timed snapshots are requested only by the explicit smoke entry.
const smokeDiagnosticsScript = `(() => {
  if (window !== window.top || location.protocol !== 'http:' || location.hostname !== 'localhost' || !window.chrome?.webview) return;
  if (window.__edenSmokeSnapshot) return;
  const errors = [], documentID = crypto.randomUUID();
  const ownPath = value => {
    try { const url = new URL(value, location.href); return url.origin === location.origin ? url.pathname.slice(0, 768) : ''; }
    catch { return ''; }
  };
  const scrub = value => {
    let text = typeof value === 'string' ? value : '';
    for (const input of document.querySelectorAll('input,textarea'))
      if (input.value) text = text.split(input.value).join('[form-value]');
    return text.replace(/https?:\/\/[^\s"'<>]+/g, value => ownPath(value) || '[external]')
      .replace(/[?#][^\s"'<>)]*/g, '[redacted]').slice(0, 768);
  };
  const stack = error => typeof error?.stack === 'string' ? error.stack.split('\n').slice(1)
    .filter(line => line.includes(location.origin + '/')).slice(0, 6).map(scrub) : [];
  const send = snapshot => chrome.webview.postMessage('eden-smoke-diagnostics:' + JSON.stringify({document: documentID, snapshot, errors}));
  const record = value => { if (errors.length < 8) { errors.push(value); send(''); } };
  addEventListener('error', event => {
    if (event.target instanceof HTMLScriptElement || event.target instanceof HTMLLinkElement) {
      record({type: 'module-load', source: scrub(ownPath(event.target.src || event.target.href))});
      return;
    }
    if (event.target && event.target !== window) return;
    record({type: 'error', source: scrub(ownPath(event.filename || location.href)), message: scrub(event.message), stack: stack(event.error)});
  }, true);
  addEventListener('unhandledrejection', event => {
    const frames = stack(event.reason);
    record({type: 'unhandledrejection', message: frames.length ? scrub(event.reason?.message) : 'Unhandled rejection', stack: frames});
  });
  Object.defineProperty(window, '__edenStartupSnapshot', {value: snapshot => send(snapshot)});
  Object.defineProperty(window, '__edenSmokeSnapshot', {value: snapshot => {
    // Two task turns include readiness-promise reactions and their unhandled
    // rejection event before the host tears down the backend intentionally.
    setTimeout(() => setTimeout(() => send(snapshot), 0), 0);
  }});
})();`

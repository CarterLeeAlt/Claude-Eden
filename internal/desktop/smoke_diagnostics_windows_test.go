package desktop

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v3/pkg/application"
)

func smokeDiagnosticMessage(t *testing.T, document, snapshot string, items []smokePageError) string {
	t.Helper()
	data, err := json.Marshal(map[string]any{"document": document, "snapshot": snapshot, "errors": items})
	if err != nil {
		t.Fatal(err)
	}
	return smokeDiagnosticsPrefix + string(data)
}

func smokeDiagnosticOrigin() *application.OriginInfo {
	return &application.OriginInfo{IsMainFrame: true, Origin: "http://localhost:7654/path#private", TopOrigin: "http://localhost:7654/"}
}

func TestSmokeDiagnosticsRejectsUntrustedAndMalformedMessages(t *testing.T) {
	message := smokeDiagnosticMessage(t, uuid.NewString(), "", []smokePageError{{Type: "error", Message: "fixture"}})
	for _, origin := range []*application.OriginInfo{nil, {}, {IsMainFrame: false, Origin: "http://localhost:7654/", TopOrigin: "http://localhost:7654/"}, {IsMainFrame: true, Origin: "http://localhost:7655/", TopOrigin: "http://localhost:7655/"}, {IsMainFrame: true, Origin: "http://localhost:7654/", TopOrigin: "http://external.example/"}} {
		var collector smokeDiagnostics
		collector.receive(message, origin, 7654)
		if len(collector.pageErrors()) != 0 {
			t.Fatal("untrusted source entered smoke diagnostics")
		}
	}
	for _, message := range []string{
		"other:" + message,
		smokeDiagnosticsPrefix + "broken",
		smokeDiagnosticsPrefix + strings.Repeat(" ", 256<<10),
		smokeDiagnosticMessage(t, "invalid document", "", []smokePageError{{Type: "error"}}),
		smokeDiagnosticMessage(t, uuid.NewString(), "", []smokePageError{{Type: "unknown"}}),
		smokeDiagnosticMessage(t, uuid.NewString(), "", make([]smokePageError, 9)),
		smokeDiagnosticMessage(t, uuid.NewString(), "", []smokePageError{{Type: "error", Message: strings.Repeat("x", 769)}}),
		smokeDiagnosticMessage(t, uuid.NewString(), "", []smokePageError{{Type: "error", Source: strings.Repeat("x", 769)}}),
		smokeDiagnosticMessage(t, uuid.NewString(), "", []smokePageError{{Type: "error", Stack: make([]string, 7)}}),
		smokeDiagnosticMessage(t, uuid.NewString(), "", []smokePageError{{Type: "error", Stack: []string{strings.Repeat("x", 769)}}}),
	} {
		var collector smokeDiagnostics
		collector.receive(message, smokeDiagnosticOrigin(), 7654)
		if len(collector.pageErrors()) != 0 {
			t.Fatal("malformed diagnostic entered smoke results")
		}
	}
}

func TestSmokeDiagnosticsSnapshotIncludesErrorsDespiteConcurrentDeliveryOrder(t *testing.T) {
	var collector smokeDiagnostics
	document := uuid.NewString()
	id, done := collector.beginSnapshot()
	collector.receive(smokeDiagnosticMessage(t, document, uuid.NewString(), nil), smokeDiagnosticOrigin(), 7654)
	select {
	case <-done:
		t.Fatal("stale snapshot acknowledged a different request")
	default:
	}
	errors := []smokePageError{{Type: "module-load", Source: "/assets/main.js?token=fixture"}, {Type: "unhandledrejection", Message: "fixture async failure"}}
	collector.receive(smokeDiagnosticMessage(t, document, id, errors), smokeDiagnosticOrigin(), 7654)
	select {
	case <-done:
	default:
		t.Fatal("complete snapshot did not acknowledge the request")
	}
	collector.receive(smokeDiagnosticMessage(t, document, "", errors[:1]), smokeDiagnosticOrigin(), 7654)
	collector.receive(smokeDiagnosticMessage(t, document, id, errors), smokeDiagnosticOrigin(), 7654)
	if len(collector.pageErrors()) != 2 {
		t.Fatal("late/duplicate notifications lost or duplicated errors")
	}
	for range 10 {
		collector.receive(smokeDiagnosticMessage(t, uuid.NewString(), "", errors), smokeDiagnosticOrigin(), 7654)
	}
	if len(collector.pageErrors()) != 8 || len(collector.seen) > 8 {
		t.Fatal("diagnostics exceeded the run-wide output budget")
	}
}

func TestSmokeDiagnosticsRedactsSecretsAndBoundsUnicodeWithoutBreakingUTF8(t *testing.T) {
	var collector smokeDiagnostics
	collector.receive(smokeDiagnosticMessage(t, uuid.NewString(), "", []smokePageError{{Type: "error", Source: "https://external.example/private?token=secret", Message: "token=private_value https://external.example/private " + strings.Repeat("中", 700), Stack: []string{"at fixture (http://localhost:7654/assets/main.js?token=private_value)"}}}), smokeDiagnosticOrigin(), 7654)
	items := collector.pageErrors()
	if len(items) != 1 || items[0].Source != "" {
		t.Fatal("diagnostic source was not constrained to a local path")
	}
	data, err := json.Marshal(items)
	if err != nil || strings.Contains(string(data), "private_value") || strings.Contains(string(data), "external.example") || !utf8.ValidString(items[0].Message) || len(items[0].Message) > 768 {
		t.Fatalf("diagnostic was not bounded and redacted: %s, %v", data, err)
	}
}

func TestSmokeReadinessDoesNotOverridePageErrorsOrCollectionFailure(t *testing.T) {
	for _, test := range []struct {
		name       string
		ready      bool
		errors     []string
		pageErrors []smokePageError
		success    bool
	}{
		{name: "ready with clean page", ready: true, success: true},
		{name: "page error after ready", ready: true, pageErrors: []smokePageError{{Type: "error", Message: "fixture"}}},
		{name: "missing collector ack", ready: true, errors: []string{"collector did not acknowledge"}},
		{name: "not ready"},
	} {
		t.Run(test.name, func(t *testing.T) {
			output := filepath.Join(t.TempDir(), "Smoke.json")
			shell := &Shell{options: Options{SmokeOutput: output}, smokeResult: &smokeResult{NativeBridgeReady: test.ready, Errors: test.errors, PageErrors: test.pageErrors}}
			if err := shell.finishSmoke(); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(output)
			if err != nil {
				t.Fatal(err)
			}
			var report smokeResult
			if err := json.Unmarshal(data, &report); err != nil || report.Success != test.success {
				t.Fatalf("incorrect smoke result: %s, %v", data, err)
			}
		})
	}
}

func TestDocumentCollectorProtectsBothProductionAndSmokeStartup(t *testing.T) {
	for _, smoke := range []bool{false, true} {
		shell := &Shell{}
		if smoke {
			shell.options.SmokeOutput = "fixture"
		}
		var options application.WebviewWindowOptions
		if err := shell.configureWebViewPolicy(&options, WebViewPolicy{}); err != nil {
			t.Fatal(err)
		}
		if options.Windows.WebViewPolicy.DocumentStartScript != smokeDiagnosticsScript+RuntimeTransportClient {
			t.Fatal("startup diagnostics and transport must be installed before frontend runtime import")
		}
	}
}

package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const runtimeMessagePrefix = "eden-runtime-v1:"

// nativeTransport retains Wails' generated bindings and dispatcher while using
// only WebView2 IPC. It exposes no runtime endpoint on the controller listener.
type nativeTransport struct {
	mu        sync.RWMutex
	processor *application.MessageProcessor
	ctx       context.Context
	cancel    context.CancelFunc
	port      func() int
	active    chan struct{}
	observe   func(application.RuntimeRequest)
	replied   func(application.RuntimeRequest, error)
}

func (t *nativeTransport) Start(ctx context.Context, processor *application.MessageProcessor) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ctx, t.cancel = context.WithCancel(ctx)
	t.processor = processor
	t.active = make(chan struct{}, 64)
	return nil
}
func (*nativeTransport) JSClient() []byte { return nil }
func (t *nativeTransport) Stop() error {
	t.mu.Lock()
	if t.cancel != nil {
		t.cancel()
	}
	t.processor = nil
	t.mu.Unlock()
	return nil
}

func validRuntimeMessage(message string, origin *application.OriginInfo, port int) bool {
	return strings.HasPrefix(message, runtimeMessagePrefix) && len(message) <= 30<<20 && trustedWorkbenchSource(origin, port)
}

func trustedWorkbenchSource(origin *application.OriginInfo, port int) bool {
	return origin != nil && origin.IsMainFrame && IsWorkbench(origin.Origin, port) && IsWorkbench(origin.TopOrigin, port)
}

func (t *nativeTransport) handle(window application.Window, message string, origin *application.OriginInfo) {
	if !validRuntimeMessage(message, origin, t.port()) {
		return
	}
	var envelope struct {
		ID      string                     `json:"id"`
		Request application.RuntimeRequest `json:"request"`
	}
	if json.Unmarshal([]byte(strings.TrimPrefix(message, runtimeMessagePrefix)), &envelope) != nil || validID(envelope.ID) != nil {
		return
	}
	if envelope.Request.Object != 0 && envelope.Request.Object != 10 {
		t.reply(window, envelope.ID, nil, errors.New("此原生运行时操作不允许"))
		return
	}
	if envelope.Request.Args == nil {
		t.reply(window, envelope.ID, nil, errors.New("原生调用缺少参数"))
		return
	}
	t.mu.RLock()
	processor, ctx, active := t.processor, t.ctx, t.active
	t.mu.RUnlock()
	if processor == nil {
		return
	}
	if envelope.Request.Object == 0 {
		select {
		case active <- struct{}{}:
		default:
			t.reply(window, envelope.ID, nil, errors.New("原生操作过多，请稍后重试"))
			return
		}
	}
	envelope.Request.WebviewWindowID = uint32(window.ID())
	envelope.Request.WebviewWindowName = ""
	envelope.Request.ClientID = ""
	if t.observe != nil {
		t.observe(envelope.Request)
	}
	go func() {
		if envelope.Request.Object == 0 {
			defer func() { <-active }()
		}
		result, err := processor.HandleRuntimeCallWithIDs(ctx, &envelope.Request)
		t.reply(window, envelope.ID, result, err)
		if t.replied != nil {
			t.replied(envelope.Request, err)
		}
	}()
}

func (*nativeTransport) reply(window application.Window, id string, result any, err error) {
	response := struct {
		ID     string                 `json:"id"`
		Result any                    `json:"result"`
		Error  *application.CallError `json:"error,omitempty"`
	}{ID: id, Result: result}
	if err != nil {
		var callError *application.CallError
		if errors.As(err, &callError) {
			response.Error = callError
		} else {
			response.Error = &application.CallError{Kind: application.RuntimeError, Message: err.Error()}
		}
	}
	data, marshalErr := json.Marshal(response)
	if marshalErr != nil {
		return
	}
	window.ExecJS("window.dispatchEvent(new CustomEvent('eden-runtime-result',{detail:" + string(data) + "}));")
}

// A bundled @wailsio/runtime instance must call setTransport with the same
// window.edenRuntimeTransport before invoking the first generated binding.
const RuntimeTransportClient = `(() => {
  if (window !== window.top || location.protocol !== 'http:' || location.hostname !== 'localhost') return;
  if (!window.chrome?.webview) return;
  if (!window.edenRuntimeTransport) {
    const pending = new Map();
    addEventListener('eden-runtime-result', event => {
      const value = event.detail, request = pending.get(value.id);
      if (!request) return;
      pending.delete(value.id);
      // The npm runtime already rejected the binding with CancelError. A second
      // rejection of its resolver would become an unhandled CancelledRejectionError.
      if (request.cancelled) { request.resolve({}); return; }
      if (value.error) {
        const error = value.error.kind === 'ReferenceError' ? new ReferenceError(value.error.message)
          : value.error.kind === 'TypeError' ? new TypeError(value.error.message) : new Error(value.error.message);
        error.name = value.error.kind || 'RuntimeError'; error.cause = value.error.cause; request.reject(error);
      }
      else request.resolve(value.result ?? {});
    });
    const transport = Object.freeze({call(object, method, windowName, args) {
      return new Promise((resolve, reject) => {
        const id = crypto.randomUUID();
        if (object === 10 && method === 0 && typeof args?.['call-id'] === 'string') {
          for (const request of pending.values())
            if (request.callID === args['call-id']) request.cancelled = true;
        }
        pending.set(id, {resolve, reject, callID: object === 0 ? args?.['call-id'] : undefined});
        try { chrome.webview.postMessage('eden-runtime-v1:' + JSON.stringify({id, request: {object, method, args}})); }
        catch (error) { pending.delete(id); reject(error); }
      });
    }});
    Object.defineProperty(window, 'edenRuntimeTransport', {value: transport});
  }
  window.wails?.setTransport?.(window.edenRuntimeTransport);
  dispatchEvent(new Event('eden-runtime-transport-ready'));
})();`

var _ application.Transport = (*nativeTransport)(nil)

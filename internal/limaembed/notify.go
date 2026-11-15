package limaembed

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/lima-vm/lima/v2/pkg/hostagent/events"
)

// eventWatch passes the hostagent's events (one JSON object a line) to w
// and offers a callback for forwarded-port events. Hosts have no native
// notification callback by default; the event stream remains intact.
type eventWatch struct {
	w      io.Writer
	vm     string
	notify func(title, msg string)

	mu      sync.Mutex
	partial []byte
}

func newEventWatch(w io.Writer, vm string) *eventWatch {
	return &eventWatch{w: w, vm: vm, notify: func(string, string) {}}
}

func (e *eventWatch) Write(p []byte) (int, error) {
	e.mu.Lock()
	e.partial = append(e.partial, p...)
	for {
		i := bytes.IndexByte(e.partial, '\n')
		if i < 0 {
			break
		}
		e.event(e.partial[:i])
		e.partial = e.partial[i+1:]
	}
	e.mu.Unlock()
	return e.w.Write(p)
}

// Preserve file flushing through the notification wrapper. Windows file
// watchers need it to observe newly appended hostagent status events.
func (e *eventWatch) Sync() error {
	if s, ok := e.w.(syncer); ok {
		return s.Sync()
	}
	return nil
}

func (e *eventWatch) event(line []byte) {
	var ev events.Event
	if json.Unmarshal(line, &ev) != nil || ev.Status.PortForward == nil {
		return
	}
	pf := ev.Status.PortForward
	// Ours (the forwarder builds it), but an address all the same.
	if _, _, err := net.SplitHostPort(pf.HostAddr); err != nil {
		return
	}
	switch pf.Type {
	case events.PortForwardEventForwarding:
		e.notify("agent-vm: "+e.vm, fmt.Sprintf("The VM now serves %s on this machine.", pf.HostAddr))
	case events.PortForwardEventFailed:
		e.notify("agent-vm: "+e.vm, fmt.Sprintf("Not forwarded: %s is in use on this machine.", pf.HostAddr))
	}
}

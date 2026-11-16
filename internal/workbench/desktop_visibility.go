package workbench

import (
	"context"
	_ "embed"
	"strconv"
	"sync"
	"time"

	"github.com/sylvinus/agent-vm/internal/diagnostics"
)

//go:embed linux-desktop-visibility.py
var linuxDesktopVisibility string

// A controller owns the local desktop viewers' display demand.
// Serialize guest changes so a late hide completes before a new viewer resumes.
type displayViewDemand struct {
	mu               sync.Mutex
	op               sync.Mutex
	viewers          int
	connections      int
	knownConnections int
	idle             *time.Timer
	known, shown     bool // guarded by op
}

func (d *displayViewDemand) apply(update func(bool) error) error {
	d.op.Lock()
	defer d.op.Unlock()
	d.mu.Lock()
	show, connections := d.viewers > 0, d.connections
	d.mu.Unlock()
	if d.known && d.shown == show && d.knownConnections == connections {
		return nil
	}
	if err := update(show); err != nil {
		d.known = false
		return err
	}
	d.known, d.shown = true, show
	d.knownConnections = connections
	return nil
}

func (d *displayViewDemand) background(update func(bool) error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.viewers != 0 {
		return
	}
	if d.idle != nil {
		d.idle.Stop()
	}
	d.idle = time.AfterFunc(500*time.Millisecond, func() { _ = d.apply(update) })
}

func (d *displayViewDemand) acquire(update func(bool) error) (func(), error) {
	d.mu.Lock()
	d.viewers++
	if d.idle != nil {
		d.idle.Stop()
		d.idle = nil
	}
	d.mu.Unlock()
	var once sync.Once
	release := func() { once.Do(func() { d.mu.Lock(); d.viewers--; d.mu.Unlock(); d.background(update) }) }
	if err := d.apply(update); err != nil {
		release()
		return nil, err
	}
	return release, nil
}

func (d *displayViewDemand) reset() {
	d.op.Lock()
	d.known = false
	d.op.Unlock()
}

// A retained transport can stop requesting frames and release its display
// demand independently. Each connection owns exactly one optional viewer.
type displayConnection struct {
	demand  *displayViewDemand
	update  func(bool) error
	release func()
}

func (d *displayViewDemand) connect(update func(bool) error) (*displayConnection, error) {
	d.mu.Lock()
	d.connections++
	d.mu.Unlock()
	c := &displayConnection{demand: d, update: update}
	if err := c.setVisible(true); err != nil {
		c.close()
		return nil, err
	}
	return c, nil
}

func (c *displayConnection) setVisible(show bool) error {
	if show && c.release == nil {
		var err error
		c.release, err = c.demand.acquire(c.update)
		return err
	}
	if !show && c.release != nil {
		c.release()
		c.release = nil
	}
	return nil
}

func (c *displayConnection) close() {
	c.demand.mu.Lock()
	c.demand.connections--
	c.demand.mu.Unlock()
	_ = c.setVisible(false)
	c.demand.background(c.update)
}

func (s *Server) updateDesktopVisibility(show bool) error {
	// Linux uses a dedicated virtual X display.
	if !s.vmRunning() {
		return nil
	}
	ctx, cancel := context.WithTimeout(s.lifetimeContext(), 5*time.Second)
	defer cancel()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	script := "/usr/bin/python3 - " + strconv.FormatBool(show) + " <<'WORKBENCH_DESKTOP_VISIBILITY'\n" + linuxDesktopVisibility + "\nWORKBENCH_DESKTOP_VISIBILITY\n"
	_, err := s.guestQuick(ctx, script)
	if err != nil && ctx.Err() == nil {
		_ = diagnostics.New(s.profile).Record("warn", "desktop-visibility", "应用窗口显示状态未能同步："+err.Error())
	}
	return err
}

func (s *Server) changeDesktopVisibility(show bool) error {
	if s.desktopDisplayUpdate != nil {
		return s.desktopDisplayUpdate(show)
	}
	return s.updateDesktopVisibility(show)
}

func (s *Server) changeBrowserVisibility(show bool) error {
	s.mu.Lock()
	ready, token := s.ready, s.credentials.Token
	s.mu.Unlock()
	if !ready {
		return nil
	}
	ctx, cancel := context.WithTimeout(s.lifetimeContext(), 5*time.Second)
	defer cancel()
	s.browserViews.mu.Lock()
	connections := s.browserViews.connections
	s.browserViews.mu.Unlock()
	_, err := s.guestAPI(ctx, token, "POST", "/visibility", []byte(`{"visible":`+strconv.FormatBool(show)+`,"managed_connections":`+strconv.Itoa(connections)+`}`))
	return err
}

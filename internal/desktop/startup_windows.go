package desktop

import (
	"errors"
	"fmt"

	"github.com/sylvinus/agent-vm/internal/diagnostics"
)

const (
	startupPending uint32 = iota
	startupReady
	startupFailed
)

// Raw messages and binding calls are dispatched concurrently by Wails. The
// current document's complete error buffer is acknowledged before accepting
// readiness, so an earlier JS error cannot lose a race to WorkbenchReady.
// This handshake has no production deadline; Quit cancels the shell lifetime.
func (s *Shell) confirmWorkbenchReady() error {
	s.pageReadyMu.Lock()
	defer s.pageReadyMu.Unlock()
	if s.window != nil && s.startupResult.Load() == startupPending {
		if err := s.capturePageDiagnostics(s.lifetime, true); err != nil {
			return err
		}
	}
	return s.markStartupReady()
}

// Startup confirmation is one terminal result, separate from the window's
// current ready bit. A later reconnect may restore the UI without replacing an
// update candidate's already accepted failure.
func (s *Shell) markStartupReady() error {
	s.startupMu.Lock()
	defer s.startupMu.Unlock()
	if s.startupResult.Load() != startupPending {
		return nil
	}
	if failures := s.smokeDiagnostics.pageErrors(); len(failures) != 0 {
		cause := errors.New(startupPageMessage(failures[0]))
		return errors.Join(cause, s.failStartupLocked(cause))
	}
	if s.options.MarkReady != nil {
		if err := s.options.MarkReady(); err != nil {
			return errors.Join(err, s.failStartupLocked(fmt.Errorf("无法确认工作台启动成功：%w", err)))
		}
	}
	s.startupResult.Store(startupReady)
	return nil
}

func (s *Shell) failStartup(cause error) (bool, error) {
	if s.startupResult.Load() != startupPending {
		return false, nil
	}
	s.startupMu.Lock()
	defer s.startupMu.Unlock()
	if s.startupResult.Load() != startupPending {
		return false, nil
	}
	return true, s.failStartupLocked(cause)
}

// The caller holds startupMu. Publish failure before the updater callback so
// its safe-exit path may re-enter Quit without recursively taking that mutex.
func (s *Shell) failStartupLocked(cause error) error {
	s.startupResult.Store(startupFailed)
	if s.options.StartupFailed == nil {
		return nil
	}
	return s.options.StartupFailed(cause)
}

func (s *Shell) reportStartupFailed(cause error) error {
	_, err := s.failStartup(cause)
	return err
}

func (s *Shell) startupError(message string) {
	if err := s.reportStartupFailed(errors.New(message)); err != nil {
		message += "\n无法报告启动失败：" + err.Error()
	}
	s.errorStatus(message)
	s.completeSmoke(false, message)
}

func (s *Shell) pageStartupError(failure smokePageError) {
	_, _ = s.failWorkbench(startupPageMessage(failure))
}

func startupPageMessage(failure smokePageError) string {
	message := fmt.Sprintf("工作台启动时发生页面错误（%s）", failure.Type)
	if failure.Source != "" {
		message += "：" + failure.Source
	}
	if failure.Message != "" {
		message += "：" + failure.Message
	}
	return message
}

func redactStartupMessage(message string) string {
	return redactMessage(diagnostics.Redact(message))
}

func (s *Shell) failWorkbench(message string) (bool, error) {
	accepted, err := s.failStartup(errors.New(message))
	if !accepted {
		return false, nil
	}
	if err != nil {
		message += "\n无法报告启动失败：" + err.Error()
	}
	s.errorStatus(message)
	s.completeSmoke(false, message)
	return true, err
}

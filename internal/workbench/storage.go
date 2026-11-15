package workbench

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/sylvinus/agent-vm/internal/diagnostics"
	"github.com/sylvinus/agent-vm/internal/eden"
)

type storageRecoveryView struct {
	Required bool   `json:"required"`
	Error    string `json:"error,omitempty"`
}

func (s *Server) storageRecoveryStatusLocked() storageRecoveryView {
	return storageRecoveryView{
		Required: s.storageRecoveryError != "", Error: s.storageRecoveryError,
	}
}

func (s *Server) storageRecoveryStatus() storageRecoveryView {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.storageRecoveryStatusLocked()
}

func (s *Server) requireStorageReady() error {
	state := s.storageRecoveryStatus()
	if state.Required {
		return fmt.Errorf("环境存储待恢复，请在设置中恢复后继续。%s", state.Error)
	}
	return nil
}

func (s *Server) failStorageRecovery(err error) error {
	s.mu.Lock()
	s.storageRecoveryError = err.Error()
	s.ready = false
	s.mu.Unlock()
	diagnostics.Log("error", "storage", "environment storage recovery failed: "+err.Error())
	return err
}

// Recovery keeps local account access and host diagnostics available. Guest
// reads can also create sessions, so admission names the host-only reads.
func storageRecoveryRoute(r *http.Request) bool {
	path := r.URL.Path
	if !strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/api/account/") || strings.HasPrefix(path, "/api/diagnostics") {
		return true
	}
	switch path {
	case "/api/account", "/api/session", "/api/launch", "/api/handoff", "/api/environment/storage/recover":
		return true
	}
	if r.Method == http.MethodGet {
		switch path {
		case "/api/status", "/api/settings", "/api/appearance", "/api/favorites", "/api/proxy", "/api/agent/routes", "/api/environment/storage", "/api/environment/initialization":
			return true
		}
	}
	return false
}

func (s *Server) recoverEnvironmentStorage(w http.ResponseWriter, r *http.Request) {
	if !s.storageRecoveryStatus().Required {
		http.Error(w, "当前环境存储无需恢复", http.StatusConflict)
		return
	}
	var input struct{}
	if !readJSON(w, r, &input) {
		return
	}
	s.launch(w, "storage-recovery", func() error {
		s.setMessage("正在恢复环境存储并检查数据位置…")
		if _, err := eden.GetStorageLocation(s.profile); err != nil {
			return s.failStorageRecovery(err)
		}
		if err := s.prepareEnvironment(); err != nil {
			return s.failStorageRecovery(err)
		}
		s.mu.Lock()
		s.storageRecoveryError = ""
		s.mu.Unlock()
		s.setMessage("环境存储已恢复，可以启动环境")
		return nil
	})
}

func (s *Server) environmentStorage(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		if recovery := s.storageRecoveryStatus(); recovery.Required {
			jsonReply(w, map[string]any{"recovery": recovery})
			return
		}
		location, err := eden.GetStorageLocation(s.profile)
		if err != nil {
			s.failStorageRecovery(err)
			jsonReply(w, map[string]any{"recovery": s.storageRecoveryStatus()})
			return
		}
		suggested := ""
		if !location.Custom && !location.HasEnvironment {
			initialization, err := s.initializationState()
			if err != nil {
				requestError(w, err)
				return
			}
			if initialization.Required && !initialization.Started && !initialization.Prepared {
				suggested = eden.InitialStorageDirectory(s.exe)
			}
		}
		jsonReply(w, struct {
			eden.StorageLocation
			SuggestedDirectory string `json:"suggested_directory,omitempty"`
		}{location, suggested})
		return
	}

	var input struct {
		Directory string `json:"directory"`
		Previous  string `json:"previous"`
		Migrate   bool   `json:"migrate"`
	}
	if !readJSON(w, r, &input) {
		return
	}
	s.launch(w, "storage", func() error {
		current, err := eden.GetStorageLocation(s.profile)
		if err != nil {
			return err
		}
		if current.Directory != input.Previous || current.HasEnvironment != input.Migrate {
			return errors.New("环境或存储位置已改变，请重新打开设置后再选择")
		}
		if err = eden.ValidateStorageTarget(s.profile, input.Directory); err != nil {
			if errors.Is(err, os.ErrPermission) {
				return errors.New("无法读取或写入所选存储目录，请在当前页面选择有权限的本地文件夹后重试，已有数据已保留")
			}
			return err
		}
		if err = s.validateStoragePlacement(input.Directory); err != nil {
			return err
		}
		if current.HasEnvironment {
			if err = s.stopEnvironment(); err != nil {
				return err
			}
		}
		s.setMessage("正在检查环境存储…")
		_, err = eden.MoveStorage(s.operationContext(), s.profile, input.Directory, current, func(copied, total int64) {
			s.setMessage(fmt.Sprintf("正在迁移环境数据 · %.1f / %.1f GiB", float64(copied)/(1<<30), float64(total)/(1<<30)))
		})
		if err != nil {
			if _, recoveryErr := eden.GetStorageLocation(s.profile); recoveryErr != nil {
				return s.failStorageRecovery(err)
			}
		}
		if errors.Is(err, os.ErrPermission) {
			return errors.New("无法写入所选存储目录。请在当前页面选择有写入权限的本地文件夹后重试，已有数据已保留")
		}
		return err
	})
}

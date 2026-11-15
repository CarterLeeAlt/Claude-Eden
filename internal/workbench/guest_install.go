package workbench

import (
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"runtime"

	"github.com/google/uuid"
	"github.com/sylvinus/agent-vm/internal/eden"
	"github.com/sylvinus/agent-vm/internal/guestos"
	"github.com/sylvinus/agent-vm/internal/host"
	"github.com/sylvinus/agent-vm/internal/hostimage"
	"github.com/sylvinus/agent-vm/internal/hostproxy"
)

type guestInstallRequest struct {
	System string `json:"guest_system"`
}

type guestInstallView struct {
	LinuxSystems []string              `json:"linux_systems,omitempty"`
	Fresh        bool                  `json:"fresh"`
	Pending      *guestos.Installation `json:"pending,omitempty"`
}

func (s *Server) guestInstallView() (guestInstallView, error) {
	v := guestInstallView{}
	pending, err := guestos.LoadInstallation(s.profile)
	if err != nil {
		return v, err
	}
	loc, err := eden.GetStorageLocation(s.profile)
	if err != nil {
		return v, err
	}
	v.Fresh = !loc.HasEnvironment
	v.Pending = pending
	if !v.Fresh && pending == nil {
		return v, nil
	}
	v.LinuxSystems = linuxGuestSystems(runtime.GOOS, runtime.GOARCH)
	if pending != nil {
		if !canInstallCatalogLinux(pending.System) || pending.Source.Host.Arch != runtime.GOARCH {
			return v, errors.New("客户机镜像架构与这台主机不匹配，请恢复原主机环境")
		}
	}
	return v, nil
}

func (s *Server) guestInstallation(w http.ResponseWriter, r *http.Request) {
	if !s.hasTrustedView(r) {
		http.Error(w, "请在本机 Eden 窗口选择客户机系统", http.StatusForbidden)
		return
	}
	v, err := s.guestInstallView()
	if err != nil {
		requestError(w, err)
		return
	}
	if r.Method == http.MethodGet {
		jsonReply(w, v)
		return
	}
	http.Error(w, "不支持此安装操作", http.StatusMethodNotAllowed)
}

// Installation failures retain the original progress and point to diagnostics.
func guestInstallFailureMessage(err error) error {
	next := "请在初始化页打开“操作详情”查看失败记录后点击「继续安装」；仍失败时在“日志与诊断”导出诊断包排查。"
	return fmt.Errorf("客户机安装未完成，已保留安装进度和文件。%s失败原因：%w", next, err)
}

func (s *Server) beginGuestInstall(w http.ResponseWriter, r *http.Request, in guestInstallRequest) {
	v, err := s.guestInstallView()
	if err != nil {
		requestError(w, err)
		return
	}
	if !canInstallCatalogLinux(in.System) || !s.hasTrustedView(r) || (!v.Fresh && v.Pending == nil) {
		http.Error(w, "当前环境无法安装所选客户机系统", http.StatusConflict)
		return
	}
	install := guestos.Installation{Version: 1, System: in.System, ID: uuid.New()}
	if v.Pending != nil {
		if v.Pending.System != in.System {
			http.Error(w, "必须使用原先选择的系统和安装介质继续安装", http.StatusConflict)
			return
		}
		install = *v.Pending
	} else {
		plan, err := linuxGuestImagePlan(r.Context(), s.profile, in.System, runtime.GOARCH)
		if err != nil {
			requestError(w, err)
			return
		}
		install.Source = plan
	}
	if err := install.Validate(); err != nil {
		requestError(w, err)
		return
	}
	s.launch(w, "guest-install", func() error {
		s.jobs.execution().setSetupProgress(&SetupProgress{Phase: "image", System: install.System, Label: "正在准备 " + guestProgressName(install.System) + " 安装材料"})
		cfg, err := loadConfig(s.configPath)
		if err != nil {
			return err
		}
		if err := cfg.RequireManagedProxy(s.profile); err != nil {
			return err
		}
		lease, loc, err := eden.AcquireStorageUse(s.profile)
		if err != nil {
			return err
		}
		defer lease.Close()
		current, err := guestos.LoadInstallation(s.profile)
		if err != nil {
			return err
		}
		if current == nil && loc.HasEnvironment || current != nil && *current != install {
			return errors.New("已有其他环境或安装任务，请重新打开安装页面")
		}
		if err := guestos.SaveInstallation(s.profile, install); err != nil {
			return err
		}
		if install.Source.Host.System != "" {
			cache := filepath.Join(s.profile, "state", "guest-media", install.System)
			unstarted, err := guestos.UnstartedInstallation(s.profile, install)
			if err != nil {
				return err
			}
			minimumGiB := 16
			if !unstarted {
				minimumGiB = 0
			}
			minimumGiB += int((install.Source.RemainingBytes(cache) + (1 << 30) - 1) / (1 << 30))
			if available, ok := host.FreeGiB(cache); ok && available < minimumGiB {
				return fmt.Errorf("创建所选系统至少需要 %d GiB 可用空间，当前有 %d GiB，请在存储设置中选择空间充足的位置", minimumGiB, available)
			}
			s.setMessage("正在准备 " + guestProgressName(install.System) + " 官方系统镜像…")
			transport := hostproxy.NewTransport()
			defer transport.CloseIdleConnections()
			prepared, prepareErr := hostimage.Prepare(s.operationContext(), cache, install.Source, s.jobs.execution(), transport)
			if prepareErr != nil {
				return prepareErr
			}
			install.Source, install.Media = prepared, prepared.Path
			if err := guestos.RecordPreparedMedia(s.profile, install.ID, prepared); err != nil {
				return err
			}
		}
		if err := s.prepareEnvironment(); err != nil {
			return err
		}
		if err := hostimage.WriteLinuxTemplate(filepath.Join(s.profile, "state"), install.Source); err != nil {
			return err
		}
		if err := s.startEnvironment(true); err != nil {
			return guestInstallFailureMessage(err)
		}
		return guestos.FinishLinuxInstallation(s.profile, install.ID)
	})
}

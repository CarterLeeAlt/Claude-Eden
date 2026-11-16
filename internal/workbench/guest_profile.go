package workbench

import (
	"errors"
	"net/http"
	"strings"

	"github.com/sylvinus/agent-vm/internal/eden"
	"github.com/sylvinus/agent-vm/internal/guestprofile"
)

func (s *Server) linuxTemplate(script string) (string, error) {
	p, err := guestprofile.Load(s.profile)
	if err != nil {
		return "", err
	}
	if p == nil {
		return script, nil
	}
	return p.Linux(script), nil
}

func (s *Server) linuxPathHasPrefix(path, prefix string) bool {
	rendered, err := s.linuxTemplate(prefix)
	return err == nil && strings.HasPrefix(path, rendered)
}

func (s *Server) guestProfileOptions(w http.ResponseWriter, r *http.Request) {
	if !s.hasTrustedView(r) {
		http.Error(w, "请在本机 Claude Eden 窗口配置客机", http.StatusForbidden)
		return
	}
	if r.Method == http.MethodPost {
		var options guestprofile.Options
		if !readJSON(w, r, &options) {
			return
		}
		p, err := guestprofile.New(options)
		if err != nil {
			requestError(w, err)
			return
		}
		jsonReply(w, p.Options)
		return
	}
	p, err := guestprofile.Load(s.profile)
	if err != nil {
		requestError(w, err)
		return
	}
	location, err := eden.GetStorageLocation(s.profile)
	if err != nil {
		requestError(w, err)
		return
	}
	jsonReply(w, struct {
		Profile     *guestprofile.Profile     `json:"profile"`
		Defaults    guestprofile.Options      `json:"defaults"`
		Editable    bool                      `json:"editable"`
		AudioModels []guestprofile.AudioModel `json:"audio_models"`
	}{p, guestprofile.Defaults(), p == nil && !location.HasEnvironment && !s.busy(), guestprofile.AudioModels()})
}

// Called under the installation operation lock before any VM disk is created.
func (s *Server) prepareGuestProfile(options *guestprofile.Options) error {
	if options != nil {
		if err := options.Validate(); err != nil {
			return err
		}
	}
	p, err := guestprofile.Load(s.profile)
	if err != nil {
		return err
	}
	if p != nil {
		if options != nil && !options.Matches(p.Options) {
			return errors.New("创建配置已保存，请继续原安装；已有账户和磁盘不会被改名")
		}
		return nil
	}
	location, err := eden.GetStorageLocation(s.profile)
	if err != nil {
		return err
	}
	if location.HasEnvironment {
		if options != nil {
			return errors.New("已有环境不能直接更换创建配置，请创建独立环境")
		}
		return nil
	}
	o := guestprofile.Options{}
	if options != nil {
		o = *options
	}
	p, err = guestprofile.Create(s.profile, o)
	if err == nil && !o.Matches(p.Options) {
		return errors.New("创建配置已由其他安装任务保存，请重新打开安装页面")
	}
	return err
}

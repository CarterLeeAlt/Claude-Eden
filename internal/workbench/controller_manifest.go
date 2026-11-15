package workbench

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"

	"github.com/sylvinus/agent-vm/internal/host"
	"github.com/sylvinus/agent-vm/internal/version"
)

const controllerAPIProtocol = 1

var controllerInstanceID = regexp.MustCompile(`^[a-f0-9]{64}$`)

type controllerInstance struct {
	ID string `json:"id"`
}

// The identity belongs to persistent controller data, independently of the
// filesystem path, process ID or current listening port.
func ensureControllerInstance(profile string) (string, error) {
	path := filepath.Join(profile, "controller-instance.json")
	if err := host.CheckPrivateFile(path); err == nil {
		data, err := os.ReadFile(path)
		var instance controllerInstance
		if err != nil || json.Unmarshal(data, &instance) != nil || !controllerInstanceID.MatchString(instance.ID) {
			return "", errors.New("本机实例身份记录无效，请保留数据并检查配置")
		}
		return instance.ID, nil
	} else if !os.IsNotExist(err) {
		return "", err
	}
	instance := controllerInstance{ID: randomToken()}
	if err := writePrivate(path, instance); err != nil {
		return "", err
	}
	return instance.ID, nil
}

type ControllerManifest struct {
	APIProtocol  int             `json:"api_protocol"`
	InstanceID   string          `json:"instance_id"`
	Version      string          `json:"version"`
	Platform     string          `json:"platform"`
	Runtime      string          `json:"runtime"`
	Origin       string          `json:"origin"`
	Capabilities map[string]bool `json:"capabilities"`
}

func (s *Server) controllerManifest(w http.ResponseWriter, r *http.Request) {
	jsonReply(w, ControllerManifest{APIProtocol: controllerAPIProtocol, InstanceID: s.instanceID,
		Version: version.Version, Platform: runtime.GOOS, Runtime: "lima", Origin: s.origin,
		Capabilities: map[string]bool{"projects_browse": true, "terminal": true, "browser": true, "desktop": true, "audio": true}})
}

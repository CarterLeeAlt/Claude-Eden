package workbench

import (
	"context"
	"errors"
	"path/filepath"
	"runtime"

	"github.com/sylvinus/agent-vm/internal/eden"
	"github.com/sylvinus/agent-vm/internal/hostimage"
)

func linuxGuestSystems(platform, arch string) []string {
	if platform == "windows" && arch == "amd64" {
		return []string{"debian-13"}
	}
	return nil
}

func linuxGuestImagePlan(ctx context.Context, profile, system, arch string) (hostimage.Plan, error) {
	if system != "debian-13" || arch != "amd64" {
		return hostimage.Plan{}, errors.New("此主机暂不支持所选 Linux 客机镜像")
	}
	template, err := hostimage.ReadImageTemplate(system)
	if err != nil {
		return hostimage.Plan{}, err
	}
	identity := hostimage.Identity{System: system, Version: "13", Arch: arch}
	// Resolution of a pinned Linux image is local. Downloads use the existing
	// installer transport only after the user submits the installation.
	return hostimage.Resolve(identity, filepath.Join(profile, "state", "guest-media", system), template)
}

func (s *Server) installedLinuxSystem() (string, error) {
	// A template or downloaded image alone does not bind a VM. Use the same
	// existence check as the system picker and storage selection endpoints.
	location, err := eden.GetStorageLocation(s.profile)
	if err != nil || !location.HasEnvironment {
		return "", err
	}
	return s.startupLinuxSystem()
}

// startupLinuxSystem includes the Debian default before any VM exists.
func (s *Server) startupLinuxSystem() (string, error) {
	return hostimage.LinuxSystem(filepath.Join(s.profile, "state"))
}

func canInstallCatalogLinux(system string) bool {
	for _, candidate := range linuxGuestSystems(runtime.GOOS, runtime.GOARCH) {
		if system == candidate {
			return true
		}
	}
	return false
}

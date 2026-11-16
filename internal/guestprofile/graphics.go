package guestprofile

import (
	"errors"
	"regexp"
	"strings"
)

var homePattern = regexp.MustCompile(`^/home/[A-Za-z][A-Za-z0-9_.-]{0,62}$`)

func validateHardwareOptions(o Options) error {
	for _, home := range []string{o.WorkHome, o.BrowserHome} {
		if home != "" && !homePattern.MatchString(home) {
			return errors.New("主目录须为 /home/ 下独立的单层目录，仅含字母、数字、点、连字符或下划线")
		}
		if home == "/home/debian" {
			return errors.New("主目录不能使用基础镜像的保留账户目录")
		}
	}
	if o.WorkHome != "" && o.WorkHome == o.BrowserHome {
		return errors.New("两个账户不能共用主目录")
	}
	switch o.GraphicsBackend {
	case "", "auto", "vulkan", "opengl", "software":
	default:
		return errors.New("Debian 客机图形后端须为自动、Vulkan、OpenGL 或软件渲染")
	}
	if o.WebGL == "disabled" && o.GraphicsBackend != "" && o.GraphicsBackend != "auto" {
		return errors.New("关闭 WebGL 时不能要求指定 3D 渲染后端")
	}
	if o.AudioModel != "" {
		model, ok := findAudioModel(o.AudioModel)
		if !ok {
			return errors.New("音频型号不存在")
		}
		if o.AudioDescription != "" && o.AudioDescription != model.Label {
			return errors.New("音频描述须与已选型号一致")
		}
		if o.AudioName != "" && !strings.HasPrefix(o.AudioName, "alsa_output.usb-"+model.Stem+"_") {
			return errors.New("音频设备名与已选型号不一致")
		}
	}
	return nil
}

func (p Profile) WorkUser() string {
	if p.Version == 0 {
		return "developer"
	}
	return p.Username
}
func (p Profile) BrowserUser() string {
	if p.Version == 0 {
		return "browser"
	}
	return p.BrowserUsername
}
func (p Profile) WorkHomePath() string {
	if p.WorkHome != "" {
		return p.WorkHome
	}
	return "/home/" + p.WorkUser()
}
func (p Profile) BrowserHomePath() string {
	if p.BrowserHome != "" {
		return p.BrowserHome
	}
	return "/home/" + p.BrowserUser()
}

// These flags select Chromium's actual renderer, never override reported names.
func (p Profile) GraphicsFlags() []string {
	switch p.GraphicsBackend {
	case "vulkan":
		return []string{"--use-gl=angle", "--use-angle=vulkan"}
	case "opengl":
		return []string{"--use-gl=angle", "--use-angle=gl"}
	case "software":
		return []string{"--use-gl=angle", "--use-angle=swiftshader"}
	}
	return nil
}

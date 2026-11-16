// Package guestprofile stores public, creation-time guest preferences. It never
// copies host hardware identifiers and none of its values are credentials.
package guestprofile

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/sylvinus/agent-vm/internal/host"
	"workbench.local/guestidentity"
)

const Filename = "guest-profile.json"

// Mode selects fixed defaults or explicit random generation. Manual values are
// validated before filling unspecified fields; installation saves the result.
type Options struct {
	Mode             string `json:"mode,omitempty"`
	Hostname         string `json:"hostname"`
	Username         string `json:"username"`
	BrowserUsername  string `json:"browser_username"`
	WorkHome         string `json:"work_home,omitempty"`
	BrowserHome      string `json:"browser_home,omitempty"`
	GraphicsBackend  string `json:"graphics_backend,omitempty"`
	AudioModel       string `json:"audio_model,omitempty"`
	AudioName        string `json:"audio_name"`
	AudioDescription string `json:"audio_description"`
	DisplayWidth     int    `json:"display_width"`
	DisplayHeight    int    `json:"display_height"`
	BrowserWidth     int    `json:"browser_width"`
	BrowserHeight    int    `json:"browser_height"`
	ExtensionName    string `json:"extension_name"`
	WebGL            string `json:"webgl"`
	Canvas           string `json:"canvas"`
}

// Profile remains comparable so configuration equality compares saved values.
// The zero value represents an existing environment without a creation record.
type Profile struct {
	Version int `json:"version"`
	Options
	ExtensionSuffix string `json:"extension_suffix"`
}

func Defaults() Options {
	return Options{Mode: "default", Username: "developer", BrowserUsername: "browser",
		AudioName: "workbench_output", AudioDescription: "Workbench", DisplayWidth: 2880,
		DisplayHeight: 1800, BrowserWidth: 1440, BrowserHeight: 900,
		ExtensionName: "Workbench browser panes", WebGL: "native", Canvas: "native", GraphicsBackend: "auto"}
}

// Matches treats omitted inputs as requests to reuse the saved creation values.
func (o Options) Matches(saved Options) bool {
	for _, p := range [][2]string{{o.Mode, saved.Mode}, {o.Hostname, saved.Hostname}, {o.Username, saved.Username}, {o.BrowserUsername, saved.BrowserUsername}, {o.WorkHome, saved.WorkHome}, {o.BrowserHome, saved.BrowserHome}, {o.GraphicsBackend, saved.GraphicsBackend}, {o.AudioModel, saved.AudioModel}, {o.AudioName, saved.AudioName}, {o.AudioDescription, saved.AudioDescription}, {o.ExtensionName, saved.ExtensionName}, {o.WebGL, saved.WebGL}, {o.Canvas, saved.Canvas}} {
		if p[0] != "" && p[0] != p[1] {
			return false
		}
	}
	for _, p := range [][2]int{{o.DisplayWidth, saved.DisplayWidth}, {o.DisplayHeight, saved.DisplayHeight}, {o.BrowserWidth, saved.BrowserWidth}, {o.BrowserHeight, saved.BrowserHeight}} {
		if p[0] != 0 && p[0] != p[1] {
			return false
		}
	}
	return true
}

var hostname = regexp.MustCompile(`^[A-Za-z](?:[A-Za-z0-9-]{0,13}[A-Za-z0-9])?$`)
var username = regexp.MustCompile(`^[a-z][a-z0-9_]{0,23}$`)
var audioName = regexp.MustCompile(`^[a-z][A-Za-z0-9_.-]{0,119}$`)
var suffix = regexp.MustCompile(`^[a-f0-9]{16}$`)
var description = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9 .()_-]{0,63}$`)

func (o Options) Validate() error {
	if o.Mode != "" && o.Mode != "default" && o.Mode != "random" && o.Mode != "manual" {
		return errors.New("创建配置方式无效")
	}
	if o.Hostname != "" && !hostname.MatchString(o.Hostname) {
		return errors.New("主机名须为 1–15 位 ASCII 字母、数字或连字符，以字母开头，以字母或数字结尾")
	}
	for _, u := range []string{o.Username, o.BrowserUsername} {
		if u != "" && (!username.MatchString(u) || reserved(u)) {
			return errors.New("Linux 账户须为 1–24 位小写字母、数字或下划线，以字母开头，且不能使用系统账户")
		}
	}
	if o.Username != "" && o.Username == o.BrowserUsername {
		return errors.New("工作账户与浏览器账户必须不同")
	}
	if o.AudioName != "" && !audioName.MatchString(o.AudioName) {
		return errors.New("音频设备名须以小写字母开头，仅含 ASCII 字母、数字、点、连字符或下划线，最长 120 位")
	}
	for _, text := range []string{o.AudioDescription, o.ExtensionName} {
		if text != "" && !description.MatchString(text) {
			return errors.New("设备描述和扩展名称须为 1–64 位普通 ASCII 文字")
		}
	}
	for _, size := range [][2]int{{o.DisplayWidth, o.DisplayHeight}, {o.BrowserWidth, o.BrowserHeight}} {
		if size == [2]int{} {
			continue
		}
		if size[0] < 800 || size[0] > 3840 || size[1] < 600 || size[1] > 2160 {
			return errors.New("尺寸须同时填写宽高：宽 800–3840，高 600–2160")
		}
	}
	if o.DisplayWidth != 0 && o.BrowserWidth != 0 && (o.BrowserWidth > o.DisplayWidth || o.BrowserHeight > o.DisplayHeight) {
		return errors.New("浏览器初始窗口不能大于虚拟显示")
	}
	if o.WebGL != "" && o.WebGL != "native" && o.WebGL != "disabled" {
		return errors.New("WebGL 选项无效")
	}
	if o.Canvas != "" && o.Canvas != "native" && o.Canvas != "blocked" {
		return errors.New("Canvas 读取选项无效")
	}
	return validateHardwareOptions(o)
}

func reserved(u string) bool {
	for _, name := range strings.Fields("root daemon bin sys sync games man lp mail news uucp proxy www_data backup list irc gnats nobody systemd_network systemd_timesync messagebus sshd pulse polkitd debian eden workbench") {
		if name == u {
			return true
		}
	}
	return false
}

func choose(n int) (int, error) {
	v, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		return 0, err
	}
	return int(v.Int64()), nil
}

func randomText(n int) (string, error) {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		v, err := choose(len(chars))
		if err != nil {
			return "", err
		}
		b[i] = chars[v]
	}
	return string(b), nil
}

func New(o Options) (*Profile, error) {
	if err := o.Validate(); err != nil {
		return nil, err
	}
	if o.Mode == "" {
		o.Mode = "default"
	}
	if o.Hostname == "" {
		identity, err := guestidentity.New()
		if err != nil {
			return nil, err
		}
		o.Hostname = identity.Hostname
	}
	defaults := Defaults()
	if o.Mode == "random" {
		for o.Username == "" || o.Username == o.BrowserUsername {
			name, err := naturalUsername()
			if err != nil {
				return nil, err
			}
			o.Username = name
		}
		for o.BrowserUsername == "" || o.BrowserUsername == o.Username {
			name, err := naturalUsername()
			if err != nil {
				return nil, err
			}
			o.BrowserUsername = name
		}
		if err := chooseAudio(&o); err != nil {
			return nil, err
		}
		if o.DisplayWidth == 0 {
			var fitting [][2]int
			for _, size := range [][2]int{{1920, 1080}, {1920, 1200}, {2560, 1440}, {2560, 1600}, {2880, 1800}, {3840, 2160}} {
				if size[0] >= o.BrowserWidth && size[1] >= o.BrowserHeight {
					fitting = append(fitting, size)
				}
			}
			i, err := choose(len(fitting))
			if err != nil {
				return nil, err
			}
			o.DisplayWidth, o.DisplayHeight = fitting[i][0], fitting[i][1]
		}
		if o.BrowserWidth == 0 {
			var fitting [][2]int
			for _, size := range [][2]int{{1024, 768}, {1280, 720}, {1280, 800}, {1366, 768}, {1440, 900}, {1600, 900}} {
				if size[0] <= o.DisplayWidth && size[1] <= o.DisplayHeight {
					fitting = append(fitting, size)
				}
			}
			if len(fitting) == 0 {
				o.BrowserWidth, o.BrowserHeight = o.DisplayWidth, o.DisplayHeight
			} else {
				i, err := choose(len(fitting))
				if err != nil {
					return nil, err
				}
				o.BrowserWidth, o.BrowserHeight = fitting[i][0], fitting[i][1]
			}
		}
		if o.ExtensionName == "" {
			names := []string{"Tab Layout", "Browser Workspace", "Window Panels", "Tab Organizer", "Workspace Tools"}
			i, err := choose(len(names))
			if err != nil {
				return nil, err
			}
			o.ExtensionName = names[i]
		}
	} else if o.AudioModel != "" {
		if err := chooseAudio(&o); err != nil {
			return nil, err
		}
	}
	for _, field := range []struct {
		value    *string
		fallback string
	}{
		{&o.Username, defaults.Username}, {&o.BrowserUsername, defaults.BrowserUsername},
		{&o.AudioName, defaults.AudioName}, {&o.AudioDescription, defaults.AudioDescription},
		{&o.ExtensionName, defaults.ExtensionName}, {&o.WebGL, defaults.WebGL}, {&o.Canvas, defaults.Canvas}, {&o.GraphicsBackend, defaults.GraphicsBackend},
	} {
		if *field.value == "" {
			*field.value = field.fallback
		}
	}
	if o.DisplayWidth == 0 {
		o.DisplayWidth, o.DisplayHeight = defaults.DisplayWidth, defaults.DisplayHeight
	}
	if o.BrowserWidth == 0 {
		o.BrowserWidth, o.BrowserHeight = min(defaults.BrowserWidth, o.DisplayWidth), min(defaults.BrowserHeight, o.DisplayHeight)
	}
	p := &Profile{Version: 1, Options: o}
	if o.Mode != "default" {
		var b [8]byte
		if _, err := io.ReadFull(rand.Reader, b[:]); err != nil {
			return nil, err
		}
		p.ExtensionSuffix = hex.EncodeToString(b[:])
	}
	return p, p.Validate()
}

func (p Profile) Validate() error {
	if p.Version != 1 || p.Mode == "" || (p.Mode == "default" && p.ExtensionSuffix != "") || (p.Mode != "default" && !suffix.MatchString(p.ExtensionSuffix)) {
		return errors.New("客机创建配置版本或扩展标识无效")
	}
	if p.Hostname == "" || p.Username == "" || p.BrowserUsername == "" || p.AudioName == "" || p.AudioDescription == "" || p.ExtensionName == "" || p.DisplayWidth == 0 || p.BrowserWidth == 0 || p.WebGL == "" || p.Canvas == "" {
		return errors.New("客机创建配置不完整")
	}
	if p.WorkHomePath() == p.BrowserHomePath() {
		return errors.New("两个账户不能共用主目录")
	}
	return p.Options.Validate()
}

// Load returns nil for environments without a record. A corrupt record fails.
func Load(directory string) (*Profile, error) {
	path := filepath.Join(directory, Filename)
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if info.Size() > 8192 {
		return nil, errors.New("客机创建配置超过大小限制")
	}
	if err := host.CheckPrivateFile(path); err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	d := json.NewDecoder(io.LimitReader(f, 8193))
	d.DisallowUnknownFields()
	var p Profile
	if err = d.Decode(&p); err != nil {
		return nil, errors.New("客机创建配置格式无效")
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return nil, errors.New("客机创建配置须仅含一个 JSON 对象")
	}
	return &p, p.Validate()
}

// Create publishes a complete record once, including when installers compete.
func Create(directory string, o Options) (*Profile, error) {
	if p, err := Load(directory); err != nil || p != nil {
		return p, err
	}
	p, err := New(o)
	if err != nil {
		return nil, err
	}
	if err = host.EnsurePrivateDir(directory); err != nil {
		return nil, err
	}
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return nil, err
	}
	f, err := os.CreateTemp(directory, ".guest-profile-*")
	if err != nil {
		return nil, err
	}
	defer os.Remove(f.Name())
	if err = host.ProtectFile(f.Name()); err == nil {
		_, err = f.Write(append(b, '\n'))
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return nil, err
	}
	if err = os.Link(f.Name(), filepath.Join(directory, Filename)); err != nil {
		if os.IsExist(err) {
			return Load(directory)
		}
		return nil, err
	}
	return p, nil
}

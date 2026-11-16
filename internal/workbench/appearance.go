package workbench

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/pelletier/go-toml/v2"
)

type AppearanceTheme struct {
	Preset        string            `json:"preset"`
	Accent        string            `json:"accent" toml:"accent"`
	Surface       string            `json:"surface" toml:"surface"`
	Ink           string            `json:"ink" toml:"ink"`
	Contrast      int               `json:"contrast" toml:"contrast"`
	OpaqueWindows bool              `json:"opaqueWindows" toml:"opaqueWindows"`
	Colors        map[string]string `json:"colors,omitempty"`
}
type SavedAppearanceTheme struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Light AppearanceTheme `json:"lightTheme"`
}
type Appearance struct {
	CustomThemes   []SavedAppearanceTheme `json:"customThemes,omitempty"`
	Light          AppearanceTheme        `json:"lightTheme"`
	UIFontSize     int                    `json:"uiFontSize"`
	CodeFontSize   int                    `json:"codeFontSize"`
	PointerCursors bool                   `json:"pointerCursors"`
	FontSmoothing  bool                   `json:"fontSmoothing"`
	ReducedMotion  string                 `json:"reducedMotion"`
}

func defaultAppearance() Appearance {
	return Appearance{Light: AppearanceTheme{Preset: "codex", Accent: "#339cff", Surface: "#ffffff", Ink: "#1a1c1f", Contrast: 45}, UIFontSize: 14, CodeFontSize: 13, FontSmoothing: true, ReducedMotion: "system"}
}

func (a Appearance) validate() error {
	if a.UIFontSize < 12 || a.UIFontSize > 24 || a.CodeFontSize < 10 || a.CodeFontSize > 24 {
		return errors.New("界面字号范围为 12–24，代码字号范围为 10–24")
	}
	if a.ReducedMotion != "system" && a.ReducedMotion != "on" && a.ReducedMotion != "off" {
		return errors.New("减少动态效果选项无效")
	}
	if err := validateAppearanceTheme(a.Light); err != nil {
		return err
	}
	if len(a.CustomThemes) > 50 {
		return errors.New("最多保存 50 个自定义主题，请先删除不需要的主题")
	}
	ids := make(map[string]bool)
	for _, saved := range a.CustomThemes {
		if !regexp.MustCompile(`^[a-zA-Z0-9_-]{1,80}$`).MatchString(saved.ID) || ids[saved.ID] {
			return errors.New("自定义主题标识无效或重复")
		}
		ids[saved.ID] = true
		if strings.TrimSpace(saved.Name) == "" || utf8.RuneCountInString(saved.Name) > 80 {
			return errors.New("请输入 1–80 个字符的主题名称")
		}
		if err := validateAppearanceTheme(saved.Light); err != nil {
			return err
		}
	}
	return nil
}

var appearanceColorKeys = strings.Fields("sidebar sidebarInk toolbar toolbarInk chrome chromeDark strong muted line input button buttonInk accentInk link focus hover selected selection success warning danger inactive")
var appearanceColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

func validateAppearanceTheme(theme AppearanceTheme) error {
	if !appearanceColor.MatchString(theme.Accent) || !appearanceColor.MatchString(theme.Surface) || !appearanceColor.MatchString(theme.Ink) {
		return errors.New("颜色须为六位十六进制值，例如 #fdf6e3")
	}
	if theme.Contrast < 0 || theme.Contrast > 100 {
		return errors.New("对比度范围为 0–100")
	}
	if utf8.RuneCountInString(theme.Preset) > 80 {
		return errors.New("主题名称过长")
	}
	for key, value := range theme.Colors {
		found := false
		for _, candidate := range appearanceColorKeys {
			if key == candidate {
				found = true
				break
			}
		}
		if !found || !appearanceColor.MatchString(value) {
			return errors.New("元素颜色无效或包含不支持的颜色项目")
		}
	}
	return nil
}

func (s *Server) getAppearance(w http.ResponseWriter, r *http.Request) {
	a := defaultAppearance()
	b, err := os.ReadFile(filepath.Join(s.profile, "appearance.json"))
	if err == nil {
		decoder := json.NewDecoder(bytes.NewReader(b))
		decoder.DisallowUnknownFields()
		err = decoder.Decode(&a)
		if err == nil && decoder.Decode(new(any)) != io.EOF {
			err = errors.New("外观设置包含多余内容")
		}
	}
	if err != nil && !os.IsNotExist(err) {
		requestError(w, errors.New("无法读取已保存的外观设置，请在外观页面重新保存当前配置"))
		return
	}
	if err = a.validate(); err != nil {
		requestError(w, err)
		return
	}
	jsonReply(w, a)
}

func (s *Server) saveAppearance(w http.ResponseWriter, r *http.Request) {
	var a Appearance
	if !readJSON(w, r, &a, 512<<10) {
		return
	}
	if err := a.validate(); err != nil {
		requestError(w, err)
		return
	}
	// Appearance has no VM or network side effects and uses its own atomic file.
	if err := writePrivate(filepath.Join(s.profile, "appearance.json"), a); err != nil {
		requestError(w, errors.New("外观设置未能保存，请重试"))
		return
	}
	jsonReply(w, a)
}

func readCodexAppearance(configPath string) (Appearance, error) {
	a := defaultAppearance()
	var config struct {
		Desktop struct {
			Light          *AppearanceTheme `toml:"appearanceLightChromeTheme"`
			LightPreset    string           `toml:"appearanceLightCodeThemeId"`
			UIFontSize     int              `toml:"sansFontSize"`
			CodeFontSize   int              `toml:"codeFontSize"`
			PointerCursors bool             `toml:"usePointerCursors"`
			FontSmoothing  *bool            `toml:"useFontSmoothing"`
		} `toml:"desktop"`
	}
	b, err := os.ReadFile(configPath)
	if err != nil {
		return a, errors.New("未找到本机 Codex 配置")
	}
	if err = toml.Unmarshal(b, &config); err != nil {
		return a, errors.New("无法解析本机 Codex 配置")
	}
	d := config.Desktop
	if d.Light != nil {
		a.Light = *d.Light
	}
	a.Light.Preset = d.LightPreset
	if a.Light.Preset == "" {
		a.Light.Preset = "codex"
	}
	if d.UIFontSize != 0 {
		a.UIFontSize = d.UIFontSize
	}
	if d.CodeFontSize != 0 {
		a.CodeFontSize = d.CodeFontSize
	}
	a.PointerCursors = d.PointerCursors
	if d.FontSmoothing != nil {
		a.FontSmoothing = *d.FontSmoothing
	}
	// This preference is stored separately by Codex; no other state is returned.
	var global struct {
		ReducedMotion string `json:"reduced-motion-preference"`
	}
	if b, err := os.ReadFile(filepath.Join(filepath.Dir(configPath), ".codex-global-state.json")); err == nil && json.Unmarshal(b, &global) == nil && global.ReducedMotion != "" {
		a.ReducedMotion = global.ReducedMotion
	}
	return a, a.validate()
}

func (s *Server) codexAppearance(w http.ResponseWriter, r *http.Request) {
	home, err := os.UserHomeDir()
	if err != nil {
		requestError(w, errors.New("无法定位本机用户目录"))
		return
	}
	a, err := readCodexAppearance(filepath.Join(home, ".codex", "config.toml"))
	if err != nil {
		requestError(w, err)
		return
	}
	jsonReply(w, a)
}

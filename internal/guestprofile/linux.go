package guestprofile

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// Linux renders an Eden-owned template exactly once, before adding user paths,
// filenames or commands. Protocol roles and service names remain unchanged.
func (p Profile) Linux(script string) string {
	if p.Version == 0 {
		return script
	}
	script = linuxHomePaths.ReplaceAllStringFunc(script, func(path string) string {
		m := linuxHomePaths.FindStringSubmatch(path)
		home := p.WorkHomePath()
		if m[1] == "browser" {
			home = p.BrowserHomePath()
		}
		return home + m[2]
	})
	// A single substitution pass also supports exchanging the two default names.
	script = linuxAccounts.ReplaceAllStringFunc(script, func(text string) string {
		m := linuxAccounts.FindStringSubmatch(text)
		name := p.WorkUser()
		if m[2] == "browser" {
			name = p.BrowserUser()
		}
		return m[1] + name + m[3]
	})
	script = linuxSudoers.ReplaceAllStringFunc(script, func(text string) string {
		name := p.WorkUser()
		if strings.HasPrefix(text, "browser") {
			name = p.BrowserUser()
		}
		return name + " ALL="
	})
	script = strings.ReplaceAll(script, "useradd --create-home --shell /bin/bash "+p.BrowserUser(), "useradd --create-home --home-dir "+p.BrowserHomePath()+" --shell /bin/bash "+p.BrowserUser())
	roles := fmt.Sprintf("{'developer': '%s', 'browser': '%s'}", p.WorkUser(), p.BrowserUser())
	for _, v := range []string{"user", "sys.argv[2]"} {
		script = strings.ReplaceAll(script, "pwd.getpwnam("+v+")", "pwd.getpwnam("+roles+".get("+v+", "+v+"))")
	}
	script = strings.ReplaceAll(script, "workbench_output", p.AudioName)
	if p.AudioDescription != "Workbench" {
		script = strings.ReplaceAll(script, "sink_properties=device.description=Workbench", `sink_properties="device.description='`+p.AudioDescription+`'"`)
	}
	script = strings.ReplaceAll(script, "2880x1800", fmt.Sprintf("%dx%d", p.DisplayWidth, p.DisplayHeight))
	script = strings.ReplaceAll(script, "--window-size=1440,900", fmt.Sprintf("--window-size=%d,%d", p.BrowserWidth, p.BrowserHeight))
	name, _ := json.Marshal(p.ExtensionName)
	script = strings.ReplaceAll(script, `"name": "Workbench browser panes"`, `"name": `+string(name))
	if p.ExtensionSuffix != "" {
		script = strings.ReplaceAll(script, "/usr/local/lib/workbench-split", "/usr/local/lib/browser-panes-"+p.ExtensionSuffix)
	}
	flags := p.GraphicsFlags()
	if p.WebGL == "disabled" {
		flags = append(flags, "--disable-3d-apis", "--disable-webgl")
	}
	if p.Canvas == "blocked" {
		flags = append(flags, "--disable-reading-from-canvas")
	}
	for _, flag := range flags {
		script = strings.ReplaceAll(script, `"--no-first-run",`, `"--no-first-run", `+fmt.Sprintf("%q", flag)+",")
	}
	return script
}

var linuxHomePaths = regexp.MustCompile(`/home/(browser|developer)([/\s"']|$)`)
var linuxSudoers = regexp.MustCompile(`(?:browser|developer) ALL=`)
var linuxAccounts = func() *regexp.Regexp {
	var prefixes []string
	for _, prefix := range []string{"User=", "Group=", "-u ", "-Hu ", `"-u", "`, "-o ", "-g ", "id ", "id -u ", "enable-linger ", "--shell /bin/bash ", "-G workbench-audio ", "ALL=(", "getpwnam('", "getgrnam('", `getpwnam("`, `getgrnam("`, `"$(id -un)" = `} {
		prefixes = append(prefixes, regexp.QuoteMeta(prefix))
	}
	return regexp.MustCompile(`(` + strings.Join(prefixes, "|") + `)(browser|developer)([^A-Za-z0-9_-]|$)`)
}()

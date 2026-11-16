package guestprofile

import (
	"strings"
	"testing"
)

func TestLinuxAccountsHomesAndProtocolRoles(t *testing.T) {
	for _, names := range [][2]string{{"alex", "web"}, {"browser", "developer"}, {"developer", "browser"}} {
		p, err := New(Options{Mode: "manual", Username: names[0], BrowserUsername: names[1], WorkHome: "/home/projects", BrowserHome: "/home/browsing"})
		if err != nil {
			t.Fatal(err)
		}
		s := p.Linux("User=browser\nGroup=workbench-audio\nUser=developer\nsudo -H -u developer cat /home/developer/file\nsudo useradd --create-home --shell /bin/bash browser\nbrowser ALL=(developer) NOPASSWD: /usr/local/bin/tool\nsystemctl restart workbench-browser.service\nmessage['source'] = 'browser'\naccount = pwd.getpwnam(user)\nif [ \"$(id -un)\" = developer ]; then true; fi\n")
		for _, want := range []string{"User=" + names[1] + "\n", "Group=workbench-audio\n", "User=" + names[0] + "\n", "-u " + names[0] + " cat /home/projects/file", "--home-dir /home/browsing --shell /bin/bash " + names[1], names[1] + " ALL=(" + names[0] + ")", "restart workbench-browser.service", "message['source'] = 'browser'", "'developer': '" + names[0] + "', 'browser': '" + names[1] + "'", `"$(id -un)" = ` + names[0]} {
			if !strings.Contains(s, want) {
				t.Fatalf("missing %q in %s", want, s)
			}
		}
	}
}

func TestLinuxDimensionsAndBrowser(t *testing.T) {
	p, err := New(Options{Mode: "manual", AudioName: "speakers", AudioDescription: "My Audio", ExtensionName: "My Tabs", WebGL: "disabled", Canvas: "blocked", DisplayWidth: 1920, DisplayHeight: 1080, BrowserWidth: 1280, BrowserHeight: 720})
	if err != nil {
		t.Fatal(err)
	}
	s := p.Linux(`load-module module-null-sink sink_name=workbench_output sink_properties=device.description=Workbench
set-default-source workbench_output.monitor
const flags = ["--no-first-run", "--window-size=1440,900"];
const directory = "/usr/local/lib/workbench-split";
{"name": "Workbench browser panes"}
Xtigervnc :77 -geometry 2880x1800
`)
	for _, want := range []string{"sink_name=speakers", "speakers.monitor", "--disable-webgl", "--disable-reading-from-canvas", "browser-panes-" + p.ExtensionSuffix, `"name": "My Tabs"`, "--window-size=1280,720", "-geometry 1920x1080"} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %q", want)
		}
	}
	var existing Profile
	if existing.Linux(s) != s {
		t.Fatal("existing profile templates changed")
	}
}

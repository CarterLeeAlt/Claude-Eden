package vm

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/sylvinus/agent-vm/internal/host"
	"golang.org/x/sys/windows"
)

func isWindowsQEMU(pid int, dir, name string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd, err := host.SystemPowerShellCommand(ctx, "-NoProfile", "-NonInteractive", "-Command",
		`[Console]::OutputEncoding=[Text.UTF8Encoding]::new($false); (Get-CimInstance Win32_Process -Filter 'ProcessId=`+strconv.Itoa(pid)+`').CommandLine | ConvertTo-Json -Compress`)
	if err != nil {
		return false
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	var command string
	if json.Unmarshal(out, &command) != nil {
		return false
	}
	return windowsQEMUCommandMatches(command, dir, name)
}

func windowsQEMUCommandMatches(command, dir, name string) bool {
	args, err := windows.DecomposeCommandLine(command)
	if err != nil || len(args) == 0 || !strings.EqualFold(filepath.Base(args[0]), "qemu-system-x86_64.exe") {
		return false
	}
	gotName, gotPID := false, false
	for i := 1; i+1 < len(args); i++ {
		switch args[i] {
		case "-name":
			gotName = args[i+1] == "lima-"+name
			i++
		case "-pidfile":
			gotPID = strings.EqualFold(filepath.Clean(args[i+1]), filepath.Join(dir, "qemu.pid"))
			i++
		}
	}
	return gotName && gotPID
}

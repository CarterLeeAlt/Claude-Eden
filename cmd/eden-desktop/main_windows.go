package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	agentvm "github.com/sylvinus/agent-vm"
	"github.com/sylvinus/agent-vm/internal/desktop"
	"github.com/sylvinus/agent-vm/internal/desktopupdate"
	"github.com/sylvinus/agent-vm/internal/version"
)

func main() { os.Exit(run(os.Args[1:])) }

func run(args []string) int {
	options := desktopAssets()
	updateLaunch := os.Getenv("EDEN_UPDATE_CONFIRMATION") != ""
	fail := func(err error) int {
		fmt.Fprintln(os.Stderr, err)
		if options.StartupFailed != nil {
			if confirmationError := options.StartupFailed(err); confirmationError != nil {
				fmt.Fprintln(os.Stderr, confirmationError)
			}
		}
		if startupFailureNeedsDialog(args, updateLaunch) {
			if dialogError := desktop.ShowStartupError(err.Error(), options); dialogError != nil {
				fmt.Fprintln(os.Stderr, dialogError)
			}
		}
		return 1
	}
	if len(args) > 0 && args[0] == "--native-test-backend" {
		if err := desktop.RunNativeTestBackend(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	}
	executable, err := os.Executable()
	if err != nil {
		return fail(err)
	}
	backend, err := desktop.NewBackend(filepath.Join(filepath.Dir(executable), "resources", "eden.exe"), "")
	if err != nil {
		return fail(err)
	}
	if len(args) == 1 && args[0] == "--install-update" {
		err = desktopupdate.RunInstaller(context.Background(), filepath.Join(backend.Root(), "updates"), executable, os.Stdin, os.Stdout)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	}
	if len(args) == 2 && args[0] == "--native-self-test" {
		report, err := desktop.RunNativeSelfTest(args[1], options)
		_ = json.NewEncoder(os.Stdout).Encode(report)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if !report.Success {
			return 1
		}
		return 0
	}
	if len(args) == 2 && args[0] == "--native-input-self-test" {
		report, err := desktop.RunNativeInputSelfTest(args[1], options)
		_ = json.NewEncoder(os.Stdout).Encode(report)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if !report.Success {
			return 1
		}
		return 0
	}
	if len(args) == 3 && args[0] == "--native-startup-self-test" {
		report, err := desktop.RunNativeStartupSelfTest(args[1], args[2], options)
		_ = json.NewEncoder(os.Stdout).Encode(report)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if !report.Success {
			return 1
		}
		return 0
	}
	var smoke string
	if len(args) == 2 && args[0] == "--smoke-test" && filepath.IsAbs(args[1]) {
		smoke = args[1]
	} else if len(args) != 0 {
		fmt.Fprintln(os.Stderr, "usage: Eden.exe [--smoke-test <absolute-result.json> | --native-self-test <output-directory> | --native-input-self-test <output-directory> | --native-startup-self-test <output-directory> <scenario> | --install-update]")
		return 2
	}
	var shell *desktop.Shell
	updater, err := desktopupdate.New(desktopupdate.Config{
		Root: filepath.Join(backend.Root(), "updates"), Target: filepath.Dir(executable), Executable: executable, Version: version.Version, ConfigPath: backend.ConfigPath, Backend: backend,
		Changed: func(state desktopupdate.State) {
			if shell != nil {
				shell.PublishUpdate(state)
			}
		},
		Quit: func() {
			if shell != nil {
				shell.Quit()
			}
		}, Reconnect: func() {
			if shell != nil {
				shell.Reconnect()
			}
		},
	})
	if err != nil {
		return fail(err)
	}
	defer updater.Close()
	options.BackendExecutable, options.ConfigPath, options.SmokeOutput = backend.Executable, backend.ConfigPath, smoke
	options.Update, options.MarkReady, options.CloseUpdate = updater.Handle, updater.MarkReady, updater.Close
	options.StartupFailed = updater.MarkStartupFailed
	shell, err = desktop.New(options)
	if err == nil {
		err = shell.Run()
	}
	if err != nil {
		return fail(err)
	}
	if smoke != "" {
		var report struct {
			Success bool `json:"success"`
		}
		data, err := os.ReadFile(smoke)
		if err != nil || json.Unmarshal(data, &report) != nil || !report.Success {
			return 1
		}
	}
	return 0
}

func desktopAssets() desktop.Options {
	return desktop.Options{Icon: agentvm.DesktopIcon, FontRegular: agentvm.DesktopFontRegular, FontBold: agentvm.DesktopFontBold}
}

// The independent update helper observes process exit. A failed update launch
// must not stay alive in a modal dialog before that failure becomes observable.
func startupFailureNeedsDialog(args []string, updateLaunch bool) bool {
	return len(args) == 0 && !updateLaunch
}

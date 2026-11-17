package desktop

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

type smokeProcess struct {
	handle windows.Handle
	pid    uint32
}

func openSmokeProcess(pid uint32, expected string) (smokeProcess, error) {
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE|windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
		return smokeProcess{}, nil
	}
	if err != nil {
		return smokeProcess{}, err
	}
	name := make([]uint16, 32768)
	length := uint32(len(name))
	if err = windows.QueryFullProcessImageName(handle, 0, &name[0], &length); err != nil {
		windows.CloseHandle(handle)
		return smokeProcess{}, err
	}
	actual := windows.UTF16ToString(name[:length])
	if expected != "" && !strings.EqualFold(filepath.Clean(actual), filepath.Clean(expected)) {
		windows.CloseHandle(handle)
		return smokeProcess{}, errors.New("自检后台身份与当前应用不符，未等待其他程序")
	}
	return smokeProcess{handle, pid}, nil
}

func closeSmokeProcesses(processes []smokeProcess) {
	for _, process := range processes {
		if process.handle != 0 {
			_ = windows.CloseHandle(process.handle)
		}
	}
}

func (b *Backend) captureSmokeControllers() ([]smokeProcess, error) {
	var processes []smokeProcess
	for _, profile := range []string{b.Root(), filepath.Join(b.Root(), "desktop-entry")} {
		data, err := os.ReadFile(filepath.Join(profile, "control.json"))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			closeSmokeProcesses(processes)
			return nil, err
		}
		var record struct {
			PID uint32 `json:"pid"`
		}
		if len(data) > 65536 || json.Unmarshal(data, &record) != nil || record.PID == 0 {
			closeSmokeProcesses(processes)
			return nil, errors.New("自检后台进程记录无效")
		}
		process, err := openSmokeProcess(record.PID, b.Executable)
		if err != nil {
			closeSmokeProcesses(processes)
			return nil, err
		}
		if process.handle != 0 {
			processes = append(processes, process)
		}
	}
	return processes, nil
}

func captureSmokeBrowsers() ([]smokeProcess, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(snapshot)
	var entries []windows.ProcessEntry32
	entry := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	for err = windows.Process32First(snapshot, &entry); err == nil; err = windows.Process32Next(snapshot, &entry) {
		entries = append(entries, entry)
	}
	if !errors.Is(err, windows.ERROR_NO_MORE_FILES) {
		return nil, err
	}
	owned := map[uint32]bool{uint32(os.Getpid()): true}
	var processes []smokeProcess
	for range len(entries) {
		changed := false
		for _, entry := range entries {
			if owned[entry.ProcessID] || !owned[entry.ParentProcessID] || !strings.EqualFold(windows.UTF16ToString(entry.ExeFile[:]), "msedgewebview2.exe") {
				continue
			}
			owned[entry.ProcessID] = true
			changed = true
			process, err := openSmokeProcess(entry.ProcessID, "")
			if err != nil {
				closeSmokeProcesses(processes)
				return nil, err
			}
			if process.handle != 0 {
				processes = append(processes, process)
			}
		}
		if !changed {
			break
		}
	}
	return processes, nil
}

type smokeResult struct {
	Success           bool             `json:"success"`
	NativeBridgeReady bool             `json:"nativeBridgeReady"`
	Version           string           `json:"version"`
	Phase             string           `json:"phase"`
	NativeStatus      string           `json:"nativeStatus"`
	PrepareExit       int              `json:"prepareExit"`
	PrepareStatus     string           `json:"prepareStatus"`
	PrepareMessage    string           `json:"prepareMessage"`
	Errors            []string         `json:"errors"`
	PageErrors        []smokePageError `json:"pageErrors"`
	PageURL           string           `json:"pageURL,omitempty"`
	WebviewVersion    string           `json:"webviewVersion,omitempty"`
	WindowBounds      Rectangle        `json:"windowBounds"`
	WorkingArea       Rectangle        `json:"workingArea"`
	DeviceDPI         int              `json:"deviceDpi"`
	NativeMenuBar     bool             `json:"nativeMenuBar"`
}

func smokeScreenshotPath(output string) string {
	return strings.TrimSuffix(output, filepath.Ext(output)) + ".png"
}

func (s *Shell) finishSmoke() error {
	if s.smokeResult == nil {
		return nil
	}
	// The Wails event loop must return before WebView2 can release its child
	// process. Waiting for that child here would keep the parent alive and make
	// the smoke check report a false shutdown timeout. The parent acceptance
	// process owns the bounded post-exit cleanup check.
	defer closeSmokeProcesses(s.smokeBrowsers)
	s.smokeResult.Success = s.smokeResult.NativeBridgeReady && len(s.smokeResult.Errors) == 0 && len(s.smokeResult.PageErrors) == 0
	if s.smokeResult.Success {
		s.smokeResult.Phase = "completed"
	}
	data, err := json.Marshal(s.smokeResult)
	if err != nil {
		return err
	}
	return os.WriteFile(s.options.SmokeOutput, data, 0600)
}

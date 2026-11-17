package desktop

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

func nativeTestMachine() (string, error) {
	mode := os.Getenv("EDEN_NATIVE_SELF_TEST")
	if mode != "vm" && mode != "approved-physical" {
		return "", errors.New("原生自检需显式授权测试环境：EDEN_NATIVE_SELF_TEST=vm 或 approved-physical")
	}
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `HARDWARE\DESCRIPTION\System\BIOS`, registry.QUERY_VALUE)
	if err != nil {
		return "", err
	}
	defer key.Close()
	manufacturer, _, _ := key.GetStringValue("SystemManufacturer")
	product, _, _ := key.GetStringValue("SystemProductName")
	if !nativeTestHostAllowed(mode, manufacturer, product) {
		return "", errors.New("原生自检未检测到虚拟机硬件，已拒绝修改剪贴板或焦点")
	}
	return manufacturer + " " + product, nil
}

func nativeTestHostAllowed(mode, manufacturer, product string) bool {
	if mode == "approved-physical" {
		return true
	}
	if mode != "vm" {
		return false
	}
	identity := strings.ToLower(manufacturer + " " + product)
	for _, name := range []string{"qemu", "kvm", "virtual machine", "virtualbox", "vmware"} {
		if strings.Contains(identity, name) {
			return true
		}
	}
	return false
}

// RunNativeTestBackend is an explicitly authorized child-process fixture for the actual
// DesktopService.RefreshSession cancellation path. It cannot control a VM,
// install anything or return a real controller token.
func RunNativeTestBackend(arguments []string) error {
	if _, err := nativeTestMachine(); err != nil {
		return err
	}
	if os.Getenv("EDEN_NATIVE_STARTUP_CASE") != "" && len(arguments) == 1 && arguments[0] == "diagnostic-event" {
		return nil
	}
	if len(arguments) != 2 || arguments[0] != "desktop" || arguments[1] != "--json" {
		return errors.New("invalid native-test backend fixture command")
	}
	output := os.Getenv("EDEN_NATIVE_TEST_OUTPUT")
	if !filepath.IsAbs(output) {
		return errors.New("native-test output must be absolute")
	}
	entry := os.Getenv("EDEN_NATIVE_TEST_ENTRY")
	if _, err := WorkbenchEntry(entry); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(output, "Backend_PID.txt"), []byte(strconv.Itoa(os.Getpid())), 0600); err != nil {
		return err
	}
	if os.Getenv("EDEN_NATIVE_STARTUP_CASE") == "" {
		time.Sleep(30 * time.Second)
	}
	return json.NewEncoder(os.Stdout).Encode(bootstrapReply{Status: "ready", URL: entry})
}

func probeBackendState(output string) (started, exited bool) {
	data, err := os.ReadFile(filepath.Join(output, "Backend_PID.txt"))
	if err != nil {
		return false, false
	}
	pid, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 32)
	if err != nil {
		return false, false
	}
	program, err := os.Executable()
	if err != nil {
		return true, false
	}
	process, err := openSmokeProcess(uint32(pid), program)
	if err != nil {
		return true, false
	}
	if process.handle == 0 {
		return true, true
	}
	defer windows.CloseHandle(process.handle)
	state, err := windows.WaitForSingleObject(process.handle, 0)
	return true, err == nil && state == windows.WAIT_OBJECT_0
}

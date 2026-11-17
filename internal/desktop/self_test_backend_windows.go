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
	if os.Getenv("EDEN_NATIVE_SELF_TEST") != "vm" {
		return "", errors.New("原生自检仅允许在测试虚拟机执行；需显式设置 EDEN_NATIVE_SELF_TEST=vm")
	}
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `HARDWARE\DESCRIPTION\System\BIOS`, registry.QUERY_VALUE)
	if err != nil {
		return "", err
	}
	defer key.Close()
	manufacturer, _, _ := key.GetStringValue("SystemManufacturer")
	product, _, _ := key.GetStringValue("SystemProductName")
	identity := strings.ToLower(manufacturer + " " + product)
	if !strings.Contains(identity, "qemu") && !strings.Contains(identity, "kvm") && !strings.Contains(identity, "virtual machine") && !strings.Contains(identity, "virtualbox") && !strings.Contains(identity, "vmware") {
		return "", errors.New("原生自检未检测到虚拟机硬件，已拒绝修改剪贴板或焦点")
	}
	return manufacturer + " " + product, nil
}

// RunNativeTestBackend is a VM-only child-process fixture for the actual
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

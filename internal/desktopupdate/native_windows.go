package desktopupdate

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"github.com/sylvinus/agent-vm/internal/installfiles"
	"golang.org/x/sys/windows"
)

var trustDLL = windows.NewLazySystemDLL("wintrust.dll")
var providerFromState = trustDLL.NewProc("WTHelperProvDataFromStateData")
var signerFromChain = trustDLL.NewProc("WTHelperGetProvSignerFromChain")
var certFromChain = trustDLL.NewProc("WTHelperGetProvCertFromChain")

type providerCert struct {
	Size        uint32
	Certificate *windows.CertContext
}

// Publisher uses Windows' cache-only Authenticode trust policy and extracts the
// actual verified signer's SPKI, not a subject name or an arbitrary embedded cert.
func signedPublisher(path string) (string, error) {
	f, e := installfiles.OpenReadLocked(path)
	if e != nil {
		return "", e
	}
	defer f.Close()
	p, e := windows.UTF16PtrFromString(path)
	if e != nil {
		return "", e
	}
	file := windows.WinTrustFileInfo{Size: uint32(unsafe.Sizeof(windows.WinTrustFileInfo{})), FilePath: p, File: windows.Handle(f.Fd())}
	data := windows.WinTrustData{Size: uint32(unsafe.Sizeof(windows.WinTrustData{})), UIChoice: 2, UnionChoice: 1, FileOrCatalogOrBlobOrSgnrOrCert: unsafe.Pointer(&file), StateAction: 1, ProvFlags: windows.WTD_CACHE_ONLY_URL_RETRIEVAL}
	action := windows.GUID{Data1: 0x00aac56b, Data2: 0xcd44, Data3: 0x11d0, Data4: [8]byte{0x8c, 0xc2, 0, 0xc0, 0x4f, 0xc2, 0x95, 0xee}}
	defer func() { data.StateAction = 2; _ = windows.WinVerifyTrustEx(windows.HWND(^uintptr(0)), &action, &data) }()
	if e = windows.WinVerifyTrustEx(windows.HWND(^uintptr(0)), &action, &data); e != nil {
		return "", fmt.Errorf("Windows 无法验证安装包签名；原应用已保留。请确认系统证书信任状态：%w", e)
	}
	provider, _, _ := providerFromState.Call(uintptr(data.StateData))
	if provider == 0 {
		return "", errors.New("无法读取 Windows 已验证的签名发布者")
	}
	signer, _, _ := signerFromChain.Call(provider, 0, 0, 0)
	if signer == 0 {
		return "", errors.New("Windows 签名缺少发布者")
	}
	cert, _, _ := certFromChain.Call(signer, 0)
	if cert == 0 {
		return "", errors.New("Windows 签名缺少发布者证书")
	}
	// WTHelper returns a native provider allocation, not Go-managed memory.
	// Copy its bounded prefix through Win32 rather than converting an opaque
	// integer address into a Go pointer that could outlive the trust state.
	var providerCertificate providerCert
	if e = windows.ReadProcessMemory(windows.CurrentProcess(), cert, (*byte)(unsafe.Pointer(&providerCertificate)), unsafe.Sizeof(providerCertificate), nil); e != nil {
		return "", e
	}
	c := providerCertificate.Certificate
	if c == nil || c.EncodedCert == nil || c.Length == 0 || c.Length > 1<<20 {
		return "", errors.New("Windows 发布者证书无效")
	}
	certificate, e := x509.ParseCertificate(unsafe.Slice(c.EncodedCert, int(c.Length)))
	if e != nil {
		return "", e
	}
	h := sha256.Sum256(certificate.RawSubjectPublicKeyInfo)
	return strings.ToUpper(hex.EncodeToString(h[:])), nil
}

func ProductVersion(path string) (string, string, error) {
	if e := installfiles.PlainPath(path); e != nil {
		return "", "", e
	}
	size, e := windows.GetFileVersionInfoSize(path, nil)
	if e != nil {
		return "", "", e
	}
	if size == 0 || size > 4<<20 {
		return "", "", errors.New("程序版本资源大小无效")
	}
	b := make([]byte, size)
	if e = windows.GetFileVersionInfo(path, 0, size, unsafe.Pointer(&b[0])); e != nil {
		return "", "", e
	}
	var p unsafe.Pointer
	var n uint32
	if e = windows.VerQueryValue(unsafe.Pointer(&b[0]), `\VarFileInfo\Translation`, unsafe.Pointer(&p), &n); e != nil {
		return "", "", e
	}
	if n < 4 || n%4 != 0 {
		return "", "", errors.New("程序版本语言资源无效")
	}
	translations := unsafe.Slice((*uint16)(p), int(n/2))
	for i := 0; i < len(translations); i += 2 {
		query := func(key string) (string, error) {
			var text *uint16
			var length uint32
			e := windows.VerQueryValue(unsafe.Pointer(&b[0]), fmt.Sprintf(`\StringFileInfo\%04x%04x\%s`, translations[i], translations[i+1], key), unsafe.Pointer(&text), &length)
			if e != nil {
				return "", e
			}
			if text == nil || length > 1<<20 {
				return "", errors.New("程序版本字符串无效")
			}
			return windows.UTF16ToString(unsafe.Slice(text, int(length))), nil
		}
		product, e := query("ProductName")
		if e != nil {
			continue
		}
		version, e := query("ProductVersion")
		if e == nil {
			return product, version, nil
		}
	}
	return "", "", errors.New("程序缺少产品版本资源")
}

type parentProcess struct{ handle windows.Handle }

func processTicks(h windows.Handle) (int64, error) {
	var creation, exit, kernel, user windows.Filetime
	if e := windows.GetProcessTimes(h, &creation, &exit, &kernel, &user); e != nil {
		return 0, e
	}
	return int64(uint64(creation.HighDateTime)<<32|uint64(creation.LowDateTime)) + 504911232000000000, nil
}
func CurrentStartTicks() (int64, error) { return processTicks(windows.CurrentProcess()) }
func openParent(pid int, ticks int64, image string) (*parentProcess, error) {
	if pid <= 0 {
		return nil, errors.New("更新请求进程标识无效")
	}
	h, e := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.SYNCHRONIZE, false, uint32(pid))
	if e != nil {
		return nil, e
	}
	fail := func(e error) (*parentProcess, error) { windows.CloseHandle(h); return nil, e }
	actual, e := processTicks(h)
	if e != nil {
		return fail(e)
	}
	text := make([]uint16, 32768)
	size := uint32(len(text))
	if e = windows.QueryFullProcessImageName(h, 0, &text[0], &size); e != nil {
		return fail(e)
	}
	if actual != ticks || !strings.EqualFold(filepath.Clean(windows.UTF16ToString(text[:size])), filepath.Clean(image)) {
		return fail(errors.New("更新请求进程身份不匹配"))
	}
	return &parentProcess{h}, nil
}
func (p *parentProcess) Close() { windows.CloseHandle(p.handle) }
func (p *parentProcess) Wait(ctx context.Context) error {
	for {
		status, e := windows.WaitForSingleObject(p.handle, 100)
		if e != nil {
			return e
		}
		if status == windows.WAIT_OBJECT_0 {
			return nil
		}
		if e = ctx.Err(); e != nil {
			return e
		}
	}
}
func installationLock() (func(), error) {
	name, _ := windows.UTF16PtrFromString(`Global\Eden.Installation`)
	h, e := windows.CreateMutex(nil, false, name)
	if e != nil {
		if h != 0 {
			windows.CloseHandle(h)
		}
		return nil, fmt.Errorf("另一个安装或卸载正在进行，已保留更新备份：%w", e)
	}
	return func() { windows.CloseHandle(h) }, nil
}
func hide(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.CREATE_NO_WINDOW}
}

func isSharingViolation(err error) bool {
	return errors.Is(err, windows.ERROR_SHARING_VIOLATION) || errors.Is(err, windows.ERROR_LOCK_VIOLATION)
}
func installerCommand(path, target, diagnostics string) *exec.Cmd {
	cmd := exec.Command(path)
	hide(cmd)
	// NSIS parses the final /D= value itself. It must not be quoted, even
	// when the directory contains spaces; no shell participates.
	line := syscall.EscapeArg(path) + " /S /UPDATE "
	if diagnostics != "" {
		line += "/DIAGNOSTICS=\"" + diagnostics + "\" "
	}
	cmd.SysProcAttr.CmdLine = line + "/D=" + target
	cmd.Dir = filepath.Dir(path)
	return cmd
}

// runProcess places the suspended installer in a kill-on-close job before any
// user code runs. Cancellation/timeout cannot leave an NSIS child installing.
func runProcess(ctx context.Context, cmd *exec.Cmd) error {
	if e := ctx.Err(); e != nil {
		return e
	}
	job, e := windows.CreateJobObject(nil, nil)
	if e != nil {
		return e
	}
	defer windows.CloseHandle(job)
	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, e = windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits))); e != nil {
		return e
	}
	if cmd.SysProcAttr == nil {
		hide(cmd)
	}
	cmd.SysProcAttr.CreationFlags |= windows.CREATE_SUSPENDED
	if e = cmd.Start(); e != nil {
		return e
	}
	h, e := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE|windows.PROCESS_SUSPEND_RESUME, false, uint32(cmd.Process.Pid))
	if e != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return e
	}
	defer windows.CloseHandle(h)
	if e = windows.AssignProcessToJobObject(job, h); e != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return e
	}
	resume := windows.NewLazySystemDLL("ntdll.dll").NewProc("NtResumeProcess")
	status, _, _ := resume.Call(uintptr(h))
	if int32(status) < 0 {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return fmt.Errorf("无法启动安装进程：NTSTATUS %x", status)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case e = <-done:
		return e
	case <-ctx.Done():
		_ = windows.TerminateJobObject(job, 1)
		<-done
		return ctx.Err()
	}
}

func showRecovery(message string) {
	text, _ := windows.UTF16PtrFromString(message)
	title, _ := windows.UTF16PtrFromString("Claude Eden 更新需要恢复")
	windows.NewLazySystemDLL("user32.dll").NewProc("MessageBoxW").Call(0, uintptr(unsafe.Pointer(text)), uintptr(unsafe.Pointer(title)), 0x00010010)
}

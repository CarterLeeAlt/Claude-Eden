package desktop

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unicode"
	"unsafe"

	"github.com/google/uuid"
	"golang.org/x/sys/windows"
)

type inputProcessorProfile struct {
	Type         uint32
	Language     uint16
	CLSID        windows.GUID
	Profile      windows.GUID
	Category     windows.GUID
	Substitute   uintptr
	Capabilities uint32
	Layout       uintptr
	Flags        uint32
}
type profileManager struct{ vtable *[11]uintptr }

func (manager *profileManager) activate(profile inputProcessorProfile) error {
	hr, _, _ := syscall.SyscallN(manager.vtable[3], uintptr(unsafe.Pointer(manager)), uintptr(profile.Type), uintptr(profile.Language), uintptr(unsafe.Pointer(&profile.CLSID)), uintptr(unsafe.Pointer(&profile.Profile)), profile.Layout, 0)
	if hr != 0 {
		return fmt.Errorf("TSF ActivateProfile HRESULT 0x%08x", uint32(hr))
	}
	return nil
}

func activatePinyinProfile() (func() error, error) {
	ole := windows.NewLazySystemDLL("ole32.dll")
	hr, _, _ := ole.NewProc("CoInitializeEx").Call(0, 2)
	if int32(hr) < 0 {
		return nil, fmt.Errorf("TSF CoInitializeEx HRESULT 0x%08x", uint32(hr))
	}
	uninitialize := func() { ole.NewProc("CoUninitialize").Call() }
	threadClass := windows.GUID{Data1: 0x529a9e6b, Data2: 0x6587, Data3: 0x4f23, Data4: [8]byte{0xab, 0x9e, 0x9c, 0x7d, 0x68, 0x3e, 0x3c, 0x50}}
	threadIID := windows.GUID{Data1: 0xaa80e801, Data2: 0x2021, Data3: 0x11d2, Data4: [8]byte{0x93, 0xe0, 0, 0x60, 0xb0, 0x67, 0xb8, 0x6e}}
	var thread *struct{ vtable *[14]uintptr }
	hr, _, _ = ole.NewProc("CoCreateInstance").Call(uintptr(unsafe.Pointer(&threadClass)), 0, 1, uintptr(unsafe.Pointer(&threadIID)), uintptr(unsafe.Pointer(&thread)))
	if int32(hr) < 0 || thread == nil {
		uninitialize()
		return nil, fmt.Errorf("创建 TSF thread manager HRESULT 0x%08x", uint32(hr))
	}
	var client uint32
	hr, _, _ = syscall.SyscallN(thread.vtable[3], uintptr(unsafe.Pointer(thread)), uintptr(unsafe.Pointer(&client)))
	if int32(hr) < 0 {
		syscall.SyscallN(thread.vtable[2], uintptr(unsafe.Pointer(thread)))
		uninitialize()
		return nil, fmt.Errorf("激活 TSF thread manager HRESULT 0x%08x", uint32(hr))
	}
	closeCOM := uninitialize
	uninitialize = func() {
		syscall.SyscallN(thread.vtable[4], uintptr(unsafe.Pointer(thread)))
		syscall.SyscallN(thread.vtable[2], uintptr(unsafe.Pointer(thread)))
		closeCOM()
	}
	clsid := windows.GUID{Data1: 0x33c53a50, Data2: 0xf456, Data3: 0x4884, Data4: [8]byte{0xb0, 0x49, 0x85, 0xfd, 0x64, 0x3e, 0xcf, 0xed}}
	iid := windows.GUID{Data1: 0x71c6e74c, Data2: 0x0f28, Data3: 0x11d8, Data4: [8]byte{0xa8, 0x2a, 0, 6, 0x5b, 0x84, 0x43, 0x5c}}
	var manager *profileManager
	hr, _, _ = ole.NewProc("CoCreateInstance").Call(uintptr(unsafe.Pointer(&clsid)), 0, 1, uintptr(unsafe.Pointer(&iid)), uintptr(unsafe.Pointer(&manager)))
	if int32(hr) < 0 || manager == nil {
		uninitialize()
		return nil, fmt.Errorf("创建 TSF profile manager HRESULT 0x%08x", uint32(hr))
	}
	release := func() { syscall.SyscallN(manager.vtable[2], uintptr(unsafe.Pointer(manager))); uninitialize() }
	profilesIID := windows.GUID{Data1: 0x1f02b6c5, Data2: 0x7842, Data3: 0x4ee6, Data4: [8]byte{0x8a, 0x0b, 0x9a, 0x24, 0x18, 0x3a, 0x95, 0xca}}
	var profiles *struct{ vtable *[21]uintptr }
	hr, _, _ = syscall.SyscallN(manager.vtable[0], uintptr(unsafe.Pointer(manager)), uintptr(unsafe.Pointer(&profilesIID)), uintptr(unsafe.Pointer(&profiles)))
	if int32(hr) < 0 || profiles == nil {
		release()
		return nil, fmt.Errorf("查询 TSF profiles HRESULT 0x%08x", uint32(hr))
	}
	closeManager := release
	release = func() { syscall.SyscallN(profiles.vtable[2], uintptr(unsafe.Pointer(profiles))); closeManager() }
	var originalLanguage uint16
	hr, _, _ = syscall.SyscallN(profiles.vtable[13], uintptr(unsafe.Pointer(profiles)), uintptr(unsafe.Pointer(&originalLanguage)))
	if hr != 0 {
		release()
		return nil, fmt.Errorf("读取原线程输入语言 HRESULT 0x%08x", uint32(hr))
	}
	originalLayout, _, _ := user32.NewProc("GetKeyboardLayout").Call(0)
	changeLanguage := func(language uint16) error {
		hr, _, _ := syscall.SyscallN(profiles.vtable[14], uintptr(unsafe.Pointer(profiles)), uintptr(language))
		if hr != 0 {
			return fmt.Errorf("切换测试线程输入语言 HRESULT 0x%08x", uint32(hr))
		}
		return nil
	}
	category := windows.GUID{Data1: 0x34745c63, Data2: 0xb2f0, Data3: 0x4784, Data4: [8]byte{0x8b, 0x67, 0x5e, 0x12, 0xc8, 0x70, 0x1a, 0x31}}
	var original inputProcessorProfile
	hr, _, _ = syscall.SyscallN(manager.vtable[10], uintptr(unsafe.Pointer(manager)), uintptr(unsafe.Pointer(&category)), uintptr(unsafe.Pointer(&original)))
	if hr != 0 || original.Type == 0 {
		release()
		return nil, fmt.Errorf("读取原 TSF profile HRESULT 0x%08x", uint32(hr))
	}
	restoreOriginal := func() error {
		languageError := changeLanguage(originalLanguage)
		profileError := manager.activate(original)
		actual, _, _ := user32.NewProc("GetKeyboardLayout").Call(0)
		var readbackError error
		if actual != originalLayout {
			readbackError = errors.New("原线程输入布局恢复回读不一致")
		}
		var restored inputProcessorProfile
		hr, _, _ := syscall.SyscallN(manager.vtable[10], uintptr(unsafe.Pointer(manager)), uintptr(unsafe.Pointer(&category)), uintptr(unsafe.Pointer(&restored)))
		if hr != 0 || restored.Type != original.Type || restored.Language != original.Language || restored.CLSID != original.CLSID || restored.Profile != original.Profile || restored.Layout != original.Layout {
			readbackError = errors.Join(readbackError, errors.New("原线程 TSF profile 恢复回读不一致"))
		}
		return errors.Join(languageError, profileError, readbackError)
	}
	abortActivation := func(activationError error) (func() error, error) {
		restoreError := restoreOriginal()
		release()
		if restoreError != nil {
			activationError = errors.Join(activationError, fmt.Errorf("恢复原线程 TSF profile：%w", restoreError))
		}
		return nil, activationError
	}
	// ActivateProfile's zero flags change only this thread. Never use its
	// FORPROCESS/FORSESSION flags or change the user's default input method.
	pinyin := inputProcessorProfile{Type: 1, Language: 0x0804, CLSID: windows.GUID{Data1: 0x81d4e9c9, Data2: 0x1d3b, Data3: 0x41bc, Data4: [8]byte{0x9e, 0x6c, 0x4b, 0x40, 0xbf, 0x79, 0xe3, 0x5e}}, Profile: windows.GUID{Data1: 0xfa550b04, Data2: 0x5ad7, Data3: 0x411f, Data4: [8]byte{0xa5, 0xac, 0xca, 0x03, 0x8e, 0xc5, 0x15, 0xd7}}}
	if err := changeLanguage(0x0804); err != nil {
		return abortActivation(err)
	}
	if err := manager.activate(pinyin); err != nil {
		return abortActivation(err)
	}
	var active inputProcessorProfile
	hr, _, _ = syscall.SyscallN(manager.vtable[10], uintptr(unsafe.Pointer(manager)), uintptr(unsafe.Pointer(&category)), uintptr(unsafe.Pointer(&active)))
	if hr != 0 || active.Type != pinyin.Type || active.CLSID != pinyin.CLSID || active.Profile != pinyin.Profile || active.Language != pinyin.Language {
		return abortActivation(errors.New("TSF profile 激活回读不是指定的微软拼音"))
	}
	return func() error { defer release(); return restoreOriginal() }, nil
}

// INPUT's native union occupies 32 bytes on Windows amd64. A Go struct with all
// three union members would have the wrong SendInput cbSize and layout.
type nativeKeyboardInput struct {
	Type        uint32
	_           uint32
	VK, Scan    uint16
	Flags, Time uint32
	_           uint32
	Extra       uintptr
	_           [8]byte
}

func sendSelfTestKey(input *nativeInput, key uint16) error {
	foreground, _, _ := getForegroundWindow.Call()
	focus, _, _ := getFocus.Call()
	if foreground != input.parent || focus != input.hwnd {
		return errors.New("VM 自检失去自有窗口焦点，已拒绝注入任何按键")
	}
	values := [2]nativeKeyboardInput{{Type: 1, VK: key}, {Type: 1, VK: key, Flags: 2}}
	count, _, err := user32.NewProc("SendInput").Call(2, uintptr(unsafe.Pointer(&values[0])), unsafe.Sizeof(values[0]))
	if count != 2 {
		return fmtNative("VM 自检 SendInput 未发送完整按键", err)
	}
	pumpNativeMessages(100 * time.Millisecond)
	return nil
}

func typeSelfTestPinyin(input *nativeInput, text string) error {
	for _, character := range strings.ToUpper(text) {
		if character < 'A' || character > 'Z' {
			return errors.New("自检拼音只接受字母")
		}
		if err := sendSelfTestKey(input, uint16(character)); err != nil {
			return err
		}
	}
	return nil
}

func visibleCandidateWindows() []uintptr {
	var candidates []uintptr
	callback := syscall.NewCallback(func(hwnd, lParam uintptr) uintptr {
		visible, _, _ := user32.NewProc("IsWindowVisible").Call(hwnd)
		if visible == 0 {
			return 1
		}
		name := make([]uint16, 256)
		user32.NewProc("GetClassNameW").Call(hwnd, uintptr(unsafe.Pointer(&name[0])), uintptr(len(name)))
		class := windows.UTF16ToString(name)
		if strings.Contains(class, "IME") && (strings.Contains(class, "Candidate") || strings.Contains(class, "UIManager")) {
			candidates = append(candidates, hwnd)
		}
		return 1
	})
	user32.NewProc("EnumWindows").Call(callback, 0)
	return candidates
}

func containsChinese(text string) bool {
	for _, character := range text {
		if unicode.Is(unicode.Han, character) {
			return true
		}
	}
	return false
}

// The caller activates and restores the thread's Pinyin profile around both
// the IMM positioning checks and these real key/selection checks.
func exerciseRealPinyin(input *nativeInput, output string, report *NativeSelfTestReport) (err error) {
	keyboard := make([]byte, 256)
	result, _, _ := user32.NewProc("GetKeyboardState").Call(uintptr(unsafe.Pointer(&keyboard[0])))
	if result == 0 {
		return errors.New("无法保存 VM 原键盘状态")
	}
	defer func() {
		result, _, nativeError := user32.NewProc("SetKeyboardState").Call(uintptr(unsafe.Pointer(&keyboard[0])))
		if result == 0 {
			err = errors.Join(err, fmtNative("恢复 VM 测试线程键盘状态失败", nativeError))
		}
	}()
	cleared := make([]byte, 256)
	result, _, nativeError := user32.NewProc("SetKeyboardState").Call(uintptr(unsafe.Pointer(&cleared[0])))
	if result == 0 {
		return fmtNative("清除 VM 测试线程键盘状态失败", nativeError)
	}
	owner := uuid.NewString()
	if _, err = input.request(RemoteInputRequest{Owner: owner, Operation: "activate", X: 160, Y: 220, Height: 28}, 1); err != nil {
		return err
	}
	pumpNativeMessages(150 * time.Millisecond)
	context, _, _ := immGetContext.Call(input.hwnd)
	if context == 0 {
		return errors.New("微软拼音没有为 EDIT 提供 IMM/TSF 输入上下文")
	}
	imm32.NewProc("ImmSetOpenStatus").Call(context, 1)
	imm32.NewProc("ImmSetConversionStatus").Call(context, 1, 0)
	immReleaseContext.Call(input.hwnd, context)
	pumpNativeMessages(150 * time.Millisecond)
	if err := captureVisibleNativeWindow(input.parent, filepath.Join(output, "Native_IME_Before.png")); err != nil {
		return err
	}
	report.Screenshots = append(report.Screenshots, "Native_IME_Before.png")
	start := len(report.InputEvents)
	if err = typeSelfTestPinyin(input, "nihao"); err != nil {
		return err
	}
	deadline := time.Now().Add(3 * time.Second)
	for !input.composing && time.Now().Before(deadline) {
		pumpNativeMessages(50 * time.Millisecond)
	}
	if !input.composing {
		return errors.New("真实微软拼音 nihao 按键未进入组合；不能以合成 IMM 消息替代通过")
	}
	compositionStart, updates := 0, 0
	for _, event := range report.InputEvents[start:] {
		if event.Kind == "compositionstart" {
			compositionStart++
		}
		if event.Kind == "compositionupdate" {
			updates++
		}
	}
	if compositionStart != 1 || updates == 0 {
		return fmt.Errorf("真实拼音组合事件异常：start=%d update=%d", compositionStart, updates)
	}
	if err = captureNativeWindow(input.parent, filepath.Join(output, "Native_IME_Composition.png")); err != nil {
		return err
	}
	report.Screenshots = append(report.Screenshots, "Native_IME_Composition.png")
	if err := captureVisibleNativeWindow(input.parent, filepath.Join(output, "Native_IME_Visible.png")); err != nil {
		return err
	}
	report.Screenshots = append(report.Screenshots, "Native_IME_Visible.png")
	candidates := visibleCandidateWindows()
	var candidateError error
	visible, note, visibilityError := renderedIMECandidate(input, filepath.Join(output, "Native_IME_Before.png"), filepath.Join(output, "Native_IME_Visible.png"))
	report.Notes = append(report.Notes, note)
	if !visible || visibilityError != nil {
		_, diagnosticError := writeIMEWindowDiagnostics(input, output, report)
		candidateError = errors.Join(errors.New("未观测到真实微软拼音可见候选条，候选框验收未通过"), visibilityError, diagnosticError)
	}
	for index, window := range candidates {
		name := fmt.Sprintf("Native_IME_Candidate_%02d.png", index+1)
		if err = captureNativeWindow(window, filepath.Join(output, name)); err != nil {
			return err
		}
		report.Screenshots = append(report.Screenshots, name)
	}
	if err = sendSelfTestKey(input, 0x20); err != nil {
		return err
	}
	pumpNativeMessages(300 * time.Millisecond)
	commits := 0
	committed := ""
	for _, event := range report.InputEvents[start:] {
		if event.Kind == "compositionend" && event.Text != "" {
			commits++
			committed += event.Text
		}
		if event.Kind == "input" && containsChinese(event.Text) {
			return errors.New("微软拼音结果在 compositionend 后又通过 input 重复发送")
		}
	}
	if commits != 1 || committed != "你好" {
		return errors.Join(candidateError, fmt.Errorf("真实微软拼音选词结果不符：commit=%d text=%q", commits, committed))
	}
	report.Checks = append(report.Checks, "真实微软拼音 TSF 激活、nihao 按键、空格选词“你好”仅提交一次")
	if candidateError == nil {
		report.Checks = append(report.Checks, "真实微软拼音候选条已绘制到实际屏幕，排除 EDIT 像素后仍存在完整候选行；截图回读“你好”")
	}
	if err := verifyIMECandidateClosed(input, output, "Commit", report); err != nil {
		return errors.Join(candidateError, err)
	}
	start = len(report.InputEvents)
	if err = typeSelfTestPinyin(input, "ceshi"); err != nil {
		return err
	}
	if !input.composing {
		return errors.New("取消测试未进入真实拼音组合")
	}
	if err = sendSelfTestKey(input, 0x1b); err != nil {
		return err
	}
	pumpNativeMessages(150 * time.Millisecond)
	if input.composing {
		return errors.New("Escape 未结束真实拼音组合")
	}
	for _, event := range report.InputEvents[start:] {
		if event.Kind == "compositionend" && event.Text != "" {
			return errors.New("取消真实拼音组合时意外提交文本")
		}
	}
	report.Checks = append(report.Checks, "真实微软拼音 Escape 取消不提交文本")
	if err := verifyIMECandidateClosed(input, output, "Escape", report); err != nil {
		return errors.Join(candidateError, err)
	}
	start = len(report.InputEvents)
	if err = typeSelfTestPinyin(input, "zhongwen"); err != nil {
		return err
	}
	if !input.composing {
		return errors.New("失焦测试未进入真实拼音组合")
	}
	setFocus.Call(input.parent)
	pumpNativeMessages(200 * time.Millisecond)
	deactivated := 0
	for _, event := range report.InputEvents[start:] {
		if event.Kind == "deactivated" {
			deactivated++
		}
		if event.Kind == "compositionend" && event.Text != "" {
			return errors.New("失焦时意外提交真实拼音文本")
		}
	}
	if input.owner != "" || input.composing || deactivated != 1 {
		return errors.New("真实拼音失焦未释放 owner/组合状态")
	}
	report.Checks = append(report.Checks, "真实微软拼音失焦取消、deactivated 单次和 owner 释放")
	if err := verifyIMECandidateClosed(input, output, "Blur", report); err != nil {
		return errors.Join(candidateError, err)
	}
	if candidateError != nil {
		candidateError = errors.Join(candidateError, exerciseVisibleEditBaseline(input, output, report))
	}
	return candidateError
}

// Failure-only comparison with an ordinary visible EDIT in the same thread and
// TSF profile. This changes no production style and keeps the original carrier
// failure in the report, even when the independent baseline works.
func exerciseVisibleEditBaseline(input *nativeInput, output string, report *NativeSelfTestReport) error {
	edit, _, nativeError := createWindowEx.Call(0, uintptr(unsafe.Pointer(windows.StringToUTF16Ptr("EDIT"))), 0, 0x50000000|0x0004|0x0040, 80, 300, 400, 50, input.parent, 0, 0, 0)
	if edit == 0 {
		return fmtNative("无法创建标准 EDIT 对照控件", nativeError)
	}
	defer destroyWindow.Call(edit)
	setFocus.Call(edit)
	pumpNativeMessages(100 * time.Millisecond)
	baseline := &nativeInput{hwnd: edit, parent: input.parent}
	if err := typeSelfTestPinyin(baseline, "nihao"); err != nil {
		return err
	}
	pumpNativeMessages(300 * time.Millisecond)
	context, _, _ := immGetContext.Call(edit)
	if context == 0 {
		return errors.New("标准 EDIT 对照缺少 IMM 上下文")
	}
	composition := compositionText(context, 8)
	immReleaseContext.Call(edit, context)
	report.Notes = append(report.Notes, fmt.Sprintf("同线程标准可见 EDIT 对照：组合文本=%q，经典候选窗口数=%d；对照结果不替代原输入载体验收", composition, len(visibleCandidateWindows())))
	if err := captureVisibleNativeWindow(input.parent, filepath.Join(output, "Native_IME_Baseline.png")); err != nil {
		return err
	}
	report.Screenshots = append(report.Screenshots, "Native_IME_Baseline.png")
	if err := sendSelfTestKey(baseline, 27); err != nil {
		return err
	}
	context, _, _ = immGetContext.Call(edit)
	if context == 0 {
		return errors.New("标准 EDIT 对照取消后缺少 IMM 上下文")
	}
	remaining := compositionText(context, 8)
	immReleaseContext.Call(edit, context)
	if remaining != "" {
		return errors.New("标准 EDIT 对照未能取消组合状态")
	}
	return nil
}

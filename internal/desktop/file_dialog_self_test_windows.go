package desktop

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

type fileDialogSelfTest struct {
	kind, title, input string
	cancel             bool
}

func runFileDialogSelfTest(shell *Shell, output string, report *NativeSelfTestReport) error {
	if _, err := nativeTestMachine(); err != nil {
		return err
	}
	const theme = `{"format":"eden-theme","version":2}`
	one, two := filepath.Join(output, "Dialog_One.json"), filepath.Join(output, "Dialog_Two.json")
	for _, file := range []string{one, two} {
		if err := os.WriteFile(file, []byte(theme), 0600); err != nil {
			return err
		}
	}
	folder := filepath.Join(output, "Dialog_Folder")
	if err := os.Mkdir(folder, 0700); err != nil {
		return err
	}
	tests := []fileDialogSelfTest{
		{kind: "single", title: "选择本机文件", input: one},
		{kind: "multiple", title: "选择本机文件", input: fmt.Sprintf(`"%s" "%s"`, one, two)},
		{kind: "import", title: "导入主题", input: one},
		{kind: "export", title: "导出主题", input: filepath.Join(output, "Dialog_Export")},
		{kind: "folder", title: "仅此文件夹会与 Claude Eden 共享。请选择具体项目。", input: folder},
		{kind: "recovery", title: "保存账户恢复密钥", input: filepath.Join(output, "Dialog_Recovery.txt")},
	}
	for _, test := range tests {
		for _, canceled := range []bool{false, true} {
			test.cancel = canceled
			name := fmt.Sprintf("Native_File_%s_%t.png", test.kind, canceled)
			ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
			driven := make(chan error, 1)
			go func() { driven <- driveNativeFileDialog(ctx, test, filepath.Join(output, name)) }()
			promptError := runFileDialogCase(shell.service, test, one, two, theme, folder)
			if promptError != nil {
				cancel()
			}
			driverError := <-driven
			cancel()
			if _, err := os.Stat(filepath.Join(output, name)); err == nil {
				report.Screenshots = append(report.Screenshots, name)
			}
			if err := errors.Join(promptError, driverError); err != nil {
				return fmt.Errorf("真实文件对话框 %s cancel=%t：%w", test.kind, canceled, err)
			}
			report.Checks = append(report.Checks, fmt.Sprintf("DesktopService 原路径文件对话框 %s cancel=%t，核对真实返回值与工作区文件", test.kind, canceled))
		}
	}
	return nil
}

func runFileDialogCase(service *DesktopService, test fileDialogSelfTest, one, two, theme, folder string) error {
	switch test.kind {
	case "single", "multiple":
		selection, err := service.ChooseUploadFiles(test.kind == "multiple")
		if err != nil {
			return err
		}
		if test.cancel {
			if !selection.Canceled || selection.FileSelection != "" || selection.Count != 0 {
				return errors.New("取消选择未返回空选择")
			}
			return nil
		}
		service.selectionMu.Lock()
		paths := service.selections[selection.FileSelection].paths
		service.selectionMu.Unlock()
		count := 1
		if test.kind == "multiple" {
			count = 2
		}
		if selection.Canceled || len(paths) != count || selection.Count != count || !strings.EqualFold(paths[0], one) {
			return errors.New("文件选择结果与实际选择不符")
		}
		if count == 2 && !strings.EqualFold(paths[1], two) {
			return errors.New("多文件选择丢失第二个文件")
		}
		return nil
	case "import":
		text, err := service.ImportTheme()
		if err != nil {
			return err
		}
		if test.cancel && text == "" || !test.cancel && text == theme {
			return nil
		}
		return errors.New("导入主题内容或取消结果不符")
	case "export":
		accepted, err := service.ExportTheme(ThemeExportRequest{Filename: "Dialog_Export.json", Text: theme})
		if err != nil {
			return err
		}
		if accepted == test.cancel {
			return errors.New("导出主题确认结果不符")
		}
		if test.cancel {
			return nil
		}
		data, err := os.ReadFile(test.input + ".json")
		if err != nil {
			return err
		}
		if string(data) != theme {
			return errors.New("导出主题默认扩展名或写入内容不符")
		}
		return nil
	case "folder":
		path, err := service.ChooseFolder("project")
		if err != nil {
			return err
		}
		if test.cancel && path == "" || !test.cancel && strings.EqualFold(filepath.Clean(path), folder) {
			return nil
		}
		return fmt.Errorf("文件夹结果不符：%q", path)
	case "recovery":
		key := strings.Repeat("1234ABCD-", 7) + "1234ABCD"
		accepted, err := service.SaveRecoveryKey(key)
		if err != nil {
			return err
		}
		if accepted == test.cancel {
			return errors.New("恢复密钥确认结果不符")
		}
		if test.cancel {
			return nil
		}
		data, err := os.ReadFile(test.input)
		if err != nil {
			return err
		}
		if string(data) != key+"\n" {
			return errors.New("恢复密钥文件内容不符")
		}
		return nil
	}
	return errors.New("未知文件对话框自检工况")
}

func driveNativeFileDialog(ctx context.Context, test fileDialogSelfTest, picture string) (err error) {
	var dialog uintptr
	callback := syscall.NewCallback(func(hwnd, _ uintptr) uintptr {
		var pid uint32
		user32.NewProc("GetWindowThreadProcessId").Call(hwnd, uintptr(unsafe.Pointer(&pid)))
		if pid != uint32(os.Getpid()) {
			return 1
		}
		visible, _, _ := user32.NewProc("IsWindowVisible").Call(hwnd)
		if visible == 0 {
			return 1
		}
		class, title := make([]uint16, 128), make([]uint16, 1024)
		user32.NewProc("GetClassNameW").Call(hwnd, uintptr(unsafe.Pointer(&class[0])), uintptr(len(class)))
		user32.NewProc("GetWindowTextW").Call(hwnd, uintptr(unsafe.Pointer(&title[0])), uintptr(len(title)))
		if windows.UTF16ToString(class) == "#32770" && windows.UTF16ToString(title) == test.title {
			dialog = hwnd
			return 0
		}
		return 1
	})
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for dialog == 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			user32.NewProc("EnumWindows").Call(callback, 0)
		}
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, captureVisibleNativeWindow(dialog, picture))
			user32.NewProc("PostMessageW").Call(dialog, 0x0010, 0, 0)
		}
	}()
	if test.cancel {
		if err = captureVisibleNativeWindow(dialog, picture); err != nil {
			return err
		}
		user32.NewProc("PostMessageW").Call(dialog, 0x0010, 0, 0)
		return nil
	}
	var filename uintptr
	var editParents []string
	childCallback := syscall.NewCallback(func(child, _ uintptr) uintptr {
		class := make([]uint16, 128)
		user32.NewProc("GetClassNameW").Call(child, uintptr(unsafe.Pointer(&class[0])), uintptr(len(class)))
		if windows.UTF16ToString(class) != "Edit" {
			return 1
		}
		id, _, _ := user32.NewProc("GetDlgCtrlID").Call(child)
		if len(editParents) < 20 {
			editParents = append(editParents, fmt.Sprintf("Edit=%d ID=%d", child, id))
		}
		// Open uses EDIT 1148, Save uses 1001 and PickFolder uses 1152. Search
		// edits use 41477, never a generic first Edit match.
		if test.kind == "folder" && id == 1152 || test.kind != "folder" && (id == 1001 || id == 1148) {
			filename = child
			return 0
		}
		return 1
	})
	for filename == 0 {
		editParents = nil
		user32.NewProc("EnumChildWindows").Call(dialog, childCallback, 0)
		if filename != 0 {
			break
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("文件对话框未显示对应的文件名／文件夹 EDIT；实际控件：%v", editParents)
		case <-ticker.C:
		}
	}
	if err := focusNativeDialogControl(ctx, dialog, filename); err != nil {
		return err
	}
	// Exercise normal editing notifications and focus transfer so Save commits
	// its filename model, not just the EDIT's visible text.
	sendMessage.Call(filename, 0x00b1, 0, ^uintptr(0))
	characters := windows.StringToUTF16(test.input)
	foreground, _, _ := getForegroundWindow.Call()
	focused, focusError := nativeDialogControlFocused(dialog, filename)
	if focusError != nil {
		return focusError
	}
	if foreground != dialog || !focused {
		return errors.New("文件对话框失去自有焦点，已拒绝注入文件名")
	}
	keys := make([]nativeKeyboardInput, 0, (len(characters)-1)*2)
	for _, character := range characters[:len(characters)-1] {
		keys = append(keys, nativeKeyboardInput{Type: 1, Scan: character, Flags: 4}, nativeKeyboardInput{Type: 1, Scan: character, Flags: 6})
	}
	if count, _, nativeError := user32.NewProc("SendInput").Call(uintptr(len(keys)), uintptr(unsafe.Pointer(&keys[0])), unsafe.Sizeof(keys[0])); count != uintptr(len(keys)) {
		return fmtNative("文件名 Unicode 按键未完整发送", nativeError)
	}
	readback := make([]uint16, len(characters)+1)
	for {
		user32.NewProc("GetWindowTextW").Call(filename, uintptr(unsafe.Pointer(&readback[0])), uintptr(len(readback)))
		if windows.UTF16ToString(readback) == test.input {
			break
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("文件对话框没有保留实际输入的完整文件名：得到 %q，预期 %q：%w", windows.UTF16ToString(readback), test.input, ctx.Err())
		case <-ticker.C:
		}
	}
	if err = captureVisibleNativeWindow(dialog, picture); err != nil {
		return err
	}
	button, _, _ := user32.NewProc("GetDlgItem").Call(dialog, 1)
	if button == 0 {
		return errors.New("文件对话框缺少确认按钮")
	}
	if err := focusNativeDialogControl(ctx, dialog, button); err != nil {
		return err
	}
	user32.NewProc("PostMessageW").Call(button, 0x00f5, 0, 0)
	for {
		valid, _, _ := isWindow.Call(dialog)
		if valid == 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func focusNativeDialogControl(ctx context.Context, dialog, control uintptr) error {
	user32.NewProc("PostMessageW").Call(dialog, 0x0028, control, 1)
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		focused, err := nativeDialogControlFocused(dialog, control)
		if err != nil {
			return err
		}
		if focused {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("文件对话框未完成实际焦点切换：%w", ctx.Err())
		case <-ticker.C:
		}
	}
}

func nativeDialogControlFocused(dialog, control uintptr) (bool, error) {
	thread, _, _ := user32.NewProc("GetWindowThreadProcessId").Call(dialog, 0)
	var info struct {
		Size, Flags                                        uint32
		Active, Focus, Capture, MenuOwner, MoveSize, Caret uintptr
		CaretBounds                                        winRect
	}
	info.Size = uint32(unsafe.Sizeof(info))
	if result, _, nativeError := user32.NewProc("GetGUIThreadInfo").Call(thread, uintptr(unsafe.Pointer(&info))); result == 0 {
		return false, fmtNative("无法回读文件对话框焦点", nativeError)
	}
	return info.Focus == control, nil
}

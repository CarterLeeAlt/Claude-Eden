package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v3/pkg/application"
	"golang.org/x/sys/windows"
)

// DesktopService is the sole application binding surface. Every operation has a
// concrete request/result contract; paths chosen by the OS stay in the host.
type DesktopService struct {
	shell       *Shell
	selectionMu sync.Mutex
	selections  map[string]selectedFiles
}
type selectedFiles struct {
	paths   []string
	expires time.Time
}

func connectionID(value string) string {
	if value == "" {
		return "local"
	}
	return value
}

func validID(value string) error {
	if _, err := uuid.Parse(value); err != nil {
		return errors.New("文件操作标识无效")
	}
	return nil
}

func (s *DesktopService) saveDialog(title, name, filter, pattern string) *application.SaveFileDialogStruct {
	picker := s.shell.app.Dialog.SaveFile()
	options := application.SaveFileDialogOptions{Title: title, Filename: name, CanCreateDirectories: true, Window: s.shell.window}
	if filter != "" {
		options.Filters = []application.FileFilter{{DisplayName: filter, Pattern: pattern}}
	}
	picker.SetOptions(&options)
	return picker
}

func (s *DesktopService) RemoteInput(request RemoteInputRequest) (RemoteInputResult, error) {
	return application.InvokeSyncWithResultAndError(func() (RemoteInputResult, error) {
		if s.shell.input == nil {
			return RemoteInputResult{}, errors.New("原生输入控件尚未就绪")
		}
		return s.shell.input.request(request, s.shell.window.GetZoom())
	})
}

func (s *DesktopService) WorkbenchReady() (bool, error) {
	if s.shell.controllerNeedsUpdate.Load() {
		return false, errors.New("后台版本尚未同步，更新未完成")
	}
	if err := s.shell.confirmWorkbenchReady(); err != nil {
		s.shell.startupError("无法确认工作台启动成功：" + err.Error())
		return false, err
	}
	s.shell.ready.Store(true)
	if s.shell.transport == nil {
		s.shell.completeSmoke(true, "")
	}
	return true, nil
}

func (s *DesktopService) WorkbenchFailed(message string) (bool, error) {
	if len(message) > 4096 || !utf8.ValidString(message) || strings.TrimSpace(message) == "" || strings.ContainsRune(message, 0) {
		return false, errors.New("工作台启动失败消息必须是 1 至 4096 字节的 UTF-8 纯文本")
	}
	return s.shell.failWorkbench(redactStartupMessage(message))
}

func (s *DesktopService) AppUpdate(operation string) (UpdateState, error) {
	if operation != "status" && operation != "check" && operation != "install" && operation != "cancel" {
		return UpdateState{}, errors.New("更新操作无效")
	}
	if s.shell.options.Update == nil {
		return UpdateState{}, errors.New("应用更新组件尚未配置")
	}
	return s.shell.options.Update(operation)
}

func (s *DesktopService) ExportDiagnostics(ctx context.Context, request ConnectionRequest) (FileResult, error) {
	path, err := s.saveDialog("导出诊断包", "eden-diagnostics.zip", "诊断包 (*.zip)", "*.zip").PromptForSingleSelection()
	if err != nil {
		return FileResult{}, err
	}
	if path == "" {
		return FileResult{Canceled: true}, nil
	}
	var result FileResult
	code, err := s.shell.backend.JSON(ctx, time.Minute, 65536, &result, "diagnostics", "--json", "--connection", connectionID(request.ConnectionID), path)
	if err == nil && code != 0 {
		err = errors.New(result.Message)
	}
	return result, err
}

func readTheme(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 256*1024+1))
	if err != nil {
		return "", err
	}
	if len(data) > 256*1024 {
		return "", errors.New("主题文件不能超过 256 KB")
	}
	if !utf8.Valid(data) {
		return "", errors.New("主题文件不是有效的 UTF-8 文本")
	}
	return strings.TrimPrefix(string(data), "\ufeff"), nil
}

func (s *DesktopService) ImportTheme() (string, error) {
	path, err := s.shell.app.Dialog.OpenFile().AttachToWindow(s.shell.window).SetTitle("导入主题").AddFilter("Claude Eden 主题 (*.json)", "*.json").PromptForSingleSelection()
	if err != nil || path == "" {
		return "", err
	}
	return readTheme(path)
}

var recoveryPattern = regexp.MustCompile(`^[A-F0-9]{8}(-[A-F0-9]{8}){7}$`)

func (s *DesktopService) SaveRecoveryKey(key string) (bool, error) {
	if !recoveryPattern.MatchString(key) {
		return false, errors.New("恢复密钥格式无效")
	}
	path, err := s.saveDialog("保存账户恢复密钥", "账户恢复密钥.txt", "文本文件 (*.txt)", "*.txt").PromptForSingleSelection()
	if err != nil || path == "" {
		return false, err
	}
	if err = os.WriteFile(path, []byte(key+"\n"), 0600); err != nil {
		return false, errors.New("恢复密钥未能保存，请检查保存位置的权限")
	}
	return true, nil
}

func validateTheme(text string) error {
	if len(text) > 256*1024 {
		return errors.New("主题文件不能超过 256 KB")
	}
	var theme struct {
		Format  string `json:"format"`
		Version int    `json:"version"`
	}
	if !utf8.ValidString(text) || json.Unmarshal([]byte(text), &theme) != nil || theme.Format != "eden-theme" || theme.Version != 2 {
		return errors.New("主题文件格式无效")
	}
	return nil
}

func safeFilename(name string) string {
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	name = strings.Map(func(r rune) rune {
		if r < 32 || strings.ContainsRune(`<>:"/\|?*`, r) {
			return '_'
		}
		return r
	}, name)
	name = strings.TrimRight(name, " .")
	if name == "" || name == "." {
		return "download"
	}
	return name
}

func (s *DesktopService) ExportTheme(request ThemeExportRequest) (bool, error) {
	if err := validateTheme(request.Text); err != nil {
		return false, err
	}
	path, err := s.saveDialog("导出主题", safeFilename(request.Filename), "Claude Eden 主题 (*.json)", "*.json").PromptForSingleSelection()
	if err != nil || path == "" {
		return false, err
	}
	err = os.WriteFile(path, []byte(request.Text), 0600)
	return err == nil, err
}

func (s *DesktopService) ChooseFolder(purpose string) (string, error) {
	message := "仅此文件夹会与 Claude Eden 共享。请选择具体项目。"
	if purpose == "environmentStorage" {
		message = "请选择用于保存镜像和虚拟磁盘的空文件夹。"
	}
	return s.shell.app.Dialog.OpenFile().AttachToWindow(s.shell.window).CanChooseFiles(false).CanChooseDirectories(true).CanCreateDirectories(true).SetTitle(message).PromptForSingleSelection()
}

func (s *DesktopService) ClipboardRead() (string, error) {
	return application.InvokeSyncWithResultAndError(s.shell.clipboard.native.readText)
}
func (s *DesktopService) ClipboardRevision() uint32 { return s.shell.clipboard.revision() }
func (s *DesktopService) ClipboardSyncCancel() bool { s.shell.clipboard.generation.Add(1); return true }
func (s *DesktopService) ClipboardSyncRead(ctx context.Context, request ConnectionRequest) (ClipboardContent, error) {
	return s.shell.clipboard.read(ctx, request)
}
func (s *DesktopService) ClipboardSyncWrite(ctx context.Context, request ClipboardWriteRequest) (ClipboardContent, error) {
	return s.shell.clipboard.write(ctx, request)
}
func (s *DesktopService) ClipboardSyncRelease(ctx context.Context, request ClipboardReleaseRequest) (bool, error) {
	if err := validID(request.Files); err != nil {
		return false, err
	}
	_, err := s.shell.clipboard.transfer(ctx, request.ConnectionID, "release", request.Files)
	return err == nil, err
}

func (s *DesktopService) ClipboardWrite(text string) (bool, error) {
	err := application.InvokeSyncWithError(func() error {
		_, _, err := s.shell.clipboard.native.write(clipboardSnapshot{text: text}, nil)
		return err
	})
	return err == nil, err
}

func (s *DesktopService) ClipboardReadContent() (ClipboardContent, error) {
	value, reader, err := s.shell.clipboard.snapshotReader()
	if err != nil {
		return ClipboardContent{}, err
	}
	if value.paths != nil {
		defer func() { s.shell.clipboard.releaseReader(reader); s.shell.clipboard.collect() }()
		selection, err := s.rememberFiles(value.paths)
		return ClipboardContent{FileSelection: selection.FileSelection, Count: selection.Count}, err
	}
	if value.png != nil {
		return encodeClipboardImage(value), nil
	}
	return ClipboardContent{Text: &value.text}, nil
}

func (s *DesktopService) DesktopFileAction(ctx context.Context, request FileActionRequest) (FileResult, error) {
	if err := validID(request.ID); err != nil {
		return FileResult{}, err
	}
	return s.shell.backend.fileCommand(ctx, "desktop-file", "--json", "--connection", connectionID(request.ConnectionID), request.ID)
}

func (s *DesktopService) BrowserDownloadAction(ctx context.Context, request DownloadRequest) (FileResult, error) {
	if err := validID(request.ID); err != nil {
		return FileResult{}, err
	}
	if request.Operation != "save" && request.Operation != "open" && request.Operation != "reveal" {
		return FileResult{}, errors.New("下载操作无效")
	}
	args := []string{"browser-file", "--json", "--connection", connectionID(request.ConnectionID), request.Operation, request.ID}
	if request.Operation == "save" {
		path, err := s.saveDialog("将下载另存为到本机", safeFilename(request.Name), "", "").PromptForSingleSelection()
		if err != nil {
			return FileResult{}, err
		}
		if path == "" {
			return FileResult{Canceled: true}, nil
		}
		args = append(args, path)
	}
	return s.shell.backend.fileCommand(ctx, args...)
}

func (s *DesktopService) AppReference(kind string) (bool, error) {
	var address string
	switch kind {
	case "source":
		address = "https://github.com/CarterLeeAlt/Claude-Eden/commits/main"
	case "releases":
		address = "https://github.com/CarterLeeAlt/Claude-Eden/releases"
	default:
		return false, errors.New("未知的项目页面")
	}
	err := windows.ShellExecute(windows.Handle(uintptr(s.shell.window.NativeWindow())), nil, windows.StringToUTF16Ptr(address), nil, nil, 1)
	return err == nil, err
}

func (s *DesktopService) rememberFiles(paths []string) (FileSelection, error) {
	if len(paths) == 0 || len(paths) > 100 {
		return FileSelection{}, errors.New("请选择 1 至 100 个文件")
	}
	id := uuid.NewString()
	now := time.Now()
	expires := now.Add(10 * time.Minute)
	s.shell.clipboard.retainReader(id, paths, expires)
	accepted := false
	defer func() {
		if !accepted {
			s.shell.clipboard.releaseReader(id)
		}
	}()
	for _, path := range paths {
		if !filepath.IsAbs(path) || strings.HasPrefix(path, `\\`) {
			return FileSelection{}, errors.New("请选择本机文件")
		}
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return FileSelection{}, errors.New("请选择普通文件，暂不支持文件夹或链接")
		}
		attributes, err := windows.GetFileAttributes(windows.StringToUTF16Ptr(path))
		if err != nil || attributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
			return FileSelection{}, errors.New("请选择普通文件，暂不支持文件夹或链接")
		}
		if info.Size() > 2<<30 {
			return FileSelection{}, errors.New("每个文件不能超过 2 GB")
		}
	}
	s.selectionMu.Lock()
	defer s.selectionMu.Unlock()
	for key, selection := range s.selections {
		if !selection.expires.After(now) {
			delete(s.selections, key)
			s.shell.clipboard.releaseReader(key)
		}
	}
	if len(s.selections) >= 16 {
		var oldest string
		var expires time.Time
		for key, value := range s.selections {
			if oldest == "" || value.expires.Before(expires) {
				oldest, expires = key, value.expires
			}
		}
		delete(s.selections, oldest)
		s.shell.clipboard.releaseReader(oldest)
	}
	s.selections[id] = selectedFiles{append([]string{}, paths...), expires}
	accepted = true
	return FileSelection{FileSelection: id, Count: len(paths)}, nil
}

func (s *DesktopService) ChooseUploadFiles(multiple bool) (FileSelection, error) {
	picker := s.shell.app.Dialog.OpenFile().AttachToWindow(s.shell.window).SetTitle("选择本机文件")
	var paths []string
	var err error
	if multiple {
		paths, err = picker.PromptForMultipleSelection()
	} else {
		var path string
		path, err = picker.PromptForSingleSelection()
		if path != "" {
			paths = []string{path}
		}
	}
	if err != nil {
		return FileSelection{}, err
	}
	if len(paths) == 0 {
		return FileSelection{Canceled: true}, nil
	}
	return s.rememberFiles(paths)
}

var terminalUploadPattern = regexp.MustCompile(`^[a-f0-9]{16}$`)

func (s *DesktopService) UploadHostFiles(ctx context.Context, request UploadRequest) (FileResult, error) {
	if request.Target != "browser" && request.Target != "desktop" && request.Target != "terminal" || (request.Target == "terminal" && !terminalUploadPattern.MatchString(request.ID)) || (request.Target != "terminal" && validID(request.ID) != nil) {
		return FileResult{}, errors.New("文件上传目标无效")
	}
	s.selectionMu.Lock()
	selection, ok := s.selections[request.Selection]
	delete(s.selections, request.Selection)
	if !ok {
		s.selectionMu.Unlock()
		return FileResult{}, errors.New("文件选择已过期，请重新复制或选择文件")
	}
	if !s.shell.clipboard.promoteReader(request.Selection, time.Now()) {
		s.shell.clipboard.releaseReader(request.Selection)
		s.selectionMu.Unlock()
		return FileResult{}, errors.New("文件选择已过期，请重新复制或选择文件")
	}
	// Once consumed, the selection remains pinned for the entire upload rather
	// than expiring while its independent backend helper is reading the files.
	s.selectionMu.Unlock()
	defer func() { s.shell.clipboard.releaseReader(request.Selection); s.shell.clipboard.collect() }()
	args := append([]string{"upload-files", "--json", "--connection", connectionID(request.ConnectionID), request.Target, request.ID}, selection.paths...)
	return s.shell.backend.fileCommand(ctx, args...)
}

func (s *DesktopService) Appearance(request AppearanceRequest) (bool, error) {
	err := s.shell.appearance(request)
	return err == nil, err
}
func (s *DesktopService) AccountDevice(operation string) (DeviceStatus, error) {
	if operation != "status" {
		return DeviceStatus{}, errors.New("此平台暂不支持快捷登录")
	}
	return DeviceStatus{Supported: false}, nil
}
func (s *DesktopService) RefreshSession(ctx context.Context) (SessionResult, error) {
	options := []string{"desktop", "--json"}
	if s.shell.controllerNeedsUpdate.Load() {
		options = append(options, "--existing")
	}
	var reply bootstrapReply
	code, err := s.shell.backend.JSON(ctx, 0, 65536, &reply, options...)
	if err != nil {
		return SessionResult{}, err
	}
	if code != 0 {
		return SessionResult{}, errors.New("工作台会话恢复失败")
	}
	entry, err := WorkbenchEntry(reply.URL)
	if err != nil {
		return SessionResult{}, err
	}
	return SessionResult{URL: entry.String()}, nil
}
func (s *DesktopService) Reconnect() bool { go s.shell.bootstrap(""); return true }
func (s *DesktopService) QuitNow() bool {
	s.shell.Quit()
	return true
}

package desktop

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// This failure-only VM diagnostic records window structure, not window titles,
// text, clipboard data or full process paths. It cannot inject input or focus.
func writeIMEWindowDiagnostics(input *nativeInput, output string, report *NativeSelfTestReport) (bool, error) {
	var thread struct {
		Size, Flags                                        uint32
		Active, Focus, Capture, MenuOwner, MoveSize, Caret uintptr
		CaretBounds                                        winRect
	}
	thread.Size = uint32(unsafe.Sizeof(thread))
	threadID, _, _ := kernel32.NewProc("GetCurrentThreadId").Call()
	threadResult, _, threadError := user32.NewProc("GetGUIThreadInfo").Call(threadID, uintptr(unsafe.Pointer(&thread)))
	var threadErrorText string
	if threadResult == 0 {
		threadErrorText = threadError.Error()
	}
	var client winRect
	var inputBounds winRect
	user32.NewProc("GetClientRect").Call(input.hwnd, uintptr(unsafe.Pointer(&client)))
	user32.NewProc("GetWindowRect").Call(input.hwnd, uintptr(unsafe.Pointer(&inputBounds)))
	style, _, _ := user32.NewProc("GetWindowLongPtrW").Call(input.hwnd, ^uintptr(15))
	type windowRecord struct {
		Handle       uintptr `json:"handle"`
		Class        string  `json:"class"`
		PID          uint32  `json:"pid"`
		Image        string  `json:"image,omitempty"`
		ImageError   string  `json:"imageError,omitempty"`
		CaptureError string  `json:"captureError,omitempty"`
		Visible      bool    `json:"visible"`
		Bounds       winRect `json:"bounds"`
	}
	record := func(hwnd uintptr) windowRecord {
		value := windowRecord{Handle: hwnd}
		class := make([]uint16, 256)
		user32.NewProc("GetClassNameW").Call(hwnd, uintptr(unsafe.Pointer(&class[0])), uintptr(len(class)))
		value.Class = windows.UTF16ToString(class)
		visible, _, _ := user32.NewProc("IsWindowVisible").Call(hwnd)
		value.Visible = visible != 0
		user32.NewProc("GetWindowRect").Call(hwnd, uintptr(unsafe.Pointer(&value.Bounds)))
		user32.NewProc("GetWindowThreadProcessId").Call(hwnd, uintptr(unsafe.Pointer(&value.PID)))
		process, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, value.PID)
		if err != nil {
			value.ImageError = err.Error()
		} else {
			defer windows.CloseHandle(process)
			name := make([]uint16, 32768)
			length := uint32(len(name))
			if err := windows.QueryFullProcessImageName(process, 0, &name[0], &length); err != nil {
				value.ImageError = err.Error()
			} else {
				value.Image = filepath.Base(windows.UTF16ToString(name[:length]))
			}
		}
		return value
	}
	var enumerated, desktop []windowRecord
	var candidateListCount uint32
	var candidateBytes uintptr
	composition := ""
	context, _, _ := immGetContext.Call(input.hwnd)
	if context != 0 {
		composition = compositionText(context, 8)
		candidateBytes, _, _ = imm32.NewProc("ImmGetCandidateListCountW").Call(context, uintptr(unsafe.Pointer(&candidateListCount)))
		immReleaseContext.Call(input.hwnd, context)
	}
	hostCaptures := 0
	var modernHost uintptr
	callback := syscall.NewCallback(func(hwnd, unused uintptr) uintptr {
		value := record(hwnd)
		if value.Visible && value.Class == "Windows.UI.Core.CoreWindow" && value.Image == "TextInputHost.exe" {
			modernHost = hwnd
			hostCaptures++
			name := fmt.Sprintf("Native_IME_Host_%02d.png", hostCaptures)
			if err := captureNativeWindow(hwnd, filepath.Join(output, name)); err != nil {
				value.CaptureError = err.Error()
			} else {
				report.Screenshots = append(report.Screenshots, name)
			}
		}
		if value.Visible {
			enumerated = append(enumerated, value)
		}
		return 1
	})
	user32.NewProc("EnumWindows").Call(callback, 0)
	top, _, _ := user32.NewProc("GetTopWindow").Call(0)
	seen := make(map[uintptr]bool)
	for hwnd := top; hwnd != 0 && len(seen) < 512 && !seen[hwnd]; {
		seen[hwnd] = true
		value := record(hwnd)
		if value.Visible {
			desktop = append(desktop, value)
		}
		hwnd, _, _ = user32.NewProc("GetWindow").Call(hwnd, 2)
	}
	data, err := json.MarshalIndent(struct {
		CaretBounds        winRect        `json:"caretBounds"`
		ClientBounds       winRect        `json:"clientBounds"`
		EditStyle          uintptr        `json:"editStyle"`
		GUIThreadError     string         `json:"guiThreadError,omitempty"`
		Composition        string         `json:"composition"`
		CandidateListCount uint32         `json:"candidateListCount"`
		CandidateBytes     uintptr        `json:"candidateBytes"`
		EnumWindows        []windowRecord `json:"enumWindows"`
		DesktopZOrder      []windowRecord `json:"desktopZOrder"`
	}{thread.CaretBounds, client, style, threadErrorText, composition, candidateListCount, candidateBytes, enumerated, desktop}, "", "  ")
	if err != nil {
		return false, err
	}
	if err := os.WriteFile(filepath.Join(output, "Native_IME_Windows.json"), data, 0600); err != nil {
		return false, err
	}
	if modernHost != 0 {
		visible, err := diagnoseModernIMEUI(modernHost, inputBounds, output)
		if err != nil {
			return false, err
		}
		if _, err := os.Stat(filepath.Join(output, "Native_IME_UIA.png")); err == nil {
			report.Screenshots = append(report.Screenshots, "Native_IME_UIA.png")
		} else if !os.IsNotExist(err) {
			return false, err
		}
		return visible, nil
	}
	return false, nil
}

func diagnoseModernIMEUI(hwnd uintptr, bounds winRect, output string) (bool, error) {
	program := filepath.Join(output, "Native_IME_UIA.ps1")
	if err := os.WriteFile(program, append([]byte{0xef, 0xbb, 0xbf}, []byte(imeUIADiagnosticScript)...), 0600); err != nil {
		return false, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe"), "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-STA", "-File", program, "-Handle", strconv.FormatUint(uint64(hwnd), 10), "-Left", strconv.Itoa(int(bounds.Left)), "-Bottom", strconv.Itoa(int(bounds.Bottom)), "-Output", filepath.Join(output, "Native_IME_UIA.json"))
	command.Dir = output
	hideProcess(command)
	log := &boundedOutput{limit: 16384}
	command.Stdout, command.Stderr = log, log
	if err := command.Start(); err != nil {
		return false, err
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	for {
		select {
		case err := <-done:
			if err != nil {
				return false, fmt.Errorf("现代 IME UI Automation 诊断失败：%w：%s", err, log.String())
			}
			data, err := os.ReadFile(filepath.Join(output, "Native_IME_UIA.json"))
			if err != nil {
				return false, err
			}
			var result struct {
				CandidateVisible bool `json:"candidateVisible"`
			}
			if err := json.Unmarshal(data, &result); err != nil {
				return false, err
			}
			return result.CandidateVisible, nil
		default:
			pumpNativeMessages(20 * time.Millisecond)
		}
	}
}

const imeUIADiagnosticScript = `param([long]$Handle, [int]$Left, [int]$Bottom, [string]$Output)
$ErrorActionPreference = 'Stop'
[Console]::InputEncoding = [Text.UTF8Encoding]::new($false)
[Console]::OutputEncoding = [Text.UTF8Encoding]::new($false)
$OutputEncoding = [Console]::OutputEncoding
if ($env:COMPUTERNAME -ne 'EDEN-WINTEST' -or $env:EDEN_NATIVE_SELF_TEST -ne 'vm') { throw 'VM-only UI Automation diagnostic.' }
Add-Type -AssemblyName UIAutomationClient
Add-Type -AssemblyName UIAutomationTypes
Add-Type -AssemblyName WindowsBase
$root = [Windows.Automation.AutomationElement]::FromHandle([IntPtr]$Handle)
$elements = $root.FindAll([Windows.Automation.TreeScope]::Subtree,[Windows.Automation.Condition]::TrueCondition)
$items = @()
$candidate = $null
for ($index=0; $index -lt [Math]::Min(800,$elements.Count); $index++) {
  $current = $elements[$index].Current
  $name = if ($current.Name -match '你好|ni.hao|拼音|候选|Pinyin|Candidate|Microsoft Text Input') { $current.Name } else { '[not-collected]' }
  $rect = $current.BoundingRectangle
  $items += @{name=$name;class=$current.ClassName;id=$current.AutomationId;type=$current.ControlType.ProgrammaticName;offscreen=$current.IsOffscreen;left=$rect.Left;top=$rect.Top;width=$rect.Width;height=$rect.Height}
  if (!$candidate -and !$current.IsOffscreen -and $current.Name -match '你好' -and $rect.Width -gt 0 -and $rect.Height -gt 0 -and $rect.Width -lt 1000 -and $rect.Height -lt 600) { $candidate=$rect }
}
# Modern IME candidates can live in a separate UI band, outside the host HWND's
# automation subtree. Probe only the controlled caret's immediate candidate
# area, and require the expected test phrase in a visible Microsoft IME element.
$hits = @()
$visited = [Collections.Generic.HashSet[string]]::new()
function Get-CandidateRectangle($Element, [string]$Process) {
  if ($Process -notin @('ChsIME','TextInputHost')) { return }
  $value = $Element.Current
  if ($value.IsOffscreen -or $value.Name -notmatch '你好') { return }
  $box = $value.BoundingRectangle
  if ($box.Width -le 0 -or $box.Height -le 0 -or $box.Width -ge 1000 -or $box.Height -ge 600) { return }
  if ([Math]::Abs($box.Left-$Left) -gt 500 -or [Math]::Abs($box.Top-$Bottom) -gt 200) { return }
  return $box
}
foreach ($dx in @(40,140,240)) {
  foreach ($dy in @(-40,0,40,80)) {
    $node = [Windows.Automation.AutomationElement]::FromPoint([Windows.Point]::new($Left+$dx,$Bottom+$dy))
    for ($depth=0; $node -and $depth -lt 8; $depth++) {
      $current = $node.Current
      if ($current.ProcessId -le 0) { break }
      $identity = $node.GetRuntimeId() -join '.'
      if (!$visited.Add($identity)) { break }
      $process = [Diagnostics.Process]::GetProcessById($current.ProcessId).ProcessName
      $rect = $current.BoundingRectangle
      $name = if ($current.Name -match '你好|ni.hao|拼音|候选|Pinyin|Candidate') { $current.Name } else { '[not-collected]' }
      $hits += @{name=$name;process=$process;class=$current.ClassName;type=$current.ControlType.ProgrammaticName;offscreen=$current.IsOffscreen;left=$rect.Left;top=$rect.Top;width=$rect.Width;height=$rect.Height}
      $candidate = Get-CandidateRectangle $node $process
      if ($candidate) { break }
      if ($process -in @('ChsIME','TextInputHost')) {
        $children = $node.FindAll([Windows.Automation.TreeScope]::Subtree,[Windows.Automation.Condition]::TrueCondition)
        for ($child=0; $child -lt [Math]::Min(200,$children.Count); $child++) {
          $candidate = Get-CandidateRectangle $children[$child] $process
          if ($candidate) { break }
        }
      }
      if ($candidate) { break }
      $node = [Windows.Automation.TreeWalker]::RawViewWalker.GetParent($node)
    }
    if ($candidate) { break }
  }
  if ($candidate) { break }
}
if ($candidate) {
  Add-Type -AssemblyName System.Drawing
  $bitmap = [Drawing.Bitmap]::new([int][Math]::Ceiling($candidate.Width),[int][Math]::Ceiling($candidate.Height))
  try {
    $graphics = [Drawing.Graphics]::FromImage($bitmap)
    try { $graphics.CopyFromScreen([int]$candidate.Left,[int]$candidate.Top,0,0,$bitmap.Size) } finally { $graphics.Dispose() }
    $bitmap.Save((Join-Path (Split-Path $Output -Parent) 'Native_IME_UIA.png'),[Drawing.Imaging.ImageFormat]::Png)
  } finally { $bitmap.Dispose() }
}
[IO.File]::WriteAllText($Output,(@{count=$elements.Count;items=$items;hits=$hits;candidateVisible=($null -ne $candidate)} | ConvertTo-Json -Depth 6),[Text.UTF8Encoding]::new($false))
`

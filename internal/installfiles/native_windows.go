package installfiles

import (
	"errors"
	"os"
	"runtime"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

var ntCreateFile = windows.NewLazySystemDLL("ntdll.dll").NewProc("NtCreateFile")
var ntStatusToError = windows.NewLazySystemDLL("ntdll.dll").NewProc("RtlNtStatusToDosError")
var setFileInformation = windows.NewLazySystemDLL("kernel32.dll").NewProc("SetFileInformationByHandle")
var getFileInformation = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetFileInformationByHandleEx")

type unicodeString struct {
	Length, MaximumLength uint16
	Buffer                *uint16
}
type objectAttributes struct {
	Length                                       uint32
	RootDirectory                                windows.Handle
	ObjectName                                   *unicodeString
	Attributes                                   uint32
	SecurityDescriptor, SecurityQualityOfService uintptr
}
type ioStatus struct{ Status, Information uintptr }
type attributeTag struct{ Attributes, Tag uint32 }

func isReparse(name string) (bool, error) {
	p, e := windows.UTF16PtrFromString(name)
	if e != nil {
		return false, e
	}
	a, e := windows.GetFileAttributes(p)
	if e != nil {
		return false, e
	}
	return a&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0, nil
}

// OpenReadLocked permits readers only, preventing writes or replacement until
// the caller closes the handle (including during Authenticode verification).
func OpenReadLocked(name string) (*os.File, error) {
	if e := PlainPath(name); e != nil {
		return nil, e
	}
	p, e := windows.UTF16PtrFromString(name)
	if e != nil {
		return nil, e
	}
	h, e := windows.CreateFile(p, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if e != nil {
		return nil, e
	}
	if e = checkHandle(h, false); e != nil {
		windows.CloseHandle(h)
		return nil, e
	}
	return os.NewFile(uintptr(h), name), nil
}
func checkHandle(h windows.Handle, directory bool) error {
	var tag attributeTag
	ok, _, e := getFileInformation.Call(uintptr(h), 9, uintptr(unsafe.Pointer(&tag)), unsafe.Sizeof(tag))
	if ok == 0 {
		return e
	}
	if tag.Attributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 || (!directory && tag.Attributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0) {
		return errors.New("安装文件是目录或文件系统链接")
	}
	return nil
}

func OpenVerified(root, relative string) (*os.File, error) {
	if !ValidRelative(relative) {
		return nil, errors.New("安装文件相对路径无效")
	}
	if e := PlainPath(root); e != nil {
		return nil, e
	}
	p, e := windows.UTF16PtrFromString(root)
	if e != nil {
		return nil, e
	}
	dir, e := windows.CreateFile(p, windows.GENERIC_READ, 7, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if e != nil {
		return nil, e
	}
	defer windows.CloseHandle(dir)
	if e = checkHandle(dir, true); e != nil {
		return nil, e
	}
	text, e := windows.UTF16FromString(strings.ReplaceAll(relative, "/", "\\"))
	if e != nil {
		return nil, e
	}
	if len(text) > 32767 {
		return nil, errors.New("安装文件路径过长")
	}
	name := unicodeString{uint16((len(text) - 1) * 2), uint16((len(text) - 1) * 2), &text[0]}
	attributes := objectAttributes{Length: uint32(unsafe.Sizeof(objectAttributes{})), RootDirectory: dir, ObjectName: &name, Attributes: 0x1040} // OBJ_CASE_INSENSITIVE | OBJ_DONT_REPARSE
	var status ioStatus
	var h windows.Handle
	result, _, _ := ntCreateFile.Call(uintptr(unsafe.Pointer(&h)), 0x80110000, uintptr(unsafe.Pointer(&attributes)), uintptr(unsafe.Pointer(&status)), 0, 0, 5, 1, 0x00200060, 0, 0)
	runtime.KeepAlive(text)
	if int32(result) < 0 {
		if h != 0 && h != windows.InvalidHandle {
			windows.CloseHandle(h)
		}
		code, _, _ := ntStatusToError.Call(result)
		return nil, windows.Errno(code)
	}
	if e = checkHandle(h, false); e != nil {
		windows.CloseHandle(h)
		return nil, e
	}
	return os.NewFile(uintptr(h), root+"/"+relative), nil
}

func DeleteVerified(f *os.File) error {
	disposition := byte(1)
	ok, _, e := setFileInformation.Call(f.Fd(), 4, uintptr(unsafe.Pointer(&disposition)), 1)
	if ok == 0 {
		return e
	}
	return nil
}
func renameNoReplace(from, to string) error {
	a, e := windows.UTF16PtrFromString(from)
	if e != nil {
		return e
	}
	b, e := windows.UTF16PtrFromString(to)
	if e != nil {
		return e
	}
	return windows.MoveFile(a, b)
}
func replaceFile(from, to string) error {
	a, e := windows.UTF16PtrFromString(from)
	if e != nil {
		return e
	}
	b, e := windows.UTF16PtrFromString(to)
	if e != nil {
		return e
	}
	return windows.MoveFileEx(a, b, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH)
}

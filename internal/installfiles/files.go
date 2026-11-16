// Package installfiles owns installation manifests, personal-file transfers and
// verified-handle deletion. Both NSIS and update recovery use this implementation.
package installfiles

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
)

const ManifestName = "Eden.install-files.json"
const IdentityName = "Eden.install-id"
const Identity = "io.github.carterleealt.claude-eden"
const maxRecord = 16 << 20

var ErrReparsePath = errors.New("安装路径包含符号链接或目录联接")

type File struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}
type Manifest struct {
	Version int    `json:"version"`
	Files   []File `json:"files"`
}
type TransferFile struct {
	Path        string `json:"path"`
	SHA256      string `json:"sha256"`
	Destination string `json:"destination,omitempty"`
}
type Move struct {
	Path        string `json:"path"`
	Destination string `json:"destination"`
	Directory   bool   `json:"directory"`
	State       string `json:"state"`
}
type Receipt struct {
	Kind        string         `json:"kind"`
	Version     int            `json:"version"`
	InstallPath string         `json:"installPath"`
	StagePath   string         `json:"stagePath"`
	BackupPath  string         `json:"backupPath"`
	Files       []TransferFile `json:"files"`
	Directories []string       `json:"directories"`
	Moves       []Move         `json:"moves"`
	KeepInPlace []string       `json:"keepInPlace"`
}
type Options struct {
	Operation                                              string
	Target, Source, Receipt, Manifest, Backup, Diagnostics string
	LegacyTarget                                           bool
	KeepInPlace                                            []string
	Output                                                 io.Writer
}

func token() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}

// ValidRelative excludes Windows alternate streams, separators, traversal and
// ambiguous Win32 names even when validation runs on a different platform.
func ValidRelative(s string) bool {
	if s == "" || strings.ContainsAny(s, "\\:\x00\r\n\"<>|?*") || strings.HasPrefix(s, "/") {
		return false
	}
	for _, part := range strings.Split(s, "/") {
		if part == "" || part == "." || part == ".." || strings.TrimRight(part, " .") != part {
			return false
		}
		base := strings.ToUpper(strings.SplitN(part, ".", 2)[0])
		if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" || (len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '1' && base[3] <= '9') {
			return false
		}
	}
	return true
}

// PlainPath rejects every existing linked ancestor, including dangling links.
func PlainPath(raw string) error {
	full, err := filepath.Abs(raw)
	if err != nil {
		return err
	}
	for {
		linked, e := isReparse(full)
		if e != nil && !errors.Is(e, os.ErrNotExist) {
			return e
		}
		if linked {
			return ErrReparsePath
		}
		parent := filepath.Dir(full)
		if parent == full {
			return nil
		}
		full = parent
	}
}

func readJSON(name string, value any) error {
	if err := PlainPath(name); err != nil {
		return err
	}
	f, err := OpenReadLocked(name)
	if err != nil {
		return err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, maxRecord+1))
	if err != nil {
		return err
	}
	if len(b) > maxRecord {
		return errors.New("安装记录超过允许大小")
	}
	return json.Unmarshal([]byte(strings.TrimPrefix(string(b), "\ufeff")), value)
}

// ReadManifest returns no ownership for an absent, linked or invalid manifest.
func ReadManifest(name string) map[string]File {
	result := map[string]File{}
	var m Manifest
	if readJSON(name, &m) != nil || m.Version != 1 {
		return result
	}
	for _, f := range m.Files {
		key := strings.ToLower(f.Path)
		_, duplicate := result[key]
		digest, err := hex.DecodeString(f.SHA256)
		if !ValidRelative(f.Path) || err != nil || len(digest) != sha256.Size || duplicate {
			return map[string]File{}
		}
		f.SHA256 = strings.ToLower(f.SHA256)
		result[key] = f
	}
	return result
}

func readReceipt(name string) (*Receipt, error) {
	var r Receipt
	if err := readJSON(name, &r); err != nil {
		return nil, err
	}
	if r.Kind != "eden-installation" || r.Version != 1 {
		return nil, errors.New("安装事务回执无效，原文件已保留")
	}
	seen := map[string]bool{}
	for _, m := range r.Moves {
		key := strings.ToLower(m.Path)
		if !ValidRelative(m.Path) || !ValidRelative(m.Destination) || seen[key] {
			return nil, errors.New("安装事务包含无效的个人文件路径")
		}
		seen[key] = true
		switch m.State {
		case "prepared", "moving", "moved", "absent", "failed", "restored":
		default:
			return nil, errors.New("安装事务状态无效")
		}
	}
	for _, f := range r.Files {
		digest, err := hex.DecodeString(f.SHA256)
		if !ValidRelative(f.Path) || (f.Destination != "" && !ValidRelative(f.Destination)) || err != nil || len(digest) != 32 {
			return nil, errors.New("安装事务文件归属无效")
		}
	}
	for _, d := range r.Directories {
		if !ValidRelative(d) {
			return nil, errors.New("安装事务目录无效")
		}
	}
	for _, k := range r.KeepInPlace {
		if !ValidRelative(k) || strings.Contains(k, "/") {
			return nil, errors.New("保留目录无效")
		}
	}
	return &r, nil
}

func writeJSON(name string, value any, replace bool) error {
	if err := PlainPath(name); err != nil {
		return err
	}
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	tmp := name + "." + token() + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer os.Remove(tmp)
	_, err = f.Write(b)
	if err == nil {
		err = f.Sync()
	}
	err = errors.Join(err, f.Close())
	if err != nil {
		return err
	}
	if replace {
		return replaceFile(tmp, name)
	}
	return renameNoReplace(tmp, name)
}

func Hash(stream io.ReadSeeker) (string, error) {
	if _, err := stream.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	h := sha256.New()
	_, err := io.Copy(h, stream)
	_, reset := stream.Seek(0, io.SeekStart)
	return hex.EncodeToString(h.Sum(nil)), errors.Join(err, reset)
}
func fileHash(name string) (string, error) {
	if err := PlainPath(name); err != nil {
		return "", err
	}
	f, err := OpenReadLocked(name)
	if err != nil {
		return "", err
	}
	defer f.Close()
	return Hash(f)
}

// RemoveMatchingFile hashes and deletes the exact same native file object. A
// renamed replacement is not deleted, and relative lookup refuses reparses.
func RemoveMatchingFile(root, relative, digest string) error {
	if !ValidRelative(relative) {
		return errors.New("安装文件相对路径无效")
	}
	f, err := OpenVerified(root, relative)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	actual, err := Hash(f)
	if err != nil {
		return err
	}
	if !strings.EqualFold(actual, digest) {
		return nil
	}
	return DeleteVerified(f)
}

func exists(name string) bool { _, err := os.Lstat(name); return err == nil }
func same(a, b string) bool   { return strings.EqualFold(filepath.Clean(a), filepath.Clean(b)) }
func contains(a []string, value string) bool {
	for _, s := range a {
		if strings.EqualFold(s, value) {
			return true
		}
	}
	return false
}
func directories(files map[string]File) map[string]bool {
	result := map[string]bool{"": true}
	for _, f := range files {
		for parent := path.Dir(f.Path); parent != "."; parent = path.Dir(parent) {
			result[strings.ToLower(parent)] = true
		}
	}
	return result
}
func destination(target, recovery, relative string, quarantine bool) string {
	conflict := quarantine || exists(filepath.Join(target, filepath.FromSlash(relative)))
	for parent := path.Dir(relative); parent != "."; parent = path.Dir(parent) {
		info, err := os.Lstat(filepath.Join(target, filepath.FromSlash(parent)))
		if err == nil && (!info.IsDir() || info.Mode()&os.ModeSymlink != 0) {
			conflict = true
		}
	}
	if conflict {
		return recovery + "/" + relative
	}
	return relative
}

func Run(ctx context.Context, o Options) (err error) {
	phase, blocked := "initializing", ""
	defer func() {
		if err != nil && o.Diagnostics != "" && !exists(o.Diagnostics) {
			// Diagnostics classify the failure without recording paths or messages.
			type diagnosticError struct {
				Type    string `json:"type"`
				HResult int32  `json:"hresult"`
			}
			chain := []diagnosticError{}
			for cause := err; cause != nil && len(chain) < 4; cause = errors.Unwrap(cause) {
				var native syscall.Errno
				var hresult int32
				if errors.As(cause, &native) {
					hresult = int32(0x80070000 | uint32(native))
				}
				chain = append(chain, diagnosticError{fmt.Sprintf("%T", cause), hresult})
			}
			_ = writeJSON(o.Diagnostics, struct {
				Operation   string            `json:"operation"`
				Phase       string            `json:"phase"`
				Blocked     string            `json:"blocked,omitempty"`
				PointerSize int               `json:"pointerSize"`
				Errors      []diagnosticError `json:"errors"`
			}{o.Operation, phase, blocked, strconv.IntSize / 8, chain}, false)
		}
	}()
	if o.Target == "" {
		return errors.New("缺少安装目标")
	}
	switch o.Operation {
	case "prepare", "complete", "detach", "move", "restore", "cleanup":
		if o.Receipt == "" {
			return errors.New("缺少安装事务回执路径")
		}
	case "uninstall":
		if o.Manifest == "" {
			return errors.New("缺少卸载文件清单")
		}
	case "attach":
	default:
		return errors.New("安装文件操作无效")
	}
	switch o.Operation {
	case "detach", "attach", "move", "restore", "cleanup":
		if o.Source == "" {
			return errors.New("缺少安装源目录")
		}
	}
	o.Target, err = filepath.Abs(o.Target)
	if err != nil {
		return err
	}
	if same(filepath.Dir(o.Target), o.Target) {
		return errors.New("请选择独立的程序目录")
	}
	for _, root := range []string{o.Target, o.Source, o.Receipt, o.Manifest, o.Backup, o.Diagnostics} {
		if root != "" {
			if e := PlainPath(root); e != nil {
				return e
			}
		}
	}
	if o.Source != "" {
		o.Source, err = filepath.Abs(o.Source)
		if err != nil {
			return err
		}
		if same(o.Source, o.Target) {
			return errors.New("安装源目录和目标目录不能相同")
		}
		source, target := strings.ToLower(filepath.Clean(o.Source)), strings.ToLower(filepath.Clean(o.Target))
		if filepath.Dir(o.Source) == o.Source || strings.HasPrefix(source, target+string(filepath.Separator)) || strings.HasPrefix(target, source+string(filepath.Separator)) {
			return errors.New("安装源目录和目标目录不能重叠或使用盘符根目录")
		}
	}
	for _, k := range o.KeepInPlace {
		if !ValidRelative(k) || strings.Contains(k, "/") {
			return errors.New("保留目录必须是顶层目录名称")
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	switch o.Operation {
	case "prepare":
		phase = "reading_new_manifest"
		return prepare(ctx, o, &phase)
	case "move", "restore":
		phase = "moving_preserved_files"
		return transfer(ctx, o)
	case "cleanup":
		phase = "verifying_previous_file_ownership"
		return cleanup(ctx, o)
	case "complete":
		phase = "completing_transaction_receipt"
		return complete(o)
	case "detach", "attach":
		phase = "detaching_previous_application"
		keep := []string{}
		if o.Operation == "detach" {
			r, e := readReceipt(o.Receipt)
			if e != nil {
				return e
			}
			keep = r.KeepInPlace
			if exists(o.Target) {
				return errors.New("备份目录已存在，原安装目录已保留")
			}
		} else {
			phase = "attaching_application"
		}
		if o.Source == "" {
			return errors.New("缺少待发布的程序目录")
		}
		if err := moveTopLevel(ctx, o.Source, o.Target, keep, o.Operation == "detach", &blocked); err != nil {
			return err
		}
		if o.Operation == "attach" {
			return os.Remove(o.Source)
		}
		return nil
	case "uninstall":
		phase = "deleting_verified_program_files"
		return uninstall(ctx, o)
	default:
		return errors.New("安装文件操作无效")
	}
}

func prepare(ctx context.Context, o Options, phase *string) error {
	if exists(o.Receipt) {
		return errors.New("仍有未完成的安装事务，请保留并检查回执：" + o.Receipt)
	}
	current := ReadManifest(filepath.Join(o.Target, ManifestName))
	legacy := len(current) == 0 && o.LegacyTarget
	if legacy {
		h, e := fileHash(filepath.Join(o.Target, IdentityName))
		if e != nil {
			return e
		}
		current[strings.ToLower(IdentityName)] = File{IdentityName, h}
	}
	if len(current) == 0 {
		return errors.New("新程序缺少有效文件清单，安装已停止")
	}
	*phase = "reading_new_uninstaller"
	h, e := fileHash(filepath.Join(o.Target, "Uninstall.exe"))
	if e != nil {
		return e
	}
	current["uninstall.exe"] = File{"Uninstall.exe", h}
	r := Receipt{Kind: "eden-installation", Version: 1, InstallPath: o.Source, StagePath: o.Target, BackupPath: o.Backup, Files: []TransferFile{}, Directories: []string{}, Moves: []Move{}, KeepInPlace: o.KeepInPlace}
	recovery := "Eden-preserved-files/" + token()
	programDirs := directories(current)
	if o.Source != "" && exists(o.Source) {
		*phase = "scanning_previous_installation"
		previous := ReadManifest(filepath.Join(o.Source, ManifestName))
		previousDirs := directories(previous)
		pending := []string{""}
		for len(pending) > 0 {
			if e := ctx.Err(); e != nil {
				return e
			}
			dir := pending[0]
			pending = pending[1:]
			entries, e := os.ReadDir(filepath.Join(o.Source, filepath.FromSlash(dir)))
			if e != nil {
				return e
			}
			for _, entry := range entries {
				relative := path.Join(dir, entry.Name())
				if contains(o.KeepInPlace, relative) {
					continue
				}
				if !ValidRelative(relative) {
					return errors.New("已有安装含无法安全保留的文件名称")
				}
				key := strings.ToLower(relative)
				source := filepath.Join(o.Source, filepath.FromSlash(relative))
				link, e := isReparse(source)
				if e != nil {
					return e
				}
				_, owned := previous[key]
				ext := strings.ToLower(path.Ext(relative))
				programContent := owned || (programDirs[strings.ToLower(strings.TrimSuffix(path.Dir(relative), "."))] && (contains([]string{".dll", ".exe", ".ocx", ".sys", ".pyd", ".so", ".dylib"}, ext) || strings.EqualFold(entry.Name(), "node_modules")))
				hasOwnedChildren := previousDirs[key]
				if legacy && len(previous) > 0 && !programContent && !hasOwnedChildren && !contains([]string{ManifestName, "Eden-preserved-files"}, relative) {
					old := filepath.Join(o.Target, filepath.FromSlash(relative))
					if exists(old) {
						dest := filepath.Join(o.Target, filepath.FromSlash(recovery+"/previous/"+relative))
						if e = os.MkdirAll(filepath.Dir(dest), 0700); e != nil {
							return e
						}
						if e = renameNoReplace(old, dest); e != nil {
							return e
						}
					}
				}
				candidate := filepath.Join(o.Target, filepath.FromSlash(relative))
				info, e := os.Lstat(source)
				if e != nil {
					return e
				}
				candidateInfo, ce := os.Lstat(candidate)
				if link || (info.IsDir() && !hasOwnedChildren && (ce != nil || !candidateInfo.IsDir())) {
					r.Moves = append(r.Moves, Move{relative, destination(o.Target, recovery, relative, programContent), info.IsDir(), "prepared"})
					continue
				}
				if info.IsDir() {
					r.Directories = append(r.Directories, relative)
					pending = append(pending, relative)
					continue
				}
				if !owned && !exists(candidate) && !strings.EqualFold(relative, ManifestName) {
					r.Moves = append(r.Moves, Move{relative, destination(o.Target, recovery, relative, programContent), false, "prepared"})
					continue
				}
				hash, e := fileHash(source)
				if e != nil {
					return e
				}
				if (owned && previous[key].SHA256 == hash) || (strings.EqualFold(relative, ManifestName) && len(previous) > 0) {
					r.Files = append(r.Files, TransferFile{relative, hash, ""})
					continue
				}
				if ce == nil && candidateInfo.Mode().IsRegular() {
					sameHash, e := fileHash(candidate)
					if e != nil {
						return e
					}
					if sameHash == hash {
						r.Files = append(r.Files, TransferFile{relative, hash, relative})
						continue
					}
				}
				r.Moves = append(r.Moves, Move{relative, destination(o.Target, recovery, relative, programContent), false, "prepared"})
			}
		}
	}
	*phase = "writing_new_manifest"
	m := Manifest{Version: 1, Files: []File{}}
	for _, f := range current {
		m.Files = append(m.Files, f)
	}
	sort.Slice(m.Files, func(i, j int) bool { return m.Files[i].Path < m.Files[j].Path })
	if e := writeJSON(filepath.Join(o.Target, ManifestName), m, true); e != nil {
		return e
	}
	*phase = "writing_transaction_receipt"
	return writeJSON(o.Receipt, r, false)
}

func transfer(ctx context.Context, o Options) error {
	r, e := readReceipt(o.Receipt)
	if e != nil {
		return e
	}
	for n := 0; n < len(r.Moves); n++ {
		if e := ctx.Err(); e != nil {
			return e
		}
		i := n
		if o.Operation == "restore" {
			i = len(r.Moves) - 1 - n
		}
		m := &r.Moves[i]
		if o.Operation == "restore" && m.State != "moving" && m.State != "moved" {
			continue
		}
		from, to := m.Path, m.Destination
		if o.Operation == "restore" {
			from, to = to, from
		}
		from = filepath.Join(o.Source, filepath.FromSlash(from))
		to = filepath.Join(o.Target, filepath.FromSlash(to))
		if !exists(from) {
			if o.Operation == "restore" && !exists(to) {
				return errors.New("待恢复的个人文件无法定位，回执已保留：" + m.Path)
			}
			if o.Operation == "move" && m.State == "prepared" {
				m.State = "absent"
				if e = writeJSON(o.Receipt, r, true); e != nil {
					return e
				}
			}
			continue
		}
		// Check parents only: the entry itself may be a personal symlink that
		// must be moved as an object, never followed.
		if e = PlainPath(filepath.Dir(from)); e != nil {
			return e
		}
		if e = PlainPath(filepath.Dir(to)); e != nil {
			return e
		}
		if e = os.MkdirAll(filepath.Dir(to), 0700); e != nil {
			return e
		}
		if o.Operation == "move" {
			m.State = "moving"
			if e = writeJSON(o.Receipt, r, true); e != nil {
				return e
			}
		}
		if e = renameNoReplace(from, to); e != nil {
			if o.Operation == "move" {
				m.State = "failed"
				e = errors.Join(e, writeJSON(o.Receipt, r, true))
			}
			return e
		}
		m.State = "moved"
		if o.Operation == "restore" {
			m.State = "restored"
		}
		if e = writeJSON(o.Receipt, r, true); e != nil {
			return e
		}
	}
	return nil
}

func moveTopLevel(ctx context.Context, from, to string, keep []string, removeEmpty bool, blocked *string) (err error) {
	program := ReadManifest(filepath.Join(from, ManifestName))
	names := map[string]bool{strings.ToLower(ManifestName): true, "uninstall.exe": true}
	for _, f := range program {
		names[strings.ToLower(strings.Split(f.Path, "/")[0])] = true
	}
	if err = os.MkdirAll(to, 0700); err != nil {
		return err
	}
	moved := []string{}
	defer func() {
		if err != nil {
			for i := len(moved) - 1; i >= 0; i-- {
				err = errors.Join(err, renameNoReplace(filepath.Join(to, moved[i]), filepath.Join(from, moved[i])))
			}
			if removeEmpty {
				if entries, e := os.ReadDir(to); e == nil && len(entries) == 0 {
					err = errors.Join(err, os.Remove(to))
				}
			}
		}
	}()
	entries, e := os.ReadDir(from)
	if e != nil {
		return e
	}
	for _, entry := range entries {
		if contains(keep, entry.Name()) {
			continue
		}
		if e = ctx.Err(); e != nil {
			return e
		}
		*blocked = "personal"
		link, e := isReparse(filepath.Join(from, entry.Name()))
		if e != nil {
			return e
		}
		if names[strings.ToLower(entry.Name())] {
			*blocked = "program"
		} else if entry.Name() == "Eden-preserved-files" {
			*blocked = "preserved"
		} else if link {
			*blocked = "link"
		}
		if e = renameNoReplace(filepath.Join(from, entry.Name()), filepath.Join(to, entry.Name())); e != nil {
			return e
		}
		moved = append(moved, entry.Name())
	}
	*blocked = ""
	return nil
}

// knownTree only descends into manifest-related directories and never follows
// reparse points. Unknown project subtrees are outside the deletion authority.
func knownTree(root string, files map[string]File) (map[string]bool, error) {
	dirs := directories(files)
	result := map[string]bool{}
	pending := []string{""}
	for len(pending) > 0 {
		dir := pending[0]
		pending = pending[1:]
		entries, e := os.ReadDir(filepath.Join(root, filepath.FromSlash(dir)))
		if e != nil {
			return nil, e
		}
		for _, entry := range entries {
			rel := path.Join(dir, entry.Name())
			link, e := isReparse(filepath.Join(root, filepath.FromSlash(rel)))
			if e != nil {
				return nil, e
			}
			if link {
				continue
			}
			result[strings.ToLower(rel)] = true
			if entry.IsDir() && dirs[strings.ToLower(rel)] {
				pending = append(pending, rel)
			}
		}
	}
	return result, nil
}

func removeEmpty(root string, dirs []string) error {
	sort.Slice(dirs, func(i, j int) bool { return len(dirs[i]) > len(dirs[j]) })
	for _, d := range dirs {
		name := filepath.Join(root, filepath.FromSlash(d))
		if e := PlainPath(name); e != nil {
			continue
		}
		entries, e := os.ReadDir(name)
		if errors.Is(e, os.ErrNotExist) {
			continue
		}
		if e != nil {
			return e
		}
		if len(entries) == 0 {
			if e = os.Remove(name); e != nil {
				return e
			}
		}
	}
	return nil
}

func cleanup(ctx context.Context, o Options) error {
	r, e := readReceipt(o.Receipt)
	if e != nil {
		return e
	}
	known := map[string]File{}
	for _, f := range r.Files {
		known[strings.ToLower(f.Path)] = File{f.Path, f.SHA256}
	}
	actual, e := knownTree(o.Source, known)
	if e != nil {
		return e
	}
	for _, f := range r.Files {
		if e = ctx.Err(); e != nil {
			return e
		}
		if !actual[strings.ToLower(f.Path)] {
			continue
		}
		if f.Destination != "" {
			h, e := fileHash(filepath.Join(o.Target, filepath.FromSlash(f.Destination)))
			if e != nil || h != f.SHA256 {
				continue
			}
		}
		// A changed or occupied file stays in the old tree, which is preserved
		// below Eden-preserved-files rather than recursively deleted.
		if e = RemoveMatchingFile(o.Source, f.Path, f.SHA256); e != nil && o.Output != nil {
			fmt.Fprintln(o.Output, "安装期间变化或被占用的文件已保留在旧目录。")
		}
	}
	if e = removeEmpty(o.Source, r.Directories); e != nil {
		return e
	}
	entries, e := os.ReadDir(o.Source)
	if e != nil {
		return e
	}
	if len(entries) > 0 {
		recovery := filepath.Join(o.Target, "Eden-preserved-files", "late-"+token())
		if e = PlainPath(filepath.Dir(recovery)); e != nil {
			return e
		}
		if e = os.MkdirAll(filepath.Dir(recovery), 0700); e != nil {
			return e
		}
		if e = renameNoReplace(o.Source, recovery); e != nil {
			return e
		}
	} else if e = os.Remove(o.Source); e != nil {
		return e
	}
	return complete(o)
}

func complete(o Options) error {
	r, e := readReceipt(o.Receipt)
	if e != nil {
		return e
	}
	if !exists(filepath.Join(o.Target, IdentityName)) {
		return errors.New("安装目录尚未恢复，事务回执已保留")
	}
	if r.BackupPath != "" && exists(r.BackupPath) {
		return errors.New("旧目录仍需恢复，事务回执已保留：" + r.BackupPath)
	}
	for _, m := range r.Moves {
		if m.State == "absent" {
			continue
		}
		relative := m.Path
		if m.State == "moved" {
			relative = m.Destination
		}
		if !exists(filepath.Join(o.Target, filepath.FromSlash(relative))) {
			return errors.New("个人文件尚未恢复，事务回执已保留：" + relative)
		}
	}
	h, e := fileHash(o.Receipt)
	if e != nil {
		return e
	}
	if e = RemoveMatchingFile(filepath.Dir(o.Receipt), filepath.Base(o.Receipt), h); e != nil {
		return e
	}
	if exists(o.Receipt) {
		return errors.New("安装回执在完成期间发生变化，已保留待核对")
	}
	return nil
}

func uninstall(ctx context.Context, o Options) error {
	owned := ReadManifest(o.Manifest)
	if len(owned) == 0 {
		return errors.New("卸载文件清单无效，原文件已保留")
	}
	installed := ReadManifest(filepath.Join(o.Target, ManifestName))
	if f, ok := installed["uninstall.exe"]; ok {
		owned["uninstall.exe"] = f
	}
	actual, e := knownTree(o.Target, owned)
	if e != nil {
		return e
	}
	for key, f := range owned {
		if e = ctx.Err(); e != nil {
			return e
		}
		if key != strings.ToLower(IdentityName) && actual[key] {
			if e = RemoveMatchingFile(o.Target, f.Path, f.SHA256); e != nil {
				return e
			}
		}
	}
	if len(installed) > 0 {
		h, e := fileHash(filepath.Join(o.Target, ManifestName))
		if e != nil {
			return e
		}
		if e = RemoveMatchingFile(o.Target, ManifestName, h); e != nil {
			return e
		}
	}
	dirs := []string{}
	for d := range directories(owned) {
		if d != "" {
			dirs = append(dirs, d)
		}
	}
	if e = removeEmpty(o.Target, dirs); e != nil {
		return e
	}
	entries, e := os.ReadDir(o.Target)
	if e != nil {
		return e
	}
	remaining := 0
	for _, entry := range entries {
		if !strings.EqualFold(entry.Name(), IdentityName) {
			remaining++
		}
	}
	if remaining == 0 {
		if f, ok := owned[strings.ToLower(IdentityName)]; ok {
			if e = RemoveMatchingFile(o.Target, f.Path, f.SHA256); e != nil {
				return e
			}
		}
		entries, e = os.ReadDir(o.Target)
		if e != nil {
			return e
		}
		if len(entries) == 0 {
			return os.Remove(o.Target)
		}
	}
	return nil
}

package desktopupdate

import (
	"encoding/hex"
	"errors"
	"os"
	"strings"

	"github.com/sylvinus/agent-vm/internal/installfiles"
)

// VerifyArchiveLocked returns the verified read handle. The caller must retain
// it until the installer exits; no writer or replacer is permitted in that span.
func VerifyArchiveLocked(root, path, digest, version, publisher string) (*os.File, error) {
	if _, e := DownloadPath(path, root); e != nil {
		return nil, e
	}
	f, e := installfiles.OpenReadLocked(path)
	if e != nil {
		return nil, e
	}
	if e = verifyArchive(f, path, digest, version, publisher); e != nil {
		f.Close()
		return nil, e
	}
	return f, nil
}
func verifyArchive(f *os.File, path, digest, version, publisher string) error {
	info, e := f.Stat()
	if e != nil {
		return e
	}
	if info.Size() <= 0 || info.Size() > 512<<20 {
		return errors.New("安装包大小无效")
	}
	b, e := hex.DecodeString(digest)
	if e != nil || len(b) != 32 {
		return errors.New("安装包下载摘要无效")
	}
	h, e := installfiles.Hash(f)
	if e != nil {
		return e
	}
	if !strings.EqualFold(h, digest) {
		return errors.New("安装包下载后发生变化，请重新下载")
	}
	if e = SamePublisher(path, publisher); e != nil {
		return e
	}
	product, v, e := ProductVersion(path)
	if e != nil {
		return e
	}
	if product != "ClaudeEden" || v != version {
		return errors.New("安装包产品身份或版本不匹配")
	}
	return nil
}
func SamePublisher(path, expected string) error {
	p, e := Publisher(path)
	if e != nil {
		return e
	}
	if p != expected {
		return errors.New("更新包的发布者签名与当前 Claude Eden 不一致")
	}
	return nil
}

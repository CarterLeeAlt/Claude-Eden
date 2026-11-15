// Package hostimage prepares pinned Linux guest installation images.
package hostimage

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"
)

type Identity struct {
	System  string `json:"system"`
	Version string `json:"version"`
	Arch    string `json:"arch"`
}

func IsLinux(system string) bool {
	return system == "debian-13"
}

// Plan is immutable after the user chooses it. Path refers only to source
// media. Download destinations are derived independently from its digest.
type Plan struct {
	Host     Identity `json:"host"`
	Version  string   `json:"version"`
	Format   string   `json:"format"`
	URL      string   `json:"url,omitempty"`
	Digest   string   `json:"digest,omitempty"`
	Bytes    int64    `json:"bytes"`
	Path     string   `json:"path,omitempty"`
	Modified int64    `json:"modified,omitempty"`
}

func (p Plan) Key() string {
	if p.URL != "" && p.Digest != "" {
		p.Path, p.Modified = "", 0
	}
	data, _ := json.Marshal(p)
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func (p Plan) Validate() error {
	if p.Host.Arch != "amd64" {
		return errors.New("所选 Linux 镜像架构不受支持")
	}
	if p.Version != "13" || p.Version != p.Host.Version || p.Bytes < 0 || p.Bytes > 64<<30 {
		return errors.New("Linux 镜像与所选系统版本不匹配")
	}
	switch p.Host.System {
	case "debian-13":
		if p.Format != "qcow2" {
			return errors.New("Linux 安装镜像与本机不匹配")
		}
	default:
		return errors.New("当前仅支持 Debian 13 镜像")
	}
	if p.Path != "" && (!filepath.IsAbs(p.Path) || filepath.Clean(p.Path) != p.Path || strings.ContainsAny(p.Path, "\r\n\x00")) {
		return errors.New("本机系统镜像路径无效")
	}
	if p.URL != "" {
		if err := trustedURL(p.URL, p.Host.System); err != nil {
			return err
		}
	}
	if _, err := digestHash(p.Digest); err != nil {
		return err
	}
	if p.Path == "" && p.URL == "" {
		return errors.New("缺少系统安装材料")
	}
	return nil
}

func trustedURL(raw, system string) error {
	u, err := url.Parse(raw)
	if err != nil || u.User != nil || u.Port() != "" || u.Fragment != "" {
		return errors.New("系统材料下载地址无效")
	}
	if u.Scheme != "https" {
		return errors.New("系统材料下载协议无效")
	}
	if system == "debian-13" && u.Hostname() == "cloud.debian.org" {
		return nil
	}
	return errors.New("系统材料下载地址不属于受支持的系统厂商")
}

// Resolve first reuses previously verified media metadata. Template is the
// application's bundled, pinned Linux image list, supplied by its owner.
func Resolve(identity Identity, cache string, template []byte) (Plan, error) {
	if plan, err := Load(cache); err == nil && plan.Host == identity {
		if plan.Path != "" && usableFile(plan.Path, plan.Bytes) {
			info, _ := os.Stat(plan.Path)
			if info.ModTime().UnixNano() == plan.Modified {
				return plan, nil
			}
		}
		if path := plan.CachedPath(cache); usableFile(path, plan.Bytes) {
			plan.Path = path
			info, _ := os.Stat(path)
			plan.Modified = info.ModTime().UnixNano()
			return plan, plan.Validate()
		}
	}
	plan, err := linuxCatalog(identity, template)
	if err != nil {
		return Plan{}, err
	}
	if candidate := plan.CachedPath(cache); usableFile(candidate, plan.Bytes) {
		info, _ := os.Stat(candidate)
		plan.Path, plan.Modified = candidate, info.ModTime().UnixNano()
		if plan.Bytes == 0 {
			plan.Bytes = info.Size()
		}
	}
	return plan, plan.Validate()
}

func usableFile(path string, size int64) bool {
	info, err := os.Lstat(path)
	return err == nil && info.Mode().IsRegular() && info.Size() > 0 && (size == 0 || info.Size() == size)
}

func linuxCatalog(host Identity, template []byte) (Plan, error) {
	var config struct {
		Images []struct {
			Location string `yaml:"location"`
			Arch     string `yaml:"arch"`
			Digest   string `yaml:"digest"`
			Bytes    int64  `yaml:"bytes"`
		} `yaml:"images"`
	}
	if err := yaml.Unmarshal(template, &config); err != nil {
		return Plan{}, err
	}
	for _, image := range config.Images {
		if host.Arch == "amd64" && image.Arch == "x86_64" && image.Digest != "" {
			return Plan{Host: host, Version: host.Version, Format: "qcow2", URL: image.Location, Digest: image.Digest, Bytes: image.Bytes}, nil
		}
	}
	return Plan{}, errors.New("此安装包缺少与本机发行版一致的已校验 Linux 镜像")
}

func (p Plan) CachedPath(cache string) string {
	key := strings.ReplaceAll(p.Digest, ":", "-")
	if key == "" {
		key = p.Key()
	}
	return filepath.Join(cache, key+"."+p.Format)
}

func (p Plan) RemainingBytes(cache string) int64 {
	if p.Path != "" {
		return 0
	}
	if usableFile(p.CachedPath(cache), p.Bytes) {
		return 0
	}
	info, err := os.Lstat(p.CachedPath(cache) + ".partial")
	if err == nil && info.Mode().IsRegular() && info.Size() <= p.Bytes {
		return p.Bytes - info.Size()
	}
	return p.Bytes
}

const sourceRecord = "host-system-source.json"

func Save(cache string, plan Plan) error {
	if err := plan.Validate(); err != nil {
		return err
	}
	if err := os.MkdirAll(cache, 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(cache, ".host-system-source-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	err = json.NewEncoder(file).Encode(plan)
	closeErr := file.Close()
	if err = errors.Join(err, closeErr); err != nil {
		return err
	}
	return os.Rename(file.Name(), filepath.Join(cache, sourceRecord))
}

func Load(cache string) (Plan, error) {
	var plan Plan
	file, err := os.Open(filepath.Join(cache, sourceRecord))
	if err != nil {
		return plan, err
	}
	defer file.Close()
	decoder := json.NewDecoder(io.LimitReader(file, 32<<10))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&plan); err != nil {
		return plan, err
	}
	return plan, plan.Validate()
}

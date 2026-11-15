package guestos

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/sylvinus/agent-vm/internal/hostimage"
)

// Installation persists the media choice and private instance identity.
type Installation struct {
	Version int            `json:"version"`
	System  string         `json:"system"`
	Media   string         `json:"media,omitempty"`
	ID      uuid.UUID      `json:"id"`
	Source  hostimage.Plan `json:"source,omitzero"`
}

const installRecord = "guest-install.json"

func (i Installation) Validate() error {
	if i.Version != 1 || i.ID == uuid.Nil || !hostimage.IsLinux(i.System) {
		return errors.New("客户机安装记录无效")
	}
	if i.Media != "" && (!filepath.IsAbs(i.Media) || strings.ContainsAny(i.Media, "\r\n\x00") || filepath.Clean(i.Media) != i.Media) {
		return errors.New("请选择本机的安装镜像")
	}
	if i.System != i.Source.Host.System {
		return errors.New("Linux 客户机安装来源与所选系统不一致")
	}
	if i.Media != "" && i.Media != i.Source.Path {
		return errors.New("安装介质与已选择的 Linux 镜像来源不一致")
	}
	return i.Source.Validate()
}

func LoadInstallation(profile string) (*Installation, error) {
	path := filepath.Join(profile, installRecord)
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > 32<<10 {
		return nil, errors.New("客户机安装记录无法读取")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var i Installation
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&i) != nil || decoder.Decode(new(any)) != io.EOF {
		return nil, errors.New("客户机安装记录格式无效")
	}
	if err := i.Validate(); err != nil {
		return nil, err
	}
	if i.Source.Host.System != "" {
		var ready struct {
			ID     uuid.UUID      `json:"id"`
			Source hostimage.Plan `json:"source"`
		}
		mediaPath := filepath.Join(profile, "guest-install-media-"+i.ID.String()+".json")
		info, err := os.Lstat(mediaPath)
		if err == nil {
			if !info.Mode().IsRegular() || info.Size() > 32<<10 {
				return nil, errors.New("已校验的安装材料记录无效")
			}
			data, readErr := os.ReadFile(mediaPath)
			if readErr != nil || json.Unmarshal(data, &ready) != nil || ready.ID != i.ID {
				return nil, errors.New("无法读取已校验的安装材料")
			}
			if err := matchPreparedMedia(i.Source, ready.Source); err != nil {
				return nil, err
			}
			i.Source, i.Media = ready.Source, ready.Source.Path
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}
	return &i, i.Validate()
}

func SaveInstallation(profile string, i Installation) error {
	if err := i.Validate(); err != nil {
		return err
	}
	if current, err := LoadInstallation(profile); err != nil {
		return err
	} else if current != nil {
		if *current == i {
			return nil
		}
		return errors.New("已有其他安装选择，请继续原安装")
	}
	data, err := json.Marshal(i)
	if err != nil {
		return err
	}
	return publishBinding(profile, installRecord, append(data, '\n'))
}

func matchPreparedMedia(chosen, ready hostimage.Plan) error {
	if err := ready.Validate(); err != nil {
		return err
	}
	if ready.Path == "" || ready.Digest == "" || ready.Host != chosen.Host || ready.Version != chosen.Version ||
		ready.Format != chosen.Format || ready.URL != chosen.URL || (chosen.Digest != "" && ready.Digest != chosen.Digest) ||
		(chosen.Bytes != 0 && ready.Bytes != chosen.Bytes) || (chosen.Path != "" && ready.Path != chosen.Path) ||
		(chosen.Modified != 0 && ready.Modified != chosen.Modified) {
		return errors.New("已校验的系统材料与原安装选择不一致")
	}
	return nil
}

// RecordPreparedMedia publishes a create-only receipt. The original choice
// remains immutable throughout downloading, installation and concurrent retries.
func RecordPreparedMedia(profile string, id uuid.UUID, ready hostimage.Plan) error {
	current, err := LoadInstallation(profile)
	if err != nil {
		return err
	}
	if current == nil || current.ID != id {
		return errors.New("安装选择已变化")
	}
	if err := matchPreparedMedia(current.Source, ready); err != nil {
		return err
	}
	data, err := json.Marshal(struct {
		ID     uuid.UUID      `json:"id"`
		Source hostimage.Plan `json:"source"`
	}{id, ready})
	if err != nil {
		return err
	}
	return publishBinding(profile, "guest-install-media-"+id.String()+".json", data)
}

func FinishLinuxInstallation(profile string, id uuid.UUID) error {
	installed, err := LoadInstallation(profile)
	if err != nil {
		return err
	}
	if installed == nil || installed.ID != id || !hostimage.IsLinux(installed.System) {
		return errors.New("Linux 安装记录已变化")
	}
	// Keep recovery information until the persistent image choice can be read.
	stored, err := hostimage.Load(filepath.Join(profile, "state", "guest-media"))
	if err != nil {
		return err
	}
	if err := matchPreparedMedia(installed.Source, stored); err != nil {
		return err
	}
	system, err := hostimage.LinuxSystem(filepath.Join(profile, "state"))
	if err != nil {
		return err
	}
	if system != installed.System {
		return errors.New("Linux 安装尚未保存所选系统，保留安装记录以便继续")
	}
	return os.Remove(filepath.Join(profile, installRecord))
}

// UnstartedInstallation reports whether base disk creation has not begun.
// A partial VM counts as started and must keep its original media binding.
func UnstartedInstallation(profile string, i Installation) (bool, error) {
	paths := []string{filepath.Join("state", "lima", "base-building"), filepath.Join("state", "lima", "base")}
	for _, rel := range paths {
		_, err := os.Lstat(filepath.Join(profile, rel))
		if err == nil {
			return false, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return false, err
		}
	}
	return true, nil
}

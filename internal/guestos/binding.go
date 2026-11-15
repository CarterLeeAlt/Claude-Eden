package guestos

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// publishBinding publishes an immutable installation or guest identity record.
func publishBinding(profile, name string, data []byte) error {
	temp, err := os.CreateTemp(profile, ".guest-binding-*")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	_, writeErr := temp.Write(data)
	if err := errors.Join(writeErr, temp.Close()); err != nil {
		return err
	}
	path := filepath.Join(profile, name)
	if err := os.Link(temp.Name(), path); !errors.Is(err, os.ErrExist) {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() != int64(len(data)) {
		return fmt.Errorf("unexpected existing %s", name)
	}
	prior, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(data, prior) {
		return fmt.Errorf("refusing to replace existing %s", name)
	}
	return nil
}

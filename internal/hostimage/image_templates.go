package hostimage

import (
	"embed"
	"fmt"
)

// These reviewed image definitions are also copied into the VM template bundle.
//
//go:embed images/debian-13.yaml
var imageTemplates embed.FS

func ReadImageTemplate(system string) ([]byte, error) {
	if !IsLinux(system) {
		return nil, fmt.Errorf("unsupported host image template %q", system)
	}
	return imageTemplates.ReadFile("images/" + system + ".yaml")
}

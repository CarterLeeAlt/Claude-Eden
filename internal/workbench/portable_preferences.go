package workbench

import (
	"path/filepath"

	"github.com/sylvinus/agent-vm/internal/eden"
)

func savePreferences(profile string, value Preferences) error {
	var err error
	value.Workspace, err = eden.StorePortablePath(profile, value.Workspace)
	if err != nil {
		return err
	}
	return writePrivate(filepath.Join(profile, "workbench.json"), value)
}

func (s *Server) saveSharedProjects(value sharedProjects) error {
	stored := sharedProjects{Paths: append([]string(nil), value.Paths...), Groups: append([]projectGroup(nil), value.Groups...)}
	for i, path := range stored.Paths {
		encoded, err := eden.StorePortablePath(s.profile, path)
		if err != nil {
			return err
		}
		stored.Paths[i] = encoded
	}
	for i := range stored.Groups {
		group := &stored.Groups[i]
		var err error
		group.Workspace, err = eden.StorePortablePath(s.profile, group.Workspace)
		if err != nil {
			return err
		}
		group.Folders = append([]string(nil), group.Folders...)
		for j, path := range group.Folders {
			group.Folders[j], err = eden.StorePortablePath(s.profile, path)
			if err != nil {
				return err
			}
		}
	}
	return writePrivate(filepath.Join(s.profile, "projects.json"), stored)
}

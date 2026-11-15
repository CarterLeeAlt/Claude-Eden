package limaembed

import (
	"io/fs"
	"reflect"
	"strings"
	"testing"
)

func TestProductTemplateClosure(t *testing.T) {
	var templates []string
	err := fs.WalkDir(assets, "assets", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && strings.HasPrefix(name, "assets/templates/") {
			templates = append(templates, strings.TrimPrefix(name, "assets/templates/"))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"_default/mounts.yaml", "_images/debian-13.yaml", "debian-13.yaml"}
	if !reflect.DeepEqual(templates, want) {
		t.Fatalf("embedded template closure: %v, want %v", templates, want)
	}
}

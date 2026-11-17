// Package uitheme owns the default ClaudeEden palette shared by the native
// shell, controller and React UI. The UI imports the same embedded JSON.
package uitheme

import (
	_ "embed"
	"encoding/json"
)

//go:embed light.json
var LightJSON []byte

type Palette struct {
	Surface string `json:"surface"`
	Ink     string `json:"ink"`
	Accent  string `json:"accent"`
}

func Default() Palette {
	var palette Palette
	if err := json.Unmarshal(LightJSON, &palette); err != nil {
		panic("invalid bundled ClaudeEden palette: " + err.Error())
	}
	return palette
}

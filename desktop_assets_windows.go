package agentvm

import _ "embed"

// DesktopIcon is the packaged icon used by the Windows window and tray.
//
//go:embed packaging/windows/Eden.ico
var DesktopIcon []byte

// DesktopFontRegular and DesktopFontBold are loaded privately by the desktop process.
var (
	//go:embed assets/fonts/EdenUI-Regular.ttf
	DesktopFontRegular []byte
	//go:embed assets/fonts/EdenUI-Bold.ttf
	DesktopFontBold []byte
)

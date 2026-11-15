package eden

import _ "embed"

//go:embed shared-clipboard.py
var SharedClipboardHelper string

func sharedClipboardScript() string {
	return "sudo tee /usr/local/bin/workbench-shared-clipboard >/dev/null <<'WORKBENCH_SHARED_CLIPBOARD'\n" + SharedClipboardHelper + "\nWORKBENCH_SHARED_CLIPBOARD\nsudo chmod 0755 /usr/local/bin/workbench-shared-clipboard\n"
}

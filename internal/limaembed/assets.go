package limaembed

import "embed"

//go:embed assets/lima-guestagent.Linux-*.gz assets/templates/debian-13.yaml assets/templates/_images/debian-13.yaml assets/templates/_default/mounts.yaml
var assets embed.FS

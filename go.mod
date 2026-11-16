module github.com/sylvinus/agent-vm

go 1.26.10

// Bundled dependencies are built from third_party/. These replaces only apply when
// this module is the main one, so `go install ...@latest` does not build
// agent-vm: releases ship binaries.
replace (
	github.com/containers/gvisor-tap-vsock => ./third_party/gvisor-tap-vsock
	github.com/lima-vm/lima/v2 => ./third_party/lima
	github.com/lima-vm/sshocker => ./third_party/sshocker
	github.com/pkg/sftp => ./third_party/sftp
	github.com/wailsapp/wails/v3 => ./third_party/wails
	workbench.local/guestidentity => ./shared/guestidentity
)

tool (
	github.com/lima-vm/lima/v2/cmd/lima-guestagent
	github.com/lima-vm/lima/v2/cmd/limactl
	github.com/wailsapp/wails/v3/cmd/wails3
)

require (
	al.essio.dev/pkg/shellescape v1.6.1
	github.com/containerd/continuity v0.5.0
	github.com/coreos/go-semver v0.3.1
	github.com/creack/pty v1.1.24
	github.com/docker/go-units v0.5.0
	github.com/google/uuid v1.6.0
	github.com/gorilla/websocket v1.5.3
	github.com/inetaf/tcpproxy v0.0.0-20250222171855-c4b9df066048
	github.com/klauspost/compress v1.18.7
	github.com/lima-vm/lima/v2 v2.0.0
	github.com/lima-vm/sshocker v0.3.11
	github.com/mattn/go-isatty v0.0.24
	github.com/miekg/dns v1.1.73
	github.com/pbnjay/memory v0.0.0-20210728143218-7b4eea64cf58
	github.com/pelletier/go-toml/v2 v2.4.3
	github.com/pkg/sftp v1.13.11
	github.com/sirupsen/logrus v1.10.2
	github.com/spf13/cobra v1.10.2
	go.yaml.in/yaml/v3 v3.0.5
	golang.org/x/crypto v0.57.0
	golang.org/x/mod v0.41.0
	golang.org/x/net v0.60.0
	golang.org/x/sys v0.48.0
	golang.org/x/term v0.46.0
	workbench.local/guestidentity v0.0.0
)

require (
	atomicgo.dev/cursor v0.2.0 // indirect
	atomicgo.dev/keyboard v0.2.9 // indirect
	atomicgo.dev/schedule v0.1.0 // indirect
	dario.cat/mergo v1.0.2 // indirect
	github.com/AlecAivazis/survey/v2 v2.3.7 // indirect
	github.com/AlekSi/pointer v1.2.0 // indirect
	github.com/BurntSushi/toml v1.6.0 // indirect
	github.com/Code-Hex/go-infinity-channel v1.0.0 // indirect
	github.com/Code-Hex/vz/v3 v3.7.1 // indirect
	github.com/Ladicle/tabwriter v1.0.0 // indirect
	github.com/Masterminds/goutils v1.1.1 // indirect
	github.com/Masterminds/semver/v3 v3.4.0 // indirect
	github.com/Masterminds/sprig/v3 v3.3.0 // indirect
	github.com/Microsoft/go-winio v0.6.3-0.20251027160822-ad3df93bed29 // indirect
	github.com/Microsoft/hcsshim v0.14.1 // indirect
	github.com/ProtonMail/go-crypto v1.3.0 // indirect
	github.com/VividCortex/ewma v1.2.0 // indirect
	github.com/a8m/envsubst v1.4.3 // indirect
	github.com/agext/levenshtein v1.2.1 // indirect
	github.com/alecthomas/chroma/v2 v2.23.1 // indirect
	github.com/alecthomas/participle/v2 v2.1.4 // indirect
	github.com/apparentlymart/go-cidr v1.1.1 // indirect
	github.com/apparentlymart/go-textseg/v15 v15.0.0 // indirect
	github.com/apparentlymart/go-textseg/v17 v17.0.1 // indirect
	github.com/atotto/clipboard v0.1.4 // indirect
	github.com/atterpac/refresh v1.1.4 // indirect
	github.com/aymanbagabas/go-osc52/v2 v2.0.1 // indirect
	github.com/aymerick/douceur v0.2.0 // indirect
	github.com/bahlo/generic-list-go v0.2.0 // indirect
	github.com/balajiv113/fd v0.0.0-20230330094840-143eec500f3e // indirect
	github.com/blakesmith/ar v0.0.0-20190502131153-809d4375e1fb // indirect
	github.com/bmatcuk/doublestar/v4 v4.7.1 // indirect
	github.com/buger/jsonparser v1.1.2 // indirect
	github.com/catppuccin/go v0.3.0 // indirect
	github.com/cavaliergopher/cpio v1.0.1 // indirect
	github.com/chainguard-dev/git-urls v1.0.2 // indirect
	github.com/charmbracelet/bubbles v0.21.1-0.20250623103423-23b8fd6302d7 // indirect
	github.com/charmbracelet/bubbletea v1.3.10 // indirect
	github.com/charmbracelet/colorprofile v0.4.1 // indirect
	github.com/charmbracelet/glamour v0.10.0 // indirect
	github.com/charmbracelet/huh v0.8.0 // indirect
	github.com/charmbracelet/lipgloss v1.1.1-0.20250404203927-76690c660834 // indirect
	github.com/charmbracelet/x/ansi v0.11.4 // indirect
	github.com/charmbracelet/x/cellbuf v0.0.14 // indirect
	github.com/charmbracelet/x/exp/slice v0.0.0-20260122224438-b01af16209d9 // indirect
	github.com/charmbracelet/x/exp/strings v0.0.0-20260122224438-b01af16209d9 // indirect
	github.com/charmbracelet/x/term v0.2.2 // indirect
	github.com/cheggaaa/pb/v3 v3.2.1 // indirect
	github.com/cilium/ebpf v0.22.0 // indirect
	github.com/clipperhouse/displaywidth v0.7.0 // indirect
	github.com/clipperhouse/stringish v0.1.1 // indirect
	github.com/clipperhouse/uax29/v2 v2.4.0 // indirect
	github.com/cloudflare/circl v1.6.3 // indirect
	github.com/containerd/cgroups/v3 v3.1.3 // indirect
	github.com/containerd/console v1.0.5 // indirect
	github.com/containerd/errdefs v1.0.0 // indirect
	github.com/containerd/errdefs/pkg v0.3.0 // indirect
	github.com/containerd/log v0.1.0 // indirect
	github.com/containerd/typeurl/v2 v2.3.0 // indirect
	github.com/containers/gvisor-tap-vsock v0.8.9 // indirect
	github.com/coreos/go-systemd/v22 v22.7.0 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.7 // indirect
	github.com/cyphar/filepath-securejoin v0.6.1 // indirect
	github.com/danieljoos/wincred v1.2.3 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/digitalocean/go-libvirt v0.0.0-20220804181439-8648fbde413e // indirect
	github.com/digitalocean/go-qemu v0.0.0-20221209210016-f035778c97f7 // indirect
	github.com/dimchansky/utfbom v1.1.1 // indirect
	github.com/diskfs/go-diskfs v1.9.4 // indirect
	github.com/djherbis/times v1.6.0 // indirect
	github.com/dlclark/regexp2 v1.11.5 // indirect
	github.com/dominikbraun/graph v0.23.0 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/ebitengine/purego v0.11.1 // indirect
	github.com/elliotchance/orderedmap v1.8.0 // indirect
	github.com/emirpasic/gods v1.18.1 // indirect
	github.com/erikgeiser/coninput v0.0.0-20211004153227-1c3628e74d0f // indirect
	github.com/fatih/color v1.19.0 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/go-git/gcfg v1.5.1-0.20230307220236-3a3c6141e376 // indirect
	github.com/go-git/go-billy/v5 v5.9.0 // indirect
	github.com/go-git/go-git/v5 v5.19.2 // indirect
	github.com/go-ini/ini v1.67.0 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/go-task/slim-sprig/v3 v3.0.0 // indirect
	github.com/go-task/template v0.2.0 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/goccy/go-json v0.10.6 // indirect
	github.com/goccy/go-yaml v1.19.2 // indirect
	github.com/godbus/dbus/v5 v5.2.2 // indirect
	github.com/golang/groupcache v0.0.0-20241129210726-2c02b8208cf8 // indirect
	github.com/google/btree v1.1.3 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/gopacket v1.1.19 // indirect
	github.com/google/jsonschema-go v0.4.3 // indirect
	github.com/google/rpmpack v0.7.1 // indirect
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510 // indirect
	github.com/google/yamlfmt v0.21.0 // indirect
	github.com/gookit/color v1.6.0 // indirect
	github.com/goreleaser/chglog v0.7.4 // indirect
	github.com/goreleaser/fileglob v1.4.0 // indirect
	github.com/goreleaser/nfpm/v2 v2.44.1 // indirect
	github.com/gorilla/css v1.0.1 // indirect
	github.com/hashicorp/hcl/v2 v2.24.0 // indirect
	github.com/huandu/xstrings v1.5.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/insomniacslk/dhcp v0.0.0-20260901064844-234b97448fae // indirect
	github.com/invopop/jsonschema v0.14.0 // indirect
	github.com/jbenet/go-context v0.0.0-20150711004518-d14ea06fba99 // indirect
	github.com/jinzhu/copier v0.4.0 // indirect
	github.com/joho/godotenv v1.5.1 // indirect
	github.com/kballard/go-shellquote v0.0.0-20180428030007-95032a82bc51 // indirect
	github.com/kevinburke/ssh_config v1.4.0 // indirect
	github.com/klauspost/cpuid/v2 v2.3.0 // indirect
	github.com/klauspost/pgzip v1.2.6 // indirect
	github.com/kr/fs v0.1.0 // indirect
	github.com/leaanthony/clir v1.7.0 // indirect
	github.com/leaanthony/dmg v0.0.0-20260731074841-5c28840cf819 // indirect
	github.com/lima-vm/go-qcow2reader v0.8.0 // indirect
	github.com/lithammer/fuzzysearch v1.1.8 // indirect
	github.com/lmittmann/tint v1.1.3 // indirect
	github.com/lucasb-eyer/go-colorful v1.3.0 // indirect
	github.com/magiconair/properties v1.18.11 // indirect
	github.com/mattn/go-colorable v0.1.15 // indirect
	github.com/mattn/go-localereader v0.0.1 // indirect
	github.com/mattn/go-runewidth v0.0.27 // indirect
	github.com/mattn/go-shellwords v1.0.15 // indirect
	github.com/mattn/go-zglob v0.0.6 // indirect
	github.com/mdlayher/netlink v1.11.2 // indirect
	github.com/mdlayher/socket v0.6.0 // indirect
	github.com/mdlayher/vsock v1.3.0 // indirect
	github.com/mgutz/ansi v0.0.0-20170206155736-9520e82c474b // indirect
	github.com/microcosm-cc/bluemonday v1.0.27 // indirect
	github.com/mikefarah/yq/v4 v4.53.6 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/go-wordwrap v1.0.1 // indirect
	github.com/mitchellh/hashstructure/v2 v2.0.2 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modelcontextprotocol/go-sdk v1.8.0 // indirect
	github.com/muesli/ansi v0.0.0-20230316100256-276c6243b2f6 // indirect
	github.com/muesli/cancelreader v0.2.2 // indirect
	github.com/muesli/reflow v0.3.0 // indirect
	github.com/muesli/termenv v0.16.0 // indirect
	github.com/nfnt/resize v0.0.0-20180221191011-83c6a9932646 // indirect
	github.com/nxadm/tail v1.4.11 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/pb33f/ordered-map/v2 v2.3.1 // indirect
	github.com/pierrec/lz4/v4 v4.1.26 // indirect
	github.com/pjbgf/sha1cd v0.6.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pterm/pterm v0.12.82 // indirect
	github.com/radovskyb/watcher v1.0.7 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/rjeczalik/notify v0.9.3 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/sabhiram/go-gitignore v0.0.0-20210923224102-525f6e181f06 // indirect
	github.com/sajari/fuzzy v1.0.0 // indirect
	github.com/santhosh-tekuri/jsonschema/v6 v6.0.3 // indirect
	github.com/segmentio/asm v1.2.1 // indirect
	github.com/segmentio/encoding v0.5.4 // indirect
	github.com/sergi/go-diff v1.4.0 // indirect
	github.com/sethvargo/go-password v0.4.0 // indirect
	github.com/shopspring/decimal v1.4.0 // indirect
	github.com/skeema/knownhosts v1.3.2 // indirect
	github.com/spakin/netpbm v1.3.2 // indirect
	github.com/spf13/cast v1.10.0 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	github.com/tc-hib/winres v0.3.1 // indirect
	github.com/u-root/uio v0.0.0-20240224005618-d2acac8f3701 // indirect
	github.com/ulikunitz/xz v0.5.15 // indirect
	github.com/wailsapp/task/v3 v3.40.1-patched3 // indirect
	github.com/wailsapp/wails/v3 v3.1.0 // indirect
	github.com/xanzy/ssh-agent v0.3.3 // indirect
	github.com/xo/terminfo v0.0.0-20220910002029-abceb7e1c41e // indirect
	github.com/yosida95/uritemplate/v3 v3.0.2 // indirect
	github.com/yuin/goldmark v1.7.17 // indirect
	github.com/yuin/goldmark-emoji v1.0.6 // indirect
	github.com/yuin/gopher-lua v1.1.2 // indirect
	github.com/zalando/go-keyring v0.2.6 // indirect
	github.com/zclconf/go-cty v1.19.0 // indirect
	github.com/zeebo/xxh3 v1.1.0 // indirect
	gitlab.com/digitalxero/go-conventional-commit v1.0.7 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.yaml.in/yaml/v4 v4.0.0-rc.6 // indirect
	golang.org/x/image v0.46.0 // indirect
	golang.org/x/oauth2 v0.36.0 // indirect
	golang.org/x/sync v0.23.0 // indirect
	golang.org/x/text v0.42.0 // indirect
	golang.org/x/time v0.15.0 // indirect
	golang.org/x/tools v0.49.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260526163538-3dc84a4a5aaa // indirect
	google.golang.org/grpc v1.83.2 // indirect
	google.golang.org/protobuf v1.36.12 // indirect
	gopkg.in/op/go-logging.v1 v1.0.0-20160211212156-b2cb9fa56473 // indirect
	gopkg.in/tomb.v1 v1.0.0-20141024135613-dd632973f1e7 // indirect
	gopkg.in/warnings.v0 v0.1.2 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	gvisor.dev/gvisor v0.0.0-20240916094835-a174eb65023f // indirect
	howett.net/plist v1.0.2-0.20250314012144-ee69052608d9 // indirect
	mvdan.cc/sh/v3 v3.12.0 // indirect
)

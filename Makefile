# Claude Eden: Windows desktop with a local Debian 13 guest.
#
#   make              build the Windows desktop and guest assets
#   make eden-windows build the Windows desktop
#   make limactl      Lima's own limactl from third_party/, for debugging
#   make test         the Go tests
#   make check-windows      the Windows build and tests compile and vet, from any OS
#   make test-third-party   the tests of the modules in third_party/, patched

GO ?= go
NODE ?= node
WORKSPACE = $(NODE) "$(CURDIR)/scripts/run-in-workspace.mjs"
ifeq ($(OS),Windows_NT)
NPM = $(NODE) "$(shell $(WORKSPACE) -- $(NODE) -p "require('node:path').join(require('node:path').dirname(process.execPath), 'node_modules/npm/bin/npm-cli.js')")"
else
NPM = npm
endif
TAILNET_GO ?= $(GO)
GOOS ?= $(shell $(WORKSPACE) -- $(GO) env GOOS)
GOARCH ?= $(shell $(WORKSPACE) -- $(GO) env GOARCH)
# Claude Eden's VERSION file owns the default. Upstream release tooling can still
# supply VERSION explicitly when building a historical agent-vm tag.
VERSION ?= $(shell tr -d '\r\n' < VERSION)
LIMA_VERSION := $(shell cat internal/limaembed/lima-version)

ifeq ($(GOARCH),amd64)
GUEST_ARCH := x86_64
else ifeq ($(GOARCH),arm64)
GUEST_ARCH := aarch64
else
GUEST_ARCH := $(GOARCH)
endif
EXE = $(if $(filter windows,$(GOOS)),.exe,)

ASSETS := internal/limaembed/assets
LDFLAGS := -s -w \
	-X github.com/sylvinus/agent-vm/internal/version.Version=$(VERSION) \
	-X github.com/lima-vm/lima/v2/pkg/version.Version=$(LIMA_VERSION)
# yq only edits Lima's YAML here: without its other formats (Lua, HCL, TOML,
# XML...), 10% smaller. Tests run with the same tags.
TAGS := yq_noxml,yq_nolua,yq_noini,yq_nohcl,yq_notoml,yq_noprops,yq_nocsv,yq_nouri,yq_nobase64,yq_nosh,yq_noshell
# The Windows host and Linux guest helpers build without a C compiler.
CGO_ENABLED ?= 0
GO_BUILD = CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) $(WORKSPACE) -- $(GO) build -trimpath -tags $(TAGS) -ldflags "$(LDFLAGS)"

.PHONY: all limactl assets test test-go test-runner check-windows test-third-party fuzz clean

all: eden-windows

eden: eden-windows

eden-ui:
	$(WORKSPACE) -- $(NPM) --prefix ui ci --no-audit --no-fund
	$(WORKSPACE) -- $(NPM) run --prefix ui build

.PHONY: eden eden-ui eden-windows package-windows check-linux

check-linux:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 $(WORKSPACE) -- $(GO) vet -tags $(TAGS) . ./cmd/... ./internal/...

eden-windows:
	$(WORKSPACE) -- pwsh -NoProfile -File scripts/build-windows.ps1

package-windows:
	$(WORKSPACE) -- pwsh -NoProfile -File scripts/package-windows.ps1

limactl:
	$(GO_BUILD) -o _output/bin/limactl$(EXE) github.com/lima-vm/lima/v2/cmd/limactl
	mkdir -p _output/share/lima
	CGO_ENABLED=0 GOOS=linux GOARCH=$(GOARCH) $(WORKSPACE) -- $(GO) build -trimpath -ldflags "-s -w" \
		-o _output/share/lima/lima-guestagent.Linux-$(GUEST_ARCH) github.com/lima-vm/lima/v2/cmd/lima-guestagent
	$(WORKSPACE) -- $(NODE) scripts/prepare-lima-templates.mjs _output/share/lima/templates

# Build each guest helper for the OS in which it will run.
assets:
	CGO_ENABLED=0 GOOS=linux GOARCH=$(GOARCH) $(WORKSPACE) -- $(GO) build -trimpath \
		-ldflags "-s -w -X github.com/lima-vm/lima/v2/pkg/version.Version=$(LIMA_VERSION)" \
		-o $(ASSETS)/lima-guestagent.Linux-$(GUEST_ARCH) github.com/lima-vm/lima/v2/cmd/lima-guestagent
	gzip -9 -n -f $(ASSETS)/lima-guestagent.Linux-$(GUEST_ARCH)
	$(WORKSPACE) -- $(NODE) scripts/prepare-lima-templates.mjs $(ASSETS)/templates

test: test-runner test-go

test-runner:
	$(WORKSPACE) -- $(NODE) --test scripts/run-in-workspace.test.mjs scripts/prepare-lima-templates.test.mjs scripts/generate-windows-cleanup.test.mjs

test-go:
	$(WORKSPACE) --cwd shared/guestidentity -- $(GO) test ./...
	$(WORKSPACE) -- $(GO) test -tags $(TAGS) . ./cmd/... ./internal/...
	GOFLAGS= $(WORKSPACE) --cwd tools/tailnet -- $(TAILNET_GO) test ./...

# The Windows build and its tests compile and vet, from any OS. The Windows
# paths' logic is tested everywhere (paths.Windows); the tests themselves
# run on Windows in CI (windows-latest).
check-windows:
	GOOS=windows GOARCH=amd64 $(WORKSPACE) -- $(GO) vet -tags $(TAGS) . ./cmd/... ./internal/...
	GOOS=windows GOARCH=arm64 $(WORKSPACE) -- $(GO) vet -tags $(TAGS) . ./cmd/... ./internal/...

# From this module, so that each library is built with the others patched.
# Lima's MCP packages and sshocker's command are not part of agent-vm (-e:
# listing them needs sums nothing built here uses; grep keeps them out of
# the tests).
test-third-party:
	$(WORKSPACE) -- $(GO) test -vet=off -race github.com/pkg/sftp/...
	$(WORKSPACE) -- $(GO) test -race github.com/lima-vm/sshocker/pkg/...
	$(WORKSPACE) -- $(GO) test $$($(WORKSPACE) -- $(GO) list -e github.com/lima-vm/lima/v2/pkg/... | grep -v /mcp)
	$(WORKSPACE) -- $(GO) test github.com/containers/gvisor-tap-vsock/pkg/...

# Each fuzzer for FUZZTIME (their seeds run with the tests). The vendored
# sshocker's through tests/fuzz/go.work, which makes it fuzzable.
FUZZTIME ?= 1m
FUZZ = github.com/lima-vm/sshocker/pkg/reversesshfs github.com/pkg/sftp ./internal/env ./internal/gitguard \
	./internal/mounts ./internal/paths ./internal/runscript ./internal/ui ./internal/vmname
fuzz:
	@set -e; for pkg in $(FUZZ); do \
	  case $$pkg in ./*) work=off ;; *) work=$(CURDIR)/tests/fuzz/go.work ;; esac; \
	  fs=$$(GOWORK=$$work $(WORKSPACE) -- $(GO) test -vet=off -list '^Fuzz' $$pkg) || { echo "$$fs"; exit 1; }; \
	  for f in $$(echo "$$fs" | grep '^Fuzz'); do \
	    echo "== $$pkg $$f"; \
	    GOWORK=$$work $(WORKSPACE) -- $(GO) test -vet=off -run '^$$' -fuzz "^$$f$$" -fuzztime $(FUZZTIME) $$pkg; \
	  done; \
	done

clean:
	rm -rf _output
	find $(ASSETS) -mindepth 1 ! -name README -exec rm -rf {} +

.PHONY: tailnet-assets
tailnet-assets:
	GO="$(TAILNET_GO)" $(WORKSPACE) -- bash scripts/build-tailnet.sh linux amd64 "$(CURDIR)/_output/tailnet/workbench-tailnet-linux-amd64"
	GO="$(TAILNET_GO)" $(WORKSPACE) -- bash scripts/build-tailnet.sh linux arm64 "$(CURDIR)/_output/tailnet/workbench-tailnet-linux-arm64"
	gzip -n -c _output/tailnet/workbench-tailnet-linux-amd64 > internal/tailnetassets/assets/workbench-tailnet-linux-amd64.gz
	gzip -n -c _output/tailnet/workbench-tailnet-linux-arm64 > internal/tailnetassets/assets/workbench-tailnet-linux-arm64.gz

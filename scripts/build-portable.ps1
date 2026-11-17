#requires -Version 7.0
[CmdletBinding()]
param(
    [string]$AppDirectory = '_output/windows-amd64/app',
    [string]$OutputDirectory = '.tmp/ClaudeEden-portable'
)
$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'windows-common.ps1')
$root = Split-Path $PSScriptRoot -Parent
$app = [IO.Path]::GetFullPath($AppDirectory, $root)
$portable = [IO.Path]::GetFullPath($OutputDirectory, $root)
$go = Join-Path $root '.tmp/toolchains/go/bin/go.exe'
$version = (Get-Content (Join-Path $root 'VERSION') -Raw).Trim()
$lima = (Get-Content (Join-Path $root 'internal/limaembed/lima-version') -Raw).Trim()
foreach ($target in @($app, $portable)) {
    & node --input-type=module -e "import { assertWorkspacePath } from './scripts/run-in-workspace.mjs'; assertWorkspacePath(process.argv[1], process.argv[2]);" $root $target
    if ($LASTEXITCODE -ne 0) { throw "Portable build path is outside the verified workspace: $target" }
}
$relativeOutput = [IO.Path]::GetRelativePath($root, $portable)
if ($relativeOutput -notmatch '^(?:\.tmp|_output)[\\/]') { throw 'Portable output must be inside the workspace .tmp or _output directory.' }
if (-not (Test-Path -LiteralPath $app -PathType Container)) { throw "Portable source is missing: $app" }
if (-not (Test-Path -LiteralPath (Join-Path $app 'ClaudeEden.exe') -PathType Leaf)) { throw 'Portable source lacks ClaudeEden.exe.' }
if (Test-Path -LiteralPath (Join-Path $app 'data')) { throw 'Portable source contains runtime data. Use the clean Windows application build, not an in-use portable directory.' }
if (Test-Path -LiteralPath $portable) { throw 'Portable output already exists. Preserve it or choose a new output directory.' }
New-Item -ItemType Directory -Path $portable -Force | Out-Null
Get-ChildItem -LiteralPath $app -Force | Copy-Item -Destination $portable -Recurse -Force
foreach ($name in @('Uninstall.exe', 'Eden.exe', 'Eden.cleanup.md', 'Eden.install-files.json', 'DEVELOPMENT-BUILD.txt', 'README.txt', 'PORTABLE-README.txt')) {
    Remove-Item -LiteralPath (Join-Path $portable $name) -Recurse -Force -ErrorAction SilentlyContinue
}
foreach ($directory in @('resources/runtime/qemu/share/doc', 'resources/runtime/qemu/share/man', 'resources/runtime/qemu/share/icons', 'resources/runtime/qemu/share/applications', 'resources/runtime/qemu/share/locale')) {
    Remove-Item -LiteralPath (Join-Path $portable $directory) -Recurse -Force -ErrorAction SilentlyContinue
}
Remove-Item -LiteralPath (Join-Path $portable 'version.txt') -Force -ErrorAction SilentlyContinue
Remove-Item -LiteralPath (Join-Path $portable 'LICENSE.txt') -Force -ErrorAction SilentlyContinue
Remove-Item -LiteralPath (Join-Path $portable 'resources/licenses') -Recurse -Force -ErrorAction SilentlyContinue
Get-ChildItem -LiteralPath $portable -Recurse -File | Where-Object {
    $_.Extension -in @('.md', '.markdown', '.txt', '.html', '.htm', '.rst', '.1', '.5', '.7', '.pdf', '.rtf', '.adoc') -or
    $_.Name -match '^(?:licen[sc]e|copying|notice)(?:[._-]|$)'
} | Remove-Item -Force
$tags = 'production,external_hcs,external_wsl2,yq_noxml,yq_nolua,yq_noini,yq_nohcl,yq_notoml,yq_noprops,yq_nocsv,yq_nouri,yq_nobase64,yq_nosh,yq_noshell'
$ldflags = "-s -w -X github.com/sylvinus/agent-vm/internal/version.Version=$version -X github.com/lima-vm/lima/v2/pkg/version.Version=$lima"
& $go build -trimpath -tags $tags -ldflags $ldflags -o (Join-Path $portable 'resources/eden.exe') ./cmd/eden
if ($LASTEXITCODE -ne 0) { throw 'Portable backend build failed.' }
& $go build -trimpath -tags production -ldflags "$ldflags -H windowsgui" -o (Join-Path $portable 'ClaudeEden.exe') ./cmd/eden-desktop
if ($LASTEXITCODE -ne 0) { throw 'Portable desktop build failed.' }
& (Join-Path $portable 'resources/eden.exe') verify-vm-assets
if ($LASTEXITCODE -ne 0) { throw 'Portable VM asset verification failed.' }
& (Join-Path $portable 'resources/eden.exe') verify-web-assets
if ($LASTEXITCODE -ne 0) { throw 'Portable web asset verification failed.' }
New-Item -ItemType Directory -Path (Join-Path $portable 'data/profile'), (Join-Path $portable 'data/state'), (Join-Path $portable 'data/user'), (Join-Path $portable 'data/temp') -Force | Out-Null
[IO.File]::WriteAllText((Join-Path $portable 'portable.flag'), "ClaudeEden portable data root`n", [Text.UTF8Encoding]::new($false))
Write-Output "Portable ClaudeEden: $portable"

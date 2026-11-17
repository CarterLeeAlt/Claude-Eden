#requires -Version 7.0
[CmdletBinding()]
param(
    [switch]$Release,
    [switch]$Sign,
    [switch]$SkipBuild,
    [switch]$VerifyDesktop,
    [string]$AppDirectory,
    [string]$OutputDirectory,
    [string]$DependencyCache,
    [string]$QemuSourceArchive
)
. (Join-Path $PSScriptRoot 'windows-common.ps1')
$repository = Split-Path $PSScriptRoot -Parent
$version = Get-EdenVersion $repository
$signed = $Release -or $Sign
if (-not $AppDirectory) { $AppDirectory = Join-Path $repository '_output/windows-amd64/app' }
if (-not $OutputDirectory) { $OutputDirectory = Join-Path $repository "_output/releases/$version/windows" }
if (-not $DependencyCache) { $DependencyCache = Join-Path $repository '_output/windows-deps' }
$AppDirectory = [IO.Path]::GetFullPath($AppDirectory)
$OutputDirectory = [IO.Path]::GetFullPath($OutputDirectory)
$DependencyCache = [IO.Path]::GetFullPath($DependencyCache)
if (Test-Path -LiteralPath (Join-Path $AppDirectory 'EdenData')) { throw 'App directory contains EdenData user data. Package a clean build directory; do not package an installed application.' }
if (-not $SkipBuild) { & (Join-Path $PSScriptRoot 'build-windows.ps1') -Release:$Release -Sign:$Sign -VerifyDesktop:$VerifyDesktop -OutputDirectory $AppDirectory -DependencyCache $DependencyCache }
if ((Get-Content -LiteralPath (Join-Path $AppDirectory 'Eden.install-id') -Raw) -ne 'io.github.carterleealt.claude-eden') { throw 'Invalid Claude Eden app directory.' }
if ((Get-Content -LiteralPath (Join-Path $AppDirectory 'version.txt') -Raw).Trim() -ne $version) { throw 'App version differs from VERSION; rebuild before packaging.' }
$makeNSIS = Get-BuildCommand @('makensis.exe', 'makensis')
$sevenZip = Get-BuildCommand @('7z.exe', '7zz', '7z')
if ($IsWindows) {
    if ([Diagnostics.FileVersionInfo]::GetVersionInfo((Join-Path $AppDirectory 'ClaudeEden.exe')).ProductVersion -ne $version) { throw 'ClaudeEden.exe version differs from VERSION; rebuild before packaging.' }
    Invoke-BuildTool (Join-Path $AppDirectory 'resources/eden.exe') @('verify-vm-assets')
    Invoke-BuildTool (Join-Path $AppDirectory 'resources/eden.exe') @('verify-web-assets')
    if ($SkipBuild -and $VerifyDesktop) { Assert-WindowsWorkbench $AppDirectory }
} else {
    $metadata = & $sevenZip 'l' '-tPE' (Join-Path $AppDirectory 'ClaudeEden.exe')
    if ($LASTEXITCODE -ne 0 -or -not ($metadata -match ('^ProductVersion: ' + [regex]::Escape($version) + '$'))) { throw 'ClaudeEden.exe version differs from VERSION; rebuild before packaging.' }
}
$signTool = $null
if ($signed) {
    Assert-WindowsSigningConfiguration
    if ($env:EDEN_WINDOWS_CERT_SHA1 -notmatch '^[0-9A-Fa-f]{40}$') { throw 'Build and verify the signed app before packaging.' }
    if (Test-Path -LiteralPath (Join-Path $AppDirectory 'DEVELOPMENT-BUILD.txt')) { throw 'Rebuild with build-windows.ps1 -Sign or -Release before packaging a signed installer.' }
    if ($Release -and (-not $QemuSourceArchive -or -not (Test-Path -LiteralPath $QemuSourceArchive -PathType Leaf))) {
        throw 'Release packaging requires -QemuSourceArchive containing the exact QEMU, firmware and linked library sources. Generate it from the built runtime with bundle-qemu-sources.sh.'
    }
    if ($Release) { Invoke-BuildTool $sevenZip @('t', '-bso0', '-bsp0', $QemuSourceArchive) }
    $signTool = Get-SigningTool
    foreach ($file in @('ClaudeEden.exe', 'resources/eden.exe', 'resources/eden-install.exe', 'resources/mihomo.exe')) {
        $signature = Get-AuthenticodeSignature -LiteralPath (Join-Path $AppDirectory $file)
        if ($signature.Status -ne 'Valid' -or $signature.SignerCertificate.Thumbprint -ne $env:EDEN_WINDOWS_CERT_SHA1) { throw "Invalid release signature: $file" }
    }
    $webview = Get-AuthenticodeSignature -LiteralPath (Join-Path $AppDirectory 'resources/runtime/MicrosoftEdgeWebView2RuntimeInstallerX64.exe')
    if ($webview.Status -ne 'Valid' -or $webview.SignerCertificate.Subject -notmatch 'O=Microsoft Corporation') { throw 'WebView2 installer must have a valid Microsoft signature.' }
}
New-Item -ItemType Directory -Path $OutputDirectory -Force | Out-Null
# Verify source material before the expensive installer compression.
$dependencies = Get-Content -LiteralPath (Join-Path $repository 'packaging/windows/dependencies.json') -Raw | ConvertFrom-Json
$qemuTar = Get-VerifiedGitArchive $dependencies.qemuSource $DependencyCache
$suffix = if ($Release) { '' } elseif ($Sign) { '-test-signed' } else { '-unsigned' }
$artifact = Join-Path $OutputDirectory "Eden-$version-windows-amd64-setup$suffix.exe"
# Rebuild the relocatable guide before hashing the payload; no install-time Markdown rewrite.
Invoke-BuildTool (Get-BuildCommand @('node.exe', 'node')) @((Join-Path $PSScriptRoot 'generate-windows-cleanup.mjs'), $AppDirectory)
$manifest = Join-Path $AppDirectory 'Eden.install-files.json'
$numericVersion = ($version -split '-')[0] + '.0'
$prefix = if ($IsWindows) { '/' } else { '-' }
# Source files are UTF-8; Windows ACP must not reinterpret Chinese UI text.
$arguments = @("${prefix}V3", "${prefix}INPUTCHARSET", 'UTF8', "${prefix}DAPP_DIR=$AppDirectory", "${prefix}DVERSION=$version", "${prefix}DVERSION_NUMERIC=$numericVersion", "${prefix}DOUTPUT_FILE=$artifact", "${prefix}DFILE_MANIFEST=$manifest")
if ($signed) {
    $arguments += @("${prefix}DSIGN_PWSH=$((Get-Process -Id $PID).Path)", "${prefix}DSIGN_SCRIPT=$(Join-Path $PSScriptRoot 'sign-windows-file.ps1')")
}
$arguments += (Join-Path $repository 'packaging/windows/Eden.nsi')
Invoke-BuildTool $makeNSIS $arguments
Invoke-BuildTool (Get-BuildCommand @('node.exe', 'node')) @(
    (Join-Path $PSScriptRoot 'verify-windows-installer-icon.mjs'),
    $artifact, (Join-Path $repository 'packaging/windows/Eden.ico'), $sevenZip
)
if ($signed) { Sign-EdenFile $artifact $signTool }
if ((Get-Item -LiteralPath $artifact).Length -gt 512MB) { throw 'Installer exceeds the application updater 512 MiB limit.' }

# Source materials travel beside the binary, not inside the application directory.
$coreSource = Join-Path $OutputDirectory "mihomo-$($dependencies.mihomo.version)-source.tar.gz"
$verifiedCoreSource = Get-VerifiedDependency $dependencies.mihomoSource $DependencyCache
Copy-Item -LiteralPath $verifiedCoreSource -Destination $coreSource -Force
Invoke-BuildTool $sevenZip @('t', '-bso0', '-bsp0', $coreSource)
$releaseFiles = @($artifact, $coreSource)
$qemuSource = Join-Path $OutputDirectory ($dependencies.qemuSource.file + '.gz')
if (Test-Path -LiteralPath $qemuSource) { Remove-Item -LiteralPath $qemuSource }
Invoke-BuildTool $sevenZip @('a', '-tgzip', '-mx=9', '-bso0', '-bsp0', $qemuSource, $qemuTar)
Invoke-BuildTool $sevenZip @('t', '-bso0', '-bsp0', $qemuSource)
$releaseFiles += $qemuSource
$qemuRuntime = Join-Path $AppDirectory 'resources/runtime/qemu'
if (Test-Path (Join-Path $qemuRuntime 'eden-build.json')) {
    $keymapArchive = Get-VerifiedDependency $dependencies.qemuKeycodemap $DependencyCache
    $keymapSource = Join-Path $OutputDirectory ('qemu-keycodemapdb-' + $dependencies.qemuKeycodemap.version + '-source.tar.gz')
    Copy-Item $keymapArchive $keymapSource -Force
    $changesSource = Join-Path $OutputDirectory ('qemu-' + $dependencies.qemu.version + '-eden-changes-source.tar.gz')
    Invoke-BuildTool 'tar' @('-czf', $changesSource, '-C', $repository, 'patches/qemu', 'scripts/build-qemu-windows.ps1',
        'scripts/build-qemu-windows.sh', '-C', $qemuRuntime, 'eden-build.json', 'build-packages.txt')
    $releaseFiles += @($keymapSource, $changesSource)
}
if ($Release) {
    $sourceExtension = if ($QemuSourceArchive.EndsWith('.tar.gz', [StringComparison]::OrdinalIgnoreCase)) { '.tar.gz' } else { [IO.Path]::GetExtension($QemuSourceArchive) }
    $qemuSourceTarget = Join-Path $OutputDirectory ('qemu-' + $dependencies.qemu.version + '-windows-full-source' + $sourceExtension)
    Copy-Item -LiteralPath $QemuSourceArchive -Destination $qemuSourceTarget -Force
    $releaseFiles += $qemuSourceTarget
}
$sums = foreach ($file in $releaseFiles) { (Get-FileHash -LiteralPath $file -Algorithm SHA256).Hash.ToLowerInvariant() + '  ' + [IO.Path]::GetFileName($file) }
[IO.File]::WriteAllLines((Join-Path $OutputDirectory 'SHA256SUMS'), $sums)
Write-Host "Windows installer: $artifact"
if (-not $signed) { Write-Warning 'Unsigned development installer. Do not publish it as an automatic update; target Windows-host runtime and installation acceptance are still required.' }

#requires -Version 7.0
# Disposable CI system integration: real installation, signing stores and optional cross-volume storage.
[CmdletBinding()]
param([switch]$Release, [switch]$SystemIntegration)
. (Join-Path $PSScriptRoot 'windows-common.ps1')
if (-not $IsWindows) { throw 'This CI entry point requires a disposable Windows runner.' }
if (-not $SystemIntegration) { throw 'This entry performs real installation/certificate integration. Use -SystemIntegration only in an explicitly authorized disposable environment.' }
if ($env:EDEN_BUILD_REF -notmatch '^[0-9A-Fa-f]{40}$') { throw 'EDEN_BUILD_REF must be a complete commit SHA.' }
$repository = Split-Path $PSScriptRoot -Parent
Push-Location $repository
$pfx = $null
$certificate = $null
$certificates = @()
$qemuSource = $null
$savedPath = $env:PATH
$savedGoRoot = $env:GOROOT
$savedCertificate = $env:EDEN_WINDOWS_CERT_SHA1
$savedStorageVolume = $env:EDEN_TEST_STORAGE_VOLUME
$storageTestRoot = $null
# One step runs every phase, because signing state must not outlive this
# process. Name each phase so a failure shows where it happened and how long
# the earlier phases took, without reading the whole log.
$script:phase = $null
$script:phaseStarted = $null
$script:phases = [System.Collections.Generic.List[object]]::new()
function Complete-CiPhase {
    param([string]$Result = 'passed')
    if (-not $script:phase) { return }
    $seconds = [int]([DateTime]::UtcNow - $script:phaseStarted).TotalSeconds
    $script:phases.Add([pscustomobject]@{ Name = $script:phase; Seconds = $seconds; Result = $Result })
    if ($env:GITHUB_ACTIONS) { Write-Host '::endgroup::' }
    Write-Host "Phase $Result after ${seconds}s: $($script:phase)"
    $script:phase = $null
}
function Enter-CiPhase {
    param([Parameter(Mandatory)][string]$Name)
    Complete-CiPhase
    $script:phase = $Name
    $script:phaseStarted = [DateTime]::UtcNow
    if ($env:GITHUB_ACTIONS) { Write-Host "::group::$Name" } else { Write-Host "== $Name" }
}
function Write-CiPhaseSummary {
    if (-not $env:GITHUB_STEP_SUMMARY -or $script:phases.Count -eq 0) { return }
    $lines = @('| Windows phase | Seconds | Result |', '| --- | ---: | --- |')
    $lines += $script:phases | ForEach-Object { "| $($_.Name) | $($_.Seconds) | $($_.Result) |" }
    $lines + '' | Out-File -FilePath $env:GITHUB_STEP_SUMMARY -Append -Encoding utf8
}
try {
    $env:GOROOT = (& (Get-BuildCommand @('go')) env GOROOT).Trim()
    if ($LASTEXITCODE -ne 0) { throw 'Could not resolve the selected Go toolchain.' }
    Enter-CiPhase 'Checkout identity and script tests'
    $actual = (& git rev-parse HEAD).Trim()
    if ($LASTEXITCODE -ne 0 -or $actual -ne $env:EDEN_BUILD_REF) { throw 'Checkout differs from the requested commit.' }
    Invoke-BuildTool 'node' @('scripts/check-version.mjs')
    Invoke-BuildTool 'node' @('--test', 'internal/eden/browser-clipboard.test.mjs')
    if ($Release) {
        Enter-CiPhase 'Release signing configuration'
        if ($env:GITHUB_EVENT_NAME -ne 'workflow_dispatch') { throw 'Release candidates require an explicitly dispatched release workflow.' }
        if ($env:EDEN_WINDOWS_SIGNING_PROVIDER -eq 'esigner') {
            foreach ($name in @('EDEN_ESIGNER_USERNAME', 'EDEN_ESIGNER_PASSWORD', 'EDEN_ESIGNER_CREDENTIAL_ID', 'EDEN_ESIGNER_CERTIFICATE_SERIAL', 'EDEN_ESIGNER_TOTP_SECRET')) {
                if (-not [Environment]::GetEnvironmentVariable($name)) { throw "Missing eden-release setting: $name" }
            }
            Assert-WindowsSigningConfiguration
            Invoke-BuildTool 'node' @('scripts/esigner.mjs', 'install')
        } elseif ($env:EDEN_WINDOWS_SIGNING_PROVIDER -and $env:EDEN_WINDOWS_SIGNING_PROVIDER -ne 'certificate-store') {
            throw 'Unknown Windows signing provider.'
        } elseif (-not $env:EDEN_WINDOWS_PFX_BASE64 -or -not $env:EDEN_WINDOWS_PFX_PASSWORD) {
            throw 'Configure EDEN_WINDOWS_PFX_BASE64 and EDEN_WINDOWS_PFX_PASSWORD in the eden-release environment.'
        }
        if ($env:EDEN_WINDOWS_SIGNING_PROVIDER -ne 'esigner') {
            $pfx = Join-Path $env:RUNNER_TEMP ('eden-signing-' + [guid]::NewGuid().ToString('N') + '.pfx')
            [IO.File]::WriteAllBytes($pfx, [Convert]::FromBase64String($env:EDEN_WINDOWS_PFX_BASE64))
            $password = ConvertTo-SecureString $env:EDEN_WINDOWS_PFX_PASSWORD -AsPlainText -Force
            $certificates = @(Import-PfxCertificate -FilePath $pfx -CertStoreLocation Cert:\CurrentUser\My -Password $password)
            Remove-Item -LiteralPath $pfx -Force
            $signers = @($certificates | Where-Object {
                $_.HasPrivateKey -and @($_.EnhancedKeyUsageList | Where-Object { $_.ObjectId -eq '1.3.6.1.5.5.7.3.3' }).Count -gt 0
            })
            if ($signers.Count -ne 1) { throw 'PFX must contain exactly one code-signing certificate with its private key.' }
            $certificate = $signers[0]
            $env:EDEN_WINDOWS_CERT_SHA1 = $certificate.Thumbprint
            Remove-Item Env:EDEN_WINDOWS_PFX_BASE64, Env:EDEN_WINDOWS_PFX_PASSWORD
        }
    }

    Enter-CiPhase 'Installer compiler and installation helper tests'
    & (Join-Path $PSScriptRoot 'install-nsis.ps1') -Destination $env:RUNNER_TEMP
    $env:PATH = (Join-Path $env:RUNNER_TEMP 'nsis-3.13') + [IO.Path]::PathSeparator + $env:PATH
    Invoke-BuildTool (Get-BuildCommand @('pwsh.exe', 'pwsh')) @('-NoProfile', '-NonInteractive', '-File', 'scripts/windows-workbench-smoke.test.ps1')
    # NSIS is x86 and resolves $SYSDIR to SysWOW64. Match its real helper host.
    $installationPowerShell = Join-Path $env:SystemRoot 'SysWOW64/WindowsPowerShell/v1.0/powershell.exe'
    $installHelper = Join-Path $repository '.tmp/tools/eden-install.exe'
    New-Item -ItemType Directory -Path (Split-Path $installHelper -Parent) -Force | Out-Null
    Invoke-BuildTool 'go' @('build', '-trimpath', '-o', $installHelper, './cmd/eden-install')
    & (Join-Path $PSScriptRoot 'test-windows-installation-files.ps1') -HelperShell $installationPowerShell -InstallHelper $installHelper -InstallerSmoke
    if ($Release) {
        Enter-CiPhase 'Corresponding sources'
        & (Join-Path $PSScriptRoot 'prepare-qemu-release.ps1')
        $qemuSource = Join-Path $repository '_output/windows-qemu-sources/full-source.tar.gz'
    }
    Enter-CiPhase $(if ($Release) { 'Build and sign the application' } else { 'Build the application' })
    & (Join-Path $PSScriptRoot 'build-windows.ps1') -Release:$Release -VerifyDesktop
    Enter-CiPhase 'Go download and desktop shell tests'
    Invoke-BuildTool 'node' @('--import', './ui/node_modules/tsx/dist/loader.mjs', '--test', 'scripts/windows-native-bridge.test.mjs')
    Invoke-BuildTool 'go' @('test', './internal/hostproxy', './cmd/eden', '-run', '^Test(ParallelPackage|DownloadLockCancellationAndReuse)', '-count=1')
    Invoke-BuildTool 'go' @('test', './internal/desktop/...', './internal/desktopupdate/...', './internal/installfiles/...')
    # Only an explicitly configured integration location may use another volume.
    if ($env:EDEN_TEST_STORAGE_VOLUME) {
        $target = [IO.Path]::GetFullPath($env:EDEN_TEST_STORAGE_VOLUME)
        if (-not [IO.Path]::IsPathFullyQualified($env:EDEN_TEST_STORAGE_VOLUME) -or $target -eq [IO.Path]::GetPathRoot($target)) { throw 'EDEN_TEST_STORAGE_VOLUME must be an absolute dedicated directory, not a volume root.' }
        if ([IO.Path]::GetPathRoot($target) -eq [IO.Path]::GetPathRoot($env:TEMP)) { throw 'EDEN_TEST_STORAGE_VOLUME must be on a different volume from workspace temp.' }
        $ancestor = $target
        while ($ancestor) {
            if (Test-Path -LiteralPath $ancestor) {
                $item = Get-Item -LiteralPath $ancestor -Force
                if (-not $item.PSIsContainer -or ($item.Attributes -band [IO.FileAttributes]::ReparsePoint)) { throw "Cross-volume location has a non-directory or reparse ancestor: $ancestor" }
            }
            $ancestor = Split-Path $ancestor -Parent
        }
        if (Test-Path -LiteralPath $target) { throw 'Use a new dedicated EDEN_TEST_STORAGE_VOLUME directory; existing data is not a test fixture.' }
        $storageTestRoot = (New-Item -ItemType Directory -Path $target).FullName
        $env:EDEN_TEST_STORAGE_VOLUME = $storageTestRoot
        Write-Host "Explicit cross-volume integration directory: $storageTestRoot"
    }
    & {
        Invoke-BuildTool 'go' @('test', './internal/workbench', './internal/appupdate', './internal/host', './internal/managedproxy', './internal/env', './internal/paths', './internal/mounts')
        Invoke-BuildTool 'go' @('test', './internal/eden', '-run', 'Test(Storage|Gate|ProxyRevision|ConfigChange|JobStop|CONNECT|SOCKS|DNS|RejectHostLAN|Credentials|ProxyHostname|SafetyReport|Renew)')
        Invoke-BuildTool 'go' @('test', './internal/vm', '-run', 'Test(NetworkConfined|WindowsQEMUIdentity)')
        Invoke-BuildTool 'go' @('test', 'github.com/lima-vm/lima/v2/pkg/driver/qemu', '-run', 'Test(WindowsFirmwareArgument|WindowsPackagedFirmwareBeatsOldUserCopy|WHPXQEMU64Features|WindowsCPUResolution)$')
        Invoke-BuildTool 'go' @('test', 'github.com/lima-vm/lima/v2/pkg/hostagent', '-run', '^TestResourceProbeCommand$')
        Invoke-BuildTool 'go' @('test', 'github.com/lima-vm/lima/v2/pkg/networks/usernet', '-run', 'TestNetstackWindowsNativeSocketPaths')
        Invoke-BuildTool 'go' @('vet', './cmd/eden', './internal/eden', './internal/workbench', './internal/appupdate', './internal/host')
    }
    Enter-CiPhase 'Package the installer'
    $packageArguments = @{ Release = $Release; SkipBuild = $true; VerifyDesktop = $true }
    if ($Release) { $packageArguments.QemuSourceArchive = $qemuSource }
    & (Join-Path $PSScriptRoot 'package-windows.ps1') @packageArguments
    $version = Get-EdenVersion $repository
    $suffix = if ($Release) { '' } else { '-unsigned' }
    Enter-CiPhase 'Install, update and uninstall with isolated data'
    & (Join-Path $PSScriptRoot 'test-windows-install.ps1') -Installer "$repository/_output/releases/$version/windows/Eden-$version-windows-amd64-setup$suffix.exe"
    if ($Release) {
        Enter-CiPhase 'Signed automatic update and rollback transactions'
        & (Join-Path $PSScriptRoot 'test-windows-update-flow.ps1') -DisposableMachine
        Invoke-BuildTool 'node' @('scripts/release-version.mjs', 'metadata', 'windows', "_output/releases/$version/windows/build-metadata.json")
    }
    Complete-CiPhase
} catch {
    $failedPhase = $script:phase
    Complete-CiPhase 'failed'
    if ($env:GITHUB_ACTIONS -and $failedPhase) {
        # Workflow commands are single-line; the full error follows below.
        $message = ("$($_.Exception.Message)" -replace '\r?\n', ' ')
        Write-Host "::error title=Windows phase failed: ${failedPhase}::${message}"
    }
    throw
} finally {
    Write-CiPhaseSummary
    if ($pfx) { Remove-Item -LiteralPath $pfx -Force -ErrorAction SilentlyContinue }
    foreach ($imported in $certificates) {
        if ($imported.HasPrivateKey) {
            Remove-Item "Cert:\CurrentUser\My\$($imported.Thumbprint)" -DeleteKey -Force -ErrorAction SilentlyContinue
        } else {
            Remove-Item "Cert:\CurrentUser\My\$($imported.Thumbprint)" -Force -ErrorAction SilentlyContinue
        }
    }
    # Verified QEMU sources remain in the build cache; packaging copies them separately.
    $env:PATH = $savedPath
    $env:GOROOT = $savedGoRoot
    $env:EDEN_WINDOWS_CERT_SHA1 = $savedCertificate
    $env:EDEN_TEST_STORAGE_VOLUME = $savedStorageVolume
    if ($storageTestRoot) { Remove-Item -LiteralPath $storageTestRoot -Recurse -Force -ErrorAction SilentlyContinue }
    Remove-Item Env:EDEN_ESIGNER_PASSWORD, Env:EDEN_ESIGNER_TOTP_SECRET -ErrorAction SilentlyContinue
    Pop-Location
}

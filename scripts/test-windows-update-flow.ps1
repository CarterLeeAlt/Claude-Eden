#requires -Version 7.0
# Runs real updater/helper/NSIS transactions on an elevated disposable runner.
# Release transport and workbench acknowledgement use headless local fixtures.
# WebView2 must already be installed by the owning CI job.
[CmdletBinding()]
param(
    [switch]$DisposableMachine,
    [ValidatePattern('^[a-fA-F0-9]{40}$')][string]$SigningCertificateThumbprint,
    [ValidateSet('CurrentUser','LocalMachine')][string]$SigningCertificateStore = 'CurrentUser',
    [string]$ForeignPublisherExecutable,
    [string]$ArtifactRoot,
    [ValidateSet('success','stop-required','installer-failure','startup-create-failure','startup-failure','startup-error','startup-cancel','startup-wrong-ack','startup-slow-success','signature-failure','digest-failure','publisher-failure','metadata-version-failure','prepare-failure','prepare-cancel','parent-start-mismatch','parent-pid-invalid','parent-image-mismatch','ready-cancel','ready-invalid-command','ready-disconnect')]
    [string[]]$Scenarios = @('success','stop-required','installer-failure','startup-create-failure','startup-failure','startup-error','startup-cancel','startup-wrong-ack','startup-slow-success','signature-failure','digest-failure','publisher-failure','metadata-version-failure','prepare-failure','prepare-cancel','parent-start-mismatch','parent-pid-invalid','parent-image-mismatch','ready-cancel','ready-invalid-command','ready-disconnect')
)
$ErrorActionPreference = 'Stop'
& (Join-Path $PSScriptRoot 'assert-workspace.ps1')
if (!$IsWindows) { throw 'Complete updater transactions require Windows.' }
if ($env:GITHUB_ACTIONS -ne 'true' -and !$DisposableMachine) { throw 'Use -DisposableMachine only on an isolated Windows test machine.' }
$computer = Get-CimInstance Win32_ComputerSystem
if ($computer.Model -notmatch 'Virtual Machine|VMware|VirtualBox|KVM|QEMU|HVM domU|Parallels' -and $computer.Manufacturer -notmatch 'QEMU|VMware|innotek') {
    throw 'System update acceptance is restricted to the approved existing Windows VM. Refusing this physical host.'
}
$identity = [Security.Principal.WindowsIdentity]::GetCurrent()
try { $administrator = [Security.Principal.WindowsPrincipal]::new($identity).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator) }
finally { $identity.Dispose() }
if (!$SigningCertificateThumbprint -and !$administrator) { throw 'Use an elevated disposable Windows runner for temporary test certificate trust, or pass the already trusted controlled CurrentUser signing certificate.' }
$repository = Split-Path $PSScriptRoot -Parent
$registration = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\ClaudeEden'
if (Test-Path $registration) { throw 'Use a test account without an installed Claude Eden application.' }
if ($ArtifactRoot) {
    $ArtifactRoot = [IO.Path]::GetFullPath($ArtifactRoot)
    $relative = [IO.Path]::GetRelativePath($env:EDEN_WORKSPACE_ROOT, $ArtifactRoot)
    if ([IO.Path]::IsPathRooted($relative) -or $relative -eq '..' -or $relative.StartsWith('..' + [IO.Path]::DirectorySeparatorChar)) { throw 'Update fixture artifacts must stay in the workspace.' }
    $ancestor = $ArtifactRoot
    while ($ancestor) {
        if ((Test-Path -LiteralPath $ancestor) -and ((Get-Item -LiteralPath $ancestor -Force).Attributes -band [IO.FileAttributes]::ReparsePoint)) { throw 'Update fixture artifacts cannot follow filesystem links.' }
        $ancestor = [IO.Path]::GetDirectoryName($ancestor)
    }
    [IO.Directory]::CreateDirectory($ArtifactRoot) | Out-Null
}
$fixtureBase = if ($ArtifactRoot) { $ArtifactRoot } else { [IO.Path]::GetTempPath() }
$root = Join-Path $fixtureBase ('eden-update-flow-' + [guid]::NewGuid().ToString('N'))
$reportBase = if ($ArtifactRoot) { $ArtifactRoot } elseif ($env:RUNNER_TEMP) { $env:RUNNER_TEMP } else { [IO.Path]::GetTempPath() }
$reportPath = Join-Path $reportBase ('eden-smoke-update-' + [guid]::NewGuid().ToString('N') + '.json')
$certificate = $null
$ownsCertificate = $false
$passed = $false
$results = @()
$machineProbe = $null
function Check([bool]$Value, [string]$Message) { if (!$Value) { throw $Message } }
function Write-Text([string]$Path, [string]$Value) {
    [IO.Directory]::CreateDirectory((Split-Path $Path -Parent)) | Out-Null
    [IO.File]::WriteAllText($Path, $Value)
}
function Wait-File([string]$Path, [switch]$AllowParentFailure) {
    # Test harness budget only; production readiness has no total deadline.
    $deadline = [DateTime]::UtcNow.AddMinutes(6)
    while ($true) {
        if (!$AllowParentFailure -and (Split-Path $Path -Leaf) -ne 'parent-failed.json' -and (Test-Path -LiteralPath (Join-Path $profile 'parent-failed.json'))) {
            $failure = Get-Content -LiteralPath (Join-Path $profile 'parent-failed.json') -Raw | ConvertFrom-Json
            throw "Native updater rejected the fixture: $($failure.message)"
        }
        if (Test-Path -LiteralPath $Path) {
            try {
                $value = Get-Content -LiteralPath $Path -Raw | ConvertFrom-Json
                if ($null -ne $value) { return $value }
            } catch [IO.IOException] { } catch [ArgumentException] { }
        }
        if ([DateTime]::UtcNow -ge $deadline) { throw "Test harness budget exhausted waiting for $Path" }
        Start-Sleep -Milliseconds 100
    }
}
function Run-Tool([string]$File, [string[]]$Arguments) {
    & $File @Arguments | Out-Host
    if ($LASTEXITCODE -ne 0) { throw "$File failed with exit $LASTEXITCODE" }
}
function Sign([string]$Path) {
    $signature = Set-AuthenticodeSignature -LiteralPath $Path -Certificate $certificate -HashAlgorithm SHA256
    Check ($signature.Status -eq 'Valid') "Fixture signature was not trusted: $Path ($($signature.Status))"
}
function Start-Fixture([string]$Executable, [string]$Profile, [string[]]$Arguments) {
    $start = [Diagnostics.ProcessStartInfo]::new($Executable)
    $start.UseShellExecute = $false
    $start.CreateNoWindow = $true
    if ($ArtifactRoot) { $start.Environment['TEMP'] = Join-Path $ArtifactRoot 'temp'; $start.Environment['TMP'] = $start.Environment['TEMP'] }
    $start.WorkingDirectory = $root
    # Test relative-to-absolute profile inheritance through the real launcher.
    $start.Environment['EDEN_CONFIG'] = [IO.Path]::GetRelativePath($root, (Join-Path $Profile 'config.json'))
    foreach ($argument in $Arguments) { $start.ArgumentList.Add($argument) }
    return [Diagnostics.Process]::Start($start)
}
function Run-Installer([string]$Installer, [string]$Target, [string]$Diagnostic) {
    $start = [Diagnostics.ProcessStartInfo]::new($Installer)
    $start.UseShellExecute = $false
    if ($ArtifactRoot) { $start.Environment['TEMP'] = Join-Path $ArtifactRoot 'temp'; $start.Environment['TMP'] = $start.Environment['TEMP'] }
    $start.WorkingDirectory = $root
    $start.Arguments = '/S /DESKTOPSHORTCUT=0 /STARTMENUSHORTCUT=0 /DIAGNOSTICS="' + $Diagnostic + '" /D=' + $Target
    $process = [Diagnostics.Process]::Start($start)
    try {
        Check ($process.WaitForExit(180000)) 'The fixture installer timed out.'
        Check ($process.ExitCode -eq 0) "The fixture installer exited $($process.ExitCode)."
    } finally { $process.Dispose() }
}
function Read-InstallerDiagnostic([string]$Path) {
    try {
        $file = Get-Item -LiteralPath $Path -ErrorAction SilentlyContinue
        if ($file -and !$file.PSIsContainer -and $file.Length -le 8192) {
            return Get-Content -LiteralPath $Path -Raw | ConvertFrom-Json
        }
    } catch {
        Write-Warning "Could not read installer diagnostic: $(Split-Path $Path -Leaf)"
    }
}
function Run-GoBuild([string]$Go, [string]$Architecture, [string[]]$Arguments) {
    $previousArch = [Environment]::GetEnvironmentVariable('GOARCH', 'Process')
    $previousOS = [Environment]::GetEnvironmentVariable('GOOS', 'Process')
    try {
        $env:GOARCH = $Architecture
        $env:GOOS = 'windows'
        Run-Tool $Go $Arguments
    } finally {
        if ($null -eq $previousArch) { $env:GOARCH = $null } else { $env:GOARCH = $previousArch }
        if ($null -eq $previousOS) { $env:GOOS = $null } else { $env:GOOS = $previousOS }
    }
}
function Get-FixtureMachine([string]$Path) {
    $stream = [IO.File]::OpenRead($Path)
    $reader = [IO.BinaryReader]::new($stream)
    try {
        if ($reader.ReadUInt16() -ne 0x5a4d) { throw 'The machine fixture is not a PE file.' }
        $stream.Position = 0x3c
        $header = $reader.ReadUInt32()
        $stream.Position = $header
        if ($reader.ReadUInt32() -ne 0x4550) { throw 'The machine fixture has no PE header.' }
        return $reader.ReadUInt16()
    } finally { $reader.Dispose() }
}
function Assert-SignedMachineRejection([string]$Executable) {
    Check ([Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString() -eq 'X64') 'The incompatible-image case requires the actual Windows x64 VM.'
    Check ((Get-FixtureMachine $Executable) -eq 0xaa64) 'The incompatible candidate is not Windows ARM64.'
    $signature = Get-AuthenticodeSignature -LiteralPath $Executable
    Check ($signature.Status -eq 'Valid' -and $signature.SignerCertificate.Thumbprint -eq $certificate.Thumbprint) 'The ARM64 image does not have the same controlled trusted publisher.'
    $version = [Diagnostics.FileVersionInfo]::GetVersionInfo($Executable)
    Check ($version.ProductName -eq 'Claude Eden' -and $version.ProductVersion -eq '0.1.8') 'The ARM64 image does not carry the required product/version resources.'
    $start = [Diagnostics.ProcessStartInfo]::new($Executable)
    $start.UseShellExecute = $false
    $start.CreateNoWindow = $true
    $start.WorkingDirectory = $root
    $start.Environment['EDEN_CONFIG'] = Join-Path $root 'machine-probe/config.json'
    $start.ArgumentList.Add('--probe-incompatible-image')
    $process = $null
    try {
        try { $process = [Diagnostics.Process]::Start($start) }
        catch {
            $native = $_.Exception.GetBaseException()
            if ($native -isnot [ComponentModel.Win32Exception] -or $native.NativeErrorCode -notin @(193,216)) { throw }
            $evidence = @{machine='ARM64';host='X64';processCreated=$false;nativeError=$native.NativeErrorCode;publisher=$certificate.Thumbprint;productVersion=$version.ProductVersion;imageSHA256=(Get-FileHash -LiteralPath $Executable -Algorithm SHA256).Hash.ToLowerInvariant()}
            Write-Text (Join-Path $root 'machine-rejection.json') ($evidence | ConvertTo-Json)
            return $evidence
        }
        throw 'Windows unexpectedly created the ARM64 process; the intended creation-failure injection is not valid.'
    } finally {
        if ($process) {
            try { $null=$process.SafeHandle; if (!$process.WaitForExit(2000)) { $process.Kill(); $process.WaitForExit() } }
            finally { $process.Dispose() }
        }
    }
}
function Build-Package([string]$Version, [ValidateSet('amd64','arm64')][string]$Architecture = 'amd64') {
    $variant = if ($Architecture -eq 'amd64') { $Version } else { $Version + '-' + $Architecture }
    $directory = Join-Path $root $variant
    $app = Join-Path $directory 'app'
    $build = Join-Path $directory 'build'
    [IO.Directory]::CreateDirectory($build) | Out-Null
    [IO.Directory]::CreateDirectory($app) | Out-Null
    Copy-Item -LiteralPath (Join-Path $repository 'internal/desktopupdate/testfixture/main.go') -Destination (Join-Path $build 'main.go')
    $info = Join-Path $build 'version.json'
    @{fixed = @{file_version = "$Version.0"; product_version = "$Version.0"}; info = @{'0409' = @{
        ProductName = 'Claude Eden'; ProductVersion = $Version; FileVersion = $Version; OriginalFilename = 'Eden.exe'
    }}} | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath $info -Encoding utf8
    $wails = Join-Path $repository '.tmp/tools/wails/wails3.exe'
    if (!(Test-Path -LiteralPath $wails)) { throw 'Build the pinned Wails tool with prepare-desktop.mjs --resources before running the VM update fixture.' }
    Run-Tool $wails @('generate', 'syso', '-arch', $Architecture, '-manifest', (Join-Path $repository 'packaging/windows/app.manifest'),
        '-icon', (Join-Path $repository 'packaging/windows/Eden.ico'), '-info', $info, '-out', (Join-Path $build ("rsrc_windows_$Architecture.syso")))
    if (!$env:GOROOT) { throw 'Set GOROOT to the workspace Go toolchain.' }
    $go = Join-Path $env:GOROOT 'bin/go.exe'
    Run-GoBuild $go $Architecture @('build', '-trimpath', '-ldflags', "-H windowsgui -X main.fixtureVersion=$Version", '-o', (Join-Path $app 'Eden.exe'), $build)
    Sign (Join-Path $app 'Eden.exe')
    [IO.Directory]::CreateDirectory((Join-Path $app 'resources')) | Out-Null
    Copy-Item -LiteralPath (Join-Path $app 'Eden.exe') -Destination (Join-Path $app 'resources/eden.exe')
    # The installer must run normally on x64 even for the deliberately
    # incompatible candidate. Only the candidate/backend image is ARM64.
    Run-GoBuild $go 'amd64' @('build', '-trimpath', '-o', (Join-Path $app 'resources/eden-install.exe'), './cmd/eden-install')
    Sign (Join-Path $app 'resources/eden-install.exe')
    Write-Text (Join-Path $app 'Eden.install-id') 'io.github.carterleealt.claude-eden'
    Write-Text (Join-Path $app 'version.txt') $Version
    Write-Text (Join-Path $app 'LICENSE.txt') 'Isolated updater transaction fixture; not a Claude Eden distribution.'
    $files = @(Get-ChildItem -LiteralPath $app -File -Recurse | ForEach-Object {
        @{path = [IO.Path]::GetRelativePath($app, $_.FullName).Replace('\','/'); sha256 = (Get-FileHash -LiteralPath $_.FullName -Algorithm SHA256).Hash.ToLowerInvariant()}
    })
    @{version = 1; files = $files} | ConvertTo-Json -Depth 4 | Set-Content -LiteralPath (Join-Path $app 'Eden.install-files.json')
    $setup = Join-Path $directory 'setup.exe'
    $compiler = (Get-Command makensis.exe -ErrorAction Stop).Source
    Run-Tool $compiler @('/V2', '/INPUTCHARSET', 'UTF8', "/DAPP_DIR=$app", "/DVERSION=$Version", "/DVERSION_NUMERIC=$Version.0",
        "/DFILE_MANIFEST=$app\Eden.install-files.json", "/DOUTPUT_FILE=$setup", (Join-Path $repository 'packaging/windows/Eden.nsi'))
    Sign $setup
    return @{setup = $setup; app = $app}
}

try {
    [IO.Directory]::CreateDirectory($root) | Out-Null
    if ($ArtifactRoot) { [IO.Directory]::CreateDirectory((Join-Path $ArtifactRoot 'temp')) | Out-Null }
    if ($SigningCertificateThumbprint) {
        $certificate = Get-Item -LiteralPath "Cert:\$SigningCertificateStore\My\$SigningCertificateThumbprint"
        if (!$certificate.HasPrivateKey) { throw 'The controlled fixture certificate has no private key in this test user.' }
    } else {
        Write-Host 'Creating a temporary test signing certificate.'
        $certificate = New-SelfSignedCertificate -Type CodeSigningCert -Subject ('CN=Updater transaction ' + [guid]::NewGuid().ToString('N')) -CertStoreLocation Cert:\CurrentUser\My
        $ownsCertificate = $true
        $publicCertificate = [Security.Cryptography.X509Certificates.X509Certificate2]::new($certificate.RawData)
        try {
            foreach ($storeName in @('Root','TrustedPublisher')) {
                Write-Host "Trusting the temporary signer in LocalMachine/$storeName."
                $store = [Security.Cryptography.X509Certificates.X509Store]::new($storeName, 'LocalMachine')
                try { $store.Open('ReadWrite'); $store.Add($publicCertificate) } finally { $store.Dispose() }
            }
        } finally { $publicCertificate.Dispose() }
    }
    Write-Host 'Building signed old and new fixture packages.'
    $old = Build-Package '0.1.7'
    $new = Build-Package '0.1.8'
    $unstartable = $null
    if ($Scenarios -contains 'startup-create-failure') {
        $unstartable = Build-Package '0.1.8' 'arm64'
        Check ((Get-FixtureMachine (Join-Path $unstartable.app 'resources/eden-install.exe')) -eq 0x8664) 'The ARM64 candidate package would fail inside the installer rather than at candidate process creation.'
        $machineProbe = Assert-SignedMachineRejection (Join-Path $unstartable.app 'Eden.exe')
    }
    foreach ($mode in $Scenarios) {
        Write-Host "Running Windows update transaction: $mode"
        $scenario = Join-Path $root $mode
        $target = Join-Path $scenario 'installed application'
        $profile = Join-Path $scenario 'selected profile'
        $installer = $new.setup
        $creationFailure = $mode -eq 'startup-create-failure'
        if ($creationFailure) { $installer = $unstartable.setup }
        if ($mode -eq 'signature-failure') {
            [IO.Directory]::CreateDirectory($scenario) | Out-Null
            $installer = Join-Path $scenario 'tampered-setup.exe'
            $bytes = [IO.File]::ReadAllBytes($new.setup)
            $bytes[1024] = $bytes[1024] -bxor 1
            [IO.File]::WriteAllBytes($installer, $bytes)
        }
        if ($mode -eq 'publisher-failure') {
            $installer = if ($ForeignPublisherExecutable) { $ForeignPublisherExecutable }
                elseif ($env:GITHUB_ACTIONS -eq 'true') { (Get-Command node.exe -ErrorAction Stop).Source }
                else { Join-Path $repository '.tmp/toolchains/node-v24.22.3-win-x64/node.exe' }
            $foreignSignature = Get-AuthenticodeSignature -LiteralPath $installer
            Check ($foreignSignature.Status -eq 'Valid' -and $foreignSignature.SignatureType -eq 'Authenticode') 'The different-publisher fixture must have a valid embedded Authenticode signature (for example the workspace Node executable).'
        }
        $initialDiagnostic = Join-Path $profile 'initial-installer.json'
        $updateDiagnostic = Join-Path $profile 'update-installer.json'
        $holder = $null; $parent = $null; $helper = $null; $disk = $null
        $probe = $mode -like 'parent-*' -or $mode -like 'ready-*'
        $preparationFailure = $mode -in @('prepare-failure','prepare-cancel')
        $successful = $mode -in @('success','stop-required','startup-slow-success')
        $candidateFailure = $mode -in @('startup-failure','startup-error','startup-cancel','startup-wrong-ack')
        try {
            Write-Text (Join-Path $profile 'fixture.json') (@{mode = $mode; installer = $installer} | ConvertTo-Json)
            Run-Installer $old.setup $target $initialDiagnostic
            Check ((Read-InstallerDiagnostic $initialDiagnostic).status -eq 'success') 'The initial installer did not record success.'
            $originalHash = (Get-FileHash -LiteralPath (Join-Path $target 'Eden.exe')).Hash
            $originalInventory = Get-Content -LiteralPath (Join-Path $target 'Eden.install-files.json') -Raw | ConvertFrom-Json
            $originalInventoryHash = (Get-FileHash -LiteralPath (Join-Path $target 'Eden.install-files.json') -Algorithm SHA256).Hash
            $personal = @{
                'account.json' = 'existing-account'; 'favorites.json' = 'existing-favorites';
                'credentials/private.txt' = 'existing-login'; 'config.json' = 'existing-config';
                'environment.json' = '{"id":"original-environment","account":"original-user","home":"/home/original-user","disk":"EdenData/state/persistent.img"}';
                'ssh/identity' = 'existing-environment-key'; 'home/history.txt' = 'existing-shell-history'
            }
            foreach ($entry in $personal.GetEnumerator()) { Write-Text (Join-Path $profile $entry.Key) $entry.Value }
            Write-Text (Join-Path $target 'my-project/work.txt') 'original project'
            Write-Text (Join-Path $target 'EdenData/state/persistent.img') 'existing-disk'
            Write-Text (Join-Path $scenario 'outside-project/file.txt') 'outside-project'
            $linkCommand = 'mklink /J "' + (Join-Path $target 'linked-project') + '" "' + (Join-Path $scenario 'outside-project') + '"'
            & cmd.exe /d /c $linkCommand | Out-Null
            Check ($LASTEXITCODE -eq 0) 'Could not create the isolated project junction.'
            if ($successful -or $creationFailure) {
                # A registered VM can keep its disk open while the update runs.
                # EdenData stays in place, so the open disk must not block it (#108).
                $disk = [IO.File]::Open((Join-Path $target 'EdenData/state/persistent.img'), 'Open', 'Read', 'Read')
            }
            if ($mode -eq 'installer-failure') {
                $holder = Start-Fixture (Join-Path $old.app 'Eden.exe') $profile @('--hold-directory', $target)
                $null = Wait-File (Join-Path $profile 'held.json')
            }
            if ($probe) {
                $probeExecutable = if ($mode -eq 'parent-image-mismatch') { Join-Path $old.app 'Eden.exe' } else { Join-Path $target 'Eden.exe' }
                $parent = Start-Fixture $probeExecutable $profile @('--probe-helper', $target)
            } else {
				$updateStartedAt = [DateTime]::UtcNow
                $parent = Start-Fixture (Join-Path $target 'Eden.exe') $profile @('--start-update')
            }
            $rejected = $mode -in @('signature-failure','digest-failure','publisher-failure','metadata-version-failure')
            if ($probe) {
                Check ($parent.WaitForExit(180000) -and $parent.ExitCode -eq 0) 'The protocol probe failed.'
                if ($mode -eq 'ready-disconnect') {
                    $ready = Wait-File (Join-Path $profile 'probe-ready.json')
                    Check ($ready.writeDenied -and $ready.renameDenied) 'The archive was replaceable after READY.'
                } else {
                    $probeResult = Wait-File (Join-Path $profile 'probe-finished.json')
                    Check ($probeResult.rejected -or $probeResult.revoked) 'The helper probe did not reject or revoke preparation.'
                    if ($mode -like 'ready-*') {
                        $ready = Wait-File (Join-Path $profile 'probe-ready.json')
                        Check ($ready.writeDenied -and $ready.renameDenied) 'The archive was replaceable after READY.'
                    }
                }
                $finished = Wait-File (Join-Path $profile 'helper-finished.json')
                Check ($finished.exit -eq 1) 'Uncommitted helper returned success.'
                Check (!(Test-Path -LiteralPath $updateDiagnostic)) 'Uncommitted helper invoked NSIS.'
                Check (!(Test-Path -LiteralPath (Join-Path $profile 'handoff-0.1.7.json'))) 'Uncommitted helper stopped the environment.'
            } elseif ($rejected -or $preparationFailure) {
                $failure = Wait-File (Join-Path $profile 'parent-failed.json')
                Check ($failure.phase -eq 'failed') 'An invalid update did not report failure.'
                if ($mode -eq 'digest-failure') { Check ($failure.message -like '*下载后发生变化*') 'The digest failure did not reach the downloaded-byte verification.' }
                if ($mode -eq 'signature-failure') { Check ($failure.message -like '*Windows 无法验证安装包签名*') 'The tampered signature did not reach Windows trust verification.' }
                if ($mode -eq 'publisher-failure') { Check ($failure.message -like '*发布者签名*不一致*') 'The trusted foreign publisher was not rejected by the publisher-key comparison.' }
                if ($mode -eq 'metadata-version-failure') { Check ($failure.message -like '*产品身份或版本不匹配*') 'The metadata version mismatch was not rejected by the product-resource check.' }
                Check ($parent.WaitForExit(30000) -and $parent.ExitCode -eq 1) 'The fixture did not finish after rejecting an invalid update.'
                if ($rejected) {
                    Check (!(Test-Path -LiteralPath (Join-Path $profile 'helper.json'))) 'An invalid update started the installer helper.'
                } else {
                    $finished = Wait-File (Join-Path $profile 'helper-finished.json') -AllowParentFailure
                    Check ($finished.exit -eq 1) 'Failed lifecycle preparation committed the helper.'
                    $null = Wait-File (Join-Path $profile 'reconnected.json') -AllowParentFailure
                    Check (!(Test-Path -LiteralPath $updateDiagnostic)) 'Failed lifecycle preparation invoked NSIS.'
                }
                Check (!(Test-Path -LiteralPath (Join-Path $profile 'handoff-0.1.7.json'))) 'An invalid update stopped the original environment.'
            } else {
                $helperInfo = Wait-File (Join-Path $profile 'helper.json')
                Check ($helperInfo.directory.TrimEnd('\') -eq (Join-Path $profile 'updates').TrimEnd('\')) 'The desktop updater launched its helper inside the installation directory.'
                try {
                    $helper = [Diagnostics.Process]::GetProcessById($helperInfo.pid)
                    if (!$helper.HasExited) { Check ($helper.MainModule.FileName -eq $helperInfo.executable -and $helper.StartTime.ToUniversalTime().Ticks -eq $helperInfo.startTicks) 'The helper identity changed before pinning its handle.' }
                    # Pin the live process handle before it exits. PID lookup
                    # alone cannot retain the exit code after Windows reaps it.
                    $null = $helper.SafeHandle
                } catch [ArgumentException] { $helper = $null }
                Check ($parent.WaitForExit(180000)) 'The parent did not hand off installation.'
                Check ($parent.ExitCode -eq 0) 'The desktop updater failed before committing the installation.'
                $finished = Wait-File (Join-Path $profile 'helper-finished.json')
                if ($helper) { Check ($helper.WaitForExit(180000)) 'The updater helper did not exit.' }
                $result = Wait-File (Join-Path $profile 'updates/last-result.json')
                Check ([bool]$result.success -eq $successful) "Unexpected installation result: $($result.message)"
                $expectedInstallerStatus = if ($mode -eq 'installer-failure') { 'failed' } else { 'success' }
                Check ((Read-InstallerDiagnostic $updateDiagnostic).status -eq $expectedInstallerStatus) "The update installer did not record $expectedInstallerStatus."
                Check ($finished.exit -eq $(if ($successful) { 0 } else { 1 })) 'The helper result and transaction return value disagreed.'
                if ($helper) { Check ($helper.ExitCode -eq $finished.exit) "The helper process exit ($($helper.ExitCode)) disagreed with the transaction result ($($finished.exit))." }
            }
            if ($probe -or $preparationFailure) {
                $helperInfo = Wait-File (Join-Path $profile 'helper.json') -AllowParentFailure
                try { $helper = [Diagnostics.Process]::GetProcessById($helperInfo.pid) } catch [ArgumentException] { $helper = $null }
                if ($helper) {
                    if (!$helper.HasExited) { Check ($helper.MainModule.FileName -eq $helperInfo.executable -and $helper.StartTime.ToUniversalTime().Ticks -eq $helperInfo.startTicks) 'The probe helper process identity changed.' }
                    Check ($helper.WaitForExit(10000)) 'The revoked helper did not finish its cross-process lifetime.'
                }
            }
            if ($successful) {
                $active = Wait-File (Join-Path $profile 'active.json')
                Check ($active.profile -eq $profile) 'The ready candidate changed the selected profile.'
                if ($mode -eq 'startup-slow-success') {
                    Check ($active.startupSeconds -ge 185) 'The real candidate did not cross the old 120-second startup limit.'
                    Check (!(Test-Path -LiteralPath (Join-Path $profile 'reopened.json'))) 'Slow successful startup reopened the original application.'
                }
                Check ([Diagnostics.FileVersionInfo]::GetVersionInfo((Join-Path $target 'Eden.exe')).ProductVersion -eq '0.1.8') 'The update did not install the requested version.'
            } elseif (!$rejected -and !$probe -and !$preparationFailure) {
                $reopened = Wait-File (Join-Path $profile 'reopened.json')
                Check ($reopened.version -eq '0.1.7' -and $reopened.profile -eq $profile) 'The original application did not reopen with its original profile.'
            }
            if (!$successful) {
                Check ((Get-FileHash -LiteralPath (Join-Path $target 'Eden.exe')).Hash -eq $originalHash) 'The failed update did not retain the original program bytes.'
            }
            $project = if ($mode -eq 'startup-failure') { 'saved by new version' } else { 'original project' }
            Check ((Get-Content -LiteralPath (Join-Path $target 'my-project/work.txt') -Raw) -eq $project) 'Update or rollback lost the latest project contents.'
            Check ((Get-Content -LiteralPath (Join-Path $target 'EdenData/state/persistent.img') -Raw) -eq 'existing-disk') 'Persistent data in the installation directory changed.'
            Check ((Get-Item -LiteralPath (Join-Path $target 'linked-project')).LinkType -eq 'Junction') 'Update or rollback replaced the project junction.'
            Check ((Get-Content -LiteralPath (Join-Path $target 'linked-project/file.txt') -Raw) -eq 'outside-project') 'The project junction target changed.'
            foreach ($entry in $personal.GetEnumerator()) { Check ((Get-Content -LiteralPath (Join-Path $profile $entry.Key) -Raw) -eq $entry.Value) "Personal data changed: $($entry.Key)" }
            Check (!(Test-Path -LiteralPath ($target + '.eden-installation.json'))) 'A completed failure left a blocking recovery receipt.'
            Check (@(Get-ChildItem -LiteralPath (Join-Path $profile 'updates') -Directory -Filter 'previous-*').Count -eq 0) 'A finished transaction retained an unnecessary program backup.'
            if ($mode -eq 'installer-failure') {
                Check (!(Test-Path -LiteralPath (Join-Path $profile 'handoff-0.1.8.json'))) 'An untouched original was treated as an installed candidate.'
                Check (@(Get-ChildItem -LiteralPath $scenario -Directory -Filter '*.restore-*').Count -eq 0) 'Installer rejection created an unnecessary rollback tree.'
            }
            if ($candidateFailure) {
                $candidate = Wait-File (Join-Path $profile 'candidate.json')
                Check ($candidate.version -eq '0.1.8' -and $candidate.profile -eq $profile) 'The candidate never reached its real application entry with the selected profile.'
                $expectedReason = switch ($mode) {
                    'startup-error' { '*fixture live controller startup failure*' }
                    'startup-cancel' { '*工作台启动已取消*' }
                    'startup-wrong-ack' { '*启动确认版本不匹配*' }
                    default { '*目标进程已退出*' }
                }
                Check ($result.message -like $expectedReason) 'The helper did not propagate the actual startup failure reason.'
                if ($mode -in @('startup-error','startup-cancel')) {
                    $live = Wait-File (Join-Path $profile 'failure-reported-live.json')
                    Check ($live.pid -eq $candidate.pid -and $live.startTicks -eq $candidate.startTicks) 'Explicit failure was not reported by the live candidate.'
                    Check (([DateTime]::UtcNow - [DateTime]::Parse($live.utc)).TotalSeconds -lt 60) 'Explicit failure waited for a startup timeout instead of triggering recovery.'
                }
                $rollback = Wait-File (Join-Path $profile 'handoff-0.1.7.json')
                $backupPrefix = Join-Path $profile 'updates/previous-'
                Check ($rollback.stop -and $rollback.executable.StartsWith($backupPrefix, [StringComparison]::OrdinalIgnoreCase)) 'Rollback did not stop the environment through the backed-up original backend.'
                Check (!(Test-Path -LiteralPath (Join-Path $profile 'handoff-0.1.8.json'))) 'Rollback executed the failed candidate backend.'
            }
            if ($creationFailure) {
                Check ($finished.nativeError -in @(193,216)) 'The helper did not return a real CreateProcess image/machine error.'
                Check ($result.message -like '*新版工作台进程未能创建*') 'The candidate was rejected before the intended process-creation step.'
                Check (!(Test-Path -LiteralPath (Join-Path $profile 'candidate.json'))) 'The incompatible candidate unexpectedly reached application entry.'
                Check ((Get-FixtureMachine (Join-Path $target 'Eden.exe')) -eq 0x8664) 'Recovery did not restore the original AMD64 application.'
                Check ((Get-FileHash -LiteralPath (Join-Path $target 'Eden.install-files.json') -Algorithm SHA256).Hash -eq $originalInventoryHash) 'Recovery changed the complete original program inventory.'
                foreach ($file in $originalInventory.files) {
                    Check ((Get-FileHash -LiteralPath (Join-Path $target $file.path) -Algorithm SHA256).Hash.ToLowerInvariant() -eq $file.sha256) ('Recovered original program differs: ' + $file.path)
                }
                $rollback = Wait-File (Join-Path $profile 'handoff-0.1.7.json')
                $backupPrefix = Join-Path $profile 'updates/previous-'
                Check ($rollback.stop -and $rollback.executable.StartsWith($backupPrefix, [StringComparison]::OrdinalIgnoreCase) -and $rollback.snapshotSHA256 -eq $originalHash.ToLowerInvariant()) 'Creation failure did not recover through the intact original snapshot/backend.'
                Check (!(Test-Path -LiteralPath (Join-Path $profile 'handoff-0.1.8.json'))) 'Recovery attempted to execute the incompatible candidate backend.'
            }
            if ($mode -eq 'stop-required') {
                Check ((Wait-File (Join-Path $profile 'handoff-0.1.7.json')).stop) 'stop_required was not followed by the explicit stop request.'
            }
            $scenarioResult = @{scenario = $mode; passed = $true; signatureVerification = $true; personalDataPreserved = $true}
            if ($creationFailure) { $scenarioResult.processCreationError = $finished.nativeError }
            $results += $scenarioResult
            Write-Host "PASS: complete Windows update transaction ($mode)."
        } catch {
            $diagnostics = @{
                initialInstaller = Read-InstallerDiagnostic $initialDiagnostic
                initialFileHelper = Read-InstallerDiagnostic "$initialDiagnostic.helper.json"
                updateInstaller = Read-InstallerDiagnostic $updateDiagnostic
                updateFileHelper = Read-InstallerDiagnostic "$updateDiagnostic.helper.json"
            }
            $results += @{scenario = $mode; passed = $false; error = $_.Exception.Message; diagnostics = $diagnostics}
            Write-Host "Windows installer diagnostics ($mode): $($diagnostics | ConvertTo-Json -Depth 6 -Compress)"
            throw
        } finally {
            if ($disk) { $disk.Dispose() }
            Write-Text (Join-Path $profile 'release.json') '{}'
            foreach ($owned in @($parent, $helper, $holder)) {
                if (!$owned) { continue }
                try { if (!$owned.WaitForExit(10000)) { $owned.Kill($true); $owned.WaitForExit() } } finally { $owned.Dispose() }
            }
            foreach ($name in @('candidate','reopened')) {
                $record = Join-Path $profile ($name + '.json')
                if (!(Test-Path -LiteralPath $record)) { continue }
                $pidValue = (Get-Content -LiteralPath $record -Raw | ConvertFrom-Json).pid
                $owned = Get-Process -Id $pidValue -ErrorAction SilentlyContinue
                if ($owned) {
                    try {
                        $identity = Get-Content -LiteralPath $record -Raw | ConvertFrom-Json
                        $matches = $owned.MainModule.FileName -eq (Join-Path $target 'Eden.exe')
                        if ($identity.startTicks) { $matches = $matches -and $owned.StartTime.ToUniversalTime().Ticks -eq $identity.startTicks }
                        if ($matches -and !$owned.WaitForExit(10000)) { $owned.Kill(); $owned.WaitForExit() }
                    } finally { $owned.Dispose() }
                }
            }
            if (Test-Path $registration) {
                Check ((Get-ItemProperty $registration).InstallLocation -eq $target) 'Fixture registration changed owner; retained for inspection.'
                Remove-Item -LiteralPath $registration -Recurse
            }
        }
    }
    $passed = $true
} finally {
    @{success = $passed; scenarios = $results; machineTypeProbe = $machineProbe; transport = 'local fixture'; readiness = 'headless acknowledgement'} | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $reportPath
    if ($certificate -and $ownsCertificate) {
        foreach ($storeName in @('Root','TrustedPublisher')) { Remove-Item "Cert:\LocalMachine\$storeName\$($certificate.Thumbprint)" -Force -ErrorAction SilentlyContinue }
        Remove-Item "Cert:\CurrentUser\My\$($certificate.Thumbprint)" -DeleteKey -Force -ErrorAction SilentlyContinue
    }
    # All paths below were created by this invocation. Preserve a failed run.
    if ($passed) { Remove-Item -LiteralPath $root -Recurse -Force }
    else { Write-Warning "Update fixture evidence retained at $root" }
    Write-Host "Windows update report: $reportPath"
}

#requires -Version 7.0
# Runs only on a disposable Windows test user. Does not start a VM.
[CmdletBinding(DefaultParameterSetName = 'Matrix')]
param(
    [Parameter(Mandatory, ParameterSetName = 'Matrix')][string]$Installer,
    [Parameter(Mandatory, ParameterSetName = 'Cleanup')][string]$CleanupReceipt,
    [Parameter(ParameterSetName = 'Matrix')][string]$ArtifactRoot,
    [switch]$DisposableMachine,
    # Explicit authorization for the dedicated physical Windows 10 acceptance
    # host. This must never be inferred from the machine model.
    [switch]$ApprovedPhysicalMachine
)
[Console]::OutputEncoding = [Text.UTF8Encoding]::new($false)
$OutputEncoding = [Console]::OutputEncoding
. (Join-Path $PSScriptRoot 'windows-common.ps1')
if (-not $IsWindows) { throw 'Installation smoke tests must run on Windows.' }
if ($env:GITHUB_ACTIONS -ne 'true' -and !$ApprovedPhysicalMachine -and (-not $DisposableMachine -or $env:COMPUTERNAME -ne 'EDEN-WINTEST')) { throw 'Use the approved disposable Windows VM, the Windows CI runner, or -ApprovedPhysicalMachine on the dedicated physical acceptance host.' }
if ([IntPtr]::Size -ne 8) { throw 'Run installation acceptance in 64-bit PowerShell.' }
$computer = Get-CimInstance Win32_ComputerSystem
$virtualMachine = $computer.Model -match 'Virtual Machine|VMware|VirtualBox|KVM|QEMU|HVM domU|Parallels' -or $computer.Manufacturer -match 'QEMU|VMware|innotek'
if (!$virtualMachine -and !$ApprovedPhysicalMachine) { throw 'Installation acceptance requires the approved VM or explicit -ApprovedPhysicalMachine authorization.' }
$desktopShortcut = Join-Path ([Environment]::GetFolderPath('DesktopDirectory')) 'ClaudeEden.lnk'
$startMenuDirectory = Join-Path ([Environment]::GetFolderPath('Programs')) 'ClaudeEden'
$savedKey = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\ClaudeEden'
function Assert-InstallTestPath([string]$Path) {
    $absolute = [IO.Path]::GetFullPath($Path)
    $relative = [IO.Path]::GetRelativePath($env:EDEN_WORKSPACE_ROOT, $absolute)
    if ([IO.Path]::IsPathRooted($relative) -or $relative -eq '.' -or $relative -eq '..' -or $relative.StartsWith('..' + [IO.Path]::DirectorySeparatorChar)) { throw 'Installation test paths must stay below the workspace.' }
    for ($current = $absolute; $current; $current = [IO.Path]::GetDirectoryName($current)) {
        if ((Test-Path -LiteralPath $current) -and ((Get-Item -LiteralPath $current -Force).Attributes -band [IO.FileAttributes]::ReparsePoint)) { throw 'Installation test paths cannot follow filesystem links.' }
    }
    return $absolute
}
function Write-InstallTestJSON([string]$Path, $Value) {
    $temporary = $Path + '.' + [guid]::NewGuid().ToString('N') + '.tmp'
    [IO.File]::WriteAllText($temporary, ($Value | ConvertTo-Json -Depth 10), [Text.UTF8Encoding]::new($false))
    [IO.File]::Move($temporary, $Path, $true)
}
function Get-InstallShortcutTarget([string]$Path) {
    $shell = New-Object -ComObject WScript.Shell
    try { return $shell.CreateShortcut($Path).TargetPath }
    finally { $null = [Runtime.InteropServices.Marshal]::FinalReleaseComObject($shell) }
}
function Read-InstallTestOwnership([string]$Path) {
    $Path = Assert-InstallTestPath $Path
    if ([IO.Path]::GetFileName($Path) -ne 'ownership.json' -or (Get-Item -LiteralPath $Path).Length -gt 16384) { throw 'Invalid installation-test ownership receipt.' }
    $owned = Get-Content -LiteralPath $Path -Raw | ConvertFrom-Json -AsHashtable
    $root = [IO.Path]::GetDirectoryName($Path)
    if ($owned.schema -ne 1 -or $owned.root -ne $root -or $owned.target -ne (Join-Path $root 'installed application') -or $owned.desktop -ne $desktopShortcut -or $owned.startMenu -ne $startMenuDirectory -or $owned.registration -ne $savedKey -or $owned.uninstallerSHA256 -notmatch '^([a-f0-9]{64})?$') { throw 'Installation-test ownership does not match this exact workspace and user.' }
    $null = Assert-InstallTestPath $owned.target
    return $owned
}
function Get-InstallTestDiagnosticPhase([string]$Path) {
    if (-not (Test-Path -LiteralPath $Path)) { return $null }
    try { return (Get-Content -LiteralPath $Path -Raw | ConvertFrom-Json -AsHashtable)['phase'] }
    catch [IO.IOException], [ArgumentException] { return $null }
}
function Invoke-InstallTestProcess([string]$Executable, [string]$Arguments, [int]$BudgetSeconds, [string]$Diagnostic = '', [IDisposable]$ReleaseOnDestinationCheck = $null) {
    $start = [Diagnostics.ProcessStartInfo]::new($Executable)
    $start.UseShellExecute = $false
    $start.CreateNoWindow = $true
    $start.WorkingDirectory = $testRoot
    $start.Arguments = $Arguments
    $temporary = Join-Path $testRoot 'installer-temp'
    [IO.Directory]::CreateDirectory($temporary) | Out-Null
    $start.Environment['TEMP'] = $temporary
    $start.Environment['TMP'] = $temporary
    $process = [Diagnostics.Process]::Start($start)
    $released = $false
    try {
        $null = $process.SafeHandle
        $deadline = [DateTime]::UtcNow.AddSeconds($BudgetSeconds)
        while (-not $process.WaitForExit(100)) {
            if ($ReleaseOnDestinationCheck -and -not $released) {
                $phase = Get-InstallTestDiagnosticPhase $Diagnostic
                if ($phase -eq 'checking_destination') {
                    Start-Sleep -Milliseconds 650
                    $ReleaseOnDestinationCheck.Dispose()
                    $released = $true
                }
            }
            if ([DateTime]::UtcNow -ge $deadline) { throw 'The installation test operation exhausted its harness budget.' }
        }
        if ($ReleaseOnDestinationCheck -and -not $released) { throw 'Transient-lock fixture never observed the real destination check.' }
        return $process.ExitCode
    } finally {
        try { if (-not $process.HasExited) { $process.Kill($true); $process.WaitForExit() } }
        finally { $process.Dispose() }
    }
}
function Get-InstallTestShortcuts($Owned) {
    return @(
        @{path=$Owned.desktop;target=(Join-Path $Owned.target 'ClaudeEden.exe')}
        @{path=(Join-Path $Owned.startMenu 'ClaudeEden.lnk');target=(Join-Path $Owned.target 'ClaudeEden.exe')}
        @{path=(Join-Path $Owned.startMenu 'Uninstall ClaudeEden.lnk');target=(Join-Path $Owned.target 'Uninstall.exe')}
    )
}
function Assert-InstallTestRegistrationOwner($Owned) {
    if (-not (Test-Path -LiteralPath $Owned.registration)) { return }
    if ((Get-ItemProperty -LiteralPath $Owned.registration).InstallLocation -ne $Owned.target -or @(Get-ChildItem -LiteralPath $Owned.registration).Count) { throw 'Registration ownership changed; preserve it.' }
}
function Assert-InstallTestSystemOwnership($Owned) {
    Assert-InstallTestRegistrationOwner $Owned
    foreach ($shortcut in Get-InstallTestShortcuts $Owned) {
        if ((Test-Path -LiteralPath $shortcut.path) -and (Get-InstallShortcutTarget $shortcut.path) -ne $shortcut.target) { throw 'Shortcut ownership changed; preserve it.' }
    }
}
function Invoke-OwnedInstallTestUninstaller($Owned) {
    $uninstaller = Join-Path $Owned.target 'Uninstall.exe'
    if (-not (Test-Path -LiteralPath $uninstaller -PathType Leaf)) { return }
    $null = Assert-InstallTestPath $uninstaller
    if ((Get-Content -LiteralPath (Join-Path $Owned.target 'Eden.install-id') -Raw).Trim() -ne 'io.github.carterleealt.claude-eden' -or -not $Owned.uninstallerSHA256 -or (Get-FileHash -LiteralPath $uninstaller -Algorithm SHA256).Hash.ToLowerInvariant() -ne $Owned.uninstallerSHA256) { throw 'Uninstaller ownership changed; retained without executing it.' }
    if (Test-Path -LiteralPath ($Owned.target + '.eden-installation.json')) { throw 'An incomplete file transaction is retained for diagnosis.' }
    Assert-InstallTestSystemOwnership $Owned
    $diagnostic = Join-Path $Owned.root ('cleanup-uninstaller-' + [guid]::NewGuid().ToString('N') + '.json')
    $code = Invoke-InstallTestProcess $uninstaller ('/S /DIAGNOSTICS="' + $diagnostic + '"') 180
    if ($code -ne 0) { throw "Owned uninstaller returned $code." }
    $deadline = [DateTime]::UtcNow.AddSeconds(120)
    while ((Test-Path -LiteralPath $uninstaller) -and [DateTime]::UtcNow -lt $deadline) { Start-Sleep -Milliseconds 100 }
    if (Test-Path -LiteralPath $uninstaller) { throw 'Owned uninstaller did not finish its temporary worker.' }
}
function Remove-OwnedInstallTestShortcut($Shortcut) {
    if (-not (Test-Path -LiteralPath $Shortcut.path)) { return }
    if ((Get-InstallShortcutTarget $Shortcut.path) -ne $Shortcut.target) { throw 'Shortcut ownership changed; retained.' }
    Remove-Item -LiteralPath $Shortcut.path
}
function Remove-EmptyInstallTestStartMenu($Owned) {
    if (-not (Test-Path -LiteralPath $Owned.startMenu)) { return }
    if (@(Get-ChildItem -LiteralPath $Owned.startMenu -Force).Count -ne 0) { throw 'Unknown start-menu entries retained.' }
    [IO.Directory]::Delete($Owned.startMenu)
}
function Remove-OwnedInstallTestRegistration($Owned) {
    if (-not (Test-Path -LiteralPath $Owned.registration)) { return }
    if ((Get-ItemProperty -LiteralPath $Owned.registration).InstallLocation -ne $Owned.target -or @(Get-ChildItem -LiteralPath $Owned.registration).Count) { throw 'Registration ownership changed; retained.' }
    Remove-Item -LiteralPath $Owned.registration
}
function Remove-InstallTestSystemState($Owned, [string]$RecordName = 'system-cleanup.json') {
    $errors = [Collections.Generic.List[string]]::new()
    try { Invoke-OwnedInstallTestUninstaller $Owned }
    catch { $errors.Add($_.Exception.Message) }
    foreach ($shortcut in Get-InstallTestShortcuts $Owned) {
        try { Remove-OwnedInstallTestShortcut $shortcut }
        catch { $errors.Add($_.Exception.Message) }
    }
    try { Remove-EmptyInstallTestStartMenu $Owned }
    catch { $errors.Add($_.Exception.Message) }
    try { Remove-OwnedInstallTestRegistration $Owned }
    catch { $errors.Add($_.Exception.Message) }
    $result = @{errors=$errors.ToArray();registrationRemains=(Test-Path -LiteralPath $Owned.registration);desktopRemains=(Test-Path -LiteralPath $Owned.desktop);startMenuRemains=(Test-Path -LiteralPath $Owned.startMenu);retainedFiles=(Test-Path -LiteralPath $Owned.target);utc=[DateTime]::UtcNow.ToString('o')}
    Write-InstallTestJSON (Join-Path $Owned.root $RecordName) $result
    return $result
}
if ($PSCmdlet.ParameterSetName -eq 'Cleanup') {
    $ownership = Read-InstallTestOwnership $CleanupReceipt
    $testRoot = $ownership.root
    $cleanup = Remove-InstallTestSystemState $ownership 'post-job-cleanup.json'
    if ($cleanup.errors.Count -or $cleanup.registrationRemains -or $cleanup.desktopRemains -or $cleanup.startMenuRemains) { throw 'Owned installation cleanup is incomplete; evidence retained.' }
    return
}
$Installer = Assert-InstallTestPath $Installer
if (-not (Test-Path -LiteralPath $Installer -PathType Leaf)) { throw 'The exact installer file is missing.' }
# Runtime installation changes shared Edge services/tasks and cannot be rolled
# back as this test's private state. Require the already provisioned runtime.
function Get-InstallTestRuntimeVersion([Microsoft.Win32.RegistryHive]$Hive) {
    $registry = [Microsoft.Win32.RegistryKey]::OpenBaseKey($Hive, [Microsoft.Win32.RegistryView]::Registry32)
    try {
        $runtime = $registry.OpenSubKey('Software\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}')
        if (-not $runtime) { return '' }
        try { return [string]$runtime.GetValue('pv') }
        finally { $runtime.Dispose() }
    } finally { $registry.Dispose() }
}
$runtimePresent = $false
foreach ($hive in @([Microsoft.Win32.RegistryHive]::LocalMachine, [Microsoft.Win32.RegistryHive]::CurrentUser)) {
    $value = Get-InstallTestRuntimeVersion $hive
    if ($value -and $value -ne '0.0.0.0') { $runtimePresent = $true }
}
if (-not $runtimePresent) { throw 'Provision WebView2 separately before package acceptance; preserve shared runtime state.' }
if (Test-Path $savedKey) { throw 'Use a disposable Windows user without an existing Claude Eden installation.' }
if ((Test-Path -LiteralPath $desktopShortcut) -or (Test-Path -LiteralPath $startMenuDirectory)) { throw 'Preserve existing Claude Eden shortcuts and start-menu contents.' }
$base = if ($ArtifactRoot) { Assert-InstallTestPath $ArtifactRoot } else { Get-WindowsWorkbenchTestBase }
$testRoot = Assert-InstallTestPath (Join-Path $base $(if ($ArtifactRoot) { 'installation-matrix' } else { 'eden-package-' + [guid]::NewGuid().ToString('N') }))
if (Test-Path -LiteralPath $testRoot) { throw 'Preserve the existing installation test run.' }
[IO.Directory]::CreateDirectory($testRoot) | Out-Null
$target = Join-Path $testRoot 'installed application'
$ownership = @{schema=1;root=$testRoot;target=$target;desktop=$desktopShortcut;startMenu=$startMenuDirectory;registration=$savedKey;uninstallerSHA256='';installerSHA256=(Get-FileHash -LiteralPath $Installer -Algorithm SHA256).Hash.ToLowerInvariant()}
$ownershipPath = Join-Path $testRoot 'ownership.json'
Write-InstallTestJSON $ownershipPath $ownership
$fixture = Join-Path $PSScriptRoot 'personal-data-fixture.mjs'
$manifest = Join-Path $testRoot 'personal-data.json'
$personalHome = Join-Path $testRoot 'personal-profile'
$results = [Collections.Generic.List[object]]::new()
$failure = $null
$passed = $false
$previousConfig = [Environment]::GetEnvironmentVariable('EDEN_CONFIG', 'Process')
function Assert-PersonalData {
    Invoke-BuildTool 'node' @($fixture, 'verify', $personalHome, $manifest)
}
function Assert-InstalledPackage {
    $inventory = Get-Content -LiteralPath (Join-Path $target 'Eden.install-files.json') -Raw | ConvertFrom-Json
    if ($inventory.version -ne 1 -or -not $inventory.files.Count) { throw 'The installed Go ownership manifest is invalid.' }
    $seen = [Collections.Generic.HashSet[string]]::new([StringComparer]::OrdinalIgnoreCase)
    foreach ($file in $inventory.files) {
        if ($file.path -match '[\\:]|^/|(^|/)\.\.?(/|$)' -or -not $seen.Add($file.path) -or $file.sha256 -notmatch '^[a-f0-9]{64}$') { throw 'Unsafe installed ownership inventory.' }
        $actual = Assert-InstallTestPath (Join-Path $target $file.path)
        if (-not (Test-Path -LiteralPath $actual -PathType Leaf) -or (Get-FileHash -LiteralPath $actual -Algorithm SHA256).Hash.ToLowerInvariant() -ne $file.sha256) { throw ('Installed package hash mismatch: ' + $file.path) }
    }
    if (-not $seen.Contains('resources/eden-install.exe') -or -not $seen.Contains('Uninstall.exe')) { throw 'The installed Go helper/uninstaller ownership is incomplete.' }
}
function Write-InstallTestDiagnostic([string]$Path) {
    try {
        if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) { return }
        if ((Get-Item -LiteralPath $Path).Length -gt 8192) { return }
        $value = Get-Content -LiteralPath $Path -Raw | ConvertFrom-Json -AsHashtable
        $safe = @{phase = $value['phase']; status = $value['status']; helperExit = $value['helperExit']; operation = $value['operation']; pointerSize = $value['pointerSize']}
        if ($value['errors']) { $safe.errors = @($value['errors'] | Select-Object -First 4 | ForEach-Object { @{type = $_['type']; hresult = $_['hresult']} }) }
        Write-Host ('Claude Eden installer diagnostic: ' + ($safe | ConvertTo-Json -Depth 4 -Compress))
    } catch { Write-Warning 'The bounded installer diagnostic could not be read.' }
}
function Run-Installer {
    param([switch]$Update, [int]$ExpectedExitCode = 0, [string[]]$ShortcutOptions = @(), [IDisposable]$ReleaseOnDestinationCheck = $null)
    Assert-InstallTestSystemOwnership $ownership
    $installationReport = Join-Path $testRoot ('installer-' + [guid]::NewGuid().ToString('N') + '.json')
    $arguments = @('/S', ('/DIAGNOSTICS="' + $installationReport + '"'))
    if ($Update) { $arguments += '/UPDATE' }
    $arguments += $ShortcutOptions
    $arguments += "/D=$target"
    $exitCode = Invoke-InstallTestProcess $Installer ($arguments -join ' ') 300 $installationReport $ReleaseOnDestinationCheck
    if ($exitCode -ne $ExpectedExitCode) {
        Write-InstallTestDiagnostic $installationReport
        Write-InstallTestDiagnostic "$installationReport.helper.json"
        throw "Installer returned $exitCode, expected $ExpectedExitCode"
    }
    if ($ExpectedExitCode -ne 0) {
        $diagnostic = Get-Content -LiteralPath $installationReport -Raw | ConvertFrom-Json -AsHashtable
        if ($diagnostic['phase'] -ne 'checking_destination' -or $diagnostic['helperExit'] -ne 'application_file_in_use') { throw 'The rejected reinstall did not reach the intended executable access check.' }
    }
    if ($exitCode -eq 0) {
        $ownership.uninstallerSHA256 = (Get-FileHash -LiteralPath (Join-Path $target 'Uninstall.exe') -Algorithm SHA256).Hash.ToLowerInvariant()
        Write-InstallTestJSON $ownershipPath $ownership
    }
}
function Assert-Shortcuts {
    param([bool]$Desktop, [bool]$StartMenu)
    if ((Test-Path -LiteralPath $desktopShortcut) -ne $Desktop) { throw 'Desktop shortcut differs from the installer selection.' }
    foreach ($name in @('ClaudeEden.lnk', 'Uninstall ClaudeEden.lnk')) {
        if ((Test-Path -LiteralPath (Join-Path $startMenuDirectory $name)) -ne $StartMenu) { throw "Start menu shortcut differs from the installer selection: $name" }
    }
    if ($Desktop -and (Get-InstallShortcutTarget $desktopShortcut) -ne (Join-Path $target 'ClaudeEden.exe')) { throw 'Desktop shortcut targets a different installation.' }
    if ($StartMenu -and ((Get-InstallShortcutTarget (Join-Path $startMenuDirectory 'ClaudeEden.lnk')) -ne (Join-Path $target 'ClaudeEden.exe') -or (Get-InstallShortcutTarget (Join-Path $startMenuDirectory 'Uninstall ClaudeEden.lnk')) -ne (Join-Path $target 'Uninstall.exe'))) { throw 'Start menu shortcuts target a different installation.' }
}
function Complete-InstallTestStep([string]$Name) {
    $results.Add(@{scenario=$Name;passed=$true;utc=[DateTime]::UtcNow.ToString('o')})
    Write-InstallTestJSON (Join-Path $base 'install-phase.json') @{phase=$Name;completed=$results.Count;utc=[DateTime]::UtcNow.ToString('o')}
}
function Run-Uninstaller {
    Assert-InstallTestSystemOwnership $ownership
    if ((Get-FileHash -LiteralPath (Join-Path $target 'Uninstall.exe') -Algorithm SHA256).Hash.ToLowerInvariant() -ne $ownership.uninstallerSHA256) { throw 'The owned uninstaller changed before execution.' }
    $diagnostic = Join-Path $testRoot ('uninstaller-' + [guid]::NewGuid().ToString('N') + '.json')
    $code = Invoke-InstallTestProcess (Join-Path $target 'Uninstall.exe') ('/S /DIAGNOSTICS="' + $diagnostic + '"') 180
    if ($code -ne 0) { throw "Uninstaller returned $code." }
    $deadline = [DateTime]::UtcNow.AddSeconds(120)
    while (((Test-Path -LiteralPath (Join-Path $target 'Uninstall.exe')) -or (Test-Path -LiteralPath $savedKey)) -and [DateTime]::UtcNow -lt $deadline) { Start-Sleep -Milliseconds 200 }
    if ((Test-Path -LiteralPath (Join-Path $target 'ClaudeEden.exe')) -or (Test-Path -LiteralPath (Join-Path $target 'Uninstall.exe')) -or (Test-Path -LiteralPath $savedKey)) { throw 'Uninstall left owned program files or registration.' }
    Assert-PersonalData
    Assert-Shortcuts $false $false
}
function Assert-InstallTestPersonalContents([array]$Files, [string]$Message) {
    foreach ($file in $Files) {
        if ([IO.File]::ReadAllText($file.path) -ne $file.value) { throw $Message }
    }
}
function Get-PreservedInstallFile([string]$Name, [string]$Value) {
    $root = Join-Path $target 'Eden-preserved-files'
    if (-not (Test-Path -LiteralPath $root -PathType Container)) { return @() }
    return @(Get-ChildItem -LiteralPath $root -File -Recurse -Force | Where-Object {
        $_.Name -eq $Name -and [IO.File]::ReadAllText($_.FullName) -eq $Value
    })
}
try {
    $env:EDEN_CONFIG = Join-Path $testRoot 'tool-profile/config.json'
    Invoke-BuildTool 'node' @($fixture, 'seed', $personalHome, $manifest)
    # Cover the directory-picker path where the user creates an empty folder first.
    New-Item -ItemType Directory -Path $target | Out-Null
    Run-Installer
    Assert-InstalledPackage
    Assert-PersonalData
    Assert-Shortcuts $true $true
    Complete-InstallTestStep 'fresh-empty-directory'

    if (-not (Test-Path -LiteralPath (Join-Path $target 'ClaudeEden.exe'))) { throw 'Installer did not create ClaudeEden.exe.' }
    Invoke-BuildTool (Join-Path $target 'resources/eden.exe') @('--help')
    Invoke-BuildTool (Join-Path $target 'resources/eden.exe') @('verify-vm-assets')
    Invoke-BuildTool (Join-Path $target 'resources/eden.exe') @('verify-web-assets')
    Invoke-BuildTool (Join-Path $target 'resources/mihomo.exe') @('-v')
    Invoke-BuildTool (Join-Path $target 'resources/runtime/qemu/qemu-system-x86_64.exe') @('--version')
    Invoke-BuildTool (Join-Path $target 'resources/runtime/openssh/ssh.exe') @('-V')
    if (-not (Test-Path -LiteralPath (Join-Path $target 'resources/eden-install.exe') -PathType Leaf)) { throw 'The Go installation helper is missing from the real package.' }
    Complete-InstallTestStep 'packaged-native-tools'

    # Use the same isolated profile, process lifetime and retained diagnostics as
    # build acceptance. A separate per-run profile holds preservation markers.
    Assert-WindowsWorkbench -AppDirectory $target
    Assert-PersonalData
    Complete-InstallTestStep 'initial-workbench-entry'

    # A locked installed executable must fail before replacing the installation.
    $before = (Get-FileHash -LiteralPath (Join-Path $target 'ClaudeEden.exe') -Algorithm SHA256).Hash
    $locked = [IO.File]::Open((Join-Path $target 'ClaudeEden.exe'), [IO.FileMode]::Open, [IO.FileAccess]::Read, [IO.FileShare]::Read)
    try { Run-Installer -Update -ExpectedExitCode 2 } finally { $locked.Dispose() }
    Assert-PersonalData
    if ((Get-FileHash -LiteralPath (Join-Path $target 'ClaudeEden.exe') -Algorithm SHA256).Hash -ne $before) { throw 'Failed update replaced the existing application.' }
    Complete-InstallTestStep 'persistent-executable-lock'

    $locked = [IO.File]::Open((Join-Path $target 'ClaudeEden.exe'), [IO.FileMode]::Open, [IO.FileAccess]::Read, [IO.FileShare]::Read)
    try { Run-Installer -Update -ReleaseOnDestinationCheck $locked } finally { $locked.Dispose() }
    Assert-PersonalData
    Complete-InstallTestStep 'transient-executable-lock'

    $executablePath = Join-Path $target 'ClaudeEden.exe'
    $attributes = [IO.File]::GetAttributes($executablePath)
    try {
        [IO.File]::SetAttributes($executablePath, $attributes -bor [IO.FileAttributes]::ReadOnly)
        Run-Installer -Update -ExpectedExitCode 2
    } finally { [IO.File]::SetAttributes($executablePath, $attributes) }
    if ((Get-FileHash -LiteralPath $executablePath -Algorithm SHA256).Hash -ne $before) { throw 'Access-denied reinstall changed the original executable.' }
    Assert-PersonalData
    Complete-InstallTestStep 'access-denied-reinstall'

    # Reinstall through the real atomic replacement path. User data must survive.
    Run-Installer -Update
    Assert-PersonalData
    Assert-Shortcuts $true $true
    Complete-InstallTestStep 'reinstall-default-shortcuts'
    Run-Installer -Update -ShortcutOptions @('/DESKTOPSHORTCUT=0', '/STARTMENUSHORTCUT=0')
    Assert-Shortcuts $false $false
    Complete-InstallTestStep 'shortcut-none'
    Run-Installer -Update
    Assert-Shortcuts $false $false
    Complete-InstallTestStep 'shortcut-none-retained'
    Run-Installer -Update -ShortcutOptions @('/DESKTOPSHORTCUT=1', '/STARTMENUSHORTCUT=0')
    Assert-Shortcuts $true $false
    Complete-InstallTestStep 'shortcut-desktop-only'
    Run-Installer -Update -ShortcutOptions @('/DESKTOPSHORTCUT=0', '/STARTMENUSHORTCUT=1')
    Assert-Shortcuts $false $true
    Complete-InstallTestStep 'shortcut-start-menu-only'
    Run-Installer -Update -ShortcutOptions @('/DESKTOPSHORTCUT=1', '/STARTMENUSHORTCUT=1')
    Assert-Shortcuts $true $true
    Assert-PersonalData
    Complete-InstallTestStep 'shortcut-both'

    Run-Uninstaller
    if (Test-Path -LiteralPath $target) { throw 'Uninstall left packaged files or directories that would block installation into the same folder.' }
    Complete-InstallTestStep 'clean-uninstall'
    Run-Installer
    Assert-InstalledPackage
    Assert-Shortcuts $true $true
    Complete-InstallTestStep 'same-folder-fresh-reinstall'

    $personalFile = Join-Path $target 'my-project/work.txt'
    $diskFile = Join-Path $target 'EdenData/state/persistent.img'
    [IO.Directory]::CreateDirectory([IO.Path]::GetDirectoryName($personalFile)) | Out-Null
    [IO.Directory]::CreateDirectory([IO.Path]::GetDirectoryName($diskFile)) | Out-Null
    [IO.File]::WriteAllText($personalFile, 'latest original project')
    [IO.File]::WriteAllText($diskFile, 'original environment disk')
    $modified = Join-Path $target 'LICENSE.txt'
    [IO.File]::WriteAllText($modified, 'user-modified packaged text')
    $outside = Join-Path $testRoot 'outside-project'
    [IO.Directory]::CreateDirectory($outside) | Out-Null
    [IO.File]::WriteAllText((Join-Path $outside 'work.txt'), 'junction target data')
    & cmd.exe /d /c ('mklink /J "' + (Join-Path $target 'linked-project') + '" "' + $outside + '"') | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'Could not create the isolated project junction.' }
    $disk = [IO.File]::Open($diskFile, 'Open', 'Read', 'Read')
    try { Run-Installer -Update } finally { $disk.Dispose() }
    $personalContents = @(
        @{path=$personalFile;value='latest original project'}
        @{path=$diskFile;value='original environment disk'}
    )
    Assert-InstallTestPersonalContents $personalContents 'Reinstall changed personal or modified files.'
    if (@(Get-PreservedInstallFile 'LICENSE.txt' 'user-modified packaged text').Count -ne 1) { throw 'Reinstall did not retain the modified packaged file in Eden-preserved-files.' }
    if ((Get-Item -LiteralPath (Join-Path $target 'linked-project')).LinkType -ne 'Junction' -or [IO.File]::ReadAllText((Join-Path $target 'linked-project/work.txt')) -ne 'junction target data') { throw 'Reinstall replaced the personal junction.' }
    Assert-PersonalData
    Complete-InstallTestStep 'reinstall-personal-files-and-open-disk'
    Run-Uninstaller
    Assert-InstallTestPersonalContents $personalContents 'Uninstall removed personal or modified files.'
    if (@(Get-PreservedInstallFile 'LICENSE.txt' 'user-modified packaged text').Count -ne 1) { throw 'Uninstall removed the retained modified packaged file.' }
    if ((Get-Item -LiteralPath (Join-Path $target 'linked-project')).LinkType -ne 'Junction' -or [IO.File]::ReadAllText((Join-Path $outside 'work.txt')) -ne 'junction target data') { throw 'Uninstall changed the personal junction or its target.' }
    Complete-InstallTestStep 'uninstall-preserves-personal-files'
    $passed = $true
} catch { $failure = $_ }
finally {
    [Environment]::SetEnvironmentVariable('EDEN_CONFIG', $previousConfig, 'Process')
    $cleanup = Remove-InstallTestSystemState $ownership
    if ($cleanup.errors.Count -or $cleanup.registrationRemains -or $cleanup.desktopRemains -or $cleanup.startMenuRemains) {
        $passed = $false
        if (-not $failure) {
            $failure = [Management.Automation.ErrorRecord]::new([InvalidOperationException]::new('Owned installation cleanup failed.'), 'InstallationCleanupFailed', 'InvalidResult', $testRoot)
        }
    }
    Write-InstallTestJSON (Join-Path $testRoot 'results.json') @{success=$passed;scenarios=$results.ToArray();installerSHA256=$ownership.installerSHA256;cleanup=$cleanup;error=if($failure){$failure.Exception.Message}else{$null};workbench='real initial/account entry; no external account login';vmStartupTested=$false;utc=[DateTime]::UtcNow.ToString('o')}
}
if ($failure) { throw $failure }
Write-Host 'Complete package installation, workbench entry, reinstall, shortcuts, uninstall and personal-data preservation passed. VM startup was not tested.'

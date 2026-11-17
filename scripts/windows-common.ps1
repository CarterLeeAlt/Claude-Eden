# Shared Windows build helpers; environment ownership lives in run-in-workspace.mjs.
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$PSNativeCommandUseErrorActionPreference = $true
[Console]::InputEncoding = [Text.UTF8Encoding]::new($false)
[Console]::OutputEncoding = [Text.UTF8Encoding]::new($false)
$OutputEncoding = [Console]::OutputEncoding

function Assert-WorkspaceEnvironment {
    & (Join-Path $PSScriptRoot 'assert-workspace.ps1')
}
Assert-WorkspaceEnvironment

function Invoke-BuildTool {
    param([string]$Tool, [string[]]$Arguments)
    if ([IO.Path]::GetFileNameWithoutExtension($Tool) -eq 'go' -and $Arguments -contains 'test') {
        & node (Join-Path $PSScriptRoot 'run-in-workspace.mjs') --cwd (Get-Location).Path -- $Tool @Arguments
    } else {
        & $Tool @Arguments
    }
    if ($LASTEXITCODE -ne 0) { throw "$Tool failed with exit code $LASTEXITCODE" }
}

function Get-WindowsWorkbenchTestBase {
    Assert-WorkspaceEnvironment
    return [IO.Path]::GetFullPath($env:TEMP)
}

function ConvertTo-SmokeSafeText {
    param($Value)
    if ($Value -isnot [string]) { return '' }
    $text = [regex]::Replace($Value, 'https?://[^\s"''<>]+', {
        param($match)
        try { return ([Uri]$match.Value).GetLeftPart([UriPartial]::Path) }
        catch { return '[URL omitted]' }
    })
    $text = $text -replace '[?#][^\s"''<>)]*', '[redacted]'
    return $text.Substring(0, [Math]::Min(768, $text.Length))
}

function Assert-WindowsWorkbenchReport {
    param([int]$ExitCode, [string]$Report)
    if (-not (Test-Path -LiteralPath $Report -PathType Leaf)) { throw "Claude Eden workbench acceptance failed ($ExitCode). No smoke report: $Report" }
    if ((Get-Item -LiteralPath $Report).Length -gt 65536) { throw 'Claude Eden smoke report exceeds the bounded diagnostic size.' }
    $result = Get-Content -LiteralPath $Report -Raw | ConvertFrom-Json -AsHashtable
    $safe = @{ phase = ConvertTo-SmokeSafeText $result['phase']; nativeStatus = ConvertTo-SmokeSafeText $result['nativeStatus']; errors = @(); pageErrors = @() }
    if ($result['errors']) {
        $safe.errors = @($result['errors'] | Select-Object -First 8 | ForEach-Object { ConvertTo-SmokeSafeText $_ })
    }
    if ($result['prepareExit'] -is [int] -or $result['prepareExit'] -is [long]) { $safe.prepareExit = $result['prepareExit'] }
    if ($result['prepareStatus'] -in @('prepared', 'stop_required', 'error', 'unexpected')) { $safe.prepareStatus = $result['prepareStatus'] }
    if ($result['prepareMessage'] -is [string]) { $safe.prepareMessage = ConvertTo-SmokeSafeText $result['prepareMessage'] }
    if ($result['pageErrors']) {
        $safe.pageErrors = @($result['pageErrors'] | Select-Object -First 8 | ForEach-Object {
            if ($_ -isnot [System.Collections.IDictionary]) { return }
            @{
                type = ConvertTo-SmokeSafeText $_['type']
                source = if ($_['source'] -is [string] -and $_['source'].StartsWith('/')) { ConvertTo-SmokeSafeText $_['source'] } else { '' }
                message = ConvertTo-SmokeSafeText $_['message']
                stack = @($_['stack'] | Select-Object -First 6 | ForEach-Object { ConvertTo-SmokeSafeText $_ })
            }
        })
    }
    # Log only these already-bounded fields, even when the process failed. Do
    # not print launch entries, page URLs, account data or the raw report.
    Write-Host ('Claude Eden smoke diagnostic: ' + ($safe | ConvertTo-Json -Depth 6 -Compress))
    if ($ExitCode -ne 0 -or $result['success'] -ne $true -or $result['nativeBridgeReady'] -ne $true -or $safe.errors.Count -gt 0 -or $safe.pageErrors.Count -gt 0) {
        throw "Claude Eden workbench/bridge acceptance failed ($ExitCode), phase $($safe.phase): $($safe.nativeStatus). Diagnostics: $Report"
    }
}

function Wait-WindowsWorkbenchFiles {
    param([Parameter(Mandatory)][string]$AppDirectory)
    # The log relay can outlive the controller's profile lock. Wait for the
    # isolated build's executable images to be released before moving them.
    $deadline = [DateTime]::UtcNow.AddSeconds(20)
    foreach ($relative in @('Eden.exe', 'resources/eden.exe')) {
        $file = Join-Path $AppDirectory $relative
        while ($true) {
            try {
                $handle = [IO.File]::Open($file, [IO.FileMode]::Open, [IO.FileAccess]::ReadWrite, [IO.FileShare]::None)
                $handle.Dispose()
                break
            } catch [IO.IOException], [UnauthorizedAccessException] {
                if ([DateTime]::UtcNow -ge $deadline) { throw "Smoke executable is still in use: $relative" }
                Start-Sleep -Milliseconds 100
            }
        }
    }
}

function Assert-WindowsWorkbench {
    param([Parameter(Mandatory)][string]$AppDirectory)
    if (-not $IsWindows) { throw 'Workbench acceptance requires Windows.' }
    $testRoot = Join-Path (Get-WindowsWorkbenchTestBase) ('eden-build-ui-' + [guid]::NewGuid().ToString('N'))
    New-Item -ItemType Directory -Path $testRoot -Force | Out-Null
    $report = Join-Path $testRoot 'smoke.json'
    $start = [Diagnostics.ProcessStartInfo]::new((Join-Path $AppDirectory 'Eden.exe'))
    $start.UseShellExecute = $false
    $start.WorkingDirectory = $AppDirectory
    $start.Environment['EDEN_CONFIG'] = Join-Path $testRoot 'profile/config.json'
    $start.ArgumentList.Add('--smoke-test')
    $start.ArgumentList.Add($report)
    $window = [Diagnostics.Process]::Start($start)
    $passed = $false
    try {
        if (-not $window.WaitForExit(180000)) { throw "Claude Eden workbench acceptance timed out. Diagnostics retained at $testRoot" }
        Assert-WindowsWorkbenchReport -ExitCode $window.ExitCode -Report $report
        Wait-WindowsWorkbenchFiles -AppDirectory $AppDirectory
        $passed = $true
    } finally {
        if (-not $window.HasExited) { $window.Kill($true); $window.WaitForExit() }
        $window.Dispose()
        if (-not $passed) {
            # Close only this test's authenticated controller. Killing every
            # eden.exe could terminate another installed application or VM.
            try {
                $cleanup = [Diagnostics.ProcessStartInfo]::new((Join-Path $AppDirectory 'resources/eden.exe'))
                $cleanup.UseShellExecute = $false
                $cleanup.CreateNoWindow = $true
                $cleanup.RedirectStandardOutput = $true
                $cleanup.RedirectStandardError = $true
                $cleanup.Environment['EDEN_CONFIG'] = Join-Path $testRoot 'profile/config.json'
                $cleanup.ArgumentList.Add('prepare-update'); $cleanup.ArgumentList.Add('--json')
                $controller = [Diagnostics.Process]::Start($cleanup)
                try {
                    $stdout = $controller.StandardOutput.ReadToEndAsync()
                    $stderr = $controller.StandardError.ReadToEndAsync()
                    if (-not $controller.WaitForExit(20000)) { $controller.Kill($true); $controller.WaitForExit() }
                    if ($controller.ExitCode -ne 0) { Write-Warning 'The isolated smoke controller did not confirm shutdown; its profile is retained.' }
                } finally { $controller.Dispose() }
            } catch { Write-Warning "Isolated smoke cleanup failed: $($_.Exception.Message)" }
        }
        # A successful smoke report is written only after the isolated backend
        # confirms shutdown. Failed profiles remain available for diagnosis.
        if ($passed) { Remove-Item -LiteralPath $testRoot -Recurse -Force -ErrorAction SilentlyContinue }
    }
}

function Get-EdenVersion {
    param([string]$Repository)
    $value = (Get-Content -LiteralPath (Join-Path $Repository 'VERSION') -Raw).Trim()
    if ($value -notmatch '^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$') { throw 'Invalid VERSION.' }
    return $value
}

function Get-BuildCommand {
    param([string[]]$Names)
    foreach ($name in $Names) {
        $command = Get-Command $name -ErrorAction SilentlyContinue
        if ($command) { return $command.Source }
    }
    throw "Missing build tool: $($Names -join ' or ')"
}

function Get-VerifiedDependency {
    param($Dependency, [string]$Cache)
    if ($Dependency.url -notlike 'https://*') { throw 'Build dependencies must use HTTPS.' }
    $path = Join-Path $Cache $Dependency.file
    if (-not (Test-Path -LiteralPath $path -PathType Leaf)) {
        New-Item -ItemType Directory -Path $Cache -Force | Out-Null
        $partial = "$path.partial"
        try {
            # Release hosts and CDNs drop connections now and then. Retry the
            # transfer; the checksum below still decides what is accepted.
            for ($attempt = 1; ; $attempt++) {
                try {
                    Invoke-WebRequest -Uri $Dependency.url -OutFile $partial -TimeoutSec 900 -MaximumRedirection 5
                    break
                } catch {
                    if ($attempt -ge 4) { throw }
                    Write-Warning "Download of $($Dependency.file) failed (attempt $attempt): $($_.Exception.Message)"
                    Start-Sleep -Seconds (5 * $attempt)
                }
            }
            Move-Item -LiteralPath $partial -Destination $path -Force
        } finally {
            if (Test-Path -LiteralPath $partial) { Remove-Item -LiteralPath $partial -Force }
        }
    }
    $actual = (Get-FileHash -LiteralPath $path -Algorithm $Dependency.algorithm).Hash
    if ($actual -ne $Dependency.hash) { throw "Dependency checksum mismatch: $($Dependency.file). Remove the cached file and retry." }
    return $path
}

function Expand-BuildArchive {
    param([string]$Archive, [string]$Destination, [string]$SevenZip)
    New-Item -ItemType Directory -Path $Destination -Force | Out-Null
    Invoke-BuildTool $SevenZip @('x', '-y', '-bso0', '-bsp0', "-o$Destination", $Archive)
}

function Get-VerifiedGitArchive {
    param($Dependency, [string]$Cache)
    $archive = Join-Path $Cache $Dependency.file
    if (-not (Test-Path -LiteralPath $archive)) {
        $git = Get-BuildCommand @('git.exe', 'git')
        $checkout = Join-Path $Cache ('source-' + [guid]::NewGuid().ToString('N'))
        $partial = "$archive.partial"
        try {
            Invoke-BuildTool $git @('init', '--bare', '--quiet', $checkout)
            Invoke-BuildTool $git @('--git-dir', $checkout, 'fetch', '--depth=1', $Dependency.repository, $Dependency.commit)
            $actualCommit = & $git '--git-dir' $checkout 'rev-parse' 'FETCH_HEAD'
            if ($LASTEXITCODE -ne 0 -or $actualCommit -ne $Dependency.commit) { throw 'Fetched source commit does not match the pinned QEMU build.' }
            # Git for Windows defaults can change archive bytes. Keep the
            # pinned source identical to the reviewed POSIX-generated archive.
            Invoke-BuildTool $git @('--git-dir', $checkout, '-c', 'core.autocrlf=false', '-c', 'tar.umask=0002', 'archive', '--format=tar', "--prefix=$($Dependency.prefix)", "--output=$partial", $Dependency.commit)
            $actualHash = (Get-FileHash -LiteralPath $partial -Algorithm SHA256).Hash.ToLowerInvariant()
            if ($actualHash -ne $Dependency.hash) { throw "Source archive checksum mismatch: $($Dependency.file); expected $($Dependency.hash), actual $actualHash" }
            Move-Item -LiteralPath $partial -Destination $archive
        } finally {
            Remove-Item -LiteralPath $checkout -Recurse -Force -ErrorAction SilentlyContinue
            Remove-Item -LiteralPath $partial -Force -ErrorAction SilentlyContinue
        }
    }
    $actualHash = (Get-FileHash -LiteralPath $archive -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($actualHash -ne $Dependency.hash) { throw "Source archive checksum mismatch: $($Dependency.file); expected $($Dependency.hash), actual $actualHash. Remove the cached file and retry." }
    return $archive
}

function Get-SigningTool {
    $command = Get-Command signtool.exe -ErrorAction SilentlyContinue
    if ($command) { return $command.Source }
    if ($IsWindows) {
        $kits = Join-Path ${env:ProgramFiles(x86)} 'Windows Kits/10/bin'
        $candidate = Get-ChildItem -LiteralPath $kits -Directory -ErrorAction SilentlyContinue |
            Sort-Object Name -Descending | ForEach-Object { Join-Path $_.FullName 'x64/signtool.exe' } |
            Where-Object { Test-Path -LiteralPath $_ } | Select-Object -First 1
        if ($candidate) { return $candidate }
    }
    throw 'Release signing requires Windows SDK signtool.exe on Windows.'
}

function Assert-WindowsSigningConfiguration {
    if (-not $IsWindows) { throw 'Trusted Windows build verification requires Windows.' }
    $null = Get-SigningTool
    if ($env:EDEN_WINDOWS_SIGNING_PROVIDER -eq 'esigner') {
        if ($env:EDEN_ESIGNER_CREDENTIAL_ID -notmatch '^[0-9A-Fa-f-]{36}$' -or
            $env:EDEN_ESIGNER_CERTIFICATE_SERIAL -notmatch '^[0-9A-Fa-f]{32,40}$') {
            throw 'eSigner requires EDEN_ESIGNER_CREDENTIAL_ID and the expected EDEN_ESIGNER_CERTIFICATE_SERIAL.'
        }
    } elseif ($env:EDEN_WINDOWS_SIGNING_PROVIDER -and $env:EDEN_WINDOWS_SIGNING_PROVIDER -ne 'certificate-store') {
        throw 'Unknown EDEN_WINDOWS_SIGNING_PROVIDER; use esigner or certificate-store.'
    } elseif ($env:EDEN_WINDOWS_CERT_SHA1 -notmatch '^[0-9A-Fa-f]{40}$') {
        throw 'Certificate-store signing requires EDEN_WINDOWS_CERT_SHA1.'
    }
}

function Sign-EdenFile {
    param([string]$Path, [string]$SignTool)
    Assert-WindowsSigningConfiguration
    if ($env:EDEN_WINDOWS_SIGNING_PROVIDER -eq 'esigner') {
        $Path = [IO.Path]::GetFullPath($Path)
        $stage = Join-Path ([IO.Path]::GetTempPath()) ('eden-signed-' + [guid]::NewGuid().ToString('N'))
        try {
            Invoke-BuildTool (Get-BuildCommand @('node.exe', 'node')) @((Join-Path $PSScriptRoot 'esigner.mjs'), 'sign', $Path, '--output-dir', $stage)
            $signed = Join-Path $stage ([IO.Path]::GetFileName($Path))
            Invoke-BuildTool $SignTool @('verify', '/pa', '/all', '/tw', $signed)
            $signature = Get-AuthenticodeSignature -LiteralPath $signed
            if ($signature.Status -ne 'Valid' -or -not $signature.TimeStamperCertificate -or
                $signature.SignerCertificate.SerialNumber -ne $env:EDEN_ESIGNER_CERTIFICATE_SERIAL -or
                ($env:EDEN_WINDOWS_CERT_SHA1 -and $signature.SignerCertificate.Thumbprint -ne $env:EDEN_WINDOWS_CERT_SHA1)) {
                throw 'eSigner output lacks the expected trusted publisher certificate or timestamp. Original file retained.'
            }
            $env:EDEN_WINDOWS_CERT_SHA1 = $signature.SignerCertificate.Thumbprint
            Copy-Item -LiteralPath $signed -Destination $Path -Force
        } finally { if (Test-Path -LiteralPath $stage) { Remove-Item -LiteralPath $stage -Recurse -Force } }
    } else {
        Invoke-BuildTool $SignTool @('sign', '/sha1', $env:EDEN_WINDOWS_CERT_SHA1, '/fd', 'SHA256',
            '/tr', 'http://timestamp.digicert.com', '/td', 'SHA256', '/d', 'Claude Eden', $Path)
        Invoke-BuildTool $SignTool @('verify', '/pa', '/all', $Path)
    }
}

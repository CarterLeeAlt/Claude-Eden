#requires -Version 5.1
# PowerShell callers share the same Go transaction engine as NSIS and recovery.
[CmdletBinding()]
param(
    [Parameter(Mandatory)][ValidateSet('prepare', 'move', 'restore', 'cleanup', 'complete', 'uninstall', 'detach', 'attach')][string]$Operation,
    [Parameter(Mandatory)][string]$Target,
    [string]$Source,
    [string]$Receipt,
    [string]$Manifest,
    [string]$Backup,
    [string]$Diagnostics,
    [switch]$LegacyTarget,
    [string[]]$KeepInPlace = @(),
    [string]$Helper = (Join-Path $PSScriptRoot 'eden-install.exe')
)
$ErrorActionPreference = 'Stop'
[Console]::InputEncoding = [Text.UTF8Encoding]::new($false)
[Console]::OutputEncoding = [Text.UTF8Encoding]::new($false)
$OutputEncoding = [Console]::OutputEncoding
if (!(Test-Path -LiteralPath $Helper -PathType Leaf)) { throw 'Build cmd/eden-install and pass its workspace executable with -Helper.' }
$arguments = @('-Operation', $Operation, '-Target', $Target)
foreach ($name in @('Source', 'Receipt', 'Manifest', 'Backup', 'Diagnostics')) {
    $value = Get-Variable -Name $name -ValueOnly
    if ($value) { $arguments += @("-$name", $value) }
}
if ($LegacyTarget) { $arguments += '-LegacyTarget' }
foreach ($name in $KeepInPlace) { $arguments += @('-KeepInPlace', $name) }
& $Helper @arguments
exit $LASTEXITCODE

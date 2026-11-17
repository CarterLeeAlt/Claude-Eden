#requires -Version 7.0
# Pure harness regressions: never run installation or touch certificate stores.
$ErrorActionPreference = 'Stop'
& (Join-Path $PSScriptRoot 'assert-workspace.ps1')
$tokens = $null
$errors = $null
$ast = [Management.Automation.Language.Parser]::ParseFile((Join-Path $PSScriptRoot 'test-windows-update-flow.ps1'), [ref]$tokens, [ref]$errors)
if ($errors.Count) { throw ($errors.Message -join '; ') }
$function = $ast.Find({ param($node) $node -is [Management.Automation.Language.FunctionDefinitionAst] -and $node.Name -eq 'Get-FixtureUtc' }, $true)
if (!$function) { throw 'Missing fixture timestamp parser.' }
. ([scriptblock]::Create($function.Extent.Text))
$culture = [Globalization.CultureInfo]::CurrentCulture
try {
    $expected = [DateTime]::new(2026, 11, 17, 11, 30, 58, [DateTimeKind]::Utc)
    foreach ($name in @('zh-CN', 'en-US', 'de-DE', 'ja-JP')) {
        [Globalization.CultureInfo]::CurrentCulture = [Globalization.CultureInfo]::GetCultureInfo($name)
        foreach ($value in @('2026-11-17T11:30:58Z', '2026-11-17T19:30:58+08:00', $expected, $expected.ToLocalTime())) {
            $actual = Get-FixtureUtc $value
            if ($actual -ne $expected -or $actual.Kind -ne [DateTimeKind]::Utc) { throw "Incorrect fixture UTC under $name." }
        }
        $json = '{"utc":"2026-11-17T11:30:58Z"}' | ConvertFrom-Json
        if ((Get-FixtureUtc $json.utc) -ne $expected) { throw "JSON timestamp changed under $name." }
    }
    $rejected = $false
    try { $null = Get-FixtureUtc 'not-a-time' } catch { $rejected = $true }
    if (!$rejected) { throw 'Malformed fixture timestamp accepted.' }
} finally { [Globalization.CultureInfo]::CurrentCulture = $culture }
Write-Output 'PASS: fixture UTC strings, JSON dates and DateTime values across four cultures; invalid input rejected.'

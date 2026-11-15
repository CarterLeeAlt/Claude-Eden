# Validate the single environment contract before a PowerShell entry writes files.
& node (Join-Path $PSScriptRoot 'run-in-workspace.mjs') --check
if ($LASTEXITCODE -ne 0) { throw 'Start with node scripts/run-in-workspace.mjs -- pwsh -NoProfile -File <script>.' }

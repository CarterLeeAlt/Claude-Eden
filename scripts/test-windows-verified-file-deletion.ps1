#requires -Version 5.1
# The race and reparse assertions exercise the shipped Go handle implementation.
$ErrorActionPreference = 'Stop'
& (Join-Path $PSScriptRoot 'assert-workspace.ps1')
if (!$env:GOROOT) { throw 'Set GOROOT to the workspace Go toolchain.' }
$go = Join-Path $env:GOROOT 'bin/go.exe'
$repository = Split-Path $PSScriptRoot -Parent
& $go run ./scripts/check-test-paths -root $repository
if ($LASTEXITCODE -ne 0) { throw 'Go test path gate failed.' }
& $go test ./internal/installfiles -run 'TestVerifiedHandleDeletionRace|TestReparseTraversalProtection|TestJunctionIdentityPreservedByUpdateAndRollback' -count=1 -v
if ($LASTEXITCODE -ne 0) { throw 'Verified-handle deletion regression failed.' }

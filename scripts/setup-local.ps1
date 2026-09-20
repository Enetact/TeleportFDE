[CmdletBinding()]
param([string]$Distribution = 'Ubuntu-24.04')
$ErrorActionPreference = 'Stop'
$repoRoot = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '..')).Path
& wsl -d $Distribution --cd $repoRoot -- bash scripts/setup-dev.sh
if ($LASTEXITCODE -ne 0) { throw "Development setup failed with exit code $LASTEXITCODE." }

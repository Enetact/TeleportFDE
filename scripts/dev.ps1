[CmdletBinding()]
param(
    [string]$Distribution = 'Ubuntu-24.04',
    [Parameter(ValueFromRemainingArguments = $true)][string[]]$MakeArguments
)
$ErrorActionPreference = 'Stop'
$repoRoot = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '..')).Path
& wsl -d $Distribution --cd $repoRoot -- bash scripts/dev.sh @MakeArguments
if ($LASTEXITCODE -ne 0) { throw "Development command failed with exit code $LASTEXITCODE." }

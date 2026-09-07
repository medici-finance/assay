<#
  bootstrap-windows.ps1 — the ~5-line first-install bootstrap for the Go-native
  installer (deskinstall). It solves the chicken-and-egg of the FIRST install:
  you need a binary to run the installer, so this fetches ONLY that first binary
  (statusgen) and HASH-VERIFIES it before executing anything. From there,
  `deskinstall`/`statusgen` does the full pinned, sha256-verified acquisition of
  the rest against paired-versions.yaml.

  The security-critical hash-verify lives in ONE tested Go implementation; this
  bootstrap is confined to a trivial, auditable download-and-verify. It refuses —
  it does NOT warn and continue — on a hash mismatch, and executes nothing before
  the hash matches. Pass the pinned sha256 (copy it from the release's
  paired-versions.yaml / checksums.txt); the bootstrap never invents or fetches
  a hash of its own.

  Usage:
    powershell -ExecutionPolicy Bypass -File bootstrap-windows.ps1 `
      -Tag v0.26.0 -Arch amd64 `
      -Sha256 fa549fa1e19a0006ef109c61265aab397d528f7c69aa9181ee963a5f8d9b2f39
#>
param(
  [Parameter(Mandatory=$true)][string]$Tag,
  [Parameter(Mandatory=$true)][ValidatePattern('^[0-9a-f]{64}$')][string]$Sha256,
  [ValidateSet('amd64','arm64')][string]$Arch = ($env:PROCESSOR_ARCHITECTURE -eq 'ARM64' ? 'arm64' : 'amd64'),
  [string]$Dest = (Join-Path $env:LOCALAPPDATA 'Assay\bin')
)
$ErrorActionPreference = 'Stop'
$asset = "statusgen-windows-$Arch.exe"
$url   = "https://github.com/medici-finance/assay/releases/download/$Tag/$asset"
$tmp   = Join-Path $env:TEMP $asset
Invoke-WebRequest -Uri $url -OutFile $tmp
$got = (Get-FileHash -Algorithm SHA256 -Path $tmp).Hash.ToLower()
if ($got -ne $Sha256) { Remove-Item $tmp -Force; throw "REFUSED: sha256 mismatch for $asset (got $got, pinned $Sha256) — no unverified bytes installed" }
New-Item -ItemType Directory -Force -Path $Dest | Out-Null
Move-Item -Force $tmp (Join-Path $Dest $asset)
Write-Host "bootstrapped $asset $Tag sha256:$Sha256 -> $Dest"

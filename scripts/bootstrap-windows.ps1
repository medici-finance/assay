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
  the hash matches.

  The pinned tag + sha256 are resolved from the committed manifest
  (plugins/assay/paired-versions.yaml, `statusgen.platforms`) — the same file CI
  and every other acquisition path reads, so there is nothing left to transcribe
  by hand. `-Sha256` is an OPTIONAL override whose only legal use is to re-assert
  the manifest value: supplied-and-equal proceeds, supplied-and-different REFUSES
  naming both digests. The bootstrap never invents or fetches a hash of its own.

  After a successful install it adds $Dest to the current user's PATH (segment-
  guarded, idempotent) so `statusgen` resolves by bare name in a NEW shell —
  already-open shells do not pick this up.

  Usage (three-command install, step 1):
    powershell -ExecutionPolicy Bypass -File bootstrap-windows.ps1 -Tag v1.0.6

  Usage (explicit override, must agree with the manifest):
    powershell -ExecutionPolicy Bypass -File bootstrap-windows.ps1 `
      -Tag v1.0.6 -Sha256 235e87d0c36e22f9798b7f9441fda8ec902a0cc2b5adf3f340ec5cf29f33916a
#>
param(
  [Parameter(Mandatory=$true)][string]$Tag,
  [Parameter()][ValidatePattern('^[0-9a-f]{64}$')][string]$Sha256,
  [ValidateSet('amd64','arm64')][string]$Arch = ($env:PROCESSOR_ARCHITECTURE -eq 'ARM64' ? 'arm64' : 'amd64'),
  [string]$Dest = (Join-Path $env:LOCALAPPDATA 'Assay\bin'),
  # Override for testing only — the adopter-facing usage never passes this; the default
  # locates the manifest committed in the same clone this script lives in.
  [string]$ManifestPath = (Join-Path (Split-Path -Parent $PSScriptRoot) 'plugins/assay/paired-versions.yaml')
)
$ErrorActionPreference = 'Stop'
$asset = "statusgen-windows-$Arch.exe"
$platformKey = "windows-$Arch"

# --- Resolve tag + sha256 from the committed manifest (never fetched live) ----------------------
# Grammar (plugins/assay/paired-versions.yaml, `statusgen.platforms`): one line per platform,
# `<platform-key>: <artifact> <tag> <sha256>`. Scoped to the `statusgen:` top-level block only —
# the sibling `desk-tools:` block carries its OWN windows-<arch> lines with a different asset/sha.
# Every resolution failure below is a distinct, named refusal; nothing is downloaded before one of
# them either passes or is bypassed by construction (there is no fall-through path).
if (-not (Test-Path $ManifestPath)) {
  throw "REFUSED: manifest not found at $ManifestPath — cannot resolve the pinned sha256 for $platformKey"  # guard:resolve
}
$manifestLines = @(Get-Content -Path $ManifestPath -ErrorAction Stop)
if ($manifestLines.Count -eq 0) {
  throw "REFUSED: manifest at $ManifestPath is empty — cannot resolve the pinned sha256 for $platformKey"  # guard:resolve
}
$blockStart = -1
for ($i = 0; $i -lt $manifestLines.Count; $i++) {
  if ($manifestLines[$i] -match '^statusgen:\s*$') { $blockStart = $i; break }
}
if ($blockStart -lt 0) {
  throw "REFUSED: manifest at $ManifestPath has no top-level 'statusgen:' block — cannot resolve the pinned sha256 for $platformKey"  # guard:resolve
}
$blockEnd = $manifestLines.Count
for ($i = $blockStart + 1; $i -lt $manifestLines.Count; $i++) {
  if ($manifestLines[$i] -match '^\S') { $blockEnd = $i; break }
}
$pinLine = $null
for ($i = $blockStart; $i -lt $blockEnd; $i++) {
  if ($manifestLines[$i] -match ('^\s*' + [regex]::Escape($platformKey) + ':\s+(\S.*)$')) {
    $pinLine = $Matches[1].Trim()
    break
  }
}
if (-not $pinLine) {
  throw "REFUSED: manifest at $ManifestPath has no statusgen.platforms line for $platformKey — cannot resolve the pinned sha256"  # guard:resolve
}
$fields = @($pinLine -split '\s+')
if ($fields.Count -ne 3) {
  throw "REFUSED: manifest pin line for $platformKey does not split into exactly 3 fields (artifact tag sha256): '$pinLine'"  # guard:resolve
}
$manifestAsset  = $fields[0]
$manifestTag    = $fields[1]
$manifestSha256 = $fields[2]
if ($manifestSha256 -notmatch '^[0-9a-f]{64}$') {
  throw "REFUSED: manifest sha256 for $platformKey is not 64 lowercase hex: '$manifestSha256'"  # guard:resolve
}
if ($manifestTag -ne $Tag) {
  throw "REFUSED: requested tag $Tag does not match the manifest's pinned tag $manifestTag for $platformKey — refusing to resolve a sha256 the manifest never pinned"  # guard:resolve
}

if ($Sha256) {
  if ($Sha256 -ne $manifestSha256) {
    throw "REFUSED: supplied -Sha256 $Sha256 disagrees with the manifest's pinned sha256 $manifestSha256 for $platformKey — refusing to install on a disputed digest"
  }
} else {
  $Sha256 = $manifestSha256
}
Write-Host "resolved $platformKey from manifest: tag=$manifestTag sha256=$Sha256"

# --- Download + verify-or-refuse (unchanged control; still precedes placement) ------------------
$url = "https://github.com/medici-finance/assay/releases/download/$Tag/$asset"
$tmp = Join-Path $env:TEMP $asset
Invoke-WebRequest -Uri $url -OutFile $tmp
$got = (Get-FileHash -Algorithm SHA256 -Path $tmp).Hash.ToLower()
if ($got -ne $Sha256) { Remove-Item $tmp -Force; throw "REFUSED: sha256 mismatch for $asset (got $got, pinned $Sha256) — no unverified bytes installed" }

# --- Place only verified bytes --------------------------------------------------------------------
New-Item -ItemType Directory -Force -Path $Dest | Out-Null
Move-Item -Force $tmp (Join-Path $Dest $asset)
# Keep the asset-named copy (deskinstall and the existing CI smoke both expect
# statusgen-windows-<arch>.exe to exist under $Dest); additionally place a bare `statusgen.exe`
# copy so the three-command install target — `statusgen --version` resolving by name — holds.
Copy-Item -Force (Join-Path $Dest $asset) (Join-Path $Dest 'statusgen.exe')
Write-Host "bootstrapped $asset $Tag sha256:$Sha256 -> $Dest"

# --- Write the user PATH entry, idempotently -------------------------------------------------------
$userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
$segments = @()
if ($userPath) { $segments = @($userPath -split ';' | Where-Object { $_ -ne '' }) }
$alreadyOnPath = $segments | Where-Object { $_.TrimEnd('\') -eq $Dest.TrimEnd('\') }
if (-not $alreadyOnPath) {
  $newPath = if ($userPath) { "$userPath;$Dest" } else { $Dest }
  [Environment]::SetEnvironmentVariable('Path', $newPath, 'User')
  Write-Host "added $Dest to the user PATH"
} else {
  Write-Host "$Dest is already on the user PATH"
}
Write-Host "NOTE: already-open shells do not see this PATH change — open a new shell to run 'statusgen'"

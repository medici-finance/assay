<#
.SYNOPSIS
  Exercise scripts/bootstrap-windows.ps1's sha256 hash-verify at Windows runtime (windows-port/03,
  #595 / decision #508). Run by the windows-smoke CI job.

.DESCRIPTION
  Three assertions, run against the REAL bootstrap script:

    1. TAMPERED checksum -> the bootstrap REFUSES: it throws with a "sha256 mismatch" message
       (distinguished from a mere download failure) and installs nothing.
    2. UNTAMPERED (the pinned) checksum -> the bootstrap SUCCEEDS and the asset lands in $Dest.
    3. NON-VACUITY -> the same tampered checksum, run against a copy of the bootstrap with the
       hash-check line REMOVED, DOES install the (unverified) asset. This proves the refusal in (1)
       is caused by the hash check, not by some unrelated failure — without it, a green (1) could be
       vacuous.

  The Tag + RealSha256 are the pinned windows-amd64 values from plugins/assay/paired-versions.yaml
  (the CI step extracts them), so this stays correct as the pinned release bumps.
#>
param(
  [Parameter(Mandatory = $true)][string]$Tag,
  [Parameter(Mandatory = $true)][ValidatePattern('^[0-9a-f]{64}$')][string]$RealSha256,
  [ValidateSet('amd64', 'arm64')][string]$Arch = 'amd64'
)
$ErrorActionPreference = 'Stop'

$repoRoot  = Split-Path -Parent $PSScriptRoot          # scripts/ -> repo root
$bootstrap = Join-Path $repoRoot 'scripts/bootstrap-windows.ps1'
if (-not (Test-Path $bootstrap)) { throw "bootstrap not found at $bootstrap" }
$asset    = "statusgen-windows-$Arch.exe"
$tampered = ('0' * 64)   # a valid-SHAPE 64-hex sha256 that cannot match the real asset

function New-CleanDest {
  Join-Path ([System.IO.Path]::GetTempPath()) ("assay-smoke-" + [guid]::NewGuid().ToString())
}

# --- 1. TAMPERED -> REFUSE (the fail-first proof) ---------------------------------------------
$dest1 = New-CleanDest
$err = $null
try   { & $bootstrap -Tag $Tag -Sha256 $tampered -Arch $Arch -Dest $dest1 }
catch { $err = $_.Exception.Message }
if (-not $err) {
  throw "SMOKE FAIL (1/3): the tampered checksum was ACCEPTED — the hash-verify did not refuse"
}
if ($err -notmatch 'sha256 mismatch') {
  throw "SMOKE FAIL (1/3): the tampered run failed, but NOT on the hash check (so not the path under test): $err"
}
if (Test-Path (Join-Path $dest1 $asset)) {
  throw "SMOKE FAIL (1/3): the tampered run left an installed asset at $dest1 — it must install NOTHING"
}
Write-Host "OK 1/3: tampered checksum REFUSED on the hash mismatch; nothing installed"

# --- 2. UNTAMPERED -> ACCEPT -------------------------------------------------------------------
$dest2 = New-CleanDest
& $bootstrap -Tag $Tag -Sha256 $RealSha256 -Arch $Arch -Dest $dest2
if (-not (Test-Path (Join-Path $dest2 $asset))) {
  throw "SMOKE FAIL (2/3): the untampered run did not install $asset to $dest2"
}
Write-Host "OK 2/3: pinned checksum ACCEPTED; $asset installed"

# --- 3. NON-VACUITY: bypass the check, tampered checksum -> installs anyway --------------------
$bypass = Join-Path ([System.IO.Path]::GetTempPath()) ("bootstrap-nohashcheck-" + [guid]::NewGuid().ToString() + ".ps1")
# Drop exactly the hash-check line (the only line comparing against $Sha256); the rest is verbatim.
$kept = Get-Content $bootstrap | Where-Object { $_ -notmatch '-ne \$Sha256' }
Set-Content -Path $bypass -Value $kept
$dest3 = New-CleanDest
& $bypass -Tag $Tag -Sha256 $tampered -Arch $Arch -Dest $dest3
if (-not (Test-Path (Join-Path $dest3 $asset))) {
  throw "SMOKE FAIL (3/3): with the hash check removed, the tampered run should still install $asset — assertion 1 would otherwise be vacuous"
}
Write-Host "OK 3/3: with the hash check removed, the tampered asset installs — assertion 1 is non-vacuous"

Write-Host "windows-bootstrap-hashcheck-smoke: PASS"

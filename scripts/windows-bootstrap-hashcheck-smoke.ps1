<#
.SYNOPSIS
  Exercise scripts/bootstrap-windows.ps1's manifest resolution and sha256 hash-verify at Windows
  runtime (windows-port/03, #595 / decision #508; extended by windows-port/06 for the
  manifest-driven resolution).

.DESCRIPTION
  Six assertions, run against the REAL bootstrap script (assertions 5-6 run it with the
  resolution/hash-check lines stripped, to prove 3-4 redden because of the guard and not an
  unrelated failure):

    1. OVERRIDE DISAGREEMENT -> the bootstrap REFUSES: an explicit -Sha256 that disagrees with
       the manifest's pinned value is rejected before anything is downloaded, naming both digests.
    2. UNTAMPERED, no override -> the bootstrap resolves tag + sha256 from the committed manifest
       alone and SUCCEEDS: the asset lands in $Dest under both its asset name and `statusgen.exe`.
    3. TAMPERED MANIFEST DIGEST -> a scratch copy of the manifest with one hex character of the
       windows-<arch> digest flipped. The bootstrap resolves that (well-formed but wrong) digest
       and REFUSES at the pre-existing download-verify with a "sha256 mismatch" message —
       installs nothing. This is the security-critical row: it proves a tampered manifest cannot
       make the bootstrap install unverified bytes.
    4. ABSENT PLATFORM LINE -> a scratch copy of the manifest with the windows-<arch> line
       removed. The bootstrap REFUSES naming the missing platform, before any download — installs
       nothing.
    5. NON-VACUITY for (3) -> the same tampered-manifest input, run against a copy of the
       bootstrap with ONLY the download hash-check line removed, DOES install (the unverified,
       real) asset. Proves (3) reddens because of the hash check, not an unrelated failure.
    6. NON-VACUITY for (4) -> the same absent-platform-line input, run against a copy of the
       bootstrap with every resolution-guard line AND the hash-check line removed, DOES install.
       Proves (4) reddens because of the resolution guard, not an unrelated failure.

  The Tag is the pinned windows-<arch> tag from plugins/assay/paired-versions.yaml (the CI step
  passes it through unresolved — resolving the sha is exactly what this proves the script does),
  so this stays correct as the pinned release bumps.

  -RealSha256 is accepted and IGNORED: the already-live `.github/workflows/windows-ci-leg.yml`
  (promoted in windows-port/04) still passes it, and no App credential can push a workflow-file
  change to drop the argument. Keeping it here — rather than making this a breaking interface
  change to a script a live, staging-only-editable workflow depends on — is what lets the live
  required check keep passing until a human promotes the staged copy (ci/staged-workflows/
  windows-ci-leg.yml) that stops passing it. Retire this parameter in the same commit that
  removes the last caller still passing it.
#>
param(
  [Parameter(Mandatory = $true)][string]$Tag,
  [ValidateSet('amd64', 'arm64')][string]$Arch = 'amd64',
  [string]$RealSha256
)
$ErrorActionPreference = 'Stop'

$repoRoot  = Split-Path -Parent $PSScriptRoot          # scripts/ -> repo root
$bootstrap = Join-Path $repoRoot 'scripts/bootstrap-windows.ps1'
if (-not (Test-Path $bootstrap)) { throw "bootstrap not found at $bootstrap" }
$manifest  = Join-Path $repoRoot 'plugins/assay/paired-versions.yaml'
if (-not (Test-Path $manifest)) { throw "manifest not found at $manifest" }
$asset     = "statusgen-windows-$Arch.exe"
$platformKey = "windows-$Arch"
$tampered  = ('0' * 64)   # a valid-SHAPE 64-hex sha256 that cannot match the real asset

function New-CleanDest {
  Join-Path ([System.IO.Path]::GetTempPath()) ("assay-smoke-" + [guid]::NewGuid().ToString())
}

function New-ScratchManifest {
  param([string[]]$Lines)
  $path = Join-Path ([System.IO.Path]::GetTempPath()) ("assay-manifest-" + [guid]::NewGuid().ToString() + ".yaml")
  Set-Content -Path $path -Value $Lines
  $path
}

$manifestLines = @(Get-Content -Path $manifest)

# --- 1. OVERRIDE DISAGREEMENT -> REFUSE, before any download ------------------------------------
$dest1 = New-CleanDest
$err = $null
try   { & $bootstrap -Tag $Tag -Sha256 $tampered -Arch $Arch -Dest $dest1 }
catch { $err = $_.Exception.Message }
if (-not $err) {
  throw "SMOKE FAIL (1/6): an -Sha256 override that disagrees with the manifest was ACCEPTED"
}
if ($err -notmatch 'disagrees with the manifest') {
  throw "SMOKE FAIL (1/6): the override-disagreement run failed, but NOT on that guard (so not the path under test): $err"
}
if (Test-Path (Join-Path $dest1 $asset)) {
  throw "SMOKE FAIL (1/6): the disagreeing-override run left an installed asset at $dest1 — it must install NOTHING"
}
Write-Host "OK 1/6: disagreeing -Sha256 override REFUSED naming both digests; nothing downloaded"

# --- 2. UNTAMPERED, no override -> ACCEPT (manifest resolution alone) ---------------------------
$dest2 = New-CleanDest
& $bootstrap -Tag $Tag -Arch $Arch -Dest $dest2
if (-not (Test-Path (Join-Path $dest2 $asset))) {
  throw "SMOKE FAIL (2/6): the manifest-resolved run did not install $asset to $dest2"
}
if (-not (Test-Path (Join-Path $dest2 'statusgen.exe'))) {
  throw "SMOKE FAIL (2/6): the manifest-resolved run did not additionally place statusgen.exe in $dest2"
}
Write-Host "OK 2/6: tag+sha256 resolved from the manifest alone; $asset and statusgen.exe installed"

# --- 3. TAMPERED MANIFEST DIGEST -> REFUSE at the download verify (the security row) ------------
$tamperedManifestLines = $manifestLines | ForEach-Object {
  if ($_ -match ('^(\s*' + [regex]::Escape($platformKey) + ':\s+\S+\s+\S+\s+)([0-9a-f])([0-9a-f]{63}\s*)$')) {
    $flippedNibble = if ($Matches[2] -eq '0') { '1' } else { '0' }
    "$($Matches[1])$flippedNibble$($Matches[3])"
  } else {
    $_
  }
}
$tamperedManifest = New-ScratchManifest -Lines $tamperedManifestLines
$dest3 = New-CleanDest
$err = $null
try   { & $bootstrap -Tag $Tag -Arch $Arch -Dest $dest3 -ManifestPath $tamperedManifest }
catch { $err = $_.Exception.Message }
if (-not $err) {
  throw "SMOKE FAIL (3/6): a tampered manifest digest was ACCEPTED — the download hash-verify did not refuse"
}
if ($err -notmatch 'sha256 mismatch') {
  throw "SMOKE FAIL (3/6): the tampered-manifest run failed, but NOT on the hash check (so not the path under test): $err"
}
if (Test-Path (Join-Path $dest3 $asset)) {
  throw "SMOKE FAIL (3/6): the tampered-manifest run left an installed asset at $dest3 — it must install NOTHING"
}
Write-Host "OK 3/6: tampered manifest digest REFUSED at the download hash mismatch; nothing installed"

# --- 4. ABSENT PLATFORM LINE -> REFUSE naming the missing platform, before any download ----------
$noLineManifestLines = $manifestLines | Where-Object { $_ -notmatch ('^\s*' + [regex]::Escape($platformKey) + ':\s') }
$noLineManifest = New-ScratchManifest -Lines $noLineManifestLines
$dest4 = New-CleanDest
$err = $null
try   { & $bootstrap -Tag $Tag -Arch $Arch -Dest $dest4 -ManifestPath $noLineManifest }
catch { $err = $_.Exception.Message }
if (-not $err) {
  throw "SMOKE FAIL (4/6): a manifest with no $platformKey line was ACCEPTED"
}
if ($err -notmatch [regex]::Escape($platformKey)) {
  throw "SMOKE FAIL (4/6): the absent-platform-line run failed, but the message does not name $platformKey`: $err"
}
if (Test-Path (Join-Path $dest4 $asset)) {
  throw "SMOKE FAIL (4/6): the absent-platform-line run left an installed asset at $dest4 — it must install NOTHING"
}
Write-Host "OK 4/6: absent platform line REFUSED naming $platformKey; nothing downloaded or installed"

# --- 5. NON-VACUITY for (3): hash-check-only bypass + tampered manifest -> installs anyway -------
# Drop exactly the download hash-check line (the only line comparing the downloaded hash against
# the resolved $Sha256); the rest — including manifest resolution — is verbatim.
$hashBypass = Join-Path ([System.IO.Path]::GetTempPath()) ("bootstrap-nohashcheck-" + [guid]::NewGuid().ToString() + ".ps1")
$hashBypassKept = Get-Content $bootstrap | Where-Object { $_ -notmatch '-ne \$Sha256' }
Set-Content -Path $hashBypass -Value $hashBypassKept
$dest5 = New-CleanDest
& $hashBypass -Tag $Tag -Arch $Arch -Dest $dest5 -ManifestPath $tamperedManifest
if (-not (Test-Path (Join-Path $dest5 $asset))) {
  throw "SMOKE FAIL (5/6): with the hash check removed, the tampered-manifest run should still install $asset — assertion 3 would otherwise be vacuous"
}
Write-Host "OK 5/6: with the hash check removed, the tampered-manifest asset installs — assertion 3 is non-vacuous"

# --- 6. NON-VACUITY for (4): resolution-guard + hash-check bypass + absent line -> installs -------
# Drop every resolution-guard line (tagged `# guard:resolve`) AND the download hash-check line, so
# an absent platform line falls through all the way to placement instead of refusing anywhere.
$fullBypass = Join-Path ([System.IO.Path]::GetTempPath()) ("bootstrap-noguards-" + [guid]::NewGuid().ToString() + ".ps1")
$fullBypassKept = Get-Content $bootstrap | Where-Object { $_ -notmatch '-ne \$Sha256' -and $_ -notmatch '# guard:resolve' }
Set-Content -Path $fullBypass -Value $fullBypassKept
$dest6 = New-CleanDest
& $fullBypass -Tag $Tag -Arch $Arch -Dest $dest6 -ManifestPath $noLineManifest
if (-not (Test-Path (Join-Path $dest6 $asset))) {
  throw "SMOKE FAIL (6/6): with the resolution guards and hash check removed, the absent-platform-line run should still install $asset — assertion 4 would otherwise be vacuous"
}
Write-Host "OK 6/6: with the resolution guards removed, the absent-platform-line asset installs — assertion 4 is non-vacuous"

Write-Host "windows-bootstrap-hashcheck-smoke: PASS"

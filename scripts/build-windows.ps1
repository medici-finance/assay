<#
  build-windows.ps1 -- the Windows counterpart to the Unix root `Makefile`.

  The shipped toolchain is Unix-first: the desk-tools build/install targets live
  in the root `Makefile`, whose only documented invocation is `make ...` on a
  POSIX host (and `sudo make desk-install` into root-owned /opt/desk-tools/bin).
  This script is the Windows-native orchestration of the SAME targets. The Go
  tools already cross-compile -- `statusgen` and everything under `tools/**` are
  plain argv CLIs -- so this is orchestration + Windows path handling (`.exe`
  suffixes, `%LOCALAPPDATA%` install dir, `Get-FileHash` manifests), NOT new
  build logic. It requires no `nmake`, no Visual Studio build tools, and no
  `make`: only PowerShell and the Go toolchain (the same dependency every target
  already has). It aligns with the PowerShell bootstrap shipped by
  windows-port/03 (scripts/bootstrap-windows.ps1).

  TARGET PARITY. The set of targets below is a mirror of the Unix Makefile's
  `.PHONY` set, and it must never silently fall behind it. The single declared
  target set is the `$MakefileParityTargets` array between the
  `MAKEFILE-PARITY TARGETS (BEGIN/END)` markers. The `tools/winparity` guard
  reads that block AND the Makefile's `.PHONY` line and FAILS if the two sets
  differ; this script runs that guard as a preflight before executing any
  target, so a target added on one side and not the other is caught at once
  rather than shipping a Windows build that quietly lost a target.

  OPT-IN AUTHENTICODE SIGNING (`-Sign`). By DEFAULT this script signs nothing and
  behaves exactly as before: `desk-build` / `desk-install` emit UNSIGNED PE files.
  Passing `-Sign` opts in to Authenticode self-signing of every built `*.exe`
  (in `tools/desk/dist` and, on `desk-install`, the per-user install dir) with
  `Set-AuthenticodeSignature` (SHA256, RFC-3161 timestamp) BEFORE `desk-manifest`
  hashes them, so `MANIFEST.sha256` matches the signed bytes on disk. The signing
  cert is resolved, in order, from `-CertThumbprint`, `$env:ASSAY_CODESIGN_THUMBPRINT`,
  or the well-known subject `CN=Assay local tools` in `Cert:\CurrentUser\My`; the
  cert MUST carry the Code Signing EKU (a TLS/server-auth cert is refused). When
  `-Sign` is set and no code-signing cert is found the build FAILS CLOSED with a
  setup snippet — it never silently ships unsigned files past a `-Sign` request.
  `-Sign` adds NO requirement for CI or Unix: it is Windows-only and off by default.

  TRUST IMPLICATIONS. The setup snippet imports the self-signed cert into the
  per-user Trusted Publishers AND Trusted Root stores (`Cert:\CurrentUser\*`). That
  makes THIS user trust anything the cert signs as a root CA — so keep the key
  NonExportable, CurrentUser-only, and this-machine-only, and know how to remove it
  (the snippet shows the removal). A self-signed publisher only NAMES the publisher;
  it does NOT clear ML/heuristic AV verdicts (e.g. `Heur.AdvML.D` on a freshly built
  unsigned Go PE) — a per-machine folder exception on the install dir is the control
  that silences that heuristic. Full write-up: docs/adopting-assay.md, "Signing
  from-source Windows builds". Signing PUBLISHED release assets (a real org/EV cert
  in CI) is a documented FOLLOW-ON, NOT implemented here.

  Usage:
    pwsh -File scripts/build-windows.ps1 <target> [options]
    pwsh -File scripts/build-windows.ps1 desk-install -Sign
    pwsh -File scripts/build-windows.ps1 desk-build -Sign -CertThumbprint <hex>
    pwsh -File scripts/build-windows.ps1 -Help
    pwsh -File scripts/build-windows.ps1 -ListTargets

  Targets (mirrored from the Unix Makefile):
    desk-build         local unprivileged build of every tools/desk/cmd/* into
                       tools/desk/dist/ (as <name>.exe). Succeeds with zero cmds.
    desk-install       build (desk-build), then install the built .exe binaries
                       into the per-user install dir (default
                       %LOCALAPPDATA%\Assay\bin), then desk-hook-install +
                       desk-manifest. On Windows this is a per-user install and
                       needs no elevation -- unlike the Unix `sudo make
                       desk-install` into root-owned /opt/desk-tools/bin.
    desk-manifest      (re)write tools/desk/MANIFEST.sha256 from the installed
                       binaries via Get-FileHash (shasum-compatible lines).
    desk-hook-install  install the pre-push hook shim into .githooks/pre-push
                       (idempotent; -Force to overwrite a foreign hook).
    desk-test          run the deskkit test suite (go test ./... -count=1).
    skillslint         run tools/skillslint over the plugin tree.
    guardrail-sync     regenerate every shared-guardrail copy from its source.
    paired-versions    assert the adopter front door (plugin.json vs
                       paired-versions.yaml) is consistent.
#>
[CmdletBinding()]
param(
    [Parameter(Position = 0)]
    [string]$Target,

    # Per-user install directory for desk-install. Windows analogue of the Unix
    # /opt/desk-tools/bin; defaults to %LOCALAPPDATA%\Assay\bin (the same
    # Assay\bin location scripts/bootstrap-windows.ps1 uses).
    [string]$InstallDir = (Join-Path $env:LOCALAPPDATA 'Assay\bin'),

    # Overwrite an existing non-deskpushguard pre-push hook (mirrors the
    # Makefile's `FORCE=1`).
    [switch]$Force,

    # Skip the Makefile-parity preflight. Escape hatch only; the guard is on by
    # default precisely so the target set cannot drift unnoticed.
    [switch]$NoParity,

    # OPT-IN Authenticode self-sign of the built PE files. DEFAULT OFF: with -Sign
    # omitted the build is byte-for-byte the previous unsigned behaviour and makes
    # no cert, network, or Trusted-Root requirement. See the header's OPT-IN
    # AUTHENTICODE SIGNING / TRUST IMPLICATIONS block before using it.
    [switch]$Sign,

    # Explicit code-signing certificate thumbprint (SHA1 hex, spaces ignored) to
    # find in Cert:\CurrentUser\My. Highest-priority cert source when -Sign is set;
    # only consulted when -Sign is set. See Resolve-CodeSigningCert for the order.
    [string]$CertThumbprint,

    # RFC-3161 Authenticode timestamp server, used only when -Sign is set. A
    # timestamp keeps signatures valid past the cert's expiry. Set to '' to sign
    # without a timestamp on an offline host. Never contacted unless -Sign is set.
    [string]$TimestampServer = 'http://timestamp.digicert.com',

    [switch]$ListTargets,
    [switch]$Help
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

# ---------------------------------------------------------------------------
# MAKEFILE-PARITY TARGETS (BEGIN)
# One entry per target mirrored from the Unix Makefile's `.PHONY` set. This is
# the single declared Windows target set; tools/winparity diffs it against the
# Makefile's `.PHONY` line and reddens on any difference. Keep the two in step.
$MakefileParityTargets = @(
    'desk-build'
    'desk-install'
    'desk-manifest'
    'desk-hook-install'
    'desk-test'
    'skillslint'
    'guardrail-sync'
    'paired-versions'
)
# MAKEFILE-PARITY TARGETS (END)
# ---------------------------------------------------------------------------

# Repo layout -- resolved from this script's own location so the target works
# from any CWD. This script lives at <repo>/scripts/build-windows.ps1.
$RepoRoot   = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$DeskDir    = Join-Path $RepoRoot 'tools\desk'
$DistDir    = Join-Path $DeskDir  'dist'
$Manifest   = Join-Path $DeskDir  'MANIFEST.sha256'
$HookSrc    = Join-Path $DeskDir  'hooks\pre-push'
$HookDst    = Join-Path $RepoRoot '.githooks\pre-push'
$DeskModule = 'github.com/medici-finance/assay/tools/desk'
$DeskPkg    = "$DeskModule/internal/deskkit"

function Get-BuildStamp {
    # SourceSHA + BuiltAt, embedded via -ldflags so every audit record and
    # --version shows exactly which source a binary was built from -- the same
    # stamp the Makefile computes.
    $sha = (& git -C $RepoRoot rev-parse --short HEAD 2>$null)
    if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($sha)) { $sha = 'unknown' }
    $builtAt = [DateTime]::UtcNow.ToString('yyyy-MM-ddTHH:mm:ssZ')
    return "-X $DeskPkg.SourceSHA=$sha -X $DeskPkg.BuiltAt=$builtAt"
}

function Get-DeskCmds {
    $cmdDir = Join-Path $DeskDir 'cmd'
    if (-not (Test-Path $cmdDir)) { return @() }
    return @(Get-ChildItem -Path $cmdDir -Directory | Sort-Object Name)
}

# ---------------------------------------------------------------------------
# OPT-IN Authenticode signing. All of this is inert unless -Sign is passed; the
# default build path never calls Resolve-CodeSigningCert or Set-AuthenticodeSignature.
# The Code Signing EKU OID; a signing cert must carry it. Guards against signing
# with a TLS/server-auth cert that merely matches by subject or thumbprint.
$CodeSigningEku = '1.3.6.1.5.5.7.3.3'
# Well-known code-signing subject used as the last-resort cert lookup.
$CodeSigningSubject = 'CN=Assay local tools'

function Test-CodeSigningCert {
    param($Cert)
    $ekus = $Cert.EnhancedKeyUsageList
    if ($null -eq $ekus -or $ekus.Count -eq 0) { return $false }
    return [bool]($ekus | Where-Object { $_.ObjectId -eq $CodeSigningEku })
}

function Get-CodeSigningSetupHelp {
    # Fail-closed setup snippet printed when -Sign is set but no cert is found.
    # Example subject only; no real cert material. The Trusted-Root import and its
    # trust implications are spelled out because this is the security-sensitive step.
    return @"
-Sign was requested but no usable code-signing certificate was found.
Create a self-signed CODE-SIGNING cert in your own user store, trust it on THIS
machine only, then re-run with -Sign. Example (subject is an example — pick your own):

  # 1. Create a code-signing (NOT TLS) self-signed cert in CurrentUser\My:
  `$cert = New-SelfSignedCertificate ``
    -Subject '$CodeSigningSubject' -Type CodeSigningCert ``
    -CertStoreLocation Cert:\CurrentUser\My ``
    -KeyUsage DigitalSignature -KeyExportPolicy NonExportable

  # 2. Trust it on THIS machine only (per-user CurrentUser stores):
  `$cer = Join-Path `$env:TEMP 'assay-codesign.cer'
  Export-Certificate -Cert `$cert -FilePath `$cer | Out-Null
  Import-Certificate -FilePath `$cer -CertStoreLocation Cert:\CurrentUser\TrustedPublisher | Out-Null
  Import-Certificate -FilePath `$cer -CertStoreLocation Cert:\CurrentUser\Root | Out-Null  # Trusted Root (per-user)
  Remove-Item `$cer

  # 3. Re-run the build, opting in:
  pwsh -File scripts/build-windows.ps1 desk-install -Sign

TRUST IMPLICATIONS — read before importing to Trusted Root:
  * Cert:\CurrentUser\Root makes THIS user trust anything the cert signs as a root
    CA. Keep the key NonExportable, CurrentUser-only, this-machine-only; never move
    it to another machine or the LocalMachine store.
  * A self-signed publisher only NAMES the publisher; it does NOT clear ML/heuristic
    AV verdicts (e.g. Heur.AdvML.D). A per-machine folder exception on the install
    dir is the control that silences that heuristic. See docs/adopting-assay.md,
    "Signing from-source Windows builds".
  * Remove trust later:
      Get-ChildItem Cert:\CurrentUser\Root, Cert:\CurrentUser\TrustedPublisher, Cert:\CurrentUser\My ``
        | Where-Object { `$_.Subject -eq '$CodeSigningSubject' } | Remove-Item
"@
}

function Resolve-CodeSigningCert {
    # Resolve a code-signing cert, in priority order:
    #   1. -CertThumbprint
    #   2. $env:ASSAY_CODESIGN_THUMBPRINT
    #   3. well-known subject $CodeSigningSubject in Cert:\CurrentUser\My
    # Fails closed (throws with the setup snippet) rather than skip-and-ship-unsigned.
    $thumb = $null
    if (-not [string]::IsNullOrWhiteSpace($CertThumbprint)) {
        $thumb = $CertThumbprint
    } elseif (-not [string]::IsNullOrWhiteSpace($env:ASSAY_CODESIGN_THUMBPRINT)) {
        $thumb = $env:ASSAY_CODESIGN_THUMBPRINT
    }

    $cert = $null
    if ($thumb) {
        $thumb = ($thumb -replace '\s', '').ToUpper()
        $cert = Get-ChildItem Cert:\CurrentUser\My |
            Where-Object { $_.Thumbprint -eq $thumb } | Select-Object -First 1
        if ($null -eq $cert) {
            throw "sign: no certificate with thumbprint $thumb in Cert:\CurrentUser\My.`n$(Get-CodeSigningSetupHelp)"
        }
    } else {
        $cert = Get-ChildItem Cert:\CurrentUser\My |
            Where-Object { $_.Subject -eq $CodeSigningSubject } |
            Where-Object { Test-CodeSigningCert $_ } |
            Sort-Object NotAfter -Descending | Select-Object -First 1
        if ($null -eq $cert) {
            throw "sign: no code-signing certificate found (checked -CertThumbprint, `$env:ASSAY_CODESIGN_THUMBPRINT, and subject '$CodeSigningSubject').`n$(Get-CodeSigningSetupHelp)"
        }
    }

    if (-not (Test-CodeSigningCert $cert)) {
        throw "sign: certificate $($cert.Thumbprint) ($($cert.Subject)) is not a code-signing certificate (missing EKU $CodeSigningEku) — refusing to sign with a non-code-signing cert.`n$(Get-CodeSigningSetupHelp)"
    }
    return $cert
}

function Invoke-SignBinaries {
    # Sign every *.exe in $Dir when -Sign is set; a no-op otherwise. Idempotent:
    # re-signing an already-signed PE is fine. Throws on any signing failure so a
    # requested -Sign never silently leaves an unsigned or invalidly-signed PE.
    param([string]$Dir, [string]$Label)
    if (-not $Sign) { return }
    if (-not (Test-Path $Dir)) { return }
    $exes = @(Get-ChildItem -Path $Dir -Filter '*.exe' -File)
    if ($exes.Count -eq 0) {
        Write-Host "sign: no *.exe in $Dir -- nothing to sign"
        return
    }
    $cert = Resolve-CodeSigningCert
    Write-Host "sign: using cert $($cert.Thumbprint) ($($cert.Subject)) for $Label"
    foreach ($exe in $exes) {
        $sigArgs = @{
            FilePath      = $exe.FullName
            Certificate   = $cert
            HashAlgorithm = 'SHA256'
        }
        if (-not [string]::IsNullOrWhiteSpace($TimestampServer)) {
            $sigArgs['TimestampServer'] = $TimestampServer
        }
        $result = Set-AuthenticodeSignature @sigArgs
        if ($result.Status -ne 'Valid') {
            throw "sign: Set-AuthenticodeSignature on $($exe.Name) returned status '$($result.Status)': $($result.StatusMessage)"
        }
        Write-Host "sign: signed $($exe.Name) ($($result.Status))"
    }
}
# ---------------------------------------------------------------------------

function Invoke-Go {
    param([string]$WorkDir, [string[]]$GoArgs)
    Push-Location $WorkDir
    try {
        & go @GoArgs
        if ($LASTEXITCODE -ne 0) { throw "go $($GoArgs -join ' ') failed (exit $LASTEXITCODE) in $WorkDir" }
    } finally {
        Pop-Location
    }
}

function Target-DeskBuild {
    New-Item -ItemType Directory -Force -Path $DistDir | Out-Null
    $cmds = Get-DeskCmds
    if ($cmds.Count -eq 0) {
        Write-Host "desk-build: no tools/desk/cmd/* yet -- nothing to build (ok)"
        return
    }
    $ldflags = Get-BuildStamp
    foreach ($d in $cmds) {
        $name = $d.Name
        Write-Host "desk-build: building $name"
        $out = Join-Path $DistDir "$name.exe"
        # Replace, don't overwrite: recent Go's `go build -o <existing.exe>` refuses
        # to clobber a non-object PE, and a signed PE from a prior -Sign run is
        # exactly that. Remove-then-write keeps a sign -> rebuild -> re-sign loop
        # working. (Unsigned rebuilds are unaffected — removing then writing is
        # equivalent to overwriting when nothing blocks the overwrite.)
        if (Test-Path $out) { Remove-Item -Force $out }
        Invoke-Go -WorkDir $DeskDir -GoArgs @('build', '-ldflags', $ldflags, '-o', $out, "./cmd/$name")
    }
    # Opt-in: sign the freshly built dist PEs. No-op unless -Sign is passed.
    Invoke-SignBinaries -Dir $DistDir -Label 'tools/desk/dist'
}

function Target-DeskInstall {
    Write-Host "desk-install: building (desk-build, per-user, unprivileged)"
    Target-DeskBuild
    Write-Host "desk-install: installing to $InstallDir (per-user; Windows needs no elevation)"
    New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
    $cmds = Get-DeskCmds
    if ($cmds.Count -eq 0) {
        Write-Host "desk-install: no tools/desk/cmd/* yet -- nothing to install (ok)"
    } else {
        foreach ($d in $cmds) {
            $name = $d.Name
            $built = Join-Path $DistDir "$name.exe"
            if (-not (Test-Path $built)) { throw "desk-install: $built missing after desk-build -- aborting" }
            Copy-Item -Force -Path $built -Destination (Join-Path $InstallDir "$name.exe")
            Write-Host "desk-install: installed $(Join-Path $InstallDir "$name.exe")"
        }
    }
    # Opt-in: sign the installed PEs BEFORE desk-manifest hashes them, so
    # MANIFEST.sha256 matches the signed bytes on disk (signing changes the sha256).
    # No-op unless -Sign is passed. Idempotent with the dist signing in desk-build.
    Invoke-SignBinaries -Dir $InstallDir -Label $InstallDir
    Target-DeskHookInstall
    Target-DeskManifest
}

function Target-DeskManifest {
    $binaries = @()
    if (Test-Path $InstallDir) {
        $binaries = @(Get-ChildItem -Path $InstallDir -File | Sort-Object Name)
    }
    if ($binaries.Count -eq 0) {
        Set-Content -Path $Manifest -Value '' -NoNewline
        Write-Host "desk-manifest: no installed binaries; wrote empty $Manifest"
        return
    }
    # shasum-compatible lines: "<sha256(lowercased)>  <filename>".
    $lines = foreach ($b in $binaries) {
        $hash = (Get-FileHash -Algorithm SHA256 -Path $b.FullName).Hash.ToLower()
        "$hash  $($b.Name)"
    }
    Set-Content -Path $Manifest -Value $lines
    Write-Host "desk-manifest: wrote $Manifest"
}

function Target-DeskHookInstall {
    if (-not (Test-Path $HookSrc)) {
        Write-Host "desk-hook-install: $HookSrc not found -- nothing to install (ok)"
        return
    }
    if (Test-Path $HookDst) {
        if (Select-String -Path $HookDst -Pattern 'deskpushguard' -Quiet) {
            Write-Host "desk-hook-install: pre-push hook already installed (idempotent skip)"
            return
        }
        $oldName = (Get-Content -Path $HookDst -TotalCount 1)
        if ($Force) {
            Write-Host "desk-hook-install: -Force: overwriting existing non-deskpushguard pre-push hook: $oldName"
        } else {
            Write-Host "desk-hook-install: refusing to clobber existing non-deskpushguard pre-push hook: $oldName"
            Write-Host "  Use -Force to overwrite, e.g.: pwsh -File scripts/build-windows.ps1 desk-hook-install -Force"
            throw "desk-hook-install: existing foreign pre-push hook"
        }
    }
    New-Item -ItemType Directory -Force -Path (Split-Path $HookDst) | Out-Null
    # Copy the shim verbatim, exactly as the Makefile does -- the installer does
    # not rewrite the shim's interpreter/path. The committed shim is a POSIX
    # `#!/bin/sh` exec of the Unix install path; on Windows it runs under Git for
    # Windows' bundled sh, and adapting its exec target to the Windows install
    # dir is a portability concern owned by windows-port/02, not this build file.
    Copy-Item -Force -Path $HookSrc -Destination $HookDst
    Write-Host "desk-hook-install: installed pre-push hook (deskpushguard)"
}

function Target-DeskTest {
    Invoke-Go -WorkDir $DeskDir -GoArgs @('test', './...', '-count=1')
}

function Target-Skillslint {
    Invoke-Go -WorkDir (Join-Path $RepoRoot 'tools\skillslint') -GoArgs @('run', '.', '--root', '../..')
}

function Target-GuardrailSync {
    Invoke-Go -WorkDir (Join-Path $RepoRoot 'tools\skillslint') -GoArgs @('run', '.', '--root', '../..', '--sync')
}

function Target-PairedVersions {
    Invoke-Go -WorkDir (Join-Path $RepoRoot 'tools\pairedversions') -GoArgs @('run', '.', '--root', '../..')
}

function Assert-MakefileParity {
    # Run the tools/winparity guard as a preflight. It reads BOTH the Makefile's
    # `.PHONY` set and this script's $MakefileParityTargets block and reddens on
    # any difference -- so a drift is caught before any target runs.
    $winparity = Join-Path $RepoRoot 'tools\winparity'
    if (-not (Test-Path $winparity)) {
        throw "parity preflight: tools/winparity not found at $winparity"
    }
    Push-Location $winparity
    try {
        & go run . --root $RepoRoot
        if ($LASTEXITCODE -ne 0) {
            throw "parity preflight FAILED: the Windows target set and the Makefile's .PHONY set disagree (see tools/winparity output above). Reconcile scripts/build-windows.ps1 and the Makefile before building."
        }
    } finally {
        Pop-Location
    }
}

$Dispatch = @{
    'desk-build'        = ${function:Target-DeskBuild}
    'desk-install'      = ${function:Target-DeskInstall}
    'desk-manifest'     = ${function:Target-DeskManifest}
    'desk-hook-install' = ${function:Target-DeskHookInstall}
    'desk-test'         = ${function:Target-DeskTest}
    'skillslint'        = ${function:Target-Skillslint}
    'guardrail-sync'    = ${function:Target-GuardrailSync}
    'paired-versions'   = ${function:Target-PairedVersions}
}

function Show-Help {
    Get-Help $PSCommandPath -Detailed | Out-String | Write-Host
}

if ($Help) { Show-Help; exit 0 }

if ($ListTargets) {
    $MakefileParityTargets | ForEach-Object { Write-Host $_ }
    exit 0
}

if ([string]::IsNullOrWhiteSpace($Target)) {
    Write-Error "no target given. Targets: $($MakefileParityTargets -join ', '). Run -Help for details."
    exit 2
}

if (-not $Dispatch.ContainsKey($Target)) {
    Write-Error "unknown target '$Target'. Targets: $($MakefileParityTargets -join ', ')."
    exit 2
}

# Cross-check the dispatch table against the declared parity set, so a target
# routed here but missing from $MakefileParityTargets (or vice versa) is a local
# error, independent of the winparity guard.
$dispatchKeys = @($Dispatch.Keys | Sort-Object)
$declaredKeys = @($MakefileParityTargets | Sort-Object)
if (@(Compare-Object $dispatchKeys $declaredKeys).Count -ne 0) {
    Write-Error "internal error: dispatch table and \$MakefileParityTargets disagree (dispatch: $($dispatchKeys -join ','); declared: $($declaredKeys -join ','))."
    exit 2
}

if (-not $NoParity) { Assert-MakefileParity }

& $Dispatch[$Target]

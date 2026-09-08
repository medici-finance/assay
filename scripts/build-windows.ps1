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

  Usage:
    pwsh -File scripts/build-windows.ps1 <target> [options]
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
        Invoke-Go -WorkDir $DeskDir -GoArgs @('build', '-ldflags', $ldflags, '-o', $out, "./cmd/$name")
    }
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

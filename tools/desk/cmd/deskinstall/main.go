// Command deskinstall is the Go-native install path for the pinned Assay
// toolchain — the Windows equivalent of the Unix `sudo make desk-install`, and
// (by construction) a cross-platform installer the Unix side can converge on.
//
// It mirrors the Unix acquire→verify→place flow step for step:
//
//	detect platform (windows-amd64 / windows-arm64 via runtime.GOOS/GOARCH)
//	  → resolve the pinned tag + sha256 for that platform from the plugin's
//	    shipped paired-versions.yaml  (a pinned repo tag, NEVER a floating ref)
//	  → download the release asset (statusgen-windows-<arch>.exe and
//	    desk-tools-windows-<arch>.tar.gz) for that tag
//	  → compute sha256 and compare; a mismatch is a hard REFUSE, not a warning,
//	    so no unverified bytes are ever placed
//	  → place the verified binaries on a PATH-resolvable dir.
//
// The sha256-verify-or-refuse step is the single load-bearing control on this
// surface: it is the one thing standing between a substituted release asset and
// a Windows adopter running unverified bytes. It runs FIRST, after the download
// and before ANY placement — a mismatch leaves nothing installed. The value it
// checks against is itself pinned in the reviewed, version-committed
// paired-versions.yaml, so the check and the pin fail for different reasons.
//
// For the very first install (chicken-and-egg: you need a binary to run this
// installer), a ~5-line PowerShell bootstrap — scripts/bootstrap-windows.ps1 —
// fetches only statusgen-windows-<arch>.exe and hash-verifies it before
// executing anything; from there this Go subcommand acquires and verifies the
// rest. The security-critical hash-verify therefore lives in ONE tested Go
// implementation, and PowerShell is confined to a trivial, auditable bootstrap.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// manifestName is the plugin-shipped pin manifest the install tag + expected
// sha256 are resolved FROM. It is a pinned, version-committed artifact — the
// installer never resolves a floating or rolling release ref.
const manifestName = "paired-versions.yaml"

const usage = `deskinstall — Go-native, version-pinned, sha256-verified toolchain installer.

usage:
  deskinstall --manifest <paired-versions.yaml> --dest <dir> [--platform <os-arch>]
  deskinstall --version
  deskinstall -h | --help

It resolves the pinned tag + per-platform sha256 from the manifest (a pinned repo
tag, never a floating ref), downloads the release assets for the detected
platform, VERIFIES their sha256 and REFUSES on any mismatch (nothing is placed),
then installs the verified binaries into --dest.

Windows assets (from the release build matrix):
  statusgen-windows-amd64.exe      / statusgen-windows-arm64.exe
  desk-tools-windows-amd64.tar.gz  / desk-tools-windows-arm64.tar.gz

  --manifest   path to the pin manifest (` + manifestName + `). Required.
  --dest       PATH-resolvable directory the verified binaries are placed in. Required.
  --platform   override auto-detection (e.g. windows-amd64). Default: this host.

exit: 0 installed & verified · 5 refused (hash mismatch, absent pin, or bad input)`

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && (args[0] == "--version" || args[0] == "-version" || args[0] == "version") {
		sha, built := deskkit.Version()
		fmt.Fprintf(stdout, "deskinstall sourceSHA=%s builtAt=%s releaseTag=%s\n", sha, built, deskkit.ReleaseTagOrDev())
		return deskkit.ExitOK
	}
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprintln(stderr, usage)
		if len(args) == 0 {
			return deskkit.ExitRefused
		}
		return deskkit.ExitOK
	}

	var manifest, dest, platform string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--manifest":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "deskinstall: --manifest needs a path")
				return deskkit.ExitRefused
			}
			i++
			manifest = args[i]
		case "--dest":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "deskinstall: --dest needs a directory")
				return deskkit.ExitRefused
			}
			i++
			dest = args[i]
		case "--platform":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "deskinstall: --platform needs an os-arch value")
				return deskkit.ExitRefused
			}
			i++
			platform = args[i]
		default:
			fmt.Fprintf(stderr, "deskinstall: unknown argument %q\n%s\n", args[i], usage)
			return deskkit.ExitRefused
		}
	}
	if manifest == "" || dest == "" {
		fmt.Fprintln(stderr, "deskinstall: --manifest and --dest are both required\n"+usage)
		return deskkit.ExitRefused
	}

	err := Install(Options{
		ManifestPath: manifest,
		DestDir:      dest,
		Platform:     platform,
		Out:          stdout,
	})
	if err != nil {
		fmt.Fprintf(stderr, "deskinstall: REFUSED — %v\n", err)
		return deskkit.ExitRefused
	}
	return deskkit.ExitOK
}

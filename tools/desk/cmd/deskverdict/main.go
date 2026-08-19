// Command deskverdict signs and verifies verdict payloads for the
// verdict-by-issue lane.
//
// A main-side workflow has to act on an issue body it did not write. deskverdict
// gives that body a trust primitive with ZERO new secrets: the drain engine SIGNS
// a canonical serialisation of the verdict payload with the verifier App's
// EXISTING local RSA key (RS256), and the workflow VERIFIES it against the
// verifier PUBLIC key before it trusts a single row. The public key is delivered
// as the ASSAY_VERIFIER_PUBKEY repo/Actions VARIABLE — NEVER a committed file — so
// no key material of any kind lives in the tree; a local adopter self-generates
// their own keypair with `keygen`.
//
//	deskverdict sign   --payload f.json          # read the LOCAL verifier PEM, emit
//	                                             # the signed issue-body block on stdout
//	deskverdict verify --body f.md               # pubkey from --pubkey or
//	                                             # ASSAY_VERIFIER_PUBKEY; 0 verified,
//	                                             # 1 refused, 6 could-not-check
//	deskverdict keygen --priv verifier.pem       # local adopter: fresh keypair; pub
//	                                             # PEM on stdout for the variable
//	deskverdict pubkey --pem f.pem               # derive the PKIX public key from a PEM
//
// deskverdict performs NO GitHub writes and reads no trust roster: sign, keygen and
// pubkey are local crypto, and verify is a pure function of (body, public key) that
// is safe to run in a public CI job. It is therefore deliberately NOT gated behind
// the desk kill-switch — a verify must be usable unconditionally where a trust
// decision is made.
//
// Exit codes:
//
//	sign / keygen / pubkey — deskkit contract: 0 ok · 5 refused · 6 unverifiable.
//	verify — 0 VERIFIED · 1 REFUSED (signature mismatch) · 6 COULD NOT CHECK
//	         (no payload block, no signature trailer, unparseable payload, or no
//	         verifier pubkey configured).
package main

import (
	"fmt"
	"os"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const usage = `deskverdict — sign & verify verdict payloads for the verdict-by-issue lane.

USAGE:
  deskverdict sign   --payload <f.json>
  deskverdict verify --body <f.md> [--pubkey <path>]
  deskverdict keygen --priv <path> [--pub <path>] [--b64]
  deskverdict pubkey --pem <path>
  deskverdict --version

sign     Canonicalise the payload JSON, sign it (RS256) with the LOCAL verifier
         key (--pem, else VERIFIER_PEM, else <config-home>/verifier-app.pem), and
         print the issue-body block (fenced canonical payload + base64 signature
         trailer) on stdout.

verify   Extract the payload + signature from a signed issue body, re-canonicalise
         the payload, and check the signature with the PUBLIC key. THREE states:
           exit 0  VERIFIED       — signature matches the canonical payload
           exit 1  REFUSED        — signature does NOT match (tamper)
           exit 6  COULD NOT CHECK — no payload block / no signature / bad JSON /
                                     no verifier pubkey configured
         The public key resolves from --pubkey <file>, else the
         ASSAY_VERIFIER_PUBKEY variable (a PEM string OR base64-of-PEM). Verify
         takes the PUBLIC key only; it never reads a private key, and it never
         silently passes when no key is configured.

keygen   Generate a fresh RSA keypair for a LOCAL adopter. Writes the private key
         (0600) to --priv and prints the public key PEM on stdout — store that in
         the ASSAY_VERIFIER_PUBKEY repo/Actions variable. --b64 also prints the
         base64-of-PEM form (newline-safe for an Actions variable).

pubkey   Derive the PKIX public-key PEM from a private-key PEM and print it on
         stdout — PUBLIC material, for the ASSAY_VERIFIER_PUBKEY variable.

Exit (sign/keygen/pubkey): 0 ok · 5 refused · 6 unverifiable.`

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 1 && (args[0] == "--version" || args[0] == "-version") {
		sha, built := deskkit.Version()
		fmt.Fprintf(stdout, "deskverdict sourceSHA=%s builtAt=%s releaseTag=%s\n", sha, built, deskkit.ReleaseTagOrDev())
		return deskkit.ExitOK
	}
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprintln(stderr, usage)
		if len(args) == 0 {
			return deskkit.ExitRefused
		}
		return deskkit.ExitOK
	}

	// Running from source (go run / unstamped) is a drift risk — say so, exactly as
	// the sibling tools do. No kill-switch gate: see the package doc.
	deskkit.WarnIfUnpinned(stderr)

	sub, rest := args[0], args[1:]
	switch sub {
	case "sign":
		return cmdSign(rest)
	case "verify":
		return cmdVerify(rest)
	case "keygen":
		return cmdKeygen(rest)
	case "pubkey":
		return cmdPubkey(rest)
	default:
		fmt.Fprintf(stderr, "deskverdict: unknown subcommand %q\n\n%s\n", sub, usage)
		return deskkit.ExitRefused
	}
}

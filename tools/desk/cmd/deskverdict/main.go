// Command deskverdict signs and verifies verdict payloads for the
// verdict-by-issue lane and, as of scan-lane-private/02, the cross-repo
// desk-batched scan-delta lane.
//
// A main-side workflow has to act on an issue body it did not write. deskverdict
// gives that body a trust primitive with ZERO new secrets: the drain engine SIGNS
// a canonical serialisation of the payload with a LOCAL RSA key (RS256), and the
// workflow VERIFIES it against the matching PUBLIC key before it trusts a single
// row. TWO signing ROLES exist, selected with --key on both `sign` and `verify`:
//
//	verifier    (default) verdict-by-issue lane; public key in ASSAY_VERIFIER_PUBKEY
//	issue-loop             cross-repo scan-delta lane; public key in ASSAY_ISSUE_LOOP_PUBKEY
//
// --key selects WHICH role's key is used; it never changes WHERE a key comes
// from. Both roles' public keys are delivered as repo/Actions VARIABLES — NEVER
// a committed file — so no key material of any kind lives in the tree for
// either role; a local adopter self-generates their own keypair with `keygen`.
// The signed block also DECLARES its signing role, and `verify` refuses (exit 1)
// a block whose declared role differs from --key, before any signature
// arithmetic decides the outcome — a second trust layer that still bites when a
// key is mis-provisioned into the wrong role's variable.
//
//	deskverdict sign   --payload f.json [--key verifier|issue-loop]
//	                                             # read the LOCAL private key for
//	                                             # --key's role, emit the signed
//	                                             # issue-body block on stdout
//	deskverdict verify --body f.md [--key verifier|issue-loop] [--pubkey path]
//	                                             # pubkey from --pubkey, else the
//	                                             # --key role's variable; 0 verified,
//	                                             # 1 refused, 6 could-not-check
//	deskverdict keygen --priv key.pem            # local adopter: fresh keypair (any
//	                                             # role); pub PEM on stdout for the
//	                                             # matching role's variable
//	deskverdict pubkey --pem f.pem               # derive the PKIX public key from a PEM
//
// keygen and pubkey are role-agnostic: they generate/derive a plain RSA
// keypair, and the OPERATOR decides which role's variable the resulting public
// key goes into (there is no --key on either — a keypair is not "for" a role
// until it is stored in that role's variable).
//
// deskverdict performs NO GitHub writes and reads no trust roster: sign, keygen and
// pubkey are local crypto, and verify is a pure function of (body, public key, role)
// that is safe to run in a public CI job. It is therefore deliberately NOT gated behind
// the desk kill-switch — a verify must be usable unconditionally where a trust
// decision is made.
//
// Exit codes:
//
//	sign / keygen / pubkey — deskkit contract: 0 ok · 5 refused · 6 unverifiable.
//	         An unrecognized --key role on `sign` is 5 refused.
//	verify — 0 VERIFIED · 1 REFUSED (signature mismatch, OR a declared-role
//	         mismatch against --key) · 6 COULD NOT CHECK (no payload block, no
//	         signature trailer, unparseable payload, an unrecognized --key role,
//	         or no pubkey configured for that role).
package main

import (
	"fmt"
	"os"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const usage = `deskverdict — sign & verify role-keyed verdict payloads (verdict-by-issue and
cross-repo scan-delta lanes).

USAGE:
  deskverdict sign   --payload <f.json> [--key verifier|issue-loop] [--pem <path>]
  deskverdict verify --body <f.md> [--key verifier|issue-loop] [--pubkey <path>]
  deskverdict keygen --priv <path> [--pub <path>] [--b64]
  deskverdict pubkey --pem <path>
  deskverdict --version

sign     Canonicalise the payload JSON, sign it (RS256) with the LOCAL private
         key for the --key role (--pem, else that role's env override
         [VERIFIER_PEM | ISSUE_LOOP_PEM], else <config-home>/<role>-app.pem),
         and print the issue-body block (fenced canonical payload + base64
         signature trailer, DECLARING the signing role) on stdout. --key
         defaults to "verifier". An unrecognized --key role is refused (exit 5)
         and never falls back to verifier.

verify   Extract the payload + signature from a signed issue body, re-canonicalise
         the payload, and check the signature with the PUBLIC key for the --key
         role. THREE states:
           exit 0  VERIFIED       — the block declares --key's role AND the
                                     signature matches the canonical payload
           exit 1  REFUSED        — signature does NOT match (tamper), OR the
                                     block declares a DIFFERENT role than --key
                                     (checked BEFORE any signature arithmetic)
           exit 6  COULD NOT CHECK — no payload block / no signature / bad JSON /
                                     unrecognized --key role / no pubkey
                                     configured for that role
         The public key resolves from --pubkey <file>, else the --key role's
         variable (ASSAY_VERIFIER_PUBKEY | ASSAY_ISSUE_LOOP_PUBKEY; a PEM
         string OR base64-of-PEM). --key defaults to "verifier", so every
         invocation predating role-keying is unaffected. Verify takes the
         PUBLIC key only; it never reads a private key, and it never silently
         passes when no key is configured for the selected role.

keygen   Generate a fresh RSA keypair for a LOCAL adopter — role-agnostic; the
         operator decides which role's variable the public half goes into.
         Writes the private key (0600) to --priv and prints the public key PEM
         on stdout — store that in the matching role's repo/Actions variable
         (ASSAY_VERIFIER_PUBKEY or ASSAY_ISSUE_LOOP_PUBKEY). --b64 also prints
         the base64-of-PEM form (newline-safe for an Actions variable).

pubkey   Derive the PKIX public-key PEM from a private-key PEM and print it on
         stdout — PUBLIC material, role-agnostic, for whichever role's variable
         the caller is provisioning.

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

package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// cmdVerify extracts the payload + signature from a signed issue body, re-canonicalises
// the payload, and checks the signature with the PUBLIC key for the selected
// --key ROLE. Three-state result:
//
//	exit 0  VERIFIED
//	exit 1  REFUSED         (signature does not match the payload — tamper — OR
//	                         the body declares a different role than --key)
//	exit 6  COULD NOT CHECK (no payload block, no signature trailer, unparseable JSON,
//	                         an unreadable/invalid public key, an unrecognized --key
//	                         role, OR no pubkey configured for the role at all)
//
// The public key is resolved WITHOUT any committed key material, in this order:
//
//  1. --pubkey <file>                        explicit path (a local adopter points at their pub.pem)
//  2. the --key role's variable               ASSAY_VERIFIER_PUBKEY (verifier) or
//     ASSAY_ISSUE_LOOP_PUBKEY (issue-loop) —
//     PEM string OR base64-of-PEM
//  3. neither                                 could-not-check (exit 6), NEVER a silent pass
//
// --key defaults to "verifier", so every existing invocation and the landed
// verdict lane are unaffected. An unrecognized --key role is COULD-NOT-CHECK
// (exit 6) and NEVER falls back to verifier.
//
// The wording on stderr distinguishes all three states; the exit code lets a workflow
// gate with a plain `if deskverdict verify …; then trust`, since only 0 is trust.
func cmdVerify(args []string) int {
	fs := flag.NewFlagSet("verify", flag.ContinueOnError)
	fs.SetOutput(stderr)
	bodyPath := fs.String("body", "", "path to the signed issue body (markdown)")
	pubkeyPath := fs.String("pubkey", "", "path to a PUBLIC key PEM (local use); overrides the --key role's variable")
	keyRole := fs.String("key", deskkit.VerdictRoleVerifier, "verify role: "+deskkit.VerdictRoleVerifier+" | "+deskkit.VerdictRoleIssueLoop+" — refuses a block whose declared role differs, before any signature arithmetic")
	if err := fs.Parse(args); err != nil {
		return deskkit.ExitUnverifiable
	}
	if *bodyPath == "" {
		fmt.Fprintln(stderr, "deskverdict verify: --body <f.md> is required")
		return deskkit.ExitUnverifiable
	}
	if !deskkit.ValidVerdictRole(*keyRole) {
		fmt.Fprintf(stderr, "deskverdict verify: could not check: unrecognized --key role %q (want %q or %q)\n", *keyRole, deskkit.VerdictRoleVerifier, deskkit.VerdictRoleIssueLoop)
		return deskkit.ExitUnverifiable
	}

	body, err := os.ReadFile(*bodyPath)
	if err != nil {
		fmt.Fprintf(stderr, "deskverdict verify: could not check: cannot read body %s: %v\n", *bodyPath, err)
		return deskkit.ExitUnverifiable
	}

	pubPEM, src, err := resolvePublicKeyPEM(*keyRole, *pubkeyPath)
	if err != nil {
		fmt.Fprintf(stderr, "deskverdict verify: could not check: %v\n", err)
		return deskkit.ExitUnverifiable
	}
	pub, err := deskkit.ParseRSAPublicKeyPEM(pubPEM)
	if err != nil {
		fmt.Fprintf(stderr, "deskverdict verify: could not check: %v (from %s)\n", err, src)
		return deskkit.ExitUnverifiable
	}

	state, msg := deskkit.VerifyVerdictBodyForRole(string(body), pub, *keyRole)
	switch state {
	case deskkit.VerdictVerified:
		fmt.Fprintln(stderr, "deskverdict verify: "+msg)
		return deskkit.ExitOK
	case deskkit.VerdictRefused:
		fmt.Fprintln(stderr, "deskverdict verify: "+msg)
		return 1
	default:
		fmt.Fprintln(stderr, "deskverdict verify: "+msg)
		return deskkit.ExitUnverifiable
	}
}

// resolvePublicKeyPEM returns role's PUBLIC key as PEM bytes, plus a short
// human label of where it came from, honouring (in order) an explicit
// --pubkey file and the role's repo/Actions variable (deskkit.PubkeyVarForRole).
// It NEVER reads a committed key file: with neither source configured it fails
// closed with a "no <role> pubkey configured" error the caller maps to
// could-not-check (exit 6) — a missing key is never a silent pass. This
// generalises resolveVerifierPubkeyPEM (below) by role (scan-lane-private/02,
// Task 1); callers pass an already-validated role (deskkit.ValidVerdictRole).
func resolvePublicKeyPEM(role, flagPath string) (pemBytes []byte, source string, err error) {
	if flagPath != "" {
		p := expandHome(flagPath)
		b, rerr := os.ReadFile(p)
		if rerr != nil {
			return nil, "", fmt.Errorf("cannot read pubkey %s: %w", p, rerr)
		}
		return b, p, nil
	}
	varName, verr := deskkit.PubkeyVarForRole(role)
	if verr != nil {
		return nil, "", verr
	}
	if v := os.Getenv(varName); v != "" {
		b, derr := deskkit.DecodePubkeyVar(v)
		if derr != nil {
			return nil, "", derr
		}
		return b, "$" + varName, nil
	}
	return nil, "", fmt.Errorf("no %s pubkey configured: pass --pubkey <file> or set %s "+
		"(a PEM string or base64-of-PEM; local adopters self-generate with `deskverdict keygen`)",
		role, varName)
}

// resolveVerifierPubkeyPEM is the VerdictRoleVerifier-role shorthand for
// resolvePublicKeyPEM, kept for symmetry with resolveVerifierPEM (sign.go);
// nothing in this package calls it today, but it documents the pre-Task-1
// resolution order this generalises.
func resolveVerifierPubkeyPEM(flagPath string) (pemBytes []byte, source string, err error) {
	return resolvePublicKeyPEM(deskkit.VerdictRoleVerifier, flagPath)
}

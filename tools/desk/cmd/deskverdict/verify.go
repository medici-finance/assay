package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// cmdVerify extracts the payload + signature from a signed issue body, re-canonicalises
// the payload, and checks the signature with the PUBLIC key. Three-state result:
//
//	exit 0  VERIFIED
//	exit 1  REFUSED         (signature does not match the payload — tamper)
//	exit 6  COULD NOT CHECK (no payload block, no signature trailer, unparseable JSON,
//	                         an unreadable/invalid public key, OR no verifier pubkey
//	                         configured at all)
//
// The public key is resolved WITHOUT any committed key material, in this order:
//
//  1. --pubkey <file>            explicit path (a local adopter points at their pub.pem)
//  2. ASSAY_VERIFIER_PUBKEY      repo/Actions variable — PEM string OR base64-of-PEM
//  3. neither                    could-not-check (exit 6), NEVER a silent pass
//
// The wording on stderr distinguishes all three states; the exit code lets a workflow
// gate with a plain `if deskverdict verify …; then trust`, since only 0 is trust.
func cmdVerify(args []string) int {
	fs := flag.NewFlagSet("verify", flag.ContinueOnError)
	fs.SetOutput(stderr)
	bodyPath := fs.String("body", "", "path to the signed issue body (markdown)")
	pubkeyPath := fs.String("pubkey", "", "path to a PUBLIC key PEM (local use); overrides "+deskkit.VerifierPubkeyVar)
	if err := fs.Parse(args); err != nil {
		return deskkit.ExitUnverifiable
	}
	if *bodyPath == "" {
		fmt.Fprintln(stderr, "deskverdict verify: --body <f.md> is required")
		return deskkit.ExitUnverifiable
	}

	body, err := os.ReadFile(*bodyPath)
	if err != nil {
		fmt.Fprintf(stderr, "deskverdict verify: could not check: cannot read body %s: %v\n", *bodyPath, err)
		return deskkit.ExitUnverifiable
	}

	pubPEM, src, err := resolveVerifierPubkeyPEM(*pubkeyPath)
	if err != nil {
		fmt.Fprintf(stderr, "deskverdict verify: could not check: %v\n", err)
		return deskkit.ExitUnverifiable
	}
	pub, err := deskkit.ParseRSAPublicKeyPEM(pubPEM)
	if err != nil {
		fmt.Fprintf(stderr, "deskverdict verify: could not check: %v (from %s)\n", err, src)
		return deskkit.ExitUnverifiable
	}

	state, msg := deskkit.VerifyVerdictBody(string(body), pub)
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

// resolveVerifierPubkeyPEM returns the verifier PUBLIC key as PEM bytes, plus a
// short human label of where it came from, honouring (in order) an explicit
// --pubkey file and the ASSAY_VERIFIER_PUBKEY repo/Actions variable. It NEVER
// reads a committed key file: with neither source configured it fails closed with
// a "no verifier pubkey configured" error the caller maps to could-not-check
// (exit 6) — a missing key is never a silent pass.
func resolveVerifierPubkeyPEM(flagPath string) (pemBytes []byte, source string, err error) {
	if flagPath != "" {
		p := expandHome(flagPath)
		b, rerr := os.ReadFile(p)
		if rerr != nil {
			return nil, "", fmt.Errorf("cannot read pubkey %s: %w", p, rerr)
		}
		return b, p, nil
	}
	if v := os.Getenv(deskkit.VerifierPubkeyVar); v != "" {
		b, derr := deskkit.DecodePubkeyVar(v)
		if derr != nil {
			return nil, "", derr
		}
		return b, "$" + deskkit.VerifierPubkeyVar, nil
	}
	return nil, "", fmt.Errorf("no verifier pubkey configured: pass --pubkey <file> or set %s "+
		"(a PEM string or base64-of-PEM; local adopters self-generate with `deskverdict keygen`)",
		deskkit.VerifierPubkeyVar)
}

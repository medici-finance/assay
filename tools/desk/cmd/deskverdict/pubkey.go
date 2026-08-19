package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// cmdPubkey derives the PKIX public-key PEM from a private-key PEM and prints it
// on stdout. The output is PUBLIC material by construction — it carries no secret
// — and is what a verifier stores in the ASSAY_VERIFIER_PUBKEY repo/Actions
// variable for `deskverdict verify`, and what `openssl pkey -pubin` consumes.
func cmdPubkey(args []string) int {
	fs := flag.NewFlagSet("pubkey", flag.ContinueOnError)
	fs.SetOutput(stderr)
	pemPath := fs.String("pem", "", "path to the private-key PEM (default: VERIFIER_PEM, else <config-home>/verifier-app.pem)")
	if err := fs.Parse(args); err != nil {
		return deskkit.ExitRefused
	}

	path, err := resolveVerifierPEM(*pemPath)
	if err != nil {
		fmt.Fprintf(stderr, "deskverdict pubkey: %v\n", err)
		return deskkit.ExitUnverifiable
	}
	privPEM, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(stderr, "deskverdict pubkey: cannot read key at %s: %v\n", path, err)
		return deskkit.ExitUnverifiable
	}
	pubPEM, err := deskkit.DeriveRSAPublicKeyPEM(privPEM)
	if err != nil {
		fmt.Fprintf(stderr, "deskverdict pubkey: %v (%s)\n", err, path)
		return deskkit.ExitUnverifiable
	}
	fmt.Fprint(stdout, string(pubPEM))
	return deskkit.ExitOK
}

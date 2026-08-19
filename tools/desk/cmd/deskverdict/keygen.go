package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"flag"
	"fmt"
	"os"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// cmdKeygen generates a fresh RSA keypair for a LOCAL adopter who does not have a
// verifier App key. It writes the PRIVATE key PEM (0600) to --priv — that half is
// the adopter's to guard and is what `deskverdict sign --pem <priv>` reads — and
// emits the PUBLIC key PEM on stdout. The public half carries no secret: the
// adopter stores it in the ASSAY_VERIFIER_PUBKEY repo/Actions variable, which
// `deskverdict verify` reads. No key material is ever committed to the tree.
//
//	deskverdict keygen --priv verifier.pem            # priv -> file, pub PEM -> stdout
//	deskverdict keygen --priv verifier.pem --pub pub.pem
//
// With --b64 the public key is ALSO printed (to stderr) as base64-of-PEM — the
// newline-safe form to paste into an Actions variable.
func cmdKeygen(args []string) int {
	fs := flag.NewFlagSet("keygen", flag.ContinueOnError)
	fs.SetOutput(stderr)
	privPath := fs.String("priv", "", "path to WRITE the generated private-key PEM (required)")
	pubPath := fs.String("pub", "", "optional path to WRITE the public-key PEM (also always printed on stdout)")
	bits := fs.Int("bits", 2048, "RSA modulus size in bits")
	b64 := fs.Bool("b64", false, "also print the public key as base64-of-PEM (for "+deskkit.VerifierPubkeyVar+") on stderr")
	if err := fs.Parse(args); err != nil {
		return deskkit.ExitRefused
	}
	if *privPath == "" {
		fmt.Fprintln(stderr, "deskverdict keygen: --priv <path> is required")
		return deskkit.ExitRefused
	}
	if *bits < 2048 {
		fmt.Fprintf(stderr, "deskverdict keygen: --bits %d too small; RSA keys must be at least 2048 bits\n", *bits)
		return deskkit.ExitRefused
	}

	key, err := rsa.GenerateKey(rand.Reader, *bits)
	if err != nil {
		fmt.Fprintf(stderr, "deskverdict keygen: cannot generate key: %v\n", err)
		return deskkit.ExitUnverifiable
	}

	privPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	if err := os.WriteFile(expandHome(*privPath), privPEM, 0o600); err != nil {
		fmt.Fprintf(stderr, "deskverdict keygen: cannot write private key %s: %v\n", *privPath, err)
		return deskkit.ExitUnverifiable
	}

	pubPEM, err := deskkit.DeriveRSAPublicKeyPEM(privPEM)
	if err != nil {
		fmt.Fprintf(stderr, "deskverdict keygen: %v\n", err)
		return deskkit.ExitUnverifiable
	}
	if *pubPath != "" {
		if err := os.WriteFile(expandHome(*pubPath), pubPEM, 0o644); err != nil {
			fmt.Fprintf(stderr, "deskverdict keygen: cannot write public key %s: %v\n", *pubPath, err)
			return deskkit.ExitUnverifiable
		}
	}

	fmt.Fprint(stdout, string(pubPEM))
	fmt.Fprintf(stderr, "deskverdict keygen: wrote private key to %s (keep it local; 0600).\n", *privPath)
	fmt.Fprintf(stderr, "deskverdict keygen: set %s to the public key above (a PEM string or base64-of-PEM).\n", deskkit.VerifierPubkeyVar)
	if *b64 {
		fmt.Fprintf(stderr, "%s=%s\n", deskkit.VerifierPubkeyVar, base64.StdEncoding.EncodeToString(pubPEM))
	}
	return deskkit.ExitOK
}

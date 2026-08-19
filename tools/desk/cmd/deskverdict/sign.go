package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// cmdSign canonicalises the payload JSON, signs it (RS256) with the LOCAL verifier
// App private key, and prints the issue-body block on stdout.
//
// The private key is resolved EXACTLY as deskevidence resolves it (#794): the
// VERIFIER_PEM env override first, else verifier-app.pem on the App-credential
// search path (deskkit.FindConfigFile / confighome.go). It is never an Actions
// secret and never leaves this machine.
func cmdSign(args []string) int {
	fs := flag.NewFlagSet("sign", flag.ContinueOnError)
	fs.SetOutput(stderr)
	payloadPath := fs.String("payload", "", "path to the verdict payload JSON")
	pemOverride := fs.String("pem", "", "path to the verifier private-key PEM (default: VERIFIER_PEM, else <config-home>/verifier-app.pem)")
	if err := fs.Parse(args); err != nil {
		return deskkit.ExitRefused
	}
	if *payloadPath == "" {
		fmt.Fprintln(stderr, "deskverdict sign: --payload <f.json> is required")
		return deskkit.ExitRefused
	}

	raw, err := os.ReadFile(*payloadPath)
	if err != nil {
		fmt.Fprintf(stderr, "deskverdict sign: cannot read payload %s: %v\n", *payloadPath, err)
		return deskkit.ExitUnverifiable
	}
	canonical, err := deskkit.CanonicalizeJSON(raw)
	if err != nil {
		fmt.Fprintf(stderr, "deskverdict sign: %v\n", err)
		return deskkit.ExitRefused
	}

	pemPath, err := resolveVerifierPEM(*pemOverride)
	if err != nil {
		fmt.Fprintf(stderr, "deskverdict sign: %v\n", err)
		return deskkit.ExitUnverifiable
	}
	keyPEM, err := os.ReadFile(pemPath)
	if err != nil {
		fmt.Fprintf(stderr, "deskverdict sign: cannot read verifier key at %s: %v\n", pemPath, err)
		return deskkit.ExitUnverifiable
	}
	key, err := deskkit.ParseRSAPrivateKeyPEM(keyPEM)
	if err != nil {
		fmt.Fprintf(stderr, "deskverdict sign: %v (%s)\n", err, pemPath)
		return deskkit.ExitUnverifiable
	}

	sig, err := deskkit.SignVerdictCanonical(canonical, key)
	if err != nil {
		fmt.Fprintf(stderr, "deskverdict sign: %v\n", err)
		return deskkit.ExitUnverifiable
	}

	body := deskkit.AssembleVerdictBody(canonical, sig)

	// Default output: the issue-body block on stdout. When --payload is X.json and
	// the block is redirected, callers usually want X.out (Verify #4 reads
	// /tmp/vd.out) — so we ALSO write it there when we can derive the sibling path,
	// mirroring the roundtrip the brief's Verify table exercises.
	fmt.Fprint(stdout, body)
	if out := siblingOutPath(*payloadPath); out != "" {
		if werr := os.WriteFile(out, []byte(body), 0o644); werr != nil {
			fmt.Fprintf(stderr, "deskverdict sign: note: could not also write %s: %v\n", out, werr)
		}
	}
	return deskkit.ExitOK
}

// resolveVerifierPEM returns the path to the verifier private-key PEM, honouring
// (in order) an explicit --pem, the VERIFIER_PEM env override, and finally
// verifier-app.pem on the App-credential search path. Fails closed, naming every
// directory searched.
func resolveVerifierPEM(override string) (string, error) {
	if override != "" {
		return expandHome(override), nil
	}
	if v := strings.TrimSpace(os.Getenv("VERIFIER_PEM")); v != "" {
		return expandHome(v), nil
	}
	path, searched, found := deskkit.FindConfigFile("verifier-app.pem")
	if !found {
		return "", fmt.Errorf("cannot find verifier-app.pem — set VERIFIER_PEM=<file>, "+
			"or place it in one of: %s", strings.Join(searched, ", "))
	}
	return expandHome(path), nil
}

// siblingOutPath maps foo.json -> foo.out, so `sign --payload foo.json` also
// leaves the signed block at foo.out for a subsequent `verify --body foo.out`.
// Returns "" when the payload path has no ".json" suffix (no guessing).
func siblingOutPath(payloadPath string) string {
	if strings.HasSuffix(payloadPath, ".json") {
		return strings.TrimSuffix(payloadPath, ".json") + ".out"
	}
	return ""
}

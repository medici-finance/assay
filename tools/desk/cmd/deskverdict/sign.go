package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// cmdSign canonicalises the payload JSON, signs it (RS256) with the LOCAL
// private key for the selected --key ROLE, and prints the issue-body block on
// stdout.
//
// The private key is resolved EXACTLY as deskevidence resolves the verifier's
// (#794), generalised by role (the house-private brief's Task 1): an explicit
// --pem, else the role's env override (VERIFIER_PEM / ISSUE_LOOP_PEM), else
// <role>-app.pem on the App-credential search path (deskkit.FindConfigFile /
// confighome.go). It is never an Actions secret and never leaves this machine,
// for either role.
func cmdSign(args []string) int {
	fs := flag.NewFlagSet("sign", flag.ContinueOnError)
	fs.SetOutput(stderr)
	payloadPath := fs.String("payload", "", "path to the verdict payload JSON")
	pemOverride := fs.String("pem", "", "path to the signer's private-key PEM (default: the --key role's env override, else <config-home>/<role>-app.pem)")
	keyRole := fs.String("key", deskkit.VerdictRoleVerifier, "signing role: "+deskkit.VerdictRoleVerifier+" | "+deskkit.VerdictRoleIssueLoop+" (selects WHICH role's key signs; never changes WHERE a key comes from)")
	if err := fs.Parse(args); err != nil {
		return deskkit.ExitRefused
	}
	if *payloadPath == "" {
		fmt.Fprintln(stderr, "deskverdict sign: --payload <f.json> is required")
		return deskkit.ExitRefused
	}
	// An unrecognized --key role is REFUSED and NEVER falls back to verifier — a
	// silent fall-back would let an issue-loop artifact be signed (and later
	// trusted) as if it were the verifier's.
	if !deskkit.ValidVerdictRole(*keyRole) {
		fmt.Fprintf(stderr, "deskverdict sign: unrecognized --key role %q (want %q or %q)\n", *keyRole, deskkit.VerdictRoleVerifier, deskkit.VerdictRoleIssueLoop)
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

	pemPath, err := resolveSignerPEM(*keyRole, *pemOverride)
	if err != nil {
		fmt.Fprintf(stderr, "deskverdict sign: %v\n", err)
		return deskkit.ExitUnverifiable
	}
	keyPEM, err := os.ReadFile(pemPath)
	if err != nil {
		fmt.Fprintf(stderr, "deskverdict sign: cannot read %s key at %s: %v\n", *keyRole, pemPath, err)
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

	body := deskkit.AssembleVerdictBodyForRole(canonical, sig, *keyRole)

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

// privKeyEnvForRole and privKeyFileForRole map a verdict ROLE to its LOCAL
// private-key resolution names (the house-private brief's Task 1). Callers must
// have already validated role with deskkit.ValidVerdictRole; an unrecognized
// role falls through to the verifier names here ONLY because both call sites
// (resolveSignerPEM) are reached exclusively after that validation — there is
// no path from an unrecognized --key to a resolved PEM.
func privKeyEnvForRole(role string) string {
	if role == deskkit.VerdictRoleIssueLoop {
		return "ISSUE_LOOP_PEM"
	}
	return "VERIFIER_PEM"
}

func privKeyFileForRole(role string) string {
	if role == deskkit.VerdictRoleIssueLoop {
		return "issue-loop-app.pem"
	}
	return "verifier-app.pem"
}

// resolveSignerPEM returns the path to role's private-key PEM, honouring (in
// order) an explicit --pem, the role's env override, and finally
// <role>-app.pem on the App-credential search path. Fails closed, naming every
// directory searched. This generalises resolveVerifierPEM (below) by role
// (the house-private brief's Task 1); the resolution ORDER is unchanged, only the
// env-var and file names now vary by role.
func resolveSignerPEM(role, override string) (string, error) {
	if override != "" {
		return expandHome(override), nil
	}
	envName := privKeyEnvForRole(role)
	if v := strings.TrimSpace(os.Getenv(envName)); v != "" {
		return expandHome(v), nil
	}
	fileName := privKeyFileForRole(role)
	path, searched, found := deskkit.FindConfigFile(fileName)
	if !found {
		return "", fmt.Errorf("cannot find %s — set %s=<file>, "+
			"or place it in one of: %s", fileName, envName, strings.Join(searched, ", "))
	}
	return expandHome(path), nil
}

// resolveVerifierPEM is the VerdictRoleVerifier-role shorthand for
// resolveSignerPEM, kept so pubkey.go (a local-only tool with no --key of its
// own) is unaffected by Task 1.
func resolveVerifierPEM(override string) (string, error) {
	return resolveSignerPEM(deskkit.VerdictRoleVerifier, override)
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

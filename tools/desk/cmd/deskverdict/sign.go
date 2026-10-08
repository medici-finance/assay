package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

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
//
// File handling (the keyless-compose split's signer half), on EVERY --payload call: on unix the
// payload's parent directory must be a real directory (not a link) owned by the
// signer's effective uid with no group or other write bit; the payload must be a
// regular file, opened without following a link and without blocking on a FIFO,
// and its bytes are read ONCE — the digest, the binding checks and the signature
// all use those bytes. The .out sibling is created exclusively, so an existing
// entry (a leftover file, a link) is never replaced or written through.
//
// Binding flags (--expect-sha256, --expect-repo, --expect-head, --not-before,
// --not-after) are optional; each one passed is checked BEFORE the signer key is
// resolved, and a mismatch is a refusal (exit 5) with nothing on stdout. A flag
// counts as passed when fs.Visit reports it, so an empty value is refused rather
// than switching its check off. The host contract in `verifyloop` usage (H3)
// requires a host signing a fenced runner's payload to pass all five.
func cmdSign(args []string) int {
	fs := flag.NewFlagSet("sign", flag.ContinueOnError)
	fs.SetOutput(stderr)
	payloadPath := fs.String("payload", "", "path to the verdict payload JSON")
	pemOverride := fs.String("pem", "", "path to the signer's private-key PEM (default: the --key role's env override, else <config-home>/<role>-app.pem)")
	keyRole := fs.String("key", deskkit.VerdictRoleVerifier, "signing role: "+deskkit.VerdictRoleVerifier+" | "+deskkit.VerdictRoleIssueLoop+" (selects WHICH role's key signs; never changes WHERE a key comes from)")
	expectSHA := fs.String("expect-sha256", "", "refuse unless the payload file's bytes have this sha256 (64 hex), the digest the composer printed")
	expectRepo := fs.String("expect-repo", "", "refuse unless the payload's top-level repo equals this owner/name, from the host's own dispatch record")
	expectHead := fs.String("expect-head", "", "refuse unless the payload's top-level head equals this sha, from the host's own dispatch record")
	notBefore := fs.String("not-before", "", "refuse unless the payload's top-level ts (RFC3339) is not earlier than this RFC3339 time")
	notAfter := fs.String("not-after", "", "refuse unless the payload's top-level ts (RFC3339) is not later than this RFC3339 time")
	if err := fs.Parse(args); err != nil {
		return deskkit.ExitRefused
	}
	passed := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { passed[f.Name] = true })
	bind, err := parseSignBinding(passed, *expectSHA, *expectRepo, *expectHead, *notBefore, *notAfter)
	if err != nil {
		fmt.Fprintf(stderr, "deskverdict sign: %v\n", err)
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

	raw, code := readPayloadNoFollow(*payloadPath)
	if code != deskkit.ExitOK {
		return code
	}
	canonical, err := deskkit.CanonicalizeJSON(raw)
	if err != nil {
		fmt.Fprintf(stderr, "deskverdict sign: %v\n", err)
		return deskkit.ExitRefused
	}
	if err := bind.check(raw); err != nil {
		fmt.Fprintf(stderr, "deskverdict sign: %s: %v — refusing before any key is resolved; nothing signed\n", *payloadPath, err)
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
		// O_CREATE|O_EXCL fails on ANY existing entry — a leftover file or a link,
		// dangling or not — so the sibling is never replaced or written through.
		f, oerr := os.OpenFile(out, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if oerr != nil {
			fmt.Fprintf(stderr, "deskverdict sign: cannot create %s exclusively (%v) — the existing entry is untouched; the signed body is on stdout only\n", out, oerr)
			return deskkit.ExitRefused
		}
		_, werr := io.WriteString(f, body)
		if cerr := f.Close(); werr == nil {
			werr = cerr
		}
		if werr != nil {
			fmt.Fprintf(stderr, "deskverdict sign: writing %s failed (%v) — it may be incomplete; the signed body is on stdout only\n", out, werr)
			return deskkit.ExitRefused
		}
	}
	return deskkit.ExitOK
}

// afterPayloadLstat, nil in production, runs between the payload's Lstat and its
// open, so a test can swap the entry in that window and prove the open does not
// trust the Lstat.
var afterPayloadLstat func(path string)

// readPayloadNoFollow reads the --payload file once, refusing (exit 5) anything
// but a regular file reached without following a link, in a parent directory
// that passes checkPayloadDir. It returns deskkit.ExitOK with the bytes, or the
// exit code to return (the message already printed). Any other open or read
// failure stays exit 6, as before.
func readPayloadNoFollow(path string) ([]byte, int) {
	if err := checkPayloadDir(filepath.Dir(path)); err != nil {
		fmt.Fprintf(stderr, "deskverdict sign: refusing payload %s: %v\n", path, err)
		return nil, deskkit.ExitRefused
	}
	before, err := os.Lstat(path)
	if err != nil {
		fmt.Fprintf(stderr, "deskverdict sign: cannot read payload %s: %v\n", path, err)
		return nil, deskkit.ExitUnverifiable
	}
	if !before.Mode().IsRegular() {
		fmt.Fprintf(stderr, "deskverdict sign: refusing payload %s: not a regular file (%s) — links, FIFOs, devices and directories are never read\n", path, before.Mode().Type())
		return nil, deskkit.ExitRefused
	}
	if afterPayloadLstat != nil {
		afterPayloadLstat(path)
	}
	f, err := openPayloadFile(path)
	if err != nil {
		if errors.Is(err, errPayloadIsLink) {
			fmt.Fprintf(stderr, "deskverdict sign: refusing payload %s: it became a link after it was checked (%v)\n", path, err)
			return nil, deskkit.ExitRefused
		}
		fmt.Fprintf(stderr, "deskverdict sign: cannot read payload %s: %v\n", path, err)
		return nil, deskkit.ExitUnverifiable
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil {
		fmt.Fprintf(stderr, "deskverdict sign: cannot read payload %s: %v\n", path, err)
		return nil, deskkit.ExitUnverifiable
	}
	if !opened.Mode().IsRegular() || !os.SameFile(before, opened) {
		fmt.Fprintf(stderr, "deskverdict sign: refusing payload %s: the opened entry is not the regular file that was checked\n", path)
		return nil, deskkit.ExitRefused
	}
	raw, err := io.ReadAll(f)
	if err != nil {
		fmt.Fprintf(stderr, "deskverdict sign: cannot read payload %s: %v\n", path, err)
		return nil, deskkit.ExitUnverifiable
	}
	return raw, deskkit.ExitOK
}

// signBinding holds the binding flags that were passed (a nil/empty field means
// not passed). Every value comes from the host's own dispatch record except the
// digest, which comes from the composer's output line (host contract H3).
type signBinding struct {
	sha256     string
	repo, head *string
	notBefore  *time.Time
	notAfter   *time.Time
}

// parseSignBinding validates the binding flags that were passed. A flag passed
// with an empty or unparseable value is an error, never a disabled check.
func parseSignBinding(passed map[string]bool, sha, repo, head, notBefore, notAfter string) (signBinding, error) {
	var b signBinding
	if passed["expect-sha256"] {
		if len(sha) != sha256.Size*2 {
			return b, fmt.Errorf("--expect-sha256 %q is not a 64-hex sha256", sha)
		}
		if _, err := hex.DecodeString(sha); err != nil {
			return b, fmt.Errorf("--expect-sha256 %q is not a 64-hex sha256", sha)
		}
		b.sha256 = strings.ToLower(sha)
	}
	if passed["expect-repo"] {
		if repo == "" {
			return b, errors.New("--expect-repo was passed empty")
		}
		b.repo = &repo
	}
	if passed["expect-head"] {
		if head == "" {
			return b, errors.New("--expect-head was passed empty")
		}
		b.head = &head
	}
	for _, tf := range []struct {
		name, val string
		dst       **time.Time
	}{{"not-before", notBefore, &b.notBefore}, {"not-after", notAfter, &b.notAfter}} {
		if !passed[tf.name] {
			continue
		}
		t, err := time.Parse(time.RFC3339, tf.val)
		if err != nil {
			return b, fmt.Errorf("--%s %q is not an RFC3339 time", tf.name, tf.val)
		}
		*tf.dst = &t
	}
	return b, nil
}

// check applies every passed binding to the payload bytes read once from the
// file. The digest is over those exact bytes; the field checks read the
// payload's TOP-LEVEL repo, head and ts.
func (b signBinding) check(raw []byte) error {
	if b.sha256 != "" {
		sum := sha256.Sum256(raw)
		if got := hex.EncodeToString(sum[:]); got != b.sha256 {
			return fmt.Errorf("sha256 %s does not match --expect-sha256 %s", got, b.sha256)
		}
	}
	if b.repo == nil && b.head == nil && b.notBefore == nil && b.notAfter == nil {
		return nil
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil || top == nil {
		return errors.New("payload is not a JSON object, so its repo/head/ts cannot be checked")
	}
	str := func(key string) (string, error) {
		var v string
		r, ok := top[key]
		if !ok {
			return "", fmt.Errorf("payload has no top-level %q", key)
		}
		if err := json.Unmarshal(r, &v); err != nil {
			return "", fmt.Errorf("payload's top-level %q is not a string", key)
		}
		return v, nil
	}
	for _, f := range []struct {
		key, flag string
		want      *string
	}{{"repo", "--expect-repo", b.repo}, {"head", "--expect-head", b.head}} {
		if f.want == nil {
			continue
		}
		got, err := str(f.key)
		if err != nil {
			return err
		}
		if got != *f.want {
			return fmt.Errorf("payload %s %q does not match %s %q", f.key, got, f.flag, *f.want)
		}
	}
	if b.notBefore == nil && b.notAfter == nil {
		return nil
	}
	tsText, err := str("ts")
	if err != nil {
		return err
	}
	ts, err := time.Parse(time.RFC3339, tsText)
	if err != nil {
		return fmt.Errorf("payload ts %q is not an RFC3339 time", tsText)
	}
	if b.notBefore != nil && ts.Before(*b.notBefore) {
		return fmt.Errorf("payload ts %s is earlier than --not-before %s", tsText, b.notBefore.Format(time.RFC3339))
	}
	if b.notAfter != nil && ts.After(*b.notAfter) {
		return fmt.Errorf("payload ts %s is later than --not-after %s", tsText, b.notAfter.Format(time.RFC3339))
	}
	return nil
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

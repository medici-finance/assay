package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// Brief desk-tools/29 — `verifyloop verdict --unsigned-out <file>` composes the verdict-v1
// payload with no key. Rows 1, 3 and 4 live here; row 2 (FIFO canaries) is in
// verdictunsigned_unix_test.go.

const (
	exRepo = "example-org/example-repo"
	exHead = "3f9c2a1b7d4e6f8091a2b3c4d5e6f708192a3b4c"
)

var sha256Line = regexp.MustCompile(`sha256=([0-9a-f]{64})`)

// noKeyEnv pins an environment in which the resolver can find no key at all: an empty HOME,
// and VERIFIER_PEM / ASSAY_CONFIG_HOME set empty.
func noKeyEnv(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("VERIFIER_PEM", "")
	t.Setenv("ASSAY_CONFIG_HOME", "")
}

// privDir is a fresh t.TempDir chmodded to 0700, so a payload in it passes the signer's unix
// parent-directory check whatever the umask is.
func privDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	return dir
}

// Row 1.
func TestUnsignedOutNeverResolvesVerifierKey(t *testing.T) {
	noKeyEnv(t)
	root := demoRoot(t)

	// Arm A: no key anywhere, the unsigned run succeeds and writes every executed row.
	outPath := filepath.Join(privDir(t), "payload.json")
	var out bytes.Buffer
	err := runVerdict(verdictRunConfig{root: root, repo: exRepo, head: exHead,
		exec: fakeExec, out: &out, unsignedOut: outPath})
	if err != nil {
		t.Fatalf("arm A: unsigned run with no key anywhere = %v, want nil", err)
	}
	raw, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("arm A: payload file not written: %v", err)
	}
	var p verdictPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatalf("arm A: payload does not parse as verdictPayload: %v\n%s", err, raw)
	}
	if p.Schema != deskkit.VerdictSchemaVersion || len(p.Entries) != 2 {
		t.Fatalf("arm A: payload schema=%q entries=%d, want %q and 2 (both runner rows)", p.Schema, len(p.Entries), deskkit.VerdictSchemaVersion)
	}

	// CONTROL: the signed path in the SAME environment finds no key — so arm A is not vacuous.
	var ctl bytes.Buffer
	err = runVerdict(verdictRunConfig{root: root, repo: exRepo, head: exHead,
		dryRun: true, window: time.Hour, exec: fakeExec, out: &ctl})
	if err == nil || deskkit.ExitCodeOf(err) != deskkit.ExitUnverifiable ||
		!strings.Contains(err.Error(), "cannot find the verifier private key") {
		t.Fatalf("control: signed path in the no-key env = %v (exit %d), want exit 6 naming the missing key", err, deskkit.ExitCodeOf(err))
	}

	// Arm B: a REAL key is reachable through VERIFIER_PEM, and still nothing is signed.
	pemPath, _ := writeTestKey(t)
	t.Setenv("VERIFIER_PEM", pemPath)
	outB := filepath.Join(privDir(t), "payload.json")
	var outBuf bytes.Buffer
	if err := runVerdict(verdictRunConfig{root: root, repo: exRepo, head: exHead,
		exec: fakeExec, out: &outBuf, unsignedOut: outB}); err != nil {
		t.Fatalf("arm B: %v", err)
	}
	rawB, err := os.ReadFile(outB)
	if err != nil {
		t.Fatalf("arm B: payload file not written: %v", err)
	}
	for _, where := range []struct{ name, text string }{{"stdout", outBuf.String()}, {"file", string(rawB)}} {
		for _, bad := range []string{verdictFenceTagLiteral, "deskverdict-signature"} {
			if strings.Contains(where.text, bad) {
				t.Fatalf("arm B: unsigned run's %s carries %q — an unsigned run must never emit a body or signature:\n%s", where.name, bad, where.text)
			}
		}
	}
}

// buildDeskverdict builds the sibling signer into a temp dir (precedent: deskpr_test.go).
func buildDeskverdict(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "deskverdict")
	cmd := exec.Command("go", "build", "-o", bin, "../deskverdict")
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build deskverdict: %v\n%s", err, b)
	}
	return bin
}

func runBin(t *testing.T, bin string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	var ob, eb bytes.Buffer
	cmd := exec.Command(bin, args...)
	cmd.Stdout, cmd.Stderr = &ob, &eb
	err := cmd.Run()
	var ee *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &ee):
		code = ee.ExitCode()
	default:
		t.Fatalf("run %s: %v", bin, err)
	}
	return ob.String(), eb.String(), code
}

// Row 3.
func TestUnsignedComposeSignedByDeskverdictMatchesCombined(t *testing.T) {
	now := time.Date(2026, 10, 8, 3, 0, 0, 0, time.UTC)
	fixedNow := func() time.Time { return now }
	root := demoRoot(t)
	pemPath, _ := writeTestKey(t)
	base := verdictRunConfig{root: root, repo: exRepo, head: exHead, session: "s-29",
		runner: "verify-desk-engine", exec: fakeExec, now: fixedNow}

	// (a) the combined path, one flush.
	var a bytes.Buffer
	ca := base
	ca.pem, ca.dryRun, ca.window, ca.out = pemPath, true, time.Hour, &a
	if err := runVerdict(ca); err != nil {
		t.Fatalf("(a) combined dry-run: %v", err)
	}
	i := strings.Index(a.String(), "\nsigned verdict ")
	if i < 0 || strings.Count(a.String(), verdictFenceTagLiteral) != 1 {
		t.Fatalf("(a) expected exactly one flushed body; got:\n%s", a.String())
	}
	bodyA := a.String()[:i]

	// (b) the unsigned path, same inputs.
	dir := privDir(t)
	payloadPath := filepath.Join(dir, "payload.json")
	var b bytes.Buffer
	cb := base
	cb.unsignedOut, cb.out = payloadPath, &b
	if err := runVerdict(cb); err != nil {
		t.Fatalf("(b) unsigned: %v", err)
	}
	fileBytes, err := os.ReadFile(payloadPath)
	if err != nil {
		t.Fatal(err)
	}

	// (c) the file IS the combined path's canonical form, and the printed digest is its sha256.
	open := verdictFenceTagLiteral + "\n"
	j := strings.Index(bodyA, open)
	k := strings.Index(bodyA, "\n```\n")
	if j < 0 || k < j {
		t.Fatalf("(c) cannot find the fenced payload in (a):\n%s", bodyA)
	}
	fenced := bodyA[j+len(open) : k]
	if string(fileBytes) != fenced+"\n" {
		t.Fatalf("(c) file is not the combined path's canonical payload plus newline:\nfile:   %q\nfenced: %q", fileBytes, fenced)
	}
	m := sha256Line.FindAllStringSubmatch(b.String(), -1)
	sum := sha256.Sum256(fileBytes)
	if len(m) != 1 || m[0][1] != hex.EncodeToString(sum[:]) {
		t.Fatalf("(c) printed digest %v != sha256(file) %x; stdout:\n%s", m, sum, b.String())
	}
	digest := m[0][1]

	// (d) the host signs the file with all five binding flags: byte-identical to (a).
	bin := buildDeskverdict(t)
	signed, serr, code := runBin(t, bin, "sign", "--payload", payloadPath, "--pem", pemPath,
		"--expect-sha256", digest, "--expect-repo", exRepo, "--expect-head", exHead,
		"--not-before", now.Add(-time.Minute).Format(time.RFC3339),
		"--not-after", now.Add(time.Minute).Format(time.RFC3339))
	if code != 0 {
		t.Fatalf("(d) deskverdict sign exit %d: %s", code, serr)
	}
	if !bytes.Equal([]byte(signed), []byte(bodyA)) {
		t.Fatalf("(d) host-signed body is not byte-identical to the combined body:\n--- combined\n%s\n--- host-signed\n%s", bodyA, signed)
	}

	// (e) the bare payload is refused on STRUCTURE (a public key is supplied).
	pubOut, perr, code := runBin(t, bin, "pubkey", "--pem", pemPath)
	if code != 0 {
		t.Fatalf("(e) pubkey exit %d: %s", code, perr)
	}
	pubPath := filepath.Join(t.TempDir(), "verifier.pub.pem")
	if err := os.WriteFile(pubPath, []byte(pubOut), 0o644); err != nil {
		t.Fatal(err)
	}
	_, verr, code := runBin(t, bin, "verify", "--body", payloadPath, "--pubkey", pubPath)
	if code != 6 || !strings.Contains(verr, "verdict-payload block found") {
		t.Fatalf("(e) verify on the bare payload = exit %d (%s), want 6 with the structural refusal", code, verr)
	}

	// (f) the host-signed body verifies.
	signedPath := filepath.Join(t.TempDir(), "signed.md")
	if err := os.WriteFile(signedPath, []byte(signed), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, verr, code := runBin(t, bin, "verify", "--body", signedPath, "--pubkey", pubPath); code != 0 {
		t.Fatalf("(f) verify on the host-signed body = exit %d (%s), want 0", code, verr)
	}
}

// recExec records every row command and delegates to fakeExec, optionally running a side
// effect first (as a hostile row would).
type recExec struct {
	calls  int
	before func()
}

func (r *recExec) run(root, command string) (int, string) {
	r.calls++
	if r.before != nil {
		r.before()
	}
	return fakeExec(root, command)
}

// Row 4.
func TestUnsignedOutRefusals(t *testing.T) {
	noKeyEnv(t)

	for _, args := range [][]string{
		{"--unsigned-out", "f", "--pem", "k"},
		{"--unsigned-out", "f", "--dry-run"},
		{"--unsigned-out", "f", "--window", "1m"},
	} {
		if _, err := parseVerdictFlags(args); deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
			t.Fatalf("parseVerdictFlags(%v) = %v, want exit 5", args, err)
		}
	}

	noDigest := func(t *testing.T, out string) {
		t.Helper()
		if strings.Contains(out, "sha256=") {
			t.Fatalf("a non-success run printed a digest:\n%s", out)
		}
	}

	t.Run("i-existing-path", func(t *testing.T) {
		path := filepath.Join(privDir(t), "payload.json")
		if err := os.WriteFile(path, []byte("planted-i"), 0o644); err != nil {
			t.Fatal(err)
		}
		rec := &recExec{}
		var out bytes.Buffer
		err := runVerdict(verdictRunConfig{root: demoRoot(t), repo: exRepo, head: exHead,
			exec: rec.run, out: &out, unsignedOut: path})
		if deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
			t.Fatalf("existing path = %v, want exit 5", err)
		}
		if rec.calls != 0 {
			t.Fatalf("%d row(s) ran before the existing-path refusal, want 0", rec.calls)
		}
		if got, _ := os.ReadFile(path); string(got) != "planted-i" {
			t.Fatalf("existing entry changed: %q", got)
		}
		noDigest(t, out.String())
	})

	t.Run("ii-row-plants-path", func(t *testing.T) {
		path := filepath.Join(privDir(t), "payload.json")
		rec := &recExec{before: func() {
			if _, err := os.Lstat(path); err != nil {
				_ = os.WriteFile(path, []byte("planted-ii"), 0o644)
			}
		}}
		var out bytes.Buffer
		err := runVerdict(verdictRunConfig{root: demoRoot(t), repo: exRepo, head: exHead,
			exec: rec.run, out: &out, unsignedOut: path})
		if deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
			t.Fatalf("path planted during the run = %v, want exit 5", err)
		}
		if got, _ := os.ReadFile(path); string(got) != "planted-ii" {
			t.Fatalf("planted bytes changed: %q", got)
		}
		noDigest(t, out.String())
	})

	t.Run("iii-write-fails", func(t *testing.T) {
		path := filepath.Join(privDir(t), "payload.json")
		var out bytes.Buffer
		err := runVerdict(verdictRunConfig{root: demoRoot(t), repo: exRepo, head: exHead,
			exec: fakeExec, out: &out, unsignedOut: path,
			writeHook: func(f *os.File, b []byte) error {
				_, _ = f.Write(b[:len(b)/2])
				return errors.New("injected write failure")
			}})
		if err == nil || deskkit.ExitCodeOf(err) == deskkit.ExitOK {
			t.Fatalf("write failure = %v, want a non-zero exit", err)
		}
		if _, lerr := os.Lstat(path); !errors.Is(lerr, os.ErrNotExist) {
			t.Fatalf("partial file left behind after a write failure (lstat err %v)", lerr)
		}
		noDigest(t, out.String())
	})

	t.Run("iv-empty-queue", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, "docs", "streams"), 0o755); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(privDir(t), "payload.json")
		var out bytes.Buffer
		err := runVerdict(verdictRunConfig{root: root, repo: exRepo, head: exHead,
			exec: fakeExec, out: &out, unsignedOut: path})
		if err != nil {
			t.Fatalf("empty queue = %v, want nil", err)
		}
		if !strings.Contains(out.String(), "nothing to compose") {
			t.Fatalf("empty queue output: %q", out.String())
		}
		if _, lerr := os.Lstat(path); !errors.Is(lerr, os.ErrNotExist) {
			t.Fatalf("empty queue wrote a file (lstat err %v)", lerr)
		}
		noDigest(t, out.String())
	})

	t.Run("v-success-line-digest-only", func(t *testing.T) {
		path := filepath.Join(privDir(t), "payload.json")
		var out bytes.Buffer
		if err := runVerdict(verdictRunConfig{root: demoRoot(t), repo: exRepo, head: exHead,
			exec: fakeExec, out: &out, unsignedOut: path}); err != nil {
			t.Fatal(err)
		}
		s := out.String()
		if strings.Count(s, "\n") != 1 || !strings.HasSuffix(s, "\n") {
			t.Fatalf("success output is not exactly one line:\n%s", s)
		}
		if strings.Count(s, "sha256=") != 1 {
			t.Fatalf("success line must carry sha256= exactly once: %s", s)
		}
		for _, bad := range []string{exRepo, exHead, "--expect-repo", "--expect-head"} {
			if strings.Contains(s, bad) {
				t.Fatalf("success line carries %q — it must print the digest only: %s", bad, s)
			}
		}
	})
}

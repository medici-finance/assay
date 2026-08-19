package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// capture swaps stdout/stderr for buffers around fn and returns what was written.
func capture(fn func() int) (out, errOut string, code int) {
	var ob, eb bytes.Buffer
	oldOut, oldErr := stdout, stderr
	stdout, stderr = &ob, &eb
	defer func() { stdout, stderr = oldOut, oldErr }()
	code = fn()
	return ob.String(), eb.String(), code
}

func writePrivPEM(t *testing.T, dir string) (string, *rsa.PrivateKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der := x509.MarshalPKCS1PrivateKey(key)
	pth := filepath.Join(dir, "verifier-app.pem")
	if err := os.WriteFile(pth, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	return pth, key
}

func TestCLIRoundtrip(t *testing.T) {
	dir := t.TempDir()
	privPath, _ := writePrivPEM(t, dir)

	// Derive the pubkey via the CLI and write it out.
	pubOut, _, code := capture(func() int { return cmdPubkey([]string{"--pem", privPath}) })
	if code != 0 {
		t.Fatalf("pubkey exit %d", code)
	}
	if !strings.Contains(pubOut, "BEGIN PUBLIC KEY") {
		t.Fatalf("pubkey output not a PKIX PEM:\n%s", pubOut)
	}
	pubPath := filepath.Join(dir, "pub.pem")
	if err := os.WriteFile(pubPath, []byte(pubOut), 0o644); err != nil {
		t.Fatal(err)
	}

	// Sign a payload.
	payloadPath := filepath.Join(dir, "vd.json")
	if err := os.WriteFile(payloadPath, []byte(`{"schema":"verdict-v1","repo":"medici-finance/assay","entries":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	signOut, _, code := capture(func() int { return cmdSign([]string{"--payload", payloadPath, "--pem", privPath}) })
	if code != 0 {
		t.Fatalf("sign exit %d", code)
	}
	if !strings.Contains(signOut, "verdict-payload") || !strings.Contains(signOut, "deskverdict-signature") {
		t.Fatalf("sign output missing block markers:\n%s", signOut)
	}
	// sign also wrote vd.out (sibling path).
	bodyPath := filepath.Join(dir, "vd.out")
	if _, err := os.Stat(bodyPath); err != nil {
		t.Fatalf("sign did not write sibling %s: %v", bodyPath, err)
	}

	// Verify: exit 0.
	_, vErr, code := capture(func() int { return cmdVerify([]string{"--body", bodyPath, "--pubkey", pubPath}) })
	if code != 0 {
		t.Fatalf("verify exit %d, stderr=%s", code, vErr)
	}
	if !strings.Contains(vErr, "verified") {
		t.Fatalf("verify stderr missing 'verified': %s", vErr)
	}
}

func TestCLIVerifyTamperExit1(t *testing.T) {
	dir := t.TempDir()
	privPath, _ := writePrivPEM(t, dir)

	pubOut, _, _ := capture(func() int { return cmdPubkey([]string{"--pem", privPath}) })
	pubPath := filepath.Join(dir, "pub.pem")
	os.WriteFile(pubPath, []byte(pubOut), 0o644)

	payloadPath := filepath.Join(dir, "vd.json")
	os.WriteFile(payloadPath, []byte(`{"pass":true}`), 0o644)
	capture(func() int { return cmdSign([]string{"--payload", payloadPath, "--pem", privPath}) })

	bodyPath := filepath.Join(dir, "vd.out")
	body, _ := os.ReadFile(bodyPath)
	tampered := strings.Replace(string(body), `"pass":true`, `"pass":false`, 1)
	tPath := filepath.Join(dir, "tampered.out")
	os.WriteFile(tPath, []byte(tampered), 0o644)

	_, vErr, code := capture(func() int { return cmdVerify([]string{"--body", tPath, "--pubkey", pubPath}) })
	if code != 1 {
		t.Fatalf("tampered verify should exit 1, got %d (%s)", code, vErr)
	}
	if !strings.Contains(vErr, "refused") {
		t.Fatalf("tampered verify stderr missing 'refused': %s", vErr)
	}
}

func TestCLIVerifyCouldNotCheckExit6(t *testing.T) {
	dir := t.TempDir()
	privPath, _ := writePrivPEM(t, dir)
	pubOut, _, _ := capture(func() int { return cmdPubkey([]string{"--pem", privPath}) })
	pubPath := filepath.Join(dir, "pub.pem")
	os.WriteFile(pubPath, []byte(pubOut), 0o644)

	bodyPath := filepath.Join(dir, "noblock.md")
	os.WriteFile(bodyPath, []byte("just prose, no verdict block here"), 0o644)

	_, vErr, code := capture(func() int { return cmdVerify([]string{"--body", bodyPath, "--pubkey", pubPath}) })
	if code != 6 {
		t.Fatalf("missing-block verify should exit 6, got %d (%s)", code, vErr)
	}
	if !strings.Contains(vErr, "could not check") {
		t.Fatalf("stderr missing 'could not check': %s", vErr)
	}
}

// signBodyWithNewKey generates an ephemeral keypair, signs a payload, and returns
// the signed body path plus the public-key PEM bytes — the fixture the env-var
// verify tests need WITHOUT any committed key material.
func signBodyWithNewKey(t *testing.T, dir string) (bodyPath string, pubPEM []byte) {
	t.Helper()
	privPath, _ := writePrivPEM(t, dir)
	pubOut, _, code := capture(func() int { return cmdPubkey([]string{"--pem", privPath}) })
	if code != 0 {
		t.Fatalf("pubkey exit %d", code)
	}
	payloadPath := filepath.Join(dir, "vd.json")
	if err := os.WriteFile(payloadPath, []byte(`{"schema":"verdict-v1","entries":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, code := capture(func() int { return cmdSign([]string{"--payload", payloadPath, "--pem", privPath}) }); code != 0 {
		t.Fatalf("sign exit %d", code)
	}
	return filepath.Join(dir, "vd.out"), []byte(pubOut)
}

// TestCLIVerifyFromEnvVarPEM: with no --pubkey, verify reads ASSAY_VERIFIER_PUBKEY
// as a literal PEM string. No committed key file is involved.
func TestCLIVerifyFromEnvVarPEM(t *testing.T) {
	dir := t.TempDir()
	bodyPath, pubPEM := signBodyWithNewKey(t, dir)

	t.Setenv(deskkit.VerifierPubkeyVar, string(pubPEM))
	_, vErr, code := capture(func() int { return cmdVerify([]string{"--body", bodyPath}) })
	if code != 0 {
		t.Fatalf("verify via env PEM should exit 0, got %d (%s)", code, vErr)
	}
	if !strings.Contains(vErr, "verified") {
		t.Fatalf("verify stderr missing 'verified': %s", vErr)
	}
}

// TestCLIVerifyFromEnvVarBase64: the variable may also hold base64-of-PEM, the
// newline-safe Actions-variable form.
func TestCLIVerifyFromEnvVarBase64(t *testing.T) {
	dir := t.TempDir()
	bodyPath, pubPEM := signBodyWithNewKey(t, dir)

	t.Setenv(deskkit.VerifierPubkeyVar, base64.StdEncoding.EncodeToString(pubPEM))
	_, vErr, code := capture(func() int { return cmdVerify([]string{"--body", bodyPath}) })
	if code != 0 {
		t.Fatalf("verify via env base64 should exit 0, got %d (%s)", code, vErr)
	}
	if !strings.Contains(vErr, "verified") {
		t.Fatalf("verify stderr missing 'verified': %s", vErr)
	}
}

// TestCLIVerifyNoPubkeyConfigured: neither --pubkey nor the variable is set, so
// verify must COULD-NOT-CHECK (exit 6) — never a silent pass.
func TestCLIVerifyNoPubkeyConfigured(t *testing.T) {
	dir := t.TempDir()
	bodyPath, _ := signBodyWithNewKey(t, dir)

	t.Setenv(deskkit.VerifierPubkeyVar, "")
	_, vErr, code := capture(func() int { return cmdVerify([]string{"--body", bodyPath}) })
	if code != 6 {
		t.Fatalf("unconfigured verify should exit 6, got %d (%s)", code, vErr)
	}
	if !strings.Contains(vErr, "could not check") || !strings.Contains(vErr, "no verifier pubkey configured") {
		t.Fatalf("stderr should say no pubkey configured: %s", vErr)
	}
}

// TestCLIKeygenSelfGenerate: a local adopter generates a keypair, signs with the
// generated private key, and verifies against the generated public key exported
// into the variable — the full self-generation loop, no committed key.
func TestCLIKeygenSelfGenerate(t *testing.T) {
	dir := t.TempDir()
	privPath := filepath.Join(dir, "verifier.pem")

	pubOut, kErr, code := capture(func() int { return cmdKeygen([]string{"--priv", privPath}) })
	if code != 0 {
		t.Fatalf("keygen exit %d (%s)", code, kErr)
	}
	if !strings.Contains(pubOut, "BEGIN PUBLIC KEY") {
		t.Fatalf("keygen stdout not a PKIX PEM:\n%s", pubOut)
	}
	fi, err := os.Stat(privPath)
	if err != nil {
		t.Fatalf("keygen did not write private key: %v", err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("private key perms = %o, want 600", fi.Mode().Perm())
	}

	payloadPath := filepath.Join(dir, "p.json")
	os.WriteFile(payloadPath, []byte(`{"schema":"verdict-v1","entries":[]}`), 0o644)
	if _, _, code := capture(func() int { return cmdSign([]string{"--payload", payloadPath, "--pem", privPath}) }); code != 0 {
		t.Fatalf("sign with keygen'd key exit %d", code)
	}

	t.Setenv(deskkit.VerifierPubkeyVar, pubOut)
	_, vErr, code := capture(func() int { return cmdVerify([]string{"--body", filepath.Join(dir, "p.out")}) })
	if code != 0 {
		t.Fatalf("verify with keygen'd pub should exit 0, got %d (%s)", code, vErr)
	}
}

func TestCLIKeygenRequiresPriv(t *testing.T) {
	if _, _, code := capture(func() int { return cmdKeygen(nil) }); code != 5 {
		t.Fatalf("keygen without --priv should exit 5, got %d", code)
	}
}

func TestCLIUsageAndUnknown(t *testing.T) {
	if _, _, code := capture(func() int { return run(nil) }); code != 5 {
		t.Fatalf("no args should exit 5, got %d", code)
	}
	if _, _, code := capture(func() int { return run([]string{"help"}) }); code != 0 {
		t.Fatalf("help should exit 0, got %d", code)
	}
	if _, _, code := capture(func() int { return run([]string{"bogus"}) }); code != 5 {
		t.Fatalf("unknown subcommand should exit 5, got %d", code)
	}
}

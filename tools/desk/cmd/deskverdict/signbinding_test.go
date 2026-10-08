package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// Keyless-compose brief, Verify row 11 — the binding flags refuse a mismatched payload BEFORE any key
// is resolved.

const (
	bindRepo = "example-org/example-repo"
	bindHead = "3f9c2a1b7d4e6f8091a2b3c4d5e6f708192a3b4c"
	bindTS   = "2026-10-08T03:00:00Z"
)

var bindPayload = `{"entries":[],"head":"` + bindHead + `","repo":"` + bindRepo + `","schema":"verdict-v1","ts":"` + bindTS + `"}` + "\n"

// payloadIn writes content to p.json in its own fresh 0700 directory, so no case meets
// another case's .out sibling.
func payloadIn(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "p.json")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func hexSHA(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func TestSignBindingRefusesMismatchBeforeKey(t *testing.T) {
	missingPEM := filepath.Join(t.TempDir(), "does-not-exist.pem")
	good := hexSHA(bindPayload)

	for _, c := range []struct {
		name    string
		payload string
		flags   []string
	}{
		{"wrong-digest", bindPayload, []string{"--expect-sha256", hexSHA("other")}},
		{"wrong-repo", bindPayload, []string{"--expect-repo", "example-org/other-repo"}},
		{"wrong-head", bindPayload, []string{"--expect-head", "4a0d3b2c8e5f7a91b2c3d4e5f60718293a4b5c6d"}},
		{"ts-before-not-before", bindPayload, []string{"--not-before", "2026-10-08T03:00:01Z"}},
		{"ts-after-not-after", bindPayload, []string{"--not-after", "2026-10-08T02:59:59Z"}},
		{"repo-on-array", `[1,2]` + "\n", []string{"--expect-repo", bindRepo}},
		{"empty-head-value", bindPayload, []string{"--expect-head", ""}},
	} {
		t.Run(c.name, func(t *testing.T) {
			args := append([]string{"--payload", payloadIn(t, c.payload), "--pem", missingPEM}, c.flags...)
			out, errOut, code := capture(func() int { return cmdSign(args) })
			if code != deskkit.ExitRefused {
				t.Fatalf("%s: exit %d (%s), want 5 — a refusal before any key lookup (6 means it reached the key)", c.name, code, errOut)
			}
			if out != "" {
				t.Fatalf("%s: refusal printed to stdout: %q", c.name, out)
			}
		})
	}

	keyDir := t.TempDir()
	privPath, key := writePrivPEM(t, keyDir)

	bound, errOut, code := capture(func() int {
		return cmdSign([]string{"--payload", payloadIn(t, bindPayload), "--pem", privPath,
			"--expect-sha256", good, "--expect-repo", bindRepo, "--expect-head", bindHead,
			"--not-before", bindTS, "--not-after", bindTS})
	})
	if code != 0 {
		t.Fatalf("matching binding: exit %d (%s), want 0", code, errOut)
	}
	if state, msg := deskkit.VerifyVerdictBody(bound, &key.PublicKey); state != deskkit.VerdictVerified {
		t.Fatalf("matching binding: body does not verify: %v (%s)", state, msg)
	}

	plain, errOut, code := capture(func() int {
		return cmdSign([]string{"--payload", payloadIn(t, bindPayload), "--pem", privPath})
	})
	if code != 0 {
		t.Fatalf("no binding flags: exit %d (%s), want 0", code, errOut)
	}
	if plain != bound {
		t.Fatalf("no binding flags: body differs from the bound body:\n%s\n---\n%s", plain, bound)
	}
}

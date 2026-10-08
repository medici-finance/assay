//go:build unix

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// The payload's parent directory is checked once and every later lookup of the payload
// is made relative to that checked directory, never by re-resolving its path. A
// directory swapped for a link after the check therefore cannot redirect the read to
// a payload in a directory that was never checked.
func TestSignPayloadDirSwappedAfterCheckIsRefused(t *testing.T) {
	keyDir := t.TempDir()
	privPath, _ := writePrivPEM(t, keyDir)
	t.Cleanup(func() { afterPayloadDirCheck = nil })

	base := signDir(t)
	checked := filepath.Join(base, "checked")
	if err := os.Mkdir(checked, 0o700); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(checked, "p.json"), bindPayload)

	// The directory the swap points at holds a DIFFERENT valid payload, so a signer
	// that re-resolves the path would read and sign it (exit 0).
	other := signDir(t)
	otherPayload := `{"entries":[],"head":"` + bindHead + `","repo":"example-org/other-repo","schema":"verdict-v1","ts":"` + bindTS + `"}` + "\n"
	writeFile(t, filepath.Join(other, "p.json"), otherPayload)

	afterPayloadDirCheck = func(dir string) {
		if err := os.Rename(dir, dir+".moved"); err != nil {
			t.Errorf("swap: %v", err)
			return
		}
		if err := os.Symlink(other, dir); err != nil {
			t.Errorf("swap: %v", err)
		}
	}

	out, _, code := capture(func() int {
		return cmdSign([]string{"--payload", filepath.Join(checked, "p.json"), "--pem", privPath})
	})
	if code != deskkit.ExitRefused {
		t.Fatalf("exit %d, want 5 — the read followed a directory swapped in after the check", code)
	}
	if out != "" {
		t.Fatalf("refusal printed to stdout: %q", out)
	}
}

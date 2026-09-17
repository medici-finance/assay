package main

import (
	"os"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// captureStderr runs fn with os.Stderr redirected to an in-process pipe and returns
// everything fn wrote — workpad_test.go's captureStdout, mirrored for the stream `run`
// actually writes a refusal's error text to.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	old := os.Stderr
	os.Stderr = w
	fn()
	if cerr := w.Close(); cerr != nil {
		t.Fatalf("close pipe writer: %v", cerr)
	}
	os.Stderr = old
	out := make([]byte, 0, 4096)
	buf := make([]byte, 4096)
	for {
		n, rerr := r.Read(buf)
		out = append(out, buf[:n]...)
		if rerr != nil {
			break
		}
	}
	return string(out)
}

// TestSchemaRefusalNamesTheOfflineCheck pins brief 27's ergonomics half for deskreply:
// its largest measured refusal class — a --workpad body missing the exact-match marker
// line (196 of 589 recorded refusals on one operating desk host) — now names the offline
// rehearsal (`--dry-run`) that would have caught it for free, without losing the original
// diagnosis.
func TestSchemaRefusalNamesTheOfflineCheck(t *testing.T) {
	work := newBaseFixture(t)
	withEnv(t, work)

	bf := bodyFileWith(t, "an ordinary reply body with no workpad marker")
	out := captureStderr(t, func() {
		rc := run([]string{"example-org/tracker", "7", "--workpad", "--body-file", bf})
		if rc != deskkit.ExitRefused {
			t.Fatalf("--workpad body without the marker rc = %d, want 5 (refused)", rc)
		}
	})
	if !strings.Contains(out, "does not carry the exact-match workpad marker") {
		t.Errorf("the original diagnosis is gone: %s", out)
	}
	if !strings.Contains(out, "deskreply") || !strings.Contains(out, "--dry-run") {
		t.Errorf("the refusal does not name the offline check that would have caught it: %s", out)
	}
}

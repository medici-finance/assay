package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// regression: #2026
// TestDeskevidenceRefusesUnpairedReadSet — build-less-brittle/15 Task step 7,
// landing half: a row naming a read-set file whose local copy is absent or
// hashes to another digest is refused before any forge call, and so is a
// matching one, because this tool's one-file write cannot pair the row and
// its file in one commit. A could-not-check token names no file and lands.
func TestDeskevidenceRefusesUnpairedReadSet(t *testing.T) {
	const evidencePath = "docs/streams/x/brief.md"
	body := []byte("assay-read-set v1\n")
	sum := sha256.Sum256(body)
	digest := hex.EncodeToString(sum[:])
	row := func(token string) string {
		return "# Brief\n\n## Evidence\n| 1 | `true` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-30 | x @ 0123456789ab " + token + " |\n"
	}
	cases := []struct {
		name   string
		token  string
		file   []byte // nil: no read-set file locally
		landed bool
		why    string
	}{
		{"absent locally", "(reads sha256:" + digest + ")", nil, false, "is absent locally"},
		{"another digest", "(reads sha256:" + digest + ")", []byte("other\n"), false, "hashes to "},
		{"matching file", "(reads sha256:" + digest + ")", body, false, "cannot land in one commit"},
		{"could-not-check token", "(reads could-not-check: recorder is Linux-only (darwin))", nil, true, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f, _ := setupFake(t)
			root := rootWithFile(t, evidencePath, row(c.token))
			if c.file != nil {
				p := filepath.Join(root, "docs", "streams", "x", "reads", digest)
				if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(p, c.file, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			f.setFile(evidencePath, "# Brief\n\n## Evidence\n")
			code := run([]string{"example-org/tracker", "main", "--evidence-file", evidencePath, "--root", root})
			if c.landed {
				if code != deskkit.ExitOK || f.putCalls != 1 {
					t.Fatalf("exit %d with %d writes, want a landing", code, f.putCalls)
				}
				return
			}
			if code != deskkit.ExitRefused {
				t.Fatalf("exit %d, want %d", code, deskkit.ExitRefused)
			}
			if f.putCalls != 0 || len(f.hits) != 0 {
				t.Fatalf("a refused read-set landing reached the forge: %d writes, hits %v", f.putCalls, f.hits)
			}
			if d := lastAudit(t).Detail; !strings.Contains(d, c.why) {
				t.Fatalf("refusal %q lacks %q", d, c.why)
			}
		})
	}
}

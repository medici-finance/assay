package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// rulinggate_diff_test.go — the CI-DIFF that keeps two copies of one parse honest.
//
// deskkit.ReadRulingSignOff is now the DECLARED SOURCE for "did a human sign ruling
// R-N?" (deskmerge's R-5 gate consumes it). deskclose still carries its own copy in
// authority.go, because migrating this package's suite is a separate change with its own
// blast radius. Two hand-maintained copies of one fact is precisely the derive-or-diff
// failure — and the copy that drifts would be one authorizing somebody's writes.
//
// So the copies are DIFFED rather than trusted: the same corpus goes through both, and
// the (url, exit-code) outcomes must be identical. Refusal TEXT is deliberately not
// compared — each tool names itself in its own message, and that is the one thing they
// are supposed to differ on.
//
// When deskclose's copy is eventually deleted in favour of the deskkit one, this file
// goes with it.
func TestSignOffParserMatchesDeskkit(t *testing.T) {
	const url = "https://github.com/medici-finance/assay/pull/444#issuecomment-5206838120"
	corpus := []struct {
		name string
		body string
	}{
		{"signed", "## R-1 Close lanes\n\n**Sign-off:** " + url + "\n"},
		{"unsigned", "## R-1 Close lanes\n\n**Sign-off:** _(empty)_\n"},
		{"a signed neighbour must not sign R-1",
			"## R-1 Close lanes\n\n**Sign-off:** _(empty)_\n\n## R-5 Merge\n\n**Sign-off:** " + url + "\n"},
		{"R-1 signed with a later ruling present",
			"## R-1 Close lanes\n\n**Sign-off:** " + url + "\n\n## R-5 Merge\n\n**Sign-off:** _(empty)_\n"},
		{"no such section", "## R-5 Merge\n\n**Sign-off:** " + url + "\n"},
		{"section present, no sign-off line", "## R-1 Close lanes\n\nStatement.\n"},
		{"prefix-matching heading", "## R-10 Something\n\n**Sign-off:** " + url + "\n"},
		{"lowercase heading spelling", "## r-1 close lanes\n\n**Sign-off:** " + url + "\n"},
		{"sign-off written without the colon", "## R-1 Close lanes\n\n**Sign-off** " + url + "\n"},
		{"CRLF line endings", "## R-1 Close lanes\r\n\r\n**Sign-off:** " + url + "\r\n"},
		{"a URL inside parentheses", "## R-1 Close lanes\n\n**Sign-off:** (" + url + ")\n"},
	}
	for _, tc := range corpus {
		t.Run(tc.name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "rulings.md")
			if err := os.WriteFile(p, []byte(tc.body), 0o600); err != nil {
				t.Fatal(err)
			}
			localURL, localErr := readRulingSignOff(p)
			kitURL, kitErr := deskkit.ReadRulingSignOff(p, rulingID, "deskclose")

			if localURL != kitURL {
				t.Fatalf("the two parsers disagree on the URL: deskclose=%q deskkit=%q",
					localURL, kitURL)
			}
			if lc, kc := deskkit.ExitCodeOf(localErr), deskkit.ExitCodeOf(kitErr); lc != kc {
				t.Fatalf("the two parsers disagree on the outcome: deskclose exit %d (%v), "+
					"deskkit exit %d (%v) — one of them is about to authorize something the "+
					"other would refuse", lc, localErr, kc, kitErr)
			}
		})
	}

	t.Run("an unreadable register agrees too", func(t *testing.T) {
		p := filepath.Join(t.TempDir(), "absent.md")
		_, localErr := readRulingSignOff(p)
		_, kitErr := deskkit.ReadRulingSignOff(p, rulingID, "deskclose")
		if lc, kc := deskkit.ExitCodeOf(localErr), deskkit.ExitCodeOf(kitErr); lc != kc {
			t.Fatalf("deskclose exit %d, deskkit exit %d", lc, kc)
		}
	})
}

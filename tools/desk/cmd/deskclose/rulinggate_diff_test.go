package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// rulinggate_diff_test.go — the CI-DIFF that keeps two (url, exit-code) MAPPINGS honest.
//
// There is one parser of the rulings register: deskkit.ReadSignOff (block-oriented,
// three-state). Both authority gates delegate the whole determination to it. What each
// gate still owns is the MAPPING of those three states onto the older (url, error)
// contract plus its own operator wording: deskclose's readRulingSignOff in authority.go,
// and deskkit.ReadRulingSignOff (which deskmerge's R-5 gate consumes). Two hand-written
// mappings of one determination is a smaller derive-or-diff surface than two parsers
// were, but a drift in either switch would still let one gate authorize a write the
// other would refuse.
//
// So the mappings are DIFFED rather than trusted: the same corpus goes through both, and
// the (url, exit-code) outcomes must be identical. Refusal TEXT is deliberately not
// compared — each tool names itself in its own message, and that is the one thing they
// are supposed to differ on. Because both now sit on ReadSignOff, agreement is true by
// construction on the divergence shapes below, where a line-oriented reader used to
// disagree; the corpus keeps that from silently regressing.
func TestSignOffParserMatchesDeskkit(t *testing.T) {
	const url = "https://github.com/medici-finance/assay/pull/444#issuecomment-5206838120"
	const url2 = "https://github.com/medici-finance/assay/pull/555#issuecomment-5206839999"
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

		// The divergence shapes surfaced in review: registers where a
		// LINE-oriented reader and the block-oriented house reader gave different
		// answers, three of them in the permissive direction. With both gates now
		// delegating to deskkit.ReadSignOff these must agree; before that delegation
		// each of these rows went red here, which is the point.
		{"prose on the label line, URL on the line below",
			"## R-1 Close lanes\n\n**Sign-off:** approved by the authority:\n" + url + "\n"},
		{"label line ends 'approved:', blank line, then a URL",
			"## R-1 Close lanes\n\n**Sign-off:** approved:\n\n" + url + "\n"},
		{"two sign-off blocks in the section",
			"## R-1 Close lanes\n\n**Sign-off:** " + url + "\n\n**Sign-off:** " + url2 + "\n"},
		{"one block naming two URLs (superseded)",
			"## R-1 Close lanes\n\n**Sign-off:** " + url + " superseded by " + url2 + "\n"},
		{"a sub-heading before the sign-off line",
			"## R-1 Close lanes\n\n### Notes\n\n**Sign-off:** " + url + "\n"},
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

package deskkit

import (
	"os"
	"path/filepath"
	"testing"
)

// rulinggate_test.go — the three states of "did a human sign ruling R-N?".
//
// The case that matters most is the LEAVE arm of the section walk: without it, a signed
// later ruling is read as an earlier one's sign-off, and "one ruling was signed" becomes
// "all of them were". The register in this repo has R-1..R-5 in one file and only some
// of them will ever be signed, so that is not a hypothetical.

func writeRulings(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "rulings.md")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

const signedURL = "https://github.com/medici-finance/assay/pull/444#issuecomment-5206838120"

func TestReadRulingSignOff(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		ruling   string
		wantURL  string
		wantCode int
	}{
		{
			name:     "a signed ruling returns its URL",
			body:     "## R-5 Desk merge-currency\n\n**Sign-off:** " + signedURL + "\n",
			ruling:   "R-5",
			wantURL:  signedURL,
			wantCode: ExitOK,
		},
		{
			name: "an EMPTY sign-off line is a REFUSAL, not could-not-check — " +
				"'the human has not granted this' is a positive determination",
			body:     "## R-5 Desk merge-currency\n\n**Sign-off:** _(empty — the blessing authority fills this)_\n",
			ruling:   "R-5",
			wantCode: ExitRefused,
		},
		{
			name: "a SIGNED NEIGHBOUR does not sign this ruling — the section walk must " +
				"LEAVE on the next heading",
			body: "## R-1 Close lanes\n\n**Sign-off:** " + signedURL + "\n\n" +
				"## R-5 Desk merge-currency\n\n**Sign-off:** _(empty)_\n",
			ruling:   "R-5",
			wantCode: ExitRefused,
		},
		{
			name: "and the mirror image: an earlier ruling is not signed by a later one",
			body: "## R-1 Close lanes\n\n**Sign-off:** _(empty)_\n\n" +
				"## R-5 Desk merge-currency\n\n**Sign-off:** " + signedURL + "\n",
			ruling:   "R-1",
			wantCode: ExitRefused,
		},
		{
			name:     "a register with no such section is could-not-check",
			body:     "## R-1 Close lanes\n\n**Sign-off:** " + signedURL + "\n",
			ruling:   "R-5",
			wantCode: ExitUnverifiable,
		},
		{
			name:     "a section with no Sign-off line at all is could-not-check",
			body:     "## R-5 Desk merge-currency\n\nStatement with no sign-off line.\n",
			ruling:   "R-5",
			wantCode: ExitUnverifiable,
		},
		{
			name:     "the heading must not prefix-match a longer ruling id",
			body:     "## R-50 Something else\n\n**Sign-off:** " + signedURL + "\n",
			ruling:   "R-5",
			wantCode: ExitUnverifiable,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ReadRulingSignOff(writeRulings(t, tc.body), tc.ruling, "testtool")
			if code := ExitCodeOf(err); code != tc.wantCode {
				t.Fatalf("exit %d, want %d (err: %v)", code, tc.wantCode, err)
			}
			if got != tc.wantURL {
				t.Fatalf("url = %q, want %q", got, tc.wantURL)
			}
		})
	}

	t.Run("an unreadable register is could-not-check, never unsigned", func(t *testing.T) {
		_, err := ReadRulingSignOff(filepath.Join(t.TempDir(), "absent.md"), "R-5", "testtool")
		if code := ExitCodeOf(err); code != ExitUnverifiable {
			t.Fatalf("exit %d, want %d — 'I could not read the register' and 'the human "+
				"declined' are different answers", code, ExitUnverifiable)
		}
	})
}

func TestCommentPermalinkRe(t *testing.T) {
	ok := []string{
		"https://github.com/medici-finance/assay/pull/444#issuecomment-5206838120",
		"https://github.com/o/r/issues/1#issuecomment-2",
	}
	for _, u := range ok {
		if !CommentPermalinkRe.MatchString(u) {
			t.Fatalf("%s should parse", u)
		}
	}
	bad := []string{
		// A link to a THREAD is not an authorization: a thread is written by whoever
		// shows up.
		"https://github.com/medici-finance/assay/pull/444",
		"https://github.com/o/r/pull/1#discussion_r2",
		"http://github.com/o/r/issues/1#issuecomment-2",
		"https://evil.example/github.com/o/r/issues/1#issuecomment-2",
	}
	for _, u := range bad {
		if CommentPermalinkRe.MatchString(u) {
			t.Fatalf("%s must NOT parse as an authorization permalink", u)
		}
	}
}

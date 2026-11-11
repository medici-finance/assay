package deskkit

import "testing"

// TestParseCommentLinkForms reads each forge's accepted form into the forge, host, object and
// comment id it names, and refuses every other shape.
// regression: #6061
func TestParseCommentLinkForms(t *testing.T) {
	ok := []struct {
		in   string
		want CommentLink
	}{
		{
			"https://github.com/example-org/tracker/issues/120#issuecomment-7003",
			CommentLink{Forge: ForgeGitHub, Host: "github.com",
				Item: ItemRef{Repo: "example-org/tracker", Number: 120, Kind: TargetIssue}, CommentID: 7003},
		},
		{
			"https://github.com/example-org/tracker/pull/41#issuecomment-9",
			CommentLink{Forge: ForgeGitHub, Host: "github.com",
				Item: ItemRef{Repo: "example-org/tracker", Number: 41, Kind: TargetChange}, CommentID: 9},
		},
		{
			"https://gitlab.example.test/grp/proj/-/issues/120#note_8001",
			CommentLink{Forge: ForgeGitLab, Host: "gitlab.example.test",
				Item: ItemRef{Repo: "grp/proj", Number: 120, Kind: TargetIssue}, CommentID: 8001},
		},
		{
			"  https://GitLab.com/grp/proj/-/merge_requests/7#note_5  ",
			CommentLink{Forge: ForgeGitLab, Host: "gitlab.com",
				Item: ItemRef{Repo: "grp/proj", Number: 7, Kind: TargetChange}, CommentID: 5},
		},
	}
	for _, c := range ok {
		got, err := ParseCommentPermalink(c.in)
		if err != nil {
			t.Fatalf("%q: unexpected refusal: %v", c.in, err)
		}
		if got != c.want {
			t.Fatalf("%q: got %+v, want %+v", c.in, got, c.want)
		}
	}

	refused := []string{
		"",
		"https://github.com/example-org/tracker/issues/120",                       // thread, GitHub
		"https://gitlab.example.test/grp/proj/-/issues/120",                       // thread, GitLab
		"https://gitlab.example.test/grp/proj/-/issues/120#",                      // empty anchor
		"https://gitlab.example.test/grp/proj/-/issues/120#discussion_1",          // unknown anchor
		"https://example.test/grp/proj/issues/120#issuecomment-1",                 // GitHub anchor off github.com
		"https://gitlab.example.test/grp/proj/issues/120#note_1",                  // note anchor, no /-/
		"https://gitlab.example.test/grp/proj/-/pull/120#note_1",                  // note anchor on a pull path
		"http://gitlab.example.test/grp/proj/-/issues/120#note_1",                 // not https
		"https://gitlab.example.test:8443/grp/proj/-/issues/120#note_1",           // port
		"https://user@gitlab.example.test/grp/proj/-/issues/120#note_1",           // user info
		"https://gitlab.example.test/grp/sub/proj/-/issues/120#note_1",            // nested project
		"https://gitlab.example.test/grp/proj/-/issues/0#note_1",                  // number 0
		"https://gitlab.example.test/grp/proj/-/issues/120#note_0",                // comment id 0
		"https://gitlab.example.test/grp/proj/-/issues/120#note_1#issuecomment-1", // two anchors
		"example-org/tracker#120",                                                 // a short ref, not a link
	}
	for _, in := range refused {
		if _, err := ParseCommentPermalink(in); !IsRefused(err) {
			t.Fatalf("%q: want a refusal, got %v", in, err)
		}
	}
}

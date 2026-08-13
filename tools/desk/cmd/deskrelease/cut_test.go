package main

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// --- argv strictness ------------------------------------------------------------

// TestParseCutArgs is the end anchor a glob does not have. Every rejection is named,
// and each asserts the REASON as well as the refusal — so a guard that is deleted (and
// whose work another layer happens to duplicate) still fails a test.
func TestParseCutArgs(t *testing.T) {
	const tag = "desk-tools/v0.1.3"

	cases := []struct {
		name       string
		argv       []string
		wantTag    string
		wantDry    bool
		wantRefuse string // substring of the refusal; "" means accept
	}{
		{name: "tag alone", argv: []string{tag}, wantTag: tag},
		{name: "tag then --dry-run", argv: []string{tag, "--dry-run"}, wantTag: tag, wantDry: true},
		{name: "--dry-run then tag", argv: []string{"--dry-run", tag}, wantTag: tag, wantDry: true},

		{name: "extra operand after tag", argv: []string{tag, "main"}, wantRefuse: "extra operand"},
		{name: "extra refspec operand", argv: []string{tag, "+HEAD:refs/heads/main"}, wantRefuse: "extra operand"},
		{name: "two tags", argv: []string{tag, "statusgen/v9.9.9"}, wantRefuse: "extra operand"},
		{name: "unknown long flag", argv: []string{tag, "--force"}, wantRefuse: "unrecognised flag"},
		{name: "bundled short opts", argv: []string{tag, "-uf"}, wantRefuse: "unrecognised flag"},
		{name: "leading-dash tag", argv: []string{"-desk-tools/v0.1.3"}, wantRefuse: "unrecognised flag"},
		{name: "bare dash", argv: []string{"-"}, wantRefuse: "unrecognised flag"},
		{name: "repeated --dry-run", argv: []string{tag, "--dry-run", "--dry-run"}, wantRefuse: "--dry-run given twice"},
		{name: "empty argument", argv: []string{tag, ""}, wantRefuse: "empty argument"},
		{name: "empty argument alone", argv: []string{""}, wantRefuse: "empty argument"},
		{name: "flag but no tag", argv: []string{"--dry-run"}, wantRefuse: "no <tag> given"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseCutArgs(tc.argv)
			if tc.wantRefuse != "" {
				if err == nil {
					t.Fatalf("parseCutArgs(%q) accepted %+v; want refusal %q", tc.argv, got, tc.wantRefuse)
				}
				if !deskkit.IsRefused(err) {
					t.Fatalf("parseCutArgs(%q) error %v maps to exit %d; want refused (5)",
						tc.argv, err, deskkit.ExitCodeOf(err))
				}
				if !strings.Contains(err.Error(), tc.wantRefuse) {
					t.Fatalf("parseCutArgs(%q) refused with %q; want it to mention %q", tc.argv, err.Error(), tc.wantRefuse)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseCutArgs(%q) refused unexpectedly: %v", tc.argv, err)
			}
			if got.tag != tc.wantTag || got.dryRun != tc.wantDry {
				t.Fatalf("parseCutArgs(%q) = %+v; want tag=%q dryRun=%v", tc.argv, got, tc.wantTag, tc.wantDry)
			}
		})
	}
}

// --- tag validation -------------------------------------------------------------

// TestValidateTag enumerates, BY NAME, every shape the glob allow-rule could not
// exclude. The metacharacter cases are the four escapes proved against a sandbox bare
// repo (#1538): peel-to-commit, bare `+` force, second refspec, and ref deletion.
func TestValidateTag(t *testing.T) {
	cases := []struct {
		name       string
		tag        string
		wantRefuse string // "" = accept
	}{
		{name: "valid desk-tools", tag: "desk-tools/v0.1.3"},
		{name: "valid statusgen", tag: "statusgen/v0.5.0"},
		{name: "valid multi-digit", tag: "desk-tools/v12.30.400"},

		// --- refspec metacharacters, refused BY NAME ---
		{name: "second refspec after a space", tag: "desk-tools/v0.1.3 +HEAD:refs/heads/main", wantRefuse: "whitespace"},
		{name: "second refspec after a tab", tag: "desk-tools/v0.1.3\tmain", wantRefuse: "whitespace"},
		{name: "bare plus force", tag: "desk-tools/v0.1.3 +main", wantRefuse: "whitespace"},
		{name: "leading plus force", tag: "+desk-tools/v0.1.3", wantRefuse: "refspec metacharacter"},
		{name: "colon destination long form", tag: "desk-tools/v0.1.3:refs/heads/main", wantRefuse: "refspec metacharacter"},
		{name: "colon destination short form", tag: "desk-tools/v0.1.3:main", wantRefuse: "refspec metacharacter"},
		{name: "peel to commit then destination", tag: "desk-tools/v0.1.3^{}:refs/heads/main", wantRefuse: "refspec metacharacter"},
		{name: "ref deletion colon prefix", tag: ":refs/heads/release", wantRefuse: "refspec metacharacter"},
		{name: "revision walk tilde", tag: "desk-tools/v0.1.3~1", wantRefuse: "refspec metacharacter"},
		{name: "reflog at-brace", tag: "desk-tools/v0.1.3@{1}", wantRefuse: "refspec metacharacter"},
		{name: "glob star", tag: "desk-tools/v*", wantRefuse: "refspec metacharacter"},
		{name: "glob question", tag: "desk-tools/v0.1.?", wantRefuse: "refspec metacharacter"},
		{name: "glob class", tag: "desk-tools/v0.1.[0-9]", wantRefuse: "refspec metacharacter"},
		{name: "backslash escape", tag: `desk-tools/v0.1.3\n`, wantRefuse: "refspec metacharacter"},
		{name: "trailing newline", tag: "desk-tools/v0.1.3\n", wantRefuse: "control character"},
		{name: "embedded NUL", tag: "desk-tools/v0.1.3\x00extra", wantRefuse: "control character"},

		// --- shape violations, refused by the anchored pattern ---
		{name: "wrong namespace", tag: "tracker/v0.1.3", wantRefuse: "does not match"},
		{name: "no namespace", tag: "v0.1.3", wantRefuse: "does not match"},
		{name: "namespace prefix only", tag: "desk-toolsv0.1.3", wantRefuse: "does not match"},
		{name: "extra namespace segment", tag: "evil/desk-tools/v0.1.3", wantRefuse: "does not match"},
		{name: "path traversal", tag: "desk-tools/../heads/main", wantRefuse: "does not match"},
		{name: "two-part version", tag: "desk-tools/v0.1", wantRefuse: "does not match"},
		{name: "four-part version", tag: "desk-tools/v0.1.3.4", wantRefuse: "does not match"},
		{name: "missing v", tag: "desk-tools/0.1.3", wantRefuse: "does not match"},
		{name: "non-numeric version", tag: "desk-tools/vX.Y.Z", wantRefuse: "does not match"},
		{name: "prerelease suffix", tag: "desk-tools/v0.1.3-rc1", wantRefuse: "does not match"},
		{name: "trailing junk", tag: "desk-tools/v0.1.3extra", wantRefuse: "does not match"},
		{name: "leading junk", tag: "xdesk-tools/v0.1.3", wantRefuse: "does not match"},
		{name: "refs prefix", tag: "refs/tags/desk-tools/v0.1.3", wantRefuse: "does not match"},
		{name: "empty", tag: "", wantRefuse: "does not match"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateTag(tc.tag)
			if tc.wantRefuse == "" {
				if err != nil {
					t.Fatalf("validateTag(%q) refused a valid tag: %v", tc.tag, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("validateTag(%q) ACCEPTED; want refusal mentioning %q", tc.tag, tc.wantRefuse)
			}
			if !deskkit.IsRefused(err) {
				t.Fatalf("validateTag(%q) error %v maps to exit %d; want refused (5)",
					tc.tag, err, deskkit.ExitCodeOf(err))
			}
			if !strings.Contains(err.Error(), tc.wantRefuse) {
				t.Fatalf("validateTag(%q) refused with %q; want it to mention %q", tc.tag, err.Error(), tc.wantRefuse)
			}
		})
	}
}

// TestTagPatternIsAnchoredAtBothEnds pins the property the whole design rests on: the
// pattern cannot be satisfied by a prefix or extended by a suffix. Dropping either
// anchor from tagPattern fails here.
func TestTagPatternIsAnchoredAtBothEnds(t *testing.T) {
	if !tagPattern.MatchString("desk-tools/v1.2.3") {
		t.Fatal("tagPattern rejects a canonical tag")
	}
	for _, s := range []string{
		"desk-tools/v1.2.3 main",
		"desk-tools/v1.2.3:refs/heads/main",
		"xdesk-tools/v1.2.3",
		"desk-tools/v1.2.3\n",
		"desk-tools/v1.2.3\nstatusgen/v1.2.3",
	} {
		if tagPattern.MatchString(s) {
			t.Fatalf("tagPattern matched %q — it is not anchored at both ends", s)
		}
	}
}

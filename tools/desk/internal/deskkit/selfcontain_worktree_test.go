package deskkit

// selfcontain_worktree_test.go — the scratch-worktree-name arm's token boundary (#2080).
//
// The arm refuses a scratch worktree directory name only where such a name can appear: at
// the start of the text, or after a byte that is not a letter, a digit, `_` or `-` — which
// covers a `/` path segment, whitespace and punctuation. A hyphenated compound that merely
// CONTAINS the prefix word (a finding-block class label, say) is ordinary prose and passes.
//
// Every worktree-shaped span below is ASSEMBLED AT RUN TIME (wtName): no line of this file
// is itself one, because the push-path check this arm feeds reads the added lines of the
// change that carries it, and a Go test file is not exempt from it.

import (
	"strconv"
	"strings"
	"testing"
)

// wtPrefix is the minted prefix, split so this file does not carry it whole.
const wtPrefix = "trac" + "ker-"

// wtName is a scratch worktree name the way deskwt mints one.
var wtName = wtPrefix + "demo-item"

const wtCat = "scratch worktree name"

// wtCheck runs the public-repo scan over body and returns its error.
func wtCheck(t *testing.T, body string) error {
	t.Helper()
	return SelfContainCheck("PR body", []byte(body),
		SelfContainOpts{Repo: scPublicRepo, NumberHint: 9000, Notices: io_Discard{}})
}

// wtWantRefused asserts body refuses in the worktree-name category, naming EXACTLY the
// worktree span — never the boundary byte in front of it.
func wtWantRefused(t *testing.T, body string) {
	t.Helper()
	err := wtCheck(t, body)
	if err == nil {
		t.Fatalf("body %q was ADMITTED; it must refuse (%s)", body, wtCat)
	}
	if ExitCodeOf(err) != ExitRefused {
		t.Fatalf("exit code = %d, want %d (refused)", ExitCodeOf(err), ExitRefused)
	}
	msg := err.Error()
	if !strings.Contains(msg, wtCat) || !strings.Contains(msg, wtCat+" "+strconv.Quote(wtName)+" ") {
		t.Fatalf("refusal must name category %q and the exact span %q: %s", wtCat, wtName, msg)
	}
}

// TestWorktreeNameRefuses: a worktree name at a token start or a path segment still
// refuses — the planted cases (b) and (c) plus every boundary the arm must keep.
func TestWorktreeNameRefuses(t *testing.T) {
	scRoster(t)
	for name, body := range map[string]string{
		// (c) a bare leading worktree name.
		"bare, start of text": wtName + " is where it ran",
		"whole text":          wtName,
		// (b) a `/`-prefixed path segment, relative so the absolute-path arm stays silent.
		"slash path segment":     "work/" + wtName + "/tools/desk",
		"dot-dot path segment":   "cd ../" + wtName,
		"trailing path segment":  "worktrees/" + wtName,
		"backslash path segment": `work\` + wtName + `\tools`,
		// Whitespace and line starts.
		"after a space":   "run it in " + wtName + " first",
		"after a tab":     "dir:\t" + wtName,
		"after a newline": "line one\n" + wtName + " line two",
		"after CRLF":      "line one\r\n" + wtName,
		// Punctuation.
		"in backticks":     "my worktree is `" + wtName + "`",
		"in double quotes": `path "` + wtName + `"`,
		"in single quotes": "path '" + wtName + "'",
		"in parentheses":   "(" + wtName + ")",
		"in brackets":      "[" + wtName + "]",
		"after a colon":    "worktree:" + wtName,
		"after equals":     "--dir=" + wtName,
		"after a comma":    "a," + wtName,
		"after a dot":      "." + wtName,
		"after a hash":     "#" + wtName,
		"after an at":      "@" + wtName,
		// A non-ASCII rune is not a letter/digit/`_`/`-` byte: the boundary holds, as it did
		// under the old ASCII `\b`.
		"after a non-ASCII letter": "é" + wtName,
		"after an em dash":         "worktree—" + wtName,
	} {
		t.Run(name, func(t *testing.T) { wtWantRefused(t, body) })
	}
}

// TestWorktreeNameCompoundPasses: the prefix word INSIDE a hyphenated compound, or glued to
// a letter, digit or `_`, names no worktree and passes. Planted case (a) is the first row:
// the finding-block class label that #2080 reports a verdict being withheld over.
func TestWorktreeNameCompoundPasses(t *testing.T) {
	scRoster(t)
	for name, body := range map[string]string{
		"(a) class-label compound":   "class: missing-issue-" + wtPrefix + "cite",
		"compound in prose":          "the item-" + wtPrefix + "row shape is fine",
		"double-hyphen compound":     "--" + wtPrefix + "flag",
		"after an underscore":        "x_" + wtName,
		"after a digit":              "9" + wtName,
		"after a letter":             "x" + wtName,
		"after an uppercase letter":  "X" + wtName,
		"compound inside a path seg": "work/item-" + wtPrefix + "row/file.md",
	} {
		t.Run(name, func(t *testing.T) {
			if err := wtCheck(t, body); err != nil {
				t.Fatalf("body %q was REFUSED; a mid-compound prefix word names no worktree: %v", body, err)
			}
		})
	}
}

// TestWorktreeNameBoundarySweep is the CLASS guard: for every ASCII byte placed directly in
// front of a worktree name, the arm refuses exactly when that byte is not a letter, a digit,
// `_` or `-`. A boundary regression anywhere in the byte range — `-` read as a token start
// again, or `/` dropped from the refusing side — fails here naming the byte.
func TestWorktreeNameBoundarySweep(t *testing.T) {
	scRoster(t)
	glued := func(b byte) bool {
		return b == '-' || b == '_' ||
			(b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
	}
	for b := 0; b < 128; b++ {
		body := "x " + string(rune(b)) + wtName + " y"
		findings, _ := selfContainFindings("PR body", body, SelfContainOpts{Repo: scPublicRepo})
		got := false
		for _, f := range findings {
			if f.category == wtCat {
				got = true
				if f.span != wtName {
					t.Errorf("byte %q: span = %q, want exactly %q", rune(b), f.span, wtName)
				}
			}
		}
		if want := !glued(byte(b)); got != want {
			t.Errorf("byte %q in front of a worktree name: refused = %v, want %v", rune(b), got, want)
		}
	}
}

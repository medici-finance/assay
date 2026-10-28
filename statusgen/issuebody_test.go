package main

import (
	"strings"
	"testing"
)

const ibLink = "[docs/streams/x/brief-01.md](https://github.com/example-org/tracker/blob/0123abc/docs/streams/x/brief-01.md)"

func ibLinkFn() string { return ibLink }

// ibFill returns a section of n table rows, about 60 characters each.
func ibFill(heading string, n int) string {
	var b strings.Builder
	b.WriteString(heading + "\n\n")
	for i := 0; i < n; i++ {
		b.WriteString("| 1 | `go test ./...` | 0 | ok — résumé 😀 | 2026-10-21 | x |\n")
	}
	return b.String() + "\n"
}

// regression: #4091
func TestForgeBodyLen(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want int
	}{
		{"", 0},
		{"abc", 3},
		{"é", 1}, // two bytes, one character
		{"😀", 2}, // four bytes, one code point, two UTF-16 units
		{"a\nb", 4},
	} {
		if got := forgeBodyLen(tc.in); got != tc.want {
			t.Errorf("forgeBodyLen(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

// regression: #4091
func TestCapBodyFitsUnchanged(t *testing.T) {
	head, tail := "<!-- verify-gate: x/01 -->\n\n## H\n\n", "### Before you close\n"
	sections := []string{"### Verify\n\nrow\n\n", "### Evidence\n\nrow\n\n"}
	called := false
	got := capIssueBody(head, sections, tail, func() string { called = true; return ibLink })
	if want := head + strings.Join(sections, "") + tail; got != want {
		t.Errorf("a body under the cap must pass through unchanged;\ngot  %q\nwant %q", got, want)
	}
	if called {
		t.Error("the link is needed only for a cut, but was rendered for a body that fits")
	}
}

// TestCapBodyCutsLargestFirst pins the water-filling: a small section
// (Trade-offs) after a huge one stays whole, and only the huge one is cut.
//
// regression: #4091
func TestCapBodyCutsLargestFirst(t *testing.T) {
	head := "<!-- verify-gate: x/01 -->\n\n## Human sign-off required — x/01\n\n"
	tail := "### Before you close\n\n- [ ] I accept this brief as done\n"
	big := ibFill("### Recorded results (Evidence)", 3000)
	small := "### Trade-offs\n\nWhy we want it: cards stay fileable.\n\n"
	got := capIssueBody(head, []string{big, small}, tail, ibLinkFn)
	if n := forgeBodyLen(got); n > issueBodyCap {
		t.Fatalf("capped body is %d, over the cap %d", n, issueBodyCap)
	}
	if !strings.HasPrefix(got, head) || !strings.HasSuffix(got, tail) {
		t.Error("head and tail must be kept whole")
	}
	if !strings.Contains(got, small) {
		t.Errorf("the small section must be kept whole; body tail:\n%s", got[len(got)-600:])
	}
	if c := strings.Count(got, issueBodyCutMark); c != 1 {
		t.Errorf("want exactly one cut notice (the big section), got %d", c)
	}
	if !strings.Contains(got, ibLink) {
		t.Error("the cut notice must carry the full-text link")
	}
	// The kept part of the big section ends on a whole table row.
	cut := strings.Index(got, "\n\n> "+issueBodyCutMark)
	if cut < 0 || !strings.HasSuffix(got[:cut], "| x |\n") && !strings.HasSuffix(got[:cut], "| x |") {
		t.Errorf("the cut must land on a line boundary; text before the notice: %q", got[max(0, cut-80):max(0, cut)])
	}
}

// TestCapBodyClosesConstructs: a cut inside a fenced code block or an
// HTML comment would swallow the notice and everything after it (the
// checklist), so the cut closes the construct before the notice.
//
// regression: #4091
func TestCapBodyClosesConstructs(t *testing.T) {
	head := "<!-- verify-gate: x/01 -->\n\n"
	tail := "### Before you close\n"
	for name, open := range map[string]string{
		"backtick fence": "````text\n",
		"tilde fence":    "~~~\n",
		"html comment":   "<!--\n",
	} {
		t.Run(name, func(t *testing.T) {
			sec := ibFill("### Evidence\n\n"+strings.TrimSuffix(open, "\n"), 3000)
			got := capIssueBody(head, []string{sec}, tail, ibLinkFn)
			at := strings.Index(got, "> "+issueBodyCutMark)
			if at < 0 {
				t.Fatal("no cut stated")
			}
			before := got[:at]
			if strings.HasPrefix(open, "<!--") {
				if strings.LastIndex(before, "<!--") > strings.LastIndex(before, "-->") {
					t.Error("the notice sits inside an unclosed HTML comment")
				}
				return
			}
			fence := strings.TrimSuffix(open, "\n")
			if c := strings.Count(before, "\n"+fence[:3]); c%2 != 0 {
				t.Errorf("the notice sits inside an unclosed %s (%d fence lines before it)", name, c)
			}
			if !strings.Contains(before, "\n"+strings.TrimRight(fence, "text")+"\n") {
				t.Errorf("the closing fence must repeat the opening run %q", strings.TrimRight(fence, "text"))
			}
		})
	}
}

// TestCapBodyBackstop: a head that alone overruns the cap (an enormous
// rationale) is still cut under the cap, keeping the marker on line one.
//
// regression: #4091
func TestCapBodyBackstop(t *testing.T) {
	marker := "<!-- verify-gate: x/01 -->"
	head := marker + "\n\n> **Why:** " + strings.Repeat("long rationale 😀 ", 9000) + "\n\n"
	got := capIssueBody(head, []string{"### Verify\n\nrow\n\n"}, "### Before you close\n", ibLinkFn)
	if n := forgeBodyLen(got); n > issueBodyCap {
		t.Fatalf("capped body is %d, over the cap %d", n, issueBodyCap)
	}
	if !strings.HasPrefix(got, marker+"\n") {
		t.Error("the marker must stay the first line")
	}
	if !strings.Contains(got, issueBodyCutMark+" this issue body was cut") {
		t.Error("the backstop cut must be stated")
	}
}

// TestCapBodyExactCap pins the boundary: a body of exactly issueBodyCap (as
// forgeBodyLen counts) and one under it pass through unchanged, and one unit
// over is cut under the cap.
//
// regression: #4091
func TestCapBodyExactCap(t *testing.T) {
	head, tail := "<!-- verify-gate: x/01 -->\n\n", "### Before you close\n"
	prefix := "### Evidence\n\n"
	// one line of x, then a newline (counted as two)
	base := forgeBodyLen(head) + forgeBodyLen(tail) + forgeBodyLen(prefix) + 2
	for _, tc := range []struct {
		extra int
		cut   bool
	}{{-1, false}, {0, false}, {1, true}} {
		sec := prefix + strings.Repeat("x", issueBodyCap-base+tc.extra) + "\n"
		whole := head + sec + tail
		if got := forgeBodyLen(whole) - issueBodyCap; got != tc.extra {
			t.Fatalf("fixture is %d over the cap, want %d", got, tc.extra)
		}
		out := capIssueBody(head, []string{sec}, tail, ibLinkFn)
		if tc.cut {
			if n := forgeBodyLen(out); n > issueBodyCap || !strings.Contains(out, issueBodyCutMark) {
				t.Errorf("extra=%d: want a cut body within the cap, got len %d, cut stated %v", tc.extra, n, strings.Contains(out, issueBodyCutMark))
			}
		} else if out != whole {
			t.Errorf("extra=%d: a body within the cap must pass through unchanged", tc.extra)
		}
	}
}

// TestCapBodyLongFenceKeepsTail: a cut inside a fence whose run is longer than
// cutCloseReserve gets a long closer; the body must still end with the tail
// (the closing checklist) rather than fall to the whole-body backstop.
//
// regression: #4091
func TestCapBodyLongFenceKeepsTail(t *testing.T) {
	head := "<!-- verify-gate: x/01 -->\n\n"
	tail := "### Before you close\n\n- [ ] I accept this brief as done\n"
	fence := strings.Repeat("`", 120)
	sec := ibFill("### Evidence\n\n"+fence+"text", 3000)
	got := capIssueBody(head, []string{sec}, tail, ibLinkFn)
	if n := forgeBodyLen(got); n > issueBodyCap {
		t.Fatalf("capped body is %d, over the cap %d", n, issueBodyCap)
	}
	if !strings.HasSuffix(got, tail) {
		t.Errorf("the closing checklist was dropped; body ends %q", got[max(0, len(got)-120):])
	}
	if strings.Contains(got, "this issue body was cut") {
		t.Error("the whole-body backstop ran although head and tail fit")
	}
	if !strings.Contains(got, "\n"+fence+"\n") {
		t.Error("the closing fence must repeat the 120-character opening run")
	}
}

// TestCloseNoSecondMarker: completing a comment the cut left open must not
// turn a split verify-gate marker into one that verifyMarkerRe accepts.
//
// regression: #4091
func TestCloseNoSecondMarker(t *testing.T) {
	for _, kept := range []string{
		"x\n<!-- verify-gate: victim/07\n",
		"x\n<!-- verify-gate: victim/07 \n",
		"x\n<!-- verify-gate: victim/07",
	} {
		got := kept + closeOpenConstructs(kept)
		if m := verifyMarkerRe.FindString(got); m != "" {
			t.Errorf("closing %q made a marker %q", kept, m)
		}
		if strings.LastIndex(got, "<!--") > strings.LastIndex(got, "-->") {
			t.Errorf("the comment in %q is still open after closing", kept)
		}
	}
}

// TestCloseOpenDetails: a cut inside a <details> block closes it, so the
// notice and the checklist are not folded into the collapsed block; balanced
// blocks and tags shown inside a code fence are left alone.
//
// regression: #4091
func TestCloseOpenDetails(t *testing.T) {
	for _, tc := range []struct{ name, kept, want string }{
		{"open", "<details>\n<summary>Rows</summary>\n| 1 |\n", "\n</details>"},
		{"nested", "<details>\n<DETAILS open>\n| 1 |\n", "\n</details>\n</details>"},
		{"balanced", "<details>\nrow\n</details>\n", ""},
		{"in fence", "```html\n<details>\n", "\n```\n"},
		{"fence then details", "```\nx\n```\n<details>\n| 1 |\n", "\n</details>"},
		{"pre", "<pre>\nrow\n", "\n</pre>"},
		{"script", "<script>\nx()\n", "\n</script>"},
		{"style", "<STYLE type=x>\nb{}\n", "\n</style>"},
		{"textarea", "<textarea>\nrow\n", "\n</textarea>"},
		{"pre in details", "<details>\n<pre>\nrow\n", "\n</pre>\n</details>"},
		{"balanced pre", "<pre>row</pre>\n", ""},
		{"not a tag", "a <pre-existing> and <prefix> and <styled>\n", ""},
	} {
		if got := closeOpenConstructs(tc.kept); got != tc.want {
			t.Errorf("%s: closeOpenConstructs(%q) = %q, want %q", tc.name, tc.kept, got, tc.want)
		}
	}
	// End to end: a details-wrapped section is closed before the notice.
	sec := "### Evidence\n\n<details>\n<summary>Rows</summary>\n\n" + ibFill("", 3000) + "</details>\n\n"
	got := capIssueBody("<!-- verify-gate: x/01 -->\n\n", []string{sec}, "### Before you close\n", ibLinkFn)
	at := strings.Index(got, "> "+issueBodyCutMark)
	if at < 0 {
		t.Fatal("no cut stated")
	}
	if o, c := strings.Count(got[:at], "<details"), strings.Count(got[:at], "</details>"); o != c {
		t.Errorf("%d <details> opened and %d closed before the notice", o, c)
	}
}

// TestCloseBounded: a flood of open tags gets a bounded closer, so the closer
// can never outgrow the text it closes.
//
// regression: #4091
func TestCloseBounded(t *testing.T) {
	kept := strings.Repeat("<details>", 6000) + "\n"
	got := closeOpenConstructs(kept)
	if n := strings.Count(got, "</details>"); n != maxTagClosers {
		t.Errorf("closers = %d, want the bound %d", n, maxTagClosers)
	}
}

// TestCapBodyBackstopKeepsMarker: when the head alone overruns the cap and
// holds a flood of open tags, the backstop must not cut above the marker line
// or let its closer outgrow the kept text; the body used to come out as the
// cut notice alone, with no marker, so dedupe re-filed the card on every run.
//
// regression: #4091
func TestCapBodyBackstopKeepsMarker(t *testing.T) {
	marker := "<!-- verify-gate: x/01 -->"
	head := marker + "\n\n## Human sign-off required (irreversible) — x/01\n\n**T**\n\n" +
		"> **Why you're being asked to sign off:** " + strings.Repeat("<details>", 6000) + "\n\n" +
		"> **Why this work exists:** " + strings.Repeat("x", 7000) + "\n\n"
	tail := "### Before you close\n\n- [ ] I accept this brief as done\n"
	for name, h := range map[string]string{
		"tags":             head,
		"tags and a fence": strings.Replace(head, "**T**", "**T**\n\n"+strings.Repeat("`", 3000)+"x", 1),
	} {
		t.Run(name, func(t *testing.T) {
			got := capIssueBody(h, []string{"### Verify\n\nrow\n\n"}, tail, ibLinkFn)
			if n := forgeBodyLen(got); n > issueBodyCap {
				t.Fatalf("capped body is %d, over the cap %d", n, issueBodyCap)
			}
			if !strings.HasPrefix(got, marker+"\n") || strings.Count(got, marker) != 1 {
				t.Errorf("the marker must be the first line, once; body starts %q", got[:min(len(got), 120)])
			}
			if !strings.Contains(got, issueBodyCutMark+" this issue body was cut") {
				t.Error("the backstop cut must be stated")
			}
			if len(got) < 1000 {
				t.Errorf("body is only %d bytes: the cut went above the header", len(got))
			}
		})
	}
}

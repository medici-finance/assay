package main

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// orderingReferenceMatch preserves the pre-optimization ordered matching path.
// The filter must preserve both priority and the exact byte span, not just whether
// some gate exists: the span separates the two dependency-reference sets.
func orderingReferenceMatch(line string) (loc, size int) {
	for _, rule := range orderingGateNegations {
		if rule.MatchString(line) {
			return -1, 0
		}
	}
	for _, rule := range orderingGateLexicon {
		if m := rule.FindStringIndex(line); m != nil {
			return m[0], m[1] - m[0]
		}
	}
	return -1, 0
}

// regression: #4360 — ordering-filter-coverage — a rejection filter must never suppress
// a phrase accepted by any original rule, including Unicode simple-fold matches.
func TestOrderingGateFilterParity(t *testing.T) {
	lines := []string{
		"plain prose names brief-01 with no prerequisite",
		"brief-01 starts before brief-02", "brief-01 STARTS\tBEFORE brief-02",
		"brief-01 ſtarts before brief-02", "brief-01 should land before brief-02",
		"brief-01 has to land before brief-02", "brief-01 needs to land before brief-02",
		"brief-01 landed before brief-02", "brief-01 before which lands brief-02",
		"brief-01 blocked on brief-02", "brief-01 gated on brief-02", "brief-01 waits on brief-02",
		"brief-01 depends on brief-02", "brief-01 never runs until brief-02",
		"brief-01 gates pods/02", "brief-01 gates #42", "brief-01 gate brief 02",
		"[BLOCKED: fixture] brief-01", "brief-01 is not blocked on brief-02",
		"brief-01 never blocks brief-02 but starts before brief-03",
		"brief-01 does not block brief-02 but is blocked on brief-03",
		"brief-01 cannot block brief-02", "brief-01 won't block brief-02",
		// First lexicon rule wins even when another rule matches earlier in the line.
		"brief-01 is blocked on brief-02 and starts before brief-03",
		"brief-01 gates #42 and waits on brief-02 and lands before brief-03",
		"", string([]byte{0xff, 0xfe}) + " brief-01 STARTS before brief-02",
	}
	err := filepath.WalkDir("testdata", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		b, e := os.ReadFile(path)
		if e != nil {
			return e
		}
		lines = append(lines, strings.Split(string(b), "\n")...)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range lines {
		a, b := orderingReferenceMatch(line)
		c, d := orderingGateMatch(line)
		if a != c || b != d {
			t.Fatalf("match changed for %q: reference %d/%d filtered %d/%d", line, a, b, c, d)
		}
	}
	// The union builder consumes all rules, rather than a separate hand-maintained
	// token list. This assertion also catches accidentally compiling only rule 0.
	for _, rule := range orderingGateLexicon {
		if !strings.Contains(orderingGateCandidate.full.String(), rule.String()) {
			t.Fatalf("union omitted %q", rule)
		}
	}
}

// regression: #4360 — ordering-filter-priority — union alternatives must not select the
// earliest phrase; the original lexicon's priority owns the reference split.
func TestOrderingGateFilterPriority(t *testing.T) {
	line := "brief-01 is blocked on brief-02 and starts before brief-03"
	loc, size := orderingGateMatch(line)
	if loc != strings.Index(line, "starts before") || size != len("starts before") {
		t.Fatalf("lexicon priority changed: %d/%d", loc, size)
	}
}

// regression: #4360 — ordering-filter-control-preservation — waiver/fence handling,
// negation, ordered reference splitting and same-process edits stay unchanged.
func TestOrderingGateFilterControlsAndEdit(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "docs", "streams", "pods")
	if e := os.MkdirAll(dir, 0755); e != nil {
		t.Fatal(e)
	}
	writeBrief(t, dir, "01", nil, nil)
	writeBrief(t, dir, "02", nil, nil)
	s := []*Stream{{Name: "pods", Dir: dir}}
	readme := filepath.Join(dir, "README.md")
	body := "# pods\nbrief-01 starts before brief-02\nbrief-01 is not blocked on brief-02\nbrief-01 starts before brief-02 <!-- graph: not-a-gate -->\n<!-- graph: derived:begin -->\nbrief-01 starts before brief-02\n<!-- graph: derived:end -->\n<!-- graph: -->\n"
	if e := os.WriteFile(readme, []byte(body), 0644); e != nil {
		t.Fatal(e)
	}
	notices := orderingGateNotices(s)
	if len(notices) != 2 || !strings.Contains(notices[0], "README.md:2") || !strings.Contains(notices[1], "bare") {
		t.Fatalf("controls changed: %v", notices)
	}
	body = strings.Replace(body, "brief-01 starts before brief-02\n", "ordinary prose without a prerequisite\n", 1)
	if e := os.WriteFile(readme, []byte(body), 0644); e != nil {
		t.Fatal(e)
	}
	after := orderingGateNotices(s)
	if len(after) != 1 || !strings.Contains(after[0], "bare") {
		t.Fatalf("stale answer after edit: %v", after)
	}
	// Isolate candidate filter from the policy caller: every union alternative is
	// compiled through the existing regexp implementation, including scoped flags.
	got := orderingPatternUnion([]*regexp.Regexp{regexp.MustCompile("(?i)before"), regexp.MustCompile("(?i)until")})
	if !got.MatchString("UNTIL") || got.MatchString("ordinary") {
		t.Fatal("pattern union lost an alternative")
	}
	if reflect.DeepEqual(notices, after) {
		t.Fatal("same-process source edit was not observed")
	}
}

// regression: #4360 — ordering-filter-fallback — unsupported/nullable regex syntax and
// non-ASCII prose retain the original regexp as the authority.
func TestOrderingGateFilterFallback(t *testing.T) {
	patterns := []string{"(?i)starts?", "(?:start|wait)", "(?:start|)", "a*", "[ab]gate", ".*gate", "(start)", "^\\bstart", "(?i)ſtart", "", "(?m)$", "\\p{L}gate"}
	lines := []string{"", "ordinary prose", "start", "START", "ſtart", "wait", "ag ate", "bgate", "prefix\ngate", "Kgate", "é gate", string([]byte{0xff}) + "gate"}
	for _, pattern := range patterns {
		original := regexp.MustCompile(pattern)
		candidate := newOrderingCandidateFilter([]*regexp.Regexp{original})
		for _, line := range lines {
			if original.MatchString(line) && !candidate.MatchString(line) {
				t.Fatalf("false rejection pattern=%q line=%q", pattern, line)
			}
		}
	}
	if !orderingGateCandidate.ascii || len(orderingGateCandidate.prefixes) == 0 {
		t.Fatal("current lexicon must use derived literal prefixes")
	}
	if orderingGateCandidate.MatchString("ordinary prose without any prerequisite vocabulary") {
		t.Fatal("ordinary ASCII prose was not rejected")
	}
	// Every supported rule's matching ASCII example is also tested after arbitrary
	// Unicode/malformed prefixes and suffixes. Unicode simple folding must use the
	// original union rather than strings.ToLower as an authority.
	for _, gate := range []string{"starts before", "blocked on", "gates brief-01", "[BLOCKED:", "ſtarts before"} {
		for _, prefix := range []string{"", "é ", "K ", string([]byte{0xff}) + " "} {
			line := prefix + gate + " brief-02"
			if orderingGateCandidate.full.MatchString(line) && !orderingGateCandidate.MatchString(line) {
				t.Fatalf("Unicode fallback dropped %q", line)
			}
		}
	}
}

func BenchmarkOrderingGateMatch(b *testing.B) {
	lines := []string{
		"This is ordinary descriptive prose about the architecture and its existing controls.",
		"The implementation records a fixture, an exact row identity and an immutable source.",
		"brief-01 is blocked on brief-02 and starts before brief-03",
		"brief-01 is not blocked on brief-02",
		"Future migration keeps the observer, trust checks and native reporting unchanged.",
	}
	for _, tc := range []struct {
		name  string
		match func(string) (int, int)
	}{{"reference", orderingReferenceMatch}, {"filtered", orderingGateMatch}} {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				tc.match(lines[i%len(lines)])
			}
		})
	}
}

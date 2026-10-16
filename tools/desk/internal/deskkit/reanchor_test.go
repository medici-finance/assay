package deskkit

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestReviewPredatesHead(t *testing.T) {
	head := time.Date(2026, 10, 15, 21, 35, 48, 0, time.UTC)
	cases := []struct {
		name      string
		submitted string
		head      time.Time
		want      bool
	}{
		{"re-anchored: submitted before the head commit existed", "2026-10-15T14:41:06Z", head, true},
		{"submitted after the head commit", "2026-10-15T22:05:00Z", head, false},
		{"submitted at the head commit's date counts as at head", "2026-10-15T21:35:48Z", head, false},
		{"head date unread fails closed", "2026-10-15T22:05:00Z", time.Time{}, true},
		{"empty submitted_at fails closed", "", head, true},
		{"unparseable submitted_at fails closed", "yesterday", head, true},
	}
	for _, c := range cases {
		if got := ReviewPredatesHead(c.submitted, c.head); got != c.want {
			t.Errorf("%s: ReviewPredatesHead(%q) = %v, want %v", c.name, c.submitted, got, c.want)
		}
	}
}

func TestParseCommitDate(t *testing.T) {
	if !ParseCommitDate("").IsZero() || !ParseCommitDate("not a date").IsZero() {
		t.Fatal("an empty or unparseable commit date must parse to the zero time (could-not-check)")
	}
	if got := ParseCommitDate("2026-10-15T21:35:48Z"); !got.Equal(time.Date(2026, 10, 15, 21, 35, 48, 0, time.UTC)) {
		t.Fatalf("ParseCommitDate = %v", got)
	}
}

// The open-change read carries the head commit's committer date on the node the rollup
// already comes from, and binds it to the head by oid: a date read off any other commit is
// left empty (the consumer then reads it per change or fails closed).
func TestOpenChangeHeadCommittedAtBoundToHead(t *testing.T) {
	for name, q := range map[string]string{"ghOpenChangesQuery": ghOpenChangesQuery, "ghReviewQueueQuery": ghReviewQueueQuery} {
		if !strings.Contains(q, "commits(last:1){nodes{commit{oid committedDate statusCheckRollup{") {
			t.Errorf("%s does not select the head commit's oid + committedDate\nquery: %s", name, q)
		}
	}
	node := func(oid string) ghOpenChangeNode {
		var n ghOpenChangeNode
		js := `{"headRefOid":"bbbb","commits":{"nodes":[{"commit":{"oid":"` + oid +
			`","committedDate":"2026-10-15T21:35:48Z","statusCheckRollup":null}}]}}`
		if err := json.Unmarshal([]byte(js), &n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if got := ghOpenChange(node("bbbb")).HeadCommittedAt; got != "2026-10-15T21:35:48Z" {
		t.Errorf("head-bound commit date = %q, want the served date", got)
	}
	if got := ghOpenChange(node("aaaa")).HeadCommittedAt; got != "" {
		t.Errorf("a date read off a non-head commit = %q, want empty (unbound)", got)
	}
}

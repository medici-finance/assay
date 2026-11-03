package schema

import (
	"strings"
	"testing"
)

// baseCell is a minimal valid cell body the answer_as / allow_anonymous_read
// tests extend, so each asserts exactly one new rule.
func cellDoc(extra string) string {
	return `
cells:
  - name: assay
    repos: [example-org/toolkit]
    streams_root: docs/streams
    loops:
      - role: metrics
        schedule: "0 * * * *"
        models: []
        app: assay-desk-app
    silence_threshold: 48h
    notification_tiers: {review: push, question: badge, notify: none}
` + extra
}

// TestAnswerAsDefault: absent answer_as resolves to the demoted default.
func TestAnswerAsDefault(t *testing.T) {
	f, err := Parse([]byte(cellDoc("")))
	if err != nil {
		t.Fatal(err)
	}
	if got := f.Cells[0].AnswerAsLogin(); got != DefaultAnswerAs() {
		t.Fatalf("default answer_as = %q, want %q", got, DefaultAnswerAs())
	}
	if f.Cells[0].AllowAnonymousRead {
		t.Fatal("allow_anonymous_read must default to false (deny-by-default)")
	}
}

// TestAnswerAsAccepted: a valid login is accepted and resolved.
func TestAnswerAsAccepted(t *testing.T) {
	f, err := Parse([]byte(cellDoc("    answer_as: some-operator\n    allow_anonymous_read: true\n")))
	if err != nil {
		t.Fatal(err)
	}
	if got := f.Cells[0].AnswerAsLogin(); got != "some-operator" {
		t.Fatalf("answer_as = %q, want some-operator", got)
	}
	if !f.Cells[0].AllowAnonymousRead {
		t.Fatal("allow_anonymous_read: true did not parse")
	}
}

// TestAnswerAsRejectsBadLogin: a non-login value is refused.
func TestAnswerAsRejectsBadLogin(t *testing.T) {
	for _, bad := range []string{"has space", "-leadinghyphen", "bad_underscore", strings.Repeat("x", 40)} {
		if _, err := Parse([]byte(cellDoc("    answer_as: \"" + bad + "\"\n"))); err == nil {
			t.Fatalf("answer_as %q should be rejected", bad)
		}
	}
}

package main

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/medici-finance/assay/cellconfig/config/schema"
)

const fixture = `
cells:
  - name: demo
    repos: [example-org/example-repo, example-org/other-repo]
    streams_root: docs/streams
    loops:
      - role: review
        schedule: "*/10 * * * *"
        models: [cheap-model, strong-model]
        app: example-reviewer-app
      - role: metrics
        schedule: "0 * * * *"
        models: []
        app: example-metrics-app
    silence_threshold: 48h
    notification_tiers:
      review: push
      question: badge
      notify: none
`

func parse(t *testing.T) *schema.File {
	t.Helper()
	f, err := schema.Parse([]byte(fixture))
	if err != nil {
		t.Fatalf("fixture does not validate against the landed schema: %v", err)
	}
	return f
}

func TestResolveEmitsTheLadderInOrder(t *testing.T) {
	out, err := Resolve(parse(t), "demo", "review", "ASSAY_")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	for _, want := range []string{
		"ASSAY_CELL='demo'",
		"ASSAY_ROLE='review'",
		"ASSAY_SCHEDULE='*/10 * * * *'",
		"ASSAY_MODELS='cheap-model strong-model'",
		"ASSAY_MODEL='cheap-model'",
		"ASSAY_APP='example-reviewer-app'",
		"ASSAY_REPOS='example-org/example-repo example-org/other-repo'",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\ngot:\n%s", want, out)
		}
	}
}

// The zero-AI loop (metrics) is the one role allowed an empty ladder.
// The entrypoint branches on ASSAY_MODEL being empty to decide "deterministic
// tick, no model call", so an empty ladder must render as an empty value rather
// than being omitted — an omitted line leaves a STALE value in the shell.
func TestResolveRendersAnEmptyLadderAsAnEmptyValue(t *testing.T) {
	out, err := Resolve(parse(t), "demo", "metrics", "ASSAY_")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !strings.Contains(out, "ASSAY_MODELS=''") || !strings.Contains(out, "ASSAY_MODEL=''") {
		t.Errorf("empty ladder not rendered as an empty assignment\ngot:\n%s", out)
	}
}

func TestResolveNamesTheAlternativesOnAMiss(t *testing.T) {
	f := parse(t)
	if _, err := Resolve(f, "nope", "review", "ASSAY_"); err == nil {
		t.Fatal("expected an error for an unknown cell")
	} else if !strings.Contains(err.Error(), "demo") {
		t.Errorf("unknown-cell error should name the cells the file defines, got: %v", err)
	}
	if _, err := Resolve(f, "demo", "dispatch", "ASSAY_"); err == nil {
		t.Fatal("expected an error for a role the cell does not run")
	} else if !strings.Contains(err.Error(), "review") || !strings.Contains(err.Error(), "metrics") {
		t.Errorf("unknown-role error should name the roles the cell runs, got: %v", err)
	}
}

// Values reach a pod's shell through `eval`, so quoting is a security property,
// not cosmetics. This asserts the rendered assignment survives a real shell
// round-trip with the value intact and nothing executed.
func TestResolveOutputIsEvalSafe(t *testing.T) {
	got := shellQuote(`a'b; touch /tmp/loopresolve-should-not-exist $(id)`)
	out, err := exec.Command("sh", "-c", "printf %s "+got).Output()
	if err != nil {
		t.Fatalf("shell rejected the quoted word %s: %v", got, err)
	}
	if string(out) != `a'b; touch /tmp/loopresolve-should-not-exist $(id)` {
		t.Errorf("quoted word did not round-trip through sh: %q", string(out))
	}
}

// entrypoint.sh evals loopresolve's stdout wholesale. If a future field were
// emitted unquoted, THAT is the line an injection rides in on — so assert the
// whole document is quoted, not just the one value above.
func TestEveryEmittedValueIsQuoted(t *testing.T) {
	out, err := Resolve(parse(t), "demo", "review", "ASSAY_")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		_, value, ok := strings.Cut(line, "=")
		if !ok {
			t.Errorf("line is not an assignment: %q", line)
			continue
		}
		if !strings.HasPrefix(value, "'") || !strings.HasSuffix(value, "'") {
			t.Errorf("value is not single-quoted, so `eval` would word-split it: %q", line)
		}
	}
}

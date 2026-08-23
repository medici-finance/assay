package schema

import "testing"

const validDoc = `
cells:
  - name: demo
    repos:
      - example-org/example-repo
    streams_root: docs/streams
    loops:
      - role: review
        schedule: "*/10 * * * *"
        models: [cheap-model, strong-model]
        app: example-reviewer-app
      - role: metrics
        schedule: "0 * * * *"
        models: []
        app: example-desk-app
    silence_threshold: 48h
    notification_tiers:
      review: push
      question: badge
      notify: none
`

func TestParse_Valid(t *testing.T) {
	f, err := Parse([]byte(validDoc))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(f.Cells) != 1 || f.Cells[0].Name != "demo" {
		t.Fatalf("unexpected result: %+v", f)
	}
}

func TestParse_UnknownField(t *testing.T) {
	doc := validDoc + "    unknown_field: nope\n"
	if _, err := Parse([]byte(doc)); err == nil {
		t.Fatal("expected an error for an unknown field, got nil")
	}
}

func TestParse_MissingModelsForAIRole(t *testing.T) {
	doc := `
cells:
  - name: demo
    repos: [example-org/example-repo]
    streams_root: docs/streams
    loops:
      - role: review
        schedule: "*/10 * * * *"
        models: []
        app: example-reviewer-app
    silence_threshold: 48h
    notification_tiers: {review: push, question: badge, notify: none}
`
	if _, err := Parse([]byte(doc)); err == nil {
		t.Fatal("expected an error for empty model ladder on a non-metrics role")
	}
}

func TestParse_UnknownRole(t *testing.T) {
	doc := `
cells:
  - name: demo
    repos: [example-org/example-repo]
    streams_root: docs/streams
    loops:
      - role: inbound
        schedule: "*/10 * * * *"
        models: [cheap-model]
        app: example-desk-app
    silence_threshold: 48h
    notification_tiers: {review: push, question: badge, notify: none}
`
	if _, err := Parse([]byte(doc)); err == nil {
		t.Fatal("expected an error for an unrecognised loop role")
	}
}

func TestParse_IncompleteTriad(t *testing.T) {
	doc := `
cells:
  - name: demo
    repos: [example-org/example-repo]
    streams_root: docs/streams
    loops:
      - role: review
        schedule: "*/10 * * * *"
        models: [cheap-model]
        app: example-reviewer-app
    silence_threshold: 48h
    notification_tiers: {review: push, question: badge}
`
	if _, err := Parse([]byte(doc)); err == nil {
		t.Fatal("expected an error for an incomplete notification triad")
	}
}

func TestParse_BadSilenceThreshold(t *testing.T) {
	doc := `
cells:
  - name: demo
    repos: [example-org/example-repo]
    streams_root: docs/streams
    loops:
      - role: review
        schedule: "*/10 * * * *"
        models: [cheap-model]
        app: example-reviewer-app
    silence_threshold: not-a-duration
    notification_tiers: {review: push, question: badge, notify: none}
`
	if _, err := Parse([]byte(doc)); err == nil {
		t.Fatal("expected an error for an unparseable silence_threshold")
	}
}

func TestParse_NoCells(t *testing.T) {
	if _, err := Parse([]byte("cells: []\n")); err == nil {
		t.Fatal("expected an error for zero cells")
	}
}

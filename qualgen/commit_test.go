package main

import (
	"encoding/json"
	"testing"
	"time"
)

// TestCommitJSONLRoundTrip proves a Commit marshals to one JSONL line and back
// without losing the parent set, raw identity, or the diff refs.
func TestCommitJSONLRoundTrip(t *testing.T) {
	in := Commit{
		SHA:            "abc123",
		ParentSHAs:     []string{"p1", "p2"},
		AuthorRaw:      "A U Thor <a@example.test>",
		AuthorName:     "A U Thor",
		AuthorEmail:    "a@example.test",
		AuthorWhen:     time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
		CommitterRaw:   "C Mitter <c@example.test>",
		CommitterName:  "C Mitter",
		CommitterEmail: "c@example.test",
		CommitterWhen:  time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC),
		Message:        "do a thing",
		FileDiffKeys:   []string{"abc123:a.go", "abc123:b.go"},
	}
	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out Commit
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.SHA != in.SHA || len(out.ParentSHAs) != 2 || out.AuthorRaw != in.AuthorRaw {
		t.Fatalf("round-trip lost data: %+v", out)
	}
	if len(out.FileDiffKeys) != 2 || out.FileDiffKeys[0] != "abc123:a.go" {
		t.Fatalf("diff refs lost: %+v", out.FileDiffKeys)
	}
	if !out.IsMerge() {
		t.Error("two parents should read as a merge")
	}
	if (Commit{}).IsRoot() != true {
		t.Error("a parentless commit should read as root")
	}
}

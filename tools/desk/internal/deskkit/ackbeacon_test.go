package deskkit

import (
	"encoding/json"
	"os"
	"testing"
)

// ackbeacon_test.go — AppendAck writes a receipt to a session's roster beacon WITHOUT
// clobbering the fields deskroster owns on that same file.

// TestAppendAckCreatesAndAppends — a first ack on a session with no beacon creates one
// carrying the record; a second appends rather than replaces.
func TestAppendAckCreatesAndAppends(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if _, err := AppendAck("s1", AckRecord{Role: "worker-desk", Repo: "at", Restatement: "first message"}); err != nil {
		t.Fatalf("AppendAck 1: %v", err)
	}
	if _, err := AppendAck("s1", AckRecord{Role: "worker-desk", Repo: "at", Restatement: "second message"}); err != nil {
		t.Fatalf("AppendAck 2: %v", err)
	}

	path, _ := AckBeaconPath("s1")
	var obj struct {
		Acks []AckRecord `json:"acks"`
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read beacon: %v", err)
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatalf("parse beacon: %v", err)
	}
	if len(obj.Acks) != 2 {
		t.Fatalf("acks = %d, want 2 (append, not replace): %s", len(obj.Acks), data)
	}
	if obj.Acks[0].Restatement != "first message" || obj.Acks[1].Restatement != "second message" {
		t.Fatalf("acks out of order or wrong: %+v", obj.Acks)
	}
	if obj.Acks[0].TS == "" {
		t.Errorf("AppendAck must stamp a ts when the caller left it blank: %+v", obj.Acks[0])
	}
}

// TestAppendAckPreservesForeignFields is the load-bearing one: deskroster keeps
// session/role/updated/open_work on the SAME file. AppendAck must round-trip every one of
// those untouched — a receipt append that dropped a session's open-work would corrupt the
// roster.
func TestAppendAckPreservesForeignFields(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	path, _ := AckBeaconPath("s2")
	if err := os.MkdirAll(dirOf(path), 0o700); err != nil {
		t.Fatal(err)
	}
	// A beacon deskroster wrote: fields deskkit's AckRecord knows nothing about.
	seed := `{
  "session": "s2",
  "role": "worker-desk",
  "updated": "2026-09-06T00:00:00Z",
  "open_work": [{"repo": "at", "pr": 42, "what": "brief x"}]
}`
	if err := os.WriteFile(path, []byte(seed), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := AppendAck("s2", AckRecord{Role: "worker-desk", Repo: "at", Restatement: "a receipt"}); err != nil {
		t.Fatalf("AppendAck: %v", err)
	}

	var obj map[string]json.RawMessage
	data, _ := os.ReadFile(path)
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatalf("parse merged beacon: %v", err)
	}
	for _, k := range []string{"session", "role", "updated", "open_work", "acks"} {
		if _, ok := obj[k]; !ok {
			t.Errorf("AppendAck dropped the %q field — a receipt write must not clobber deskroster's fields: %s", k, data)
		}
	}
	// open_work's DATA must survive intact (formatting may be re-indented, the content
	// may not change).
	var work []struct {
		Repo string `json:"repo"`
		PR   int    `json:"pr"`
		What string `json:"what"`
	}
	if err := json.Unmarshal(obj["open_work"], &work); err != nil {
		t.Fatalf("open_work no longer parses: %v", err)
	}
	if len(work) != 1 || work[0].Repo != "at" || work[0].PR != 42 || work[0].What != "brief x" {
		t.Errorf("open_work data was altered: %+v", work)
	}
}

// TestAppendAckMalformedBeaconFailsClosed — a corrupt beacon is NOT silently overwritten
// (that would lose whatever a human or another writer put there); it is an error.
func TestAppendAckMalformedBeaconFailsClosed(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path, _ := AckBeaconPath("s3")
	if err := os.MkdirAll(dirOf(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := AppendAck("s3", AckRecord{Role: "worker-desk", Restatement: "x"}); err == nil {
		t.Fatal("AppendAck silently overwrote a malformed beacon — it must fail closed")
	}
}

func dirOf(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' {
			return p[:i]
		}
	}
	return "."
}

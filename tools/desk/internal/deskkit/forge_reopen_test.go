package deskkit

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// forge_reopen_test.go — ReopenIssue on both backends (forge-neutral brief 16, Task 1).
//
// The golden corpora pin the exact wire byte-for-byte; this test states the CONTRACT in
// prose a reviewer can read without opening a fixture: on each backend the op emits exactly
// ONE request, at the ISSUE endpoint, carrying the reopen state and NO state reason. A reopen
// that emitted a note, touched the merge-request sequence, or carried a reason would be a
// different op wearing this one's name.

func TestReopenIssueOpBothBackends(t *testing.T) {
	t.Run("github", func(t *testing.T) {
		s := newGoldenServer(t)
		s.issue = map[string]any{"number": 33, "state": "open"}
		if err := s.forge().ReopenIssue(forgeTestRepo, 33); err != nil {
			t.Fatalf("ReopenIssue: %v", err)
		}
		if len(s.requests) != 1 {
			t.Fatalf("want exactly 1 request, got %d: %+v", len(s.requests), s.requests)
		}
		r := s.requests[0]
		if r.Method != http.MethodPatch || !strings.HasSuffix(r.Path, "/issues/33") {
			t.Fatalf("want PATCH …/issues/33, got %s %s", r.Method, r.Path)
		}
		var body map[string]any
		if err := json.Unmarshal(r.Body, &body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body["state"] != "open" {
			t.Fatalf("want state:open, got %v", body)
		}
		if _, has := body["state_reason"]; has {
			t.Fatalf("a reopen must carry no state_reason (a close-time field), got %v", body)
		}
	})

	t.Run("gitlab", func(t *testing.T) {
		s := newGLServer(t)
		s.issue = glIssue(map[string]any{"iid": 33, "state": "opened"})
		if err := s.forge().ReopenIssue(glRepo, 33); err != nil {
			t.Fatalf("ReopenIssue: %v", err)
		}
		if len(s.requests) != 1 {
			t.Fatalf("want exactly 1 request (no note, no second write), got %d: %+v", len(s.requests), s.requests)
		}
		r := s.requests[0]
		if r.Method != http.MethodPut || !strings.HasSuffix(r.Path, "/issues/33") {
			t.Fatalf("want PUT …/issues/33 (the ISSUE sequence, never merge_requests), got %s %s", r.Method, r.Path)
		}
		if strings.Contains(r.Path, "merge_requests") {
			t.Fatalf("reopen addressed the merge-request sequence: %s", r.Path)
		}
		var body map[string]any
		if err := json.Unmarshal(r.Body, &body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body["state_event"] != "reopen" {
			t.Fatalf("want state_event:reopen, got %v", body)
		}
	})
}

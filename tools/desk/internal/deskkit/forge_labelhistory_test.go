package deskkit

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// forge_labelhistory_test.go — the label-removal hold brief: the actor id on LabelEvent and the
// IssueLabelHistory read (op 64).

// ghLabelPage serves one full page (100) of labeled events with a fixed actor, for a walk that
// never ends.
func ghFullLabelPage() []map[string]any {
	out := make([]map[string]any, forgeFilePerPage)
	for i := range out {
		out[i] = map[string]any{"event": "labeled", "label": map[string]any{"name": "needs-decision"},
			"actor": map[string]any{"login": "u", "id": 5}}
	}
	return out
}

func glFullLabelPage() []map[string]any {
	out := make([]map[string]any, gitlabPerPage)
	for i := range out {
		out[i] = map[string]any{"id": i + 1, "action": "add", "user": map[string]any{"id": 5, "username": "u"},
			"label": map[string]any{"id": 9, "name": "needs-decision"}}
	}
	return out
}

// regression: #2414 — a login alone cannot be pinned to an identity.
func TestLabelEventsCarryActorID(t *testing.T) {
	t.Run("github", func(t *testing.T) {
		s := newGoldenServer(t)
		s.timeline = []map[string]any{
			{"event": "labeled", "label": map[string]any{"name": "a"}, "actor": map[string]any{"login": "desk[bot]", "id": 4242}},
			{"event": "unlabeled", "label": map[string]any{"name": "a"}, "actor": map[string]any{"login": "ian", "id": 20959}},
			{"event": "unlabeled", "label": map[string]any{"name": "b"}},
		}
		f := &GitHubForge{Token: "stub", BaseURL: s.srv.URL}
		for _, read := range []struct {
			name string
			fn   func() ([]LabelEvent, error)
		}{
			{"ListLabelEvents", func() ([]LabelEvent, error) { return f.ListLabelEvents(forgeTestRepo, 7) }},
			{"ListIssueLabelEvents", func() ([]LabelEvent, error) { return f.ListIssueLabelEvents(forgeTestRepo, 7) }},
		} {
			got, err := read.fn()
			if err != nil {
				t.Fatalf("%s: %v", read.name, err)
			}
			if len(got) != 3 || got[0].AppliedBy != "desk[bot]" || got[0].AppliedByID != 4242 || got[0].Removed ||
				got[1].AppliedBy != "ian" || got[1].AppliedByID != 20959 || !got[1].Removed ||
				got[2].AppliedBy != "" || got[2].AppliedByID != 0 {
				t.Errorf("%s: events = %+v", read.name, got)
			}
		}
	})
	t.Run("gitlab", func(t *testing.T) {
		s := newGLServer(t)
		s.labelEvents = []map[string]any{
			{"id": 1, "action": "add", "user": map[string]any{"id": 42, "username": "desk-bot"}, "label": map[string]any{"name": "a"}},
			{"id": 2, "action": "remove", "user": map[string]any{"id": 5, "username": "ian"}, "label": map[string]any{"name": "a"}},
			{"id": 3, "action": "add", "label": map[string]any{"name": "b"}},
		}
		f := s.forge()
		for _, read := range []struct {
			name string
			fn   func() ([]LabelEvent, error)
		}{
			{"ListLabelEvents", func() ([]LabelEvent, error) { return f.ListLabelEvents(glRepo, 7) }},
			{"ListIssueLabelEvents", func() ([]LabelEvent, error) { return f.ListIssueLabelEvents(glRepo, 7) }},
		} {
			got, err := read.fn()
			if err != nil {
				t.Fatalf("%s: %v", read.name, err)
			}
			if len(got) != 3 || got[0].AppliedBy != "desk-bot" || got[0].AppliedByID != 42 || got[0].Removed ||
				got[1].AppliedBy != "ian" || got[1].AppliedByID != 5 || !got[1].Removed ||
				got[2].AppliedBy != "" || got[2].AppliedByID != 0 {
				t.Errorf("%s: events = %+v", read.name, got)
			}
		}
	})
}

// regression: #2414 — op 64 reports what the existing label-event reads drop or error on.
func TestIssueLabelHistory(t *testing.T) {
	t.Run("github named events carry login and id", func(t *testing.T) {
		s := newGoldenServer(t)
		s.timeline = []map[string]any{
			{"event": "labeled", "created_at": "2026-10-01T10:00:00Z", "label": map[string]any{"name": "needs-decision"}, "actor": map[string]any{"login": "desk[bot]", "id": 4242}},
			{"event": "unlabeled", "created_at": "2026-10-02T10:00:00Z", "label": map[string]any{"name": "needs-decision"}, "actor": map[string]any{"login": "desk[bot]", "id": 4242}},
		}
		h, err := (&GitHubForge{Token: "stub", BaseURL: s.srv.URL}).IssueLabelHistory(forgeTestRepo, 7)
		if err != nil || !h.Complete || len(h.Events) != 2 {
			t.Fatalf("history = %+v, err = %v", h, err)
		}
		if e := h.Events[1]; e.Name != "needs-decision" || e.AppliedBy != "desk[bot]" || e.AppliedByID != 4242 || !e.Removed || e.CreatedAt != "2026-10-02T10:00:00Z" {
			t.Errorf("event = %+v", e)
		}
	})
	t.Run("gitlab deleted-label event kept where the existing read drops it", func(t *testing.T) {
		s := newGLServer(t)
		s.labelEvents = []map[string]any{
			{"id": 1, "action": "add", "created_at": "2026-10-01T10:00:00Z", "user": map[string]any{"id": 42, "username": "desk-bot"}, "label": map[string]any{"id": 9, "name": "needs-decision"}},
			{"id": 2, "action": "remove", "created_at": "2026-10-02T10:00:00Z", "user": map[string]any{"id": 43, "username": "x"}, "label": map[string]any{"id": 0, "name": ""}},
		}
		f := s.forge()
		h, err := f.IssueLabelHistory(glRepo, 7)
		if err != nil || !h.Complete || len(h.Events) != 2 {
			t.Fatalf("history = %+v, err = %v", h, err)
		}
		if e := h.Events[0]; e.Name != "needs-decision" || e.AppliedBy != "desk-bot" || e.AppliedByID != 42 || e.Removed {
			t.Errorf("named event = %+v", e)
		}
		if e := h.Events[1]; e.Name != "" || e.AppliedByID != 43 || !e.Removed {
			t.Errorf("unnamed event = %+v", e)
		}
		old, err := f.ListIssueLabelEvents(glRepo, 7)
		if err != nil || len(old) != 1 {
			t.Errorf("ListIssueLabelEvents must keep dropping the unnamed event: %+v, err = %v", old, err)
		}
	})
	t.Run("github page bound reports incomplete, not an error and not a complete list", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(ghFullLabelPage())
		}))
		defer srv.Close()
		f := &GitHubForge{Token: "stub", BaseURL: srv.URL}
		h, err := f.IssueLabelHistory(forgeTestRepo, 7)
		if err != nil {
			t.Fatalf("a page bound must be reported, not an error: %v", err)
		}
		if h.Complete {
			t.Errorf("Complete = true past the page bound")
		}
		if _, err := f.ListIssueLabelEvents(forgeTestRepo, 7); err == nil {
			t.Errorf("ListIssueLabelEvents must still error past the page bound")
		}
	})
	t.Run("gitlab page bound reports incomplete", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Next-Page", "2")
			_ = json.NewEncoder(w).Encode(glFullLabelPage())
		}))
		defer srv.Close()
		f := &GitLabForge{Token: glTestToken, BaseURL: srv.URL, Client: srv.Client()}
		h, err := f.IssueLabelHistory(glRepo, 7)
		if err != nil {
			t.Fatalf("a page bound must be reported, not an error: %v", err)
		}
		if h.Complete || len(h.Events) == 0 {
			t.Errorf("history = complete %v, %d events", h.Complete, len(h.Events))
		}
	})
	t.Run("a failed request is an error on both forges", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, `{"message":"boom"}`, http.StatusForbidden)
		}))
		defer srv.Close()
		if h, err := (&GitHubForge{Token: "stub", BaseURL: srv.URL}).IssueLabelHistory(forgeTestRepo, 7); err == nil {
			t.Errorf("github: want error, got %+v", h)
		}
		if h, err := (&GitLabForge{Token: glTestToken, BaseURL: srv.URL, Client: srv.Client()}).IssueLabelHistory(glRepo, 7); err == nil {
			t.Errorf("gitlab: want error, got %+v", h)
		}
	})
}

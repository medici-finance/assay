package deskkit

import (
	"fmt"
	"net/http"
	"testing"
)

// regression: #5853
func TestChangesByBriefWalksPRs(t *testing.T) {
	gh := newB33GitHub(t, func(w http.ResponseWriter, r *http.Request, _ string) {
		q := r.URL.Query()
		if r.URL.Path != "/search/issues" || q.Get("q") != `repo:example-org/tracker is:pr "Brief: example-port/11" in:body` {
			t.Errorf("unexpected scope/query %s", r.URL)
		}
		fmt.Fprint(w, `{"total_count":3,"incomplete_results":false,"items":[`+
			`{"number":41,"state":"closed","body":"Brief: example-port/11","pull_request":{"merged_at":"2026-01-02T00:00:00Z"}},`+
			`{"number":42,"state":"closed","body":"Brief: example-port/11","pull_request":{"merged_at":null}},`+
			`{"number":43,"state":"open","body":"Brief: example-port/11","pull_request":{}}]}`)
	})
	lookup := ChangesByBriefReader(OutboundChecked(gh.forge(), "worker"))
	if lookup == nil {
		t.Fatal("a GitHub forge behind the outbound wrapper has no brief-trailer lookup")
	}
	got, err := lookup(ForgeRepo{Owner: "example-org", Name: "tracker"}, "example-port/11")
	if err != nil || len(got) != 3 {
		t.Fatalf("lookup: %+v %v", got, err)
	}
	for i, want := range []string{"MERGED", "CLOSED", "OPEN"} {
		if got[i].State != want {
			t.Errorf("#%d state = %q, want %q", got[i].Number, got[i].State, want)
		}
	}
	if m := RepresentedBriefPRs(got)["example-port/11"]; m.Number != 41 || !m.Merged {
		t.Errorf("reduction = %+v, want merged #41", m)
	}
}

// regression: #5853
func TestChangesByBriefIncompleteErrs(t *testing.T) {
	for name, body := range map[string]string{
		"partial":  `{"total_count":1,"incomplete_results":true,"items":[{"number":1,"state":"open","body":"x","pull_request":{}}]}`,
		"issue":    `{"total_count":1,"incomplete_results":false,"items":[{"number":1,"state":"open","body":"x"}]}`,
		"ceiling":  `{"total_count":1001,"incomplete_results":false,"items":[]}`,
		"no_items": `{"total_count":2,"incomplete_results":false,"items":[]}`,
	} {
		t.Run(name, func(t *testing.T) {
			gh := newB33GitHub(t, func(w http.ResponseWriter, _ *http.Request, _ string) { fmt.Fprint(w, body) })
			if got, err := ChangesByBriefReader(gh.forge())(forgeTestRepo, "example-port/11"); err == nil || got != nil {
				t.Fatalf("a partial lookup became an answer: %+v %v", got, err)
			}
		})
	}
	gh := newB33GitHub(t, func(http.ResponseWriter, *http.Request, string) { t.Error("a refused id reached the forge") })
	if _, err := ChangesByBriefReader(gh.forge())(forgeTestRepo, `example-port/11" is:issue`); err == nil {
		t.Fatal("an id carrying search syntax was not refused")
	}
	if ChangesByBriefReader(&GitLabForge{}) != nil {
		t.Fatal("a backend without the search returned a reader")
	}
}

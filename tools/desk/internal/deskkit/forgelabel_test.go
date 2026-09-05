package deskkit

// forgelabel_test.go — the two cross-backend controls this brief's Verify table names:
//
//	TestLabelOpBothBackends       — the label operation runs the SAME scenarios against both
//	                                recorded backends, so a divergence is a NAMED failing
//	                                scenario rather than silence.
//	TestFlipRefusesUnsupportedForge — the ready flip refuses could-not-check, naming the forge
//	                                and the operation, when the resolved backend serves no
//	                                opaque change id — and writes NOTHING, asserted by a
//	                                recording transport that must show zero write calls.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// labelScenario is one label reconciliation, expressed once and run against BOTH backends.
// Its `want` is the forge-neutral outcome — the only thing a caller of the seam can see —
// so the two implementations are compared on what they PROMISE rather than on the requests
// they happen to emit (which the golden corpora pin separately, per backend).
type labelScenario struct {
	name   string
	change LabelChange
	// ghSetup / glSetup put each backend's recorded instance into the same STATE: a change
	// carrying `size:xl` and `keep-me`, and a project/repo that already has the labels.
	ghSetup func(s *goldenServer)
	glSetup func(s *glServer)
	want    LabelOutcome
}

func labelScenarios() []labelScenario {
	return []labelScenario{
		{
			// The family reconciliation: the stale same-family member goes, the unrelated
			// label stays, the current one is applied.
			name: "replaces_a_stale_family_member",
			change: LabelChange{
				Add:            []LabelSpec{{Name: "size:s", Color: "c5def5", Description: "size"}},
				RemoveFamilies: []string{"size:"},
			},
			ghSetup: func(s *goldenServer) {
				s.prLabels = []map[string]any{{"name": "size:xl"}, {"name": "keep-me"}}
			},
			glSetup: func(s *glServer) {
				s.mr = glMR(map[string]any{"labels": []string{"size:xl", "keep-me"}})
				s.updateMR = glMR(nil)
			},
			want: LabelOutcome{Added: []string{"size:s"}, Removed: []string{"size:xl"}},
		},
		{
			// A family the caller has NO definite value for is not named, so nothing in it
			// is touched. An absent signal removes nothing — the property deskpost's
			// three-state surface label depends on.
			name: "an_unnamed_family_is_left_alone",
			change: LabelChange{
				Add: []LabelSpec{{Name: "size:s", Color: "c5def5"}},
			},
			ghSetup: func(s *goldenServer) {
				s.prLabels = []map[string]any{{"name": "surface:core"}, {"name": "size:xl"}}
			},
			glSetup: func(s *glServer) {
				s.mr = glMR(map[string]any{"labels": []string{"surface:core", "size:xl"}})
				s.updateMR = glMR(nil)
			},
			want: LabelOutcome{Added: []string{"size:s"}},
		},
		{
			// The queue-label swap deskflip makes: one named removal, one addition.
			name: "swaps_one_named_label_for_another",
			change: LabelChange{
				Add:    []LabelSpec{{Name: "approval-needed", Color: "0e8a16"}},
				Remove: []string{"authorization-needed"},
			},
			ghSetup: func(s *goldenServer) {
				s.prLabels = []map[string]any{{"name": "authorization-needed"}}
			},
			glSetup: func(s *glServer) {
				s.mr = glMR(map[string]any{"labels": []string{"authorization-needed"}})
				s.updateMR = glMR(nil)
			},
			want: LabelOutcome{Added: []string{"approval-needed"}, Removed: []string{"authorization-needed"}},
		},
		{
			// A label named in BOTH halves is a caller bug, not an instruction to churn it:
			// neither backend may remove a label it is also applying.
			name: "a_label_in_both_halves_is_not_churned",
			change: LabelChange{
				Add:            []LabelSpec{{Name: "size:s", Color: "c5def5"}},
				Remove:         []string{"size:s"},
				RemoveFamilies: []string{"size:"},
			},
			ghSetup: func(s *goldenServer) {
				s.prLabels = []map[string]any{{"name": "size:s"}}
			},
			glSetup: func(s *glServer) {
				s.mr = glMR(map[string]any{"labels": []string{"size:s"}})
				s.updateMR = glMR(nil)
			},
			want: LabelOutcome{Added: []string{"size:s"}},
		},
	}
}

// TestLabelOpBothBackends runs each scenario against both recorded backends under the SAME
// subtest name.
//
// The point is the shared name. A per-backend test would report each implementation agreeing
// with its own expectations; running one scenario list through both makes a divergence show
// up as `TestLabelOpBothBackends/<scenario>/gitlab` failing beside a green `/github` — a
// named gap, which is what the brief asks for instead of "the GitLab mapping was
// approximated and nobody noticed".
func TestLabelOpBothBackends(t *testing.T) {
	for _, sc := range labelScenarios() {
		sc := sc
		t.Run(sc.name, func(t *testing.T) {
			t.Run("github", func(t *testing.T) {
				srv := newGoldenServer(t)
				sc.ghSetup(srv)
				got, err := srv.forge().ApplyLabels(forgeTestRepo, 7, sc.change)
				assertLabelOutcome(t, "github", got, err, sc.want)
			})
			t.Run("gitlab", func(t *testing.T) {
				srv := newGLServer(t)
				sc.glSetup(srv)
				got, err := srv.forge().ApplyLabels(glRepo, 7, sc.change)
				assertLabelOutcome(t, "gitlab", got, err, sc.want)
			})
		})
	}
}

func assertLabelOutcome(t *testing.T, backend string, got *LabelOutcome, err error, want LabelOutcome) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: ApplyLabels returned %v — the mapping is either unsupported (which must be a "+
			"could-not-check naming the gap) or broken; either way this scenario is not covered", backend, err)
	}
	if got == nil {
		t.Fatalf("%s: ApplyLabels returned no outcome and no error — a write whose result cannot be "+
			"reported is not a write a caller can audit", backend)
	}
	if !sameStrings(got.Added, want.Added) {
		t.Errorf("%s: applied %v, want %v", backend, got.Added, want.Added)
	}
	if !sameStrings(got.Removed, want.Removed) {
		t.Errorf("%s: removed %v, want %v", backend, got.Removed, want.Removed)
	}
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// writeRecorder is an httptest server that serves one canned pull-request payload and
// records EVERY request, classified into reads (GET) and writes (everything else). It exists
// so "performs NO write" can be asserted against the transport rather than inferred from an
// exit code — a refusal that returned the right code after issuing a POST would pass an
// exit-code-only assertion and fail this one.
type writeRecorder struct {
	srv    *httptest.Server
	reads  []string
	writes []string
}

func newWriteRecorder(t *testing.T, pull map[string]any) *writeRecorder {
	t.Helper()
	w := &writeRecorder{}
	w.srv = httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		line := r.Method + " " + r.URL.Path
		if r.Method == http.MethodGet {
			w.reads = append(w.reads, line)
		} else {
			w.writes = append(w.writes, line)
		}
		rw.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(rw).Encode(pull)
	}))
	t.Cleanup(w.srv.Close)
	return w
}

// TestFlipRefusesUnsupportedForge is the NEGATIVE path for the ready flip.
//
// The unsupported case is modelled as the thing that actually makes a backend unable to serve
// this operation: it returns a change carrying NO opaque id. MarkReadyForReview takes only
// that id, so a backend that mints none cannot be asked to perform the transition — and the
// only correct response is could-not-check naming the forge and the operation, with no write
// and no locally composed id.
func TestFlipRefusesUnsupportedForge(t *testing.T) {
	t.Run("refuses_could_not_check_and_writes_nothing", func(t *testing.T) {
		rec := newWriteRecorder(t, map[string]any{
			"number": 7, "state": "open", "draft": true,
			// node_id deliberately ABSENT — this is the backend that cannot serve the flip.
			"head": map[string]any{"sha": "abc123", "ref": "feat/x"},
			"user": map[string]any{"login": "worker[bot]", "id": 99},
		})
		f := &GitHubForge{Token: "test-token", BaseURL: rec.srv.URL, Client: rec.srv.Client()}
		res := ForgeResolution{Repo: forgeTestRepo, Kind: ForgeGitHub, Source: "repo-config:" + EnvRepoForges}

		pr, err := f.GetPullRequest(forgeTestRepo, 7)
		if err != nil {
			t.Fatalf("reading the change: %v", err)
		}
		err = ReadyFlip(res, f, pr)
		if err == nil {
			t.Fatal("the flip succeeded against a backend that serves no opaque change id — it must " +
				"refuse rather than compose one")
		}
		if code := ExitCodeOf(err); code != ExitUnverifiable {
			t.Errorf("exit %d, want %d (could-not-check): a backend that cannot serve the operation is "+
				"could-not-check, not a settled refusal", code, ExitUnverifiable)
		}
		msg := err.Error()
		for _, want := range []string{string(ForgeGitHub), "MarkReadyForReview", forgeTestRepo.Slug()} {
			if !strings.Contains(msg, want) {
				t.Errorf("the refusal does not name %q: %s", want, msg)
			}
		}
		if len(rec.writes) != 0 {
			t.Fatalf("the refusal issued %d write call(s): %v — a gate that refuses AFTER writing has "+
				"gated nothing", len(rec.writes), rec.writes)
		}
	})

	// The positive control, without which the negative one proves nothing: with an opaque id
	// present the SAME code path does perform the mutation.
	t.Run("performs_the_mutation_when_the_backend_serves_an_id", func(t *testing.T) {
		rec := newWriteRecorder(t, map[string]any{
			"number": 7, "state": "open", "draft": true, "node_id": "PR_node",
			"head": map[string]any{"sha": "abc123", "ref": "feat/x"},
			"user": map[string]any{"login": "worker[bot]", "id": 99},
		})
		f := &GitHubForge{Token: "test-token", BaseURL: rec.srv.URL, Client: rec.srv.Client()}
		res := ForgeResolution{Repo: forgeTestRepo, Kind: ForgeGitHub, Source: "repo-config:" + EnvRepoForges}

		pr, err := f.GetPullRequest(forgeTestRepo, 7)
		if err != nil {
			t.Fatalf("reading the change: %v", err)
		}
		if err := ReadyFlip(res, f, pr); err != nil {
			t.Fatalf("the flip refused a change that DOES carry an opaque id: %v", err)
		}
		if len(rec.writes) != 1 || !strings.HasSuffix(rec.writes[0], "/graphql") {
			t.Fatalf("writes were %v, want exactly one mutation — otherwise the negative case's "+
				"zero-write assertion is vacuous", rec.writes)
		}
	})

	// A nil change is not a licence to compose an id either.
	t.Run("refuses_when_there_is_no_change_to_flip", func(t *testing.T) {
		f := &GitHubForge{Token: "test-token"}
		res := ForgeResolution{Repo: forgeTestRepo, Kind: ForgeGitHub}
		err := ReadyFlip(res, f, nil)
		if err == nil || ExitCodeOf(err) != ExitUnverifiable {
			t.Fatalf("ReadyFlip(nil change) = %v, want a could-not-check refusal", err)
		}
	})
}

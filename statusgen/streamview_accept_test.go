package main

// Acceptance tests for the stream-view contract (docs/stream-view-contract.md),
// driven by tests/stream-brief/32.sh. Every assertion reads its expected value
// from testdata/streamview/expect.json and its source records from the
// fixture READMEs, so the runner's mutation case can corrupt either and watch
// the named assertion go red. A failing assertion prints ASSERT-FAIL[<id>] —
// the marker the runner keys on, so a build failure or a missing test can
// never be mistaken for an observed assertion.
//
// STREAMVIEW_FIXTURES overrides the fixture root (the runner points it at a
// mutated copy). Offline: no network, no forge; git runs only on temporary
// repositories the test creates.

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/statusgen/streamview"
)

// svObserved is the injected observation time; every view uses it.
var svObserved = time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

const svZeroRev = "0000000000000000000000000000000000000000"

type svBriefExpect struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	Verified string `json:"verified"`
	Reviewed string `json:"reviewed"`
}

type svStreamExpect struct {
	Readme      string `json:"readme"`
	DefaultRepo string `json:"default_repo"`
	Key         string `json:"key"`
	Repo        string `json:"repo"`
}

type svCriterionExpect struct {
	Criterion string   `json:"criterion"`
	Evidence  []string `json:"evidence"`
}

type svExpect struct {
	Legacy struct {
		Readme       string          `json:"readme"`
		DefaultRepo  string          `json:"default_repo"`
		Key          string          `json:"key"`
		DisplayName  string          `json:"display_name"`
		Status       string          `json:"status"`
		Briefs       []svBriefExpect `json:"briefs"`
		Origin       string          `json:"origin"`
		Outcome      string          `json:"outcome"`
		SuccessCount int             `json:"success_count"`
	} `json:"legacy"`
	Identity struct {
		Streams      []svStreamExpect `json:"streams"`
		DistinctKeys int              `json:"distinct_keys"`
		ConflictRepo string           `json:"conflict_default_repo"`
	} `json:"identity"`
	Negative struct {
		Cases []struct {
			Readme string `json:"readme"`
			Diag   string `json:"diag"`
		} `json:"cases"`
		Unsupported string `json:"unsupported_contract"`
	} `json:"negative"`
	Flow struct {
		Readme      string              `json:"readme"`
		Repo        string              `json:"repo"`
		Outcome     string              `json:"outcome"`
		Success     []svCriterionExpect `json:"success"`
		Commitments []string            `json:"commitments"`
		Exclusions  []string            `json:"exclusions"`
	} `json:"flow"`
	Deref struct {
		Repos     map[string]string `json:"repos"`
		Views     []svStreamExpect  `json:"views"`
		Resolved  []string          `json:"resolved"`
		Planned   []string          `json:"planned"`
		Unchecked []string          `json:"unchecked"`
	} `json:"dereference"`
}

func svFixtures(t *testing.T) string {
	t.Helper()
	if d := os.Getenv("STREAMVIEW_FIXTURES"); d != "" {
		return d
	}
	return filepath.Join("testdata", "streamview")
}

func svLoadExpect(t *testing.T, fx string) svExpect {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(fx, "expect.json"))
	if err != nil {
		t.Fatalf("ASSERT-FAIL[fixture-read]: %v", err)
	}
	var e svExpect
	if err := json.Unmarshal(raw, &e); err != nil {
		t.Fatalf("ASSERT-FAIL[fixture-read]: expect.json: %v", err)
	}
	return e
}

// svCheck records a failed assertion under its stable id.
func svCheck(t *testing.T, id string, ok bool, format string, a ...any) {
	t.Helper()
	if !ok {
		t.Errorf("ASSERT-FAIL[%s]: %s", id, fmt.Sprintf(format, a...))
	}
}

func svParse(t *testing.T, fx, rel string) *Stream {
	t.Helper()
	s, err := parseStreamREADME(filepath.Join(fx, rel))
	if err != nil {
		t.Fatalf("ASSERT-FAIL[parse]: %s: %v", rel, err)
	}
	return s
}

// svRepoRel strips the fixture repo directory ("repo-a/") from a fixture path,
// giving the README's repository-relative path.
func svRepoRel(rel string) string {
	if i := strings.Index(rel, "/docs/"); i >= 0 {
		return rel[i+1:]
	}
	return rel
}

func svEvidenceString(e streamview.EvidenceRef) string {
	switch e.Kind {
	case streamview.EvidencePath:
		if e.Planned {
			return "planned:" + e.Repo + ":" + e.Path
		}
		return "path:" + e.Repo + ":" + e.Path
	case streamview.EvidenceForge:
		return fmt.Sprintf("forge:%s#%d", e.Repo, e.Number)
	case streamview.EvidenceURL:
		return "url:" + e.URL
	}
	return "unknown:" + string(e.Kind)
}

// TestStreamViewLegacy — Verify row 1: a legacy README (no mission block)
// parses to the same identity and state, its mission is the legacy prose
// outcome, and success criteria stay absent — in the struct AND on the wire.
func TestStreamViewLegacy(t *testing.T) {
	fx := svFixtures(t)
	e := svLoadExpect(t, fx).Legacy
	s := svParse(t, fx, e.Readme)

	svCheck(t, "legacy-mission-absent", s.Mission == nil && len(s.MissionDiagnostics) == 0,
		"legacy README must parse with no mission and no diagnostics, got mission=%v diags=%v", s.Mission, s.MissionDiagnostics)
	svCheck(t, "legacy-status", s.Status == e.Status, "status %q, want %q", s.Status, e.Status)
	var got []svBriefExpect
	for _, b := range s.Briefs {
		got = append(got, svBriefExpect{ID: s.Name + "/" + b.Num, Status: b.Status, Verified: b.Verified, Reviewed: b.Reviewed})
	}
	svCheck(t, "legacy-briefs", reflect.DeepEqual(got, e.Briefs), "briefs %+v, want %+v", got, e.Briefs)

	v, err := streamViewSeed(s, e.DefaultRepo, svRepoRel(e.Readme), strings.Repeat("a", 40), svObserved, streamview.AccessContext{})
	if err != nil {
		t.Fatalf("ASSERT-FAIL[legacy-seed]: %v", err)
	}
	svCheck(t, "legacy-identity", v.Identity.Key.String() == e.Key, "key %q, want %q", v.Identity.Key, e.Key)
	svCheck(t, "legacy-display", v.Identity.DisplayName == e.DisplayName, "display %q, want %q", v.Identity.DisplayName, e.DisplayName)
	svCheck(t, "legacy-origin", string(v.Mission.Origin) == e.Origin, "origin %q, want %q", v.Mission.Origin, e.Origin)
	svCheck(t, "legacy-outcome", v.Mission.Outcome == e.Outcome, "outcome %q, want %q", v.Mission.Outcome, e.Outcome)
	svCheck(t, "legacy-success-absent", len(v.Mission.Success) == e.SuccessCount,
		"success criteria %d, want %d — a legacy stream's criteria are never invented", len(v.Mission.Success), e.SuccessCount)
	svCheck(t, "legacy-state", v.CurrentState.Status == e.Status && len(v.CurrentState.Briefs) == len(e.Briefs),
		"current_state status %q / %d briefs, want %q / %d", v.CurrentState.Status, len(v.CurrentState.Briefs), e.Status, len(e.Briefs))
	for i, b := range v.CurrentState.Briefs {
		if i < len(e.Briefs) {
			svCheck(t, "legacy-state", b.ID == e.Briefs[i].ID && b.Status == e.Briefs[i].Status,
				"current_state brief %d = %s/%s, want %s/%s", i, b.ID, b.Status, e.Briefs[i].ID, e.Briefs[i].Status)
		}
	}
	enc, err := streamview.Encode(v)
	if err != nil {
		t.Fatalf("ASSERT-FAIL[legacy-encode]: %v", err)
	}
	var wire struct {
		Mission map[string]json.RawMessage `json:"mission"`
	}
	if err := json.Unmarshal(enc, &wire); err != nil {
		t.Fatalf("ASSERT-FAIL[legacy-encode]: %v", err)
	}
	_, hasSuccess := wire.Mission["success"]
	svCheck(t, "legacy-success-absent", (e.SuccessCount == 0) == !hasSuccess,
		"wire mission.success present=%v; an absent criterion list must be absent on the wire", hasSuccess)

	// Adding a mission block changes nothing else: the authored fixture with
	// its block stripped parses to the same identity and state.
	authored := filepath.Join(fx, "repo-a", "docs", "streams", "shared", "README.md")
	raw, err := os.ReadFile(authored)
	if err != nil {
		t.Fatalf("ASSERT-FAIL[fixture-read]: %v", err)
	}
	stripped := svStripMission(string(raw))
	dir := filepath.Join(t.TempDir(), "shared")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(stripped), 0o644); err != nil {
		t.Fatal(err)
	}
	with, err1 := parseStreamREADME(authored)
	without, err2 := parseStreamREADME(filepath.Join(dir, "README.md"))
	if err1 != nil || err2 != nil {
		t.Fatalf("ASSERT-FAIL[parse]: %v / %v", err1, err2)
	}
	svCheck(t, "legacy-same-state", without.Mission == nil && with.Mission != nil, "strip control: with=%v without=%v", with.Mission != nil, without.Mission != nil)
	svCheck(t, "legacy-same-state",
		with.Name == without.Name && with.Status == without.Status && with.Repo == without.Repo &&
			with.Priority == without.Priority && reflect.DeepEqual(with.Briefs, without.Briefs),
		"a mission block changed the parsed identity/state")

	// The real corpus: every stream README in this repository still parses,
	// with no mission diagnostics — no stream needs a migration.
	corpus, _, err := loadStreams("..")
	if err != nil {
		t.Fatalf("ASSERT-FAIL[legacy-corpus]: could not load this repository's streams: %v", err)
	}
	svCheck(t, "legacy-corpus", len(corpus) > 0, "corpus loaded no streams")
	for _, cs := range corpus {
		svCheck(t, "legacy-corpus", len(cs.MissionDiagnostics) == 0, "%s: %v", cs.Name, cs.MissionDiagnostics)
	}
	svCheck(t, "legacy-corpus", len(missionProblems(corpus)) == 0, "corpus raises mission problems: %v", missionProblems(corpus))
}

// svStripMission removes the `mission:` block (the key line and its indented
// continuation) from a README's frontmatter.
func svStripMission(readme string) string {
	var out []string
	skipping := false
	for _, ln := range strings.Split(readme, "\n") {
		if strings.HasPrefix(ln, "mission:") {
			skipping = true
			continue
		}
		if skipping && (strings.HasPrefix(ln, " ") || strings.HasPrefix(ln, "\t")) {
			continue
		}
		skipping = false
		out = append(out, ln)
	}
	return strings.Join(out, "\n")
}

// TestStreamViewIdentity — Verify row 2: two repositories with the same slug
// stay distinct; a rename is a new identity that carries no history even when
// its display name and mission are identical; cell context never enters the
// key; the repo is never guessed.
func TestStreamViewIdentity(t *testing.T) {
	fx := svFixtures(t)
	e := svLoadExpect(t, fx).Identity
	var ids []streamview.Identity
	keys := map[streamview.Key]bool{}
	for _, st := range e.Streams {
		s := svParse(t, fx, st.Readme)
		id, err := streamIdentity(s, st.DefaultRepo)
		if err != nil {
			t.Fatalf("ASSERT-FAIL[identity-key]: %s: %v", st.Readme, err)
		}
		svCheck(t, "identity-key", id.Key.String() == st.Key, "%s: key %q, want %q", st.Readme, id.Key, st.Key)
		ids = append(ids, id)
		keys[id.Key] = true
	}
	svCheck(t, "identity-distinct", len(keys) == e.DistinctKeys, "%d distinct keys, want %d", len(keys), e.DistinctKeys)
	if len(ids) < 3 {
		t.Fatalf("ASSERT-FAIL[identity-key]: need three fixture streams, have %d", len(ids))
	}
	a, b, renamed := ids[0], ids[1], ids[2]
	svCheck(t, "identity-distinct", a.Key.Slug == b.Key.Slug && !streamview.SameStream(a, b),
		"same slug %q in two repos must be two streams", a.Key.Slug)
	svCheck(t, "identity-distinct", !streamview.ContinuesHistory(a.Key, b.Key), "history crossed repositories")
	svCheck(t, "identity-rename", a.DisplayName == renamed.DisplayName,
		"rename control: fixture display names should match (%q vs %q)", a.DisplayName, renamed.DisplayName)
	svCheck(t, "identity-rename", !streamview.SameStream(a, renamed) && !streamview.ContinuesHistory(a.Key, renamed.Key),
		"a renamed stream (%s → %s) must not inherit history by matching display name", a.Key, renamed.Key)
	svCheck(t, "identity-rename", streamview.ContinuesHistory(a.Key, a.Key), "an unchanged key must continue its own history")

	// Cell context lives beside the key: two cells, one stream, same identity bytes.
	sa := svParse(t, fx, e.Streams[0].Readme)
	v1, err1 := streamViewSeed(sa, e.Streams[0].DefaultRepo, svRepoRel(e.Streams[0].Readme), strings.Repeat("b", 40), svObserved, streamview.AccessContext{Cell: "cell-one"})
	v2, err2 := streamViewSeed(sa, e.Streams[0].DefaultRepo, svRepoRel(e.Streams[0].Readme), strings.Repeat("b", 40), svObserved, streamview.AccessContext{Cell: "cell-two"})
	if err1 != nil || err2 != nil {
		t.Fatalf("ASSERT-FAIL[identity-cell]: %v / %v", err1, err2)
	}
	j1, _ := json.Marshal(v1.Identity)
	j2, _ := json.Marshal(v2.Identity)
	svCheck(t, "identity-cell", streamview.SameStream(v1.Identity, v2.Identity) && string(j1) == string(j2),
		"cell context leaked into identity: %s vs %s", j1, j2)
	kt := reflect.TypeOf(streamview.Key{})
	var fields []string
	for i := 0; i < kt.NumField(); i++ {
		fields = append(fields, kt.Field(i).Name)
	}
	svCheck(t, "identity-cell", reflect.DeepEqual(fields, []string{"Repo", "Slug"}), "Key fields %v, want exactly [Repo Slug]", fields)

	// Precedence: declared repo vs producer repo — a conflict and an absence
	// are errors, never a silent pick.
	if _, err := streamIdentity(sa, e.ConflictRepo); err == nil {
		t.Errorf("ASSERT-FAIL[identity-precedence]: declared repo %q vs producer repo %q must conflict", sa.Repo, e.ConflictRepo)
	}
	sb := svParse(t, fx, e.Streams[1].Readme)
	if _, err := streamIdentity(sb, ""); err == nil {
		t.Errorf("ASSERT-FAIL[identity-precedence]: a stream with no declared or configured repo got an identity")
	}
}

// TestStreamViewNegative — Verify row 3: every malformed mission block yields
// its explicit diagnostic, a lint PROBLEM, and a could-not-check mission
// section — never the legacy prose fallback, never a success default.
func TestStreamViewNegative(t *testing.T) {
	fx := svFixtures(t)
	e := svLoadExpect(t, fx).Negative
	svCheck(t, "negative-diag", len(e.Cases) > 0, "no negative cases")
	for _, c := range e.Cases {
		s := svParse(t, fx, c.Readme)
		joined := strings.Join(s.MissionDiagnostics, "\n")
		svCheck(t, "negative-diag", s.Mission == nil && strings.Contains(joined, c.Diag),
			"%s: want diagnostic containing %q, got mission=%v diags=%q", c.Readme, c.Diag, s.Mission != nil, joined)
		probs := missionProblems([]*Stream{s})
		svCheck(t, "negative-lint", len(probs) > 0 && strings.Contains(strings.Join(probs, "\n"), s.Name+": invalid mission metadata"),
			"%s: invalid mission must be a lint PROBLEM, got %v", c.Readme, probs)
		v, err := streamViewSeed(s, "example-org/negative", svRepoRel(c.Readme), strings.Repeat("c", 40), svObserved, streamview.AccessContext{})
		if err != nil {
			t.Fatalf("ASSERT-FAIL[negative-section]: %s: %v", c.Readme, err)
		}
		svCheck(t, "negative-section", v.Mission.Provenance.Availability == streamview.CouldNotCheck &&
			v.Mission.Origin == "" && v.Mission.Outcome == "" && len(v.Mission.Success) == 0 &&
			len(v.Mission.Provenance.Diagnostics) > 0,
			"%s: invalid mission must be could-not-check with diagnostics and no content, got %+v", c.Readme, v.Mission)
	}

	// Contract versions: unknown and missing versions fail visibly.
	good, err := streamViewSeed(svParse(t, fx, "repo-a/docs/streams/shared/README.md"), "", "docs/streams/shared/README.md", strings.Repeat("d", 40), svObserved, streamview.AccessContext{})
	if err != nil {
		t.Fatalf("ASSERT-FAIL[negative-version]: %v", err)
	}
	enc, err := streamview.Encode(good)
	if err != nil {
		t.Fatalf("ASSERT-FAIL[negative-version]: %v", err)
	}
	future := strings.Replace(string(enc), `"contract": "`+streamview.Version+`"`, `"contract": "`+e.Unsupported+`"`, 1)
	svCheck(t, "negative-version", future != string(enc), "version substitution did not apply")
	_, err = streamview.Decode([]byte(future))
	var uv *streamview.UnsupportedVersionError
	svCheck(t, "negative-version", errors.As(err, &uv) && uv.Got == e.Unsupported, "decode of %q: %v", e.Unsupported, err)
	missing := strings.Replace(string(enc), `"contract": "`+streamview.Version+`",`, "", 1)
	_, err = streamview.Decode([]byte(missing))
	svCheck(t, "negative-version", errors.As(err, &uv) && uv.Got == "", "decode with no contract field: %v", err)
}

// TestStreamViewFlow — Verify row 4: authored mission and evidence refs,
// parsed from the README, survive Encode → Decode (the documented consumer)
// with the exact source revision and every value, in authored order.
func TestStreamViewFlow(t *testing.T) {
	fx := svFixtures(t)
	e := svLoadExpect(t, fx).Flow
	repoDir := svGitRepo(t, filepath.Join(fx, strings.SplitN(e.Readme, "/", 2)[0]))
	rev := svGitOut(t, repoDir, "rev-parse", "HEAD")
	s := svParse(t, fx, e.Readme)
	v, err := streamViewSeed(s, "", svRepoRel(e.Readme), rev, svObserved, streamview.AccessContext{Cell: "cell-one"})
	if err != nil {
		t.Fatalf("ASSERT-FAIL[flow-seed]: %v", err)
	}
	enc, err := streamview.Encode(v)
	if err != nil {
		t.Fatalf("ASSERT-FAIL[flow-encode]: %v", err)
	}
	dec, err := streamview.Decode(enc)
	if err != nil {
		t.Fatalf("ASSERT-FAIL[flow-decode]: %v", err)
	}
	svCheck(t, "flow-roundtrip", reflect.DeepEqual(v, dec), "decoded view differs from the encoded one")
	again, _ := streamview.Encode(dec)
	svCheck(t, "flow-roundtrip", string(again) == string(enc), "re-encode is not byte-stable")

	m := dec.Mission
	svCheck(t, "flow-revision", len(m.Provenance.Sources) == 1 && m.Provenance.Sources[0].Revision == rev &&
		m.Provenance.Sources[0].Path == svRepoRel(e.Readme) && m.Provenance.Sources[0].Repo == e.Repo,
		"mission source %+v, want %s:%s@%s", m.Provenance.Sources, e.Repo, svRepoRel(e.Readme), rev)
	svCheck(t, "flow-value", m.Origin == streamview.OriginAuthored && m.Outcome == e.Outcome, "origin/outcome %q/%q, want authored/%q", m.Origin, m.Outcome, e.Outcome)
	svCheck(t, "flow-value", len(m.Success) == len(e.Success), "%d success criteria, want %d", len(m.Success), len(e.Success))
	for i := range e.Success {
		if i >= len(m.Success) {
			break
		}
		var refs []string
		for _, r := range m.Success[i].Evidence {
			refs = append(refs, svEvidenceString(r))
		}
		svCheck(t, "flow-value", m.Success[i].Criterion == e.Success[i].Criterion && reflect.DeepEqual(refs, e.Success[i].Evidence),
			"success[%d] = %q %v, want %q %v", i, m.Success[i].Criterion, refs, e.Success[i].Criterion, e.Success[i].Evidence)
	}
	svCheck(t, "flow-value", reflect.DeepEqual(m.Commitments, e.Commitments) && reflect.DeepEqual(m.Exclusions, e.Exclusions),
		"commitments/exclusions %v/%v, want %v/%v", m.Commitments, m.Exclusions, e.Commitments, e.Exclusions)
	svCheck(t, "flow-context", dec.Context.Cell == "cell-one" && dec.Identity.Key.Repo == e.Repo, "context/identity did not survive: %+v %+v", dec.Context, dec.Identity)
	svCheck(t, "flow-window", dec.Changes.Window.Label == streamview.TrailingWindowLabel && dec.Changes.Window.End.Equal(svObserved),
		"window %+v, want %q ending %s", dec.Changes.Window, streamview.TrailingWindowLabel, svObserved)
}

// svGitResolver resolves bindings against temporary git repositories.
type svGitResolver struct {
	t    *testing.T
	dirs map[string]string
}

func (r svGitResolver) Blob(repo, rev, path string) (string, error) {
	dir, ok := r.dirs[repo]
	if !ok {
		return "", streamview.ErrUnknownRepo
	}
	if err := exec.Command("git", "-C", dir, "cat-file", "-e", rev+"^{commit}").Run(); err != nil {
		return "", streamview.ErrUnknownRevision
	}
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--verify", "--quiet", rev+":"+path).Output()
	if err != nil {
		return "", nil
	}
	return strings.TrimSpace(string(out)), nil
}

func (r svGitResolver) Head(repo string) (string, error) {
	dir, ok := r.dirs[repo]
	if !ok {
		return "", streamview.ErrUnknownRepo
	}
	return svGitOut(r.t, dir, "rev-parse", "HEAD"), nil
}

func svGitEnv() []string {
	return append(os.Environ(),
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid",
		"GIT_COMMITTER_NAME=fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid",
		"GIT_AUTHOR_DATE=2026-10-01T00:00:00Z", "GIT_COMMITTER_DATE=2026-10-01T00:00:00Z")
}

func svGitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = svGitEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ASSERT-FAIL[git]: git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// svGitRepo copies a fixture repository tree into a temp dir and commits it.
func svGitRepo(t *testing.T, src string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Fatalf("ASSERT-FAIL[git]: could-not-check: git not on PATH")
	}
	dst := filepath.Join(t.TempDir(), filepath.Base(src))
	err := filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, b, 0o644)
	})
	if err != nil {
		t.Fatalf("ASSERT-FAIL[fixture-read]: %v", err)
	}
	svGitOut(t, dst, "init", "-q")
	svGitOut(t, dst, "add", "-A")
	svGitOut(t, dst, "commit", "-q", "-m", "fixture")
	return dst
}

// TestStreamViewDereference — Verify row 5: every recorded source revision
// and evidence path resolves against its owning repository; planned outputs
// are reported as planned; external refs are reported as unchecked residue;
// absent, stale, unknown-repo and planned-but-present bindings are rejected.
func TestStreamViewDereference(t *testing.T) {
	fx := svFixtures(t)
	e := svLoadExpect(t, fx).Deref
	dirs := map[string]string{}
	for repo, sub := range e.Repos {
		dirs[repo] = svGitRepo(t, filepath.Join(fx, sub))
	}
	res := svGitResolver{t: t, dirs: dirs}
	var all streamview.BindingReport
	var views []*streamview.StreamView
	for _, vw := range e.Views {
		s := svParse(t, fx, vw.Readme)
		head := svGitOut(t, dirs[vw.Repo], "rev-parse", "HEAD")
		def := ""
		if s.Repo == "" {
			def = vw.Repo
		}
		v, err := streamViewSeed(s, def, svRepoRel(vw.Readme), head, svObserved, streamview.AccessContext{})
		if err != nil {
			t.Fatalf("ASSERT-FAIL[deref-seed]: %s: %v", vw.Readme, err)
		}
		views = append(views, v)
		rep := streamview.CheckBindings(v, res)
		all.Resolved = append(all.Resolved, rep.Resolved...)
		all.Planned = append(all.Planned, rep.Planned...)
		all.Unchecked = append(all.Unchecked, rep.Unchecked...)
		all.Problems = append(all.Problems, rep.Problems...)
	}
	svCheck(t, "deref-clean", len(all.Problems) == 0, "genuine bindings raised problems: %v", all.Problems)
	joined := strings.Join(all.Resolved, "\n")
	for _, want := range e.Resolved {
		svCheck(t, "deref-resolved", strings.Contains(joined, want+"@"), "binding %q not resolved; resolved=%v", want, all.Resolved)
	}
	svCheck(t, "deref-planned", reflect.DeepEqual(all.Planned, e.Planned), "planned %v, want %v", all.Planned, e.Planned)
	svCheck(t, "deref-unchecked", reflect.DeepEqual(all.Unchecked, e.Unchecked), "unchecked %v, want %v", all.Unchecked, e.Unchecked)
	if len(views) == 0 {
		t.Fatal("ASSERT-FAIL[deref-seed]: no views")
	}

	// Negative controls on the genuine view: each corrupted binding must be
	// rejected with its own reason.
	base := views[0]
	mutate := func(name, wantSub string, f func(v *streamview.StreamView)) {
		t.Helper()
		raw, _ := json.Marshal(base)
		var v streamview.StreamView
		_ = json.Unmarshal(raw, &v)
		f(&v)
		rep := streamview.CheckBindings(&v, res)
		svCheck(t, "deref-reject", strings.Contains(strings.Join(rep.Problems, "\n"), wantSub),
			"%s: want a problem containing %q, got %v", name, wantSub, rep.Problems)
	}
	mutate("absent revision", streamview.ErrUnknownRevision.Error(), func(v *streamview.StreamView) {
		v.Mission.Provenance.Sources[0].Revision = svZeroRev
	})
	mutate("unknown repo", streamview.ErrUnknownRepo.Error(), func(v *streamview.StreamView) {
		v.Mission.Provenance.Sources[0].Repo = "example-org/elsewhere"
	})
	mutate("absent path", "absent at", func(v *streamview.StreamView) {
		v.Mission.Provenance.Sources[0].Path = "docs/streams/shared/MISSING.md"
	})
	mutate("absent evidence", "absent at", func(v *streamview.StreamView) {
		v.Mission.Success[0].Evidence[0].Path = "docs/evidence/missing.md"
	})
	mutate("planned but present", "marked planned but exists", func(v *streamview.StreamView) {
		v.Mission.Success[1].Evidence[0].Path = "docs/evidence/acceptance.md"
	})

	// Stale: the README changes after the recorded revision.
	repoA := dirs[base.Identity.Key.Repo]
	readme := filepath.Join(repoA, filepath.FromSlash(base.Mission.Provenance.Sources[0].Path))
	b, _ := os.ReadFile(readme)
	if err := os.WriteFile(readme, append(b, []byte("\nedited after the snapshot\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	svGitOut(t, repoA, "commit", "-q", "-am", "edit")
	rep := streamview.CheckBindings(base, res)
	svCheck(t, "deref-reject", strings.Contains(strings.Join(rep.Problems, "\n"), "is stale"), "stale binding not rejected: %v", rep.Problems)
}

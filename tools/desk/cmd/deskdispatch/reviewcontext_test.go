package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/packet"
	rf "github.com/medici-finance/assay/tools/desk/internal/reviewflow"
	"github.com/medici-finance/assay/tools/desk/internal/reviewtest"
)

type prepareFixture struct {
	head, base, root string
	transport        *reviewtest.Transport
	moved            string
	prReads          int
	diffReads        int
	symlink          bool
	blockContent     bool
}

func prepareTestFixture(t *testing.T) *prepareFixture {
	return prepareSourceFixture(t, "assigned bytes\n")
}
func prepareSourceFixture(t *testing.T, assigned string) *prepareFixture {
	t.Helper()
	f := &prepareFixture{root: t.TempDir()}
	git := func(args ...string) string {
		c := exec.Command("git", args...)
		c.Dir = f.root
		c.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
		b, e := c.CombinedOutput()
		if e != nil {
			t.Fatal(e, string(b))
		}
		return strings.TrimSpace(string(b))
	}
	git("init", "-q")
	commit := func(body string) string {
		if e := os.WriteFile(filepath.Join(f.root, "a.go"), []byte(body), 0600); e != nil {
			t.Fatal(e)
		}
		git("add", "a.go")
		git("-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "fixture")
		return git("rev-parse", "HEAD")
	}
	f.base = commit("base bytes\n")
	f.head = commit(assigned)
	commit("wrong allocated head\n")
	reviewtest.Home(t, reviewtest.Request{Home: t.TempDir()}, `ASSAY_BLESS_LOGIN=ada:2001
ASSAY_TRUSTED_LOGINS=ada:2001
ASSAY_TRUSTED_BOT_SLUGS=reviewer=github:reviewer:41987965
ASSAY_ALLOWED_REPOS=example-org/tracker:ci:private
ASSAY_REPO_FORGES=example-org/tracker=github
`)
	deskkit.ReloadConfig()
	t.Cleanup(deskkit.ReloadConfig)
	deskkit.SetGitHubCustodyMinter(func(string, deskkit.ForgeRepo) (string, string, error) { return "fixture-token", "", nil })
	t.Cleanup(func() { deskkit.SetGitHubCustodyMinter(nil) })
	oldReviewer := reviewPacketReviewerFn
	reviewPacketReviewerFn = func() (string, bool) { return "reviewer", true }
	t.Cleanup(func() { reviewPacketReviewerFn = oldReviewer })
	f.transport = &reviewtest.Transport{Answer: func(r *http.Request) (int, string) {
		path := r.URL.Path
		if r.Context().Err() != nil {
			return 408, `{"message":"request cancelled"}`
		}
		if r.Method != "GET" {
			t.Fatalf("preparation wrote %s %s", r.Method, path)
		}
		if strings.HasSuffix(path, "/pulls/77") && !strings.Contains(r.Header.Get("Accept"), "diff") {
			f.prReads++
			if f.prReads > 1 && f.moved == "access" {
				return 403, `{"message":"denied"}`
			}
			head, base, body := f.head, f.base, "description"
			if f.moved == "missing-base" {
				base = ""
			}
			if f.prReads > 1 {
				switch f.moved {
				case "head":
					head = strings.Repeat("e", 40)
				case "base":
					base = strings.Repeat("e", 40)
				case "body":
					body = "edited"
				}
			}
			return 200, fmt.Sprintf(`{"number":77,"state":"open","body":%q,"changed_files":1,"head":{"sha":%q,"ref":"feat/x"},"base":{"sha":%q,"ref":"main"}}`, body, head, base)
		}
		if strings.HasSuffix(path, "/timeline") {
			return 200, `[]`
		}
		if strings.HasSuffix(path, "/reviews") {
			if f.prReads > 1 && f.moved == "reviews" {
				return 200, `[]`
			}
			return 200, fmt.Sprintf(`[{"id":1,"state":"CHANGES_REQUESTED","body":"Verdict: request-changes\nF-1: fix the bytes","commit_id":%q,"user":{"login":"reviewer"},"submitted_at":"2026-10-01T00:00:00Z"}]`, f.base)
		}
		if strings.HasSuffix(path, "/protection/required_status_checks") {
			if f.prReads > 1 && f.moved == "policy" {
				return 200, `{"contexts":["changed"]}`
			}
			return 200, `{"contexts":["build"]}`
		}
		if strings.HasSuffix(path, "/check-runs") {
			conclusion := "success"
			if f.prReads > 1 && f.moved == "checks" {
				conclusion = "failure"
			}
			return 200, fmt.Sprintf(`{"total_count":1,"check_runs":[{"name":"build","status":"completed","conclusion":%q}]}`, conclusion)
		}
		if strings.HasSuffix(path, "/status") {
			return 200, `{"state":"success","total_count":0,"statuses":[]}`
		}
		if strings.HasSuffix(path, "/files") {
			return 200, `[{"filename":"a.go","status":"modified","patch":"@@ -1 +1 @@\n-base\n+assigned"}]`
		}
		if strings.Contains(path, "/compare/") {
			return 200, fmt.Sprintf(`{"total_commits":1,"commits":[{"sha":%q,"parents":[{"sha":%q}]}],"files":[{"filename":"a.go"}]}`, f.head, f.base)
		}
		if strings.Contains(path, "/commits/") {
			return 200, fmt.Sprintf(`{"sha":%q,"parents":[{"sha":%q}],"files":[{"filename":"a.go"}]}`, f.head, f.base)
		}
		if strings.Contains(path, "/contents/") {
			if f.symlink {
				return 200, `{"type":"symlink","target":"target"}`
			}
			if f.blockContent {
				<-r.Context().Done()
				return 408, `{"message":"packet read deadline"}`
			}
			return 200, fmt.Sprintf(`{"type":"file","sha":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","encoding":"base64","content":%q}`, base64.StdEncoding.EncodeToString([]byte("assigned bytes\n")))
		}
		if strings.Contains(r.Header.Get("Accept"), "diff") {
			f.diffReads++
			return 200, "diff --git a/a.go b/a.go\n-base bytes\n+assigned bytes\n"
		}
		t.Fatalf("unexpected preparation read %s", path)
		return 500, ""
	}}
	old := http.DefaultTransport
	http.DefaultTransport = f.transport
	t.Cleanup(func() { http.DefaultTransport = old })
	oldRound := reviewRoundForgeFn
	reviewRoundForgeFn = liveReviewRoundForge
	t.Cleanup(func() { reviewRoundForgeFn = oldRound })
	return f
}
func prepareFlow(t *testing.T, f *prepareFixture) (*preparedReview, packet.Spec) {
	t.Helper()
	o := dispatchOpts{kit: "review", pr: 77, root: f.root}
	p, e := newPreparedReview(o, rrRepo)
	if e != nil || p == nil {
		t.Fatal("context not prepared", e)
	}
	t.Cleanup(p.cancel)
	plan := dispatchPlan{claimKey: rrKeyC, reviewContext: p}
	round, e := prepareReviewRound(o, plan, rrRepo)
	if e != nil || round.head != f.head {
		t.Fatal("round lost assigned head", e, round)
	}
	spec, e := reviewPacket(packetInput{o: o, plan: plan, repo: rrRepo, home: f.root})
	if e != nil {
		t.Fatal(e)
	}
	return p, spec
}
func TestReviewPrepareAdaptersFlow(t *testing.T) {
	legacy := prepareTestFixture(t)
	o := dispatchOpts{kit: "review", pr: 77, root: legacy.root}
	plan := dispatchPlan{claimKey: rrKeyC}
	if _, e := prepareReviewRound(o, plan, rrRepo); e != nil {
		t.Fatal(e)
	}
	oldPacket := reviewPacketForgeFn
	reviewPacketForgeFn = func(repo string) (reviewPacketForge, deskkit.ForgeRepo, error) { return liveReviewRoundPacket(repo) }
	t.Cleanup(func() { reviewPacketForgeFn = oldPacket })
	before, e := reviewPacket(packetInput{o: o, plan: plan, repo: rrRepo, home: legacy.root})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = packet.Build(before); e != nil {
		t.Fatal(e)
	}
	beforeCalls := len(legacy.transport.Snapshot())
	f := prepareTestFixture(t)
	p, spec := prepareFlow(t, f)
	built, e := packet.Build(spec)
	if e != nil {
		t.Fatal("prepared packet failed", e)
	}
	if !strings.Contains(string(built.Text), "assigned bytes") || strings.Contains(string(built.Text), "wrong allocated head") {
		t.Fatal("packet did not use assigned-head bytes")
	}
	if f.prReads != 2 {
		t.Fatalf("shared preparation reread PR %d times; want initial + terminal", f.prReads)
	}
	counts := map[string]int{}
	for _, a := range f.transport.Snapshot() {
		counts[a.Path]++
	}
	if counts["/repos/"+rrRepo+"/pulls/77/files"] != 1 {
		t.Fatal("shared changed files were fetched twice", counts)
	}
	if counts["/repos/"+rrRepo+"/pulls/77/reviews"] != 2 {
		t.Fatal("reviews not shared before terminal check", counts)
	}
	for _, a := range f.transport.Snapshot() {
		if strings.Contains(a.Path, "/contents/") {
			t.Fatal("local exact blob incurred a provider read")
		}
	}
	for i := 0; i < 2; i++ {
		v, e := p.ReadFile(p.repo, deskkit.ReadFileInput{File: "a.go", Ref: f.head})
		if e != nil || string(v.Content) != "assigned bytes\n" {
			t.Fatal(v, e)
		}
	}
	if len(p.content) != 1 {
		t.Fatal("immutable content was not memoized once")
	}
	if beforeCalls != 17 || len(f.transport.Snapshot()) != 14 {
		t.Fatalf("unexpected cold cost before=%d after=%d", beforeCalls, len(f.transport.Snapshot()))
	}
	if len(f.transport.Snapshot()) >= beforeCalls {
		t.Fatalf("preparation did not reduce provider attempts: before=%d after=%d", beforeCalls, len(f.transport.Snapshot()))
	}
	t.Logf("legacy=%d full round+packet attempts=%d; PR=2 reviews=2 files=1 content=0; agent immutable refetch=0 under packet contract", beforeCalls, len(f.transport.Snapshot()))
}
func TestReviewPrepareAdaptersInvalidation(t *testing.T) {
	for _, change := range []string{"head", "base", "body", "reviews", "checks", "access", "policy"} {
		t.Run(change, func(t *testing.T) {
			f := prepareTestFixture(t)
			_, spec := prepareFlow(t, f)
			f.moved = change
			if _, e := packet.Build(spec); e == nil {
				t.Fatal("mutable preparation accepted changed " + change)
			}
		})
	}
	t.Run("wrong-kit", func(t *testing.T) {
		_, e := validateCallerPreconditions(dispatchOpts{kit: "worker", observation: "saved.json", pr: 77})
		if e == nil || !strings.Contains(e.Error(), "--observation requires") {
			t.Fatal("observation bypassed kit admission")
		}
	})
}

func liveReviewRoundPacket(repo string) (reviewPacketForge, deskkit.ForgeRepo, error) {
	fg, fr, e := liveReviewRoundForge(repo)
	if e != nil {
		return nil, fr, e
	}
	return fg.(reviewPacketForge), fr, nil
}

// Full entrypoint: the normal claim/workspace/stamp seams remain in control;
// only their external effects are replaced by the existing dispatch harness.
func prepareDispatchFixture(t *testing.T) (*stub, *prepareFixture, *ghStampServer, *[]queueLabelCall, []string) {
	t.Helper()
	s := &stub{}
	s.install(t)
	realTransport := http.DefaultTransport
	f := prepareTestFixture(t)
	plantScripts(t, f.root)
	s.replies = happyReplies(filepath.Join(t.TempDir(), "review-home"))
	t.Setenv("DESK_LOOP", "pr-review-desk")
	fg, fr, e := liveReviewRoundForge(rrRepo)
	if e != nil {
		t.Fatal(e)
	}
	reviewRoundForgeFn = func(string) (reviewRoundForge, deskkit.ForgeRepo, error) { return fg, fr, nil }
	stamp := installGHStamp(t)
	http.DefaultTransport = prepareRoutingTransport{preparation: f.transport, effects: realTransport}
	queue := stubQueueLabel(t, nil)
	usePacketBuilder(t)
	args := stampArgs(t, f.root, "--kit", "review", "--quiet", "--repo", rrRepo, "--observation", savedPrepareObservation(t, f, "valid"))
	args[0] = rrKeyC

	return s, f, stamp, queue, args
}
func TestReviewPrepareDispatchWiring(t *testing.T) {
	_, f, _, _, args := prepareDispatchFixture(t)
	rc, stderr := runCapturingStderr(t, args)
	if rc != 0 {
		t.Fatalf("full preparation dispatch failed %d: %s", rc, stderr)
	}
	if f.prReads != 2 {
		t.Fatalf("dispatcher did not read preparation boundaries: PR reads=%d", f.prReads)
	}
	raw, e := os.ReadFile(emittedPathFor(promptFileOf(t, args)))
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(string(raw), "Packet:") || !strings.Contains(string(raw), f.head) {
		t.Fatal("dispatch lost exact-source packet/round")
	}
}

func savedPrepareObservation(t *testing.T, f *prepareFixture, change string) string {
	t.Helper()
	now := time.Now().Add(-time.Second)
	repo := rf.Repository{Provider: "github", Host: "github.com", Repository: rrRepo}
	id := rf.ChangeIdentity{Repository: repo, Number: 77, Head: f.head, Base: f.base, BaseUsed: true}
	if change == "head" {
		id.Head = strings.Repeat("e", 40)
	}
	if change == "base" {
		id.Base = strings.Repeat("e", 40)
	}
	if change == "subject" {
		id.Number = 78
	}
	prov := rf.InputProvenance{Source: "fixture", Version: "v1", Revision: "set1", ObservedAt: now, Freshness: "fresh", Complete: true}
	ap := rf.Applicability{Head: id.Head, Base: id.Base, Body: prov.Revision, Reviews: prov.Revision, Checks: prov.Revision}
	in := rf.ObservationInputs{Evaluator: rf.ObservationEvaluator, Policy: "untrusted-advisory-policy", ReviewerDisplay: "reviewer", Requested: []rf.Repository{repo}, Admitted: []rf.Repository{repo}, Collection: rf.CollectionInterval{Start: now, End: now}, Population: []rf.PopulationCoverage{{Repository: repo, Input: prov, Count: 1}}, Changes: []rf.ObservedChange{{Identity: id, AppliesTo: ap, Inputs: rf.ReviewInputs{Subject: prov, Body: prov, Reviews: prov, Checks: prov, Enrichment: prov}, Facts: rf.ClassificationFacts{Change: rf.ChangeFact{Labels: []string{}, Checks: []rf.CheckFact{}}}, Lanes: []rf.LaneConclusion{{Name: "correctness", State: "missing", AppliesTo: ap}, {Name: "security", State: "missing", AppliesTo: ap}}}}}
	o, e := rf.BuildObservation(in, rf.ObservationPolicy{Now: now, Oldest: now, Policy: in.Policy})
	if e != nil {
		t.Fatal(e)
	}
	if change == "partial" {
		o.Inputs.Population[0].Input.Complete = false
	}
	if change == "schema" {
		o.Schema = "unknown"
	}
	b, e := json.Marshal(o)
	if e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(t.TempDir(), "observation.json")
	if e = os.WriteFile(path, b, 0600); e != nil {
		t.Fatal(e)
	}
	return path
}
func TestReviewPrepareObservation(t *testing.T) {
	for _, change := range []string{"valid", "head", "base", "subject", "partial", "schema"} {
		t.Run(change, func(t *testing.T) {
			f := prepareTestFixture(t)
			path := savedPrepareObservation(t, f, change)
			p, e := newPreparedReview(dispatchOpts{kit: "review", pr: 77, root: f.root, observation: path}, rrRepo)
			if change == "valid" {
				if e != nil || p == nil {
					t.Fatal("valid advisory observation refused", e)
				}
				defer p.cancel()
				f.moved = "body"
				if p.recheck() == nil {
					t.Fatal("saved observation bypassed current body recheck")
				}
			} else {
				if e == nil {
					if p != nil {
						p.cancel()
					}
					t.Fatal("invalid advisory observation accepted " + change)
				}
			}
		})
	}
}

type prepareRoutingTransport struct{ preparation, effects http.RoundTripper }

func (t prepareRoutingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.URL.Hostname() == "api.github.com" {
		return t.preparation.RoundTrip(r)
	}
	return t.effects.RoundTrip(r)
}

func TestReviewPrepareKeys(t *testing.T) {
	f := prepareTestFixture(t)
	p, _ := prepareFlow(t, f)
	p.local = nil
	for i := 0; i < 2; i++ {
		if _, e := p.ReadFile(p.repo, deskkit.ReadFileInput{File: "a.go", Ref: f.head}); e != nil {
			t.Fatal(e)
		}
	}
	calls := func(part string) int {
		n := 0
		for _, a := range f.transport.Snapshot() {
			if strings.Contains(a.Path, part) {
				n++
			}
		}
		return n
	}
	if calls("/contents/") != 1 {
		t.Fatal("immutable provider content read repeated")
	}
	if _, e := p.ReadFile(p.repo, deskkit.ReadFileInput{File: "a.go", Ref: f.base}); e == nil {
		t.Fatal("another source head reused assigned bytes")
	}
	if _, e := p.ReadFile(p.repo, deskkit.ReadFileInput{File: "../a.go", Ref: f.head}); e == nil {
		t.Fatal("unsafe path reused source bytes")
	}
	before := calls("/compare/")
	for i := 0; i < 2; i++ {
		if _, e := p.CompareRefs(p.repo, f.base, f.head); e != nil {
			t.Fatal(e)
		}
	}
	if calls("/compare/") != before {
		t.Fatal("exact endpoint comparison from round was not reused")
	}
	for i := 0; i < 2; i++ {
		if _, e := p.CompareRefs(p.repo, "main", f.head); e != nil {
			t.Fatal(e)
		}
	}
	if calls("/compare/") != before+2 {
		t.Fatal("mutable branch comparison was cached")
	}
	if _, e := p.CompareRefs(p.repo, strings.Repeat("d", 40), f.head); e != nil {
		t.Fatal(e)
	}
	if calls("/compare/") != before+3 {
		t.Fatal("comparison reused another base endpoint")
	}
}

func TestReviewPrepareMissingBase(t *testing.T) {
	f := prepareTestFixture(t)
	f.moved = "missing-base"
	p, spec := prepareFlow(t, f)
	o := dispatchOpts{kit: "review", pr: 77, root: f.root}
	round, e := prepareReviewRound(o, dispatchPlan{claimKey: rrKeyC, reviewContext: p}, rrRepo)
	if e != nil || !round.scope.full {
		t.Fatal("unknown exact base claimed delta coverage", e)
	}
	built, e := packet.Build(spec)
	if e != nil {
		t.Fatal(e)
	}
	if len(built.Omitted) == 0 || !strings.Contains(string(built.Text), "exact base endpoint") {
		t.Fatal("unknown base did not omit unfrozen diff")
	}
}

func TestReviewPrepareDispatchAbort(t *testing.T) {
	for _, change := range []string{"body", "access"} {
		t.Run(change, func(t *testing.T) {
			s, f, stamp, queue, args := prepareDispatchFixture(t)
			buildPacketFn = func(dispatchOpts, dispatchPlan, string, string) string { f.moved = change; return "" }
			rc, stderr := runCapturingStderr(t, args)
			want := "review preparation changed before dispatch"
			if change == "access" {
				want = "review preparation could not be rechecked"
			}
			if rc != deskkit.ExitUnverifiable || !strings.Contains(stderr, want) {
				t.Fatalf("dispatch did not abort on %s: rc=%d %s", change, rc, stderr)
			}
			released := false
			for _, call := range s.calls {
				if strings.Contains(strings.Join(call, " "), "release") {
					released = true
				}
			}
			if !released {
				t.Fatal("invalidated dispatch did not release its claim")
			}
			if len(stamp.requests) != 0 || len(*queue) != 0 {
				t.Fatal("invalidated dispatch wrote a stamp or queue label")
			}
			if _, e := os.Stat(emittedPathFor(promptFileOf(t, args))); !os.IsNotExist(e) {
				t.Fatal("invalidated dispatch emitted a prompt")
			}
		})
	}
}
func TestReviewPrepareBudgets(t *testing.T) {
	f := prepareTestFixture(t)
	old := reviewStageBudget
	reviewStageBudget = 100 * time.Millisecond
	t.Cleanup(func() { reviewStageBudget = old })
	o := dispatchOpts{kit: "review", pr: 77, root: f.root}
	p, e := newPreparedReview(o, rrRepo)
	if e != nil {
		t.Fatal(e)
	}
	defer func() { p.cancel() }()
	if _, e := prepareReviewRound(o, dispatchPlan{claimKey: rrKeyC, reviewContext: p}, rrRepo); e != nil {
		t.Fatal(e)
	}
	// Claim/workspace latency is outside the later packet budget.
	time.Sleep(150 * time.Millisecond)
	spec, e := reviewPacket(packetInput{o: o, plan: dispatchPlan{claimKey: rrKeyC, reviewContext: p}, repo: rrRepo})
	if e != nil {
		t.Fatal(e)
	}
	p.local = nil
	if _, e := p.ReadFile(p.repo, deskkit.ReadFileInput{Ref: f.head, File: "budget-probe.go"}); e != nil {
		t.Fatal("claim/workspace latency consumed packet budget", e)
	}
	f.blockContent = true
	built, e := packet.Build(spec)
	if e != nil {
		t.Fatal("packet expiry poisoned the fresh terminal check", e)
	}
	if len(built.Omitted) == 0 {
		t.Fatal("expired packet failed to report partial coverage")
	}
	if !p.checked {
		t.Fatal("packet expiry skipped the fresh terminal check")
	}
}
func TestReviewPrepareVolatileMetadata(t *testing.T) {
	p := &deskkit.PullRequest{HeadSHA: rrHead1, BaseSHA: rrMain, BaseRef: "main", Body: "description", State: "open"}
	n := *p
	n.Title = "new title"
	n.Mergeable = "MERGEABLE"
	n.UpdatedAt = "later"
	n.Labels = []string{"review-in-flight"}
	n.Draft = true
	if !samePreparedSubject(p, &n) {
		t.Fatal("volatile metadata spuriously invalidates the prepared source")
	}
	n.Labels = []string{decisionLabel}
	if samePreparedSubject(p, &n) {
		t.Fatal("decision-label transition bypassed gate freshness")
	}
}

type opaquePreparationForge struct{ deskkit.Forge }

func TestReviewPrepareUnknownIdentityFallback(t *testing.T) {
	f := prepareTestFixture(t)
	fg, fr, e := liveReviewRoundForge(rrRepo)
	if e != nil {
		t.Fatal(e)
	}
	opaque := opaquePreparationForge{Forge: fg.(deskkit.Forge)}
	reviewRoundForgeFn = func(string) (reviewRoundForge, deskkit.ForgeRepo, error) { return opaque, fr, nil }
	p, e := newPreparedReview(dispatchOpts{kit: "review", pr: 77, root: f.root}, rrRepo)
	if e != nil || p != nil {
		t.Fatal("unknown wrapper did not retain unprepared compatibility path", p, e)
	}
}
func TestReviewPrepareLocalOmission(t *testing.T) {
	f := prepareSourceFixture(t, strings.Repeat("x", packet.DefaultPerItemCap+1))
	_, spec := prepareFlow(t, f)
	built, e := packet.Build(spec)
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(string(built.Text), "local exact blob exceeds the packet per-item byte cap") {
		t.Fatal("local cap omission attributed to provider")
	}
}

func TestReviewPrepareDiffCoverage(t *testing.T) {
	for _, mode := range []string{"small-edit-large-file", "budget-fallback", "nonregular-fallback"} {
		t.Run(mode, func(t *testing.T) {
			size := 100 << 10
			if mode == "budget-fallback" {
				size = 5 << 20
			}
			body := strings.Repeat("x", size) + "\n"
			if mode == "small-edit-large-file" {
				body = strings.Repeat("unchanged fixture line\n", 5000)
			}
			f := prepareSourceFixture(t, body)
			git := func(args ...string) string {
				c := exec.Command("git", args...)
				c.Dir = f.root
				c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
				b, e := c.CombinedOutput()
				if e != nil {
					t.Fatal(e, string(b))
				}
				return strings.TrimSpace(string(b))
			}
			if mode != "budget-fallback" {
				git("update-ref", "HEAD", f.head)
				if mode == "small-edit-large-file" {
					if e := os.WriteFile(filepath.Join(f.root, "a.go"), []byte(body+"small edit\n"), 0600); e != nil {
						t.Fatal(e)
					}
				} else {
					if e := os.Remove(filepath.Join(f.root, "a.go")); e != nil {
						t.Fatal(e)
					}
					if e := os.Symlink("target", filepath.Join(f.root, "a.go")); e != nil {
						t.Skip(e)
					}
					f.symlink = true
				}
				git("add", "a.go")
				git("-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "edit")
				f.base = f.head
				f.head = git("rev-parse", "HEAD")
			}
			p, spec := prepareFlow(t, f)
			built, e := packet.Build(spec)
			if e != nil {
				t.Fatal(e)
			}
			text := string(built.Text)
			if mode == "small-edit-large-file" {
				if !p.localDiff || !strings.Contains(text, "small edit") {
					t.Fatal("small edit in large file lost its local diff")
				}
			} else {
				if !strings.Contains(text, "+assigned") {
					t.Fatal("local resource fallback dropped collected patches")
				}
				if p.localDiffSkipped == "" {
					t.Fatal("fixture did not exercise local input preflight")
				}
			}
			if f.diffReads != 0 {
				t.Fatal("bounded fallback issued an unbounded whole-provider diff request")
			}
		})
	}
}

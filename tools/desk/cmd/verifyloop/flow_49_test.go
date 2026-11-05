package main

// flow_49_test.go — the verifyloop half of the verifier filing identity contract (brief 49).
//
// Land files a bug for every failed or unlandable verify. The failure is one CASE (the repair
// obligation's immutable id), and a re-land of the same case must reconcile to the issue already
// filed for it — found by the exact identity through the forge the loop already reads with, in
// the repository the case names — instead of filing a second one under a different title. The
// store below is a small in-memory repository set, so what one Land files is what the next
// Land's lookup really sees.

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

const (
	flowTracker = "example-org/tracker"
	flowAgents  = "example-org/agents"

	// The filing App as the forge reports an issue author (pinned in the planted roster), and an
	// account the roster does not name.
	flowFilerLogin = "assay-verifier-app[bot]"
	flowFilerID    = 300000005
)

type flowStored struct {
	number int
	state  string
	title  string
	body   string
	labels []string

	author   string
	authorID int64
}

type flowRepoForge struct {
	deskkit.Forge
	repo  string
	world *flowWorld
}

type flowWorld struct {
	issues  map[string][]*flowStored
	failing bool // every filing-key read fails
	partial bool // every filing-key read is truncated
	reads   int
	filed   []string
}

func (f flowRepoForge) SearchIssues(r deskkit.ForgeRepo, in deskkit.SearchIssuesInput) ([]deskkit.IssueSearchResult, error) {
	if in.FilingKey == "" {
		return nil, errors.New("flow_49: only the exact lookup is expected here")
	}
	f.world.reads++
	if f.world.failing {
		return nil, deskkit.Unverifiable("dial tcp: connection refused", nil)
	}
	if f.world.partial {
		return nil, deskkit.Unverifiable("filing-key search is incomplete", nil)
	}
	var out []deskkit.IssueSearchResult
	// A deliberately sloppy index: it ignores both the repository and the open-state scope the
	// real query asks for, so the key's own repository component and the local confirmation are
	// what keep a lookup exact.
	for slug, issues := range f.world.issues {
		for _, is := range issues {
			if strings.Contains(is.body, deskkit.FilingKeyMarkerText(in.FilingKey)) {
				out = append(out, deskkit.IssueSearchResult{Number: is.number, Title: is.title, State: is.state,
					Labels: is.labels, Body: is.body, Author: is.author, AuthorID: is.authorID,
					URL: fmt.Sprintf("https://example.test/%s/issues/%d", slug, is.number)})
			}
		}
	}
	return out, nil
}

// flowDurable is the durable sink with the identity-carrying filer: what it files lands in the
// world the way `deskfile new --filing-brief --filing-case --raised-by verifier` would make it.
type flowDurable struct {
	*recordingDurable
	world *flowWorld
	title int
}

func (d *flowDurable) FileBugFor(it loopengine.Item, detail string, id FilingIdentity) (string, error) {
	key, err := deskkit.FilingKey(id.Repo, id.Brief, "verifier", id.Case)
	if err != nil {
		return "", err
	}
	d.title++
	n := 200 + len(d.world.issues[id.Repo]) + 1
	// A different, unrelated title every time: the title is not the identity.
	d.world.issues[id.Repo] = append(d.world.issues[id.Repo], &flowStored{number: n, state: "open",
		title:  fmt.Sprintf("verification report %d: %s", d.title, strings.Repeat("x", d.title)),
		body:   detail + "\n" + deskkit.FilingKeyMarker(key),
		labels: []string{"bug", "raised-by:verifier"}, author: flowFilerLogin, authorID: flowFilerID})
	d.world.filed = append(d.world.filed, id.Repo+"#"+fmt.Sprint(n))
	return fmt.Sprintf("https://example.test/%s/issues/%d", id.Repo, n), nil
}

func flowLoop(t *testing.T, w *flowWorld, repos ...string) (*VerifyLoop, *flowDurable, *bytes.Buffer) {
	t.Helper()
	plantAllowedRoster(t, repos...)
	d := &flowDurable{recordingDurable: newRec(), world: w}
	var out bytes.Buffer
	src := &forgeIssueSource{forgeFor: func(r deskkit.ForgeRepo) (deskkit.Forge, error) {
		return flowRepoForge{repo: r.Slug(), world: w}, nil
	}}
	v := &VerifyLoop{DurableSink: d, Now: fixedNow, Issues: src, Emit: &out}
	return v, d, &out
}

func flowFail(brief, repo, sha string, failing ...int) loopengine.Result {
	res := loopengine.Result{
		Item:     loopengine.Item{ID: brief, TargetSHA: sha, Payload: map[string]string{"repo": repo}},
		Verdict:  loopengine.VerdictFail,
		RunnerID: "local:glm",
	}
	for i := 1; i <= 3; i++ {
		exit := 0
		for _, f := range failing {
			if f == i {
				exit = 1
			}
		}
		res.Rows = append(res.Rows, loopengine.EvidenceRow{Command: fmt.Sprintf("check-%d", i), Exit: exit, Output: "out"})
	}
	return res
}

func flowLand(t *testing.T, v *VerifyLoop, r loopengine.Result) error {
	t.Helper()
	admitFixture(v, r.Item)
	return v.Land(r)
}

// regression: example-stream/49
func TestFlowFileKey(t *testing.T) {
	t.Run("the same case landed again reconciles instead of filing under another title", func(t *testing.T) {
		w := &flowWorld{issues: map[string][]*flowStored{}}
		v, _, out := flowLoop(t, w, flowTracker)
		r := flowFail("example-stream/49", flowTracker, "abc1234", 2)
		for i := 0; i < 3; i++ {
			// Different clocks and different failure output must not change the case.
			r.Rows[1].Output = fmt.Sprintf("different output on attempt %d", i)
			if err := flowLand(t, v, r); err != nil {
				t.Fatalf("land %d: %v", i, err)
			}
		}
		if len(w.filed) != 1 {
			t.Fatalf("filed %v, want exactly one issue for one case", w.filed)
		}
		if !strings.Contains(out.String(), "already filed as "+flowTracker+"#201") {
			t.Fatalf("the reconcile must print the existing number; output:\n%s", out.String())
		}
	})

	t.Run("a second independent case and other roots stay separate", func(t *testing.T) {
		w := &flowWorld{issues: map[string][]*flowStored{}}
		v, _, _ := flowLoop(t, w, flowTracker, flowAgents)
		for _, r := range []loopengine.Result{
			flowFail("example-stream/49", flowTracker, "abc1234", 2),
			// A different failing row set on the same brief and source.
			flowFail("example-stream/49", flowTracker, "abc1234", 3),
			// A different source receipt for the same rows.
			flowFail("example-stream/49", flowTracker, "def5678", 2),
			// Another brief with the same sha and rows.
			flowFail("example-stream/50", flowTracker, "abc1234", 2),
			// The very same brief, sha and rows in another root.
			flowFail("example-stream/49", flowAgents, "abc1234", 2),
		} {
			if err := flowLand(t, v, r); err != nil {
				t.Fatalf("land %s %s: %v", r.Item.Payload["repo"], r.Item.ID, err)
			}
		}
		if len(w.filed) != 5 {
			t.Fatalf("filed %v, want five separate issues", w.filed)
		}
		// Every one of them is found again by its own identity and none files anything new.
		if err := flowLand(t, v, flowFail("example-stream/49", flowAgents, "abc1234", 2)); err != nil {
			t.Fatal(err)
		}
		if len(w.filed) != 5 {
			t.Fatalf("a repeat in the second root filed again: %v", w.filed)
		}
	})

	t.Run("an unreadable or partial search files nothing", func(t *testing.T) {
		for name, set := range map[string]func(*flowWorld){
			"unreadable": func(w *flowWorld) { w.failing = true },
			"truncated":  func(w *flowWorld) { w.partial = true },
		} {
			t.Run(name, func(t *testing.T) {
				w := &flowWorld{issues: map[string][]*flowStored{}}
				set(w)
				v, d, out := flowLoop(t, w, flowTracker)
				err := flowLand(t, v, flowFail("example-stream/49", flowTracker, "abc1234", 2))
				if err == nil {
					t.Fatal("an unknown read must surface as a land error, not as a filed bug")
				}
				if len(w.filed) != 0 || len(d.bugs) != 0 {
					t.Fatalf("filed %v / bugs %v — an unknown read must file nothing", w.filed, d.bugs)
				}
				if !strings.Contains(out.String(), "could-not-check") {
					t.Fatalf("the unknown must be named; output:\n%s", out.String())
				}
			})
		}
	})

	t.Run("a repository outside the configured set is unknown and files nothing", func(t *testing.T) {
		w := &flowWorld{issues: map[string][]*flowStored{}}
		v, d, _ := flowLoop(t, w, flowTracker)
		if err := flowLand(t, v, flowFail("example-stream/49", "other-owner/elsewhere", "abc1234", 2)); err == nil {
			t.Fatal("an out-of-set repository must not be read or filed into")
		}
		if len(w.filed) != 0 || len(d.bugs) != 0 || w.reads != 0 {
			t.Fatalf("filed=%v bugs=%v reads=%d, want none", w.filed, d.bugs, w.reads)
		}
	})

	t.Run("closed, foreign-role and prose-only mentions are not the case's filing", func(t *testing.T) {
		w := &flowWorld{issues: map[string][]*flowStored{}}
		v, _, _ := flowLoop(t, w, flowTracker)
		r := flowFail("example-stream/49", flowTracker, "abc1234", 2)
		_, key, ok := repairFilingIdentity(buildRepairObligation(r, fixedNow().Format("2006-01-02"), ""))
		if !ok {
			t.Fatal("test void: the failure has no filing identity")
		}
		m := deskkit.FilingKeyMarker(key)
		w.issues[flowTracker] = []*flowStored{
			{number: 1, state: "closed", title: "closed", body: m, labels: []string{"raised-by:verifier"}, author: flowFilerLogin, authorID: flowFilerID},
			{number: 2, state: "open", title: "other role", body: m, labels: []string{"raised-by:worker"}, author: flowFilerLogin, authorID: flowFilerID},
			{number: 3, state: "open", title: "prose mention", body: "see filing-key: " + key, author: flowFilerLogin, authorID: flowFilerID},
		}
		if err := flowLand(t, v, r); err != nil {
			t.Fatal(err)
		}
		if len(w.filed) != 1 {
			t.Fatalf("filed %v, want the case filed once — none of the planted issues is its open verifier filing", w.filed)
		}
	})

	t.Run("the loop has no override; closing the filing is what allows a fresh one", func(t *testing.T) {
		// The audited override (--force-new with a reason) is deskfile's and is covered there; the
		// loop never passes one. What the loop must not do is treat a CLOSED filing as the case's
		// open one: once a human closes it, the same case may be filed again.
		w := &flowWorld{issues: map[string][]*flowStored{}}
		v, _, _ := flowLoop(t, w, flowTracker)
		r := flowFail("example-stream/49", flowTracker, "abc1234", 2)
		if err := flowLand(t, v, r); err != nil {
			t.Fatal(err)
		}
		w.issues[flowTracker][0].state = "closed"
		if err := flowLand(t, v, r); err != nil {
			t.Fatal(err)
		}
		if len(w.filed) != 2 {
			t.Fatalf("filed %v, want a fresh filing after the first was closed", w.filed)
		}
	})

	t.Run("a result without a deliverable repository keeps the legacy filing", func(t *testing.T) {
		w := &flowWorld{issues: map[string][]*flowStored{}}
		v, d, _ := flowLoop(t, w, flowTracker)
		r := flowFail("example-stream/49", "", "abc1234", 2)
		if err := flowLand(t, v, r); err != nil {
			t.Fatal(err)
		}
		if len(d.bugs) != 1 || len(w.filed) != 0 || w.reads != 0 {
			t.Fatalf("bugs=%v filed=%v reads=%d, want one legacy FileBug and no exact lookup", d.bugs, w.filed, w.reads)
		}
	})

	// regression: verifier-filing-identity-inert-in-production-item-shape (review finding F1)
	t.Run("the production item shapes carry a filing identity and dedup", func(t *testing.T) {
		// The multi-root scanner names an item "<owner>/<repo>:<stream>/<NN>" and sets payload
		// repo; the single-root scanner names it by the bare "<stream>/<NN>" with no payload repo,
		// the plan's own repository being the loop's RootRepo.
		shapes := map[string]struct {
			id, payloadRepo, rootRepo string
		}{
			"multi-root prefixed id":  {flowTracker + ":example-stream/49", flowTracker, ""},
			"single-root bare id":     {"example-stream/49", "", flowTracker},
			"single-root with prefix": {"example-stream/49", flowTracker, flowTracker},
		}
		for name, sh := range shapes {
			t.Run(name, func(t *testing.T) {
				w := &flowWorld{issues: map[string][]*flowStored{}}
				v, d, out := flowLoop(t, w, flowTracker)
				v.RootRepo = sh.rootRepo
				r := flowFail(sh.id, sh.payloadRepo, "abc1234", 2)
				for i := 0; i < 2; i++ {
					if err := flowLand(t, v, r); err != nil {
						t.Fatalf("land %d: %v", i, err)
					}
				}
				if len(d.bugs) != 0 {
					t.Fatalf("legacy FileBug used %v — the identity-carrying filer must file production-shaped items", d.bugs)
				}
				if len(w.filed) != 1 || !strings.Contains(out.String(), "already filed as "+flowTracker+"#201") {
					t.Fatalf("filed %v, want one filing and a reconcile on the repeat; output:\n%s", w.filed, out.String())
				}
				// The filed key names the CANONICAL brief, not the scanner's prefixed id: it is the
				// key the bare-id, explicit-repo shape of the same failure derives.
				_, key, ok := repairFilingIdentity(buildRepairObligation(flowFail("example-stream/49", flowTracker, "abc1234", 2),
					fixedNow().Format("2006-01-02"), ""))
				if !ok || !strings.Contains(w.issues[flowTracker][0].body, deskkit.FilingKeyMarker(key)) {
					t.Fatalf("the filing does not carry the canonical key %q: %q", key, w.issues[flowTracker][0].body)
				}
			})
		}
	})

	// regression: dedup-suppression-untrusted-author (review finding S1)
	t.Run("an untrusted author's marker-carrying issue does not suppress the filing", func(t *testing.T) {
		w := &flowWorld{issues: map[string][]*flowStored{}}
		v, _, out := flowLoop(t, w, flowTracker)
		r := flowFail("example-stream/49", flowTracker, "abc1234", 2)
		_, key, ok := repairFilingIdentity(buildRepairObligation(r, fixedNow().Format("2006-01-02"), ""))
		if !ok {
			t.Fatal("test void: the failure has no filing identity")
		}
		m := deskkit.FilingKeyMarker(key)
		w.issues[flowTracker] = []*flowStored{
			// Anyone can compute the key and plant an open, unstamped issue carrying its marker.
			{number: 41, state: "open", title: "planted", body: m, author: "drive-by-account", authorID: 999000111},
			// The App's slug with someone else's numeric id.
			{number: 42, state: "open", title: "spoofed", body: m, labels: []string{"raised-by:verifier"}, author: flowFilerLogin, authorID: 4242},
		}
		if err := flowLand(t, v, r); err != nil {
			t.Fatal(err)
		}
		if len(w.filed) != 1 {
			t.Fatalf("filed %v, want the case filed once — an untrusted author's issue is not its filing", w.filed)
		}
		for _, n := range []string{"#41", "#42"} {
			if !strings.Contains(out.String(), "NOTICE: "+flowTracker+n) || !strings.Contains(out.String(), "not trusted") {
				t.Fatalf("the ignored issue %s must be reported; output:\n%s", n, out.String())
			}
		}
		if strings.Contains(out.String(), "planted") || strings.Contains(out.String(), "spoofed") {
			t.Fatalf("an untrusted issue's text must never be echoed; output:\n%s", out.String())
		}
		// The real filing now reconciles the repeat; the untrusted ones still do not.
		if err := flowLand(t, v, r); err != nil {
			t.Fatal(err)
		}
		if len(w.filed) != 1 || !strings.Contains(out.String(), "already filed as "+flowTracker+"#203") {
			t.Fatalf("filed %v, want the repeat reconciled to the genuine filing #203; output:\n%s", w.filed, out.String())
		}
	})
}

package main

// flow_49_test.go — the deskfile half of the verifier filing identity contract (brief 49).
//
// The property under test is not "a flag exists". It is that ONE exact identity — repository,
// canonical brief, raising role and case — is consulted before the fuzzy title matcher, that it
// suppresses exactly the repeated observations of that case and nothing else, and that every way
// the exact read can fail files nothing. The fixture is a small in-memory repository store rather
// than canned search hits, so a filing made by one run is what the next run's lookup actually sees.

import (
	"fmt"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const (
	flowOtherRepo = "example-org/tracker"
	flowCaseA     = "repair/0123456789abcdef"
	flowCaseB     = "repair/fedcba9876543210"

	// The filing App's identity as the forge reports an issue author (a roster-pinned bot), and
	// an account the roster does not name.
	flowTrustedAuthor   = "assay-verifier-app[bot]"
	flowTrustedAuthorID = 300000005
	flowStranger        = "drive-by-account"
	flowStrangerID      = 999000111
)

type flowIssue struct {
	number int
	state  string
	title  string
	body   string
	labels []string

	author   string
	authorID int64
}

// flowForge is a store-backed deskkit.Forge for the filing-key flow: per-repository issues, an
// exact filing-key search that behaves like the typed backend (complete-or-error), and a create
// that can land while reporting failure.
type flowForge struct {
	*dfForge
	store map[string][]*flowIssue

	searchErr error // every search fails (unreadable)
	partial   bool  // the exact search reports a truncated walk
	lostReply bool  // FileIssue stores the issue but reports a transport failure
	creates   int
	exact     int // exact filing-key searches served
}

// seed plants an issue written by the trusted filing App.
func (f *flowForge) seed(repo string, n int, state, title, body string, labels ...string) {
	f.seedBy(flowTrustedAuthor, flowTrustedAuthorID, repo, n, state, title, body, labels...)
}

// seedBy plants an issue written by an arbitrary author.
func (f *flowForge) seedBy(author string, authorID int64, repo string, n int, state, title, body string, labels ...string) {
	f.store[repo] = append(f.store[repo], &flowIssue{number: n, state: state, title: title, body: body, labels: labels,
		author: author, authorID: authorID})
}

func (f *flowForge) SearchIssues(repo deskkit.ForgeRepo, in deskkit.SearchIssuesInput) ([]deskkit.IssueSearchResult, error) {
	if f.searchErr != nil {
		return nil, f.searchErr
	}
	var out []deskkit.IssueSearchResult
	if in.FilingKey != "" {
		f.exact++
		if f.partial {
			return nil, deskkit.Unverifiable("filing-key search is incomplete", nil)
		}
		// A deliberately sloppy index: it ignores both the repository and the open-state scope the
		// real query asks for, so the key's own repository component and the local confirmation
		// are what keep a lookup exact.
		for slug, issues := range f.store {
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
	for _, is := range f.store[repo.Slug()] {
		if is.state == "open" {
			out = append(out, deskkit.IssueSearchResult{Number: is.number, Title: is.title, State: is.state, Labels: is.labels,
				URL: fmt.Sprintf("https://example.test/%s/issues/%d", repo.Slug(), is.number)})
		}
	}
	return out, nil
}

func (f *flowForge) FileIssue(repo deskkit.ForgeRepo, in deskkit.IssueInput) (*deskkit.IssueRef, error) {
	f.creates++
	n := 100 + len(f.store[repo.Slug()]) + 1
	f.store[repo.Slug()] = append(f.store[repo.Slug()], &flowIssue{number: n, state: "open", title: in.Title, body: in.Body,
		author: flowTrustedAuthor, authorID: flowTrustedAuthorID})
	if f.lostReply {
		return nil, deskkit.Unverifiable("HTTP 502: response lost", nil)
	}
	return &deskkit.IssueRef{Number: n, URL: fmt.Sprintf("https://example.test/%s/issues/%d", repo.Slug(), n)}, nil
}

func (f *flowForge) ApplyLabels(repo deskkit.ForgeRepo, number int, change deskkit.LabelChange) (*deskkit.LabelOutcome, error) {
	out, err := f.dfForge.ApplyLabels(repo, number, change)
	if err != nil {
		return out, err
	}
	for _, is := range f.store[repo.Slug()] {
		if is.number == number {
			for _, l := range change.Add {
				is.labels = append(is.labels, l.Name)
			}
		}
	}
	return out, nil
}

// flowSetup installs the store-backed forge over withEnv's fake and plants the labels the stamp
// resolves against. The new-issue rate is raised so a case matrix is not a rate test.
func flowSetup(t *testing.T) *flowForge {
	t.Helper()
	withEnv(t)
	t.Setenv("FAKEGH_LABELS", labelsJSON(t, "raised-by:verifier", "raised-by:worker"))
	t.Setenv("ASSAY_DESKFILE_NEW_RATE", "50")
	ff := &flowForge{dfForge: curForge, store: map[string][]*flowIssue{}}
	prev := forgeForFn
	forgeForFn = func(repo string, ro bool) (deskkit.Forge, deskkit.ForgeRepo, deskkit.ForgeKind, error) {
		_, fr, kind, err := prev(repo, ro)
		return deskkit.OutboundChecked(ff, "worker"), fr, kind, err
	}
	t.Cleanup(func() { forgeForFn = prev })
	return ff
}

// flowFile runs `deskfile new` with a structured identity and returns the exit code and output.
func flowFile(t *testing.T, repo, title, brief, caseID, role string, extra ...string) (int, string) {
	t.Helper()
	args := []string{"new", "-R", repo, "--title", title, "--body-file", bodyFileWith(t, "observation for "+title),
		"--raised-by", role}
	if brief != "" {
		args = append(args, "--filing-brief", brief, "--filing-case", caseID)
	}
	return runCapture(append(args, extra...))
}

// regression: example-stream/49
func TestFlowFileKey(t *testing.T) {
	t.Run("same case under unrelated titles is deduped", func(t *testing.T) {
		ff := flowSetup(t)
		if rc, out := flowFile(t, allowedRepo, "oracle price feed went stale", "example-stream/49", flowCaseA, "verifier"); rc != deskkit.ExitOK {
			t.Fatalf("first filing rc = %d; out=%s", rc, out)
		}
		filed := ff.store[allowedRepo][0]
		if !strings.Contains(filed.body, "filing-key: fk1/") {
			t.Fatalf("the filed body must carry the filing-key marker; body=%q", filed.body)
		}
		// A different title, a different spelling of the same brief, the same case.
		for _, tc := range []struct{ title, brief string }{
			{"settlement choice rejects mature vault", "example-stream/49"},
			{"completely different wording about a queue", "assay:assay:example-stream:49"},
			{"Example-Stream/49 verification failure", "ASSAY:assay:example-stream:49"},
		} {
			if score := matchScore(tokenize(tc.title), filed.title, false); score >= matchThreshold {
				t.Fatalf("test void: title %q already trips the fuzzy matcher (score %.2f)", tc.title, score)
			}
			rc, out := flowFile(t, allowedRepo, tc.title, tc.brief, flowCaseA, "verifier")
			if rc != deskkit.ExitRefused {
				t.Fatalf("%q repeated observation rc = %d, want 5; out=%s", tc.title, rc, out)
			}
			if !strings.Contains(out, fmt.Sprintf("#%d", filed.number)) || !strings.Contains(out, "already filed") {
				t.Fatalf("refusal must print the existing issue number #%d; out=%s", filed.number, out)
			}
		}
		if ff.creates != 1 {
			t.Fatalf("creates = %d, want exactly 1 for one case", ff.creates)
		}
	})

	t.Run("without an identity only the title matcher runs", func(t *testing.T) {
		ff := flowSetup(t)
		for _, title := range []string{"oracle price feed went stale", "settlement choice rejects mature vault"} {
			if rc, out := flowFile(t, allowedRepo, title, "", "", "verifier"); rc != deskkit.ExitOK {
				t.Fatalf("%q rc = %d; out=%s", title, rc, out)
			}
		}
		if ff.creates != 2 || ff.exact != 0 {
			t.Fatalf("creates=%d exact lookups=%d, want 2 and 0 — a filing without an identity must not use the exact lookup", ff.creates, ff.exact)
		}
		rc, out := flowFile(t, allowedRepo, "oracle price feed went stale again", "", "", "verifier")
		if rc != deskkit.ExitRefused || !strings.Contains(out, deskkit.DedupeRefusalPrefix) {
			t.Fatalf("legacy near-duplicate rc = %d, want the title refusal; out=%s", rc, out)
		}
	})

	t.Run("distinct cases and cross-root same numbers remain separate", func(t *testing.T) {
		ff := flowSetup(t)
		type filing struct{ repo, title, brief, caseID, role string }
		for i, f := range []filing{
			{allowedRepo, "alpha first wording", "example-stream/49", flowCaseA, "verifier"},
			// A second independent case on the same brief: a different source receipt.
			{allowedRepo, "beta second wording", "example-stream/49", flowCaseB, "verifier"},
			// An unrelated defect class on another brief of the same stream, same case ids.
			{allowedRepo, "gamma third wording", "example-stream/50", flowCaseA, "verifier"},
			// The same numbers in another root.
			{flowOtherRepo, "delta fourth wording", "example-stream/49", flowCaseA, "verifier"},
			// The same case raised by a different role.
			{allowedRepo, "epsilon fifth wording", "example-stream/49", flowCaseA, "worker"},
		} {
			if rc, out := flowFile(t, f.repo, f.title, f.brief, f.caseID, f.role); rc != deskkit.ExitOK {
				t.Fatalf("filing %d (%+v) rc = %d, want 0 — it must not be suppressed; out=%s", i, f, rc, out)
			}
		}
		if ff.creates != 5 {
			t.Fatalf("creates = %d, want 5 independent filings", ff.creates)
		}
		// Each is still found by its own identity.
		if rc, _ := flowFile(t, flowOtherRepo, "again other root", "example-stream/49", flowCaseA, "verifier"); rc != deskkit.ExitRefused {
			t.Fatalf("a repeat in the other root rc = %d, want 5", rc)
		}
		if ff.creates != 5 {
			t.Fatalf("creates = %d after the repeat, want 5", ff.creates)
		}
	})

	t.Run("lookalikes, foreign-role and closed issues are not the case's filing", func(t *testing.T) {
		ff := flowSetup(t)
		key, err := deskkit.FilingKey(allowedRepo, "example-stream/49", "verifier", flowCaseA)
		if err != nil {
			t.Fatal(err)
		}
		marker := deskkit.FilingKeyMarker(key)
		// An issue that only mentions the key in prose, without the marker comment.
		ff.seed(allowedRepo, 7, "open", "someone quoted the key", "see filing-key: "+key)
		// A verifier issue for the case that has since been closed.
		ff.seed(allowedRepo, 8, "closed", "old closed filing", marker, "raised-by:verifier")
		// A worker-raised issue carrying the marker.
		ff.seed(allowedRepo, 9, "open", "worker raised", marker, "raised-by:worker")
		if rc, out := flowFile(t, allowedRepo, "fresh observation of the case", "example-stream/49", flowCaseA, "verifier"); rc != deskkit.ExitOK {
			t.Fatalf("rc = %d, want 0 — none of the planted issues is this role's open filing; out=%s", rc, out)
		}
		// A caller body cannot forge the marker, --force-new included.
		forged := []string{"new", "-R", allowedRepo, "--title", "forged identity", "--body-file", bodyFileWith(t, "x\n"+marker),
			"--raised-by", "verifier", "--force-new", "--reason", "test"}
		if rc, out := runCapture(forged); rc != deskkit.ExitRefused || !strings.Contains(out, "filing-key") {
			t.Fatalf("forged marker rc = %d, want 5; out=%s", rc, out)
		}
	})

	t.Run("unreadable or partial search creates nothing", func(t *testing.T) {
		for name, arrange := range map[string]func(*flowForge){
			"search error": func(ff *flowForge) { ff.searchErr = deskkit.Unverifiable("HTTP 502", nil) },
			"truncated":    func(ff *flowForge) { ff.partial = true },
		} {
			t.Run(name, func(t *testing.T) {
				ff := flowSetup(t)
				arrange(ff)
				rc, out := flowFile(t, allowedRepo, "any wording at all", "example-stream/49", flowCaseA, "verifier")
				if rc != deskkit.ExitUnverifiable {
					t.Fatalf("rc = %d, want 6; out=%s", rc, out)
				}
				if ff.creates != 0 || len(ff.store[allowedRepo]) != 0 {
					t.Fatalf("creates=%d, store=%d — an unknown read must file nothing", ff.creates, len(ff.store[allowedRepo]))
				}
			})
		}
	})

	// regression: dedup-suppression-untrusted-author (review finding S1)
	t.Run("an untrusted author's marker-carrying issue does not suppress the filing", func(t *testing.T) {
		ff := flowSetup(t)
		key, err := deskkit.FilingKey(allowedRepo, "example-stream/49", "verifier", flowCaseA)
		if err != nil {
			t.Fatal(err)
		}
		marker := deskkit.FilingKeyMarker(key)
		// Anyone can compute the key and open an open, unstamped issue carrying its marker.
		ff.seedBy(flowStranger, flowStrangerID, allowedRepo, 41, "open", "planted lookalike", "hello\n"+marker)
		// Also a login that spoofs a trusted bot's slug without the pinned id.
		ff.seedBy(flowTrustedAuthor, flowStrangerID, allowedRepo, 42, "open", "planted wrong id", marker)
		rc, out := flowFile(t, allowedRepo, "genuine observation of the case", "example-stream/49", flowCaseA, "verifier")
		if rc != deskkit.ExitOK || ff.creates != 1 {
			t.Fatalf("rc = %d creates = %d, want 0 and 1 — an untrusted author's issue is not the case's filing; out=%s", rc, ff.creates, out)
		}
		if !strings.Contains(out, "#41") || !strings.Contains(out, "not trusted") {
			t.Fatalf("the ignored issue must be reported by number; out=%s", out)
		}
		if strings.Contains(out, "planted lookalike") {
			t.Fatalf("an untrusted issue's text must never be echoed; out=%s", out)
		}
		// The genuine filing now suppresses a repeat, and the untrusted issues still do not.
		rc, out = flowFile(t, allowedRepo, "yet another wording", "example-stream/49", flowCaseA, "verifier")
		if rc != deskkit.ExitRefused || !strings.Contains(out, "#103") || strings.Contains(out, "#41)") {
			t.Fatalf("repeat rc = %d, want 5 naming the genuine filing #103; out=%s", rc, out)
		}
	})

	t.Run("an unstamped filing is still the case's filing", func(t *testing.T) {
		// The raised-by stamp is applied after the create and may fail without failing the
		// filing; the lookup must not depend on it, or the half-finished filing is filed twice.
		ff := flowSetup(t)
		key, _ := deskkit.FilingKey(allowedRepo, "example-stream/49", "verifier", flowCaseA)
		ff.seed(allowedRepo, 31, "open", "left behind without a stamp", deskkit.FilingKeyMarker(key))
		rc, out := flowFile(t, allowedRepo, "another wording entirely", "example-stream/49", flowCaseA, "verifier")
		if rc != deskkit.ExitRefused || !strings.Contains(out, "#31") || ff.creates != 0 {
			t.Fatalf("rc = %d creates = %d, want 5 naming #31 and no create; out=%s", rc, ff.creates, out)
		}
	})

	t.Run("a create whose reply was lost is reconciled before another create", func(t *testing.T) {
		ff := flowSetup(t)
		ff.lostReply = true
		rc, out := flowFile(t, allowedRepo, "reply lost in transit", "example-stream/49", flowCaseA, "verifier")
		if rc != deskkit.ExitUnverifiable {
			t.Fatalf("lost-reply rc = %d, want 6; out=%s", rc, out)
		}
		if ff.creates != 1 {
			t.Fatalf("creates = %d, want 1", ff.creates)
		}
		if !strings.Contains(out, "#101") || !strings.Contains(out, "do not re-file") {
			t.Fatalf("the ambiguous create must be reconciled to the issue that landed; out=%s", out)
		}
		// The issue exists but the run that made it never applied the role stamp; the retry must
		// still not create a second one.
		ff.lostReply = false
		rc, out = flowFile(t, allowedRepo, "retry with other words", "example-stream/49", flowCaseA, "verifier")
		if rc != deskkit.ExitRefused || !strings.Contains(out, "#101") {
			t.Fatalf("retry rc = %d, want 5 naming #101; out=%s", rc, out)
		}
		if ff.creates != 1 {
			t.Fatalf("creates = %d after the retry, want 1", ff.creates)
		}
	})

	t.Run("audited override still records its reason", func(t *testing.T) {
		ff := flowSetup(t)
		if rc, out := flowFile(t, allowedRepo, "first observation", "example-stream/49", flowCaseA, "verifier"); rc != deskkit.ExitOK {
			t.Fatalf("rc = %d; out=%s", rc, out)
		}
		// Without the override the repeat is refused.
		if rc, _ := flowFile(t, allowedRepo, "second observation", "example-stream/49", flowCaseA, "verifier"); rc != deskkit.ExitRefused {
			t.Fatalf("repeat rc = %d, want 5", rc)
		}
		// With it, the filing goes through and the reason is on the audit line.
		rc, out := flowFile(t, allowedRepo, "second observation", "example-stream/49", flowCaseA, "verifier",
			"--force-new", "--reason", "the first filing was against a superseded receipt")
		if rc != deskkit.ExitOK {
			t.Fatalf("override rc = %d; out=%s", rc, out)
		}
		if ff.creates != 2 {
			t.Fatalf("creates = %d, want 2", ff.creates)
		}
		detail := lastNewDetail(t)
		if !strings.Contains(detail, "force-new: the first filing was against a superseded receipt") || !strings.Contains(detail, "filing-key fk1/") {
			t.Fatalf("audit detail must carry the reason and the key; got %q", detail)
		}
	})

	t.Run("an incomplete or role-less identity is refused before any read", func(t *testing.T) {
		ff := flowSetup(t)
		body := bodyFileWith(t, "x")
		for _, args := range [][]string{
			{"new", "-R", allowedRepo, "--title", "t one", "--body-file", body, "--raised-by", "verifier", "--filing-brief", "example-stream/49"},
			{"new", "-R", allowedRepo, "--title", "t two", "--body-file", body, "--filing-brief", "example-stream/49", "--filing-case", flowCaseA},
			{"new", "-R", allowedRepo, "--title", "t three", "--body-file", body, "--raised-by", "verifier", "--filing-brief", "not a brief", "--filing-case", flowCaseA},
			{"new", "-R", allowedRepo, "--title", "t four", "--body-file", body, "--raised-by", "verifier", "--filing-brief", "example-stream/49", "--filing-case", "repair/zz"},
		} {
			if rc, out := runCapture(args); rc != deskkit.ExitRefused {
				t.Fatalf("%v rc = %d, want 5; out=%s", args, rc, out)
			}
		}
		if ff.creates != 0 || ff.exact != 0 {
			t.Fatalf("creates=%d exact=%d, want none", ff.creates, ff.exact)
		}
	})
}

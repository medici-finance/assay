package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

type adjudicatedRouteFixture struct {
	root, readme, brief string
	manifest            acceptanceManifest
	forge               *rlForge
	ruling              string
}

func writeRouteIssueFixture(t *testing.T, kind string, issue int, value any) {
	t.Helper()
	key := kind
	if kind == "label-events" {
		key = "labelEvents"
	}
	item := map[string]any{"repo": rlRepo, "number": issue, key: value}
	raw, err := json.Marshal(map[string]any{"schema": deskreadSchema, "kind": kind, "items": []any{item}})
	if err != nil {
		t.Fatal(err)
	}
	putFile(t, routeIssueFixturePath(kind, issue), string(raw))
}

func routeIssueFixturePath(kind string, issue int) string {
	return filepath.Join(os.Getenv("CREDIT_FAKE_DIR"), fmt.Sprintf("%s-%d.json", kind, issue))
}

func newAdjudicatedRouteFixture(t *testing.T, later, secondResult string) adjudicatedRouteFixture {
	t.Helper()
	return newAdjudicatedRouteFixtureWith(t, later, secondResult, false)
}

// newAdjudicatedRouteFixtureWith optionally lands a twin, vf/02: a second
// gate: human brief, committed with vf/01, carrying the same held Evidence.
// The route serves any such brief, so its records must never cross between
// them.
func newAdjudicatedRouteFixtureWith(t *testing.T, later, secondResult string, twin bool) adjudicatedRouteFixture {
	t.Helper()
	creditFakeDeskread(t)
	scanWithRoster(t, rosterWith(map[string]string{
		scanEnvTrustedBotSlugs: "desk=assay-desk-app:300000001,verifier=assay-verifier-app:300000005,worker=assay-worker-app:300000006",
	}))
	o := vfDefaults()
	o.gate, o.evidence = "human", ""
	o.verify = "| # | Command | Expect |\n|---|---|---|\n| 1 | `true` | exit 0 |\n"
	root, readme := vfFixture(t, o)
	vfGit(t, root, vfMailHuman, "remote", "add", "origin", "https://github.com/"+rlRepo+".git")
	path := vfBrief(root)
	twinOf := func() string { return strings.Replace(getFile(t, path), "brief: vf/01\n", "brief: vf/02\n", 1) }
	if twin {
		// The twin exists at the baseline, so its protected original table
		// resolves exactly as vf/01's does.
		putFile(t, adjudicatedTwinBrief(root), twinOf())
		putFile(t, readme, getFile(t, readme)+"| 02 | [t](./brief-02-flip.md) | 0 | S | implemented | — | "+vfReview+" |\n")
		vfCommit(t, root, vfMailHuman)
	}
	base := vfRev(t, root, "HEAD")
	putFile(t, filepath.Join(root, "delivery.txt"), "delivery\n")
	vfCommit(t, root, vfMailWorker)
	delivery := vfRev(t, root, "HEAD")
	rel, err := filepath.Rel(root, path)
	if err != nil {
		t.Fatal(err)
	}
	marker := fmt.Sprintf("Verification-Table: delivery=%s base=%s original=%s\n\n", delivery, base, base)
	runs := []acceptanceRun{{Run: fixtureRun, Source: base}, {Run: strings.Repeat("d", 64), Source: delivery}}
	evidence := "\n"
	for i, run := range runs {
		model := []string{"claude-opus-4", "claude-sonnet-4"}[i]
		issue := 51 + i
		for _, kind := range []string{"issue", "trust", "label-events"} {
			dir := os.Getenv("CREDIT_FAKE_DIR")
			old := filepath.Join(dir, fmt.Sprintf("%s-%d.json", kind, 41+i))
			data := strings.NewReplacer(creditRepo, rlRepo, creditBriefRel, filepath.ToSlash(rel),
				fixtureSource, run.Source, "claude-opus-4", model, "\"number\":"+strconv.Itoa(41+i), "\"number\":"+strconv.Itoa(issue)).Replace(getFile(t, old))
			putFile(t, filepath.Join(dir, fmt.Sprintf("%s-%d.json", kind, issue)), data)
		}
		body := creditBlock(t, creditAtt(rlRepo, issue, run.Run, run.Source, filepath.ToSlash(rel), model, "strong"), creditClosed)
		if i == 1 && secondResult != "" {
			span, bracket, ok := strings.Cut(body, admissionBracketPrefix)
			if !ok {
				t.Fatal("fixture lost bracket")
			}
			changed := strings.Replace(span, "pass exit=0", secondResult, 1)
			if secondResult == "omit row" {
				changed = strings.Replace(span, creditRow, strings.Replace(creditRow, "| 1 |", "| 2 |", 1), 1)
			}
			body = changed + admissionBracketPrefix + strings.Replace(bracket, digestOf(t, span), digestOf(t, changed), 1)
		}
		evidence += marker + body + "\n" + vfPass + "\n\n"
		if i == 0 {
			evidence += "An unrouted obligation is HELD pending disposition.\n\n"
		}
	}
	vfAppend(t, root, evidence+later)
	if twin {
		putFile(t, adjudicatedTwinBrief(root), twinOf())
	}
	vfCommit(t, root, vfMailVerifier)

	source, err := readAcceptanceSource(root, path)
	if err != nil {
		t.Fatal(err)
	}
	def, err := readAcceptanceDefinition(root, source)
	if err != nil {
		t.Fatal(err)
	}
	if len(source.Refusals) == 0 {
		t.Fatal("fixture needs a real ordinary hold")
	}
	m := acceptanceManifest{Source: source, AcceptanceVersion: def.version,
		Original: def.original, ProtectedVerifyHash: def.tableHash, Runs: runs}
	for _, refusal := range source.Refusals {
		m.Dispositions = append(m.Dispositions, acceptanceDisposition{
			Line: refusal.Line, Digest: refusal.Digest, Kind: "diagnostic-only", Explanation: "Synthetic diagnostic for this fixture.",
		})
	}
	f := adjudicatedRouteFixture{root: root, readme: readme, brief: path, manifest: m,
		forge: defaultRLForge(), ruling: rlURL(rlRepo, rlIssue, rlComment)}
	f.writePreparation(t, "assay-verifier-app[bot]", 300000005)
	human, err := acceptanceAdjudicationText(m, rlIssue)
	if err != nil {
		t.Fatal(err)
	}
	f.forge.commentBody = rlCommentJSONWith(rlHuman, "User", rlRepo, rlIssue, human, rlPosted, rlPosted)
	f.forge.issueBody = rlIssueJSON(acceptanceAdjudicationMarker(source.BriefPath))
	return f
}

func adjudicatedTwinBrief(root string) string {
	return filepath.Join(root, "docs", "streams", "vf", "brief-02-flip.md")
}

// adjudicate records m as the verifier's preparation and the human's exact
// adjudication of it on the fixture's decision issue.
func (f *adjudicatedRouteFixture) adjudicate(t *testing.T, m acceptanceManifest) {
	t.Helper()
	f.manifest = m
	f.writePreparation(t, "assay-verifier-app[bot]", 300000005)
	text, err := acceptanceAdjudicationText(m, rlIssue)
	if err != nil {
		t.Fatal(err)
	}
	f.forge.commentBody = rlCommentJSONWith(rlHuman, "User", rlRepo, rlIssue, text, rlPosted, rlPosted)
	f.forge.issueBody = rlIssueJSON(acceptanceAdjudicationMarker(m.Source.BriefPath))
}

// manifestFor prepares a fresh, otherwise valid manifest for the brief at path
// on the current checkout, reusing the fixture's runs and disposition shape.
func (f adjudicatedRouteFixture) manifestFor(t *testing.T, path string) acceptanceManifest {
	t.Helper()
	source, err := readAcceptanceSource(f.root, path)
	if err != nil {
		t.Fatal(err)
	}
	def, err := readAcceptanceDefinition(f.root, source)
	if err != nil {
		t.Fatal(err)
	}
	if len(source.Refusals) == 0 {
		t.Fatal("fixture brief needs a real ordinary hold")
	}
	m := acceptanceManifest{Source: source, AcceptanceVersion: def.version,
		Original: def.original, ProtectedVerifyHash: def.tableHash, Runs: f.manifest.Runs}
	for _, refusal := range source.Refusals {
		m.Dispositions = append(m.Dispositions, acceptanceDisposition{
			Line: refusal.Line, Digest: refusal.Digest, Kind: "diagnostic-only", Explanation: "Synthetic diagnostic for this fixture.",
		})
	}
	return m
}

func (f adjudicatedRouteFixture) writePreparation(t *testing.T, login string, id int64) {
	t.Helper()
	body, err := acceptancePreparationBody(f.manifest)
	if err != nil {
		t.Fatal(err)
	}
	writeRouteIssueFixture(t, "issue", rlIssue, creditIssue{Title: "Acceptance preparation: " + f.manifest.Source.BriefPath,
		Body: body, Author: &creditAccount{Login: login, ID: id, Type: "Bot"}})
	yes := true
	writeRouteIssueFixture(t, "trust", rlIssue, creditTrust{Complete: true, BodyHistoryKnown: &yes})
}

func TestAdjudicatedRouteIssuesAndClosesOnlyWithComposedChecks(t *testing.T) {
	f := newAdjudicatedRouteFixture(t, "", "")
	c := f.forge.client(t)
	before := getFile(t, f.readme)
	if _, _, _, err := closeVerifyFlipPlan(f.root, "vf/01", time.Now()); err == nil || !strings.Contains(err.Error(), "HELD") {
		t.Fatalf("ordinary close must still report the hold: %v", err)
	}

	prep, err := locateAcceptanceBrief(f.root, "vf/01")
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := prepareAcceptanceManifest(f.root, prep, acceptancePreparationInput{f.manifest.Runs, f.manifest.Dispositions})
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	digest, err := prepared.digest()
	want, _ := f.manifest.digest()
	if err != nil || digest != want {
		t.Fatalf("preparation changed the manifest: %q (%v)", digest, err)
	}
	issue, err := makeAdjudicatedVerifyIssue(f.root, "vf/01", rlIssue, f.ruling, c, rlLogins, false)
	if err != nil || issue.Marker != verifyMarker("vf/01") {
		t.Fatalf("issuer did not render the canonical card: %+v (%v)", issue, err)
	}
	_, plan, _, err := planAdjudicatedClose(f.root, "vf/01", f.manifest, rlIssue, f.ruling, time.Now(), c, rlLogins, false)
	if err != nil || !strings.Contains(string(plan), "| done |") || getFile(t, f.readme) != before {
		t.Fatalf("close did not plan a write-free done transition: %v", err)
	}
	cfg := writeAdjudicatedRouteCardFixture(t, adjudicatedRouteCard(issue.Body))
	if err := closeAdjudicatedCard(f.root, "vf/01", rlRepo, 60, c, cfg); err != nil || !strings.Contains(getFile(t, f.readme), "| done |") {
		t.Fatalf("canonical close did not apply: %v", err)
	}
}

func adjudicatedRouteCard(body string) creditIssue {
	return creditIssue{State: "closed", Labels: []string{"verify-gate"}, Body: body,
		Author:   &creditAccount{Login: "github-actions[bot]", ID: 41898282, Type: "Bot"},
		ClosedBy: &creditAccount{Login: rlHuman, ID: 1234, Type: "User"}}
}

func writeAdjudicatedRouteCardFixture(t *testing.T, card creditIssue) scanConfig {
	t.Helper()
	t.Setenv("CREDIT_FAKE_CI", "1")
	writeRouteIssueFixture(t, "issue", 60, card)
	yes := true
	writeRouteIssueFixture(t, "trust", 60, creditTrust{Complete: true, BodyHistoryKnown: &yes})
	return scanConfig{Bless: scanIdentity{Login: rlHuman, ID: 1234}, HumanLogins: rlLogins}
}

func TestAdjudicatedRouteRefusesBeforeIssuingOrWriting(t *testing.T) {
	for _, tc := range []struct {
		name, later, result, errText string
		change                       func(*testing.T, adjudicatedRouteFixture)
	}{
		{name: "worker preparation", errText: "pinned verifier App",
			change: func(t *testing.T, f adjudicatedRouteFixture) {
				f.writePreparation(t, "assay-worker-app[bot]", 300000006)
			}},
		{name: "generic human approval", errText: "exact acceptance adjudication",
			change: func(_ *testing.T, f adjudicatedRouteFixture) {
				f.forge.commentBody = rlCommentJSONWith(rlHuman, "User", rlRepo, rlIssue, "Approve acceptance-adjudication", rlPosted, rlPosted)
			}},
		{name: "missing label history", errText: "could-not-check",
			change: func(t *testing.T, _ adjudicatedRouteFixture) { t.Setenv("CREDIT_FAKE_FAIL", "label-events") }},
		{name: "recorded FAIL remains a separate guard", later: "\n<!-- VERIFY: FAIL -->\n", errText: "records a VERIFY: FAIL"},
		{name: "original row held", result: "HELD exit=0", errText: "original row 1"},
		{name: "original row failed", result: "fail exit=1", errText: "original row 1"},
		{name: "original row missing", result: "omit row", errText: "original row 1"},
		{name: "original row could not check", result: "could-not-check exit=0", errText: "original row 1"},
		{name: "source changed", errText: "differs from the committed blob",
			change: func(t *testing.T, f adjudicatedRouteFixture) {
				putFile(t, f.readme, getFile(t, f.readme)+"\nchanged README\n")
			}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newAdjudicatedRouteFixture(t, tc.later, tc.result)
			if tc.change != nil {
				tc.change(t, f)
			}
			c := f.forge.client(t)
			before := getFile(t, f.readme)
			issue, err := makeAdjudicatedVerifyIssue(f.root, "vf/01", rlIssue, f.ruling, c, rlLogins, false)
			if err == nil || !strings.Contains(err.Error(), tc.errText) || issue.Body != "" {
				t.Fatalf("wanted no issue and refusal containing %q, got body=%q err=%v", tc.errText, issue.Body, err)
			}
			_, plan, _, err := planAdjudicatedClose(f.root, "vf/01", f.manifest, rlIssue, f.ruling, time.Now(), c, rlLogins, false)
			if err == nil || !strings.Contains(err.Error(), tc.errText) || len(plan) != 0 || getFile(t, f.readme) != before {
				t.Fatalf("wanted no plan or write and refusal containing %q, got %v", tc.errText, err)
			}
			digest, err := f.manifest.digest()
			if err != nil {
				t.Fatal(err)
			}
			ref := adjudicatedCardRef{Preparation: rlIssue, Ruling: f.ruling, Manifest: digest}
			routeMarker, err := ref.marker()
			if err != nil {
				t.Fatal(err)
			}
			cfg := writeAdjudicatedRouteCardFixture(t, adjudicatedRouteCard(verifyMarker("vf/01")+"\n"+routeMarker))
			err = closeAdjudicatedCard(f.root, "vf/01", rlRepo, 60, c, cfg)
			if err == nil || !strings.Contains(err.Error(), tc.errText) || getFile(t, f.readme) != before {
				t.Fatalf("wanted the live close shape to refuse without a write: %v", err)
			}
		})
	}
}

func TestAdjudicatedCloseRejectsBadCardAfterValidPreparation(t *testing.T) {
	for _, tc := range []struct {
		name, errText   string
		changeCard      func(*creditIssue)
		changeMarker    func(*adjudicatedCardRef)
		history         *creditTrust
		changesOnReread bool
	}{
		{name: "bot closer", errText: "final close is not from the pinned human",
			changeCard: func(card *creditIssue) { card.ClosedBy.Type = "Bot" }},
		{name: "other mapped closer", errText: "final close is not from the pinned human",
			changeCard: func(card *creditIssue) { card.ClosedBy.Login = rlOther }},
		{name: "wrong issuer", errText: "expected a closed card issued by GitHub Actions",
			changeCard: func(card *creditIssue) { card.Author.ID++ }},
		{name: "missing verify gate label", errText: "missing verify-gate label",
			changeCard: func(card *creditIssue) { card.Labels = nil }},
		{name: "wrong brief marker", errText: "wrong or ambiguous brief marker",
			changeCard: func(card *creditIssue) {
				card.Body = strings.Replace(card.Body, verifyMarker("vf/01"), verifyMarker("vf/02"), 1)
			}},
		{name: "edited card", errText: "body was edited",
			history: &creditTrust{Complete: true, BodyHistoryKnown: new(true), BodyEditedAt: rlPosted}},
		{name: "partial history", errText: "body history is incomplete",
			history: &creditTrust{Complete: false, BodyHistoryKnown: new(true)}},
		{name: "wrong digest", errText: "preparation does not match the card's manifest digest",
			changeMarker: func(ref *adjudicatedCardRef) { ref.Manifest = strings.Repeat("0", 64) }},
		{name: "foreign ruling", errText: "adjudication is outside the card's repository",
			changeMarker: func(ref *adjudicatedCardRef) { ref.Ruling = rlURL("other-org/other-repo", rlIssue, rlComment) }},
		{name: "changed second read", errText: "issue changed during close revalidation", changesOnReread: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newAdjudicatedRouteFixture(t, "", "")
			c := f.forge.client(t)
			before := getFile(t, f.readme)
			issue, err := makeAdjudicatedVerifyIssue(f.root, "vf/01", rlIssue, f.ruling, c, rlLogins, false)
			if err != nil {
				t.Fatalf("test requires a valid preparation and ruling: %v", err)
			}
			card := adjudicatedRouteCard(issue.Body)
			if tc.changeMarker != nil {
				ref, present, err := parseAdjudicatedCardRef(card.Body)
				if !present || err != nil {
					t.Fatalf("issuer did not render a canonical marker: %v", err)
				}
				oldMarker, err := ref.marker()
				if err != nil {
					t.Fatal(err)
				}
				tc.changeMarker(&ref)
				marker, err := ref.marker()
				if err != nil {
					t.Fatal(err)
				}
				card.Body = strings.Replace(card.Body, oldMarker, marker, 1)
			}
			if tc.changeCard != nil {
				tc.changeCard(&card)
			}
			if tc.changesOnReread {
				changed := card
				changed.State = "open"
				writeRouteIssueFixture(t, "issue", 60, changed)
				path := routeIssueFixturePath("issue", 60)
				if err := os.Rename(path, path+".next"); err != nil {
					t.Fatal(err)
				}
			}
			cfg := writeAdjudicatedRouteCardFixture(t, card)
			if tc.history != nil {
				writeRouteIssueFixture(t, "trust", 60, *tc.history)
			}
			err = closeAdjudicatedCard(f.root, "vf/01", rlRepo, 60, c, cfg)
			if err == nil || !strings.Contains(err.Error(), tc.errText) {
				t.Errorf("wanted close refusal containing %q, got %v", tc.errText, err)
			}
			if got := getFile(t, f.readme); got != before {
				t.Error("card refusal changed the README")
			}
		})
	}
}

func TestOrdinaryScanStillHoldsLandedVerificationHandoff03(t *testing.T) {
	paths, err := filepath.Glob("../docs/streams/verification-handoff/brief-03-*.md")
	if err != nil || len(paths) != 1 {
		t.Fatalf("expected the landed /03 brief: %v (%v)", paths, err)
	}
	_, evidence := briefSectionsOf(getFile(t, paths[0]))
	signals := scanHeldRefusals(evidence, false)
	if len(signals) == 0 || closeVerifyHeldRefusal("verification-handoff/03", "implemented", evidence) == nil {
		t.Fatal("the unchanged /03 Evidence must still trigger the ordinary full scan")
	}
}

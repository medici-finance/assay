package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// regression: F-card-purpose
func TestFlowCard(t *testing.T) {
	t.Run("emitter contract", func(t *testing.T) {
		root, streams := loadDCStreams(t)
		decisions := decisionIssues(root, streams, nil)
		raw, err := json.Marshal(decisions[0])
		if err != nil {
			t.Fatal(err)
		}
		var payload map[string]any
		if err := json.Unmarshal(raw, &payload); err != nil {
			t.Fatal(err)
		}
		card, ok := payload["card"].(map[string]any)
		if !ok {
			t.Fatal("decision emitter lacks typed card identity and purpose")
		}
		if card["purpose"] != "decision" {
			t.Fatalf("decision purpose = %v", card)
		}
		if strings.Contains(decisions[0].Body, "<!-- verify-gate:") {
			t.Fatal("decision card acquired acceptance marker")
		}
		root, streams = loadVGStreams(t)
		gates := verifyIssues(root, streams, nil)
		raw, err = json.Marshal(gates[0])
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(raw, &payload); err != nil {
			t.Fatal(err)
		}
		card, ok = payload["card"].(map[string]any)
		if !ok || card["purpose"] != "verification-acceptance" {
			t.Fatalf("verification purpose = %v", card)
		}
	})
	t.Run("identity and revisions", flowCardIdentity)
	t.Run("shared marker fixtures", flowCardMarkers)
	t.Run("independent authority", flowCardAuthority)
	t.Run("legacy readability", flowCardLegacy)
	t.Run("verify rationale budget", flowCardBudget)
}

func flowCardIdentity(t *testing.T) {
	root, _ := vfFixture(t, vfDefaults())
	path := filepath.Join(root, "docs/streams/vf/brief-01-flip.md")
	bf, ok, err := parseBriefFile(path)
	if err != nil || !ok {
		t.Fatalf("parse: %v", err)
	}
	bf.DecisionIssue = 41
	card := makeGateCard(root, bf, cardAcceptance)
	if card.SourceRevision != gitCurrentSHA(root) || card.VerifyRevision != vfSHA || !strings.HasSuffix(card.DecisionURL, "/41") {
		t.Fatalf("missing revision/decision links: %+v", card)
	}
	// Legacy and v2 spellings converge only after resolving the declaring repo.
	writeVGRegistry(t, root)
	for _, tc := range []struct {
		id, local string
		valid     bool
	}{
		{"vf/01", "vf/01", true}, {"example:app:vf:01", "vf/01", true},
		{"app:vf/01", "vf/01", true}, {"example:app:vf/01", "vf/01", true},
		{"example:other:vf:01", "", false}, {"ghost:vf/01", "", false},
		{"foreign:app:vf:01", "", false}, {"vf/01 --> injected", "", false},
		{"example:app:vf:01a", "vf/01a", true},
	} {
		t.Run(tc.id, func(t *testing.T) {
			subject := *bf
			subject.Brief = tc.id
			if strings.HasSuffix(tc.local, "01a") {
				subject.Path = filepath.Join(root, "docs/streams/vf/brief-01a-flip.md")
			}
			canonical, local, err := cardBriefIdentity(root, &subject)
			if (err == nil) != tc.valid {
				t.Fatalf("identity %q: %v", tc.id, err)
			}
			if tc.valid && (local != tc.local || canonical != "example:app:vf:"+strings.TrimPrefix(local, "vf/")) {
				t.Fatalf("identity = %s %s", canonical, local)
			}
		})
	}
	t.Run("unsupported source profile", func(t *testing.T) {
		t.Setenv("GIT_CONFIG_COUNT", "0")
		if got := makeGateCard(root, bf, cardAcceptance); got.SourceRevision != "" || got.Unavailable == "" {
			t.Fatalf("unsupported source profile must remain unknown: %+v", got)
		}
	})
	// A source link must describe the emitted bytes, never a dirty tree as HEAD.
	raw, _ := os.ReadFile(path)
	if err := os.WriteFile(path, append(raw, []byte("\nChanged input.\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := makeGateCard(root, bf, cardAcceptance); got.SourceRevision != "" || got.Unavailable == "" {
		t.Fatalf("dirty source certified: %+v", got)
	}
	// The same existing run selector must refuse a second row at another revision.
	bf.Evidence = strings.Replace(vfDefaults().evidence, vfRow2, strings.Replace(vfRow2, vfSHA, "0123456789ab", 1), 1)
	if got := makeGateCard(root, bf, cardAcceptance); got.VerifyRevision != "" {
		t.Fatalf("mixed revisions linked as one run: %+v", got)
	}
}

func flowCardMarkers(t *testing.T) {
	base := gateCard{Schema: "gate-card/v1", Purpose: cardDecision, Brief: dhBoardID, Local: dhBoardID, Repository: rlRepo, Version: 1}
	for _, tc := range []struct {
		name  string
		edit  func(*gateCard)
		body  func(string) string
		valid bool
	}{
		{name: "legacy marker", body: func(string) string { return dhMarker }, valid: true},
		{name: "typed decision", valid: true},
		{name: "typed v2", edit: func(c *gateCard) { c.Brief = "house:ex:sdlc:18" }, valid: true},
		{name: "purpose mismatch", edit: func(c *gateCard) { c.Purpose = cardAcceptance }},
		{name: "unknown purpose", edit: func(c *gateCard) { c.Purpose = "combined" }},
		{name: "foreign repository", edit: func(c *gateCard) { c.Repository = "other/repo" }},
		{name: "foreign alias", edit: func(c *gateCard) { c.Brief = "house:other:sdlc:18" }},
		{name: "mixed subject", edit: func(c *gateCard) { c.Local = "sdlc/19" }},
		{name: "no revision", edit: func(c *gateCard) { c.Version = 0 }},
		// Card text in code or a quote is not a card: it neither narrows nor
		// grants, so the legacy marker decides — a foreign purpose there is inert.
		{name: "fenced", edit: func(c *gateCard) { c.Purpose = cardAcceptance }, body: func(s string) string {
			return "```text\n" + strings.TrimSuffix(s, dhMarker) + "\n```\n" + dhMarker
		}, valid: true},
		{name: "fenced only", body: func(s string) string { return "```text\n" + strings.TrimSuffix(s, dhMarker) + "\n```" }},
		{name: "duplicate", body: func(s string) string { return s + s }},
		{name: "quoted", edit: func(c *gateCard) { c.Repository = "other/repo" }, body: func(s string) string {
			return strings.Replace(s, cardPrefix, "> "+cardPrefix, 1)
		}, valid: true},
		{name: "quoted only", body: func(s string) string {
			return strings.Replace(strings.TrimSuffix(s, dhMarker), cardPrefix, "> "+cardPrefix, 1)
		}},
		{name: "legacy prose mention", body: func(string) string { return dhMarker + "\nThe `assay-card:` line is informational only.\n" }, valid: true},
		{name: "legacy bare mention", body: func(string) string { return dhMarker + "\nSee assay-card: for details.\n" }, valid: true},
		{name: "card prose mention", body: func(s string) string { return s + "\nThe `assay-card:` line is informational only.\n" }, valid: true},
		{name: "crlf card", body: func(s string) string { return strings.ReplaceAll(s, "\n", "\r\n") }, valid: true},
		{name: "crlf legacy", body: func(string) string { return "Decision needed.\r\n" + dhMarker + "\r\n" }, valid: true},
		{name: "crlf mismatch", edit: func(c *gateCard) { c.Purpose = cardAcceptance }, body: func(s string) string { return strings.ReplaceAll(s, "\n", "\r\n") }},
		{name: "crlf duplicate", body: func(s string) string { return strings.ReplaceAll(s+s, "\n", "\r\n") }},
		{name: "trailing JSON", body: func(s string) string { return strings.Replace(s, "} -->", "} {} -->", 1) }},
		{name: "truncated", body: func(s string) string { return strings.Replace(s, "} -->", "}", 1) }},
		{name: "malicious field", body: func(s string) string { return strings.Replace(s, `"version":1`, `"version":1,"acceptance":true`, 1) }},
		{name: "combined marker", body: func(s string) string { return s + "\n<!-- verify-gate: sdlc/18 -->" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := base
			if tc.edit != nil {
				tc.edit(&c)
			}
			body := renderCardLinks(c) + dhMarker
			if tc.body != nil {
				body = tc.body(body)
			}
			if got := briefIssueRulesOn(body, rlRepo, []string{dhBoardID}, nil, []string{"ex"}); got != tc.valid {
				t.Fatalf("decision match=%v want %v", got, tc.valid)
			}
		})
	}
	// A link with no original decision-gate marker is never a ruling.
	if briefIssueRulesOn(renderCardLinks(base), rlRepo, []string{dhBoardID}, nil, nil) {
		t.Fatal("link created decision authority")
	}
	// Legacy decision-only text cannot become an acceptance marker or dedupe it.
	root, streams := loadVGStreams(t)
	path := filepath.Join(root, "closed-decisions.txt")
	if err := os.WriteFile(path, []byte("closed\n"+decisionMarker("vg/01")+"\n"+dhMarker), 0o644); err != nil {
		t.Fatal(err)
	}
	existing, err := loadExistingMarkers(path)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, card := range verifyIssues(root, streams, existing) {
		if card.Brief == "vg/01" {
			found = true
		}
	}
	if !found {
		t.Fatal("old closed decision suppressed final acceptance card")
	}
}

func flowCardAuthority(t *testing.T) {
	// The existing forge adapter checks the actual actor, even with valid links.
	for _, bot := range []bool{false, true} {
		t.Run(map[bool]string{false: "human", true: "bot"}[bot], func(t *testing.T) {
			forge := dhForge("Option 1 for " + dhBoardID + ".")
			c := gateCard{Schema: "gate-card/v1", Purpose: cardDecision, Brief: dhBoardID, Local: dhBoardID, Repository: rlRepo, Version: 1}
			forge.issueBody = rlIssueJSON(renderCardLinks(c) + dhMarker)
			if bot {
				forge.commentBody = rlCommentJSONWith("example-desk[bot]", "Bot", rlRepo, rlIssue, "Option 1 for "+dhBoardID+".", rlPosted, rlPosted)
			}
			dhSeams(t, forge)
			root := dhFixture(t, unruledBase())
			dhWrite(t, root, map[string]string{dhReadmePath: dhReadme(dhRow("18", "implemented")), dhBriefPath: dhFM("gate: human", dhRuledLines())})
			code, out := dhGate(t, root)
			if (code == 0) == bot {
				t.Fatalf("bot=%v exit=%d: %s", bot, code, out)
			}
		})
	}
	// The independent close prerequisite refuses an approved design without PASS.
	root, _ := loadDCStreams(t)
	if _, _, _, err := closeVerifyFlipPlan(root, "dc/01", time.Now()); err == nil {
		t.Fatal("design-only decision advanced to done")
	}
	// The original exact sign-off path remains usable; foreign aliases still fail.
	for _, ref := range []string{"vg/01", "example:app:vg:01", "other:vg/01", "ghost:vg/01"} {
		root, _ := loadVGStreams(t)
		writeVGRegistry(t, root)
		_, _, _, err := closeVerifyFlipPlan(root, ref, time.Now())
		good := ref == "vg/01" || ref == "example:app:vg:01"
		if (err == nil) != good {
			t.Fatalf("close %s: %v", ref, err)
		}
	}
	o := vfDefaults()
	o.evidence = strings.Replace(o.evidence, vfRow2, strings.Replace(vfRow2, vfSHA, "0123456789ab", 1), 1)
	vfRefuses(t, o, "sha mismatch")
}

// The --corroborate record path is the second caller of the decision-issue
// binding; legacy readability must hold there end to end, not only in the
// shared classifier.
func flowCardLegacy(t *testing.T) {
	card := gateCard{Schema: "gate-card/v1", Purpose: cardDecision, Brief: rrBriefID, Local: rrBriefID, Repository: rlRepo, Version: 1}
	for name, tc := range map[string]struct {
		body string
		want int
	}{
		"prose mention": {"A human decision is needed.\n\n" + rlMarker + "\n" + rrBriefMarker + "\nThe `assay-card:` line is informational only.\n", 0},
		"crlf card":     {strings.ReplaceAll(renderCardLinks(card)+rlMarker+"\n"+rrBriefMarker+"\n", "\n", "\r\n"), 0},
		"crlf foreign":  {strings.ReplaceAll(strings.Replace(renderCardLinks(card), rlRepo, "other/repo", 1)+rlMarker+"\n"+rrBriefMarker+"\n", "\n", "\r\n"), 1},
	} {
		t.Run(name, func(t *testing.T) {
			f := rrBoundForge()
			f.issueBody = rlIssueJSON(tc.body)
			root := rrSetup(t, f)
			if got := runCorroborateRecord(root, rlFile, rrBriefRel); got != tc.want {
				t.Fatalf("corroborate exit=%d want %d", got, tc.want)
			}
		})
	}
}

// Authored rationale shares the cuttable budget; only the generated header
// and the close prompts are kept whole.
func flowCardBudget(t *testing.T) {
	root, _ := loadVGStreams(t)
	bf, ok, err := parseBriefFile(filepath.Join(root, "docs/streams/vg/brief-01-human-verified.md"))
	if err != nil || !ok {
		t.Fatalf("parse: %v", err)
	}
	bf.GateWhy = strings.Repeat("reason ", 10000)
	bf.Why = strings.Repeat("value ", 2000)
	body := renderVerifyBody(root, bf)
	if n := vgcUnits(body); n >= vgcForgeLimit {
		t.Fatalf("body is %d characters, not under the forge's %d limit", n, vgcForgeLimit)
	}
	for _, keep := range []string{"<!-- verify-gate: ", cardPrefix, "### Before you close", "- [ ] I accept this brief as done"} {
		if !strings.Contains(body, keep) {
			t.Errorf("body lost %q", keep)
		}
	}
}

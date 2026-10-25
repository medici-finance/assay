package web

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/deskd/config/schema"
	"github.com/medici-finance/assay/deskd/internal/adapter"
	"github.com/medici-finance/assay/deskd/internal/domain"
	"github.com/medici-finance/assay/deskd/internal/identity"
	"github.com/medici-finance/assay/deskd/internal/index"
)

// renderPaneWithViewer serves one pane route against a seeded DB + optional
// config AND a wired ViewerSource, so the pane's per-repo surface authorizer is
// exercised (the security path), not just the anonymous-unfiltered shape.
func renderPaneWithViewer(t *testing.T, db *index.DB, cfg *schema.File, path string, v ViewerSource) (int, string) {
	t.Helper()
	srv, err := New(Deps{DB: db, Config: cfg, Now: fixedNow, Viewer: v})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	return rr.Code, rr.Body.String()
}

// TestEpicHidesBlockedOnRowsWithoutCapability (Security-Review 5096440185
// sibling-route fix): the /epic pane's "blocked on" list is built from the same
// unfiltered DB.Inbox("", true) source the inbox feed reads, and emits each
// Waiting escalation's Title + item deep-link + platform deep-link — the SAME
// withheld private-repo metadata. An authenticated viewer with NO read
// capability on the row's private repo must NOT receive it. Pre-fix
// (blockedOnRows over the unfiltered rows) this leaked; post-fix the rows are
// gated per-repo through the surface authorizer, so the private row is dropped.
func TestEpicHidesBlockedOnRowsWithoutCapability(t *testing.T) {
	db := oneWaitingRole(t, "assay", "review") // Waiting escalation E1, issue #1, on example-org/console (private)
	markPoll(t, db, 30*time.Second)

	// Authenticated as bob, who is Anonymous (no read) on the row's repo.
	deny := denyRepoViewer{
		info:      identity.ViewerInfo{Authenticated: true, Login: "bob"},
		denyOwner: "example-org",
		denyName:  "console",
	}
	code, body := renderPaneWithViewer(t, db, nil, "/epic/console-service", deny)
	if code != http.StatusOK {
		t.Fatalf("epic pane status = %d, want 200 (a filtered pane, not a block)", code)
	}
	if strings.Contains(body, "a real open escalation") {
		t.Fatal("LEAK: an authenticated viewer with no read on the row's private repo received the escalation Title on /epic")
	}
	if strings.Contains(body, "/item/E1") {
		t.Fatal("LEAK: the private escalation's item deep-link (/item/E1) was served on /epic to an unauthorized viewer")
	}
	if strings.Contains(body, "/issues/1") {
		t.Fatal("LEAK: the private escalation's platform deep-link (issues/1) was served on /epic to an unauthorized viewer")
	}

	// Control: a viewer authorized on that repo still sees the blocked-on row.
	ok := denyRepoViewer{
		info:      identity.ViewerInfo{Authenticated: true, Login: "operator"},
		denyOwner: "nobody",
		denyName:  "nowhere",
	}
	_, okBody := renderPaneWithViewer(t, db, nil, "/epic/console-service", ok)
	if !strings.Contains(okBody, "a real open escalation") || !strings.Contains(okBody, "/item/E1") {
		t.Fatal("an authorized viewer must still see the blocked-on row — the filter over-blocked")
	}
}

// TestPortfolioHidesEscalationFactsWithoutCapability (Security-Review 5096440185
// audit route): the /portfolio running-automations strip derives, per (cell,
// role), whether a specific repo's automation has an open Waiting escalation and
// that escalation's age — private-repo escalation facts. An authenticated viewer
// with no read on the repo must see the configured automation as SILENT with no
// oldest-escalation age (the config STRUCTURE still renders — it is operator
// config, not private-repo metadata), never its private waiting-existence/age.
func TestPortfolioHidesEscalationFactsWithoutCapability(t *testing.T) {
	db := oneWaitingRole(t, "assay", "review") // Waiting escalation on example-org/console (private)
	markPoll(t, db, 30*time.Second)
	cfg := oneCellConfig("assay", schema.RoleReview)

	deny := denyRepoViewer{
		info:      identity.ViewerInfo{Authenticated: true, Login: "bob"},
		denyOwner: "example-org",
		denyName:  "console",
	}
	code, body := renderPaneWithViewer(t, db, cfg, "/portfolio", deny)
	if code != http.StatusOK {
		t.Fatalf("portfolio pane status = %d, want 200", code)
	}
	// The configured automation still appears (config structure is not private).
	if !strings.Contains(body, `data-automation="assay/review"`) {
		t.Fatal("the configured automation row must still render (operator config, not private-repo metadata)")
	}
	// But its escalation-derived facts must be withheld: no waiting state, no
	// oldest-escalation age link into the private cell's inbox.
	if strings.Contains(body, `data-state="waiting"`) {
		t.Fatal("LEAK: an uncapable viewer saw a private repo's Waiting escalation existence on the portfolio strip")
	}
	if strings.Contains(body, `href="/inbox?cell=assay"`) {
		t.Fatal("LEAK: an uncapable viewer received the private escalation's oldest-age link on the portfolio strip")
	}

	// Control: a viewer authorized on that repo sees the waiting state + age link.
	ok := denyRepoViewer{
		info:      identity.ViewerInfo{Authenticated: true, Login: "operator"},
		denyOwner: "nobody",
		denyName:  "nowhere",
	}
	_, okBody := renderPaneWithViewer(t, db, cfg, "/portfolio", ok)
	if !strings.Contains(okBody, `data-state="waiting"`) || !strings.Contains(okBody, `href="/inbox?cell=assay"`) {
		t.Fatal("an authorized viewer must still see the automation's waiting state and oldest-age link — the filter over-blocked")
	}
}

// fixedNow pins the clock so age/staleness renders are deterministic.
func fixedNow() time.Time { return time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC) }

// renderPane serves one pane route against a seeded DB + optional config.
func renderPane(t *testing.T, db *index.DB, cfg *schema.File, path string) (int, string) {
	t.Helper()
	srv, err := New(Deps{DB: db, Config: cfg, Now: fixedNow})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	return rr.Code, rr.Body.String()
}

// oneWaitingRole seeds a single open escalation attributed to a given cell/role,
// so its derived attention state is `waiting` (open escalation ⇒ waiting, §1.2).
func oneWaitingRole(t *testing.T, cell, role string) *index.DB {
	t.Helper()
	escs := []domain.Escalation{{
		ID: "E1", Cell: domain.CellName(cell),
		Automation: domain.AutomationRef{Cell: domain.CellName(cell), Role: domain.Role(role)},
		Kind:       domain.KindNeedsDecision, Provenance: domain.AgentRaised,
		Urgency: domain.UrgencyRoutine, Title: "a real open escalation",
		Situation: "holds the epic", FiledAt: fixedNow().Add(-2 * time.Hour),
	}}
	raw := map[domain.EscalationID]adapter.RawIssue{"E1": {Number: 1, URL: issueURL(1)}}
	return seed(t, escs, raw)
}

// markPoll ages the index by writing the last-poll timestamp. `ago` past 2× the
// poll interval renders the strip stale (§10).
func markPoll(t *testing.T, db *index.DB, ago time.Duration) {
	t.Helper()
	if err := db.MarkPoll(fixedNow().Add(-ago-time.Second), fixedNow().Add(-ago), "ok"); err != nil {
		t.Fatalf("MarkPoll: %v", err)
	}
}

func oneCellConfig(cell string, roles ...schema.LoopRole) *schema.File {
	c := schema.Cell{Name: cell}
	for _, r := range roles {
		c.Loops = append(c.Loops, schema.Loop{Role: r, Schedule: "*/5 * * * *",
			Models: []string{"opus-4.8"}, App: "assay-" + string(r) + "-app"})
	}
	return &schema.File{Cells: []schema.Cell{c}}
}

// twoCellConfig returns two configured cells, each with the same role set —
// so a strip-per-cell test isolates the PANE's cell-keying, not a difference
// in role shape between the two cells.
func twoCellConfig(cellA, cellB string, roles ...schema.LoopRole) *schema.File {
	mk := func(cell string) schema.Cell {
		c := schema.Cell{Name: cell}
		for _, r := range roles {
			c.Loops = append(c.Loops, schema.Loop{Role: r, Schedule: "*/5 * * * *",
				Models: []string{"opus-4.8"}, App: "assay-" + string(r) + "-app"})
		}
		return c
	}
	return &schema.File{Cells: []schema.Cell{mk(cellA), mk(cellB)}}
}

// TestVerify10_PortfolioStripDistinguishesTwoCells is Verify row 10 (re-pointed,
// the service contract): with two cells configured, the /portfolio pane renders
// one automations table per cell (data-cell="<name>"), and an escalation
// attributed to ONE cell's automation is reflected ONLY on that cell's row —
// the other cell's same-role row stays silent with no oldest-escalation link.
// Every prior fixture in this file seeds a single cell, so nothing before this
// test exercises the cell-keying itself (a copy-paste bug that duplicated one
// cell's strip onto a second cell's name would pass every existing assertion).
func TestVerify10_PortfolioStripDistinguishesTwoCells(t *testing.T) {
	db := oneWaitingRole(t, "alpha", "review") // open escalation on cell "alpha" only
	markPoll(t, db, 30*time.Second)
	cfg := twoCellConfig("alpha", "beta", schema.RoleReview)

	_, body := renderPane(t, db, cfg, "/portfolio")

	tables := regexp.MustCompile(`data-cell="([a-z]+)"`).FindAllStringSubmatch(body, -1)
	if len(tables) != 2 {
		t.Fatalf("expected 2 data-cell tables (one per configured cell), got %d:\n%s", len(tables), firstLines(body, 40))
	}
	names := map[string]bool{}
	for _, m := range tables {
		names[m[1]] = true
	}
	if !names["alpha"] || !names["beta"] {
		t.Fatalf("expected data-cell tables for both %q and %q, got %v", "alpha", "beta", names)
	}

	// alpha's row: waiting, with its own oldest-escalation link into its cell.
	alphaSection := sectionForCell(t, body, "alpha")
	if !strings.Contains(alphaSection, `data-state="waiting"`) {
		t.Fatalf("cell alpha's row did not render waiting:\n%s", alphaSection)
	}
	if !strings.Contains(alphaSection, `href="/inbox?cell=alpha"`) {
		t.Fatalf("cell alpha's row did not link its oldest escalation into its own cell's inbox:\n%s", alphaSection)
	}

	// beta's row: same role, no escalation of its own — silent, and critically
	// NOT carrying alpha's waiting state or alpha's inbox link (the leak this
	// row is here to catch: one cell's escalation bleeding onto another cell's
	// strip because both rows share a (role) key without the cell qualifier).
	betaSection := sectionForCell(t, body, "beta")
	if strings.Contains(betaSection, `data-state="waiting"`) {
		t.Fatalf("LEAK: cell beta's row rendered waiting — alpha's escalation bled across cells:\n%s", betaSection)
	}
	if strings.Contains(betaSection, `href="/inbox?cell=alpha"`) {
		t.Fatalf("LEAK: cell beta's table carries alpha's inbox link:\n%s", betaSection)
	}
	if !strings.Contains(betaSection, `class="automation-row silent"`) {
		t.Fatalf("cell beta's role-only row did not render silent:\n%s", betaSection)
	}
}

// sectionForCell returns the HTML slice for one cell's <table data-cell="...">
// element, up to (but not including) the next data-cell table or end of body —
// so assertions against one cell's section cannot accidentally match content
// that belongs to a sibling cell's table.
func sectionForCell(t *testing.T, body, cell string) string {
	t.Helper()
	start := strings.Index(body, `data-cell="`+cell+`"`)
	if start == -1 {
		t.Fatalf("no data-cell=%q table found in body", cell)
	}
	rest := body[start:]
	if next := strings.Index(rest[1:], `data-cell="`); next != -1 {
		return rest[:next+1]
	}
	return rest
}

// TestVerify1_EveryPaneHasAmbientStripWithScope is Verify row 1: every pane
// route's DOM carries the ambient strip element with a scope attribute.
func TestVerify1_EveryPaneHasAmbientStripWithScope(t *testing.T) {
	db := oneWaitingRole(t, "assay", "review")
	markPoll(t, db, 30*time.Second)
	cfg := oneCellConfig("assay", schema.RoleReview, schema.RoleVerify)

	cases := []struct{ path, scope string }{
		{"/roadmap", "program"},
		{"/epic/console-service", "epic"},
		{"/measures", "portfolio"},
		{"/portfolio", "portfolio"},
		{"/teams", "team"},
	}
	strip := regexp.MustCompile(`class="ambient-strip"[^>]*data-scope="([a-z]+)"`)
	for _, c := range cases {
		code, body := renderPane(t, db, cfg, c.path)
		if code != 200 {
			t.Fatalf("%s: status %d", c.path, code)
		}
		m := strip.FindStringSubmatch(body)
		if m == nil {
			t.Fatalf("%s: no ambient-strip element with a data-scope attribute", c.path)
		}
		if m[1] != c.scope {
			t.Fatalf("%s: ambient strip scope = %q, want %q", c.path, m[1], c.scope)
		}
	}
}

// TestVerify3_MeasuresTilesCarryQueryAndHardness is Verify row 3: every stat
// tile in Measures — including the two new escalation tiles — carries data-query
// and data-hardness, and the acceptance-rate tile is tagged ◐.
func TestVerify3_MeasuresTilesCarryQueryAndHardness(t *testing.T) {
	db := oneWaitingRole(t, "assay", "review")
	markPoll(t, db, 30*time.Second)
	_, body := renderPane(t, db, nil, "/measures")

	// Match only the OUTER tile span (class begins "tile" then a space or the
	// closing quote — never "tile-hardness"/"tile-label"/etc).
	tile := regexp.MustCompile(`<span class="tile[ "][^>]*>`)
	tiles := tile.FindAllString(body, -1)
	if len(tiles) < 15 {
		t.Fatalf("expected the full Measures tile catalogue, got %d tiles", len(tiles))
	}
	for i, tag := range tiles {
		if !strings.Contains(tag, "data-query=") {
			t.Fatalf("tile %d carries no data-query: %s", i, tag)
		}
		if !strings.Contains(tag, "data-hardness=") {
			t.Fatalf("tile %d carries no data-hardness: %s", i, tag)
		}
	}
	// the acceptance-rate tile is ◐ (content-derived, tuning-only)
	accept := regexp.MustCompile(`data-key="acceptance_rate"[^>]*data-hardness="([^"]+)"`)
	am := accept.FindStringSubmatch(body)
	if am == nil {
		t.Fatal("no acceptance_rate tile found")
	}
	if am[1] != "◐" {
		t.Fatalf("acceptance_rate hardness = %q, want ◐", am[1])
	}
	if !strings.Contains(body, `data-key="time_to_decision_p50"`) ||
		!strings.Contains(body, `data-key="dwell_after_look"`) {
		t.Fatal("the new decision-first tiles (time-to-decision, dwell-after-look) are missing")
	}
}

// TestVerify6_WaitingRendersHealthyNotFault is Verify row 6: a DOM element with
// data-state="waiting" uses the healthy palette token, never the fault token.
func TestVerify6_WaitingRendersHealthyNotFault(t *testing.T) {
	db := oneWaitingRole(t, "assay", "review")
	markPoll(t, db, 30*time.Second)
	cfg := oneCellConfig("assay", schema.RoleReview, schema.RoleVerify)
	_, body := renderPane(t, db, cfg, "/portfolio")

	waiting := regexp.MustCompile(`data-state="waiting"[^>]*data-palette="([a-z]+)"`)
	m := waiting.FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("no data-state=\"waiting\" element found:\n%s", firstLines(body, 60))
	}
	if m[1] != "healthy" {
		t.Fatalf("waiting row palette = %q, want healthy (§3.5: waiting is calm, never the fault token)", m[1])
	}
	if m[1] == "fault" {
		t.Fatal("waiting row rendered the fault token — §3.5 violation")
	}
}

// TestVerify7_PortfolioStripDerivedRowsNoContextBurn is Verify row 7: the strip
// row count equals the running automations in the cell, every column is derived
// (no self-reported field), and there is no context-burn column.
func TestVerify7_PortfolioStripDerivedRowsNoContextBurn(t *testing.T) {
	db := oneWaitingRole(t, "assay", "review")
	markPoll(t, db, 30*time.Second)
	cfg := oneCellConfig("assay", schema.RoleReview, schema.RoleVerify, schema.RoleMetrics)
	_, body := renderPane(t, db, cfg, "/portfolio")

	rows := strings.Count(body, `class="automation-row`)
	if rows != 3 {
		t.Fatalf("automation-row count = %d, want 3 (one per configured loop)", rows)
	}
	// no context-burn COLUMN (§3.4/D2 rejected for v1): the strip's header cells
	// carry no "context" column, and no col-context cell class exists. (The D2
	// note in the pane prose deliberately NAMES the rejection — that is not a
	// column, so we check structure, not the word.)
	headers := regexp.MustCompile(`(?s)<thead>.*?</thead>`).FindString(body)
	if strings.Contains(strings.ToLower(headers), "context") {
		t.Fatalf("a context-burn column header is present — D2 rejected it for v1:\n%s", headers)
	}
	if strings.Contains(body, "col-context") {
		t.Fatal("a context-burn column cell is present — D2 rejected it for v1")
	}
	if n := strings.Count(headers, "<th>"); n != 7 {
		t.Fatalf("running-automations strip has %d columns, want the 7 derived columns", n)
	}
	// derived-not-ingested columns render an explicit could-not-check, never a
	// blank or a fabricated value (no self-reported field).
	if !strings.Contains(body, "cnc") {
		t.Fatal("derived-not-ingested columns did not render a could-not-check marker")
	}
	// the two derivable columns are populated: the review automation (which has
	// the open escalation) links its oldest open escalation into the inbox.
	if !strings.Contains(body, `href="/inbox?cell=assay"`) {
		t.Fatal("the waiting automation did not link its oldest open escalation into the inbox")
	}
}

// TestVerify8_StaleIndexGreysChipSilentAmber is Verify row 8: rendered against a
// stale index (poller stopped past 2× the poll interval), the "as of" chip is
// present and greyed, and a silent row renders amber-as-a-question, never green.
func TestVerify8_StaleIndexGreysChipSilentAmber(t *testing.T) {
	cfg := oneCellConfig("assay", schema.RoleReview, schema.RoleVerify)

	// fresh: within the poll interval — the chip is NOT stale.
	fresh := oneWaitingRole(t, "assay", "other-loop") // attributed to a non-config role ⇒ all config rows silent
	markPoll(t, fresh, 30*time.Second)
	_, freshBody := renderPane(t, fresh, cfg, "/portfolio")
	if strings.Contains(freshBody, `class="ambient-strip" data-scope="portfolio" data-stale="true"`) {
		t.Fatal("a fresh index (30s) rendered the strip stale")
	}

	// stale: past 2× the poll interval — the chip greys and the strip is stale.
	stale := oneWaitingRole(t, "assay", "other-loop")
	markPoll(t, stale, 10*time.Minute)
	_, staleBody := renderPane(t, stale, cfg, "/portfolio")
	if !strings.Contains(staleBody, `data-stale="true"`) {
		t.Fatal("a stale index (10m > 2× the 60s poll) did not mark the strip stale")
	}
	// the "as of" chip is present and greyed
	if !regexp.MustCompile(`class="as-of stale"`).MatchString(staleBody) {
		t.Fatalf("the as-of chip is not present-and-greyed under staleness:\n%s", firstLines(staleBody, 30))
	}
	// a silent automation row renders amber (data-palette="amber"), never green
	if !regexp.MustCompile(`class="automation-row silent"[^>]*data-palette="amber"`).MatchString(staleBody) {
		t.Fatal("a silent automation row is not amber-as-a-question (§3.4, §10)")
	}
}

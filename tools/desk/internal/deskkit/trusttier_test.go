package deskkit

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---- pure-loader helpers (offline, no filesystem) --------------------------

// fakeLedger installs an in-memory LedgerLoader for the duration of the test.
func fakeLedger(t *testing.T, rows []LedgerRow) {
	t.Helper()
	restore := withLedgerLoader(func() ([]LedgerRow, error) { return rows, nil })
	t.Cleanup(restore)
}

// fakeLedgerAbsent installs a loader that reports "no ledger configured".
func fakeLedgerAbsent(t *testing.T) {
	t.Helper()
	restore := withLedgerLoader(func() ([]LedgerRow, error) { return nil, errLedgerNotConfigured })
	t.Cleanup(restore)
}

// fakeLedgerBroken installs a loader that reports an unrelated read/parse
// failure — never errLedgerNotConfigured — so ResolveTier must take the
// default-unreadable arm, not the default-absent one.
func fakeLedgerBroken(t *testing.T, why string) {
	t.Helper()
	restore := withLedgerLoader(func() ([]LedgerRow, error) { return nil, errors.New(why) })
	t.Cleanup(restore)
}

func TestTrustTierOrderingAndNames(t *testing.T) {
	if !(TierUnknown < TierBlessedOnce && TierBlessedOnce < TierContributor && TierContributor < TierMaintainer) {
		t.Fatalf("tier order is not unknown < blessed-once < contributor < maintainer")
	}
	for tier, want := range map[Tier]string{
		TierUnknown:     "unknown",
		TierBlessedOnce: "blessed-once",
		TierContributor: "contributor",
		TierMaintainer:  "maintainer",
	} {
		if got := tier.String(); got != want {
			t.Errorf("Tier(%d).String() = %q, want %q", tier, got, want)
		}
	}
	if !TierMaintainer.AtLeast(TierContributor) {
		t.Error("TierMaintainer.AtLeast(TierContributor) = false, want true")
	}
	if TierUnknown.AtLeast(TierBlessedOnce) {
		t.Error("TierUnknown.AtLeast(TierBlessedOnce) = true, want false")
	}
}

// TestTrustTierAbsentLedger: pre-mortem "the ledger is absent on a fresh machine
// and every external identity silently becomes a contributor". No roster, no
// ledger configured at all — the identity must resolve unknown with the
// LEGITIMATE-empty provenance, never an anomaly.
func TestTrustTierAbsentLedger(t *testing.T) {
	withNoRoster(t)
	fakeLedgerAbsent(t)

	tier, prov := ResolveTier("medici-finance/example", "outsider", 555)
	t.Logf("ResolveTier = tier=%s provenance=%s", tier, prov)

	if tier != TierUnknown {
		t.Errorf("tier = %s, want unknown", tier)
	}
	if prov != ProvenanceDefaultAbsent {
		t.Errorf("provenance = %s, want %s", prov, ProvenanceDefaultAbsent)
	}
}

// TestTrustTierUnreadableLedger: pre-mortem "a corrupt or half-written ledger is
// read as an empty one and the anomaly is invisible". A malformed ledger file
// must resolve unknown too, but through the DISTINCT default-unreadable
// provenance — never silently treated as an empty ledger, and never granting any
// tier above unknown.
//
// This test's own output must never contain the word "contributor" (Verify row
// 3): a malformed ledger silently resolving to the `contributor` tier is exactly
// the failure this test exists to catch, so nothing here — the fixture identity,
// the malformed content, or any message on the code path — may name that tier.
func TestTrustTierUnreadableLedger(t *testing.T) {
	withNoRoster(t)
	fakeLedgerBroken(t, "malformed ledger fixture: not valid JSON")

	tier, prov := ResolveTier("medici-finance/example", "outsider", 555)
	t.Logf("ResolveTier = tier=%s provenance=%s", tier, prov)

	if tier != TierUnknown {
		t.Errorf("tier = %s, want unknown", tier)
	}
	if prov != ProvenanceDefaultUnreadable {
		t.Errorf("provenance = %s, want %s", prov, ProvenanceDefaultUnreadable)
	}
}

// TestTrustTierUnreadableLedgerRealFile exercises loadLedgerFile
// itself (not the injected fake) against a real malformed file on disk, so the
// production loader's own malformed-line detection is proven, not just the
// resolution logic above it.
func TestTrustTierUnreadableLedgerRealFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ledger.jsonl")
	if err := os.WriteFile(path, []byte("not json at all\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	withRoster(t, map[string]string{
		EnvBlessLogin:        fmt.Sprintf("%s:%d", fixtureBlessLogin, fixtureBlessID),
		EnvTrustedLogins:     fmt.Sprintf("%s:%d", fixtureBlessLogin, fixtureBlessID),
		EnvContributorLedger: path,
	})

	_, err := loadLedgerFile()
	if err == nil {
		t.Fatal("loadLedgerFile did not error on a malformed line")
	}
	if errors.Is(err, errLedgerNotConfigured) {
		t.Fatalf("a malformed file must not read as errLedgerNotConfigured (that collapses a broken "+
			"ledger into an empty one): %v", err)
	}
	t.Logf("loadLedgerFile correctly refused: %v", err)
}

// TestTrustTierRosterWins: pre-mortem "a ledger row demotes a maintainer and the
// desk locks itself out". A ledger row naming a ROSTER identity at a LOWER tier
// must never lower it — roster membership resolves first and the ledger is never
// even consulted for that identity.
func TestTrustTierRosterWins(t *testing.T) {
	withRoster(t, map[string]string{
		EnvBlessLogin:    "ada:2001",
		EnvTrustedLogins: "ada:2001",
	})
	fakeLedger(t, []LedgerRow{
		{
			Repo: "medici-finance/example", Login: "ada", ID: 2001,
			Tier: "blessed-once", Date: "2026-09-01", Recorder: "kryton",
			Reason: "negative control — must never be consulted for a roster identity",
		},
	})

	tier, prov := ResolveTier("medici-finance/example", "ada", 2001)
	t.Logf("ResolveTier = tier=%s provenance=%s", tier, prov)

	if tier != TierMaintainer {
		t.Errorf("tier = %s, want maintainer — a ledger row must never lower a roster identity", tier)
	}
	if prov != ProvenanceRoster {
		t.Errorf("provenance = %s, want %s — the ledger must not even be the source here", prov, ProvenanceRoster)
	}
}

// TestTrustTierUnpinnedHuman: pre-mortem "a row naming a login with no pinned id
// grants a tier, re-opening the recycled-login hole the existing strict path
// closes". A ledger row with NO pinned numeric id (ID: 0) must grant nothing, on
// the same strict path trust.go's trustedContentAuthor enforces.
func TestTrustTierUnpinnedHuman(t *testing.T) {
	withRoster(t, map[string]string{
		EnvBlessLogin:    "ada:2001",
		EnvTrustedLogins: "ada:2001",
	})
	fakeLedger(t, []LedgerRow{
		{
			Repo: "medici-finance/example", Login: "newcomer", ID: 0,
			Tier: "contributor", Date: "2026-09-01", Recorder: "kryton",
			Reason: "negative control — an unpinned row must grant nothing",
		},
	})

	// The caller DOES supply a numeric id for the surface — there is simply
	// nothing in the row to compare it against.
	tier, prov := ResolveTier("medici-finance/example", "newcomer", 9009)
	t.Logf("ResolveTier = tier=%s provenance=%s", tier, prov)

	if tier != TierUnknown {
		t.Errorf("tier = %s, want unknown — an unpinned ledger row must grant nothing", tier)
	}
	if prov != ProvenanceDefaultAbsent {
		t.Errorf("provenance = %s, want %s", prov, ProvenanceDefaultAbsent)
	}

	// And the reverse shape: a PINNED row, but the caller's surface carries no id
	// (0) — there is still nothing to compare, so this must also grant nothing.
	fakeLedger(t, []LedgerRow{
		{
			Repo: "medici-finance/example", Login: "newcomer", ID: 9009,
			Tier: "contributor", Date: "2026-09-01", Recorder: "kryton",
			Reason: "negative control — a supplied id of 0 has nothing to compare against",
		},
	})
	tier2, prov2 := ResolveTier("medici-finance/example", "newcomer", 0)
	if tier2 != TierUnknown || prov2 != ProvenanceDefaultAbsent {
		t.Errorf("ResolveTier with a zero supplied id = (%s, %s), want (unknown, %s)",
			tier2, prov2, ProvenanceDefaultAbsent)
	}
}

// TestTrustTierLedgerRowGrants is the positive control the negative tests above
// need: a well-formed, pinned row for a NON-roster identity DOES grant its
// recorded tier, so the strict-path refusals above are proven against a check
// that can actually pass.
func TestTrustTierLedgerRowGrants(t *testing.T) {
	withRoster(t, map[string]string{
		EnvBlessLogin:    "ada:2001",
		EnvTrustedLogins: "ada:2001",
	})
	fakeLedger(t, []LedgerRow{
		{
			Repo: "medici-finance/example", Login: "regular", ID: 4004,
			Tier: "contributor", Date: "2026-09-01", Recorder: "kryton",
			Reason: "positive control",
		},
	})

	tier, prov := ResolveTier("medici-finance/example", "regular", 4004)
	if tier != TierContributor {
		t.Errorf("tier = %s, want contributor", tier)
	}
	if prov != ProvenanceLedgerRow {
		t.Errorf("provenance = %s, want %s", prov, ProvenanceLedgerRow)
	}

	// A different repository must not match — the ledger is per-repository.
	tier2, prov2 := ResolveTier("medici-finance/other", "regular", 4004)
	if tier2 != TierUnknown || prov2 != ProvenanceDefaultAbsent {
		t.Errorf("ResolveTier on an unlisted repo = (%s, %s), want (unknown, %s) — the ledger is "+
			"per-repository and must not leak a grant across repos", tier2, prov2, ProvenanceDefaultAbsent)
	}

	// A mismatched id on the SAME login must not match either (recycled-login).
	tier3, _ := ResolveTier("medici-finance/example", "regular", 9999)
	if tier3 != TierUnknown {
		t.Errorf("ResolveTier with a mismatched id = %s, want unknown (recycled-login defence)", tier3)
	}
}

// TestTrustTierCapabilityTableGrantsNoMergeAuthority pins the one invariant the
// brief states in prose: no tier's capability set may ever be extended with a
// merge-authority-shaped entry. There is no "merge" Capability constant; this
// guards a future edit from adding one.
func TestTrustTierCapabilityTableGrantsNoMergeAuthority(t *testing.T) {
	for tier, caps := range capabilityTable {
		for _, c := range caps {
			if strings.Contains(strings.ToLower(string(c)), "merge") {
				t.Errorf("tier %s carries capability %q — no tier may grant merge authority", tier, c)
			}
		}
	}
}

// ---- cross-tree coupling ----------------------------------------------------

// TestContributorLedgerRosterCoupling binds the new ASSAY_CONTRIBUTOR_LEDGER
// roster key across the two documented-duplicate readers (this package and
// statusgen), the way TestRosterKeySchemaCoupling binds every other key — plus a
// BEHAVIOURAL check that this reader actually resolves a configured path.
//
// The two modules deliberately share no code (separate Go modules), so the
// cross-module half is read as source text, the same technique
// TestScanIssuesTrustGateEnforced already uses to bind trust.go's twin.
func TestContributorLedgerRosterCoupling(t *testing.T) {
	// ---- (1) declared in the shared vector file, both directions ----
	vec := loadCouplingVectors(t)
	found := false
	for _, k := range vec.KnownRosterKeys {
		if k == EnvContributorLedger {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("%s does not declare %s in knownRosterKeys — the shared schema and this key have "+
			"drifted", couplingVectorPath, EnvContributorLedger)
	}

	mine := map[string]bool{}
	for _, k := range knownRosterKeys() {
		mine[k] = true
	}
	if !mine[EnvContributorLedger] {
		t.Errorf("knownRosterKeys() does not recognise %s — a roster carrying it would report this "+
			"tree's whole trust configuration unconfigured", EnvContributorLedger)
	}

	// ---- (2) the statusgen twin recognises the IDENTICAL key literal ----
	base := filepath.Join("..", "..", "..", "..", "statusgen")
	rc, err := os.ReadFile(filepath.Join(base, "rosterconfig.go"))
	if err != nil {
		t.Fatalf("cannot read %s/rosterconfig.go — if statusgen moved, re-point this coupling test, "+
			"do NOT delete it: %v", base, err)
	}
	src := string(rc)
	if !strings.Contains(src, `"`+EnvContributorLedger+`"`) {
		t.Errorf("statusgen/rosterconfig.go does not declare the literal %q — a roster.env carrying "+
			"%s would collapse statusgen's whole trust configuration on the unknown-ASSAY_-key refusal",
			EnvContributorLedger, EnvContributorLedger)
	}
	if !strings.Contains(src, "scanEnvContributorLedger") {
		t.Error("statusgen/rosterconfig.go has no scanEnvContributorLedger constant — the twin must " +
			"name the same key deskkit does")
	}
	if !strings.Contains(src, "scanKnownRosterKeys") {
		t.Fatal("statusgen/rosterconfig.go: scanKnownRosterKeys is gone — nothing declares the " +
			"recognised roster schema on that side")
	}
	// Windowed: the new constant must actually be LISTED inside scanKnownRosterKeys's
	// body, not merely declared and forgotten (the same class of gap #491 was).
	fnIdx := strings.Index(src, "func scanKnownRosterKeys()")
	if fnIdx < 0 {
		t.Fatal("statusgen/rosterconfig.go: scanKnownRosterKeys function body not found")
	}
	closeIdx := strings.Index(src[fnIdx:], "\n}")
	if closeIdx < 0 {
		t.Fatal("statusgen/rosterconfig.go: scanKnownRosterKeys function body has no closing brace " +
			"within scan range")
	}
	body := src[fnIdx : fnIdx+closeIdx]
	if !strings.Contains(body, "scanEnvContributorLedger") {
		t.Error("statusgen/rosterconfig.go: scanKnownRosterKeys() does not list scanEnvContributorLedger " +
			"— declaring the constant without recognising it changes nothing")
	}

	// ---- (3) behavioural: this reader actually resolves a configured path ----
	dir := t.TempDir()
	ledgerPath := filepath.Join(dir, "ledger.jsonl")
	withRoster(t, map[string]string{
		EnvBlessLogin:        fmt.Sprintf("%s:%d", fixtureBlessLogin, fixtureBlessID),
		EnvTrustedLogins:     fmt.Sprintf("%s:%d", fixtureBlessLogin, fixtureBlessID),
		EnvContributorLedger: ledgerPath,
	})
	if got := ContributorLedgerPath(); got != ledgerPath {
		t.Errorf("ContributorLedgerPath() = %q, want %q", got, ledgerPath)
	}

	// ---- (4) unset is a complete, legitimate configuration ----
	withRoster(t, map[string]string{
		EnvBlessLogin:    fmt.Sprintf("%s:%d", fixtureBlessLogin, fixtureBlessID),
		EnvTrustedLogins: fmt.Sprintf("%s:%d", fixtureBlessLogin, fixtureBlessID),
	})
	if got := ContributorLedgerPath(); got != "" {
		t.Errorf("ContributorLedgerPath() with the key unset = %q, want \"\"", got)
	}
	if !EffectiveConfig().Configured() {
		t.Fatal("a roster with no contributor-ledger key must still be a configured roster — the key " +
			"is optional")
	}
}

// ---- published model vs. code -----------------------------------------------

// TestTrustTierPublishedModelMatchesTable parses the capability table out of
// docs/contributor-trust.md and compares it, cell by cell, to capabilityTable in
// code. A published model that CLAIMS a capability the code does not grant is a
// bar nobody enforces; a code grant the doc does not mention is a bar nobody can
// read. Either direction fails.
func TestTrustTierPublishedModelMatchesTable(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "docs", "contributor-trust.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read the published contributor-trust model at %s: %v", path, err)
	}
	lines := strings.Split(string(data), "\n")

	headerIdx := -1
	for i, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "| Tier |") {
			headerIdx = i
			break
		}
	}
	if headerIdx < 0 {
		t.Fatalf("%s: no capability table found (expected a header row starting `| Tier |`)", path)
	}
	header := splitTableRow(lines[headerIdx])
	if len(header) < 2 {
		t.Fatalf("%s: capability table header has no capability columns: %v", path, header)
	}
	caps := header[1:]

	if headerIdx+1 >= len(lines) || !strings.HasPrefix(strings.TrimSpace(lines[headerIdx+1]), "|---") &&
		!strings.HasPrefix(strings.TrimSpace(lines[headerIdx+1]), "| ---") {
		t.Fatalf("%s: no markdown table separator row immediately after the header", path)
	}

	rowFor := map[string][]string{}
	for _, l := range lines[headerIdx+2:] {
		trimmed := strings.TrimSpace(l)
		if !strings.HasPrefix(trimmed, "|") {
			break
		}
		cells := splitTableRow(trimmed)
		if len(cells) == 0 {
			continue
		}
		rowFor[cells[0]] = cells[1:]
	}

	tierByName := map[string]Tier{
		"unknown":      TierUnknown,
		"blessed-once": TierBlessedOnce,
		"contributor":  TierContributor,
		"maintainer":   TierMaintainer,
	}
	order := []string{"unknown", "blessed-once", "contributor", "maintainer"}

	for _, name := range order {
		cells, ok := rowFor[name]
		if !ok {
			t.Errorf("%s: capability table has no row for tier %q", path, name)
			continue
		}
		if len(cells) != len(caps) {
			t.Errorf("%s: tier %q row has %d cells, want %d (one per capability column)",
				path, name, len(cells), len(caps))
			continue
		}
		tier := tierByName[name]
		granted := map[string]bool{}
		for _, c := range tier.Capabilities() {
			granted[c] = true
		}
		for i, capName := range caps {
			docSaysYes := strings.EqualFold(cells[i], "yes")
			codeGrants := granted[capName]
			switch {
			case docSaysYes && !codeGrants:
				t.Errorf("%s claims tier %q is granted capability %q, but the code's capability table "+
					"does not grant it — the published model must never claim more than the code enforces",
					path, name, capName)
			case !docSaysYes && codeGrants:
				t.Errorf("the code's capability table grants tier %q capability %q, but %s says %q — "+
					"the published model must state the FULL grant", name, capName, path, cells[i])
			}
		}
	}
	if t.Failed() {
		return
	}
	t.Logf("PASS: %s's capability table matches capabilityTable for all four tiers", path)
}

// splitTableRow splits one markdown table row into trimmed cells, dropping the
// leading and trailing "|".
func splitTableRow(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")
	parts := strings.Split(line, "|")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		out = append(out, strings.TrimSpace(p))
	}
	return out
}

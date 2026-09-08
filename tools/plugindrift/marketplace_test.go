package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const pinSHA = "9a949fb543d44cfb406f61bcab99d95d7f12cf1d"

func catalogJSON(source string) []byte {
	return []byte(`{"name":"assay","plugins":[
		{"name":"assay","source":"./plugins/assay"},
		{"name":"impeccable","source":` + source + `}
	]}`)
}

func extSource() string {
	return `{"source":"git-subdir","url":"https://github.com/medici-finance/impeccable.git","path":"plugin","ref":"skill-v4.0.4","sha":"` + pinSHA + `"}`
}

func TestParseMarketplaceSkipsInRepoAndLiftsExternal(t *testing.T) {
	pins, err := ParseMarketplace(catalogJSON(extSource()))
	if err != nil {
		t.Fatalf("ParseMarketplace: %v", err)
	}
	if len(pins) != 1 {
		t.Fatalf("want 1 external pin (in-repo string source skipped), got %d", len(pins))
	}
	p := pins[0]
	if p.Plugin != "impeccable" || p.Repo != "medici-finance/impeccable" || p.Path != "plugin" || p.Ref != "skill-v4.0.4" || p.SHA != pinSHA {
		t.Fatalf("pin fields wrong: %+v", p)
	}
}

func TestParseMarketplaceRejectsMissingSHA(t *testing.T) {
	src := `{"source":"github","repo":"medici-finance/impeccable","ref":"skill-v4.0.4"}`
	_, err := ParseMarketplace(catalogJSON(src))
	if err == nil || !strings.Contains(err.Error(), "40-hex sha") {
		t.Fatalf("want a missing-sha error, got %v", err)
	}
}

func TestParseMarketplaceRejectsUnknownKind(t *testing.T) {
	src := `{"source":"marketplace","repo":"a/b","sha":"` + pinSHA + `"}`
	if _, err := ParseMarketplace(catalogJSON(src)); err == nil {
		t.Fatal("want an unknown-kind error, got nil")
	}
}

func TestGithubSlug(t *testing.T) {
	cases := map[string]string{
		"https://github.com/medici-finance/impeccable.git": "medici-finance/impeccable",
		"https://github.com/medici-finance/impeccable":     "medici-finance/impeccable",
		"git@github.com:medici-finance/impeccable.git":     "medici-finance/impeccable",
		"https://gitlab.com/acme/thing.git":                "",
		"https://github.com/onlyowner":                     "",
	}
	for in, want := range cases {
		if got := githubSlug(in); got != want {
			t.Errorf("githubSlug(%q) = %q, want %q", in, got, want)
		}
	}
}

func mustPins(t *testing.T) []MarketplacePin {
	t.Helper()
	pins, err := ParseMarketplace(catalogJSON(extSource()))
	if err != nil {
		t.Fatalf("ParseMarketplace: %v", err)
	}
	return pins
}

func TestCheckMarketplaceInSync(t *testing.T) {
	c := &fakeClient{
		branch:     "main",
		tagCommits: map[string]string{key("medici-finance/impeccable", "skill-v4.0.4"): pinSHA},
		commitDate: map[string]string{key("medici-finance/impeccable", pinSHA): "2026-08-01T00:00:00Z"},
		commits:    map[string][2]int{key("medici-finance/impeccable", "plugin"): {0, 0}},
	}
	results := CheckMarketplace(c, mustPins(t), 5)
	if len(results) != 1 {
		t.Fatalf("want 1 result, got %d", len(results))
	}
	r := results[0]
	if r.Status != StatusInSync || r.File != "marketplace:impeccable" || r.Origin != "catalog" || r.Advisory {
		t.Fatalf("want non-advisory in-sync marketplace:impeccable/catalog, got %+v", r)
	}
}

func TestCheckMarketplaceBehind(t *testing.T) {
	c := &fakeClient{
		branch:     "main",
		tagCommits: map[string]string{key("medici-finance/impeccable", "skill-v4.0.4"): pinSHA},
		commitDate: map[string]string{key("medici-finance/impeccable", pinSHA): "2026-08-01T00:00:00Z"},
		commits:    map[string][2]int{key("medici-finance/impeccable", "plugin"): {7, 0}},
	}
	r := CheckMarketplace(c, mustPins(t), 5)[0]
	if r.Status != StatusBehind || r.Commits != 7 || !r.Advisory {
		t.Fatalf("want ADVISORY behind/7 (a tag pin is deliberate; upstream movement is re-pin material), got %+v", r)
	}
}

func TestCheckMarketplaceTagMovedIsMoved(t *testing.T) {
	c := &fakeClient{
		branch:     "main",
		tagCommits: map[string]string{key("medici-finance/impeccable", "skill-v4.0.4"): strings.Repeat("ab", 20)},
	}
	r := CheckMarketplace(c, mustPins(t), 5)[0]
	if r.Status != StatusMoved {
		t.Fatalf("re-pointed tag must report moved, got %+v", r)
	}
}

func TestCheckMarketplaceMissingTagIsMoved(t *testing.T) {
	c := &fakeClient{
		branch: "main",
		tagErr: map[string]error{key("medici-finance/impeccable", "skill-v4.0.4"): ErrNotFound},
	}
	r := CheckMarketplace(c, mustPins(t), 5)[0]
	if r.Status != StatusMoved {
		t.Fatalf("missing tag must report moved, got %+v", r)
	}
}

func TestCheckMarketplaceUnreadableTagIsUnreachable(t *testing.T) {
	c := &fakeClient{
		branch: "main",
		tagErr: map[string]error{key("medici-finance/impeccable", "skill-v4.0.4"): errors.New("boom")},
	}
	r := CheckMarketplace(c, mustPins(t), 5)[0]
	if r.Status != StatusUnreachable {
		t.Fatalf("unreadable tag must report unreachable (an unknown, never a pass), got %+v", r)
	}
}

func TestCheckMarketplaceNonGithubIsUnreachable(t *testing.T) {
	src := `{"source":"url","url":"https://gitlab.com/acme/thing.git","sha":"` + pinSHA + `"}`
	pins, err := ParseMarketplace(catalogJSON(src))
	if err != nil {
		t.Fatalf("ParseMarketplace: %v", err)
	}
	r := CheckMarketplace(&fakeClient{}, pins, 5)[0]
	if r.Status != StatusUnreachable {
		t.Fatalf("non-github source must report unreachable, got %+v", r)
	}
}

func TestMarketplaceBehindIsAdvisoryNotDrift(t *testing.T) {
	c := &fakeClient{
		branch:     "main",
		tagCommits: map[string]string{key("medici-finance/impeccable", "skill-v4.0.4"): pinSHA},
		commitDate: map[string]string{key("medici-finance/impeccable", pinSHA): "2026-08-01T00:00:00Z"},
		commits:    map[string][2]int{key("medici-finance/impeccable", "plugin"): {3, 0}},
	}
	s := Summarize(CheckMarketplace(c, mustPins(t), 5))
	if s.Drift {
		t.Fatal("an intact tag pin with upstream movement is re-pin material, not drift — it must not fail --fail-on-drift")
	}
}

func TestMarketplaceBrokenPinIsDrift(t *testing.T) {
	c := &fakeClient{
		branch:     "main",
		tagCommits: map[string]string{key("medici-finance/impeccable", "skill-v4.0.4"): strings.Repeat("ab", 20)},
	}
	s := Summarize(CheckMarketplace(c, mustPins(t), 5))
	if !s.Drift {
		t.Fatal("a re-pointed tag is a broken provenance claim and must count as drift")
	}
}

// TestMarketplaceCatalogRowAdvisoryFatalSplit locks the whole four-state split the
// ui-craft/04 cadence routes on in one table: an intact tag pin is never drift (a
// `behind` catalog row is the fork's designed steady state, advisory exit 0), while
// `moved` and `unreachable` are the actionable / could-not-check states that must
// fail `--fail-on-drift` (strict exit 1). This is the contract the workflow's
// routing grep and its strict-verdict exit code both depend on.
func TestMarketplaceCatalogRowAdvisoryFatalSplit(t *testing.T) {
	base := func() *fakeClient {
		return &fakeClient{
			branch:     "main",
			tagCommits: map[string]string{key("medici-finance/impeccable", "skill-v4.0.4"): pinSHA},
			commitDate: map[string]string{key("medici-finance/impeccable", pinSHA): "2026-08-01T00:00:00Z"},
			commits:    map[string][2]int{key("medici-finance/impeccable", "plugin"): {0, 0}},
		}
	}
	cases := []struct {
		name       string
		mutate     func(*fakeClient)
		wantStatus string
		wantDrift  bool
	}{
		{"in-sync", func(c *fakeClient) {}, StatusInSync, false},
		{"behind-is-advisory", func(c *fakeClient) {
			c.commits[key("medici-finance/impeccable", "plugin")] = [2]int{7, 0}
		}, StatusBehind, false},
		{"moved-is-fatal", func(c *fakeClient) {
			c.tagCommits[key("medici-finance/impeccable", "skill-v4.0.4")] = strings.Repeat("ab", 20)
		}, StatusMoved, true},
		{"unreachable-is-fatal", func(c *fakeClient) {
			c.tagErr = map[string]error{key("medici-finance/impeccable", "skill-v4.0.4"): errors.New("boom")}
		}, StatusUnreachable, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := base()
			tc.mutate(c)
			results := CheckMarketplace(c, mustPins(t), 5)
			if got := results[0].Status; got != tc.wantStatus {
				t.Fatalf("status: got %q, want %q", got, tc.wantStatus)
			}
			if got := Summarize(results).Drift; got != tc.wantDrift {
				t.Fatalf("Summarize().Drift: got %v, want %v", got, tc.wantDrift)
			}
		})
	}
}

// TestParseMarketplaceCatalogWithNoExternalPins covers the precondition the
// --marketplace-only fail-closed guard in main() defends: a catalog whose only
// plugins are in-repo string sources (the impeccable entry renamed or removed)
// parses to ZERO external pins. In --marketplace-only mode that is the whole signal
// gone, which the next test proves must exit 2 rather than fall through to CLEAN.
func TestParseMarketplaceCatalogWithNoExternalPins(t *testing.T) {
	catalog := []byte(`{"name":"assay","plugins":[{"name":"assay","source":"./plugins/assay"}]}`)
	pins, err := ParseMarketplace(catalog)
	if err != nil {
		t.Fatalf("ParseMarketplace: %v", err)
	}
	if len(pins) != 0 {
		t.Fatalf("want 0 external pins for an in-repo-only catalog, got %d", len(pins))
	}
}

// TestMarketplaceOnlyEmptyPinsFailsClosed exercises main()'s guard end to end: with
// the external impeccable entry gone from the catalog, `--marketplace-only` must
// exit 2 and name the catalog path — never report CLEAN / exit 0, which would let a
// renamed or removed entry silence the cadence permanently.
func TestMarketplaceOnlyEmptyPinsFailsClosed(t *testing.T) {
	root := t.TempDir()
	mpDir := filepath.Join(root, ".claude-plugin")
	if err := os.MkdirAll(mpDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Catalog present and valid, but with no external plugin pin left.
	catalog := `{"name":"assay","plugins":[{"name":"assay","source":"./plugins/assay"}]}`
	if err := os.WriteFile(filepath.Join(mpDir, "marketplace.json"), []byte(catalog), 0o644); err != nil {
		t.Fatal(err)
	}
	// Build a real binary so the process exit code is the tool's own (go run masks
	// it, always exiting 1 and only printing "exit status N").
	bin := filepath.Join(t.TempDir(), "plugindrift")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	cmd := exec.Command(bin, "--root", root, "--marketplace-only")
	out, err := cmd.CombinedOutput()
	var ee *exec.ExitError
	if !errors.As(err, &ee) {
		t.Fatalf("want a non-zero exit, got err=%v, output:\n%s", err, out)
	}
	if ee.ExitCode() != 2 {
		t.Fatalf("want exit 2 (fail-closed), got %d, output:\n%s", ee.ExitCode(), out)
	}
	if !strings.Contains(string(out), "0 external plugin pin(s)") || !strings.Contains(string(out), ".claude-plugin/marketplace.json") {
		t.Fatalf("want a message naming the empty pin set and the catalog path, got:\n%s", out)
	}
}

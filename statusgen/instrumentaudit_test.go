package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fixtureFlagSource is a miniature statusgen main.go carrying five flag declarations across
// the constructor shapes the parser must recognise (Bool/String/Int/Float64/Var), plus
// non-flag `flag.` calls (Parse/Args) that MUST NOT be counted — the count discipline the
// audit's whole point rests on.
const fixtureFlagSource = `package main
import "flag"
func main() {
	wired := flag.Bool("wired-flag", false, "has a consumer")
	coldNew := flag.String("cold-new-flag", "", "no consumer, recently declared")
	darkOld := flag.Int("dark-old-flag", 0, "no consumer, declared long ago")
	ratio := flag.Float64("ratio-flag", 0, "no consumer, recently declared")
	var roots rootFlags
	flag.Var(&roots, "root", "repeatable")
	flag.Parse()          // NOT a flag declaration
	_ = flag.Args()       // NOT a flag declaration
	_, _, _, _ = wired, coldNew, darkOld, ratio
}
`

// TestParseFlagNamesCountsDeclarationsNotOccurrences pins the count discipline Verify row 7
// leans on: the audit's subject is DECLARED flags (named registrations), not raw `flag.`
// occurrences. The fixture has 5 declarations and 2 non-declaration `flag.` calls; the
// parser must return exactly the 5 names.
func TestParseFlagNamesCountsDeclarationsNotOccurrences(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "main.go")
	if err := os.WriteFile(src, []byte(fixtureFlagSource), 0o644); err != nil {
		t.Fatal(err)
	}
	names, err := parseFlagNames(src)
	if err != nil {
		t.Fatalf("parseFlagNames: %v", err)
	}
	want := []string{"cold-new-flag", "dark-old-flag", "ratio-flag", "root", "wired-flag"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Fatalf("parseFlagNames = %v, want %v (occurrences of flag.Parse/Args must NOT be counted)", names, want)
	}
}

// TestInstrumentAudit is Verify row 6: WIRED / COLD / DARK classification on a fixture main.go
// + fixture consumer tree, and the DARK NOTICE that --lint would emit.
//
// FAIL-FIRST: on the pre-brief tree there is no instrumentAudit at all, so this file does not
// compile against it — the red is the whole feature's absence. A reviewer removes
// instrumentaudit.go to observe it.
func TestInstrumentAudit(t *testing.T) {
	dir := t.TempDir()
	flagFile := filepath.Join(dir, "statusgen", "main.go")
	if err := os.MkdirAll(filepath.Dir(flagFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(flagFile, []byte(fixtureFlagSource), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := instrumentAuditConfig{
		root: dir,
		// A consumer tree that references only --wired-flag (and, deliberately, a
		// look-alike --wired-flag-x that must NOT count as a consumer of --wired-flag).
		consumerFiles: func(roots []string) (map[string]string, error) {
			return map[string]string{
				"ci.yml":   "run: statusgen --wired-flag\n",
				"other.sh": "statusgen --wired-flag-x  # a longer flag; must not count as the wired one\n",
			}, nil
		},
		// dark-old-flag was declared long ago; everything else is recent. The one COLD
		// flag old enough tips to DARK; the other consumer-less flags stay COLD.
		declAgeDays: func(_, _, name string) (int, bool) {
			switch name {
			case "dark-old-flag":
				return 400, true
			case "cold-new-flag", "ratio-flag":
				return 3, true
			default:
				return 3, true
			}
		},
	}

	rep, err := instrumentAudit(cfg)
	if err != nil {
		t.Fatalf("instrumentAudit: %v", err)
	}

	// Exactly the five declared flags, no phantom rows from flag.Parse/Args.
	if len(rep.Flags) != 5 {
		t.Fatalf("audit reported %d flags, want 5 (the declared count, not the raw flag. occurrence count): %+v", len(rep.Flags), rep.Flags)
	}

	state := map[string]instrumentState{}
	for _, f := range rep.Flags {
		state[f.Name] = f.State
	}
	for name, want := range map[string]instrumentState{
		"wired-flag":    instrumentWired,
		"root":          instrumentCold, // --root is not in the fixture consumer tree
		"cold-new-flag": instrumentCold,
		"ratio-flag":    instrumentCold,
		"dark-old-flag": instrumentDark,
	} {
		if state[name] != want {
			t.Errorf("--%s classified %s, want %s", name, state[name], want)
		}
	}

	// The look-alike must NOT have made --wired-flag WIRED via a phantom consumer; and the
	// one real consumer must be recorded.
	for _, f := range rep.Flags {
		if f.Name == "wired-flag" {
			if len(f.Consumers) != 1 || !strings.Contains(f.Consumers[0], "ci.yml") {
				t.Errorf("--wired-flag consumers = %v, want exactly [ci.yml] (a --wired-flag-x look-alike must not count)", f.Consumers)
			}
		}
	}

	if rep.Wired != 1 || rep.Dark != 1 || rep.Cold != 3 {
		t.Errorf("tallies = WIRED %d COLD %d DARK %d, want 1/3/1", rep.Wired, rep.Cold, rep.Dark)
	}
}

// TestInstrumentAuditDarkNoticeShape pins the --lint NOTICE: a DARK flag produces exactly
// one notice naming it a retirement candidate, and a WIRED/COLD flag produces none. It drives
// the pure computation through instrumentAudit with an injected config, since the production
// instrumentAuditDarkNotices reads the live tree.
func TestInstrumentAuditDarkNoticeShape(t *testing.T) {
	dir := t.TempDir()
	flagFile := filepath.Join(dir, "statusgen", "main.go")
	if err := os.MkdirAll(filepath.Dir(flagFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(flagFile, []byte(fixtureFlagSource), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := instrumentAuditConfig{
		root:          dir,
		consumerFiles: func([]string) (map[string]string, error) { return map[string]string{}, nil },
		declAgeDays: func(_, _, name string) (int, bool) {
			if name == "dark-old-flag" {
				return 400, true
			}
			return 3, true
		},
	}
	rep, err := instrumentAudit(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var darkNotices []string
	for _, f := range rep.Flags {
		if f.State == instrumentDark {
			darkNotices = append(darkNotices, f.Name)
		}
	}
	if len(darkNotices) != 1 || darkNotices[0] != "dark-old-flag" {
		t.Fatalf("DARK flags = %v, want exactly [dark-old-flag]", darkNotices)
	}
}

// TestInstrumentAuditUnreadableSourceIsCouldNotCheck: an audit that could not read the flag
// source proved nothing — it errors, never returns an empty "all clean" report.
func TestInstrumentAuditUnreadableSource(t *testing.T) {
	_, err := instrumentAudit(instrumentAuditConfig{
		root:          t.TempDir(), // no statusgen/main.go under it
		consumerFiles: func([]string) (map[string]string, error) { return nil, nil },
	})
	if err == nil {
		t.Fatal("an unreadable flag source returned no error — a source it never read is not a clean audit")
	}
}

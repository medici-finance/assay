package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	git "github.com/go-git/go-git/v5"

	"github.com/medici-finance/assay/qualgen/verifier"
)

// sweep.go is the `sweep` mode orchestrator (quality/16): it runs the three
// legs in order and computes the standing-lane diff (new / persistent / cleared
// suspects) against the prior artifacts, writing only under --out. The lane is
// REPORT-ONLY: it never edits the target repo, never auto-fixes, never files
// issues, never dispatches work. Triage is a downstream human step.
//
//	leg 1 (suspects.go)  deterministic linters nominate suspects
//	leg 2 (verdicts.go)  the configured agent verifies each NEW suspect;
//	                     the emitter enforces the evidence contract
//	leg 3 (sweepreport.go) an evidenced markdown report per run
//
// Incrementality is the cost control that makes a cadence viable: a run reads
// the prior suspects/verdicts, and only NEW fingerprints reach leg 2 (unless
// --reverify-all). Prior verdicts carry forward; the report sections suspects
// new / persistent / cleared.

// SweepSuspect is the persisted suspect record: the boundary Suspect plus the
// run date its fingerprint was first seen. It is appended to suspects.jsonl
// once per fingerprint, ever — the append-only, extend-never-replace invariant
// the quality/01 store guarantees.
type SweepSuspect struct {
	verifier.Suspect
	FirstSeen string `json:"first_seen"`
}

// SweepVerdict is the persisted verdict record: the boundary Verdict plus the
// run date it was adjudicated. Appended to verdicts.jsonl once per fingerprint
// (unless --reverify-all re-runs the agent).
type SweepVerdict struct {
	verifier.Verdict
	VerifiedAt string `json:"verified_at"`
}

// StreamSuspects streams the sweep suspects table back as typed records,
// reusing the quality/01 streamJSONL reader.
func (s *Store) StreamSuspects(fn func(SweepSuspect) error) error {
	path, _ := s.tablePath(KindSweepSuspect)
	return streamJSONL(path, fn)
}

// ReadSuspects collects the whole sweep suspects table.
func (s *Store) ReadSuspects() ([]SweepSuspect, error) {
	var out []SweepSuspect
	err := s.StreamSuspects(func(r SweepSuspect) error {
		out = append(out, r)
		return nil
	})
	return out, err
}

// StreamVerdicts streams the sweep verdicts table back as typed records.
func (s *Store) StreamVerdicts(fn func(SweepVerdict) error) error {
	path, _ := s.tablePath(KindSweepVerdict)
	return streamJSONL(path, fn)
}

// ReadVerdicts collects the whole sweep verdicts table.
func (s *Store) ReadVerdicts() ([]SweepVerdict, error) {
	var out []SweepVerdict
	err := s.StreamVerdicts(func(r SweepVerdict) error {
		out = append(out, r)
		return nil
	})
	return out, err
}

// SweepRun is everything leg 3 needs to render a report: the target SHA, the
// per-category leg-1 results (carrying the three-state measure), the current
// suspects partitioned against history, and the verdict for every current
// suspect (freshly produced or carried forward).
type SweepRun struct {
	RunDate    string
	TargetSHA  string
	Categories []CategoryResult
	Config     SweepConfig

	New        []verifier.Suspect
	Persistent []verifier.Suspect
	Cleared    []SweepSuspect

	// Verdicts is keyed by fingerprint and covers every current suspect
	// (new + persistent). Cleared suspects keep their prior verdict too.
	Verdicts map[string]verifier.Verdict
	// Reclassified is the set of fingerprints the emitter's evidence gate
	// reclassified this run (surfaced in the report for transparency).
	Reclassified map[string]bool
}

// loadSweepConfig reads and parses --config. An empty path yields an empty
// config (every category could-not-measure) — a run with no config is a
// legitimate "nothing configured yet" state, not an error.
func loadSweepConfig(path string) (SweepConfig, error) {
	var cfg SweepConfig
	if path == "" {
		return cfg, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("reading config %q: %w", path, err)
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return cfg, fmt.Errorf("parsing config %q: %w", path, err)
	}
	return cfg, nil
}

// buildVerifier constructs the leg-2 agent adapter selected by config. Only the
// offline "fixture" reference adapter is wired here; the live headless-agent
// kind is configuration shipped separately (brief scope boundary). An empty
// kind yields a nil verifier — legitimate when no suspect needs verifying (leg
// 2 errors loudly if one does).
func buildVerifier(cfg VerifierConfig) (verifier.AgentVerifier, error) {
	switch cfg.Kind {
	case "":
		return nil, nil
	case "fixture":
		if cfg.Scripts == "" {
			return nil, fmt.Errorf("verifier kind %q requires a scripts path", cfg.Kind)
		}
		return verifier.LoadFixture(cfg.Scripts)
	default:
		return nil, fmt.Errorf("unknown verifier kind %q (this build wires only \"fixture\"; a live agent adapter is configuration)", cfg.Kind)
	}
}

// targetSHA resolves the target repo's HEAD, read-only. A repo whose HEAD
// cannot be resolved (an empty repo, a detached corrupt ref) yields a
// could-not-measure marker string rather than failing the sweep — the report
// records what it could see.
func targetSHA(repoPath string) string {
	r, err := git.PlainOpenWithOptions(repoPath, &git.PlainOpenOptions{EnableDotGitCommonDir: true})
	if err != nil {
		return "could-not-measure: " + err.Error()
	}
	head, err := r.Head()
	if err != nil {
		return "could-not-measure: " + err.Error()
	}
	return head.Hash().String()
}

// runSweepLane is the testable core: it runs the three legs against an already
// resolved config, verifier, and command runner, and returns the assembled
// SweepRun. runSweep is the CLI wrapper that parses flags and wires the
// production runner. Splitting them keeps the whole lane offline-testable
// without the CLI or a live toolchain.
func runSweepLane(repoPath string, store *Store, cfg SweepConfig, av verifier.AgentVerifier, runner commandRunner, reverifyAll bool, now time.Time) (*SweepRun, error) {
	runDate := now.UTC().Format("2006-01-02")

	// Prior state.
	priorSuspects, err := store.ReadSuspects()
	if err != nil {
		return nil, fmt.Errorf("reading prior suspects: %w", err)
	}
	knownSuspect := make(map[string]SweepSuspect, len(priorSuspects))
	for _, s := range priorSuspects {
		knownSuspect[s.Fingerprint] = s
	}
	priorVerdicts, err := store.ReadVerdicts()
	if err != nil {
		return nil, fmt.Errorf("reading prior verdicts: %w", err)
	}
	knownVerdict := make(map[string]verifier.Verdict, len(priorVerdicts))
	for _, v := range priorVerdicts {
		knownVerdict[v.Fingerprint] = v.Verdict
	}

	// Leg 1.
	categories := runSuspects(repoPath, cfg, runner)
	current := map[string]verifier.Suspect{}
	var currentOrder []verifier.Suspect
	for _, cat := range categories {
		for _, s := range cat.Suspects {
			if _, dup := current[s.Fingerprint]; dup {
				continue
			}
			current[s.Fingerprint] = s
			currentOrder = append(currentOrder, s)
		}
	}

	run := &SweepRun{
		RunDate:      runDate,
		TargetSHA:    targetSHA(repoPath),
		Categories:   categories,
		Config:       cfg,
		Verdicts:     map[string]verifier.Verdict{},
		Reclassified: map[string]bool{},
	}

	// Partition current suspects against history.
	var toVerify []verifier.Suspect
	for _, s := range currentOrder {
		if _, known := knownSuspect[s.Fingerprint]; known {
			run.Persistent = append(run.Persistent, s)
			if reverifyAll {
				toVerify = append(toVerify, s)
			} else if v, ok := knownVerdict[s.Fingerprint]; ok {
				run.Verdicts[s.Fingerprint] = v // carry the prior verdict forward
			}
		} else {
			run.New = append(run.New, s)
			toVerify = append(toVerify, s)
		}
	}
	// Cleared = known fingerprints absent from the current scan.
	for _, ks := range priorSuspects {
		if _, still := current[ks.Fingerprint]; !still {
			run.Cleared = append(run.Cleared, ks)
			if v, ok := knownVerdict[ks.Fingerprint]; ok {
				run.Verdicts[ks.Fingerprint] = v
			}
		}
	}

	// Leg 2 — verify only the suspects that need it.
	outcomes, err := verifySuspects(repoPath, av, toVerify)
	if err != nil {
		return nil, err
	}
	for i, oc := range outcomes {
		fp := toVerify[i].Fingerprint
		run.Verdicts[fp] = oc.Verdict
		if oc.Reclassified {
			run.Reclassified[fp] = true
		}
	}

	// Persist: append NEW suspects and every freshly produced verdict. Reruns
	// over an unchanged tree add nothing (toVerify empty, run.New empty).
	for _, s := range run.New {
		if err := store.Append(KindSweepSuspect, SweepSuspect{Suspect: s, FirstSeen: runDate}); err != nil {
			return nil, fmt.Errorf("appending suspect: %w", err)
		}
	}
	for i := range outcomes {
		fp := toVerify[i].Fingerprint
		if err := store.Append(KindSweepVerdict, SweepVerdict{Verdict: run.Verdicts[fp], VerifiedAt: runDate}); err != nil {
			return nil, fmt.Errorf("appending verdict: %w", err)
		}
	}

	return run, nil
}

// runSweep is the `sweep` mode entry point.
func runSweep(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("sweep", flag.ContinueOnError)
	fs.SetOutput(stderr)
	repo := fs.String("repo", "", "path to the target git repository to sweep (read-only) (required)")
	out := fs.String("out", "", "tracking root the sweep artifacts land under (required)")
	configPath := fs.String("config", "", "sweep config: per-category tool set + verifier selection")
	reverifyAll := fs.Bool("reverify-all", false, "re-verify every current suspect, not just new fingerprints")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *repo == "" {
		fmt.Fprintln(stderr, "qualgen sweep: --repo <dir> is required (the target repo to sweep)")
		return 2
	}
	if *out == "" {
		fmt.Fprintln(stderr, "qualgen sweep: --out <dir> is required (the tracking root artifacts land under)")
		return 2
	}

	cfg, err := loadSweepConfig(*configPath)
	if err != nil {
		fmt.Fprintln(stderr, "qualgen sweep:", err)
		return 1
	}
	av, err := buildVerifier(cfg.Verifier)
	if err != nil {
		fmt.Fprintln(stderr, "qualgen sweep:", err)
		return 1
	}

	store := NewStore(*out)
	run, err := runSweepLane(*repo, store, cfg, av, execRunner, *reverifyAll, time.Now())
	if err != nil {
		fmt.Fprintln(stderr, "qualgen sweep:", err)
		return 1
	}

	reportPath, err := writeSweepReport(store, run)
	if err != nil {
		fmt.Fprintln(stderr, "qualgen sweep:", err)
		return 1
	}
	fmt.Fprintf(stdout, "qualgen sweep: %d new / %d persistent / %d cleared suspect(s); report %s\n",
		len(run.New), len(run.Persistent), len(run.Cleared), reportPath)
	return 0
}

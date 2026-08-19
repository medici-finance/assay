package main

// runnertable.go — the tier→runner configuration table.
//
// The initial native-dispatch design landed a single runner value (VerifyLoop.RunnerCmd):
// the whole runner selection was one argv, good enough to prove the native ACP dispatch path end-to-end but
// with no way to map a Tier onto a concrete runner. This file is that map. Each dispatchable
// Tier (`local`, `cheap`, `session`) resolves to a pinned runner entry `{cmd, model, pin}`;
// TierHuman stays non-dispatchable (routes via Land, unchanged). The table is the first real
// home for dual-track runner selection (Claude / Codex / Gemini) as a config row rather than
// new code (acp-dispatch-spec §4.3).
//
// It is additive-and-inert-by-default, exactly as the Native flag is: with NO runner-table
// env key present, native dispatch stays on the legacy single-value path and boot
// validation is a no-op. Nothing here flips on implicitly.
//
// Three fail-closed rules, all refusals (deskkit exit 5) — a runner is safety plumbing:
//   - MANDATORY PINNING (spec §7.3): an entry with no version pin is a config error, the same
//     posture as the repo's `.assay-versions` pins — the ACP adapter ecosystem renames and
//     releases fast, so an unpinned runner is a moving target, not a runner.
//   - UNKNOWN / NON-DISPATCHABLE TIER: a key that is not one of local/cheap/session refuses
//     (a `human` entry is a config error — TierHuman is never dispatched).
//   - THE C FLOOR: a runner that cannot ISOLATE the implementer REFUSES the work;
//     it never silently degrades to a shared checkout. isolate=false is therefore a config
//     error, not an accepted-but-degraded entry. Only CONVENIENCE gaps may degrade-with-
//     statement — never isolation.
//
// Namespace: `ASSAY_*` env keys ONLY — this is generic methodology config, never the
// product-deploy namespace (product values must not wear the ASSAY_ prefix, and this
// config must not read product keys).

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

// Config env keys (ASSAY_* namespace only — see the package doc).
const (
	// envRunnerTableFile points at a JSON file holding the whole table:
	// {"local": {...}, "cheap": {...}, "session": {...}}. When set it is authoritative
	// and the per-tier keys are ignored.
	envRunnerTableFile = "ASSAY_RUNNER_TABLE"
	// envRunnerPrefix + <TIER> (LOCAL/CHEAP/SESSION) each hold ONE entry as a JSON object.
	envRunnerPrefix = "ASSAY_RUNNER_"
)

// dispatchableTierNames maps a config key onto its Tier. TierHuman is deliberately ABSENT: it
// is non-dispatchable (routes via Land), so a runner entry named "human" is a config error.
var dispatchableTierNames = map[string]loopengine.Tier{
	"local":   loopengine.TierLocal,
	"cheap":   loopengine.TierCheap,
	"session": loopengine.TierSession,
}

// RunnerEntry is one resolved tier→runner mapping: the agent command the engine spawns for a
// dispatchable Tier, the model it runs, and the MANDATORY version pin.
type RunnerEntry struct {
	Cmd   []string `json:"cmd"`
	Model string   `json:"model"`
	Pin   string   `json:"pin"`
	// Isolate declares whether this runner can run the dispatched verifier in an ISOLATED
	// worktree. A nil pointer (key absent) defaults to true; an explicit false is the C-floor
	// config error (LoadRunnerTable refuses it). It is a pointer precisely so "unset" and
	// "explicitly false" are distinguishable — a plain bool would let a false silently read as
	// the zero value.
	Isolate *bool `json:"isolate,omitempty"`
}

// RunnerID is the identity the engine dispatched AS for this entry — derived from the resolved
// table row (tier + cmd + model + pin), known at SPAWN, never a worker self-report. This is the
// value that flows onto loopengine.Result.RunnerID so Evidence attribution reflects the real
// child process the engine started.
func (e RunnerEntry) RunnerID(tier loopengine.Tier) string {
	id := "acp:" + tier.String() + ":" + strings.Join(e.Cmd, " ") + "@" + e.Pin
	if e.Model != "" {
		id += "#" + e.Model
	}
	return id
}

// RunnerTable maps each dispatchable Tier to its resolved, validated RunnerEntry.
type RunnerTable struct {
	entries map[loopengine.Tier]RunnerEntry
}

// RunnerTableConfigured reports whether ANY runner-table env key is present. When none is,
// native dispatch stays on the LE/15 legacy single-value path and boot validation is a no-op
// (additive-and-inert-by-default). getenv is injected for tests.
func RunnerTableConfigured(getenv func(string) string) bool {
	if strings.TrimSpace(getenv(envRunnerTableFile)) != "" {
		return true
	}
	for name := range dispatchableTierNames {
		if strings.TrimSpace(getenv(envRunnerPrefix+strings.ToUpper(name))) != "" {
			return true
		}
	}
	return false
}

// LoadRunnerTable builds the tier→runner table from config, failing CLOSED on every rule in the
// package doc. getenv and readFile are injected for tests; a nil readFile uses os.ReadFile.
func LoadRunnerTable(getenv func(string) string, readFile func(string) ([]byte, error)) (*RunnerTable, error) {
	if readFile == nil {
		readFile = os.ReadFile
	}

	raw := map[string]RunnerEntry{}
	if path := strings.TrimSpace(getenv(envRunnerTableFile)); path != "" {
		data, err := readFile(path)
		if err != nil {
			return nil, deskkit.Unverifiable("runner table: cannot read "+envRunnerTableFile+" file "+path, err)
		}
		if err := json.Unmarshal(data, &raw); err != nil {
			return nil, deskkit.Refused("runner table: " + path + " is not valid JSON: " + err.Error())
		}
	} else {
		for name := range dispatchableTierNames {
			val := strings.TrimSpace(getenv(envRunnerPrefix + strings.ToUpper(name)))
			if val == "" {
				continue
			}
			var e RunnerEntry
			if err := json.Unmarshal([]byte(val), &e); err != nil {
				return nil, deskkit.Refused("runner table: " + envRunnerPrefix + strings.ToUpper(name) + " is not valid JSON: " + err.Error())
			}
			raw[name] = e
		}
	}

	if len(raw) == 0 {
		return nil, deskkit.Refused("runner table: no runner entries configured (set " + envRunnerTableFile + " or " + envRunnerPrefix + "<TIER>)")
	}

	// Deterministic key order so a table with several errors reports the same one every run.
	names := make([]string, 0, len(raw))
	for name := range raw {
		names = append(names, name)
	}
	sort.Strings(names)

	entries := make(map[loopengine.Tier]RunnerEntry, len(raw))
	for _, name := range names {
		tier, ok := dispatchableTierNames[name]
		if !ok {
			return nil, deskkit.Refused(fmt.Sprintf(
				"runner table: unknown/non-dispatchable tier key %q — refusing (dispatchable tiers are local, cheap, session; human is non-dispatchable and routes via Land)", name))
		}
		if err := validateEntry(name, raw[name]); err != nil {
			return nil, err
		}
		entries[tier] = raw[name]
	}
	return &RunnerTable{entries: entries}, nil
}

// validateEntry enforces the fail-closed rules for one entry (all refusals).
func validateEntry(name string, e RunnerEntry) error {
	if len(e.Cmd) == 0 || strings.TrimSpace(e.Cmd[0]) == "" {
		return deskkit.Refused(fmt.Sprintf(
			"runner table: tier %q has no cmd — refusing (a runner that reaches the network needs an explicit command)", name))
	}
	// Mandatory pinning (spec §7.3): an unpinned runner is a config error.
	if strings.TrimSpace(e.Pin) == "" {
		return deskkit.Refused(fmt.Sprintf(
			"runner table: tier %q runner %v has no version pin — refusing (pinning is mandatory; an unpinned runner is a config error, spec §7.3, same posture as .assay-versions)", name, e.Cmd))
	}
	// The C floor: a runner that cannot isolate refuses; it never degrades.
	if e.Isolate != nil && !*e.Isolate {
		return deskkit.Refused(fmt.Sprintf(
			"runner table: tier %q runner %v declares isolate=false — REFUSED (a runner that cannot isolate the implementer refuses the work; isolation never silently degrades to a shared checkout — C floor)", name, e.Cmd))
	}
	return nil
}

// Resolve returns the runner entry for a dispatchable tier, or a refusal if the tier is not
// configured — an unconfigured tier must never resolve to a silent no-runner dispatch.
func (t *RunnerTable) Resolve(tier loopengine.Tier) (RunnerEntry, error) {
	e, ok := t.entries[tier]
	if !ok {
		return RunnerEntry{}, deskkit.Refused(fmt.Sprintf(
			"runner table: no runner configured for tier %q — refusing (add it to the tier→runner table)", tier))
	}
	return e, nil
}

// ValidateReachable checks the table covers every tier the loop can actually EMIT. An
// unconfigured-but-reachable tier is a BOOT error, not a dispatch-time surprise (brief Task 3 /
// spec §4.3): the desk refuses to boot with a hole a TierPolicy decision could fall into.
func (t *RunnerTable) ValidateReachable(reachable []loopengine.Tier) error {
	for _, tier := range reachable {
		if _, ok := t.entries[tier]; !ok {
			return deskkit.Refused(fmt.Sprintf(
				"runner table: tier %q is reachable (a TierPolicy decision can emit it) but has no configured runner — refusing to boot (an unconfigured reachable tier is a startup error, not a dispatch-time surprise)", tier))
		}
	}
	return nil
}

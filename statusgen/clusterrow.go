package main

// Cluster Verify rows and the code-verified / cluster-pending queue (verdict-lane/07,
// the pod/online verify lane).
//
// A `check:cluster` Verify row is deterministic but env-bound to a LIVE cluster:
// a probe run against a real participant/validator, whose runner must be the
// privileged pod runner. The OFFLINE verify lane holds no cluster access, so it
// can never execute one — and a brief whose only non-green rows are cluster rows
// would otherwise sit permanently at `implemented`, one live confirmation short
// of `verified`, with nothing to tell "parked awaiting the pod" apart from
// "skipped, will run later".
//
// This file gives statusgen the three pieces that lane needs:
//
//	(1) the probe a cluster row names (clusterProbe) + its documented-probe
//	    registry (loadPodProbes) — so an UNKNOWN probe is a lint PROBLEM, never a
//	    silent pass (clusterRowProblems);
//	(2) the stable, greppable could-not-check marker the offline lane records for
//	    a cluster row (clusterPendingMarker / clusterMarkerProbes) — distinct from
//	    an ordinary env-bound skip; and
//	(3) the derivation of the pod runner's worklist: the briefs that are
//	    code-verified but cluster-pending (clusterPendingQueue), emitted read-only
//	    by --cluster-pending-queue.
//
// verifyrun records the marker (runWitnesses routes a cluster row to
// could-not-run with a clusterPendingMarker note); the lint validates the probe;
// the queue reads both back off the brief file. All three are DETERMINISTIC and
// READ-ONLY — nothing here writes a board or a brief.

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// podProbesRelPath is the documented-probe registry, repo-relative. One probe
// basename per line; `#` comments and blank lines are ignored. It is the list of
// probe scripts the pod runner actually provides (documented alongside the pod
// runner, verdict-lane/09) — the set a cluster row's probe must belong to. It is
// a hidden file under docs/streams/ for the same reason .history.jsonl is: it is
// board-adjacent state, not a stream of its own.
const podProbesRelPath = "docs/streams/.pod-probes"

// probeScriptRe matches a probe script token: a bare `*.sh` filename. A cluster
// row's Command names the probe it runs (as a bare script name or inside a
// larger invocation such as `kubectl exec … -- probe-participant.sh`); the probe
// is the LAST such token, so the shell wrapper around it does not shadow it.
var probeScriptRe = regexp.MustCompile(`[A-Za-z0-9]([A-Za-z0-9._-]*)\.sh\b`)

// clusterProbe returns the probe script a `check:cluster` row names — the
// basename of the last `*.sh` token in its Command cell — or "" when the Command
// names no probe (which the lint treats as a PROBLEM: a cluster row that names no
// probe routes nowhere).
func clusterProbe(command string) string {
	ms := probeScriptRe.FindAllString(strings.TrimSpace(command), -1)
	if len(ms) == 0 {
		return ""
	}
	return filepath.Base(ms[len(ms)-1])
}

// loadPodProbes reads the documented-probe registry under root. It returns the
// set of probe basenames, whether the file EXISTS at all, and any read error.
//
// The exists bit is load-bearing: a cluster row present with NO registry to
// validate against is a config gap (a PROBLEM), not a clean pass — the lint must
// be able to tell "probe not in the list" from "there is no list".
func loadPodProbes(root string) (set map[string]bool, exists bool, err error) {
	path := filepath.Join(root, filepath.FromSlash(podProbesRelPath))
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	defer f.Close()
	set = map[string]bool{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		set[filepath.Base(line)] = true
	}
	if err := sc.Err(); err != nil {
		return nil, true, err
	}
	return set, true, nil
}

// ---------------------------------------------------------------------------
// Offline-lane marker
// ---------------------------------------------------------------------------

// clusterPendingMarker is the stable, greppable could-not-check note the OFFLINE
// lane records for a cluster row (verifyrun writes it into the witness; a
// verifier records it by hand for a hand-run row). Its shape is FIXED — a
// consumer (clusterMarkerProbes, --cluster-pending-queue, a human grep) matches
// it verbatim, so it must never drift:
//
//	cluster-row (pod-runner-pending, probe=<script>)
//
// It is deliberately distinct from verifyEnvBoundNote (an ordinary env-bound
// skip): the whole point is that a reader — or the queue derivation — can tell a
// row PARKED for the pod from a row that will simply run later in CI.
func clusterPendingMarker(probe string) string {
	return fmt.Sprintf("cluster-row (pod-runner-pending, probe=%s)", probe)
}

// clusterMarkerRe parses the probe out of every clusterPendingMarker occurrence.
var clusterMarkerRe = regexp.MustCompile(`cluster-row \(pod-runner-pending, probe=([^)]+)\)`)

// clusterMarkerProbes returns the set of probes the offline lane has PARKED in a
// brief's Evidence — every probe named by a clusterPendingMarker. Empty when the
// Evidence records no cluster-pending marker.
func clusterMarkerProbes(evidence string) map[string]bool {
	set := map[string]bool{}
	for _, m := range clusterMarkerRe.FindAllStringSubmatch(evidence, -1) {
		set[strings.TrimSpace(m[1])] = true
	}
	return set
}

// ---------------------------------------------------------------------------
// Lint: unknown / unnamed probe
// ---------------------------------------------------------------------------

// clusterRowProblems is the cluster-probe lint. A `check:cluster` row routes to a
// pod-side probe script; if the probe cannot be resolved the row proves nothing,
// so each shape is a hard PROBLEM:
//
//	(a) a cluster row whose Command names no probe script (`*.sh`) — it routes
//	    nowhere; and
//	(b) a cluster row whose probe is not a DOCUMENTED probe — either the registry
//	    (docs/streams/.pod-probes) is absent while cluster rows exist, or the probe
//	    is not listed in it. An undocumented probe names a script no pod runner
//	    provides, so the row can never be executed.
//
// Like the unknown-class arm of verifyRowClassProblems, this fires on every
// brief-v1 file regardless of README status: a probe that names nothing (or names
// a script that does not exist) is wrong the moment it is written, not once the
// brief is worked. The registry is loaded once per root.
func clusterRowProblems(streams []*Stream) []string {
	var problems []string
	add := func(format string, a ...any) { problems = append(problems, fmt.Sprintf(format, a...)) }

	probesByRoot := map[string]map[string]bool{}
	existsByRoot := map[string]bool{}
	loaded := map[string]bool{}

	for _, s := range streams {
		if !loaded[s.Root] {
			set, exists, err := loadPodProbes(s.Root)
			if err == nil {
				probesByRoot[s.Root] = set
				existsByRoot[s.Root] = exists
			}
			loaded[s.Root] = true
		}
		probes, exists := probesByRoot[s.Root], existsByRoot[s.Root]

		for _, path := range briefFilePaths(s) {
			bf, ok, err := parseBriefFile(path)
			if err != nil || !ok {
				continue // malformed reported by checkBriefFiles; legacy exempt
			}
			verifyRowTable(bf.Verify, func(r verifyRowCells) {
				if r.class() != classCheckCluster {
					return
				}
				where := "a Verify row"
				if r.Num != "" {
					where = "Verify row " + r.Num
				}
				probe := clusterProbe(r.Command)
				switch {
				case probe == "":
					add("%s: %s is a `check:cluster` row whose Command names no probe script — a cluster row is run by a pod-side probe (`*.sh`), and one that names none routes nowhere. Put the probe the pod runner executes in the Command cell — verdict-lane/07", path, where)
				case !exists:
					add("%s: %s is a `check:cluster` row naming probe %q, but no documented-probe registry (%s) exists to validate it against — a cluster row must resolve to a probe the pod runner actually provides. Add %s listing the documented probes — verdict-lane/07", path, where, probe, podProbesRelPath, podProbesRelPath)
				case !probes[probe]:
					add("%s: %s is a `check:cluster` row naming probe %q, which no documented probe provides (%s lists the pod runner's probes). A cluster row pointing at an undocumented probe can never be executed; fix the probe name or document it — verdict-lane/07", path, where, probe, podProbesRelPath)
				}
			})
		}
	}
	sort.Strings(problems)
	return problems
}

// ---------------------------------------------------------------------------
// The code-verified / cluster-pending queue
// ---------------------------------------------------------------------------

// clusterPendingEntry is one brief on the pod runner's worklist: a brief that is
// code-verified but cluster-pending. Probes is the sorted set of cluster probes
// the pod runner still has to run for it.
type clusterPendingEntry struct {
	Brief  string   `json:"brief"`  // "<stream>/<NN>"
	Stream string   `json:"stream"` //
	Status string   `json:"status"` // always "implemented" — the parked state
	Repo   string   `json:"repo,omitempty"`
	Probes []string `json:"probes"` // the cluster probes still pending
}

// clusterPendingQueue derives the pod runner's worklist: the briefs the offline
// lane has handed off code-verified, whose ONLY unrun Verify rows are cluster
// rows. A brief qualifies when ALL of:
//
//   - its README status is `implemented` — code landed and code-verified, parked
//     one step short of `verified` (verdict-lane/07: a cluster-only-blocked brief
//     sits permanently at implemented);
//   - it declares at least one `check:cluster` Verify row;
//   - the OFFLINE lane has PARKED every one of those cluster probes — each
//     appears in a clusterPendingMarker in the brief's Evidence (so a probe the
//     offline lane never reached does not count as pending-for-the-pod); and
//   - the Evidence records no VERIFY:FAIL — a failing non-cluster row is rework
//     the implementer owns, not work for the pod (lastVerifyVerdict).
//
// It is DETERMINISTIC and READ-ONLY. The result is sorted by brief id.
func clusterPendingQueue(streams []*Stream) []clusterPendingEntry {
	var out []clusterPendingEntry
	for _, s := range streams {
		repo, _ := rootRepo([]*Stream{s})
		status := map[string]string{}
		for _, br := range s.Briefs {
			status[br.Num] = br.Status
		}
		for _, path := range briefFilePaths(s) {
			bf, ok, err := parseBriefFile(path)
			if err != nil || !ok {
				continue
			}
			_, num, _ := expectedBriefID(path)
			if status[num] != "implemented" {
				continue // only the parked state is a pod worklist candidate
			}

			// Collect the cluster probes this brief declares.
			clusterProbes := map[string]bool{}
			verifyRowTable(bf.Verify, func(r verifyRowCells) {
				if r.class() != classCheckCluster {
					return
				}
				if p := clusterProbe(r.Command); p != "" {
					clusterProbes[p] = true
				}
			})
			if len(clusterProbes) == 0 {
				continue // no cluster row — not this queue's business
			}
			// A failing non-cluster row is rework, not pod work.
			if lastVerifyVerdict(bf.Evidence) == verdictFail {
				continue
			}
			// Every declared cluster probe must be PARKED by the offline lane.
			parked := clusterMarkerProbes(bf.Evidence)
			allParked := true
			for p := range clusterProbes {
				if !parked[p] {
					allParked = false
					break
				}
			}
			if !allParked {
				continue
			}

			probes := make([]string, 0, len(clusterProbes))
			for p := range clusterProbes {
				probes = append(probes, p)
			}
			sort.Strings(probes)
			out = append(out, clusterPendingEntry{
				Brief:  s.Name + "/" + num,
				Stream: s.Name,
				Status: "implemented",
				Repo:   repo,
				Probes: probes,
			})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Brief < out[j].Brief })
	return out
}

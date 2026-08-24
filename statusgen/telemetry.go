package main

// Opt-in, anonymized statusgen telemetry — a category-counts-only fleet-drift
// signal that is OFF BY DEFAULT and leaves the machine only after an explicit
// double opt-in. The design has one job above all others: nothing that could
// identify a repo, a person, or the content of their work may ever be
// transmitted. Two properties enforce that:
//
//	1. Double opt-in. Telemetry is armed only when BOTH the --telemetry flag is
//	   passed AND the environment variable ASSAY_TELEMETRY is exactly "1". Two
//	   independent switches, one on the command line and one in the environment,
//	   so no CI vendor default, wrapper script, or inherited env can flip
//	   collection on silently. Absent either, statusgen collects and sends
//	   nothing — the default run makes no network call at all.
//
//	2. Anonymized payload. The payload is COUNTS ONLY: category tallies of lint
//	   failures, tallies of lifecycle transitions between the fixed status
//	   vocabulary, brief/stream counts, and the statusgen version. It never
//	   carries a repo name, a stream name, a brief number or title, a file path,
//	   register text, or any identity. The category and transition keys are all
//	   drawn from a fixed vocabulary defined in THIS file — never from the input
//	   — so a lint message that embeds a name or path cannot escape through a
//	   category label. See classifyLintProblem and normalizeStatus.
//
// Every armed run PRINTS the exact payload it would send (printTelemetryPayload)
// — the "you can always see what leaves your machine" promise. A --telemetry-dry-run
// forces the print-only path and never transmits, even were an endpoint configured.
//
// No receiver exists yet: telemetryEndpoint is empty in every shipped build, so
// sendTelemetry is a no-op that dials nothing. Standing up the receiver is
// Console-side, out of scope here. See docs/telemetry.md.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// telemetryEnvVar is the environment half of the double opt-in. Telemetry is
// armed only when this is exactly "1" AND --telemetry is on the command line.
const telemetryEnvVar = "ASSAY_TELEMETRY"

// telemetrySchemaVersion versions the payload shape. Bump it (and the field
// table in docs/telemetry.md) whenever a field is added, removed, or its
// meaning changes, so a receiver can tell payloads apart.
const telemetrySchemaVersion = "telemetry-v1"

// telemetryEndpoint is the receiver URL. It is EMPTY in every shipped build: no
// receiver has been stood up (that is Console-side v-next work, out of scope
// here). While it is empty, sendTelemetry returns before any network call —
// nothing ever leaves the machine. It is a var, not a const, only so a future
// build that stands up a receiver could set it with `-ldflags -X`, exactly as
// statusgenVersion is stamped; there is deliberately NO runtime flag or env that
// sets it, so it cannot be pointed at a host at run time.
var telemetryEndpoint = ""

// telemetryArmed reports whether telemetry may act. Both switches are required:
// the --telemetry flag (flagSet) and ASSAY_TELEMETRY=1 in the environment.
func telemetryArmed(flagSet bool) bool {
	return flagSet && os.Getenv(telemetryEnvVar) == "1"
}

// telemetryKnownStatuses is the fixed lifecycle vocabulary. normalizeStatus maps
// anything outside it to a generic label so an unexpected or hand-edited status
// string can never travel verbatim in a transition key.
var telemetryKnownStatuses = map[string]bool{
	"todo":        true,
	"in-progress": true,
	"implemented": true,
	"verified":    true,
	"done":        true,
	"blocked":     true,
}

// normalizeStatus collapses a brief/transition status to the fixed vocabulary.
// "" (the seed/unset state) becomes "none"; an unrecognized value becomes
// "other". The return is ALWAYS one of these constants — never the input — so a
// custom status string cannot leak through a status count or a transition key.
func normalizeStatus(s string) string {
	if s == "" {
		return "none"
	}
	if telemetryKnownStatuses[s] {
		return s
	}
	return "other"
}

// Lint-failure categories. Every value below is a constant defined here; the
// classifier returns one of them and never any substring of the message it was
// handed. Reused by docs/telemetry.md's field table.
const (
	lintCatFrontmatter       = "frontmatter"
	lintCatArchiveHygiene    = "archive-hygiene"
	lintCatConcurrencyConfig = "concurrency-config"
	lintCatBriefNumbering    = "brief-numbering"
	lintCatBriefStatus       = "brief-status"
	lintCatFindingAck        = "finding-ack"
	lintCatFindingReference  = "finding-reference"
	lintCatUnresolvedFinding = "unresolved-finding"
	lintCatWordBudget        = "word-budget"
	lintCatLinkCheck         = "link-check"
	lintCatUncategorized     = "uncategorized"
)

// classifyLintProblem maps a raw lint-problem message to a stable, anonymous
// category. It NEVER returns any part of msg: the return value is always one of
// the lintCat* constants above. A message that embeds a stream name, brief
// title, file path, or register text therefore cannot escape through the
// category. An unrecognized shape falls through to lintCatUncategorized — a
// count, never the text. Matching is done on lower-cased distinctive fragments
// of the message TEMPLATES (not their %q-interpolated identifiers), so
// re-categorization stays a display concern with no privacy weight: even a
// miscategorized message leaks nothing, because only the label travels.
func classifyLintProblem(msg string) string {
	m := strings.ToLower(msg)
	switch {
	case strings.Contains(m, "docs/archive"):
		return lintCatArchiveHygiene
	case strings.Contains(m, "max-concurrent"):
		return lintCatConcurrencyConfig
	case strings.Contains(m, "duplicate brief number"):
		return lintCatBriefNumbering
	case strings.Contains(m, "requires a verified entry"),
		strings.Contains(m, "requires verified and reviewed"),
		strings.Contains(m, "invalid status"):
		return lintCatBriefStatus
	case strings.Contains(m, "malformed ack"):
		return lintCatFindingAck
	case strings.Contains(m, "affects references unknown"):
		return lintCatFindingReference
	case strings.Contains(m, "unresolved") && strings.Contains(m, "demote to todo"):
		return lintCatUnresolvedFinding
	case strings.Contains(m, "invalid stream status"),
		strings.Contains(m, "frontmatter stream"),
		strings.Contains(m, "requires a priority"),
		strings.Contains(m, "invalid priority"),
		strings.Contains(m, "invalid track"),
		strings.Contains(m, "tiering present"):
		return lintCatFrontmatter
	case strings.Contains(m, "budget"):
		return lintCatWordBudget
	case strings.Contains(m, "link"), strings.Contains(m, "broken reference"):
		return lintCatLinkCheck
	default:
		return lintCatUncategorized
	}
}

// TelemetryPayload is the entire wire format. Every field is either an integer
// count or a map from a fixed-vocabulary/constant key to an integer count.
// There is deliberately no field capable of carrying a name, title, path, or
// free text — the type itself is the anonymization guarantee.
type TelemetryPayload struct {
	Schema                string         `json:"schema"`
	StatusgenVersion      string         `json:"statusgen_version"`
	StreamCount           int            `json:"stream_count"`
	BriefCount            int            `json:"brief_count"`
	BriefStatusCounts     map[string]int `json:"brief_status_counts"`
	LintFailureCategories map[string]int `json:"lint_failure_categories"`
	LifecycleTransitions  map[string]int `json:"lifecycle_transitions"`
}

// buildTelemetryPayload assembles the payload from already-loaded, structural
// inputs. It is pure (no I/O) so the leak-invariant is unit-testable in
// isolation: feed it streams/problems/history full of sentinel identifiers and
// assert none survive into the marshaled JSON.
func buildTelemetryPayload(streams []*Stream, problems []string, history []HistoryEntry) TelemetryPayload {
	p := TelemetryPayload{
		Schema:                telemetrySchemaVersion,
		StatusgenVersion:      statusgenVersion,
		StreamCount:           len(streams),
		BriefStatusCounts:     map[string]int{},
		LintFailureCategories: map[string]int{},
		LifecycleTransitions:  map[string]int{},
	}
	for _, s := range streams {
		for _, b := range s.Briefs {
			p.BriefCount++
			p.BriefStatusCounts[normalizeStatus(b.Status)]++
		}
	}
	for _, msg := range problems {
		p.LintFailureCategories[classifyLintProblem(msg)]++
	}
	for _, e := range history {
		key := normalizeStatus(e.From) + "->" + normalizeStatus(e.To)
		p.LifecycleTransitions[key]++
	}
	return p
}

// collectTelemetryPayload loads one root's source tree and derives the payload.
// It is self-contained and STATUS.md-free — same discipline as --dora/--gate-telemetry:
// it reads the streams, the lint-problem set, and the append-only history log,
// and touches nothing else. A missing or malformed history log is not fatal to a
// diagnostic (the transitions are optional signal), so it degrades to no
// transitions rather than failing the run.
func collectTelemetryPayload(root string) (TelemetryPayload, error) {
	streams, findings, err := loadStreams(root)
	if err != nil {
		return TelemetryPayload{}, err
	}
	attachPlaceholders(streams)
	problems, _ := check(streams, findings)
	histPath := filepath.Join(root, filepath.FromSlash(historyRelPath))
	history, herr := LoadHistory(histPath)
	if herr != nil {
		fmt.Fprintln(os.Stderr, "telemetry: history log unreadable, proceeding with no transitions:", herr)
		history = nil
	}
	return buildTelemetryPayload(streams, problems, history), nil
}

// printTelemetryPayload writes the exact JSON that would be transmitted — the
// "you can always see what leaves your machine" promise. Deterministic:
// encoding/json sorts map keys.
func printTelemetryPayload(w io.Writer, p TelemetryPayload) error {
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "telemetry: payload (schema %s) — this is exactly what would be sent:\n", p.Schema); err != nil {
		return err
	}
	_, err = w.Write(append(b, '\n'))
	return err
}

// errTelemetryNoEndpoint is returned by sendTelemetry whenever no receiver is
// configured — the case in every shipped build.
var errTelemetryNoEndpoint = fmt.Errorf("telemetry: no endpoint configured — nothing sent (a receiver is v-next; see docs/telemetry.md)")

// sendTelemetry transmits the payload to telemetryEndpoint. When the endpoint is
// empty — every shipped build — it returns errTelemetryNoEndpoint WITHOUT
// dialing, so a run never contacts the network. The POST below is the client the
// future receiver will use; it is unreachable until a build stamps a non-empty
// endpoint.
func sendTelemetry(p TelemetryPayload) error {
	if telemetryEndpoint == "" {
		return errTelemetryNoEndpoint
	}
	body, err := json.Marshal(p)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, telemetryEndpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("telemetry: endpoint returned %s", resp.Status)
	}
	return nil
}

// runTelemetry collects, prints, and (unless dryRun) attempts to send the
// payload for one root. It is only reached when telemetry is armed. Telemetry
// never fails the surrounding run: a collection or send error is reported on
// stderr and swallowed, so a lint/regen exit code is never masked by it.
func runTelemetry(root string, dryRun bool) {
	p, err := collectTelemetryPayload(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "telemetry: could not collect payload:", err)
		return
	}
	if perr := printTelemetryPayload(os.Stdout, p); perr != nil {
		fmt.Fprintln(os.Stderr, "telemetry: could not print payload:", perr)
	}
	if dryRun {
		fmt.Fprintln(os.Stderr, "telemetry: dry-run — nothing sent.")
		return
	}
	if serr := sendTelemetry(p); serr != nil {
		fmt.Fprintln(os.Stderr, serr)
	}
}

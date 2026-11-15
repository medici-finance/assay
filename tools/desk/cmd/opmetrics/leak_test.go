package main

// leak_test.go — THE ACCEPTANCE CRITERION, not a formality.
//
// This tool reads a person's private session transcripts and writes a file into a git
// repository. The single thing that must never happen is a fragment of what was said
// crossing that boundary. The brief gates on a human review of exactly this, so the
// test is built to CATCH a leak rather than to record an intention.
//
// Four independent instruments, because one is a single point of failure:
//
//	TestLeakEmitStringFieldsAreAllowlisted   — the TYPE cannot hold text
//	TestLeakEmittedStringValuesAreEnumerated — the VALUES come from closed vocabularies
//	TestLeakNoTranscriptContentReachesTheArtifact — nothing from the fixtures is present
//	TestLeakScannerSeesPlantedLeak       — the scanner above actually works
//	TestLeakEmittedKeysAreAllowlisted    — every emitted KEY, counts included, is pinned
//
// with a planted-input control for the vocabulary and key instruments too
// (TestLeakEnumScannerSeesPlantedFreeText, TestLeakKeyScannerSeesPlantedKey).
//
// The last one is the three-states rule applied to the test itself: a scan that finds
// nothing because the SCAN is broken must never read as clean. It plants a leak into a
// doctored artifact and fails if the scanner stays quiet.
//
// A NOTE ON THE SENTINEL'S SHAPE. The sentinel is word-shaped
// ("sentinel-must-not-appear-in-any-emit") rather than token-shaped. A realistic
// credential literal in a fixture would trip the repository's own secret scanners and
// the desk BodyCheck, which would make this file unmergeable for a reason unrelated to
// what it proves. What the test actually needs is a string that (a) exists only in the
// fixture transcripts and (b) is long and distinctive; both hold, and the ≥12-character
// substring sweep below covers the general case regardless of shape.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
)

const (
	sentinelSecret  = "sentinel-must-not-appear-in-any-emit"
	sentinelPhrase  = "distinctive-fixture-phrase-zulu"
	minLeakFragment = 12
)

// emitFixtureArtifact runs the full CLI over the fixtures and returns the bytes that
// would land in the repository. Every leak test scans THESE bytes — the real artifact,
// not a hand-built struct.
func emitFixtureArtifact(t *testing.T) []byte {
	t.Helper()
	isolateHome(t)
	root := t.TempDir()
	var stdout, stderr strings.Builder
	// Both fixture trees: the operator-turn fixtures, and the dispatch fixture whose
	// prompts, subagent briefs and final reports feed the dispatch and token blocks.
	code := run([]string{
		"--transcripts", fxTranscripts,
		"--transcripts", fxDispatch,
		"--desk-tools", fxDeskTools,
		"--gh-fixture", fxGH,
		"--root", root,
		"--date", "2026-07-22",
		"--now", "2026-07-22T15:00:00Z",
		"--tz", "UTC",
		"--trend", "7",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("emit exited %d: %s", code, stderr.String())
	}
	raw, err := os.ReadFile(DayFilePath(root, "2026-07-22"))
	if err != nil {
		t.Fatalf("reading the emitted day-file: %v", err)
	}
	return raw
}

// transcriptStrings collects every message body in the fixture transcripts — user
// turns, assistant turns and tool results alike. This is the PRIVATE material; the
// scan's corpus is deliberately wider than the subset the collector actually reads,
// because a leak from a record type the collector "ignores" is still a leak.
func transcriptStrings(t *testing.T) []string {
	t.Helper()
	var out []string
	for _, root := range []string{fxTranscripts, fxDispatch} {
		out = append(out, transcriptStringsUnder(t, root)...)
	}
	if len(out) < 10 {
		t.Fatalf("only %d fixture messages collected — the scan corpus is not being built, "+
			"so a clean result here would prove nothing", len(out))
	}
	return out
}

// transcriptStringsUnder collects one fixture root's text: message bodies, text and
// tool-result parts, and the free-text inputs of tool calls (a dispatch prompt, a
// subagent's final report).
func transcriptStringsUnder(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".jsonl") {
			return nil
		}
		raw, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		for _, line := range strings.Split(string(raw), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			var rec struct {
				Message struct {
					Content json.RawMessage `json:"content"`
				} `json:"message"`
			}
			if json.Unmarshal([]byte(line), &rec) != nil || len(rec.Message.Content) == 0 {
				continue
			}
			var s string
			if json.Unmarshal(rec.Message.Content, &s) == nil {
				out = append(out, s)
				continue
			}
			var parts []struct {
				Text    string         `json:"text"`
				Content string         `json:"content"`
				Input   map[string]any `json:"input"`
			}
			if json.Unmarshal(rec.Message.Content, &parts) == nil {
				for _, p := range parts {
					if p.Text != "" {
						out = append(out, p.Text)
					}
					if p.Content != "" {
						out = append(out, p.Content)
					}
					for _, v := range p.Input {
						if s, ok := v.(string); ok && s != "" {
							out = append(out, s)
						}
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("collecting fixture transcript text under %s: %v", root, err)
	}
	return out
}

// scanForLeaks reports every way a fragment of corpus text appears in artifact: the
// two named sentinels, and any run of >= minLeakFragment characters taken from any
// corpus message. The substring sweep is what makes this general — it catches a leak
// through a field nobody thought to check, including one added next year.
func scanForLeaks(artifact []byte, corpus []string) []string {
	hay := string(artifact)
	var findings []string
	for _, s := range []string{sentinelSecret, sentinelPhrase} {
		if strings.Contains(hay, s) {
			findings = append(findings, "sentinel present: "+s)
		}
	}
	for _, msg := range corpus {
		r := []rune(msg)
		for i := 0; i+minLeakFragment <= len(r); i++ {
			frag := string(r[i : i+minLeakFragment])
			if strings.TrimSpace(frag) == "" {
				continue
			}
			if strings.Contains(hay, frag) {
				findings = append(findings, "fragment present: "+frag)
			}
		}
	}
	sort.Strings(findings)
	return findings
}

func TestLeakNoTranscriptContentReachesTheArtifact(t *testing.T) {
	artifact := emitFixtureArtifact(t)
	corpus := transcriptStrings(t)
	if findings := scanForLeaks(artifact, corpus); len(findings) > 0 {
		t.Fatalf("the emitted day-file carries transcript content:\n  %s\nartifact:\n%s",
			strings.Join(findings, "\n  "), artifact)
	}
	t.Logf("leak scan clean: %d fixture messages, %d artifact bytes, sentinels absent",
		len(corpus), len(artifact))
}

// TestLeakScannerSeesPlantedLeak is the CONTROL. It doctors the real artifact by
// splicing in a sentinel and a message fragment, and fails if the scanner still reads
// clean. Without it, a scanner that silently stopped working would turn every future
// run of the test above into a green light for a leak.
func TestLeakScannerSeesPlantedLeak(t *testing.T) {
	artifact := emitFixtureArtifact(t)
	corpus := transcriptStrings(t)

	doctored := append([]byte(nil), artifact...)
	doctored = append(doctored, []byte("\n"+sentinelSecret+"\n")...)
	if findings := scanForLeaks(doctored, corpus); len(findings) == 0 {
		t.Fatal("the scanner did not notice a planted sentinel — it cannot prove anything about a clean run")
	}

	var longest string
	for _, m := range corpus {
		if len([]rune(m)) > len([]rune(longest)) {
			longest = m
		}
	}
	frag := string([]rune(longest)[:minLeakFragment+8])
	doctored2 := append([]byte(nil), artifact...)
	doctored2 = append(doctored2, []byte("\n"+frag+"\n")...)
	if findings := scanForLeaks(doctored2, corpus); len(findings) == 0 {
		t.Fatalf("the scanner did not notice a planted message fragment %q", frag)
	}
}

// TestLeakEmitStringFieldsAreAllowlisted is the STRUCTURAL instrument. It reflects
// over the whole Report type and fails if any string-typed leaf exists whose JSON path
// is not pinned below. Adding a `SampleMessage string` — or a map[string]int keyed on
// anything transcript-derived — fails here, at compile-and-test time, long before it
// can run against a real transcript.
func TestLeakEmitStringFieldsAreAllowlisted(t *testing.T) {
	allowed := map[string]string{
		"schema":                           "fixed literal",
		"date":                             "YYYY-MM-DD",
		"generatedAt":                      "RFC3339 stamp",
		"classifierVersion":                "fixed literal",
		"operator.status":                  "Status enum",
		"operator.reason":                  "Reason enum",
		"intervention.status":              "Status enum",
		"intervention.reason":              "Reason enum",
		"decision_latency.status":          "Status enum",
		"decision_latency.reason":          "Reason enum",
		"session_hygiene.status":           "Status enum",
		"session_hygiene.reason":           "Reason enum",
		"correction_recurrence.status":     "Status enum",
		"correction_recurrence.reason":     "Reason enum",
		"unmeasured[]":                     "Unmeasured code enum",
		"trend.status":                     "Status enum",
		"trend.reason":                     "Reason enum",
		"dispatch.status":                  "Status enum",
		"dispatch.reason":                  "Reason enum",
		"token_efficiency.status":          "Status enum",
		"token_efficiency.reason":          "Reason enum",
		"token_efficiency.noop_classifier": "fixed literal (NoopClassifierVersion)",
	}
	found := map[string]bool{}
	walkStringLeaves(t, reflect.TypeOf(Report{}), "", found)

	for path := range found {
		if _, ok := allowed[path]; !ok {
			t.Errorf("the day-file type gained a string field at %q that is not on the allowlist. "+
				"Every string in this artifact must be a fixed literal, a closed enum, or a date — "+
				"a free string field is where transcript content leaks. Either drop the field or "+
				"add it here WITH the closed vocabulary it draws from", path)
		}
	}
	for path := range allowed {
		if !found[path] {
			t.Errorf("allowlist entry %q no longer exists in the Report type — a stale allowlist "+
				"hides the fields it was meant to pin", path)
		}
	}
}

// walkStringLeaves records the JSON path of every string-kinded leaf in t, following
// pointers, slices and nested structs. A map with a string key or value is reported as
// a leaf too: dynamic keys are exactly the hole the allowlist exists to close.
func walkStringLeaves(t *testing.T, typ reflect.Type, prefix string, out map[string]bool) {
	t.Helper()
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	switch typ.Kind() {
	case reflect.String:
		out[strings.TrimSuffix(prefix, ".")] = true
		return
	case reflect.Slice, reflect.Array:
		walkStringLeaves(t, typ.Elem(), strings.TrimSuffix(prefix, ".")+"[].", out)
		return
	case reflect.Map:
		out[strings.TrimSuffix(prefix, ".")+"{}"] = true
		return
	case reflect.Struct:
		for i := 0; i < typ.NumField(); i++ {
			f := typ.Field(i)
			name := strings.Split(f.Tag.Get("json"), ",")[0]
			if name == "-" {
				continue
			}
			if name == "" {
				name = f.Name
			}
			walkStringLeaves(t, f.Type, prefix+name+".", out)
		}
		return
	}
}

var (
	dateRe    = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	rfc3339Re = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})$`)
)

// TestLeakEmittedStringValuesAreEnumerated is the VOCABULARY instrument. It walks the
// EMITTED json — not the type — and fails on any string value outside the closed sets.
// It is the backstop for a field that is allowlisted but starts carrying free text.
func TestLeakEmittedStringValuesAreEnumerated(t *testing.T) {
	artifact := emitFixtureArtifact(t)
	if bad := nonEnumeratedStrings(t, artifact); len(bad) > 0 {
		t.Fatalf("the day-file carries string values outside the closed vocabularies "+
			"(fixed literals, Status, Reason, Unmeasured codes, dates):\n  %s", strings.Join(bad, "\n  "))
	}
}

// TestLeakEnumScannerSeesPlantedFreeText is the vocabulary instrument's CONTROL:
// free text planted into an allowlisted field of the real artifact must be reported.
func TestLeakEnumScannerSeesPlantedFreeText(t *testing.T) {
	var doc map[string]any
	if err := json.Unmarshal(emitFixtureArtifact(t), &doc); err != nil {
		t.Fatal(err)
	}
	doc["dispatch"].(map[string]any)["reason"] = "please dispatch the gamma worker"
	planted, _ := json.Marshal(doc)
	if bad := nonEnumeratedStrings(t, planted); len(bad) != 1 || !strings.HasPrefix(bad[0], "dispatch.reason") {
		t.Fatalf("free text planted at dispatch.reason was not the one finding: %v", bad)
	}
}

// nonEnumeratedStrings walks emitted JSON and returns every string value outside the
// closed vocabularies, as "path = value".
func nonEnumeratedStrings(t *testing.T, artifact []byte) []string {
	t.Helper()
	var doc any
	if err := json.Unmarshal(artifact, &doc); err != nil {
		t.Fatalf("artifact is not valid JSON: %v", err)
	}

	allowedValue := func(s string) bool {
		switch s {
		case SchemaVersion, ClassifierVersion, NoopClassifierVersion:
			return true
		}
		for _, st := range []Status{StatusOK, StatusCouldNotCheck, StatusNoPriorData} {
			if s == string(st) {
				return true
			}
		}
		for _, r := range AllReasons {
			if s == string(r) {
				return true
			}
		}
		for _, u := range AllUnmeasured {
			if s == u {
				return true
			}
		}
		return dateRe.MatchString(s) || rfc3339Re.MatchString(s)
	}

	var bad []string
	var walk func(node any, path string)
	walk = func(node any, path string) {
		switch v := node.(type) {
		case map[string]any:
			keys := make([]string, 0, len(v))
			for k := range v {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				walk(v[k], path+"."+k)
			}
		case []any:
			for i, e := range v {
				walk(e, fmt.Sprintf("%s[%d]", path, i))
			}
		case string:
			if !allowedValue(v) {
				bad = append(bad, fmt.Sprintf("%s = %q", strings.TrimPrefix(path, "."), v))
			}
		}
	}
	walk(doc, "")
	return bad
}

// emittedKeys is every key path the day-file may carry, pinned. Arrays are "[]".
// Adding a key — a count included — means adding it here, in review, next to the
// closed vocabulary or the count it is. A key that is not here fails
// TestLeakEmittedKeysAreAllowlisted on the emitted bytes.
var emittedKeys = []string{
	"schema", "date", "generatedAt", "classifierVersion", "relay_ratio",
	"operator", "operator.status", "operator.reason", "operator.messages_total",
	"operator.messages_classified", "operator.relay_messages", "operator.substantive_messages",
	"operator.empty_messages", "operator.relay_ratio", "operator.relay_families",
	"operator.relay_families.sync", "operator.relay_families.state_echo",
	"operator.relay_families.poke", "operator.relay_families.lookup",
	"operator.relay_families.duplicate", "operator.attention_families",
	"operator.attention_families.route", "operator.attention_families.status",
	"operator.attention_families.toil", "operator.attention_families.correction",
	"operator.attention_families.decision", "operator.attention_families.idea",
	"operator.attention_families.ack", "operator.attention_families.other",
	"operator.transcript_files", "operator.unparseable_lines",
	"intervention", "intervention.status", "intervention.reason",
	"intervention.operator_messages", "intervention.merged_prs",
	"intervention.messages_per_merged_pr",
	"decision_latency", "decision_latency.status", "decision_latency.reason",
	"decision_latency.samples", "decision_latency.p50_minutes", "decision_latency.p90_minutes",
	"decision_latency.basis", "decision_latency.basis.ready_flip",
	"decision_latency.basis.decision_request", "decision_latency.basis.created_at",
	"session_hygiene", "session_hygiene.status", "session_hygiene.reason",
	"session_hygiene.sessions_observed", "session_hygiene.sessions_over_24h",
	"session_hygiene.roster_beacons", "session_hygiene.zombie_agents",
	"session_hygiene.claims_filed", "session_hygiene.claims_open",
	"session_hygiene.sessions_active", "session_hygiene.unparseable_files",
	"correction_recurrence", "correction_recurrence.status", "correction_recurrence.reason",
	"correction_recurrence.corrective_messages", "correction_recurrence.recurrence_candidates",
	"dispatch", "dispatch.status", "dispatch.reason", "dispatch.total_dispatches",
	"dispatch.loop_initiated", "dispatch.operator_initiated", "dispatch.unattributed",
	"token_efficiency", "token_efficiency.status", "token_efficiency.reason",
	"token_efficiency.subagent_tokens", "token_efficiency.merged_prs",
	"token_efficiency.noop_dispatches", "token_efficiency.total_dispatches",
	"token_efficiency.reports_unread", "token_efficiency.noop_classifier",
	"unmeasured", "unmeasured[]",
	"trend", "trend.status", "trend.reason", "trend.window_days", "trend.prior_files_found",
	"trend.prior_files_other_classifier", "trend.relay_ratio", "trend.relay_ratio.prior_mean",
	"trend.relay_ratio.delta", "trend.messages_per_merged_pr",
	"trend.messages_per_merged_pr.prior_mean", "trend.messages_per_merged_pr.delta",
}

// unallowlistedKeys returns every key path in artifact that is not in emittedKeys.
func unallowlistedKeys(t *testing.T, artifact []byte) []string {
	t.Helper()
	var doc any
	if err := json.Unmarshal(artifact, &doc); err != nil {
		t.Fatalf("artifact is not valid JSON: %v", err)
	}
	allowed := map[string]bool{}
	for _, k := range emittedKeys {
		allowed[k] = true
	}
	seen := map[string]bool{}
	var walk func(node any, path string)
	walk = func(node any, path string) {
		switch v := node.(type) {
		case map[string]any:
			for k, child := range v {
				p := k
				if path != "" {
					p = path + "." + k
				}
				seen[p] = true
				walk(child, p)
			}
		case []any:
			for _, e := range v {
				seen[path+"[]"] = true
				walk(e, path+"[]")
			}
		}
	}
	walk(doc, "")
	var bad []string
	for p := range seen {
		if !allowed[p] {
			bad = append(bad, p)
		}
	}
	sort.Strings(bad)
	return bad
}

// TestLeakEmittedKeysAreAllowlisted is the KEY instrument: every key in the emitted
// day-file — counts included — is on the pinned list. It is what catches a new block
// arriving unreviewed, and a dynamic key (a session id, a branch) used as a map key.
func TestLeakEmittedKeysAreAllowlisted(t *testing.T) {
	artifact := emitFixtureArtifact(t)
	if bad := unallowlistedKeys(t, artifact); len(bad) > 0 {
		t.Fatalf("the day-file carries keys that are not on the pinned list:\n  %s", strings.Join(bad, "\n  "))
	}
	// Every pinned key is actually emitted by the fixture run (trend included, via
	// --trend), so a stale entry cannot sit on the list unseen.
	var doc any
	_ = json.Unmarshal(artifact, &doc)
	for _, k := range emittedKeys {
		if !keyPresent(doc, strings.Split(k, ".")) {
			t.Errorf("pinned key %q is not in the fixture artifact — a stale pin hides nothing", k)
		}
	}
}

// keyPresent reports whether the dotted path exists in doc. "[]" descends into the
// first array element; an empty array counts as present for "x[]".
func keyPresent(doc any, parts []string) bool {
	if len(parts) == 0 {
		return true
	}
	m, ok := doc.(map[string]any)
	if !ok {
		return false
	}
	head := parts[0]
	if strings.HasSuffix(head, "[]") {
		arr, ok := m[strings.TrimSuffix(head, "[]")].([]any)
		if !ok {
			return false
		}
		if len(parts) == 1 {
			return true
		}
		return len(arr) > 0 && keyPresent(arr[0], parts[1:])
	}
	// Optional ",omitempty" reasons are absent on ok blocks; they are pinned so a
	// could-not-check run passes, and are present-or-omitted by construction.
	child, ok := m[head]
	if !ok {
		return len(parts) == 1 && head == "reason"
	}
	return keyPresent(child, parts[1:])
}

// TestLeakKeyScannerSeesPlantedKey is the key instrument's CONTROL: a key planted
// into the real artifact — one a careless change might add — must be reported.
func TestLeakKeyScannerSeesPlantedKey(t *testing.T) {
	var doc map[string]any
	if err := json.Unmarshal(emitFixtureArtifact(t), &doc); err != nil {
		t.Fatal(err)
	}
	doc["dispatch"].(map[string]any)["by_session"] = map[string]any{"session-gamma": 9}
	planted, _ := json.Marshal(doc)
	bad := unallowlistedKeys(t, planted)
	want := []string{"dispatch.by_session", "dispatch.by_session.session-gamma"}
	if strings.Join(bad, ",") != strings.Join(want, ",") {
		t.Fatalf("planted keys reported as %v, want %v", bad, want)
	}
}

// TestLeakTranscriptsAreNeverModified pins the read-only rule with a measurement
// rather than a promise: the fixture transcript tree is hashed before and after a full
// emit, and any difference fails. The production default points at a human's real
// transcripts, so "we only read" has to be a checked property.
func TestLeakTranscriptsAreNeverModified(t *testing.T) {
	before := hashTree(t, fxTranscripts)
	beforeDT := hashTree(t, fxDeskTools)
	emitFixtureArtifact(t)
	if after := hashTree(t, fxTranscripts); after != before {
		t.Fatal("the transcript tree changed during an emit — this tool must be strictly read-only against transcripts")
	}
	if after := hashTree(t, fxDeskTools); after != beforeDT {
		t.Fatal("the desk-tools state tree changed during an emit — the collector must not write to the state it measures")
	}
}

func hashTree(t *testing.T, root string) string {
	t.Helper()
	h := sha256.New()
	var paths []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("hashing %s: %v", root, err)
	}
	sort.Strings(paths)
	for _, p := range paths {
		raw, rerr := os.ReadFile(p)
		if rerr != nil {
			t.Fatalf("hashing %s: %v", p, rerr)
		}
		h.Write([]byte(p))
		h.Write(raw)
	}
	return hex.EncodeToString(h.Sum(nil))
}

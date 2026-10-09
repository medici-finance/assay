package main

// runtimeresource_test.go — the runtime adapter contract (sdlc/24,
// docs/runtime-adapters.md). Every test here is named TestRuntimeResourceContract*
// so the brief's single `-run TestRuntimeResourceContract` selector runs all of
// them, and a rename that orphans one is visible as a missing --- PASS line.

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

const (
	rrRecordsDir   = "testdata/runtime-resource/records"
	rrAdaptersFile = "testdata/runtime-resource/adapters.yaml"
	rrFixtureAsOf  = "2026-01-01T02:00:00Z"
)

func rrAsOf(t *testing.T) time.Time {
	t.Helper()
	asOf, err := time.Parse(time.RFC3339, rrFixtureAsOf)
	if err != nil {
		t.Fatal(err)
	}
	return asOf
}

func rrSchemaAndAdapters(t *testing.T) (*schemaNode, map[string]runtimeAdapter) {
	t.Helper()
	schema, err := parseSchema(embeddedRuntimeResourceSchemaBytes())
	if err != nil {
		t.Fatalf("embedded runtime-resource schema does not parse: %v", err)
	}
	adapters, err := loadRuntimeAdapters(rrAdaptersFile)
	if err != nil {
		t.Fatalf("fixture adapter registry does not load: %v", err)
	}
	return schema, adapters
}

// TestRuntimeResourceContract runs every committed fixture through the validator
// at a fixed instant. A good fixture must be checked-clean with no violation at
// all; a bad fixture must be checked-failed AND name the rule its case exists for
// — a bad fixture that fails for some other reason proves nothing about its rule.
func TestRuntimeResourceContract(t *testing.T) {
	schema, adapters := rrSchemaAndAdapters(t)
	asOf := rrAsOf(t)

	cases := []struct {
		file  string
		want  patternState
		rules []string // rule tags that MUST appear (bad fixtures)
		names []string // further substrings that MUST appear
	}{
		{file: "good-valid.yaml", want: patternStateClean},
		{file: "good-lost-ack.yaml", want: patternStateClean},
		{file: "good-claimed-released.yaml", want: patternStateClean},
		{file: "bad-missing-ttl.yaml", want: patternStateFailed, rules: []string{ruleRRSchemaViolation}, names: []string{`missing required key "ttl"`}},
		{file: "bad-expired.yaml", want: patternStateFailed, rules: []string{ruleRRExpired}},
		{file: "bad-unknown-adapter.yaml", want: patternStateFailed, rules: []string{ruleRRUnknownAdapter}, names: []string{"vendor-unregistered"}},
		{file: "bad-claim-no-principal.yaml", want: patternStateFailed, rules: []string{ruleRRClaimWithoutPrincipal}, names: []string{"agent:worker-2"}},
		{file: "bad-release-unreconciled.yaml", want: patternStateFailed, rules: []string{ruleRRUnreconciledRelease}},
		{file: "bad-lost-ack-unreconciled.yaml", want: patternStateFailed, rules: []string{ruleRRUnreconciledEffect}, names: []string{"must not be inferred from the absence"}},
	}

	// Every committed record fixture is in the table: a fixture added without a
	// case would sit in the directory unasserted.
	entries, err := os.ReadDir(rrRecordsDir)
	if err != nil {
		t.Fatal(err)
	}
	var onDisk, inTable []string
	for _, e := range entries {
		onDisk = append(onDisk, e.Name())
	}
	for _, c := range cases {
		inTable = append(inTable, c.file)
	}
	sort.Strings(onDisk)
	sort.Strings(inTable)
	if !reflect.DeepEqual(onDisk, inTable) {
		t.Fatalf("fixture directory %v and case table %v disagree", onDisk, inTable)
	}

	for _, c := range cases {
		t.Run(strings.TrimSuffix(c.file, ".yaml"), func(t *testing.T) {
			state, lines := lintRuntimeResourceFile(filepath.Join(rrRecordsDir, c.file), schema, adapters, asOf)
			if state != c.want {
				t.Fatalf("state = %v, want %v; lines:\n%s", state, c.want, strings.Join(lines, "\n"))
			}
			if c.want == patternStateClean && len(lines) != 0 {
				t.Fatalf("clean fixture produced lines:\n%s", strings.Join(lines, "\n"))
			}
			for _, want := range append(append([]string{}, c.rules...), c.names...) {
				if !anyContains(lines, want) {
					t.Errorf("violations do not name %q:\n%s", want, strings.Join(lines, "\n"))
				}
			}
		})
	}
}

// TestRuntimeResourceContractRules covers each remaining rule with a one-edit
// variant of a good fixture, so every rule tag has a case that fires it alone.
func TestRuntimeResourceContractRules(t *testing.T) {
	schema, adapters := rrSchemaAndAdapters(t)
	asOf := rrAsOf(t)
	read := func(name string) string {
		b, err := os.ReadFile(filepath.Join(rrRecordsDir, name))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	valid := read("good-valid.yaml")
	lostAck := read("good-lost-ack.yaml")
	claimed := read("good-claimed-released.yaml")

	cases := []struct {
		name, base, old, new, rule string
	}{
		{"class not registered for adapter", valid, "id: local-mock", "id: local-readonly-corpus", ruleRRUnknownAdapter},
		{"verb not implemented by adapter", strings.NewReplacer("class: database-branch", "class: eval-corpus", "id: local-mock", "id: local-readonly-corpus").Replace(valid), "verb: inspect", "verb: snapshot", ruleRRUnknownAdapter},
		{"ttl above adapter max-ttl", valid, "ttl: 4h", "ttl: 2d", ruleRRTTLExceedsBound},
		{"agent credential outlives ttl", valid, "lifetime: 1h", "lifetime: 5h", ruleRRCredentialOutlivesTTL},
		{"succeeded effect without receipt", valid, "    receipt:\n      adapter-ref: mock://branch/rr-valid-01\n      observed-at: \"2026-01-01T01:00:01Z\"\n      observed-state: present\n", "", ruleRRReceiptMissing},
		{"lost-ack reconcile receipt removed", lostAck, "    receipt:\n      adapter-ref: mock://branch/rr-lost-ack-01\n      observed-at: \"2026-01-01T00:02:01Z\"\n      observed-state: present\n", "", ruleRRUnreconciledEffect},
		{"lost-ack reconcile observed absent but state provisioned", lostAck, "      observed-state: present", "      observed-state: absent", ruleRRStateUnsupported},
		{"reconcile names a later effect", lostAck, "reconciles: eff-0001", "reconciles: eff-0009", ruleRRUnreconciledEffect},
		{"claim without credential rotation", claimed, "    - after: eff-0002\n      action: rotated\n      at: \"2026-01-01T00:30:01Z\"\n", "", ruleRRCredentialNotRotated},
		{"release rotated instead of revoked", claimed, "action: revoked", "action: rotated", ruleRRCredentialNotRotated},
		{"claimed record with no ownership principal", claimed, "state: released", "state: claimed", ruleRRStateUnsupported},
		{"provisioned recorded for a released resource", claimed, "state: released", "state: provisioned", ruleRRStateUnsupported},
		{"claimed retention elapsed", strings.Replace(strings.Split(claimed, "  - id: eff-0003\n")[0], "state: released", "state: claimed", 1), "retain-until: \"2026-01-08T00:00:00Z\"", "retain-until: \"2026-01-01T01:00:00Z\"", ruleRRExpired},
		{"expired recorded inside the ttl", valid, "state: provisioned", "state: expired", ruleRRStateUnsupported},
		{"ownership principal is an agent", claimed, "  principal: human:reviewer-a\n  claimed-at", "  principal: agent:worker-2\n  claimed-at", ruleRRSchemaViolation},
		{"agent named as cleanup owner", valid, "cleanup-owner: control-plane", "cleanup-owner: agent:worker-1", ruleRRSchemaViolation},
		{"agent bypasses the control plane", valid, "via: control-plane", "via: direct", ruleRRSchemaViolation},
		{"durable credential handed to the agent", valid, "durable-custody: control-plane", "durable-custody: agent", ruleRRSchemaViolation},
		{"unbounded resource (no quota)", valid, "quota:\n  - dimension: storage-mb\n    limit: 512\n  - dimension: compute-minutes\n    limit: 60\n", "quota: []\n", ruleRRSchemaViolation},
	}
	dir := t.TempDir()
	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if !strings.Contains(c.base, c.old) {
				t.Fatalf("anchor %q not found in base fixture — the case no longer edits anything", c.old)
			}
			path := filepath.Join(dir, "case-"+string(rune('a'+i))+".yaml")
			if err := os.WriteFile(path, []byte(strings.Replace(c.base, c.old, c.new, 1)), 0o644); err != nil {
				t.Fatal(err)
			}
			state, lines := lintRuntimeResourceFile(path, schema, adapters, asOf)
			if state != patternStateFailed {
				t.Fatalf("state = %v, want checked-failed; lines:\n%s", state, strings.Join(lines, "\n"))
			}
			if !anyContains(lines, "["+c.rule+"]") {
				t.Errorf("violations do not name %q:\n%s", c.rule, strings.Join(lines, "\n"))
			}
		})
	}
}

// TestRuntimeResourceContractCLI drives the subcommand end to end: the
// three-state exit, and fail-closed refusal when the registry cannot be trusted.
func TestRuntimeResourceContractCLI(t *testing.T) {
	run := func(args ...string) (int, string) {
		var out, errb bytes.Buffer
		code := runRuntimeResource(args, &out, &errb)
		return code, out.String() + errb.String()
	}

	code, out := run("--lint", "--adapters", rrAdaptersFile, "--as-of", rrFixtureAsOf,
		filepath.Join(rrRecordsDir, "good-valid.yaml"), filepath.Join(rrRecordsDir, "good-lost-ack.yaml"), filepath.Join(rrRecordsDir, "good-claimed-released.yaml"))
	if code != runtimeResourceExitClean || !strings.Contains(out, "3 checked-clean, 0 checked-failed, 0 could-not-check") {
		t.Errorf("good fixtures: exit %d, want 0; output:\n%s", code, out)
	}

	code, out = run("--lint", "--adapters", rrAdaptersFile, "--as-of", rrFixtureAsOf, rrRecordsDir)
	if code != runtimeResourceExitFailed || !strings.Contains(out, "3 checked-clean, 6 checked-failed, 0 could-not-check (9 file(s) scanned") {
		t.Errorf("fixture directory: exit %d, want 1 with 3 clean / 6 failed; output:\n%s", code, out)
	}

	dir := t.TempDir()
	badRegistry := filepath.Join(dir, "adapters.yaml")
	if err := os.WriteFile(badRegistry, []byte("schema: runtime-adapters-v1\nadapters:\n  - id: half-adapter\n    classes: [snapshot]\n    verbs: [create, inspect]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	emptyDir := filepath.Join(dir, "empty")
	if err := os.Mkdir(emptyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, args := range map[string][]string{
		"registry missing a required verb": {"--lint", "--adapters", badRegistry, rrRecordsDir},
		"registry file absent":             {"--lint", "--adapters", filepath.Join(dir, "nope.yaml"), rrRecordsDir},
		"record path absent":               {"--lint", "--adapters", rrAdaptersFile, filepath.Join(dir, "nope")},
		"directory with no records":        {"--lint", "--adapters", rrAdaptersFile, emptyDir},
	} {
		code, out := run(args...)
		if code != runtimeResourceExitCouldNot || !strings.Contains(out, "could-not-check") {
			t.Errorf("%s: exit %d, want could-not-check (2); output:\n%s", name, code, out)
		}
	}

	for name, args := range map[string][]string{
		"no verb":       {"--adapters", rrAdaptersFile, rrRecordsDir},
		"no registry":   {"--lint", rrRecordsDir},
		"no paths":      {"--lint", "--adapters", rrAdaptersFile},
		"bad --as-of":   {"--lint", "--adapters", rrAdaptersFile, "--as-of", "yesterday", rrRecordsDir},
		"unknown flags": {"--lint", "--adapters", rrAdaptersFile, "--fix", rrRecordsDir},
	} {
		if code, out := run(args...); code != runtimeResourceExitUsageError {
			t.Errorf("%s: exit %d, want usage refusal (2); output:\n%s", name, code, out)
		}
	}
}

// TestRuntimeResourceContractSchemaParity pins the embedded schema byte-identical
// to the canonical repo-root file, and the validator able to enforce every
// keyword the schema uses.
func TestRuntimeResourceContractSchemaParity(t *testing.T) {
	canonical, err := os.ReadFile(filepath.Join("..", "schemas", "runtime-resource-v1.json"))
	if err != nil {
		t.Fatalf("read canonical schema: %v", err)
	}
	if !bytes.Equal(canonical, embeddedRuntimeResourceSchemaBytes()) {
		t.Fatal("statusgen/schemas/runtime-resource-v1.json differs from schemas/runtime-resource-v1.json — copy the canonical file over the embedded one")
	}
	schema, err := parseSchema(canonical)
	if err != nil {
		t.Fatal(err)
	}
	assertValidatorCoversKeywords(t, schema)
}

// TestRuntimeResourceContractVocabulary pins the Go-side closed vocabularies to
// the schema's enums, so a class or verb added in one place cannot be missing
// from the other.
func TestRuntimeResourceContractVocabulary(t *testing.T) {
	var s struct {
		Properties struct {
			Class struct {
				Enum []string `json:"enum"`
			} `json:"class"`
			State struct {
				Enum []string `json:"enum"`
			} `json:"state"`
			Effects struct {
				Items struct {
					Properties struct {
						Verb struct {
							Enum []string `json:"enum"`
						} `json:"verb"`
					} `json:"properties"`
				} `json:"items"`
			} `json:"effects"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(embeddedRuntimeResourceSchemaBytes(), &s); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(s.Properties.Class.Enum, runtimeResourceClasses) {
		t.Errorf("schema class enum %v != runtimeResourceClasses %v", s.Properties.Class.Enum, runtimeResourceClasses)
	}
	if !reflect.DeepEqual(s.Properties.Effects.Items.Properties.Verb.Enum, runtimeResourceVerbs) {
		t.Errorf("schema verb enum %v != runtimeResourceVerbs %v", s.Properties.Effects.Items.Properties.Verb.Enum, runtimeResourceVerbs)
	}
	wantStates := []string{"requested", "provisioned", "claimed", "expired", "released", "failed"}
	if !reflect.DeepEqual(s.Properties.State.Enum, wantStates) {
		t.Errorf("schema state enum %v != documented states %v", s.Properties.State.Enum, wantStates)
	}
}

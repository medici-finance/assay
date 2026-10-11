package main

// runtimeresource_test.go — the runtime adapter contract (sdlc/24,
// docs/runtime-adapters.md). Every test here is named TestRuntimeResourceContract*
// so the brief's single `-run TestRuntimeResourceContract` selector runs all of
// them, and a rename that orphans one is visible as a missing --- PASS line.

import (
	"bytes"
	"encoding/json"
	"fmt"
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
		{file: "good-requested.yaml", want: patternStateClean},
		{file: "good-failed.yaml", want: patternStateClean},
		{file: "bad-missing-ttl.yaml", want: patternStateFailed, rules: []string{ruleRRSchemaViolation}, names: []string{`missing required key "ttl"`}},
		{file: "bad-expired.yaml", want: patternStateFailed, rules: []string{ruleRRExpired}},
		{file: "bad-unknown-adapter.yaml", want: patternStateFailed, rules: []string{ruleRRUnknownAdapter}, names: []string{"vendor-unregistered"}},
		{file: "bad-claim-no-principal.yaml", want: patternStateFailed, rules: []string{ruleRRClaimWithoutPrincipal}, names: []string{"agent:worker-2"}},
		{file: "bad-release-unreconciled.yaml", want: patternStateFailed, rules: []string{ruleRRUnreconciledRelease}},
		{file: "bad-lost-ack-unreconciled.yaml", want: patternStateFailed, rules: []string{ruleRRUnreconciledEffect}, names: []string{"must not be inferred from the absence"}},
		// regression: a requested record whose create already failed is failed.
		{file: "bad-requested-after-failed-create.yaml", want: patternStateFailed, rules: []string{ruleRRStateUnsupported}, names: []string{"that record is failed"}},
		// regression: the latest authoritative observation wins, in both directions.
		{file: "bad-provisioned-observed-absent.yaml", want: patternStateFailed, rules: []string{ruleRRStateUnsupported}, names: []string{"latest authoritative receipt observed the resource absent"}},
		{file: "bad-released-recreated.yaml", want: patternStateFailed, rules: []string{ruleRRStateUnsupported}, names: []string{"observed the resource present again"}},
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

// rrFixture reads one committed record fixture.
func rrFixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(rrRecordsDir, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// rrLintText writes a record body to a temp file and lints it.
func rrLintText(t *testing.T, dir, name, body string) (patternState, []string) {
	t.Helper()
	schema, adapters := rrSchemaAndAdapters(t)
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return lintRuntimeResourceFile(path, schema, adapters, rrAsOf(t))
}

// TestRuntimeResourceContractRules covers every guard with a one-edit variant of
// a good fixture. Each case names the rule AND the message only that guard
// emits: several guards share a rule tag, so a tag alone would let one guard's
// line stand in for another's deletion.
func TestRuntimeResourceContractRules(t *testing.T) {
	valid := rrFixture(t, "good-valid.yaml")
	lostAck := rrFixture(t, "good-lost-ack.yaml")
	claimed := rrFixture(t, "good-claimed-released.yaml")
	failedRec := rrFixture(t, "good-failed.yaml")
	requested := rrFixture(t, "good-requested.yaml")
	// claimedLive is the claimed fixture before its release: a live claim.
	claimedLive := strings.Replace(strings.Split(claimed, "  - id: eff-0003\n")[0], "state: released", "state: claimed", 1)
	ownershipBlock := "ownership:\n  principal: human:reviewer-a\n  claimed-at: \"2026-01-01T00:30:00Z\"\n  retain-until: \"2026-01-08T00:00:00Z\"\n"
	releaseByOwner := "requested-at: \"2026-01-01T01:30:00Z\"\n    outcome: succeeded\n    principal: human:reviewer-a\n"
	inspectAbsent := "  - id: eff-0009\n    verb: inspect\n    idempotency-key: run-0003/inspect-9\n    requested-at: \"2026-01-01T01:00:00Z\"\n    outcome: succeeded\n    principal: agent:worker-2\n    receipt:\n      adapter-ref: mock://snapshot/rr-claimed-01\n      observed-at: \"2026-01-01T01:00:01Z\"\n      observed-state: absent\n"
	issuedWhilePending := "  - id: eff-0003\n    verb: inspect\n    idempotency-key: run-0002/inspect\n    requested-at: \"2026-01-01T00:01:00Z\"\n    outcome: failed\n    principal: agent:worker-1\n  - id: eff-0002\n    verb: reconcile"

	cases := []struct {
		name, base, old, new, rule, msg string
	}{
		{"class not registered for adapter", valid, "id: local-mock", "id: local-readonly-corpus", ruleRRUnknownAdapter, `is not registered for class "database-branch"`},
		{"verb not implemented by adapter", strings.NewReplacer("class: database-branch", "class: eval-corpus", "id: local-mock", "id: local-readonly-corpus").Replace(valid), "verb: inspect", "verb: snapshot", ruleRRUnknownAdapter, `effect "eff-0002": adapter "local-readonly-corpus" does not implement verb "snapshot"`},
		{"ttl above adapter max-ttl", valid, "ttl: 4h", "ttl: 2d", ruleRRTTLExceedsBound, "exceeds adapter"},
		{"agent credential outlives ttl", valid, "lifetime: 1h", "lifetime: 5h", ruleRRCredentialOutlivesTTL, "agent credential lifetime 5h0m0s exceeds"},
		{"succeeded effect without receipt", valid, "    receipt:\n      adapter-ref: mock://branch/rr-valid-01\n      observed-at: \"2026-01-01T01:00:01Z\"\n      observed-state: present\n", "", ruleRRReceiptMissing, `effect "eff-0002" ("inspect") is recorded succeeded with no receipt`},
		{"lost-ack reconcile receipt removed", lostAck, "    receipt:\n      adapter-ref: mock://branch/rr-lost-ack-01\n      observed-at: \"2026-01-01T00:02:01Z\"\n      observed-state: present\n", "", ruleRRUnreconciledEffect, "no later reconcile with a receipt resolves it"},
		{"lost-ack reconciled absent, provisioned", lostAck, "      observed-state: present", "      observed-state: absent", ruleRRStateUnsupported, "no create receipt (or reconcile receipt) observed the resource present"},
		{"reconcile names a later effect", lostAck, "reconciles: eff-0001", "reconciles: eff-0009", ruleRRUnreconciledEffect, `reconciles "eff-0009", which is not an earlier effect`},
		// regression: deleting the names-nothing guard left the suite green.
		{"reconcile names no effect", lostAck, "    reconciles: eff-0001\n", "", ruleRRUnreconciledEffect, "names no effect it reconciles"},
		// regression: deleting the issued-while-pending guard left the suite green.
		{"effect issued while one is pending", lostAck, "  - id: eff-0002\n    verb: reconcile", issuedWhilePending, ruleRRUnreconciledEffect, `effect "eff-0003" ("inspect") was issued while effect "eff-0001" ("create") had outcome unknown`},
		// regression: deleting the duplicate-id guard left the suite green.
		{"duplicate effect id", valid, "  - id: eff-0002\n", "  - id: eff-0001\n", ruleRRSchemaViolation, `effect id "eff-0001" appears more than once`},
		{"claim without credential rotation", claimed, "    - after: eff-0002\n      action: rotated\n      at: \"2026-01-01T00:30:01Z\"\n", "", ruleRRCredentialNotRotated, `claim effect "eff-0002" is not followed by a recorded rotation`},
		{"release rotated instead of revoked", claimed, "action: revoked", "action: rotated", ruleRRCredentialNotRotated, `release effect "eff-0003" is not followed by a recorded revocation`},
		{"claimed recorded after the release", claimed, "state: released", "state: claimed", ruleRRStateUnsupported, "state is claimed but a release was reconciled to absent"},
		{"provisioned recorded after release", claimed, "state: released", "state: provisioned", ruleRRStateUnsupported, "state is provisioned but a release was reconciled to absent"},
		{"expired recorded after release", claimed, "state: released", "state: expired", ruleRRStateUnsupported, "that record is released"},
		{"claimed retention elapsed", claimedLive, "retain-until: \"2026-01-08T00:00:00Z\"", "retain-until: \"2026-01-01T01:00:00Z\"", ruleRRExpired, "claimed retention ended at"},
		{"expired recorded inside the ttl", valid, "state: provisioned", "state: expired", ruleRRStateUnsupported, "neither the ttl nor a claimed retention has elapsed"},
		// regression: deleting each ownership guard left the suite green.
		{"claimed with no ownership block", claimedLive, ownershipBlock, "", ruleRRClaimWithoutPrincipal, "no ownership block names the principal"},
		{"ownership no claim backs", claimed, "  principal: human:reviewer-a\n  claimed-at", "  principal: human:reviewer-b\n  claimed-at", ruleRRClaimWithoutPrincipal, `ownership names "human:reviewer-b" but no succeeded claim`},
		// regression: deleting each state leg left the suite green.
		{"requested with a present receipt", valid, "state: provisioned", "state: requested", ruleRRStateUnsupported, "state is requested but a receipt shows the resource present"},
		{"claimed with no claim", valid, "state: provisioned", "state: claimed", ruleRRStateUnsupported, "no established resource with a succeeded claim"},
		{"claimed but observed absent since", claimedLive + inspectAbsent, "state: claimed", "state: claimed", ruleRRStateUnsupported, "state is claimed but the latest authoritative receipt observed the resource absent"},
		{"expired never present", failedRec, "state: failed", "state: expired", ruleRRStateUnsupported, "no receipt shows the resource was ever present"},
		{"failed with a present receipt", valid, "state: provisioned", "state: failed", ruleRRStateUnsupported, "state is failed but a receipt shows the resource present"},
		{"failed with no failed create", requested, "state: requested", "state: failed", ruleRRStateUnsupported, "no create effect failed or was reconciled absent"},
		{"released with no release", valid, "state: provisioned", "state: released", ruleRRUnreconciledRelease, "no release effect was reconciled to absent"},
		// regression: a bound the rules cannot read switched its rule off.
		{"ttl too long to evaluate", valid, "ttl: 4h", "ttl: 99999999999999999999h", ruleRRSchemaViolation, `ttl "99999999999999999999h" is not a duration the rules can evaluate`},
		{"ttl that would wrap", valid, "ttl: 4h", "ttl: 9999999999d", ruleRRSchemaViolation, `ttl "9999999999d" is not a duration the rules can evaluate`},
		{"lifetime too long to evaluate", valid, "lifetime: 1h", "lifetime: 123456h", ruleRRSchemaViolation, `lifetime "123456h" is not a duration the rules can evaluate`},
		{"retain-until in month 13", claimedLive, "retain-until: \"2026-01-08T00:00:00Z\"", "retain-until: \"2026-13-08T00:00:00Z\"", ruleRRSchemaViolation, `retain-until "2026-13-08T00:00:00Z" is not a real RFC3339 instant`},
		{"observed-at on Feb 30", valid, "observed-at: \"2026-01-01T00:00:05Z\"", "observed-at: \"2026-02-30T00:00:05Z\"", ruleRRSchemaViolation, `observed-at "2026-02-30T00:00:05Z" is not a real RFC3339 instant`},
		{"requested-at at hour 25", valid, "requested-at: \"2026-01-01T01:00:00Z\"", "requested-at: \"2026-01-01T25:00:00Z\"", ruleRRSchemaViolation, `requested-at "2026-01-01T25:00:00Z" is not a real RFC3339 instant`},
		// regression: a receipt dated outside its effect's window moved the TTL anchor.
		{"receipt dated after as-of", valid, "observed-at: \"2026-01-01T01:00:01Z\"", "observed-at: \"2026-01-01T03:00:01Z\"", ruleRRStateUnsupported, "a receipt cannot observe the future"},
		{"receipt dated before request", valid, "observed-at: \"2026-01-01T01:00:01Z\"", "observed-at: \"2026-01-01T00:59:00Z\"", ruleRRStateUnsupported, "before the effect was requested at"},
		// regression: an agent could end a resource a human had claimed.
		{"agent releases a claimed resource", claimed, releaseByOwner, strings.Replace(releaseByOwner, "human:reviewer-a", "agent:worker-2", 1), ruleRRClaimWithoutPrincipal, `release effect "eff-0003" ends a claimed resource but is authorized by "agent:worker-2"`},
		{"unattributed release of a claim", claimed, releaseByOwner, strings.Replace(releaseByOwner, "    principal: human:reviewer-a\n", "", 1), ruleRRClaimWithoutPrincipal, "ends a claimed resource but is authorized by no principal"},
		{"ownership principal is an agent", claimed, "  principal: human:reviewer-a\n  claimed-at", "  principal: agent:worker-2\n  claimed-at", ruleRRSchemaViolation, "ownership.principal"},
		{"agent named as cleanup owner", valid, "cleanup-owner: control-plane", "cleanup-owner: agent:worker-1", ruleRRSchemaViolation, "cleanup-owner"},
		{"agent bypasses the control plane", valid, "via: control-plane", "via: direct", ruleRRSchemaViolation, "provisioning.via"},
		{"durable credential handed to agent", valid, "durable-custody: control-plane", "durable-custody: agent", ruleRRSchemaViolation, "durable-custody"},
		{"unbounded resource (no quota)", valid, "quota:\n  - dimension: storage-mb\n    limit: 512\n  - dimension: compute-minutes\n    limit: 60\n", "quota: []\n", ruleRRSchemaViolation, "quota"},
	}
	dir := t.TempDir()
	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if !strings.Contains(c.base, c.old) {
				t.Fatalf("anchor %q not found in base fixture — the case no longer edits anything", c.old)
			}
			state, lines := rrLintText(t, dir, fmt.Sprintf("case-%02d.yaml", i), strings.Replace(c.base, c.old, c.new, 1))
			if state != patternStateFailed {
				t.Fatalf("state = %v, want checked-failed; lines:\n%s", state, strings.Join(lines, "\n"))
			}
			if !anyContains(lines, "["+c.rule+"]") {
				t.Errorf("violations do not name %q:\n%s", c.rule, strings.Join(lines, "\n"))
			}
			if !rrLineHas(lines, c.rule, c.msg) {
				t.Errorf("no [%s] line carries %q:\n%s", c.rule, c.msg, strings.Join(lines, "\n"))
			}
		})
	}
}

// rrLineHas reports whether one line carries both the rule tag and the message.
func rrLineHas(lines []string, rule, msg string) bool {
	for _, l := range lines {
		if strings.Contains(l, "["+rule+"]") && strings.Contains(l, msg) {
			return true
		}
	}
	return false
}

// TestRuntimeResourceContractOrd pins the ordered reading of the receipts from
// the clean side: the state the latest authoritative observation supports is
// accepted. A resource created again after a reconciled release is live, with a
// fresh lifetime and no claim carried over; a lost create reconciled absent is
// failed.
func TestRuntimeResourceContractOrd(t *testing.T) {
	recreated := rrFixture(t, "bad-released-recreated.yaml")
	lostAck := rrFixture(t, "good-lost-ack.yaml")
	cases := []struct{ name, base, old, new string }{
		{"re-created after release", recreated, "state: released", "state: provisioned"},
		{"lost create reconciled absent", strings.Replace(lostAck, "      observed-state: present", "      observed-state: absent", 1), "state: provisioned", "state: failed"},
	}
	dir := t.TempDir()
	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if !strings.Contains(c.base, c.old) {
				t.Fatalf("anchor %q not found", c.old)
			}
			state, lines := rrLintText(t, dir, fmt.Sprintf("ord-%02d.yaml", i), strings.Replace(c.base, c.old, c.new, 1))
			if state != patternStateClean {
				t.Fatalf("state = %v, want checked-clean; lines:\n%s", state, strings.Join(lines, "\n"))
			}
		})
	}
}

// regression: deleting the per-effect release check, or reading a release's own
// receipt as an observation, left the suite green — the released-state check
// emitted the same tag, and no case pinned the exact line set.
//
// TestRuntimeResourceContractAck pins an acknowledged, unreconciled release on a
// record that still says provisioned: the release-level guard is the only one
// that may fire, and the release receipt's "absent" must not end the resource.
func TestRuntimeResourceContractAck(t *testing.T) {
	valid := rrFixture(t, "good-valid.yaml")
	release := "  - id: eff-0003\n    verb: release\n    idempotency-key: run-0001/release\n    requested-at: \"2026-01-01T01:30:00Z\"\n    outcome: succeeded\n    principal: agent:worker-1\n    receipt:\n      adapter-ref: mock://branch/rr-valid-01\n      observed-at: \"2026-01-01T01:30:01Z\"\n      observed-state: absent\n"
	revoked := "rotations:\n    - after: eff-0003\n      action: revoked\n      at: \"2026-01-01T01:30:02Z\"\n"
	if !strings.Contains(valid, "rotations: []\n") {
		t.Fatal("anchor rotations: [] not found")
	}
	body := strings.Replace(valid, "rotations: []\n", revoked, 1) + release
	state, lines := rrLintText(t, t.TempDir(), "ack.yaml", body)
	if state != patternStateFailed {
		t.Fatalf("state = %v, want checked-failed; lines:\n%s", state, strings.Join(lines, "\n"))
	}
	if len(lines) != 1 || !rrLineHas(lines, ruleRRUnreconciledRelease, `release effect "eff-0003" succeeded but no later reconcile observed the resource absent`) {
		t.Fatalf("want exactly the release-level unreconciled-release line; got:\n%s", strings.Join(lines, "\n"))
	}
}

// TestRuntimeResourceContractReg covers every registry refusal. The registry is
// the trust anchor, so each defect is could-not-check for the whole run, and
// each case names the message only its guard emits.
func TestRuntimeResourceContractReg(t *testing.T) {
	b, err := os.ReadFile(rrAdaptersFile)
	if err != nil {
		t.Fatal(err)
	}
	base := string(b)
	edit := func(old, new string) string {
		if !strings.Contains(base, old) {
			t.Fatalf("anchor %q not found in the fixture registry", old)
		}
		return strings.Replace(base, old, new, 1)
	}
	cases := []struct{ name, body, msg string }{
		// regression: a misspelt key silently removed the ceiling.
		{"misspelt max_ttl", edit("    max-ttl: 24h", "    max_ttl: 24h"), `adapters[0]: unknown key(s) "max_ttl"`},
		{"unknown top-level key", "owner: someone\n" + base, `adapter registry: unknown key(s) "owner"`},
		{"second YAML document", base + "---\nschema: runtime-adapters-v1\n", "more than one YAML document"},
		{"empty file", "", "empty adapter registry"},
		{"not a mapping", "- local-mock\n", "adapter registry is not a mapping"},
		{"wrong schema marker", edit("schema: runtime-adapters-v1", "schema: runtime-adapters-v2"), `not "runtime-adapters-v1"`},
		{"no adapters", "schema: runtime-adapters-v1\nadapters: []\n", "lists no adapters"},
		{"adapter not a mapping", "schema: runtime-adapters-v1\nadapters:\n  - local-mock\n", "adapters[0]: not a mapping"},
		{"malformed adapter id", edit("  - id: local-mock", "  - id: Local_Mock"), `id "Local_Mock" does not match`},
		{"adapter registered twice", edit("  - id: local-readonly-corpus", "  - id: local-mock"), `id "local-mock" is registered twice`},
		{"unknown class", edit("classes: [eval-corpus]", "classes: [eval-corpus, tape-archive]"), `unknown class "tape-archive"`},
		{"no classes", edit("classes: [eval-corpus]", "classes: []"), `adapter "local-readonly-corpus": lists no classes`},
		{"unknown verb", edit("verbs: [create, inspect, release, reconcile]", "verbs: [create, inspect, release, reconcile, teleport]"), `unknown verb "teleport"`},
		{"required verb missing", edit("verbs: [create, inspect, release, reconcile]", "verbs: [create, inspect, release]"), `required verb "reconcile"`},
		{"max-ttl not a duration", edit("max-ttl: 24h", "max-ttl: 24 hours"), `max-ttl "24 hours" is not a duration`},
		// regression: a max-ttl that overflowed wrapped instead of refusing.
		{"max-ttl that would wrap", edit("max-ttl: 24h", "max-ttl: 9999999999d"), `max-ttl "9999999999d" is not a duration`},
	}
	dir := t.TempDir()
	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path := filepath.Join(dir, fmt.Sprintf("adapters-%02d.yaml", i))
			if err := os.WriteFile(path, []byte(c.body), 0o644); err != nil {
				t.Fatal(err)
			}
			_, err := loadRuntimeAdapters(path)
			if err == nil {
				t.Fatalf("registry loaded; want a refusal carrying %q", c.msg)
			}
			if !strings.Contains(err.Error(), c.msg) {
				t.Errorf("refusal %q does not carry %q", err, c.msg)
			}
		})
	}
}

// TestRuntimeResourceContractFile covers the record-file refusals: each is
// could-not-check, never clean.
func TestRuntimeResourceContractFile(t *testing.T) {
	valid := rrFixture(t, "good-valid.yaml")
	cases := []struct{ name, body, msg string }{
		{"wrong schema marker", strings.Replace(valid, "schema: runtime-resource-v1", "schema: runtime-resource-v2", 1), `not "runtime-resource-v1"`},
		// regression: only the first document of a file was decoded.
		{"second YAML document", valid + "---\n" + valid, "more than one YAML document"},
		{"empty file", "", "empty document"},
		{"not a mapping", "- rr-valid-01\n", "the document is not a mapping"},
	}
	dir := t.TempDir()
	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			state, lines := rrLintText(t, dir, fmt.Sprintf("rec-%02d.yaml", i), c.body)
			if state != patternStateCouldNot {
				t.Fatalf("state = %v, want could-not-check; lines:\n%s", state, strings.Join(lines, "\n"))
			}
			if !anyContains(lines, c.msg) {
				t.Errorf("lines do not carry %q:\n%s", c.msg, strings.Join(lines, "\n"))
			}
		})
	}
}

// TestRuntimeResourceContractTag pins every runtime-resource rule tag into the
// enforcement-status registry, so the authoring guidance's generated block
// cannot read one of them as unregistered.
func TestRuntimeResourceContractTag(t *testing.T) {
	registered := map[string]EnforcementStatus{}
	for _, r := range lintRuleRegistry {
		registered[r.Tag] = r.Status
	}
	for _, tag := range []string{
		ruleRRSchemaViolation, ruleRRUnknownAdapter, ruleRRTTLExceedsBound, ruleRRExpired,
		ruleRRClaimWithoutPrincipal, ruleRRUnreconciledRelease, ruleRRUnreconciledEffect,
		ruleRRReceiptMissing, ruleRRStateUnsupported, ruleRRCredentialNotRotated, ruleRRCredentialOutlivesTTL,
	} {
		if st, ok := registered[tag]; !ok {
			t.Errorf("rule %q is not in lintRuleRegistry", tag)
		} else if st != StatusFatal {
			t.Errorf("rule %q is registered %q; every runtime-resource rule makes the subcommand exit 1, so it is fatal", tag, st)
		}
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
	if code != runtimeResourceExitFailed || !strings.Contains(out, "5 checked-clean, 9 checked-failed, 0 could-not-check (14 file(s) scanned") {
		t.Errorf("fixture directory: exit %d, want 1 with 5 clean / 9 failed; output:\n%s", code, out)
	}

	dir := t.TempDir()

	// regression: a directory scan skipped .yml files and subdirectories, so a
	// failing record there was never read and the run reported clean.
	scanDir := filepath.Join(dir, "scan")
	if err := os.MkdirAll(filepath.Join(scanDir, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(scanDir, "nested", "expired.yml"), []byte(rrFixture(t, "bad-expired.yaml")), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out = run("--lint", "--adapters", rrAdaptersFile, "--as-of", rrFixtureAsOf, scanDir)
	if code != runtimeResourceExitFailed || !strings.Contains(out, "0 checked-clean, 1 checked-failed, 0 could-not-check (1 file(s) scanned") {
		t.Errorf("nested .yml record: exit %d, want 1 with the record read and failed; output:\n%s", code, out)
	}

	badRegistry := filepath.Join(dir, "adapters.yaml")
	if err := os.WriteFile(badRegistry, []byte("schema: runtime-adapters-v1\nadapters:\n  - id: half-adapter\n    classes: [snapshot]\n    verbs: [create, inspect]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	reg, err := os.ReadFile(rrAdaptersFile)
	if err != nil {
		t.Fatal(err)
	}
	misspelt := filepath.Join(dir, "misspelt.yaml")
	if err := os.WriteFile(misspelt, []byte(strings.Replace(string(reg), "max-ttl:", "max_ttl:", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	emptyDir := filepath.Join(dir, "empty")
	if err := os.Mkdir(emptyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, args := range map[string][]string{
		"registry missing a required verb": {"--lint", "--adapters", badRegistry, rrRecordsDir},
		"registry with a misspelt key":     {"--lint", "--adapters", misspelt, rrRecordsDir},
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

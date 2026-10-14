package main

// instance_test.go — graph-execution/09: the workflow-instance-v1 contract,
// `statusgen instance validate`, export/import and the role packet view.
//
// Every flow test drives production code (instantiate, validateInstanceFile,
// exportInstance, importInstance, projectPacket, buildHandoff, acceptHandoff).
// Negative rows each prove a DISTINCT lower boundary: the import-time Cell
// check is tested apart from the document-level actor Cell rule, and the
// acceptance-time dependency re-check apart from the packet's own delta.
// instance-mutations.json holds the mutation entries named in the Verify table.
//
// Synthetic credential material is assembled at run time (synthToken) so no
// secret-shaped literal lives in the tree.

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const instRoot = "testdata/instances/root"

func instPath(name string) string { return filepath.Join("testdata", "instances", name) }

func mustDecodeInstance(t *testing.T, path string) workflowInstance {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	schema, err := instanceSchema()
	if err != nil {
		t.Fatalf("instance schema: %v", err)
	}
	inst, fs, refusals := decodeInstance(raw, schema)
	if len(fs) > 0 || len(refusals) > 0 {
		t.Fatalf("decode %s: findings %v refusals %v", path, fs, refusals)
	}
	return inst
}

func ruleSet(fs []instanceFinding) []string {
	seen := map[string]bool{}
	var out []string
	for _, f := range fs {
		if !seen[f.Rule] {
			seen[f.Rule] = true
			out = append(out, f.Rule)
		}
	}
	return out
}

// pinnedObservations is what a runner observes when nothing moved since
// instantiation: every dependency at its pinned fingerprint.
func pinnedObservations(inst workflowInstance) map[string]string {
	m := map[string]string{}
	for _, d := range inst.Dependencies {
		m[d.Ref] = d.Fingerprint
	}
	return m
}

// synthToken assembles a forge-token-shaped string at run time.
func synthToken() string { return "gh" + "p_" + strings.Repeat("Q7", 18) }

// regression: graph-execution/09 — each fixture yields its declared verdict
func TestInstanceFixtures(t *testing.T) {
	cases := []struct {
		file, prior, state string
		rules              []string
	}{
		{"valid.yaml", "", "checked-clean", nil},
		{"superseding.yaml", "valid.yaml", "checked-clean", nil},
		{"rewrite-same-id.yaml", "valid.yaml", "checked-failed", []string{ruleInstanceRewrite}},
		{"stale-acceptance.yaml", "", "checked-failed", []string{ruleInstanceAcceptStale}},
		{"unknown-mandatory-field.yaml", "", "could-not-check", []string{ruleInstanceCapability}},
		{"cross-cell.yaml", "", "checked-failed", []string{ruleInstanceCrossCell}},
		{"reporter-trust-promoted.yaml", "", "checked-failed", []string{ruleInstanceTrust}},
		{"missing-pattern.yaml", "", "checked-failed", []string{ruleInstancePatternUnbound}},
		{"capability-minted.yaml", "", "checked-failed", []string{ruleInstanceCapMinted, ruleInstanceEffectPattern}},
		{"silent-truncation.yaml", "", "checked-failed", []string{ruleInstanceTruncation}},
	}
	for _, c := range cases {
		t.Run(c.file, func(t *testing.T) {
			opts := instanceValidateOptions{File: instPath(c.file), Root: instRoot}
			if c.prior != "" {
				opts.Prior = instPath(c.prior)
			}
			rep := validateInstanceFile(opts)
			if rep.State != c.state {
				t.Fatalf("state %s, want %s (findings %v, could-not-check %v)", rep.State, c.state, rep.Findings, rep.CouldNotCheck)
			}
			got := ruleSet(append(append([]instanceFinding{}, rep.Findings...), rep.CouldNotCheck...))
			want := append([]string(nil), c.rules...)
			if len(got) == 0 {
				got = nil
			}
			sortStrings(got)
			sortStrings(want)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("rules %v, want exactly %v (findings %v, could-not-check %v)", got, want, rep.Findings, rep.CouldNotCheck)
			}
		})
	}
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// regression: graph-execution/09 — an actor or an import from another Cell is refused
func TestInstanceCrossCellDenied(t *testing.T) {
	// Document level: a node actor bound in another Cell.
	rep := validateInstanceFile(instanceValidateOptions{File: instPath("cross-cell.yaml"), Root: instRoot})
	if rep.State != "checked-failed" || !reflect.DeepEqual(ruleSet(rep.Findings), []string{ruleInstanceCrossCell}) {
		t.Fatalf("cross-cell actor: state %s findings %v, want checked-failed [%s]", rep.State, rep.Findings, ruleInstanceCrossCell)
	}

	// Lower boundary, independent of the actor rule: a wholly self-consistent
	// instance of Cell example-cell offered to another Cell's import.
	inst := mustDecodeInstance(t, instPath("valid.yaml"))
	sealed, err := exportInstance(inst)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if _, err := importInstance(sealed, "example-cell"); err != nil {
		t.Fatalf("own-Cell import (positive control) refused: %v", err)
	}
	_, err = importInstance(sealed, "example-cell-b")
	if err == nil || !strings.Contains(err.Error(), ruleInstanceCrossCell) {
		t.Fatalf("import into example-cell-b: err %v, want a %s refusal", err, ruleInstanceCrossCell)
	}

	// The validate CLI's --cell expectation is the same boundary for a reader.
	rep = validateInstanceFile(instanceValidateOptions{File: instPath("valid.yaml"), Root: instRoot, Cell: "example-cell-b"})
	if rep.State != "checked-failed" || !reflect.DeepEqual(ruleSet(rep.Findings), []string{ruleInstanceCrossCell}) {
		t.Fatalf("--cell example-cell-b: state %s findings %v", rep.State, rep.Findings)
	}
}

// bindingFrom lifts the binding of an existing instance so the round-trip test
// can re-instantiate the same work through instantiate().
func bindingFrom(inst workflowInstance) instanceBinding {
	b := instanceBinding{
		ID: inst.Instance.ID, WorkID: inst.Work.ID, Cell: inst.Work.Cell, Intent: inst.Work.Intent,
		Acceptance: inst.Acceptance, Policy: inst.Policy, Budget: inst.Budget, Assignment: inst.Assignment,
		Inputs: inst.Inputs, Dependencies: inst.Dependencies, Actors: map[string]instanceActor{},
	}
	for _, n := range inst.Nodes {
		b.Actors[n.Role] = n.Actor
	}
	return b
}

// regression: graph-execution/09 — instantiate, validate, export, import keep one identity
func TestInstanceRoundTripIdentity(t *testing.T) {
	pb, fs, refusals := resolveInstancePattern(instRoot, "implementation")
	if len(fs) > 0 || len(refusals) > 0 {
		t.Fatalf("resolve pattern: %v %v", fs, refusals)
	}
	base := mustDecodeInstance(t, instPath("valid.yaml"))
	inst, err := instantiate(pb, bindingFrom(base))
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}

	// Validate the instantiated document through the CLI pipeline.
	sealed, err := exportInstance(inst)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	file := filepath.Join(t.TempDir(), "instantiated.json")
	if err := os.WriteFile(file, sealed, 0o644); err != nil {
		t.Fatal(err)
	}
	rep := validateInstanceFile(instanceValidateOptions{File: file, Root: instRoot})
	if rep.State != "checked-clean" {
		t.Fatalf("instantiated instance: %s %v %v", rep.State, rep.Findings, rep.CouldNotCheck)
	}
	// The instantiated contract is the reviewed fixture's contract: same pattern
	// binding, nodes, effects and claims, so the same contract digest.
	if contractDigest(inst) != contractDigest(base) {
		t.Fatalf("instantiated contract %s differs from the fixture's %s", contractDigest(inst), contractDigest(base))
	}

	// Import returns the same identity, contract and projection.
	got, err := importInstance(sealed, inst.Work.Cell)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if got.Instance.ID != inst.Instance.ID || got.Work != inst.Work || contractDigest(got) != contractDigest(inst) {
		t.Fatalf("identity moved across export/import: %s/%s/%s vs %s/%s/%s",
			got.Instance.ID, got.Work.ID, contractDigest(got), inst.Instance.ID, inst.Work.ID, contractDigest(inst))
	}
	again, err := exportInstance(got)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(again, sealed) {
		t.Fatal("re-export after import is not byte-identical — the canonical form is not a fixed point")
	}

	// Altered after export: the seal no longer matches and import refuses.
	tampered := bytes.Replace(sealed, []byte(inst.Work.Intent), []byte("Widen the scope."), 1)
	if _, err := importInstance(tampered, inst.Work.Cell); err == nil || !strings.Contains(err.Error(), ruleInstanceSeal) {
		t.Fatalf("tampered import: err %v, want %s", err, ruleInstanceSeal)
	}

	// The instantiator never widens an actor to fit the pattern.
	b := bindingFrom(base)
	b.Actors["worker"] = instanceActor{Binding: "roster/example-cell/worker", Cell: "example-cell", Capabilities: []string{"comment"}}
	if _, err := instantiate(pb, b); err == nil || !strings.Contains(err.Error(), "lacks capability") {
		t.Fatalf("instantiate with an under-capable actor: err %v, want a refusal", err)
	}
}

// packetScenario is one testdata/instances/packet/*.json fixture.
type packetScenario struct {
	Description string `json:"description"`
	Instance    string `json:"instance"`
	Node        string `json:"node"`
	Projection  struct {
		Set  map[string]string `json:"set"`
		Drop []string          `json:"drop"`
	} `json:"projection"`
	Acceptance struct {
		Set  map[string]string `json:"set"`
		Drop []string          `json:"drop"`
	} `json:"acceptance"`
	Result struct {
		Scope          []string           `json:"scope"`
		Evidence       []instanceEvidence `json:"evidence"`
		Artifacts      []instanceArtifact `json:"artifacts"`
		Omissions      []instanceOmission `json:"omissions"`
		NextSafeAction string             `json:"next-safe-action"`
	} `json:"result"`
	Expect struct {
		Accepted            bool     `json:"accepted"`
		Error               string   `json:"error"`
		Changed             []string `json:"changed"`
		Affected            []string `json:"affected"`
		Completed           []string `json:"completed"`
		Remaining           []string `json:"remaining"`
		CoverageUnknown     []string `json:"coverage-unknown"`
		UnknownFingerprints []string `json:"unknown-fingerprints"`
		ReChanged           []string `json:"reprojected-changed"`
		ReAffected          []string `json:"reprojected-affected"`
		ReCompleted         []string `json:"reprojected-completed"`
		ReRemaining         []string `json:"reprojected-remaining"`
	} `json:"expect"`
}

func loadPacketScenario(t *testing.T, name string) (packetScenario, workflowInstance) {
	t.Helper()
	path := filepath.Join("testdata", "instances", "packet", name)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var sc packetScenario
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&sc); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return sc, mustDecodeInstance(t, filepath.Join(filepath.Dir(path), sc.Instance))
}

func observe(inst workflowInstance, set map[string]string, drop []string) map[string]string {
	m := pinnedObservations(inst)
	for k, v := range set {
		m[k] = v
	}
	for _, k := range drop {
		delete(m, k)
	}
	return m
}

func (sc packetScenario) result() handoffResult {
	return handoffResult{Scope: sc.Result.Scope, Evidence: sc.Result.Evidence, Artifacts: sc.Result.Artifacts,
		Omissions: sc.Result.Omissions, NextSafeAction: sc.Result.NextSafeAction}
}

func changedRefs(p instancePacket) []string {
	out := []string{}
	for _, c := range p.Delta.Changed {
		out = append(out, c.Ref)
	}
	return out
}

func nz(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// runPacketScenario projects, hands off and accepts one scenario, checking its
// declared expectations. It returns the packet, handoff and acceptance error.
func runPacketScenario(t *testing.T, name string) (instancePacket, instanceHandoff, error) {
	t.Helper()
	sc, inst := loadPacketScenario(t, name)
	pkt, err := projectPacket(inst, sc.Node, observe(inst, sc.Projection.Set, sc.Projection.Drop))
	if err != nil {
		t.Fatalf("%s: project: %v", name, err)
	}
	h, err := buildHandoff(pkt, sc.result())
	if err != nil {
		t.Fatalf("%s: handoff: %v", name, err)
	}
	accObs := observe(inst, sc.Acceptance.Set, sc.Acceptance.Drop)
	accErr := acceptHandoff(inst, h, accObs)
	if sc.Expect.Accepted && accErr != nil {
		t.Fatalf("%s: handoff refused: %v", name, accErr)
	}
	if !sc.Expect.Accepted && (accErr == nil || !strings.Contains(accErr.Error(), sc.Expect.Error)) {
		t.Fatalf("%s: acceptance err %v, want a refusal containing %q", name, accErr, sc.Expect.Error)
	}
	if sc.Expect.Changed != nil && !reflect.DeepEqual(changedRefs(pkt), nz(sc.Expect.Changed)) {
		t.Errorf("%s: delta changed %v, want %v", name, changedRefs(pkt), sc.Expect.Changed)
	}
	if sc.Expect.Affected != nil && !reflect.DeepEqual(pkt.Delta.Affected, nz(sc.Expect.Affected)) {
		t.Errorf("%s: affected %v, want %v", name, pkt.Delta.Affected, sc.Expect.Affected)
	}
	if sc.Expect.Completed != nil && !reflect.DeepEqual(pkt.Completed, nz(sc.Expect.Completed)) {
		t.Errorf("%s: completed %v, want %v", name, pkt.Completed, sc.Expect.Completed)
	}
	if sc.Expect.Remaining != nil && !reflect.DeepEqual(pkt.Remaining, nz(sc.Expect.Remaining)) {
		t.Errorf("%s: remaining %v, want %v", name, pkt.Remaining, sc.Expect.Remaining)
	}
	if sc.Expect.CoverageUnknown != nil && !reflect.DeepEqual(pkt.Coverage.Unknown, nz(sc.Expect.CoverageUnknown)) {
		t.Errorf("%s: coverage unknown %v, want %v", name, pkt.Coverage.Unknown, sc.Expect.CoverageUnknown)
	}
	for _, ref := range sc.Expect.UnknownFingerprints {
		found := false
		for _, d := range pkt.Dependencies {
			if d.Ref == ref && d.Fingerprint == packetUnknownFingerprint {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: dependency %s is not reported with the explicit fingerprint %q", name, ref, packetUnknownFingerprint)
		}
	}
	if sc.Expect.ReChanged != nil {
		re, err := projectPacket(inst, sc.Node, accObs)
		if err != nil {
			t.Fatalf("%s: re-project: %v", name, err)
		}
		if !reflect.DeepEqual(changedRefs(re), sc.Expect.ReChanged) || !reflect.DeepEqual(re.Delta.Affected, sc.Expect.ReAffected) ||
			!reflect.DeepEqual(re.Completed, nz(sc.Expect.ReCompleted)) || !reflect.DeepEqual(re.Remaining, nz(sc.Expect.ReRemaining)) {
			t.Errorf("%s: re-projected delta %v affected %v completed %v remaining %v, want %v %v %v %v", name,
				changedRefs(re), re.Delta.Affected, re.Completed, re.Remaining,
				sc.Expect.ReChanged, sc.Expect.ReAffected, sc.Expect.ReCompleted, sc.Expect.ReRemaining)
		}
		if !strings.HasPrefix(re.NextSafeAction, "revalidate acceptance rows") {
			t.Errorf("%s: re-projected next safe action %q does not revalidate the affected rows", name, re.NextSafeAction)
		}
		if err := packetIsCurrent(pkt, inst, accObs); err == nil {
			t.Errorf("%s: the earlier packet still reads as current after a dependency moved", name)
		}
	}
	return pkt, h, accErr
}

// regression: graph-execution/09 — instance to role view to handoff keeps revision, scope, omissions
func TestInstancePacketViewProvenance(t *testing.T) {
	inst := mustDecodeInstance(t, instPath("valid.yaml"))
	pkt, h, _ := runPacketScenario(t, "same-input.json")

	// The packet is a view: re-projecting the same input yields the same bytes.
	again, err := projectPacket(inst, pkt.Node, pinnedObservations(inst))
	if err != nil {
		t.Fatal(err)
	}
	if !jsonEqual(pkt, again) {
		t.Fatal("projecting the same instance and input twice gave different packets")
	}
	if pkt.View != "role-packet" || pkt.Instance != inst.Instance.ID || pkt.ContractDigest != contractDigest(inst) {
		t.Fatalf("packet identity %s/%s/%s, want role-packet/%s/%s", pkt.View, pkt.Instance, pkt.ContractDigest, inst.Instance.ID, contractDigest(inst))
	}
	// Revision and scope carried from the canonical instance.
	if !reflect.DeepEqual(pkt.Acceptance, canonicalInstance(inst).Acceptance) || pkt.Policy != inst.Policy {
		t.Fatalf("packet acceptance/policy %+v %+v differ from the instance's", pkt.Acceptance, pkt.Policy)
	}
	// The handoff answers the same identifiers and revision.
	if h.Instance != pkt.Instance || h.ContractDigest != pkt.ContractDigest || h.Node != pkt.Node || h.Attempt != pkt.Attempt ||
		h.Cell != pkt.Cell || h.Work != pkt.Work || h.AcceptanceVersion != inst.Acceptance.Version || h.AcceptanceRef != inst.Acceptance.Ref {
		t.Fatalf("handoff identity %+v does not answer the packet's", h)
	}
	if !reflect.DeepEqual(h.Exclusions, inst.Acceptance.Exclusions) {
		t.Fatalf("handoff exclusions %v, want %v", h.Exclusions, inst.Acceptance.Exclusions)
	}
	// Omissions: the excerpted diff is recorded with its coverage index and the
	// omission travels from instance to packet to handoff.
	if len(pkt.Omissions) != 1 || !reflect.DeepEqual(h.Omissions, pkt.Omissions) || pkt.Omissions[0].Ref != "artifact/example-cell/diff-a1" {
		t.Fatalf("omissions instance→packet→handoff: packet %v handoff %v", pkt.Omissions, h.Omissions)
	}
	// Trust provenance: the reporter's label is surfaced as unresolved, never as
	// a disposition.
	if len(pkt.Unresolved) != 1 || !strings.Contains(pkt.Unresolved[0], "reporter-note") || !strings.Contains(pkt.Unresolved[0], "is not a disposition") {
		t.Fatalf("unresolved %v, want the reporter-labelled input listed as unresolved", pkt.Unresolved)
	}
	// Runner, control profile, budget and assignment are references only.
	if pkt.References != (packetReferences{Backend: "example-backend", Generation: 4, RunnerProfile: inst.Assignment.RunnerProfile,
		ControlProfile: inst.Policy.ControlProfile, Budget: inst.Budget.Ref}) {
		t.Fatalf("references %+v", pkt.References)
	}

	// A handoff that answers another attempt, or acceptance version, is refused.
	other := h
	other.Attempt = pkt.Node + ".a2"
	if err := acceptHandoff(inst, other, pinnedObservations(inst)); err == nil {
		t.Fatal("a handoff naming a different attempt was accepted")
	}
	other = h
	other.AcceptanceVersion = inst.Acceptance.Version - 1
	if err := acceptHandoff(inst, other, pinnedObservations(inst)); err == nil {
		t.Fatal("a handoff answering an older acceptance version was accepted")
	}
	// Result scope outside the acceptance scope is refused at build time.
	if _, err := buildHandoff(pkt, handoffResult{Scope: []string{"9"}}); err == nil {
		t.Fatal("a result scoped to a row outside acceptance was bound")
	}
	// Evidence that names an artifact the handoff does not return is not evidence.
	unbacked, err := buildHandoff(pkt, handoffResult{Scope: []string{"2"},
		Evidence: []instanceEvidence{{Claim: "Verify rows pass", Artifact: "artifact/example-cell/test-log-a1"}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := acceptHandoff(inst, unbacked, pinnedObservations(inst)); err == nil || !strings.Contains(err.Error(), "mandatory evidence") {
		t.Fatalf("evidence with no returned artifact: err %v, want a refusal", err)
	}
	// Omitted mandatory evidence and unknown coverage scenarios.
	runPacketScenario(t, "omitted-evidence.json")
	runPacketScenario(t, "unknown-coverage.json")

	// Silent truncation: an excerpt with no coverage index is never projected.
	trunc := mustDecodeInstance(t, instPath("valid.yaml"))
	trunc.Nodes[0].Attempts[0].Artifacts[0].Index = ""
	if _, err := projectPacket(trunc, pkt.Node, pinnedObservations(trunc)); err == nil || !strings.Contains(err.Error(), ruleInstanceTruncation) {
		t.Fatalf("projecting a silently truncated artifact: err %v", err)
	}
	cut := 10
	if _, err := buildHandoff(pkt, handoffResult{Artifacts: []instanceArtifact{{Ref: "artifact/example-cell/log", Digest: "sha256:" + strings.Repeat("ab", 32), Bytes: 100, IncludedBytes: &cut}}}); err == nil {
		t.Fatal("a handoff carrying a silently truncated artifact was bound")
	}
}

// regression: graph-execution/09 — a handoff bound to a moved dependency is refused
func TestInstancePacketChangedDependency(t *testing.T) {
	runPacketScenario(t, "changed-policy.json")
	// Omitted mandatory evidence cannot stand in for the current packet either.
	runPacketScenario(t, "omitted-evidence.json")
	// A packet projected before the move is no longer current; only a
	// re-projection is.
	inst := mustDecodeInstance(t, instPath("valid.yaml"))
	moved := observe(inst, map[string]string{"policy/example-authorization.yaml": "sha256:" + strings.Repeat("25", 32)}, nil)
	old, err := projectPacket(inst, inst.Nodes[0].ID, pinnedObservations(inst))
	if err != nil {
		t.Fatal(err)
	}
	if err := packetIsCurrent(old, inst, moved); err == nil {
		t.Fatal("a packet projected before the dependency moved reads as current")
	}
	fresh, err := projectPacket(inst, inst.Nodes[0].ID, moved)
	if err != nil {
		t.Fatal(err)
	}
	if err := packetIsCurrent(fresh, inst, moved); err != nil {
		t.Fatalf("the re-projected packet is not current: %v", err)
	}
}

// regression: graph-execution/09 — export/import keeps assignment; substitution refused
func TestInstanceBackendRoundTrip(t *testing.T) {
	inst := mustDecodeInstance(t, instPath("valid.yaml"))
	sealed, err := exportInstance(inst)
	if err != nil {
		t.Fatal(err)
	}
	got, err := importInstance(sealed, "example-cell")
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if got.Instance.ID != inst.Instance.ID || got.Assignment != inst.Assignment || got.ExecutionState != inst.ExecutionState {
		t.Fatalf("identity/assignment moved across export/import: %+v %+v vs %+v %+v", got.Instance, got.Assignment, inst.Instance, inst.Assignment)
	}
	if !reflect.DeepEqual(got.Acceptance, canonicalInstance(inst).Acceptance) || contractDigest(got) != contractDigest(inst) {
		t.Fatalf("acceptance or contract moved across export/import")
	}
	if !jsonEqual(got.Nodes, canonicalInstance(inst).Nodes) || !jsonEqual(got.Decisions, canonicalInstance(inst).Decisions) {
		t.Fatal("attempt or decision records moved across export/import")
	}
	if fs := checkAgainstPrior(inst, got); len(fs) > 0 {
		t.Fatalf("the imported instance is not a valid revision of the exported one: %v", fs)
	}

	// A transfer to another backend advances the generation: accepted.
	moved := got
	moved.Assignment = instanceAssignment{Backend: "example-backend-b", Generation: 5, RunnerProfile: got.Assignment.RunnerProfile}
	moved.ExecutionState.Generation = 5
	if fs := checkAgainstPrior(got, moved); len(fs) > 0 {
		t.Fatalf("a generation-advancing transfer was refused: %v", fs)
	}
	if !jsonEqual(contractDigest(moved), contractDigest(got)) {
		t.Fatal("a backend transfer changed the contract digest")
	}

	// Substitution: another backend under the same generation.
	sub := got
	sub.Assignment.Backend = "example-backend-b"
	if fs := checkAgainstPrior(got, sub); !reflect.DeepEqual(ruleSet(fs), []string{ruleInstanceSubstituted}) {
		t.Fatalf("same-generation backend change: %v, want [%s]", fs, ruleInstanceSubstituted)
	}
	// A generation that goes backwards is a substitution too.
	back := got
	back.Assignment.Generation = 3
	back.ExecutionState.Generation = 3
	if fs := checkAgainstPrior(got, back); !reflect.DeepEqual(ruleSet(fs), []string{ruleInstanceSubstituted}) {
		t.Fatalf("backwards generation: %v", fs)
	}
	// A dropped attempt record is a rewrite of history.
	dropped := mustDecodeInstance(t, instPath("valid.yaml"))
	dropped.Nodes[0].Attempts = nil
	if fs := checkAgainstPrior(inst, dropped); !reflect.DeepEqual(ruleSet(fs), []string{ruleInstanceRecordRewritten}) {
		t.Fatalf("dropped attempt: %v", fs)
	}
	// An export whose seal is stripped is not importable.
	var generic map[string]any
	if err := json.Unmarshal(sealed, &generic); err != nil {
		t.Fatal(err)
	}
	delete(generic, "digest")
	unsealed, _ := json.Marshal(generic)
	if _, err := importInstance(unsealed, "example-cell"); err == nil || !strings.Contains(err.Error(), ruleInstanceSeal) {
		t.Fatalf("unsealed import: %v", err)
	}
}

// regression: graph-execution/09 — superseding instance must name prior id and digest
func TestInstanceSupersession(t *testing.T) {
	prior := mustDecodeInstance(t, instPath("valid.yaml"))
	next := mustDecodeInstance(t, instPath("superseding.yaml"))
	if fs := checkAgainstPrior(prior, next); len(fs) > 0 {
		t.Fatalf("superseding.yaml: %v", fs)
	}
	wrong := next
	wrong.Instance.Supersedes = &instanceSupersedes{ID: prior.Instance.ID, ContractDigest: "sha256:" + strings.Repeat("0f", 32)}
	if fs := checkAgainstPrior(prior, wrong); !reflect.DeepEqual(ruleSet(fs), []string{ruleInstanceSupersession}) {
		t.Fatalf("wrong supersedes digest: %v", fs)
	}
	carried := next
	carried.Decisions = prior.Decisions
	if fs := checkAgainstPrior(prior, carried); len(fs) == 0 {
		t.Fatal("a superseding instance carried the prior's decision as its own")
	}
	self := next
	self.Instance.Supersedes = &instanceSupersedes{ID: next.Instance.ID, ContractDigest: contractDigest(prior)}
	if !reflect.DeepEqual(ruleSet(checkInstanceRules(self)), []string{ruleInstanceSupersession}) {
		t.Fatal("an instance naming itself in supersedes passed")
	}
}

// regression: graph-execution/09 — an unknown required capability is refused
func TestInstanceUnknownCapability(t *testing.T) {
	inst := mustDecodeInstance(t, instPath("valid.yaml"))
	schema, _ := instanceSchema()
	for _, mutate := range []func(map[string]any){
		func(m map[string]any) { m["requires"] = []any{"packet-view@1", "settlement@1"} },
		func(m map[string]any) { m["execution-state"].(map[string]any)["projection-version"] = 2 },
	} {
		var m map[string]any
		b, _ := json.Marshal(canonicalInstance(inst))
		_ = json.Unmarshal(b, &m)
		mutate(m)
		raw, _ := json.Marshal(m)
		_, fs, refusals := decodeInstance(raw, schema)
		if len(fs) > 0 || len(refusals) == 0 {
			t.Fatalf("want a could-not-check refusal, got findings %v refusals %v", fs, refusals)
		}
	}
	// An unknown field with no requires entry is a schema violation, not ignored.
	var m map[string]any
	b, _ := json.Marshal(canonicalInstance(inst))
	_ = json.Unmarshal(b, &m)
	m["settlement"] = map[string]any{"mode": "hold"}
	raw, _ := json.Marshal(m)
	if _, fs, _ := decodeInstance(raw, schema); !reflect.DeepEqual(ruleSet(fs), []string{ruleInstanceSchema}) {
		t.Fatalf("unknown top-level field: %v", fs)
	}
}

// regression: graph-execution/09 — credential material refused in record, packet, handoff
func TestInstanceCredentialRejected(t *testing.T) {
	inst := mustDecodeInstance(t, instPath("valid.yaml"))
	schema, _ := instanceSchema()
	secret := synthToken()

	// In the record: a credential-shaped value, and a credential-named field.
	for _, mutate := range []func(map[string]any){
		func(m map[string]any) { m["work"].(map[string]any)["intent"] = "use " + secret },
		func(m map[string]any) { m["extensions"] = map[string]any{"api-key": "ref-only"} },
		func(m map[string]any) {
			m["budget"].(map[string]any)["ref"] = "https://user:" + secret + "@example.invalid/b"
		},
	} {
		var m map[string]any
		b, _ := json.Marshal(canonicalInstance(inst))
		_ = json.Unmarshal(b, &m)
		mutate(m)
		raw, _ := json.Marshal(m)
		_, fs, _ := decodeInstance(raw, schema)
		found := false
		for _, f := range fs {
			found = found || f.Rule == ruleInstanceCredential
		}
		if !found {
			t.Fatalf("credential material not refused: %v", fs)
		}
		if strings.Contains(findingsError("x", fs).Error(), secret) {
			t.Fatal("the finding echoes the credential value")
		}
	}

	// In the packet: an instance carrying a secret is never projected.
	bad := mustDecodeInstance(t, instPath("valid.yaml"))
	bad.Nodes[0].Attempts[0].NextSafeAction = "push with " + secret
	if _, err := projectPacket(bad, bad.Nodes[0].ID, pinnedObservations(bad)); err == nil {
		t.Fatal("a packet carrying credential material was projected")
	}
	// In the handoff: a result carrying a secret is never bound.
	pkt, err := projectPacket(inst, inst.Nodes[0].ID, pinnedObservations(inst))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := buildHandoff(pkt, handoffResult{Unresolved: []string{"Authorization: Bearer " + secret}}); err == nil {
		t.Fatal("a handoff carrying credential material was bound")
	}
	// The clean packet and handoff carry none.
	if err := credentialFree(pkt); err != nil {
		t.Fatalf("clean packet flagged: %v", err)
	}
}

// regression: graph-execution/09 — a reporter label never becomes a disposition
func TestInstanceTrustNotPromoted(t *testing.T) {
	rep := validateInstanceFile(instanceValidateOptions{File: instPath("reporter-trust-promoted.yaml"), Root: instRoot})
	if !reflect.DeepEqual(ruleSet(rep.Findings), []string{ruleInstanceTrust}) {
		t.Fatalf("reporter-trust-promoted.yaml: %v", rep.Findings)
	}
	inst := mustDecodeInstance(t, instPath("valid.yaml"))
	pkt, err := projectPacket(inst, inst.Nodes[0].ID, pinnedObservations(inst))
	if err != nil {
		t.Fatal(err)
	}
	for _, in := range pkt.Inputs {
		if in.Trust.Source == "reporter" && in.Trust.Disposition != "unresolved" {
			t.Fatalf("packet input %s promoted to %s", in.Name, in.Trust.Disposition)
		}
	}
	if len(pkt.Unresolved) != 1 {
		t.Fatalf("unresolved %v", pkt.Unresolved)
	}
	// Through export/import the reporter label stays a label.
	sealed, err := exportInstance(inst)
	if err != nil {
		t.Fatal(err)
	}
	got, err := importInstance(sealed, inst.Work.Cell)
	if err != nil {
		t.Fatal(err)
	}
	if got.Inputs[1].Trust != inst.Inputs[1].Trust || got.Inputs[1].Trust.Disposition != "unresolved" {
		t.Fatalf("reporter input trust after import: %+v", got.Inputs[1].Trust)
	}
	// An exported document whose reporter label was promoted is refused at import.
	promoted := mustDecodeInstance(t, instPath("reporter-trust-promoted.yaml"))
	sealed, err = exportInstance(promoted)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := importInstance(sealed, promoted.Work.Cell); err == nil || !strings.Contains(err.Error(), ruleInstanceTrust) {
		t.Fatalf("import of a promoted reporter label: %v", err)
	}
}

// regression: graph-execution/09 — effects stay within pattern and actor capability
func TestInstanceEffectsSubset(t *testing.T) {
	rep := validateInstanceFile(instanceValidateOptions{File: instPath("capability-minted.yaml"), Root: instRoot})
	got := ruleSet(rep.Findings)
	sortStrings(got)
	if !reflect.DeepEqual(got, []string{ruleInstanceCapMinted, ruleInstanceEffectPattern}) {
		t.Fatalf("capability-minted.yaml: %v", rep.Findings)
	}
	// An effect inside the pattern but outside the actor's resolved capabilities.
	inst := mustDecodeInstance(t, instPath("valid.yaml"))
	inst.Nodes[0].Actor.Capabilities = []string{"push", "comment"}
	if !reflect.DeepEqual(ruleSet(checkInstanceRules(inst)), []string{ruleInstanceEffectActor}) {
		t.Fatalf("effect beyond actor: %v", checkInstanceRules(inst))
	}
}

// regression: graph-execution/09 — a missing pattern binding never degrades to no-pattern
func TestInstanceMissingPattern(t *testing.T) {
	rep := validateInstanceFile(instanceValidateOptions{File: instPath("missing-pattern.yaml"), Root: instRoot})
	if rep.State != "checked-failed" || !reflect.DeepEqual(ruleSet(rep.Findings), []string{ruleInstancePatternUnbound}) {
		t.Fatalf("missing-pattern.yaml: %s %v", rep.State, rep.Findings)
	}
	// A root with no pattern directory at all is could-not-check, never clean.
	rep = validateInstanceFile(instanceValidateOptions{File: instPath("valid.yaml"), Root: t.TempDir()})
	if rep.State == "checked-clean" {
		t.Fatal("an instance validated clean against a root with no patterns")
	}
}

// regression: graph-execution/09 — CLI JSON report and exit codes
func TestInstanceCLIJSON(t *testing.T) {
	for _, c := range []struct {
		file  string
		code  int
		state string
	}{
		{"valid.yaml", instanceExitClean, "checked-clean"},
		{"stale-acceptance.yaml", instanceExitFailed, "checked-failed"},
		{"unknown-mandatory-field.yaml", instanceExitCouldNot, "could-not-check"},
	} {
		var out, errb bytes.Buffer
		code := runInstance([]string{"validate", "--file", instPath(c.file), "--root", instRoot, "--json"}, &out, &errb)
		if code != c.code {
			t.Fatalf("%s: exit %d, want %d (stderr %s)", c.file, code, c.code, errb.String())
		}
		var rep instanceReport
		if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
			t.Fatalf("%s: not JSON: %v", c.file, err)
		}
		if rep.State != c.state {
			t.Fatalf("%s: state %s", c.file, rep.State)
		}
		if c.state == "checked-clean" && (rep.Eligibility == nil || rep.Eligibility.Verdict != "eligible") {
			t.Fatalf("%s: eligibility %+v", c.file, rep.Eligibility)
		}
	}
	var out, errb bytes.Buffer
	if code := runInstance([]string{"validate"}, &out, &errb); code != instanceExitUsageError {
		t.Fatalf("missing --file: exit %d", code)
	}
}

// regression: graph-execution/09 — embedded schema copy matches the canonical one
func TestInstanceSchemaParity(t *testing.T) {
	canonical, err := os.ReadFile(filepath.Join("..", "schemas", "workflow-instance-v1.json"))
	if err != nil {
		t.Fatalf("reading canonical schema: %v", err)
	}
	if !bytes.Equal(canonical, embeddedInstanceSchemaBytes()) {
		t.Fatal("statusgen/schemas/workflow-instance-v1.json is not byte-identical to schemas/workflow-instance-v1.json")
	}
	schema, err := instanceSchema()
	if err != nil {
		t.Fatal(err)
	}
	assertValidatorCoversKeywords(t, schema)
}

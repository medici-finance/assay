package comms

import (
	"os"
	"strings"
	"testing"
)

// compiled_test.go — the enforcement half of the lane-ACL single-sourcing, plus
// the ACL refusal battery. It copies topology/drift_test.go's shape:
//
//   - TestACLCompiledMatchesSourceDiff diffs the compiled derivation against the
//     declared source (the "Diff" Verify row).
//   - the positive-control refusal tests feed LoadACL deliberately-bad sources and
//     assert it refuses with the right typed error, so a green LoadACL cannot be a
//     LoadACL that accepts everything.
//   - the CrossCell tests pin the two cross-cell properties: a non-coordinator row
//     is refused at LOAD, and every cross-cell verb is refused at CHECK time.

// loadSourceACL reads and compiles the declared source, failing (never passing
// quietly) if it cannot. go test runs with the package dir as cwd, so the source
// is at "laneacl.yaml".
func loadSourceACL(t *testing.T) *ACL {
	t.Helper()
	data, err := os.ReadFile(ACLSourceFile)
	if err != nil {
		t.Fatalf("COULD-NOT-CHECK: reading the declared source %s: %v", ACLSourceFile, err)
	}
	acl, err := LoadACL(data)
	if err != nil {
		t.Fatalf("COULD-NOT-CHECK: the declared source %s does not compile: %v", ACLSourceFile, err)
	}
	return acl
}

// TestACLCompiledMatchesSourceDiff is the Diff Verify row: the compiled derivation
// must equal the source, byte-for-byte in canonical form.
func TestACLCompiledMatchesSourceDiff(t *testing.T) {
	src := loadSourceACL(t)
	want := src.canonical()
	got := Compiled().canonical()
	if got != want {
		t.Errorf("DERIVATION DRIFT — internal/comms/laneacl.go compiledACL disagrees with %s.\n"+
			"Edit the SOURCE first, then mirror it into compiledACL. The source wins.\n"+
			"  source:\n%s\n  derivation:\n%s", ACLSourceFile, indent(want), indent(got))
	}
}

func indent(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i := range lines {
		lines[i] = "    " + lines[i]
	}
	return strings.Join(lines, "\n")
}

// --- refusal battery (LoadACL side) --------------------------------------------

// TestRefuseReservedVerb: a matrix naming a human-gate action is refused at LOAD,
// via the one deskkit deny-list. This is a construction-time refusal, not a
// message-time one — an ACL that could ever permit a human-gate move cannot be
// built.
func TestRefuseReservedVerb(t *testing.T) {
	// `approve` is a human-gate verb; putting it in the within-cell verb set must
	// be refused. (If this ever LOADS, the deny-list is not wired.)
	src := `schema: laneacl-v1
within_cell:
  roles: [the-desk, worker-desk]
  verbs: [handoff, approve]
cross_cell:
  pairs: []
  verbs: []
`
	_, err := LoadACL([]byte(src))
	if err == nil {
		t.Fatal("LoadACL accepted a within-cell verb `approve` — a human-gate verb must be refused at load")
	}
	if !isErr(err, ErrReservedVerb) {
		t.Fatalf("wrong error class: want ErrReservedVerb, got %v", err)
	}
}

// TestRefuseUnknownVerb: a verb outside the compiled within-cell vocabulary is
// refused at envelope parse. (Envelope-side refusal; named "Refus" so the battery
// row runs it.)
func TestRefuseUnknownVerb(t *testing.T) {
	raw := mustJSON(t, sampleEnvelopeMap(func(m map[string]any) {
		m["verb"] = "teleport" // not in {ask, handoff, notify}
	}))
	_, err := ParseEnvelope(raw)
	if err == nil {
		t.Fatal("ParseEnvelope accepted an unknown verb")
	}
	if !isErr(err, ErrUnknownVerb) {
		t.Fatalf("wrong error class: want ErrUnknownVerb, got %v", err)
	}
}

// --- CrossCell properties ------------------------------------------------------

// TestCrossCellNonCoordinatorRefusedAtLoad: a cross-cell row that is not
// the-desk <-> the-desk is refused at LOAD.
func TestCrossCellNonCoordinatorRefusedAtLoad(t *testing.T) {
	src := `schema: laneacl-v1
within_cell:
  roles: [the-desk, worker-desk]
  verbs: [handoff]
cross_cell:
  pairs:
    - from: worker-desk
      to: the-desk
  verbs: []
`
	_, err := LoadACL([]byte(src))
	if err == nil {
		t.Fatal("LoadACL accepted a cross-cell (worker-desk -> the-desk) row — cross-cell reach is the-desk <-> the-desk only")
	}
	if !isErr(err, ErrCrossCellReach) {
		t.Fatalf("wrong error class: want ErrCrossCellReach, got %v", err)
	}
}

// TestCrossCellVerbRefusedAtCheck: with the shipped (empty) cross-cell verb set,
// EVERY cross-cell message is refused by Allow — including one between two
// the-desk coordinators, which passes the reach check but has no permitted verb.
func TestCrossCellVerbRefusedAtCheck(t *testing.T) {
	acl := loadSourceACL(t)
	// the-desk -> the-desk across cells: reach is permitted, but no verb is.
	for _, verb := range []string{"handoff", "notify", "ask", "anything"} {
		if acl.Allow("cell-a", "the-desk", verb, "cell-b", "the-desk") {
			t.Errorf("Allow permitted cross-cell verb %q — the cross-cell verb set ships EMPTY, so every cross-cell message must be refused", verb)
		}
	}
	// A within-cell handoff between two distinct roles is the positive control:
	// the ACL is not refusing everything.
	if !acl.Allow("cell-a", "worker-desk", "handoff", "cell-a", "pr-review-desk") {
		t.Fatal("Allow refused a within-cell worker-desk -> pr-review-desk handoff — the matrix is refusing legitimate traffic")
	}
}

// TestReservedVerbGrepControl guards Verify row 4 (grep for approve/merge finds
// nothing) at the Go layer too: no shipped verb or role names a human-gate action.
func TestReservedVerbGrepControl(t *testing.T) {
	acl := Compiled()
	all := append([]string{}, acl.WithinVerbs...)
	all = append(all, acl.WithinRoles...)
	all = append(all, acl.CrossVerbs...)
	for _, p := range acl.CrossPairs {
		all = append(all, p.From, p.To)
	}
	for _, m := range all {
		for _, bad := range []string{"approve", "merge", "flip", "ready", "sign"} {
			if strings.Contains(m, bad) {
				t.Errorf("shipped matrix member %q contains reserved token %q", m, bad)
			}
		}
	}
}

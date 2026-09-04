package comms

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"gopkg.in/yaml.v3"
)

// laneacl.go — the (fromCell, fromRole, verb, toCell, toRole) allow-matrix,
// compiled from ONE declared source (laneacl.yaml) and diffed against it by a
// test (compiled_test.go). This copies the topology package's derive-or-diff
// convention: the declared source is the truth, the compiled value is a
// derivation a test pins to it, and a hand-maintained second copy is the defect.
//
// STRICTNESS, copied from topology's strict parse. Absent is refused (least
// authority — an absent lane grants nothing). An unrecognised value is a parse
// ERROR naming the line, never a best-effort read. A duplicate is an ERROR — a
// silent overwrite of an allow rule is exactly the kind of drift an ACL exists to
// prevent.
//
// WHY yaml.v3 HERE (and hand-rolled in topology). tools/desk now requires
// gopkg.in/yaml.v3 directly (see go.mod), so reading this small source with it is
// the reuse-ladder-correct choice — an existing dependency over a second
// hand-rolled reader — and yaml.Node gives the per-value line numbers the
// line-named errors need. topology predates that dependency and stayed
// hand-rolled; this package does not re-litigate that, it just does not add a
// second hand-rolled reader when the module already carries a strict one.
//
// RESERVED VERBS refused at LOAD, not at message time. A row or verb naming a
// human-gate action (approve / flip / merge / ready / sign) is refused when the
// ACL is constructed, using the ONE deny-list deskkit already carries
// (deskkit.ReservedMember, exposed from internal/deskkit/decide.go) rather than a
// mirrored copy. So an ACL that could ever permit a human-gate move cannot be
// loaded at all.

// compiledACL is this package's DERIVATION of laneacl.yaml — the copy that ships
// compiled into the binary, because the gateway runs from an arbitrary working
// directory and cannot read the source file at message time. It is field-for-field
// with laneacl.yaml, and TestACLCompiledMatchesSourceDiff (compiled_test.go) reads the
// source, compiles it, and fails NAMING THE DIFFERENCE when the two disagree. This
// is the identical derive-or-diff binding topology.compiled has to topology.yaml.
//
// TO CHANGE THE MATRIX: edit laneacl.yaml FIRST, then mirror it here, then run
//
//	cd tools/desk && go test -run TestACLCompiledMatchesSourceDiff ./internal/comms/
//
// Editing one without the other is a red test, not a silent divergence.
var compiledACL = ACL{
	// Within-cell: the full desk mesh (design ruling 5). Every standard desk role
	// may message every OTHER standard desk role, subject to this ACL.
	WithinRoles: []string{"intake-desk", "pr-review-desk", "the-desk", "verify-desk", "worker-desk"},
	// Within-cell verbs. Deliberately minimal and non-human-gate: a message hands a
	// work item to the next role (handoff), tells a role something with no action
	// required (notify), or asks a role a question (ask). None names a human-gate
	// action; extending this set is a recorded decision, not a silent addition.
	WithinVerbs: []string{"ask", "handoff", "notify"},
	// Cross-cell: the-desk <-> the-desk only (ruling 5).
	CrossPairs: []RolePair{{From: coordinatorRole, To: coordinatorRole}},
	// Cross-cell verbs — the four ruled by assay-toolkit#1896 (ratified 2026-09-02),
	// all read-only or advisory on the the-desk <-> the-desk lane: focus-on (a REQUEST
	// to prioritise named work; advisory only, the receiving desk may decline),
	// help-offered (things the receiving cell might need, answered with no obligation),
	// metrics (the cell's measured performance figures, read-derived), status (prose on
	// how the cell is going). None mutates state on the receiving cell. Kept in sync
	// with laneacl.yaml cross_cell.verbs by TestACLCompiledMatchesSourceDiff.
	CrossVerbs: []string{"focus-on", "help-offered", "metrics", "status"},
}

// Compiled returns a COPY of the compiled lane ACL. An accessor that handed out
// the package variable would let one caller's mutation change every other
// caller's allow decisions.
func Compiled() ACL {
	return ACL{
		WithinRoles: append([]string(nil), compiledACL.WithinRoles...),
		WithinVerbs: append([]string(nil), compiledACL.WithinVerbs...),
		CrossPairs:  append([]RolePair(nil), compiledACL.CrossPairs...),
		CrossVerbs:  append([]string(nil), compiledACL.CrossVerbs...),
	}
}

// ACLSchema is the one lane-ACL schema this reader accepts.
const ACLSchema = "laneacl-v1"

// ACLSourceFile is the repo-relative-to-this-package path of the declared source.
const ACLSourceFile = "laneacl.yaml"

// Lane-ACL typed errors. Distinct so the refusal battery can tell a reserved-verb
// refusal from a cross-cell-reach refusal from a parse error.
var (
	// ErrACLParse — the source is not well-formed laneacl-v1 (bad YAML, wrong
	// shape, unknown key, duplicate, unrecognised value). Names the line.
	ErrACLParse = errors.New("comms: lane-ACL source does not parse")
	// ErrReservedVerb — a role or verb in the matrix names a human-gate action.
	// Refused at LOAD.
	ErrReservedVerb = errors.New("comms: lane-ACL names a human-gate (reserved) action")
	// ErrCrossCellReach — a cross-cell row is not the-desk <-> the-desk. Cross-cell
	// reach is coordinator-to-coordinator only.
	ErrCrossCellReach = errors.New("comms: cross-cell row is not the-desk <-> the-desk")
)

// coordinatorRole is the one role permitted on a cross-cell lane, either end.
// Cross-cell reach is the-desk <-> the-desk only (the cell-boundary analogue of
// "the one sanctioned seam between cells is coordinator-to-coordinator").
const coordinatorRole = "the-desk"

// RolePair is one permitted (from-role, to-role) lane, used for cross-cell rows.
type RolePair struct {
	From string
	To   string
}

// ACL is the compiled allow-matrix. Its slices are sorted and de-duplicated, so
// two ACLs compiled from the same facts render identically (the diff test relies
// on that). It is deliberately small: within-cell reach is a full role mesh over
// a verb set, and cross-cell reach is a short pair list over a (currently empty)
// verb set — so the matrix is those factors, not an enumerated product that could
// drift a cell at a time.
type ACL struct {
	// WithinRoles are the standard desk roles that may message each other
	// within a cell (full mesh, distinct roles).
	WithinRoles []string
	// WithinVerbs are the verbs permitted on a within-cell lane.
	WithinVerbs []string
	// CrossPairs are the permitted cross-cell (from-role, to-role) lanes. Every
	// pair is the-desk <-> the-desk; the field is a list rather than a bool so the
	// shape generalises if a future ruling widens reach, without changing Allow.
	CrossPairs []RolePair
	// CrossVerbs are the verbs permitted on a cross-cell lane. Ships EMPTY (fail
	// closed) — an OPEN DECISION until a human rules on it.
	CrossVerbs []string
}

// KnownVerb reports whether verb is a member of the compiled vocabulary — a
// within-cell verb OR a cross-cell verb (assay-toolkit#1896). ParseEnvelope uses it
// to refuse an unknown verb at parse; the four cross-cell verbs are folded in so a
// cross-cell status/metrics/help-offered/focus-on message parses. A verb legal only
// cross-cell still has to clear Allow's reach + cross-verb check, and a within-cell
// send carrying a cross-only verb is refused there as out-of-lane.
func KnownVerb(verb string) bool {
	for _, v := range compiledACL.WithinVerbs {
		if v == verb {
			return true
		}
	}
	for _, v := range compiledACL.CrossVerbs {
		if v == verb {
			return true
		}
	}
	return false
}

// Allow reports whether a message on lane (fromCell, fromRole) -> (toCell, toRole)
// carrying verb is permitted. It is the whole point of the matrix: ABSENT IS
// REFUSED. A tuple the compiled matrix does not explicitly permit returns false —
// there is no default-allow, no wildcard, and no "unknown means fine".
//
//   - within-cell (fromCell == toCell): both roles must be standard desk roles,
//     they must be DISTINCT (a lane is between two roles, not a role and itself),
//     and verb must be a within-cell verb.
//   - cross-cell (fromCell != toCell): the (fromRole, toRole) pair must be a
//     permitted cross pair (the-desk <-> the-desk) AND verb must be a cross-cell
//     verb. Since CrossVerbs ships empty, every cross-cell verb is refused until a
//     human rules on the open decision.
func (a *ACL) Allow(fromCell, fromRole, verb, toCell, toRole string) bool {
	if a == nil {
		return false
	}
	if fromCell == "" || toCell == "" || fromRole == "" || toRole == "" || verb == "" {
		return false
	}
	if fromCell == toCell {
		if fromRole == toRole {
			return false
		}
		return contains(a.WithinRoles, fromRole) &&
			contains(a.WithinRoles, toRole) &&
			contains(a.WithinVerbs, verb)
	}
	// cross-cell
	permittedPair := false
	for _, p := range a.CrossPairs {
		if p.From == fromRole && p.To == toRole {
			permittedPair = true
			break
		}
	}
	return permittedPair && contains(a.CrossVerbs, verb)
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

// canonical renders the ACL as a deterministic string, so the diff test can
// byte-compare a source-compiled ACL against the derivation.
func (a ACL) canonical() string {
	var b strings.Builder
	writeList := func(name string, xs []string) {
		cp := append([]string(nil), xs...)
		sort.Strings(cp)
		fmt.Fprintf(&b, "%s: [%s]\n", name, strings.Join(cp, " "))
	}
	writeList("within_roles", a.WithinRoles)
	writeList("within_verbs", a.WithinVerbs)
	pairs := make([]string, 0, len(a.CrossPairs))
	for _, p := range a.CrossPairs {
		pairs = append(pairs, p.From+"->"+p.To)
	}
	writeList("cross_pairs", pairs)
	writeList("cross_verbs", a.CrossVerbs)
	return b.String()
}

// LoadACL parses and validates a lane-ACL source. Every error is an
// ErrACLParse / ErrReservedVerb / ErrCrossCellReach naming the offending line;
// it never returns a partially-built ACL alongside an error, and never an empty
// ACL with a nil error.
func LoadACL(data []byte) (*ACL, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrACLParse, err)
	}
	root, err := documentMapping(&doc)
	if err != nil {
		return nil, err
	}

	// schema — refuse an unversioned or wrong-versioned source rather than read it
	// on a best-effort basis.
	schemaNode, ok := mapField(root, "schema")
	if !ok {
		return nil, fmt.Errorf("%w: no `schema:` key — an unversioned lane-ACL source is refused", ErrACLParse)
	}
	if schemaNode.Value != ACLSchema {
		return nil, fmt.Errorf("%w: line %d: schema %q is not the %q this reader accepts", ErrACLParse, schemaNode.Line, schemaNode.Value, ACLSchema)
	}

	// Reject unknown top-level keys — a key this reader does not model is a typo or
	// a silently-added surface, never ignored.
	if err := onlyKeys(root, "schema", "within_cell", "cross_cell"); err != nil {
		return nil, err
	}

	acl := &ACL{}

	// within_cell.roles + within_cell.verbs
	within, ok := mapField(root, "within_cell")
	if !ok {
		return nil, fmt.Errorf("%w: no `within_cell:` key", ErrACLParse)
	}
	if err := onlyKeys(within, "roles", "verbs"); err != nil {
		return nil, err
	}
	if acl.WithinRoles, err = strSeqField(within, "roles", true); err != nil {
		return nil, err
	}
	if acl.WithinVerbs, err = strSeqField(within, "verbs", true); err != nil {
		return nil, err
	}

	// cross_cell.pairs + cross_cell.verbs
	cross, ok := mapField(root, "cross_cell")
	if !ok {
		return nil, fmt.Errorf("%w: no `cross_cell:` key", ErrACLParse)
	}
	if err := onlyKeys(cross, "pairs", "verbs"); err != nil {
		return nil, err
	}
	if acl.CrossPairs, err = pairSeqField(cross, "pairs"); err != nil {
		return nil, err
	}
	// verbs is OPTIONAL and ships empty; absence means the empty (fail-closed) set,
	// not a parse error — an absent allow-set is the least-authority reading.
	if acl.CrossVerbs, err = strSeqField(cross, "verbs", false); err != nil {
		return nil, err
	}

	// Reserved-verb refusal, at LOAD. Every role and every verb in the matrix is
	// checked against the ONE human-gate deny-list, so a matrix that could permit a
	// human-gate move cannot be constructed.
	for _, list := range [][2]any{
		{"within_cell.roles", acl.WithinRoles},
		{"within_cell.verbs", acl.WithinVerbs},
		{"cross_cell.verbs", acl.CrossVerbs},
	} {
		where := list[0].(string)
		for _, m := range list[1].([]string) {
			if root, bad := deskkit.ReservedMember(m); bad {
				return nil, fmt.Errorf("%w: %s member %q names a human-gate action (%q) — refused at load, "+
					"so the matrix can never permit a move only a human may make", ErrReservedVerb, where, m, root)
			}
		}
	}
	for _, p := range acl.CrossPairs {
		for _, r := range []string{p.From, p.To} {
			if root, bad := deskkit.ReservedMember(r); bad {
				return nil, fmt.Errorf("%w: cross_cell pair role %q names a human-gate action (%q)", ErrReservedVerb, r, root)
			}
		}
	}

	// Cross-cell reach: every pair must be the-desk <-> the-desk. A non-coordinator
	// cross-cell row is refused at load.
	for _, p := range acl.CrossPairs {
		if p.From != coordinatorRole || p.To != coordinatorRole {
			return nil, fmt.Errorf("%w: pair (%s -> %s) — cross-cell reach is %s <-> %s only",
				ErrCrossCellReach, p.From, p.To, coordinatorRole, coordinatorRole)
		}
	}

	// Normalise to sorted, deduped slices so a source-compiled ACL renders
	// identically to the derivation.
	acl.WithinRoles = sortedUnique(acl.WithinRoles)
	acl.WithinVerbs = sortedUnique(acl.WithinVerbs)
	acl.CrossVerbs = sortedUnique(acl.CrossVerbs)
	return acl, nil
}

// ---------------------------------------------------------------------------
// yaml.Node helpers — strict, line-aware reads over the modelled shape.
// ---------------------------------------------------------------------------

// documentMapping unwraps a DocumentNode to its root mapping, refusing anything
// that is not a mapping at the root.
func documentMapping(doc *yaml.Node) (*yaml.Node, error) {
	n := doc
	if n.Kind == yaml.DocumentNode {
		if len(n.Content) == 0 {
			return nil, fmt.Errorf("%w: source is empty — a reader that parsed nothing knows nothing", ErrACLParse)
		}
		n = n.Content[0]
	}
	if n.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%w: line %d: source root must be a mapping", ErrACLParse, n.Line)
	}
	return n, nil
}

// mapField returns the value node for key in a mapping, and whether it is present.
// A duplicate key is not resolved here — onlyKeys is where duplicates are caught,
// so every mapping this reader consumes is passed through it.
func mapField(m *yaml.Node, key string) (*yaml.Node, bool) {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1], true
		}
	}
	return nil, false
}

// onlyKeys asserts a mapping's keys are a subset of allowed and that no key is
// duplicated. Both are refusals naming the line: an unknown key is a typo or a
// silently-added surface, and a duplicate key is a silent overwrite.
func onlyKeys(m *yaml.Node, allowed ...string) error {
	ok := make(map[string]bool, len(allowed))
	for _, a := range allowed {
		ok[a] = true
	}
	seen := map[string]bool{}
	for i := 0; i+1 < len(m.Content); i += 2 {
		k := m.Content[i]
		if !ok[k.Value] {
			return fmt.Errorf("%w: line %d: unknown key %q (modelled keys: %s)", ErrACLParse, k.Line, k.Value, strings.Join(allowed, ", "))
		}
		if seen[k.Value] {
			return fmt.Errorf("%w: line %d: duplicate key %q — a duplicate is a silent overwrite, refused", ErrACLParse, k.Line, k.Value)
		}
		seen[k.Value] = true
	}
	return nil
}

// strSeqField reads a sequence-of-scalars field. When required is true, an absent
// or empty sequence is a parse error; when false, absence yields nil (the
// least-authority empty set). A duplicate member is refused. A non-sequence, or a
// non-scalar member, is a parse error naming the line.
func strSeqField(m *yaml.Node, key string, required bool) ([]string, error) {
	v, ok := mapField(m, key)
	if !ok {
		if required {
			return nil, fmt.Errorf("%w: no `%s:` key", ErrACLParse, key)
		}
		return nil, nil
	}
	if v.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("%w: line %d: %q is not a sequence", ErrACLParse, v.Line, key)
	}
	var out []string
	seen := map[string]bool{}
	for _, item := range v.Content {
		if item.Kind != yaml.ScalarNode {
			return nil, fmt.Errorf("%w: line %d: %q holds a non-scalar item", ErrACLParse, item.Line, key)
		}
		if strings.TrimSpace(item.Value) == "" {
			return nil, fmt.Errorf("%w: line %d: %q holds an empty member", ErrACLParse, item.Line, key)
		}
		if seen[item.Value] {
			return nil, fmt.Errorf("%w: line %d: %q holds a duplicate member %q", ErrACLParse, item.Line, key, item.Value)
		}
		seen[item.Value] = true
		out = append(out, item.Value)
	}
	if required && len(out) == 0 {
		return nil, fmt.Errorf("%w: line %d: %q is empty", ErrACLParse, v.Line, key)
	}
	return out, nil
}

// pairSeqField reads a sequence of {from, to} mappings. Absence yields nil (no
// cross-cell lanes — the least-authority reading).
func pairSeqField(m *yaml.Node, key string) ([]RolePair, error) {
	v, ok := mapField(m, key)
	if !ok {
		return nil, nil
	}
	if v.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("%w: line %d: %q is not a sequence", ErrACLParse, v.Line, key)
	}
	var out []RolePair
	seen := map[string]bool{}
	for _, item := range v.Content {
		if item.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("%w: line %d: %q holds a non-mapping item", ErrACLParse, item.Line, key)
		}
		if err := onlyKeys(item, "from", "to"); err != nil {
			return nil, err
		}
		from, okF := mapField(item, "from")
		to, okT := mapField(item, "to")
		if !okF || !okT || from.Kind != yaml.ScalarNode || to.Kind != yaml.ScalarNode ||
			strings.TrimSpace(from.Value) == "" || strings.TrimSpace(to.Value) == "" {
			return nil, fmt.Errorf("%w: line %d: a %q item needs scalar `from:` and `to:` roles", ErrACLParse, item.Line, key)
		}
		dupKey := from.Value + "\x00" + to.Value
		if seen[dupKey] {
			return nil, fmt.Errorf("%w: line %d: duplicate pair (%s -> %s)", ErrACLParse, item.Line, from.Value, to.Value)
		}
		seen[dupKey] = true
		out = append(out, RolePair{From: from.Value, To: to.Value})
	}
	return out, nil
}

func sortedUnique(xs []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(xs))
	for _, x := range xs {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	sort.Strings(out)
	return out
}

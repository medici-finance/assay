package main

// assign.go — the (action, class, risk) -> Tier assignment table, compiled
// from ONE declared source (assign.yaml) and diffed against it by a test
// (assign_test.go: TestAssignCompiledMatchesSourceDiff), copying the lane-ACL
// derive-or-diff convention (internal/comms/laneacl.go + laneacl.yaml).
//
// The prose router (a companion package landing separately) chooses an ACTION from a closed
// vocabulary and NEVER names a model or tier — routing is a content judgment,
// model assignment is a table lookup with an audit trail, including for the
// decider's OWN model (that half is internal/runnertable's DeciderEntry, a
// companion deliverable landing alongside this file). This file is the
// action-side half of that lookup: (action, class, risk) -> Tier. Tier then
// resolves through the pinned tier->runner table (internal/runnertable) to a
// concrete runner — see TestFlowActionToRunner for the one-test end-to-end proof.
//
// ABSENT IS REFUSED. A (action, class, risk) triple this table does not
// explicitly carry a row for refuses — there is no default tier and no
// best-effort resolution (deskkit exit 5, the same fail-closed posture every
// other guard in this package family uses). Extending the table is a
// recorded decision: edit assign.yaml FIRST, mirror it into compiledAssign,
// then run
//
//	cd tools/desk && go test -run TestAssignCompiledMatchesSourceDiff ./cmd/commsloop/
//
// Editing one without the other is a red test, not a silent divergence.

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
	"gopkg.in/yaml.v3"
)

// assignSchema is the one assign-table schema this reader accepts.
const assignSchema = "assign-v1"

// assignSourceFile is the repo-relative-to-this-package path of the declared
// source (mirrors laneacl.ACLSourceFile).
const assignSourceFile = "assign.yaml"

// KnownActions is the closed action vocabulary the prose router chooses
// from. It is MIRRORED here, not imported, because the router has not landed
// yet; once it does, its own vocabulary constant becomes the declared source
// of this list and a diff test binds the two — the same relationship
// compiledAssign already has to assign.yaml today. Absent is refused: an
// action outside this set never resolves to a tier.
var KnownActions = map[string]bool{
	"route-work-ready":     true,
	"route-review":         true,
	"route-verify":         true,
	"land-report":          true,
	"file-question-issue":  true,
	"escalate-human-issue": true,
	"quarantine":           true,
}

// KnownClasses mirrors internal/comms.KnownClasses (routine, sensitive) — the
// envelope's handling-class vocabulary this table keys on. It is not imported
// directly: this change does not otherwise need a cmd/commsloop ->
// internal/comms dependency edge, and the follow-up that wires the full
// inbound path (and already imports internal/comms for envelope parsing) is
// the natural point to replace this mirror with a direct reference.
var KnownClasses = map[string]bool{
	"routine":   true,
	"sensitive": true,
}

// tierByName / tierName are the closed tier-name <-> loopengine.Tier mapping
// this table's source and compiled form both use. "human" is included
// deliberately: TierHuman rows are legal in the assign table (they route to
// Land/escalation, never dispatch) — mirroring the engine's TierHuman
// semantics — even though internal/runnertable.LoadRunnerTable refuses a
// "human" RUNNER entry (the two are different surfaces: this table may
// ASSIGN TierHuman, the runner table may never be asked to DISPATCH it; see
// TestAssignHumanTierNeverDispatches).
var tierByName = map[string]loopengine.Tier{
	"local":   loopengine.TierLocal,
	"cheap":   loopengine.TierCheap,
	"session": loopengine.TierSession,
	"human":   loopengine.TierHuman,
}

func tierName(t loopengine.Tier) string {
	for name, v := range tierByName {
		if v == t {
			return name
		}
	}
	return t.String()
}

// assignKey is the compiled table's lookup key — the (action, class, risk)
// triple the brief names.
type assignKey struct {
	Action string
	Class  string
	Risk   bool
}

// compiledAssign is this package's DERIVATION of assign.yaml — the copy that
// ships compiled into the binary, because commsloop's boot cannot assume a
// working directory that carries the source file at message time (the same
// reason internal/comms/laneacl.go compiles its matrix rather than reading
// laneacl.yaml at runtime). TestAssignCompiledMatchesSourceDiff reads the
// source, compiles it, and fails NAMING THE DIFFERENCE when the two disagree.
//
// See assign.yaml for the declared rule set this table implements: risk:
// "yes" on any (action, class) forces TierHuman; dispatch-class actions
// (route-work-ready / route-review / route-verify) default to TierSession on
// the non-risk path (the standing fan-out convention); the two bookkeeping
// actions (land-report / file-question-issue) scale TierLocal -> TierCheap
// with class instead; escalate-human-issue and quarantine are ALWAYS
// TierHuman, regardless of class or risk.
var compiledAssign = map[assignKey]loopengine.Tier{
	// route-work-ready
	{Action: "route-work-ready", Class: "routine", Risk: false}:   loopengine.TierSession,
	{Action: "route-work-ready", Class: "sensitive", Risk: false}: loopengine.TierSession,
	{Action: "route-work-ready", Class: "routine", Risk: true}:    loopengine.TierHuman,
	{Action: "route-work-ready", Class: "sensitive", Risk: true}:  loopengine.TierHuman,

	// route-review
	{Action: "route-review", Class: "routine", Risk: false}:   loopengine.TierSession,
	{Action: "route-review", Class: "sensitive", Risk: false}: loopengine.TierSession,
	{Action: "route-review", Class: "routine", Risk: true}:    loopengine.TierHuman,
	{Action: "route-review", Class: "sensitive", Risk: true}:  loopengine.TierHuman,

	// route-verify
	{Action: "route-verify", Class: "routine", Risk: false}:   loopengine.TierSession,
	{Action: "route-verify", Class: "sensitive", Risk: false}: loopengine.TierSession,
	{Action: "route-verify", Class: "routine", Risk: true}:    loopengine.TierHuman,
	{Action: "route-verify", Class: "sensitive", Risk: true}:  loopengine.TierHuman,

	// land-report
	{Action: "land-report", Class: "routine", Risk: false}:   loopengine.TierLocal,
	{Action: "land-report", Class: "sensitive", Risk: false}: loopengine.TierCheap,
	{Action: "land-report", Class: "routine", Risk: true}:    loopengine.TierHuman,
	{Action: "land-report", Class: "sensitive", Risk: true}:  loopengine.TierHuman,

	// file-question-issue
	{Action: "file-question-issue", Class: "routine", Risk: false}:   loopengine.TierLocal,
	{Action: "file-question-issue", Class: "sensitive", Risk: false}: loopengine.TierCheap,
	{Action: "file-question-issue", Class: "routine", Risk: true}:    loopengine.TierHuman,
	{Action: "file-question-issue", Class: "sensitive", Risk: true}:  loopengine.TierHuman,

	// escalate-human-issue — always human
	{Action: "escalate-human-issue", Class: "routine", Risk: false}:   loopengine.TierHuman,
	{Action: "escalate-human-issue", Class: "sensitive", Risk: false}: loopengine.TierHuman,
	{Action: "escalate-human-issue", Class: "routine", Risk: true}:    loopengine.TierHuman,
	{Action: "escalate-human-issue", Class: "sensitive", Risk: true}:  loopengine.TierHuman,

	// quarantine — always human
	{Action: "quarantine", Class: "routine", Risk: false}:   loopengine.TierHuman,
	{Action: "quarantine", Class: "sensitive", Risk: false}: loopengine.TierHuman,
	{Action: "quarantine", Class: "routine", Risk: true}:    loopengine.TierHuman,
	{Action: "quarantine", Class: "sensitive", Risk: true}:  loopengine.TierHuman,
}

// Assign resolves the (action, class, risk) triple the prose router (05)
// hands back to a dispatch Tier. ABSENT IS REFUSED, at every stage:
//   - an empty action or class (the absent-triple case) refuses;
//   - an action outside KnownActions, or a class outside KnownClasses,
//     refuses (checked even though the compiled map would also miss, so the
//     error names WHICH part of the triple was the problem);
//   - a well-formed (action, class, risk) triple with no compiled row
//     refuses — there is no default tier.
func Assign(action, class string, risk bool) (loopengine.Tier, error) {
	if strings.TrimSpace(action) == "" || strings.TrimSpace(class) == "" {
		return 0, deskkit.Refused(fmt.Sprintf(
			"assign: absent (action, class, risk) triple (action=%q class=%q) — refusing (no default tier)", action, class))
	}
	if !KnownActions[action] {
		return 0, deskkit.Refused(fmt.Sprintf(
			"assign: unknown action %q — refusing (not in the closed action vocabulary)", action))
	}
	if !KnownClasses[class] {
		return 0, deskkit.Refused(fmt.Sprintf(
			"assign: unknown class %q — refusing (not a recognised handling class)", class))
	}
	tier, ok := compiledAssign[assignKey{Action: action, Class: class, Risk: risk}]
	if !ok {
		return 0, deskkit.Refused(fmt.Sprintf(
			"assign: no row for action=%q class=%q risk=%v — refusing (absent triple, no default tier)", action, class, risk))
	}
	return tier, nil
}

// --- declared-source loader (assign.yaml -> map[assignKey]loopengine.Tier) ---

// assignSourceDoc / assignSourceRow model assign.yaml's shape. yaml.v3's
// KnownFields(true) decoder option (set in LoadAssign) rejects an unknown
// field the same way internal/topology and internal/comms's laneacl reader
// reject an unmodelled key — this is the reuse-ladder-correct choice (an
// existing dependency already used for exactly this in this module) over a
// second hand-rolled strict reader.
type assignSourceDoc struct {
	Schema string            `yaml:"schema"`
	Rows   []assignSourceRow `yaml:"rows"`
}

type assignSourceRow struct {
	Action string `yaml:"action"`
	Class  string `yaml:"class"`
	Risk   string `yaml:"risk"` // "no" | "yes"
	Tier   string `yaml:"tier"` // local | cheap | session | human
}

// LoadAssign parses and validates a declared assign-table source (the
// assign.yaml shape), returning the same map[assignKey]loopengine.Tier shape
// compiledAssign holds so the diff test can compare them directly. Every
// failure is a distinct, line-free but row-numbered deskkit.Refused — never a
// partially-built table alongside an error.
func LoadAssign(data []byte) (map[assignKey]loopengine.Tier, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	var doc assignSourceDoc
	if err := dec.Decode(&doc); err != nil {
		return nil, deskkit.Refused("assign: source does not parse: " + err.Error())
	}
	if doc.Schema != assignSchema {
		return nil, deskkit.Refused(fmt.Sprintf(
			"assign: schema %q is not the %q this reader accepts", doc.Schema, assignSchema))
	}
	if len(doc.Rows) == 0 {
		return nil, deskkit.Refused("assign: source has no rows — an empty table refuses every triple, which is not the same as an absent source")
	}

	out := make(map[assignKey]loopengine.Tier, len(doc.Rows))
	for i, r := range doc.Rows {
		if strings.TrimSpace(r.Action) == "" || strings.TrimSpace(r.Class) == "" {
			return nil, deskkit.Refused(fmt.Sprintf("assign: row %d: action/class must not be empty", i))
		}
		if !KnownActions[r.Action] {
			return nil, deskkit.Refused(fmt.Sprintf("assign: row %d: unknown action %q", i, r.Action))
		}
		if !KnownClasses[r.Class] {
			return nil, deskkit.Refused(fmt.Sprintf("assign: row %d: unknown class %q", i, r.Class))
		}
		var risk bool
		switch r.Risk {
		case "no":
			risk = false
		case "yes":
			risk = true
		default:
			return nil, deskkit.Refused(fmt.Sprintf("assign: row %d: risk must be \"no\" or \"yes\", got %q", i, r.Risk))
		}
		tier, ok := tierByName[r.Tier]
		if !ok {
			return nil, deskkit.Refused(fmt.Sprintf("assign: row %d: unknown tier %q", i, r.Tier))
		}
		key := assignKey{Action: r.Action, Class: r.Class, Risk: risk}
		if _, dup := out[key]; dup {
			return nil, deskkit.Refused(fmt.Sprintf(
				"assign: row %d: duplicate row for action=%q class=%q risk=%s — a duplicate is a silent overwrite, refused", i, r.Action, r.Class, r.Risk))
		}
		out[key] = tier
	}
	return out, nil
}

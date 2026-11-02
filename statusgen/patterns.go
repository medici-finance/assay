package main

// patterns.go — `statusgen patterns --lint [--root DIR]...` (graph-execution/02).
//
// WHY A SEPARATE SUBCOMMAND, NOT A `--lint` LEG. Like `conform` (conform.go), this
// validates a distinct artifact class — `spec/workflow-patterns/*.yaml` pattern
// files — against its own schema (schemas/workflow-pattern-v1.json) plus the
// cross-field MUST rules a schema cannot express. It reads no brief and mutates
// nothing, and it is a DIFFERENT surface from the board lint (methodology rules
// over briefs), so it is a subcommand rather than a `--lint` leg, exactly as
// conform.go's header explains for the schema-contract surface it owns.
//
// WHAT A SCHEMA CANNOT EXPRESS. `schemas/workflow-pattern-v1.json` covers shape:
// required keys, field types, closed value sets (see conform.go's minimal JSON
// Schema validator, reused here — parseSchema/validateValue). It cannot express
// the pattern's own resolved node graph: whether a non-`effect`-kind node's
// effect target is among its own outputs (needs each node's own output list),
// whether a node's effect kind is permitted for its role (needs the
// role→effect-kind table), whether a review node's role differs from every
// node that produced its inputs (needs edges resolved between
// inputs[].artifact and other nodes' outputs), whether `join` names a
// `check`-kind node (needs the node set looked up by id), or whether
// `risk-input` states all four verdicts (deliberately NOT a schema `required`
// list — see schemas/workflow-pattern-v1.json's risk-input description — so a
// missing verdict reports as a MUST-rule violation, not a shape violation). Those
// five rules are implemented here, in Go, over the parsed document.
//
// THREE-STATE, SAME CONTRACT AS conform/verifyrun/shardcheck. checked-clean (0) /
// checked-failed (1) / could-not-check (2), fail-closed: an unreadable file, a
// missing spec/workflow-patterns directory, or an unparseable embedded schema is
// could-not-check, never rounded up to clean.

import (
	"embed"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// embeddedPatternSchemaFS holds the committed workflow-pattern-v1 schema,
// compiled into the binary for the same reason conform.go embeds the brief
// schemas: a pinned release binary runs against an arbitrary --root, so there is
// no guarantee schemas/workflow-pattern-v1.json exists on disk at the root being
// scanned. TestEmbeddedPatternSchemaMatchesCommitted pins this copy identical to
// the canonical repo-root file.
//
//go:embed schemas/workflow-pattern-v1.json
var embeddedPatternSchemaFS embed.FS

const embeddedPatternSchemaName = "schemas/workflow-pattern-v1.json"

// patternSchemaMarker is the only `schema:` value this binary's pattern lint
// recognizes. A file with any other marker (or none) is could-not-check: a
// pattern file that does not declare its own schema cannot be validated against
// one.
const patternSchemaMarker = "workflow-pattern-v1"

// patterns exit codes — the same three-state contract, and the same numbers, as
// conform/verifyrun/shardcheck in this same binary.
const (
	patternsExitClean      = 0 // every pattern file validates
	patternsExitFailed     = 1 // at least one pattern file violates the schema or a MUST rule
	patternsExitCouldNot   = 2 // at least one file/root could not be checked (fail-closed)
	patternsExitUsageError = 2 // usage/refusal shares the could-not-check code
)

// Rule tags — the stable identities `statusgen enforcement-status` and Verify
// rows cite. Kept as consts, like the verifyrows.go / consumers.go rule tags,
// so a message and its registry entry cannot drift from each other by typo.
const (
	rulePatternHumanFloor          = "pattern-human-floor-output"
	rulePatternHumanRole           = "pattern-human-role-not-decision"
	rulePatternSchemaViolation     = "pattern-schema-violation"
	rulePatternEffectExceedsRole   = "pattern-effect-exceeds-role"
	rulePatternEffectTargetUnowned = "pattern-effect-target-not-owned"
	rulePatternReviewSameRole      = "pattern-review-same-role"
	rulePatternJoinNotCheck        = "pattern-join-not-check"
	rulePatternRiskInputMissing    = "pattern-risk-input-missing-verdict"

	// Signal-instantiated patterns (graph-execution/34, spec §4.3).
	rulePatternSignalInput      = "pattern-signal-input"
	rulePatternReplayMandatory  = "pattern-replay-not-mandatory"
	rulePatternAdmissionOpen    = "pattern-signal-admission-open"
	rulePatternRemedySetOpen    = "pattern-remedy-set-open"
	rulePatternJudgementDefault = "pattern-judgement-no-default"
	rulePatternCulpritRecord    = "pattern-culprit-record"
	rulePatternDecisionOptions  = "pattern-decision-options"
	rulePatternContainmentSet   = "pattern-containment-set"
)

// Closed sets of the signal-pattern rules (spec/workflow-pattern-v1.md §4.3).
// Each is a constant beside the rules that read it, never a pattern-supplied value.
const (
	// patternReplayArtifact is the reserved artifact a signal pattern MUST
	// produce under a mandatory command claim (pattern-replay-not-mandatory).
	patternReplayArtifact = "replay-fixture"
	// patternAdmittedPushRef is the only ref a `push` admission may name. It is
	// fully qualified so a tag named `main` can never satisfy it.
	patternAdmittedPushRef = "refs/heads/main"
	// patternGateConfigFloor is the gate-configuration floor every class of a
	// pattern with an effect-bearing node MUST refuse. It is a subset of gate
	// configuration: a runtime allow-list may refuse more, never less.
	patternGateConfigFloor = ".github/"
)

var (
	patternAdmissionTriggers = []string{"schedule", "push"}
	patternRemedySet         = []string{"gofmt", "stray-file-deletion", "exact-stale-string-replace"}
	patternContainmentSet    = []string{"quarantine", "rollback", "secondary-auth", "stop", "none-needed"}
	patternCulpritFields     = []string{"check", "file", "line", "problem", "class", "origin"}
)

// Human-floor artifacts correspond one-to-one to admission's humanFloorOperations:
// merge-commit ↔ merge, release ↔ release, deployment ↔ deploy. Add both sides
// in the same change when adding a floor operation; effect labels grant no exception.
const (
	patternFloorMerge   = "merge-commit"
	patternFloorRelease = "release"
	patternFloorDeploy  = "deployment"
)

func isHumanFloorArtifact(artifact string) bool {
	switch artifact {
	case patternFloorMerge, patternFloorRelease, patternFloorDeploy:
		return true
	}
	return false
}

// patternRoleEffectPermissions is `role → permitted effect kinds`
// (spec/workflow-pattern-v1.md §7), derived from what each `topology.yaml`
// `apps:` role does today. This is the ONE control `pattern-effect-exceeds-role`
// enforces — see that rule's single-point-of-failure note in
// docs/streams/graph-execution/brief-02-pattern-schema-and-node-contract.md
// §Context.
var patternRoleEffectPermissions = map[string]map[string]bool{
	"worker":   {"push": true, "pr-open": true, "comment": true},
	"reviewer": {"review": true, "comment": true},
	"verifier": {"evidence-commit": true, "comment": true},
	"desk":     {"file-issue": true, "comment": true, "dispatch": true},
}

// patternRiskVerdicts is the fixed four-value risk-class verdict set
// `risk-input` MUST map every one of (spec/workflow-pattern-v1.md §4.2 rule 4).
var patternRiskVerdicts = []string{"low", "standard", "elevated", "human"}

// embeddedPatternSchemaBytes returns the raw bytes of the embedded
// workflow-pattern-v1 schema.
func embeddedPatternSchemaBytes() []byte {
	b, err := embeddedPatternSchemaFS.ReadFile(embeddedPatternSchemaName)
	if err != nil {
		// A build that compiled has the file embedded; a read failure here is a
		// programming error, not a runtime condition.
		panic(fmt.Sprintf("statusgen: embedded %s unreadable: %v", embeddedPatternSchemaName, err))
	}
	return b
}

// runPatterns is the `statusgen patterns` entry point. It returns the process
// exit code. Today the only verb is --lint; an unrecognized invocation (no verb,
// or a future verb this build does not know) is a usage refusal.
func runPatterns(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("patterns", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var roots rootFlags
	flags.Var(&roots, "root", "repository root to scan (repeatable; default \".\")")
	lint := flags.Bool("lint", false, "validate every spec/workflow-patterns/*.yaml file against the schema and MUST rules")
	if err := flags.Parse(args); err != nil {
		return patternsExitUsageError
	}
	if !*lint {
		fmt.Fprintln(stderr, "statusgen patterns: no verb given (want --lint)")
		return patternsExitUsageError
	}

	resolvedRoots, err := resolveRoots(roots)
	if err != nil {
		fmt.Fprintln(stderr, "patterns:", err)
		return patternsExitUsageError
	}

	schema, err := parseSchema(embeddedPatternSchemaBytes())
	if err != nil {
		fmt.Fprintf(stderr, "patterns: embedded schema is not valid JSON: %v\n", err)
		return patternsExitCouldNot
	}

	var clean, failed, couldNot, scanned int
	for _, root := range resolvedRoots {
		dir := filepath.Join(root, "spec", "workflow-patterns")
		info, statErr := os.Stat(dir)
		if statErr != nil || !info.IsDir() {
			// A root with no spec/workflow-patterns directory is could-not-look, not
			// a clean pass — the three-state instrument rule forbids reading
			// "nothing scanned" as "nothing wrong".
			couldNot++
			fmt.Fprintf(stdout, "could-not-check: %s: no spec/workflow-patterns directory to scan (%v)\n", root, statErr)
			continue
		}
		entries, readErr := os.ReadDir(dir)
		if readErr != nil {
			couldNot++
			fmt.Fprintf(stdout, "could-not-check: %s: %v\n", dir, readErr)
			continue
		}
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
				continue
			}
			names = append(names, e.Name())
		}
		sort.Strings(names) // deterministic scan/report order
		for _, name := range names {
			scanned++
			path := filepath.Join(dir, name)
			state, violations := lintPatternFile(path, schema)
			switch state {
			case patternStateClean:
				clean++
			case patternStateFailed:
				failed++
				for _, v := range violations {
					fmt.Fprintf(stdout, "checked-failed: %s\n", v)
				}
			case patternStateCouldNot:
				couldNot++
				for _, v := range violations {
					fmt.Fprintf(stdout, "could-not-check: %s\n", v)
				}
			}
		}
	}

	fmt.Fprintf(stdout, "patterns: %d checked-clean, %d checked-failed, %d could-not-check (%d file(s) scanned)\n",
		clean, failed, couldNot, scanned)

	switch {
	case failed > 0:
		return patternsExitFailed
	case couldNot > 0:
		return patternsExitCouldNot
	default:
		return patternsExitClean
	}
}

type patternState int

const (
	patternStateClean patternState = iota
	patternStateFailed
	patternStateCouldNot
)

// lintPatternFile validates one spec/workflow-patterns/*.yaml file: first the
// schema (shape), then the MUST rules (§4.2/§5/§6 of
// spec/workflow-pattern-v1.md) over the resolved node graph. Schema violations
// and MUST-rule violations are reported as distinct messages, each tagged with
// its rule so `statusgen enforcement-status` and a reader can tell which class
// fired (spec/workflow-pattern-v1.md §8.2).
func lintPatternFile(path string, schema *schemaNode) (patternState, []string) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return patternStateCouldNot, []string{fmt.Sprintf("%s: %v", path, err)}
	}

	var data map[string]any
	if err := yaml.Unmarshal(raw, &data); err != nil {
		return patternStateCouldNot, []string{fmt.Sprintf("%s: %v", path, err)}
	}
	if data == nil {
		return patternStateCouldNot, []string{fmt.Sprintf("%s: empty document", path)}
	}

	marker, _ := data["schema"].(string)
	if marker != patternSchemaMarker {
		return patternStateCouldNot, []string{fmt.Sprintf(
			"%s: `schema:` is %q, not %q — a pattern file in spec/workflow-patterns/ MUST declare its own schema to be validated against one",
			path, marker, patternSchemaMarker)}
	}

	var violations []string

	// Shape first: the JSON-schema pass.
	for _, v := range validateValue(schema, data, "") {
		violations = append(violations, fmt.Sprintf("%s [%s]: %s", path, rulePatternSchemaViolation, v))
	}

	// MUST rules over the resolved graph. Run even when the shape failed above —
	// the graph rules operate defensively on whatever partial structure decoded
	// (missing fields simply resolve to zero values below), so a reader sees every
	// class of problem in one run rather than fixing shape, re-running, then
	// discovering a MUST-rule violation.
	pat, parseErr := parsePatternDoc(data)
	if parseErr != nil {
		violations = append(violations, fmt.Sprintf("%s: %v", path, parseErr))
	} else {
		violations = append(violations, checkPatternMustRules(path, pat)...)
	}

	if len(violations) > 0 {
		sort.Strings(violations)
		return patternStateFailed, violations
	}
	return patternStateClean, nil
}

// patternNode is the Go-side view of one `nodes[]` entry, resolved from the
// generic map[string]any the YAML decoder produces.
type patternNode struct {
	ID                string
	Kind              string
	Role              string
	Inputs            []string // inputs[].artifact, in document order
	Outputs           []string
	Effects           []patternEffect
	Evidence          []patternEvidence // graph-execution/03: the node's full evidence[] list, not just the review bit
	HasReviewEvidence bool              // true iff any evidence[] entry has kind: review

	// graph-execution/34. Inputs above stays artifact names only, so every
	// earlier rule and instance.go see what they saw before; the full input
	// items, signal inputs included, are carried separately here.
	InputItems     []patternInputItem
	Options        []string
	HasOptions     bool
	Default        string
	HasDefault     bool
	Containment    []string
	HasContainment bool
}

// patternInputItem is one `nodes[].inputs[]` entry by key presence, so the
// lint can tell an item naming neither or both of artifact/signal.
type patternInputItem struct {
	HasArtifact bool
	Signal      *patternSignal
}

// patternSignal is the `signal` shape of an input item (spec §4.3).
type patternSignal struct {
	Source        string
	LivenessCheck string
	Classes       []string
}

// patternSignalClass is one top-level `signal-classes[]` entry. Presence flags
// matter: several rules refuse a missing key, not an empty value.
type patternSignalClass struct {
	ID              string
	Exit            string
	MechanicalExit  string
	JudgementExit   string
	Admission       *patternAdmission
	Culprit         *patternCulprit
	Remedies        []string
	HasRemedies     bool
	RefusedPaths    []string
	HasRefusedPaths bool
}

type patternAdmission struct {
	Triggers    []string
	PushRefs    []string
	HasPushRefs bool
}

type patternCulprit struct {
	Fields          []string
	DefaultClass    string
	WithheldClass   string
	WithheldPresent bool
	FilePresent     bool
	FileIsNull      bool
}

type patternEdge struct {
	From string
	To   string
}

type patternEffect struct {
	Kind   string
	Target string
}

// patternEvidence is one `nodes[].evidence[]` entry — a claim the node owes.
// Kind is one of command | review | witness | observe (spec/workflow-pattern-v1.md
// §4.1). Signal/Band/Window/Source are populated ONLY for kind: observe
// (graph-execution/03 task 3) — the signal watched over a window after a change
// lands, and the source the coverage rule fills the claim FROM. They are "" for
// every other kind, by construction: a command/review/witness claim is resolved
// against Verify rows and forge state, never against a monitored source.
type patternEvidence struct {
	Kind      string
	Claim     string
	Mandatory bool
	Signal    string
	Band      string
	Window    string
	Source    string
}

// patternDoc is the Go-side view of the whole document, resolved once so the
// MUST-rule checks below never re-walk the raw map[string]any.
type patternDoc struct {
	Name      string // the `pattern:` field — the pattern's own name
	Nodes     []patternNode
	NodeByID  map[string]patternNode
	Join      string
	RiskInput map[string]bool // verdict name -> present (value not needed: presence is the rule)

	// graph-execution/34: the edges and signal classes the signal rules read.
	Edges         []patternEdge
	SignalClasses []patternSignalClass
}

func parsePatternDoc(data map[string]any) (patternDoc, error) {
	var doc patternDoc
	doc.NodeByID = map[string]patternNode{}
	doc.RiskInput = map[string]bool{}

	if name, ok := data["pattern"].(string); ok {
		doc.Name = name
	}
	if join, ok := data["join"].(string); ok {
		doc.Join = join
	}

	if ri, ok := data["risk-input"].(map[string]any); ok {
		for k := range ri {
			doc.RiskInput[k] = true
		}
	}

	rawNodes, _ := data["nodes"].([]any)
	for i, rn := range rawNodes {
		nm, ok := rn.(map[string]any)
		if !ok {
			return doc, fmt.Errorf("nodes[%d]: not a mapping", i)
		}
		n := patternNode{}
		n.ID, _ = nm["id"].(string)
		n.Kind, _ = nm["kind"].(string)
		n.Role, _ = nm["role"].(string)

		if rawInputs, ok := nm["inputs"].([]any); ok {
			for _, ri := range rawInputs {
				if im, ok := ri.(map[string]any); ok {
					if a, ok := im["artifact"].(string); ok && a != "" {
						n.Inputs = append(n.Inputs, a)
					}
					item := patternInputItem{}
					_, item.HasArtifact = im["artifact"]
					if rs, present := im["signal"]; present {
						sig := &patternSignal{}
						if sm, ok := rs.(map[string]any); ok {
							sig.Source, _ = sm["source"].(string)
							sig.LivenessCheck, _ = sm["liveness-check"].(string)
							sig.Classes = patternStringList(sm["classes"])
						}
						item.Signal = sig
					}
					n.InputItems = append(n.InputItems, item)
				}
			}
		}
		_, n.HasOptions = nm["options"]
		n.Options = patternStringList(nm["options"])
		_, n.HasDefault = nm["default"]
		n.Default, _ = nm["default"].(string)
		_, n.HasContainment = nm["containment"]
		n.Containment = patternStringList(nm["containment"])
		if rawOutputs, ok := nm["outputs"].([]any); ok {
			for _, ro := range rawOutputs {
				if s, ok := ro.(string); ok {
					n.Outputs = append(n.Outputs, s)
				}
			}
		}
		if rawEvidence, ok := nm["evidence"].([]any); ok {
			for _, re := range rawEvidence {
				if em, ok := re.(map[string]any); ok {
					ev := patternEvidence{}
					ev.Kind, _ = em["kind"].(string)
					ev.Claim, _ = em["claim"].(string)
					ev.Mandatory, _ = em["mandatory"].(bool)
					ev.Signal, _ = em["signal"].(string)
					ev.Band, _ = em["band"].(string)
					ev.Window, _ = em["window"].(string)
					ev.Source, _ = em["source"].(string)
					n.Evidence = append(n.Evidence, ev)
					if ev.Kind == "review" {
						n.HasReviewEvidence = true
					}
				}
			}
		}
		if rawEffects, ok := nm["effects"].([]any); ok {
			for _, re := range rawEffects {
				if em, ok := re.(map[string]any); ok {
					eff := patternEffect{}
					eff.Kind, _ = em["kind"].(string)
					eff.Target, _ = em["target"].(string)
					n.Effects = append(n.Effects, eff)
				}
			}
		}

		doc.Nodes = append(doc.Nodes, n)
		if n.ID != "" {
			doc.NodeByID[n.ID] = n
		}
	}

	if rawEdges, ok := data["edges"].([]any); ok {
		for _, re := range rawEdges {
			if em, ok := re.(map[string]any); ok {
				e := patternEdge{}
				e.From, _ = em["from"].(string)
				e.To, _ = em["to"].(string)
				doc.Edges = append(doc.Edges, e)
			}
		}
	}
	if rawClasses, ok := data["signal-classes"].([]any); ok {
		for _, rc := range rawClasses {
			cm, ok := rc.(map[string]any)
			if !ok {
				doc.SignalClasses = append(doc.SignalClasses, patternSignalClass{})
				continue
			}
			doc.SignalClasses = append(doc.SignalClasses, parseSignalClass(cm))
		}
	}
	return doc, nil
}

func parseSignalClass(cm map[string]any) patternSignalClass {
	c := patternSignalClass{}
	c.ID, _ = cm["id"].(string)
	c.Exit, _ = cm["exit"].(string)
	c.MechanicalExit, _ = cm["mechanical-exit"].(string)
	c.JudgementExit, _ = cm["judgement-exit"].(string)
	if ra, present := cm["admission"]; present {
		adm := &patternAdmission{}
		if am, ok := ra.(map[string]any); ok {
			adm.Triggers = patternStringList(am["triggers"])
			_, adm.HasPushRefs = am["push-refs"]
			adm.PushRefs = patternStringList(am["push-refs"])
		}
		c.Admission = adm
	}
	if rr, present := cm["culprit-record"]; present {
		cr := &patternCulprit{}
		if rm, ok := rr.(map[string]any); ok {
			cr.Fields = patternStringList(rm["fields"])
			cr.DefaultClass, _ = rm["default-class"].(string)
			if rw, present := rm["withheld"]; present {
				cr.WithheldPresent = true
				if wm, ok := rw.(map[string]any); ok {
					cr.WithheldClass, _ = wm["class"].(string)
					var file any
					file, cr.FilePresent = wm["file"]
					cr.FileIsNull = cr.FilePresent && file == nil
				}
			}
		}
		c.Culprit = cr
	}
	_, c.HasRemedies = cm["remedies"]
	c.Remedies = patternStringList(cm["remedies"])
	_, c.HasRefusedPaths = cm["refused-paths"]
	c.RefusedPaths = patternStringList(cm["refused-paths"])
	return c
}

// patternStringList returns the string entries of a decoded YAML list, in order;
// anything else (absent, a scalar, a mapping) yields nil. Non-string entries
// are kept as their %v text so a membership rule still sees and refuses them.
func patternStringList(v any) []string {
	raw, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, e := range raw {
		if s, ok := e.(string); ok {
			out = append(out, s)
		} else {
			out = append(out, fmt.Sprintf("%v", e))
		}
	}
	return out
}

// checkPatternMustRules runs the cross-field MUST rules a JSON schema cannot
// express, over the resolved document. Each returned string names the
// rule tag so it reads identically whether printed by the CLI or scraped by a
// test.
func checkPatternMustRules(path string, pat patternDoc) []string {
	var out []string

	// Rule: risk-input maps every one of the four verdicts
	// (spec/workflow-pattern-v1.md §4.2 rule 4).
	var missing []string
	for _, verdict := range patternRiskVerdicts {
		if !pat.RiskInput[verdict] {
			missing = append(missing, verdict)
		}
	}
	if len(missing) > 0 {
		out = append(out, fmt.Sprintf("%s [%s]: risk-input is missing verdict(s) %s — every one of %s MUST be mapped",
			path, rulePatternRiskInputMissing, strings.Join(missing, ", "), strings.Join(patternRiskVerdicts, "/")))
	}

	// Rule: join names a check-kind node (§4.2 rule 5 / §6).
	if pat.Join == "" {
		out = append(out, fmt.Sprintf("%s [%s]: join is empty — a pattern MUST name its integration-check node", path, rulePatternJoinNotCheck))
	} else if joinNode, ok := pat.NodeByID[pat.Join]; !ok {
		out = append(out, fmt.Sprintf("%s [%s]: join names %q, which is not a node in this pattern", path, rulePatternJoinNotCheck, pat.Join))
	} else if joinNode.Kind != "check" {
		out = append(out, fmt.Sprintf("%s [%s]: join names %q, whose kind is %q, not %q — the integration check MUST be a check-kind node",
			path, rulePatternJoinNotCheck, pat.Join, joinNode.Kind, "check"))
	}

	for _, n := range pat.Nodes {
		nodeLabel := n.ID
		if nodeLabel == "" {
			nodeLabel = "(unnamed node)"
		}

		// Human ownership is a decision boundary, never an effect permission.
		if n.Role == "human" && (n.Kind != "decision" || len(n.Effects) != 0) {
			out = append(out, fmt.Sprintf("%s [%s]: node %q with role human MUST be an effect-free decision", path, rulePatternHumanRole, nodeLabel))
		}
		for _, artifact := range n.Outputs {
			if isHumanFloorArtifact(artifact) && (n.Kind != "decision" || n.Role != "human") {
				out = append(out, fmt.Sprintf("%s [%s]: node %q outputs %q and MUST be a human decision", path, rulePatternHumanFloor, nodeLabel, artifact))
			}
		}
		for _, eff := range n.Effects {
			if isHumanFloorArtifact(eff.Target) {
				out = append(out, fmt.Sprintf("%s [%s]: node %q declares effect %q targeting human-floor artifact %q", path, rulePatternHumanFloor, nodeLabel, eff.Kind, eff.Target))
			}
		}

		// Rule: a node with effects is kind: effect, or every effect's target is
		// among its own outputs (§4.2 rule 2).
		if n.Kind != "effect" {
			owned := map[string]bool{}
			for _, o := range n.Outputs {
				owned[o] = true
			}
			for _, eff := range n.Effects {
				if eff.Target != "" && !owned[eff.Target] {
					out = append(out, fmt.Sprintf(
						"%s [%s]: node %q (kind %q) declares an effect targeting %q, which is not among its own outputs %v",
						path, rulePatternEffectTargetUnowned, nodeLabel, n.Kind, eff.Target, n.Outputs))
				}
			}
		}

		// Rule: a node's effect kind MUST be permitted for its role
		// (pattern-effect-exceeds-role — spec/workflow-pattern-v1.md §7).
		permitted, roleKnown := patternRoleEffectPermissions[n.Role]
		for _, eff := range n.Effects {
			if eff.Kind == "" {
				continue
			}
			if !roleKnown || !permitted[eff.Kind] {
				out = append(out, fmt.Sprintf(
					"%s [%s]: node %q has role %q, which is not permitted to perform effect kind %q",
					path, rulePatternEffectExceedsRole, nodeLabel, n.Role, eff.Kind))
			}
		}

		// Rule: a review node's role differs from every node that produced its
		// inputs (pattern-review-same-role — §4.2 rule 3).
		if n.HasReviewEvidence {
			for _, inputArtifact := range n.Inputs {
				producer, foundProducer := findProducerOf(pat.Nodes, inputArtifact, n.ID)
				if !foundProducer {
					continue // artifact produced outside this pattern (e.g. the brief itself) — nothing to compare
				}
				if producer.Role == n.Role {
					out = append(out, fmt.Sprintf(
						"%s [%s]: review node %q has role %q, the same role as %q, which produced its input %q — a review node MUST NOT share the role of whoever it is reviewing",
						path, rulePatternReviewSameRole, nodeLabel, n.Role, producer.ID, inputArtifact))
				}
			}
		}
	}

	out = append(out, checkSignalPatternRules(path, pat)...)
	return out
}

// checkSignalPatternRules runs the eight signal-pattern MUST rules
// (spec/workflow-pattern-v1.md §4.3). Admission and the gate-configuration
// floor key on the PRESENCE of an effect-bearing node anywhere in the pattern
// (non-empty effects, any kind, reachable or not), never on a class's exit, a
// node's kind or a route, so neither depends on a runtime enforcing routes.
func checkSignalPatternRules(path string, pat patternDoc) []string {
	var out []string
	add := func(rule, format string, args ...any) {
		out = append(out, fmt.Sprintf("%s [%s]: ", path, rule)+fmt.Sprintf(format, args...))
	}

	incoming := map[string]bool{}
	for _, e := range pat.Edges {
		incoming[e.To] = true
	}
	var signalNodes []string
	var signalInputs []*patternSignal
	for _, n := range pat.Nodes {
		for _, item := range n.InputItems {
			hasSignal := item.Signal != nil
			if item.HasArtifact == hasSignal {
				add(rulePatternSignalInput, "node %q has an input item naming %s of artifact/signal — it MUST name exactly one",
					nodeLabelOf(n), map[bool]string{true: "both", false: "neither"}[hasSignal])
			}
			if !hasSignal {
				continue
			}
			signalNodes = append(signalNodes, n.ID)
			signalInputs = append(signalInputs, item.Signal)
			if incoming[n.ID] {
				add(rulePatternSignalInput, "node %q carries a signal input but an edge points at it — a signal input MUST sit on an entry node", nodeLabelOf(n))
			}
			if strings.TrimSpace(item.Signal.Source) == "" {
				add(rulePatternSignalInput, "node %q has a signal input with an empty source", nodeLabelOf(n))
			}
			if strings.TrimSpace(item.Signal.LivenessCheck) == "" {
				add(rulePatternSignalInput, "node %q has a signal input with an empty liveness-check — it MUST state how a dead source is detected", nodeLabelOf(n))
			}
		}
	}
	isSignal := len(signalInputs) > 0

	declared := map[string]int{}
	for _, c := range pat.SignalClasses {
		declared[c.ID]++
	}
	for _, id := range sortedKeys(declared) {
		if count := declared[id]; count > 1 {
			add(rulePatternSignalInput, "signal class %q is declared %d times — each class id MUST be declared once", id, count)
		}
	}
	if isSignal && len(pat.SignalClasses) == 0 {
		add(rulePatternSignalInput, "the pattern has a signal input but declares no signal-classes")
	}
	if len(pat.SignalClasses) > 0 && len(signalInputs) != 1 {
		add(rulePatternSignalInput, "the pattern declares signal-classes and has %d signal input(s) — it MUST have exactly one", len(signalInputs))
	}
	for _, sig := range signalInputs {
		listed := map[string]int{}
		for _, name := range sig.Classes {
			listed[name]++
		}
		for _, name := range sortedKeys(listed) {
			if listed[name] > 1 {
				add(rulePatternSignalInput, "the signal input lists class %q %d times — each class MUST be listed once", name, listed[name])
			}
			if declared[name] == 0 {
				add(rulePatternSignalInput, "the signal input lists class %q, which no signal-classes entry declares", name)
			}
		}
		for _, c := range pat.SignalClasses {
			if listed[c.ID] == 0 {
				add(rulePatternSignalInput, "signal class %q is declared but the signal input does not list it — every declared class MUST be listed", c.ID)
			}
		}
	}
	if isSignal {
		reached := reachableFrom(signalNodes, pat.Edges)
		for _, n := range pat.Nodes {
			if !reached[n.ID] {
				add(rulePatternSignalInput, "node %q is not reachable along edges from the signal-input node — a signal pattern MUST have no disconnected node", nodeLabelOf(n))
			}
		}
	}

	// pattern-replay-not-mandatory: a signal pattern produces the reserved
	// replay artifact, and every producer owes it under a mandatory command claim.
	if isSignal {
		producers := 0
		for _, n := range pat.Nodes {
			if !containsString(n.Outputs, patternReplayArtifact) {
				continue
			}
			producers++
			mandatoryCommand := false
			for _, ev := range n.Evidence {
				if ev.Kind == "command" && ev.Mandatory {
					mandatoryCommand = true
				}
			}
			if !mandatoryCommand {
				add(rulePatternReplayMandatory, "node %q outputs %s but carries no kind: command evidence with mandatory: true", nodeLabelOf(n), patternReplayArtifact)
			}
		}
		if producers == 0 {
			add(rulePatternReplayMandatory, "the pattern has a signal input but no node outputs the reserved artifact %s", patternReplayArtifact)
		}
	}

	effectBearing := ""
	for _, n := range pat.Nodes {
		if len(n.Effects) > 0 {
			effectBearing = nodeLabelOf(n)
			break
		}
	}

	for _, c := range pat.SignalClasses {
		cl := c.ID
		if cl == "" {
			cl = "(unnamed class)"
		}

		// pattern-signal-input: exits are nodes, the exit shape is one of the two,
		// and the mechanical exit and culprit record travel together.
		for _, ref := range [][2]string{{"exit", c.Exit}, {"mechanical-exit", c.MechanicalExit}, {"judgement-exit", c.JudgementExit}} {
			if ref[1] == "" {
				continue
			}
			if _, ok := pat.NodeByID[ref[1]]; !ok {
				add(rulePatternSignalInput, "signal class %q names %s %q, which is not a node in this pattern", cl, ref[0], ref[1])
			}
		}
		pair := c.MechanicalExit != "" && c.JudgementExit != ""
		if (c.Exit != "") == (c.MechanicalExit != "" || c.JudgementExit != "") || (c.Exit == "" && !pair) {
			add(rulePatternSignalInput, "signal class %q MUST name either exit alone, or both mechanical-exit and judgement-exit", cl)
		}
		if (c.MechanicalExit != "") != (c.Culprit != nil) {
			add(rulePatternSignalInput, "signal class %q: mechanical-exit and culprit-record MUST appear together", cl)
		}
		if c.MechanicalExit != "" || c.Culprit != nil {
			var missingKeys []string
			if c.JudgementExit == "" {
				missingKeys = append(missingKeys, "judgement-exit")
			}
			if c.Admission == nil {
				missingKeys = append(missingKeys, "admission")
			}
			if !c.HasRemedies {
				missingKeys = append(missingKeys, "remedies")
			}
			if !c.HasRefusedPaths {
				missingKeys = append(missingKeys, "refused-paths")
			}
			if len(missingKeys) > 0 {
				add(rulePatternSignalInput, "signal class %q carries a mechanical exit or culprit record but omits %s", cl, strings.Join(missingKeys, ", "))
			}
		}

		// pattern-signal-admission-open.
		if c.Admission != nil {
			for _, t := range c.Admission.Triggers {
				if !containsString(patternAdmissionTriggers, t) {
					add(rulePatternAdmissionOpen, "signal class %q admits trigger %q — triggers MUST be a subset of %s", cl, t, strings.Join(patternAdmissionTriggers, ", "))
				}
			}
			if containsString(c.Admission.Triggers, "push") {
				if !c.Admission.HasPushRefs || len(c.Admission.PushRefs) != 1 || c.Admission.PushRefs[0] != patternAdmittedPushRef {
					add(rulePatternAdmissionOpen, "signal class %q admits push with push-refs %v — push-refs MUST be exactly [%s]", cl, c.Admission.PushRefs, patternAdmittedPushRef)
				}
			}
		}
		if effectBearing != "" && c.Admission == nil {
			add(rulePatternAdmissionOpen, "signal class %q declares no admission, and node %q declares effects — every class MUST declare admission once any node is effect-bearing", cl, effectBearing)
		}

		// pattern-remedy-set-open.
		for _, r := range c.Remedies {
			if !containsString(patternRemedySet, r) {
				add(rulePatternRemedySetOpen, "signal class %q lists remedy %q — remedies MUST be a subset of %s", cl, r, strings.Join(patternRemedySet, ", "))
			}
		}
		if c.MechanicalExit != "" {
			if len(c.Remedies) == 0 {
				add(rulePatternRemedySetOpen, "signal class %q names a mechanical-exit with no remedies — remedies MUST be a non-empty subset of %s", cl, strings.Join(patternRemedySet, ", "))
			}
			if mn, ok := pat.NodeByID[c.MechanicalExit]; ok && mn.Kind != "effect" {
				add(rulePatternRemedySetOpen, "signal class %q: mechanical-exit %q is kind %q — it MUST be kind effect", cl, c.MechanicalExit, mn.Kind)
			}
		}
		if effectBearing != "" && !containsString(c.RefusedPaths, patternGateConfigFloor) {
			add(rulePatternRemedySetOpen, "signal class %q refused-paths %v omits the gate-configuration floor %s, and node %q declares effects — every class MUST refuse it once any node is effect-bearing",
				cl, c.RefusedPaths, patternGateConfigFloor, effectBearing)
		}

		// pattern-judgement-no-default.
		if c.JudgementExit != "" {
			if jn, ok := pat.NodeByID[c.JudgementExit]; ok {
				if jn.Kind != "decision" || jn.Role != "human" {
					add(rulePatternJudgementDefault, "signal class %q: judgement-exit %q is kind %q role %q — it MUST be a human decision", cl, c.JudgementExit, jn.Kind, jn.Role)
				}
				if len(jn.Options) == 0 {
					add(rulePatternJudgementDefault, "signal class %q: judgement-exit %q declares no options", cl, c.JudgementExit)
				}
				if !jn.HasDefault || !containsString(jn.Options, jn.Default) {
					add(rulePatternJudgementDefault, "signal class %q: judgement-exit %q MUST declare a default that is one of its options", cl, c.JudgementExit)
				}
			}
		}

		// pattern-culprit-record.
		if cr := c.Culprit; cr != nil {
			if !sameStringSet(cr.Fields, patternCulpritFields) {
				add(rulePatternCulpritRecord, "signal class %q: culprit-record fields %v MUST be exactly %s, each once", cl, cr.Fields, strings.Join(patternCulpritFields, ", "))
			}
			if cr.DefaultClass != "judgement" {
				add(rulePatternCulpritRecord, "signal class %q: culprit-record default-class is %q — it MUST be judgement", cl, cr.DefaultClass)
			}
			if !cr.WithheldPresent || cr.WithheldClass != "judgement" {
				add(rulePatternCulpritRecord, "signal class %q: culprit-record withheld.class is %q — it MUST be judgement", cl, cr.WithheldClass)
			}
			if !cr.FileIsNull {
				add(rulePatternCulpritRecord, "signal class %q: culprit-record withheld.file MUST be YAML null", cl)
			}
		}
	}

	containmentNodes := 0
	for _, n := range pat.Nodes {
		label := nodeLabelOf(n)

		// pattern-decision-options.
		if (n.HasOptions || n.HasDefault || n.HasContainment) && n.Kind != "decision" {
			add(rulePatternDecisionOptions, "node %q is kind %q — options, default and containment MUST appear only on decision nodes", label, n.Kind)
		}
		if n.HasDefault && !containsString(n.Options, n.Default) {
			add(rulePatternDecisionOptions, "node %q default %q is not one of its options %v", label, n.Default, n.Options)
		}
		seen := map[string]bool{}
		for _, o := range n.Options {
			if seen[o] {
				add(rulePatternDecisionOptions, "node %q lists option %q more than once", label, o)
			}
			seen[o] = true
		}

		// pattern-containment-set.
		if !n.HasContainment {
			continue
		}
		containmentNodes++
		if !sameStringSet(n.Containment, patternContainmentSet) {
			add(rulePatternContainmentSet, "node %q containment %v MUST be exactly %s, each once", label, n.Containment, strings.Join(patternContainmentSet, ", "))
		}
		if n.Kind != "decision" || n.Role != "human" || len(n.Effects) != 0 {
			add(rulePatternContainmentSet, "node %q carries containment and MUST be an effect-free human decision (kind %q, role %q, %d effect(s))", label, n.Kind, n.Role, len(n.Effects))
		}
		if n.HasDefault {
			add(rulePatternContainmentSet, "node %q carries containment and a default %q — no containment outcome may be pre-selected", label, n.Default)
		}
	}
	if isSignal && containmentNodes != 1 {
		add(rulePatternContainmentSet, "the pattern has a signal input and %d containment node(s) — it MUST have exactly one", containmentNodes)
	}

	return out
}

func nodeLabelOf(n patternNode) string {
	if n.ID == "" {
		return "(unnamed node)"
	}
	return n.ID
}

// reachableFrom returns every node id reachable along edges from roots,
// roots included.
func reachableFrom(roots []string, edges []patternEdge) map[string]bool {
	next := map[string][]string{}
	for _, e := range edges {
		next[e.From] = append(next[e.From], e.To)
	}
	seen := map[string]bool{}
	queue := append([]string(nil), roots...)
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		if seen[id] {
			continue
		}
		seen[id] = true
		queue = append(queue, next[id]...)
	}
	return seen
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// sameStringSet reports whether got holds exactly the entries of want, each once.
func sameStringSet(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	seen := map[string]bool{}
	for _, g := range got {
		if seen[g] || !containsString(want, g) {
			return false
		}
		seen[g] = true
	}
	return true
}

// findProducerOf returns the node (other than selfID) whose outputs include
// artifact, if any. When more than one node claims the same output name the
// first in document order wins — pattern files are small and reviewed, so this
// is a deterministic, good-enough resolution rather than a modeled ambiguity.
func findProducerOf(nodes []patternNode, artifact, selfID string) (patternNode, bool) {
	for _, n := range nodes {
		if n.ID == selfID {
			continue
		}
		for _, o := range n.Outputs {
			if o == artifact {
				return n, true
			}
		}
	}
	return patternNode{}, false
}

// loadPatternDocs reads every spec/workflow-patterns/*.yaml file under root and
// returns the parsed documents keyed by their own `pattern:` name.
//
// graph-execution/03's coverage rule reuses this rather than re-implementing a
// second YAML walk: the SAME parse `patterns --lint` already validates is what
// coverage's pattern-mandatory-evidence union (Task item 1(b)) and join check
// (Task item 2) read. A file this cannot parse is DROPPED from the map (not
// panicked on) — `patterns --lint` is the authority on whether a pattern file is
// well-formed; coverage's job is to compute claims for briefs bound to a NAMED
// pattern, and a binding naming a pattern absent from this map resolves to "no
// pattern" (fact (a) alone), never a crash. ok is false only when the directory
// itself could not be read (fail-closed, matching runPatterns' own could-not-check
// treatment of a missing/unreadable spec/workflow-patterns directory).
func loadPatternDocs(root string) (map[string]patternDoc, bool) {
	out := map[string]patternDoc{}
	dir := filepath.Join(root, "spec", "workflow-patterns")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return out, false
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var data map[string]any
		if err := yaml.Unmarshal(raw, &data); err != nil || data == nil {
			continue
		}
		if marker, _ := data["schema"].(string); marker != patternSchemaMarker {
			continue
		}
		pat, err := parsePatternDoc(data)
		if err != nil || pat.Name == "" {
			continue
		}
		out[pat.Name] = pat
	}
	return out, true
}

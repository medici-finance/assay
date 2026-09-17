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
// the pattern's own resolved node graph: whether a node's effect kind is
// permitted for its role (needs the role→effect-kind table), whether a review
// node's role differs from every node that produced its inputs (needs edges
// resolved between inputs[].artifact and other nodes' outputs), whether `join`
// names a `check`-kind node (needs the node set looked up by id), or whether
// `risk-input` states all four verdicts (deliberately NOT a schema `required`
// list — see schemas/workflow-pattern-v1.json's risk-input description — so a
// missing verdict reports as a MUST-rule violation, not a shape violation). Those
// four rules are implemented here, in Go, over the parsed document.
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
	rulePatternSchemaViolation     = "pattern-schema-violation"
	rulePatternEffectExceedsRole   = "pattern-effect-exceeds-role"
	rulePatternEffectTargetUnowned = "pattern-effect-target-not-owned"
	rulePatternReviewSameRole      = "pattern-review-same-role"
	rulePatternJoinNotCheck        = "pattern-join-not-check"
	rulePatternRiskInputMissing    = "pattern-risk-input-missing-verdict"
)

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
	HasReviewEvidence bool // true iff any evidence[] entry has kind: review
}

type patternEffect struct {
	Kind   string
	Target string
}

// patternDoc is the Go-side view of the whole document, resolved once so the
// MUST-rule checks below never re-walk the raw map[string]any.
type patternDoc struct {
	Nodes     []patternNode
	NodeByID  map[string]patternNode
	Join      string
	RiskInput map[string]bool // verdict name -> present (value not needed: presence is the rule)
}

func parsePatternDoc(data map[string]any) (patternDoc, error) {
	var doc patternDoc
	doc.NodeByID = map[string]patternNode{}
	doc.RiskInput = map[string]bool{}

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
				}
			}
		}
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
					if k, _ := em["kind"].(string); k == "review" {
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
	return doc, nil
}

// checkPatternMustRules runs the four cross-field MUST rules a JSON schema
// cannot express, over the resolved document. Each returned string names the
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

	return out
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

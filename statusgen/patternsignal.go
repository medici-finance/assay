package main

// patternsignal.go — the signal-instantiated pattern rules (graph-execution/08).
//
// A signal pattern starts from a red gate, an alert or a band crossing rather than
// an authored item. Its first node declares `signal`; the rules below apply only
// to a pattern that declares one, so the authored-item patterns (implementation,
// research) are checked exactly as before.
//
// Containment is the one place a worker node may name an action outside §7's role
// table: the closed taxonomy quarantine | rollback | secondary-auth | stop, and
// only on the node that receives the signal. The role table itself is unchanged,
// so an instance still cannot carry a containment effect (instance.go checks
// capabilities against that table) and the run-time roster binding still refuses
// it. This file only lets the reviewed pattern declare the choice.

import (
	"fmt"
	"path"
	"strings"
)

const (
	rulePatternFixtureOptional  = "pattern-fixture-not-mandatory"
	rulePatternSignalRoute      = "pattern-signal-route-invalid"
	rulePatternGateRedAdmission = "pattern-gate-red-admission"
	rulePatternGateRedMissing   = "pattern-gate-red-incomplete"
	rulePatternRemedySetOpen    = "pattern-remedy-set-open"
	rulePatternJudgementDefault = "pattern-judgement-no-default"
	rulePatternWithheldClass    = "pattern-withheld-not-judgement"
)

// patternContainmentSteps is the closed containment taxonomy. The schema's
// containment.steps enum states the same set; this copy is the lint's own.
var patternContainmentSteps = map[string]bool{
	"quarantine": true, "rollback": true, "secondary-auth": true, "stop": true,
}

// patternContainmentRoles are the roles that may declare containment, on the
// signal node only.
var patternContainmentRoles = map[string]bool{"worker": true}

// patternRemedySet is the mechanical exit's closed remedy set.
var patternRemedySet = map[string]bool{
	"gofmt": true, "stray-file-deletion": true, "exact-stale-string-replace": true,
}

// patternGateConfigFloor is the deny-list floor: gate-configuration paths every
// remedies.refused-paths list must cover. Matching is case-insensitive, because a
// case-insensitive filesystem resolves `.GitHub/workflows` to the same directory.
var patternGateConfigFloor = []string{".github", ".claude"}

type patternSignal struct {
	Classes   []string
	Admission []patternAdmit // admission.gate-red
}

type patternAdmit struct {
	Trigger string
	Ref     string
}

type patternCulprit struct {
	Withheld []patternWithheld
}

type patternWithheld struct {
	Gate    string
	Class   string
	File    string
	HasFile bool // file is a non-null value
}

type patternRemedies struct {
	Set       []patternRemedy
	Refused   []string
	Otherwise string
}

type patternRemedy struct {
	Kind string
	Path string
}

func (s *patternSignal) hasClass(c string) bool {
	for _, x := range s.Classes {
		if x == c {
			return true
		}
	}
	return false
}

func patternStrings(v any) []string {
	var out []string
	if xs, ok := v.([]any); ok {
		for _, x := range xs {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}
	}
	return out
}

// parseSignalKeys reads the graph-execution/08 keys of one node mapping.
func parseSignalKeys(nm map[string]any, n *patternNode) {
	if sm, ok := nm["signal"].(map[string]any); ok {
		sig := &patternSignal{Classes: patternStrings(sm["classes"])}
		if am, ok := sm["admission"].(map[string]any); ok {
			for _, e := range anyList(am["gate-red"]) {
				em, _ := e.(map[string]any)
				a := patternAdmit{}
				a.Trigger, _ = em["trigger"].(string)
				a.Ref, _ = em["ref"].(string)
				sig.Admission = append(sig.Admission, a)
			}
		}
		n.Signal = sig
	}
	if cm, ok := nm["containment"].(map[string]any); ok {
		n.Containment = patternStrings(cm["steps"])
	}
	if rm, ok := nm["routes"].(map[string]any); ok {
		n.Routes = map[string]string{}
		for k, v := range rm {
			s, _ := v.(string)
			n.Routes[k] = s
		}
	}
	if cm, ok := nm["culprit"].(map[string]any); ok {
		c := &patternCulprit{}
		for _, e := range anyList(cm["withheld"]) {
			em, _ := e.(map[string]any)
			w := patternWithheld{}
			w.Gate, _ = em["gate"].(string)
			w.Class, _ = em["class"].(string)
			if fv := em["file"]; fv != nil {
				w.HasFile = true
				w.File = fmt.Sprint(fv)
			}
			c.Withheld = append(c.Withheld, w)
		}
		n.Culprit = c
	}
	if rm, ok := nm["remedies"].(map[string]any); ok {
		r := &patternRemedies{Refused: patternStrings(rm["refused-paths"])}
		r.Otherwise, _ = rm["otherwise"].(string)
		for _, e := range anyList(rm["set"]) {
			em, _ := e.(map[string]any)
			x := patternRemedy{}
			x.Kind, _ = em["kind"].(string)
			x.Path, _ = em["path"].(string)
			r.Set = append(r.Set, x)
		}
		n.Remedies = r
	}
	n.Default, _ = nm["default"].(string)
}

func anyList(v any) []any {
	xs, _ := v.([]any)
	return xs
}

// checkContainmentSteps is pattern-effect-exceeds-role's containment leg: a
// containment step is permitted only to a containment role, on an effect node
// that declares `signal`, and only from the closed taxonomy.
func checkContainmentSteps(file, label string, n patternNode) []string {
	var out []string
	scoped := n.Signal != nil && n.Kind == "effect" && patternContainmentRoles[n.Role]
	for _, step := range n.Containment {
		if !scoped || !patternContainmentSteps[step] {
			out = append(out, fmt.Sprintf(
				"%s [%s]: node %q has role %q, which is not permitted containment step %q — containment is the closed taxonomy, on a worker effect node that declares signal",
				file, rulePatternEffectExceedsRole, label, n.Role, step))
		}
	}
	return out
}

// canonicalRepoPath cleans a repo-relative path for prefix matching. ok is false
// for a path that is absolute or escapes the repository.
func canonicalRepoPath(p string) (string, bool) {
	p = strings.ReplaceAll(strings.TrimSpace(p), `\`, "/")
	if p == "" || strings.HasPrefix(p, "/") || (len(p) > 1 && p[1] == ':') {
		return "", false
	}
	c := path.Clean(p)
	if c == ".." || strings.HasPrefix(c, "../") {
		return "", false
	}
	return strings.ToLower(c), true
}

// underPrefix reports whether canonical path c is prefix or lies beneath it.
func underPrefix(c, prefix string) bool {
	return c == prefix || strings.HasPrefix(c, prefix+"/")
}

// checkSignalRules runs the graph-execution/08 rules. A pattern with no node
// declaring `signal` and no signal key anywhere gets none of them.
func checkSignalRules(file string, pat patternDoc) []string {
	var out []string
	add := func(rule, format string, args ...any) {
		out = append(out, fmt.Sprintf("%s [%s]: ", file, rule)+fmt.Sprintf(format, args...))
	}

	var sig *patternSignal
	for i, n := range pat.Nodes {
		if n.Signal == nil {
			continue
		}
		if i != 0 || sig != nil {
			add(rulePatternSignalRoute, "node %q declares signal; only the pattern's first node may", n.ID)
			continue
		}
		sig = n.Signal
		hasInput := false
		for _, in := range n.Inputs {
			hasInput = hasInput || in == "signal"
		}
		if !hasInput {
			add(rulePatternSignalRoute, "node %q declares signal but does not take the signal input", n.ID)
		}
	}

	// Remedy sets and judgement routes are checked wherever they appear.
	routes := map[string]string{}
	for _, n := range pat.Nodes {
		if n.Routes != nil && n.Kind != "decision" {
			add(rulePatternSignalRoute, "node %q declares routes but is kind %q, not a decision", n.ID, n.Kind)
		}
		for class, target := range n.Routes {
			if _, ok := pat.NodeByID[target]; !ok {
				add(rulePatternSignalRoute, "node %q routes %s to %q, which is not a node", n.ID, class, target)
			}
			routes[class] = target
		}
		if n.Remedies != nil {
			out = append(out, checkRemedies(file, n, pat)...)
		}
	}

	if target, ok := routes["outage"]; ok {
		if t := pat.NodeByID[target]; t.Kind != "decision" || t.Role != "human" {
			add(rulePatternSignalRoute, "the outage route names %q, which is not a human decision — an outage exit is a mitigation left for a human", target)
		}
	}
	if target, ok := routes["judgement"]; ok {
		out = append(out, checkJudgementNode(file, "the judgement route", target, pat)...)
	}

	if sig == nil {
		return out
	}

	for _, class := range []string{"outage", "regression"} {
		if sig.hasClass(class) && routes[class] == "" {
			add(rulePatternSignalRoute, "signal declares class %s but no decision node routes it", class)
		}
	}

	// The replay fixture is mandatory on every artifact node of a signal pattern.
	for _, n := range pat.Nodes {
		if n.Kind != "artifact" {
			continue
		}
		mandatory := false
		for _, ev := range n.Evidence {
			mandatory = mandatory || (ev.Kind == "command" && ev.Mandatory)
		}
		if !mandatory {
			add(rulePatternFixtureOptional, "artifact node %q owes no mandatory command evidence — a signal pattern's root-cause node MUST carry its replay fixture as mandatory command evidence", n.ID)
		}
	}

	if !sig.hasClass("gate-red") {
		return out
	}
	out = append(out, checkGateRedAdmission(file, sig)...)

	var culprit, remedies bool
	for _, n := range pat.Nodes {
		if n.Culprit != nil {
			culprit = true
			for _, w := range n.Culprit.Withheld {
				if w.Class != "judgement" || w.HasFile {
					file := "null"
					if w.HasFile {
						file = fmt.Sprintf("%q", w.File)
					}
					add(rulePatternWithheldClass, "node %q classes withheld-detail gate %q as %q with file %s — a gate that withholds its detail is always judgement with file: null", n.ID, w.Gate, w.Class, file)
				}
			}
		}
		remedies = remedies || n.Remedies != nil
	}
	missing := map[string]bool{
		"a culprit record": !culprit, "a remedies set": !remedies, "top-level limits": !pat.HasLimits,
		"a mechanical route": routes["mechanical"] == "",
	}
	for _, what := range []string{"a culprit record", "a remedies set", "top-level limits", "a mechanical route"} {
		if missing[what] {
			add(rulePatternGateRedMissing, "signal declares class gate-red but the pattern has no %s", what)
		}
	}
	if routes["judgement"] == "" {
		add(rulePatternJudgementDefault, "signal declares class gate-red but no decision node routes judgement to a human decision with a default")
	}
	return out
}

// checkGateRedAdmission: only a scheduled run, or a push whose ref is main.
func checkGateRedAdmission(file string, sig *patternSignal) []string {
	var out []string
	if len(sig.Admission) == 0 {
		out = append(out, fmt.Sprintf("%s [%s]: signal declares class gate-red but admits no run — admission.gate-red MUST list schedule and/or push to main", file, rulePatternGateRedAdmission))
	}
	for _, a := range sig.Admission {
		ok := (a.Trigger == "schedule" && a.Ref == "") ||
			(a.Trigger == "push" && (a.Ref == "main" || a.Ref == "refs/heads/main"))
		if !ok {
			out = append(out, fmt.Sprintf("%s [%s]: gate-red admits trigger %q ref %q — only schedule, or push to main, is admitted; any other run's log is contributor-authored", file, rulePatternGateRedAdmission, a.Trigger, a.Ref))
		}
	}
	return out
}

// checkRemedies: the closed remedy set, the gate-configuration floor, and a
// refused remedy's route to a human decision with a default.
func checkRemedies(file string, n patternNode, pat patternDoc) []string {
	var out []string
	add := func(format string, args ...any) {
		out = append(out, fmt.Sprintf("%s [%s]: node %q ", file, rulePatternRemedySetOpen, n.ID)+fmt.Sprintf(format, args...))
	}
	var refused []string
	for _, r := range n.Remedies.Refused {
		if c, ok := canonicalRepoPath(r); ok {
			refused = append(refused, c)
		}
	}
	for _, floor := range patternGateConfigFloor {
		covered := false
		for _, r := range refused {
			covered = covered || underPrefix(floor, r)
		}
		if !covered {
			add("does not refuse gate-configuration path %s/ — refused-paths MUST cover the floor %v", floor, patternGateConfigFloor)
		}
	}
	for _, r := range n.Remedies.Set {
		if !patternRemedySet[r.Kind] {
			add("names remedy %q, outside the closed set gofmt | stray-file-deletion | exact-stale-string-replace", r.Kind)
		}
		if r.Path == "" {
			continue // the culprit file, resolved at run time
		}
		c, ok := canonicalRepoPath(r.Path)
		if !ok {
			add("names remedy path %q, which is absolute or leaves the repository", r.Path)
			continue
		}
		for _, floor := range append(append([]string(nil), patternGateConfigFloor...), refused...) {
			if underPrefix(c, floor) {
				add("names remedy path %q, under gate-configuration path %s/ — such a remedy routes to the judgement exit", r.Path, floor)
				break
			}
		}
	}
	out = append(out, checkJudgementNode(file, fmt.Sprintf("node %q remedies.otherwise", n.ID), n.Remedies.Otherwise, pat)...)
	return out
}

// checkJudgementNode: the named node is a human decision carrying a default.
func checkJudgementNode(file, what, target string, pat patternDoc) []string {
	t, ok := pat.NodeByID[target]
	if ok && t.Kind == "decision" && t.Role == "human" && strings.TrimSpace(t.Default) != "" {
		return nil
	}
	return []string{fmt.Sprintf("%s [%s]: %s names %q, which is not a human decision carrying a recommended default", file, rulePatternJudgementDefault, what, target)}
}

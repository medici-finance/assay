package main

// runtimeresource.go — `statusgen runtime-resource --lint --adapters FILE
// [--as-of RFC3339] PATH...` (sdlc/24, docs/runtime-adapters.md).
//
// WHAT THIS CHECKS. A runtime resource record (schemas/runtime-resource-v1.json)
// describes one stateful resource an agent run used — a database branch, a
// snapshot, an eval corpus, an object-storage prefix, a disposable app backend —
// together with the ordered list of external effects performed on it and the
// typed receipt each effect produced. The schema pass covers shape. The rules in
// this file cover what shape cannot: the record resolved against an adapter
// registry and against its own effect list, in order.
//
// The three rules the contract stands on are:
//
//   - OWNERSHIP IS NOT PROVISIONING. An agent may request a bounded, ephemeral
//     resource through the control plane. Only a human or organization principal
//     may claim it — retain it, promote it, or pay for it past its TTL. A claim
//     effect authorized by anything else fails `runtime-resource-claim-without-
//     principal`, whatever else the record says.
//   - SUCCESS IS NEVER INFERRED FROM ABSENCE. An effect whose acknowledgment was
//     lost has outcome `unknown`. Before the run does anything else, a reconcile
//     effect must read the resource's real state from the adapter and record it
//     as a receipt. An unknown effect with no resolving reconcile, or any effect
//     issued while one is pending, fails `runtime-resource-unreconciled-effect`.
//   - A RELEASE IS DONE WHEN A RECEIPT SAYS SO. A succeeded release still needs a
//     later reconcile that observed the resource absent; until then the release
//     is a request, not a fact (`runtime-resource-unreconciled-release`).
//
// WHY A SEPARATE SUBCOMMAND. Like `conform` and `patterns`, this validates a
// distinct artifact class against its own schema and rules, reads no brief, and
// mutates nothing, so it is a subcommand rather than a board `--lint` leg.
//
// OFFLINE BY CONSTRUCTION. The validator reads the record files and the adapter
// registry file it is given and nothing else: it never contacts an adapter, a
// cluster, or a vendor API. The receipts it reasons over are the ones the control
// plane already recorded. That is what makes the contract implementable — and
// testable — against a local mock adapter.
//
// THREE-STATE, SAME CONTRACT AS conform/patterns. checked-clean (0) /
// checked-failed (1) / could-not-check (2), fail-closed: an unreadable record, an
// unreadable or invalid adapter registry, or an unparseable embedded schema is
// could-not-check, never rounded up to clean.

import (
	"embed"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// embeddedRuntimeResourceSchemaFS holds the committed runtime-resource-v1
// schema, compiled in for the reason patterns.go gives: a pinned release binary
// runs against arbitrary files, with no guarantee the schema exists on disk.
// TestRuntimeResourceContractSchemaParity pins this copy identical to the
// canonical repo-root file.
//
//go:embed schemas/runtime-resource-v1.json
var embeddedRuntimeResourceSchemaFS embed.FS

const embeddedRuntimeResourceSchemaName = "schemas/runtime-resource-v1.json"

// The only `schema:` markers this build recognizes, for a record and for the
// adapter registry. A file declaring any other marker is could-not-check.
const (
	runtimeResourceSchemaMarker = "runtime-resource-v1"
	runtimeAdaptersSchemaMarker = "runtime-adapters-v1"
)

const (
	runtimeResourceExitClean      = 0
	runtimeResourceExitFailed     = 1
	runtimeResourceExitCouldNot   = 2
	runtimeResourceExitUsageError = 2 // usage/refusal shares the could-not-check code
)

// Rule tags, kept as consts so a message and the docs that cite it cannot drift.
const (
	ruleRRSchemaViolation       = "runtime-resource-schema-violation"
	ruleRRUnknownAdapter        = "runtime-resource-unknown-adapter"
	ruleRRTTLExceedsBound       = "runtime-resource-ttl-exceeds-bound"
	ruleRRExpired               = "runtime-resource-expired"
	ruleRRClaimWithoutPrincipal = "runtime-resource-claim-without-principal"
	ruleRRUnreconciledRelease   = "runtime-resource-unreconciled-release"
	ruleRRUnreconciledEffect    = "runtime-resource-unreconciled-effect"
	ruleRRReceiptMissing        = "runtime-resource-receipt-missing"
	ruleRRStateUnsupported      = "runtime-resource-state-unsupported"
	ruleRRCredentialNotRotated  = "runtime-resource-credential-not-rotated"
	ruleRRCredentialOutlivesTTL = "runtime-resource-credential-outlives-ttl"
)

// runtimeResourceClasses and runtimeResourceVerbs are the closed vocabularies of
// docs/runtime-adapters.md. TestRuntimeResourceContractVocabulary pins them equal
// to the schema's `class` and `effects[].verb` enums.
var runtimeResourceClasses = []string{"database-branch", "snapshot", "eval-corpus", "object-prefix", "app-backend"}

var runtimeResourceVerbs = []string{"create", "inspect", "snapshot", "restore", "claim", "release", "reconcile"}

// runtimeAdapterRequiredVerbs are the verbs every registered adapter must
// implement: without create there is no resource, without inspect and reconcile
// a lost acknowledgment cannot be resolved, and without release nothing is ever
// cleaned up.
var runtimeAdapterRequiredVerbs = []string{"create", "inspect", "release", "reconcile"}

var runtimeIDRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// runtimeDurationRe caps the count at five digits, in step with the schema's
// `ttl` and `lifetime` patterns: 99999d is about 273 years, inside what a
// time.Duration can hold, so no accepted value can overflow. parseRuntimeDuration
// checks the multiplication anyway, so a pattern loosened later still refuses.
var runtimeDurationRe = regexp.MustCompile(`^([1-9][0-9]{0,4})([smhd])$`)

// runtimeRegistryTopKeys and runtimeRegistryAdapterKeys are the registry's
// closed key sets. The registry is the trust anchor: a key it does not know (a
// misspelt `max_ttl`) would otherwise read exactly like a key left out on
// purpose, and silently remove a ceiling.
var (
	runtimeRegistryTopKeys     = []string{"schema", "adapters"}
	runtimeRegistryAdapterKeys = []string{"id", "classes", "verbs", "max-ttl"}
)

func embeddedRuntimeResourceSchemaBytes() []byte {
	b, err := embeddedRuntimeResourceSchemaFS.ReadFile(embeddedRuntimeResourceSchemaName)
	if err != nil {
		panic(fmt.Sprintf("statusgen: embedded %s unreadable: %v", embeddedRuntimeResourceSchemaName, err))
	}
	return b
}

// runRuntimeResource is the `statusgen runtime-resource` entry point; it returns
// the process exit code.
func runRuntimeResource(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("runtime-resource", flag.ContinueOnError)
	flags.SetOutput(stderr)
	lint := flags.Bool("lint", false, "validate runtime-resource-v1 records against the schema and the contract rules")
	adaptersPath := flags.String("adapters", "", "adapter registry file (runtime-adapters-v1); required")
	asOfFlag := flags.String("as-of", "", "evaluate TTL expiry at this RFC3339 instant (default: now)")
	if err := flags.Parse(args); err != nil {
		return runtimeResourceExitUsageError
	}
	if !*lint {
		fmt.Fprintln(stderr, "statusgen runtime-resource: no verb given (want --lint)")
		return runtimeResourceExitUsageError
	}
	if *adaptersPath == "" {
		fmt.Fprintln(stderr, "statusgen runtime-resource: --adapters is required — a record is checked against a registry, never against itself")
		return runtimeResourceExitUsageError
	}
	if flags.NArg() == 0 {
		fmt.Fprintln(stderr, "statusgen runtime-resource: no record file or directory given")
		return runtimeResourceExitUsageError
	}
	asOf := time.Now().UTC()
	if *asOfFlag != "" {
		t, err := time.Parse(time.RFC3339, *asOfFlag)
		if err != nil {
			fmt.Fprintf(stderr, "statusgen runtime-resource: --as-of %q is not RFC3339: %v\n", *asOfFlag, err)
			return runtimeResourceExitUsageError
		}
		asOf = t
	}

	schema, err := parseSchema(embeddedRuntimeResourceSchemaBytes())
	if err != nil {
		fmt.Fprintf(stdout, "could-not-check: embedded schema is not valid JSON: %v\n", err)
		return runtimeResourceExitCouldNot
	}
	adapters, err := loadRuntimeAdapters(*adaptersPath)
	if err != nil {
		// Without a registry no record can be checked; reporting each one clean
		// would trust every adapter on its own say-so.
		fmt.Fprintf(stdout, "could-not-check: %s: %v\n", *adaptersPath, err)
		return runtimeResourceExitCouldNot
	}

	var files []string
	var clean, failed, couldNot int
	for _, arg := range flags.Args() {
		info, statErr := os.Stat(arg)
		if statErr != nil {
			couldNot++
			fmt.Fprintf(stdout, "could-not-check: %s: %v\n", arg, statErr)
			continue
		}
		if !info.IsDir() {
			files = append(files, arg)
			continue
		}
		// A directory is walked whole, and both YAML extensions are read: a
		// record the scan skips is a resource nobody checked, so the scan does
		// not get to choose which ones it looks at.
		var found int
		walkErr := filepath.WalkDir(arg, func(p string, e fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if e.IsDir() || !rrIsYAMLName(e.Name()) {
				return nil
			}
			files = append(files, p)
			found++
			return nil
		})
		if walkErr != nil {
			couldNot++
			fmt.Fprintf(stdout, "could-not-check: %s: %v\n", arg, walkErr)
			continue
		}
		if found == 0 {
			// Nothing scanned is not nothing wrong.
			couldNot++
			fmt.Fprintf(stdout, "could-not-check: %s: no *.yaml or *.yml record under the directory\n", arg)
		}
	}
	sort.Strings(files)

	for _, path := range files {
		state, lines := lintRuntimeResourceFile(path, schema, adapters, asOf)
		switch state {
		case patternStateClean:
			clean++
		case patternStateFailed:
			failed++
			for _, l := range lines {
				fmt.Fprintf(stdout, "checked-failed: %s\n", l)
			}
		case patternStateCouldNot:
			couldNot++
			for _, l := range lines {
				fmt.Fprintf(stdout, "could-not-check: %s\n", l)
			}
		}
	}

	fmt.Fprintf(stdout, "runtime-resource: %d checked-clean, %d checked-failed, %d could-not-check (%d file(s) scanned, as of %s)\n",
		clean, failed, couldNot, len(files), asOf.Format(time.RFC3339))
	switch {
	case failed > 0:
		return runtimeResourceExitFailed
	case couldNot > 0:
		return runtimeResourceExitCouldNot
	default:
		return runtimeResourceExitClean
	}
}

// runtimeAdapter is one registry entry: the classes it can hold, the verbs it
// implements, and an optional ceiling on the TTL a resource may be requested for.
type runtimeAdapter struct {
	ID      string
	Classes map[string]bool
	Verbs   map[string]bool
	MaxTTL  time.Duration // 0 = no ceiling declared
}

// loadRuntimeAdapters reads and validates the adapter registry. Any defect is an
// error (the caller's could-not-check): a registry that cannot be trusted cannot
// vouch for any record.
func loadRuntimeAdapters(path string) (map[string]runtimeAdapter, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	decoded, err := rrDecodeSingleDocument(raw)
	if err != nil {
		return nil, err
	}
	if decoded == nil {
		return nil, fmt.Errorf("empty adapter registry")
	}
	data, ok := decoded.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("adapter registry is not a mapping")
	}
	if err := rrRefuseUnknownKeys("adapter registry", data, runtimeRegistryTopKeys); err != nil {
		return nil, err
	}
	if marker, _ := data["schema"].(string); marker != runtimeAdaptersSchemaMarker {
		return nil, fmt.Errorf("`schema:` is %q, not %q", marker, runtimeAdaptersSchemaMarker)
	}
	list, _ := data["adapters"].([]any)
	if len(list) == 0 {
		return nil, fmt.Errorf("adapter registry lists no adapters")
	}
	knownClass := rrSetOf(runtimeResourceClasses)
	knownVerb := rrSetOf(runtimeResourceVerbs)
	out := map[string]runtimeAdapter{}
	for i, item := range list {
		m, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("adapters[%d]: not a mapping", i)
		}
		if err := rrRefuseUnknownKeys(fmt.Sprintf("adapters[%d]", i), m, runtimeRegistryAdapterKeys); err != nil {
			return nil, err
		}
		id, _ := m["id"].(string)
		if !runtimeIDRe.MatchString(id) {
			return nil, fmt.Errorf("adapters[%d]: id %q does not match %s", i, id, runtimeIDRe)
		}
		if _, dup := out[id]; dup {
			return nil, fmt.Errorf("adapters[%d]: id %q is registered twice", i, id)
		}
		a := runtimeAdapter{ID: id, Classes: map[string]bool{}, Verbs: map[string]bool{}}
		for _, c := range rrStringList(m["classes"]) {
			if !knownClass[c] {
				return nil, fmt.Errorf("adapter %q: unknown class %q", id, c)
			}
			a.Classes[c] = true
		}
		if len(a.Classes) == 0 {
			return nil, fmt.Errorf("adapter %q: lists no classes", id)
		}
		for _, v := range rrStringList(m["verbs"]) {
			if !knownVerb[v] {
				return nil, fmt.Errorf("adapter %q: unknown verb %q", id, v)
			}
			a.Verbs[v] = true
		}
		for _, v := range runtimeAdapterRequiredVerbs {
			if !a.Verbs[v] {
				return nil, fmt.Errorf("adapter %q: does not implement required verb %q", id, v)
			}
		}
		if mt, present := m["max-ttl"]; present {
			s, _ := mt.(string)
			d, ok := parseRuntimeDuration(s)
			if !ok {
				return nil, fmt.Errorf("adapter %q: max-ttl %q is not a duration of 1-5 digits and one unit s/m/h/d, like 4h — a ceiling the validator cannot read is never treated as no ceiling", id, fmt.Sprint(mt))
			}
			a.MaxTTL = d
		}
		out[id] = a
	}
	return out, nil
}

// lintRuntimeResourceFile validates one record: schema first, then the contract
// rules. Both always run, so a reader sees every class of problem in one pass.
func lintRuntimeResourceFile(path string, schema *schemaNode, adapters map[string]runtimeAdapter, asOf time.Time) (patternState, []string) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return patternStateCouldNot, []string{fmt.Sprintf("%s: %v", path, err)}
	}
	decodedAny, err := rrDecodeSingleDocument(raw)
	if err != nil {
		return patternStateCouldNot, []string{fmt.Sprintf("%s: %v", path, err)}
	}
	if decodedAny == nil {
		return patternStateCouldNot, []string{fmt.Sprintf("%s: empty document", path)}
	}
	decoded, ok := decodedAny.(map[string]any)
	if !ok {
		return patternStateCouldNot, []string{fmt.Sprintf("%s: the document is not a mapping", path)}
	}
	data, _ := normalizeYAMLTimes(decoded).(map[string]any)
	if marker, _ := data["schema"].(string); marker != runtimeResourceSchemaMarker {
		return patternStateCouldNot, []string{fmt.Sprintf(
			"%s: `schema:` is %q, not %q — a record must declare its own schema to be validated against one",
			path, marker, runtimeResourceSchemaMarker)}
	}

	var violations []string
	for _, v := range validateValue(schema, data, "") {
		violations = append(violations, fmt.Sprintf("%s [%s]: %s", path, ruleRRSchemaViolation, v))
	}
	doc := parseRuntimeResource(data)
	violations = append(violations, checkRuntimeResourceRules(path, doc, adapters, asOf)...)
	if len(violations) > 0 {
		sort.Strings(violations)
		return patternStateFailed, violations
	}
	return patternStateClean, nil
}

// normalizeYAMLTimes turns any timestamp the YAML decoder resolved to time.Time
// back into its RFC3339 string, so an unquoted timestamp validates the same as a
// quoted one.
func normalizeYAMLTimes(v any) any {
	switch t := v.(type) {
	case time.Time:
		return t.UTC().Format(time.RFC3339Nano)
	case map[string]any:
		for k, sub := range t {
			t[k] = normalizeYAMLTimes(sub)
		}
		return t
	case []any:
		for i, sub := range t {
			t[i] = normalizeYAMLTimes(sub)
		}
		return t
	}
	return v
}

type runtimeEffect struct {
	Index         int
	ID            string
	Verb          string
	Outcome       string
	Principal     string
	Reconciles    string
	RequestedAt   runtimeTimeField
	HasReceipt    bool
	ObservedAt    runtimeTimeField
	ObservedState string
}

type runtimeRotation struct {
	After  string
	Action string
	At     runtimeTimeField
}

// runtimeTimeField is a timestamp the rules read. Set says the key was present;
// OK says it parsed to a real instant. A field that is Set but not OK is one the
// schema's shape check let through (a month 13, a day 31 in a 30-day month) and
// the rules cannot evaluate, which is a violation, never a rule that holds.
type runtimeTimeField struct {
	Raw string
	Set bool
	At  time.Time
	OK  bool
}

// runtimeDurationField is the same three-way reading for a duration.
type runtimeDurationField struct {
	Raw string
	Set bool
	D   time.Duration
	OK  bool
}

type runtimeResourceDoc struct {
	ID          string
	Class       string
	AdapterID   string
	State       string
	TTL         runtimeDurationField
	CredLife    runtimeDurationField
	HasOwner    bool
	OwnerPrinc  string
	ClaimedAt   runtimeTimeField
	RetainUntil runtimeTimeField
	Rotations   []runtimeRotation
	Effects     []runtimeEffect
}

func rrReadTime(m map[string]any, key string) runtimeTimeField {
	v, set := m[key]
	if !set {
		return runtimeTimeField{}
	}
	f := runtimeTimeField{Raw: fmt.Sprint(v), Set: true}
	if s, ok := v.(string); ok {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			f.At, f.OK = t, true
		}
	}
	return f
}

func rrReadDuration(m map[string]any, key string) runtimeDurationField {
	v, set := m[key]
	if !set {
		return runtimeDurationField{}
	}
	f := runtimeDurationField{Raw: fmt.Sprint(v), Set: true}
	if s, ok := v.(string); ok {
		f.D, f.OK = parseRuntimeDuration(s)
	}
	return f
}

// parseRuntimeResource resolves the generic decoded map defensively: a missing
// or mistyped field becomes a zero value, which the schema pass has already
// reported, so the rules below never panic on a malformed record. An effect item
// that is not a mapping is dropped, so an effect's Index is its position in
// d.Effects, never its raw list position: every rule that indexes d.Effects by
// it reads the slice it was taken from.
func parseRuntimeResource(data map[string]any) runtimeResourceDoc {
	var d runtimeResourceDoc
	d.ID, _ = data["id"].(string)
	d.Class, _ = data["class"].(string)
	d.State, _ = data["state"].(string)
	if a, ok := data["adapter"].(map[string]any); ok {
		d.AdapterID, _ = a["id"].(string)
	}
	d.TTL = rrReadDuration(data, "ttl")
	if c, ok := data["credentials"].(map[string]any); ok {
		if ac, ok := c["agent-credential"].(map[string]any); ok {
			d.CredLife = rrReadDuration(ac, "lifetime")
		}
		rots, _ := c["rotations"].([]any)
		for _, r := range rots {
			if rm, ok := r.(map[string]any); ok {
				after, _ := rm["after"].(string)
				action, _ := rm["action"].(string)
				d.Rotations = append(d.Rotations, runtimeRotation{After: after, Action: action, At: rrReadTime(rm, "at")})
			}
		}
	}
	if o, ok := data["ownership"].(map[string]any); ok {
		d.HasOwner = true
		d.OwnerPrinc, _ = o["principal"].(string)
		d.ClaimedAt = rrReadTime(o, "claimed-at")
		d.RetainUntil = rrReadTime(o, "retain-until")
	}
	effects, _ := data["effects"].([]any)
	for _, e := range effects {
		m, ok := e.(map[string]any)
		if !ok {
			continue
		}
		var eff runtimeEffect
		eff.Index = len(d.Effects)
		eff.ID, _ = m["id"].(string)
		eff.Verb, _ = m["verb"].(string)
		eff.Outcome, _ = m["outcome"].(string)
		eff.Principal, _ = m["principal"].(string)
		eff.Reconciles, _ = m["reconciles"].(string)
		eff.RequestedAt = rrReadTime(m, "requested-at")
		if r, ok := m["receipt"].(map[string]any); ok {
			eff.HasReceipt = true
			eff.ObservedState, _ = r["observed-state"].(string)
			eff.ObservedAt = rrReadTime(r, "observed-at")
		}
		d.Effects = append(d.Effects, eff)
	}
	return d
}

// runtimeDerived is what the receipts establish, folded over the effect list IN
// ORDER, so the latest authoritative observation wins. A receipt observing the
// resource present after a reconciled release means it is live again; an
// observation of absent after a create means it is gone, whatever came before.
type runtimeDerived struct {
	everPresent   bool      // some authoritative receipt has observed the resource present
	present       bool      // the latest authoritative observation is present
	lifetimeStart time.Time // observed-at of the observation that began the current lifetime
	claimed       bool      // a succeeded claim with a receipt in the current lifetime
	released      bool      // the latest authoritative observation is a reconciled release observing absent
	everReleased  bool      // some release was reconciled to absent
	createFailed  bool      // a create failed, or was reconciled absent, and nothing since observed it present
	// lifetime[i] numbers the lifetime in force once effect i is read: 0 before
	// any observation of present, and one more each time the resource is
	// observed present again after evidence that it ended — a reconciled release
	// observing it absent, or a create or restore receipt observing it present
	// after an absence. An inspect that finds nothing is not that evidence, so an
	// absent/present pair of inspects keeps the lifetime. Rule 6 scopes ownership
	// to it, so a claim and its release protection end with the lifetime they
	// were made in.
	lifetime []int
}

// deriveRuntimeState folds the effects in order. The authoritative observations
// are the receipts of create, inspect, snapshot, restore and claim effects, and
// the receipt of a reconcile that resolves an earlier effect. A release's own
// receipt is not one: it says the adapter accepted the request, and only a
// later reconcile says the resource is gone (docs/runtime-adapters.md, Recovery
// rule 4).
func deriveRuntimeState(d runtimeResourceDoc, pos map[string]int) runtimeDerived {
	dv := runtimeDerived{lifetime: make([]int, len(d.Effects))}
	cur := 0
	// recreates is true for the receipt of an effect that re-creates the
	// resource (create, restore), and for a reconcile that resolves one whose
	// acknowledgment was lost: both are evidence the resource came back.
	observe := func(state string, at time.Time, recreates bool) {
		switch state {
		case "present":
			// regression: any absence followed by any present began a new
			// lifetime, so an agent's inspect gap reset the TTL anchor and
			// dropped a human's claim. Only evidence that the resource ended and
			// came back begins one; otherwise the lifetime, its claim and its
			// TTL anchor continue.
			if !dv.present && (!dv.everPresent || recreates || dv.released) {
				// A new lifetime: its TTL counts from here, and a claim from an
				// earlier lifetime does not carry over.
				dv.lifetimeStart, dv.claimed = at, false
				cur++
			}
			dv.present, dv.everPresent = true, true
			dv.released, dv.createFailed = false, false
		case "absent":
			dv.present = false
		}
	}
	for i, e := range d.Effects {
		switch e.Verb {
		case "reconcile":
			j, ok := pos[e.Reconciles]
			if !ok || j >= e.Index || e.Outcome != "succeeded" || !e.HasReceipt {
				dv.lifetime[i] = cur
				continue
			}
			// regression: a create whose answer was lost, once reconciled
			// present, is the same physical event as an acknowledged create, so
			// it begins a lifetime after an absence too (Recovery rule 3).
			tv := d.Effects[j].Verb
			observe(e.ObservedState, e.ObservedAt.At, tv == "create" || tv == "restore")
			if e.ObservedState == "absent" {
				switch d.Effects[j].Verb {
				case "create":
					dv.createFailed = true
				case "release":
					dv.released, dv.everReleased = true, true
				}
			}
		case "release":
			// An acknowledgment, not an observation; see the function comment.
		case "create":
			if e.Outcome == "failed" && !dv.present {
				dv.createFailed = true
			}
			if e.Outcome != "unknown" && e.HasReceipt {
				observe(e.ObservedState, e.ObservedAt.At, true)
			}
		case "claim":
			if e.Outcome == "succeeded" && e.HasReceipt {
				observe(e.ObservedState, e.ObservedAt.At, false)
				if dv.present {
					dv.claimed = true
				}
			}
		default: // inspect, snapshot, restore
			if e.Outcome != "unknown" && e.HasReceipt {
				observe(e.ObservedState, e.ObservedAt.At, e.Verb == "restore")
			}
		}
		dv.lifetime[i] = cur
	}
	return dv
}

// checkRuntimeResourceRules applies the contract rules a schema cannot express.
// Every record-controlled string reaches a message through %q, so a record that
// already fails cannot add lines of its own to the output.
func checkRuntimeResourceRules(path string, d runtimeResourceDoc, adapters map[string]runtimeAdapter, asOf time.Time) []string {
	var out []string
	add := func(rule, format string, args ...any) {
		out = append(out, fmt.Sprintf("%s [%s]: %s", path, rule, fmt.Sprintf(format, args...)))
	}

	// 0. Every bound the rules read must be one they can evaluate. A value the
	// schema's shape check let through but that does not parse to an in-range
	// duration or a real instant would otherwise switch its rule off.
	for _, f := range []struct {
		name string
		v    runtimeDurationField
	}{{"ttl", d.TTL}, {"credentials.agent-credential.lifetime", d.CredLife}} {
		if f.v.Set && !f.v.OK {
			add(ruleRRSchemaViolation, "%s %q is not a duration the rules can evaluate (1-5 digits and one unit, s/m/h/d) — a bound that cannot be read is a violation, never a bound that holds", f.name, f.v.Raw)
		}
	}
	if d.RetainUntil.Set && !d.RetainUntil.OK {
		add(ruleRRSchemaViolation, "ownership.retain-until %q is not a real RFC3339 instant — a retention that cannot be read is a violation, never one that holds", d.RetainUntil.Raw)
	}
	if d.ClaimedAt.Set && !d.ClaimedAt.OK {
		add(ruleRRSchemaViolation, "ownership.claimed-at %q is not a real RFC3339 instant", d.ClaimedAt.Raw)
	}
	for _, r := range d.Rotations {
		if r.At.Set && !r.At.OK {
			add(ruleRRSchemaViolation, "rotation after effect %q: at %q is not a real RFC3339 instant", r.After, r.At.Raw)
		}
	}
	// Effects are listed in the order they were issued; every ordered rule
	// below reads the list, so a list whose request times run backwards is
	// one whose order cannot be trusted.
	var prev, prevObs runtimeEffect
	for _, e := range d.Effects {
		if e.RequestedAt.Set && !e.RequestedAt.OK {
			add(ruleRRSchemaViolation, "effect %q: requested-at %q is not a real RFC3339 instant", e.ID, e.RequestedAt.Raw)
		}
		if e.RequestedAt.OK && prev.RequestedAt.OK && e.RequestedAt.At.Before(prev.RequestedAt.At) {
			add(ruleRRStateUnsupported, "effect %q: requested at %s, before the effect listed ahead of it (%q, %s) — effects are listed in the order they were issued", e.ID, e.RequestedAt.At.Format(time.RFC3339), prev.ID, prev.RequestedAt.At.Format(time.RFC3339))
		}
		if e.RequestedAt.OK {
			prev = e
		}
		if e.ObservedAt.Set && !e.ObservedAt.OK {
			add(ruleRRSchemaViolation, "effect %q: receipt observed-at %q is not a real RFC3339 instant — a receipt time that cannot be read cannot anchor a ttl", e.ID, e.ObservedAt.Raw)
		}
		// A receipt is the adapter's statement of what it saw, so it cannot be
		// dated after the instant the record is read at, nor before its effect
		// was requested. Either would move the TTL anchor the expiry rule uses.
		if e.ObservedAt.OK && e.ObservedAt.At.After(asOf) {
			add(ruleRRStateUnsupported, "effect %q: receipt is dated %s, after the as-of instant %s — a receipt cannot observe the future", e.ID, e.ObservedAt.At.Format(time.RFC3339), asOf.Format(time.RFC3339))
		}
		if e.ObservedAt.OK && e.RequestedAt.OK && e.ObservedAt.At.Before(e.RequestedAt.At) {
			add(ruleRRStateUnsupported, "effect %q: receipt is dated %s, before the effect was requested at %s", e.ID, e.ObservedAt.At.Format(time.RFC3339), e.RequestedAt.At.Format(time.RFC3339))
		}
		// The state rules read the receipts in list order and take the last as
		// the latest observation, so a receipt dated before one listed ahead
		// of it makes "latest" ambiguous: it is refused, never guessed.
		if e.ObservedAt.OK && prevObs.ObservedAt.OK && e.ObservedAt.At.Before(prevObs.ObservedAt.At) {
			add(ruleRRStateUnsupported, "effect %q: receipt is dated %s, before the receipt of %q listed ahead of it (%s) — receipts are read in list order, so one dated out of that order leaves the latest observation ambiguous", e.ID, e.ObservedAt.At.Format(time.RFC3339), prevObs.ID, prevObs.ObservedAt.At.Format(time.RFC3339))
		}
		if e.ObservedAt.OK && (!prevObs.ObservedAt.OK || e.ObservedAt.At.After(prevObs.ObservedAt.At)) {
			prevObs = e
		}
	}

	// 1. The adapter must be registered, for this class and for every verb used.
	if a, ok := adapters[d.AdapterID]; !ok {
		add(ruleRRUnknownAdapter, "adapter %q is not in the adapter registry — an adapter is trusted because it is registered, never on a record's own say-so", d.AdapterID)
	} else {
		if d.Class != "" && !a.Classes[d.Class] {
			add(ruleRRUnknownAdapter, "adapter %q is not registered for class %q", d.AdapterID, d.Class)
		}
		for _, e := range d.Effects {
			if e.Verb != "" && !a.Verbs[e.Verb] {
				add(ruleRRUnknownAdapter, "effect %q: adapter %q does not implement verb %q", e.ID, d.AdapterID, e.Verb)
			}
		}
		if a.MaxTTL > 0 && d.TTL.OK && d.TTL.D > a.MaxTTL {
			add(ruleRRTTLExceedsBound, "ttl %s exceeds adapter %q's max-ttl %s — an ephemeral resource is requested within the adapter's bound; retaining it longer is a claim", d.TTL.D, d.AdapterID, a.MaxTTL)
		}
	}

	// 2. The agent's credential never outlives the resource it is scoped to.
	if d.TTL.OK && d.CredLife.OK && d.CredLife.D > d.TTL.D {
		add(ruleRRCredentialOutlivesTTL, "agent credential lifetime %s exceeds the resource ttl %s", d.CredLife.D, d.TTL.D)
	}

	// 3. Effect ids are stable and unique; reconciles point backwards.
	pos := map[string]int{}
	for _, e := range d.Effects {
		if e.ID == "" {
			continue
		}
		if _, dup := pos[e.ID]; dup {
			add(ruleRRSchemaViolation, "effect id %q appears more than once — effect ids are stable and unique within a record", e.ID)
			continue
		}
		pos[e.ID] = e.Index
	}

	// 4. Receipts, and reconcile-before-retry over the ordered effect list.
	pending := ""
	pendingVerb := ""
	for _, e := range d.Effects {
		if e.Outcome == "succeeded" && !e.HasReceipt {
			add(ruleRRReceiptMissing, "effect %q (%q) is recorded succeeded with no receipt — an acknowledged effect carries the adapter's typed receipt", e.ID, e.Verb)
		}
		resolving := false
		if e.Verb == "reconcile" {
			if e.Reconciles == "" {
				add(ruleRRUnreconciledEffect, "reconcile effect %q names no effect it reconciles", e.ID)
			} else if j, ok := pos[e.Reconciles]; !ok || j >= e.Index {
				add(ruleRRUnreconciledEffect, "reconcile effect %q reconciles %q, which is not an earlier effect in this record", e.ID, e.Reconciles)
			} else if e.Outcome == "succeeded" && e.HasReceipt {
				resolving = true
			}
		}
		if pending != "" {
			switch {
			case e.Verb == "reconcile" && e.Reconciles == pending && resolving:
				pending = ""
			case e.Verb == "reconcile" && e.Reconciles == pending:
				// A reconcile attempt that itself failed or lost its answer leaves
				// the effect pending; retrying the reconcile is the correct move.
			default:
				add(ruleRRUnreconciledEffect, "effect %q (%q) was issued while effect %q (%q) had outcome unknown — the unknown effect must be reconciled against a receipt before the run does anything else", e.ID, e.Verb, pending, pendingVerb)
			}
			continue
		}
		if e.Outcome == "unknown" && e.Verb != "reconcile" {
			pending, pendingVerb = e.ID, e.Verb
		}
	}
	if pending != "" {
		add(ruleRRUnreconciledEffect, "effect %q (%q) has outcome unknown and no later reconcile with a receipt resolves it — success must not be inferred from the absence of a failure", pending, pendingVerb)
	}

	// 5. What the receipts establish, in order; and a succeeded release still
	// needs its reconcile.
	dv := deriveRuntimeState(d, pos)
	for _, e := range d.Effects {
		if e.Verb != "release" || e.Outcome != "succeeded" {
			continue
		}
		if !rrReleaseReconciledAbsent(d, e) {
			add(ruleRRUnreconciledRelease, "release effect %q succeeded but no later reconcile observed the resource absent — a release is complete when a reconcile receipt shows the resource gone, not when the adapter acknowledges the request", e.ID)
		}
	}

	// 6. Ownership: only a human or organization principal claims, and once one
	// owns the resource, an agent cannot end it either. Ownership belongs to one
	// lifetime: a resource on a new lifetime (deriveRuntimeState says when one
	// begins) is unclaimed until a claim in that lifetime, so both the release
	// protection and the claims backing the ownership block reset at the
	// boundary.
	var claimPrincipals = map[string]bool{}
	ownedBefore := false
	gen := 0
	for i, e := range d.Effects {
		if dv.lifetime[i] != gen {
			gen = dv.lifetime[i]
			claimPrincipals, ownedBefore = map[string]bool{}, false
		}
		switch e.Verb {
		case "claim":
			if !isOwnerPrincipal(e.Principal) {
				add(ruleRRClaimWithoutPrincipal, "claim effect %q is authorized by %s — only a human or organization principal may claim a resource (retain it, promote it, or pay for it past its TTL); an agent may only request one", e.ID, rrPrincipalOrNone(e.Principal))
			} else if e.Outcome == "succeeded" && e.HasReceipt {
				claimPrincipals[e.Principal] = true
			}
			if e.Outcome == "succeeded" && e.HasReceipt {
				ownedBefore = true
			}
		case "release":
			if ownedBefore && !isOwnerPrincipal(e.Principal) && e.Principal != "control-plane" {
				add(ruleRRClaimWithoutPrincipal, "release effect %q ends a claimed resource but is authorized by %s — once a human or organization principal owns a resource, only such a principal or the control plane may release it", e.ID, rrPrincipalOrNone(e.Principal))
			}
		}
	}
	if d.State == "claimed" && !d.HasOwner {
		add(ruleRRClaimWithoutPrincipal, "state is claimed but no ownership block names the principal holding the resource")
	}
	if d.HasOwner && !claimPrincipals[d.OwnerPrinc] {
		add(ruleRRClaimWithoutPrincipal, "ownership names %q but no succeeded claim effect by that principal in the resource's current lifetime backs it", d.OwnerPrinc)
	}

	// 7. A claim rotates, a release revokes, the agent credential — recorded.
	for _, e := range d.Effects {
		switch {
		case e.Verb == "claim" && e.Outcome == "succeeded":
			if !rrHasRotation(d.Rotations, e.ID, "") {
				add(ruleRRCredentialNotRotated, "claim effect %q is not followed by a recorded rotation or revocation of the agent credential", e.ID)
			}
		case e.Verb == "release" && (e.Outcome == "succeeded" || rrReleaseReconciledAbsent(d, e)):
			if !rrHasRotation(d.Rotations, e.ID, "revoked") {
				add(ruleRRCredentialNotRotated, "release effect %q is not followed by a recorded revocation of the agent credential", e.ID)
			}
		}
	}

	// 8. The recorded state must be the one the receipts support — the latest
	// authoritative observation, not whether an event ever happened.
	ttlEnd := dv.lifetimeStart.Add(d.TTL.D)
	pastTTL := dv.everPresent && d.TTL.OK && asOf.After(ttlEnd)
	pastRetain := dv.claimed && d.RetainUntil.OK && asOf.After(d.RetainUntil.At)
	switch d.State {
	case "requested":
		if dv.everPresent {
			add(ruleRRStateUnsupported, "state is requested but a receipt shows the resource present")
		} else if dv.createFailed {
			add(ruleRRStateUnsupported, "state is requested but a create failed or was reconciled absent — that record is failed")
		}
	case "provisioned":
		switch {
		case dv.released:
			add(ruleRRStateUnsupported, "state is provisioned but a release was reconciled to absent")
		case !dv.everPresent:
			add(ruleRRStateUnsupported, "state is provisioned but no create receipt (or reconcile receipt) observed the resource present")
		case !dv.present:
			add(ruleRRStateUnsupported, "state is provisioned but the latest authoritative receipt observed the resource absent")
		}
		if dv.claimed {
			add(ruleRRStateUnsupported, "state is provisioned but a succeeded claim makes the resource claimed")
		}
	case "claimed":
		switch {
		case dv.released:
			add(ruleRRStateUnsupported, "state is claimed but a release was reconciled to absent")
		case !dv.everPresent || !dv.claimed:
			add(ruleRRStateUnsupported, "state is claimed but the receipts show no established resource with a succeeded claim")
		case !dv.present:
			add(ruleRRStateUnsupported, "state is claimed but the latest authoritative receipt observed the resource absent")
		}
	case "expired":
		switch {
		case !dv.everPresent:
			add(ruleRRStateUnsupported, "state is expired but no receipt shows the resource was ever present")
		case dv.released:
			add(ruleRRStateUnsupported, "state is expired but a release was already reconciled to absent — that record is released")
		case (dv.claimed && !pastRetain) || (!dv.claimed && !pastTTL):
			add(ruleRRStateUnsupported, "state is expired but neither the ttl nor a claimed retention has elapsed at %s", asOf.Format(time.RFC3339))
		}
	case "released":
		switch {
		case dv.everReleased && !dv.released:
			add(ruleRRStateUnsupported, "state is released but a receipt after the reconciled release observed the resource present again — the latest authoritative observation wins")
		case !dv.released:
			add(ruleRRUnreconciledRelease, "state is released but no release effect was reconciled to absent")
		}
	case "failed":
		if dv.everPresent {
			add(ruleRRStateUnsupported, "state is failed but a receipt shows the resource present")
		} else if !dv.createFailed {
			add(ruleRRStateUnsupported, "state is failed but no create effect failed or was reconciled absent")
		}
	}

	// 9. Expiry: an unclaimed resource past its TTL, or a claimed one past its
	// retention, is expired whatever state it still records.
	switch d.State {
	case "requested", "provisioned":
		if pastTTL && !dv.claimed && dv.present {
			add(ruleRRExpired, "resource outlived its ttl (%s from %s, ended %s) unclaimed at %s — release it, or a human or organization principal claims it to retain it",
				d.TTL.D, dv.lifetimeStart.Format(time.RFC3339), ttlEnd.Format(time.RFC3339), asOf.Format(time.RFC3339))
		}
	case "claimed":
		if pastRetain {
			add(ruleRRExpired, "claimed retention ended at %s, before %s — the owning principal extends the claim, or the resource is released",
				d.RetainUntil.At.Format(time.RFC3339), asOf.Format(time.RFC3339))
		}
	}
	return out
}

// rrReleaseReconciledAbsent reports whether a later succeeded reconcile with a
// receipt names this release and observed the resource absent.
func rrReleaseReconciledAbsent(d runtimeResourceDoc, rel runtimeEffect) bool {
	for _, e := range d.Effects[rel.Index+1:] {
		if e.Verb == "reconcile" && e.Reconciles == rel.ID && e.Outcome == "succeeded" && e.HasReceipt && e.ObservedState == "absent" {
			return true
		}
	}
	return false
}

// parseRuntimeDuration parses the contract's duration form: a positive integer
// of at most five digits and one unit, s/m/h/d (a day is 24h). It refuses a
// product that would overflow a time.Duration rather than let it wrap.
func parseRuntimeDuration(s string) (time.Duration, bool) {
	m := runtimeDurationRe.FindStringSubmatch(s)
	if m == nil {
		return 0, false
	}
	n, err := strconv.ParseInt(m[1], 10, 64)
	if err != nil || n <= 0 {
		return 0, false
	}
	unit := map[string]time.Duration{"s": time.Second, "m": time.Minute, "h": time.Hour, "d": 24 * time.Hour}[m[2]]
	if n > int64(math.MaxInt64/int64(unit)) {
		return 0, false
	}
	return time.Duration(n) * unit, true
}

func isOwnerPrincipal(p string) bool {
	for _, prefix := range []string{"human:", "org:"} {
		if strings.HasPrefix(p, prefix) && len(strings.TrimSpace(p)) > len(prefix) {
			return true
		}
	}
	return false
}

func rrHasRotation(rots []runtimeRotation, after, action string) bool {
	for _, r := range rots {
		if r.After == after && (action == "" || r.Action == action) {
			return true
		}
	}
	return false
}

// rrPrincipalOrNone renders a principal for a message: quoted when the record
// gave one, so a crafted value cannot add a line of its own.
func rrPrincipalOrNone(p string) string {
	if p == "" {
		return "no principal"
	}
	return strconv.Quote(p)
}

// rrIsYAMLName reports whether a directory-scan entry is a record file. Both
// spellings of the YAML extension count.
func rrIsYAMLName(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return ext == ".yaml" || ext == ".yml"
}

// rrDecodeSingleDocument decodes exactly one YAML document. A file holding a
// second document is refused: only the first would be checked, and whatever
// followed it would ride along unvalidated. An empty file decodes to nil.
func rrDecodeSingleDocument(raw []byte) (any, error) {
	dec := yaml.NewDecoder(strings.NewReader(string(raw)))
	var first any
	if err := dec.Decode(&first); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, nil
		}
		return nil, err
	}
	var second any
	if err := dec.Decode(&second); !errors.Is(err, io.EOF) {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("more than one YAML document — one file holds one record, so nothing after the first rides along unchecked")
	}
	return first, nil
}

// rrRefuseUnknownKeys refuses a mapping carrying any key outside the allowed set.
func rrRefuseUnknownKeys(where string, m map[string]any, allowed []string) error {
	ok := rrSetOf(allowed)
	var unknown []string
	for k := range m {
		if !ok[k] {
			unknown = append(unknown, strconv.Quote(k))
		}
	}
	if len(unknown) == 0 {
		return nil
	}
	sort.Strings(unknown)
	return fmt.Errorf("%s: unknown key(s) %s (allowed: %s) — a misspelt key would read exactly like one left out on purpose", where, strings.Join(unknown, ", "), strings.Join(allowed, ", "))
}

// rrStringList reads a YAML list of strings. A non-string item is returned as
// its printed form, so the caller's vocabulary check refuses it rather than the
// item silently vanishing from the list.
func rrStringList(v any) []string {
	list, _ := v.([]any)
	out := make([]string, 0, len(list))
	for _, item := range list {
		if s, ok := item.(string); ok {
			out = append(out, s)
		} else {
			out = append(out, fmt.Sprintf("%v", item))
		}
	}
	return out
}

func rrSetOf(items []string) map[string]bool {
	m := make(map[string]bool, len(items))
	for _, s := range items {
		m[s] = true
	}
	return m
}

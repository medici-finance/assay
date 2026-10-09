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
	"flag"
	"fmt"
	"io"
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

var runtimeDurationRe = regexp.MustCompile(`^([1-9][0-9]*)([smhd])$`)

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
		entries, readErr := os.ReadDir(arg)
		if readErr != nil {
			couldNot++
			fmt.Fprintf(stdout, "could-not-check: %s: %v\n", arg, readErr)
			continue
		}
		var found int
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
				continue
			}
			files = append(files, filepath.Join(arg, e.Name()))
			found++
		}
		if found == 0 {
			// Nothing scanned is not nothing wrong.
			couldNot++
			fmt.Fprintf(stdout, "could-not-check: %s: no *.yaml record in directory\n", arg)
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
	var data map[string]any
	if err := yaml.Unmarshal(raw, &data); err != nil {
		return nil, err
	}
	if data == nil {
		return nil, fmt.Errorf("empty adapter registry")
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
				return nil, fmt.Errorf("adapter %q: max-ttl %v is not a duration like 4h", id, mt)
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
	var decoded map[string]any
	if err := yaml.Unmarshal(raw, &decoded); err != nil {
		return patternStateCouldNot, []string{fmt.Sprintf("%s: %v", path, err)}
	}
	if decoded == nil {
		return patternStateCouldNot, []string{fmt.Sprintf("%s: empty document", path)}
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
	HasReceipt    bool
	ObservedAt    time.Time
	ObservedAtOK  bool
	ObservedState string
}

type runtimeRotation struct {
	After  string
	Action string
}

type runtimeResourceDoc struct {
	ID          string
	Class       string
	AdapterID   string
	State       string
	TTL         time.Duration
	TTLOK       bool
	CredLife    time.Duration
	CredLifeOK  bool
	HasOwner    bool
	OwnerPrinc  string
	RetainUntil time.Time
	RetainOK    bool
	Rotations   []runtimeRotation
	Effects     []runtimeEffect
}

// parseRuntimeResource resolves the generic decoded map defensively: a missing
// or mistyped field becomes a zero value, which the schema pass has already
// reported, so the rules below never panic on a malformed record.
func parseRuntimeResource(data map[string]any) runtimeResourceDoc {
	var d runtimeResourceDoc
	d.ID, _ = data["id"].(string)
	d.Class, _ = data["class"].(string)
	d.State, _ = data["state"].(string)
	if a, ok := data["adapter"].(map[string]any); ok {
		d.AdapterID, _ = a["id"].(string)
	}
	ttl, _ := data["ttl"].(string)
	d.TTL, d.TTLOK = parseRuntimeDuration(ttl)
	if c, ok := data["credentials"].(map[string]any); ok {
		if ac, ok := c["agent-credential"].(map[string]any); ok {
			life, _ := ac["lifetime"].(string)
			d.CredLife, d.CredLifeOK = parseRuntimeDuration(life)
		}
		rots, _ := c["rotations"].([]any)
		for _, r := range rots {
			if rm, ok := r.(map[string]any); ok {
				after, _ := rm["after"].(string)
				action, _ := rm["action"].(string)
				d.Rotations = append(d.Rotations, runtimeRotation{After: after, Action: action})
			}
		}
	}
	if o, ok := data["ownership"].(map[string]any); ok {
		d.HasOwner = true
		d.OwnerPrinc, _ = o["principal"].(string)
		if ru, ok := o["retain-until"].(string); ok {
			if t, err := time.Parse(time.RFC3339, ru); err == nil {
				d.RetainUntil, d.RetainOK = t, true
			}
		}
	}
	effects, _ := data["effects"].([]any)
	for i, e := range effects {
		m, ok := e.(map[string]any)
		if !ok {
			continue
		}
		var eff runtimeEffect
		eff.Index = i
		eff.ID, _ = m["id"].(string)
		eff.Verb, _ = m["verb"].(string)
		eff.Outcome, _ = m["outcome"].(string)
		eff.Principal, _ = m["principal"].(string)
		eff.Reconciles, _ = m["reconciles"].(string)
		if r, ok := m["receipt"].(map[string]any); ok {
			eff.HasReceipt = true
			eff.ObservedState, _ = r["observed-state"].(string)
			if oa, ok := r["observed-at"].(string); ok {
				if t, err := time.Parse(time.RFC3339, oa); err == nil {
					eff.ObservedAt, eff.ObservedAtOK = t, true
				}
			}
		}
		d.Effects = append(d.Effects, eff)
	}
	return d
}

// checkRuntimeResourceRules applies the contract rules a schema cannot express.
func checkRuntimeResourceRules(path string, d runtimeResourceDoc, adapters map[string]runtimeAdapter, asOf time.Time) []string {
	var out []string
	add := func(rule, format string, args ...any) {
		out = append(out, fmt.Sprintf("%s [%s]: %s", path, rule, fmt.Sprintf(format, args...)))
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
				add(ruleRRUnknownAdapter, "effect %s: adapter %q does not implement verb %q", e.ID, d.AdapterID, e.Verb)
			}
		}
		if a.MaxTTL > 0 && d.TTLOK && d.TTL > a.MaxTTL {
			add(ruleRRTTLExceedsBound, "ttl %s exceeds adapter %q's max-ttl %s — an ephemeral resource is requested within the adapter's bound; retaining it longer is a claim", d.TTL, d.AdapterID, a.MaxTTL)
		}
	}

	// 2. The agent's credential never outlives the resource it is scoped to.
	if d.TTLOK && d.CredLifeOK && d.CredLife > d.TTL {
		add(ruleRRCredentialOutlivesTTL, "agent credential lifetime %s exceeds the resource ttl %s", d.CredLife, d.TTL)
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
	// resolved maps an effect id to the last resolving reconcile of it — a
	// reconcile that itself succeeded and carries a receipt.
	resolved := map[string]runtimeEffect{}
	pending := ""
	pendingVerb := ""
	for _, e := range d.Effects {
		if e.Outcome == "succeeded" && !e.HasReceipt {
			add(ruleRRReceiptMissing, "effect %s (%s) is recorded succeeded with no receipt — an acknowledged effect carries the adapter's typed receipt", e.ID, e.Verb)
		}
		resolving := false
		if e.Verb == "reconcile" {
			if e.Reconciles == "" {
				add(ruleRRUnreconciledEffect, "reconcile effect %s names no effect it reconciles", e.ID)
			} else if j, ok := pos[e.Reconciles]; !ok || j >= e.Index {
				add(ruleRRUnreconciledEffect, "reconcile effect %s reconciles %q, which is not an earlier effect in this record", e.ID, e.Reconciles)
			} else if e.Outcome == "succeeded" && e.HasReceipt {
				resolving = true
				resolved[e.Reconciles] = e
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
				add(ruleRRUnreconciledEffect, "effect %s (%s) was issued while effect %s (%s) had outcome unknown — the unknown effect must be reconciled against a receipt before the run does anything else", e.ID, e.Verb, pending, pendingVerb)
			}
			continue
		}
		if e.Outcome == "unknown" && e.Verb != "reconcile" {
			pending, pendingVerb = e.ID, e.Verb
		}
	}
	if pending != "" {
		add(ruleRRUnreconciledEffect, "effect %s (%s) has outcome unknown and no later reconcile with a receipt resolves it — success must not be inferred from the absence of a failure", pending, pendingVerb)
	}

	// 5. What the receipts establish.
	var established bool
	var establishedAt time.Time
	var createFailed, claimed, released bool
	for _, e := range d.Effects {
		switch e.Verb {
		case "create":
			switch {
			case e.Outcome == "succeeded" && e.HasReceipt && e.ObservedState == "present":
				if !established {
					established, establishedAt = true, e.ObservedAt
				}
			case e.Outcome == "failed":
				createFailed = true
			case e.Outcome == "unknown":
				if r, ok := resolved[e.ID]; ok {
					if r.ObservedState == "present" {
						if !established {
							established, establishedAt = true, r.ObservedAt
						}
					} else {
						createFailed = true
					}
				}
			}
		case "claim":
			if e.Outcome == "succeeded" && e.HasReceipt {
				claimed = true
			}
		case "release":
			if r, ok := resolved[e.ID]; ok && r.ObservedState == "absent" {
				released = true
			} else if e.Outcome == "succeeded" {
				add(ruleRRUnreconciledRelease, "release effect %s succeeded but no later reconcile observed the resource absent — a release is complete when a reconcile receipt shows the resource gone, not when the adapter acknowledges the request", e.ID)
			}
		}
	}

	// 6. Ownership: only a human or organization principal claims.
	var claimPrincipals = map[string]bool{}
	for _, e := range d.Effects {
		if e.Verb != "claim" {
			continue
		}
		if !isOwnerPrincipal(e.Principal) {
			who := e.Principal
			if who == "" {
				who = "no principal"
			}
			add(ruleRRClaimWithoutPrincipal, "claim effect %s is authorized by %s — only a human or organization principal may claim a resource (retain it, promote it, or pay for it past its TTL); an agent may only request one", e.ID, rrQuoteIfSet(who))
		} else if e.Outcome == "succeeded" && e.HasReceipt {
			claimPrincipals[e.Principal] = true
		}
	}
	if d.State == "claimed" && !d.HasOwner {
		add(ruleRRClaimWithoutPrincipal, "state is claimed but no ownership block names the principal holding the resource")
	}
	if d.HasOwner && !claimPrincipals[d.OwnerPrinc] {
		add(ruleRRClaimWithoutPrincipal, "ownership names %q but no succeeded claim effect by that principal backs it", d.OwnerPrinc)
	}

	// 7. A claim rotates, a release revokes, the agent credential — recorded.
	for _, e := range d.Effects {
		switch {
		case e.Verb == "claim" && e.Outcome == "succeeded":
			if !rrHasRotation(d.Rotations, e.ID, "") {
				add(ruleRRCredentialNotRotated, "claim effect %s is not followed by a recorded rotation or revocation of the agent credential", e.ID)
			}
		case e.Verb == "release" && (e.Outcome == "succeeded" || resolved[e.ID].ObservedState == "absent"):
			if !rrHasRotation(d.Rotations, e.ID, "revoked") {
				add(ruleRRCredentialNotRotated, "release effect %s is not followed by a recorded revocation of the agent credential", e.ID)
			}
		}
	}

	// 8. The recorded state must be the one the receipts support.
	ttlEnd := establishedAt.Add(d.TTL)
	pastTTL := established && d.TTLOK && asOf.After(ttlEnd)
	pastRetain := claimed && d.RetainOK && asOf.After(d.RetainUntil)
	switch d.State {
	case "requested":
		if established {
			add(ruleRRStateUnsupported, "state is requested but a receipt shows the resource present")
		}
	case "provisioned":
		if !established {
			add(ruleRRStateUnsupported, "state is provisioned but no create receipt (or reconcile receipt) observed the resource present")
		}
		if claimed {
			add(ruleRRStateUnsupported, "state is provisioned but a succeeded claim makes the resource claimed")
		}
		if released {
			add(ruleRRStateUnsupported, "state is provisioned but a release was reconciled to absent")
		}
	case "claimed":
		if !established || !claimed {
			add(ruleRRStateUnsupported, "state is claimed but the receipts show no established resource with a succeeded claim")
		}
		if released {
			add(ruleRRStateUnsupported, "state is claimed but a release was reconciled to absent")
		}
	case "expired":
		if !established {
			add(ruleRRStateUnsupported, "state is expired but no receipt shows the resource was ever present")
		} else if (claimed && !pastRetain) || (!claimed && !pastTTL) {
			add(ruleRRStateUnsupported, "state is expired but neither the ttl nor a claimed retention has elapsed at %s", asOf.Format(time.RFC3339))
		}
	case "released":
		if !released {
			add(ruleRRUnreconciledRelease, "state is released but no release effect was reconciled to absent")
		}
	case "failed":
		if established {
			add(ruleRRStateUnsupported, "state is failed but a receipt shows the resource present")
		} else if !createFailed {
			add(ruleRRStateUnsupported, "state is failed but no create effect failed or was reconciled absent")
		}
	}

	// 9. Expiry: an unclaimed resource past its TTL, or a claimed one past its
	// retention, is expired whatever state it still records.
	switch d.State {
	case "requested", "provisioned":
		if pastTTL && !claimed {
			add(ruleRRExpired, "resource outlived its ttl (%s from %s, ended %s) unclaimed at %s — release it, or a human or organization principal claims it to retain it",
				d.TTL, establishedAt.Format(time.RFC3339), ttlEnd.Format(time.RFC3339), asOf.Format(time.RFC3339))
		}
	case "claimed":
		if pastRetain {
			add(ruleRRExpired, "claimed retention ended at %s, before %s — the owning principal extends the claim, or the resource is released",
				d.RetainUntil.Format(time.RFC3339), asOf.Format(time.RFC3339))
		}
	}
	return out
}

// parseRuntimeDuration parses the contract's duration form: a positive integer
// and one unit, s/m/h/d (a day is 24h).
func parseRuntimeDuration(s string) (time.Duration, bool) {
	m := runtimeDurationRe.FindStringSubmatch(s)
	if m == nil {
		return 0, false
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return 0, false
	}
	unit := map[string]time.Duration{"s": time.Second, "m": time.Minute, "h": time.Hour, "d": 24 * time.Hour}[m[2]]
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

func rrQuoteIfSet(s string) string {
	if s == "no principal" {
		return s
	}
	return strconv.Quote(s)
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

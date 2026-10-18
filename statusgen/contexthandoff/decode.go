package contexthandoff

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Rule codes. Every refusal names exactly one; spec/context-handoff-v1.md
// §"Refusals" lists them, and the fixture file names lead with them.
const (
	RuleMalformed        = "malformed"         // not one well-formed UTF-8 JSON value
	RuleDuplicateKey     = "duplicate-key"     // an object repeats a key
	RuleUnknownField     = "unknown-field"     // a key this version does not define
	RuleMissingField     = "missing-field"     // a defined key is absent
	RuleWrongType        = "wrong-type"        // a value of the wrong JSON type, including null where a value is required
	RuleBadValue         = "bad-value"         // a value outside its pattern, enum or minimum count
	RuleSchemaVersion    = "schema-version"    // "schema" is not this version's identifier
	RuleQualifiedID      = "qualified-id"      // a brief reference that is not a qualified ID
	RuleApproval         = "approval"          // target state and approval disagree
	RuleCoverage         = "coverage"          // a coverage gap is not stated
	RuleAmendUnbound     = "amend-unbound"     // a manifest obligation with no bound patch or follow-up
	RuleImpactUnbound    = "impact-unbound"    // a changing disposition with no amendment obligation
	RuleDuplicateID      = "duplicate-id"      // an identity listed twice where it must be unique
	RuleActiveWork       = "active-work"       // an active-work binding that is not acknowledged
	RuleReadiness        = "readiness"         // readiness contradicts the records it summarizes
	RuleNotApproved      = "not-approved"      // an approved target is required and this one is not
	RuleManifestMismatch = "manifest-mismatch" // a manifest that does not match its handoff
)

var allRules = []string{
	RuleMalformed, RuleDuplicateKey, RuleUnknownField, RuleMissingField, RuleWrongType,
	RuleBadValue, RuleSchemaVersion, RuleQualifiedID, RuleApproval, RuleCoverage,
	RuleAmendUnbound, RuleImpactUnbound, RuleDuplicateID, RuleActiveWork, RuleReadiness,
	RuleNotApproved, RuleManifestMismatch,
}

// Error is a refusal: the rule broken, where, and why. The package fails fast:
// it reports the first refusal it reaches.
type Error struct {
	Rule string
	Path string
	Msg  string
}

func (e *Error) Error() string { return e.Rule + ": " + e.Path + ": " + e.Msg }

// RuleOf returns the rule code of a refusal from this package, or "" for any
// other error (including nil).
func RuleOf(err error) string {
	if e, ok := err.(*Error); ok && e != nil {
		return e.Rule
	}
	return ""
}

// Whitespace is an explicit union of Unicode White_Space and ECMA-262
// whitespace (which also includes BOM), shared with the published schemas.
const whitespace = "\t\n\v\f\r \u0085\u00a0\u1680\u2000-\u200a\u2028\u2029\u202f\u205f\u3000\ufeff"

var (
	revisionRe  = regexp.MustCompile(`^[0-9a-f]{7,64}$`)
	digestRe    = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	nameRe      = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)
	locatorRe   = regexp.MustCompile("^[^" + whitespace + "]+$")
	textRe      = regexp.MustCompile("[^" + whitespace + "]")
	timestampRe = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$`)
	schemeRe    = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*:`)
)

// parseStrict reads exactly one JSON value into a generic tree (objects are
// map[string]any, arrays []any, strings, json.Number, bool, nil), refusing
// invalid UTF-8, duplicate keys and trailing data.
func parseStrict(data []byte) (any, *Error) {
	if !utf8.Valid(data) {
		return nil, &Error{RuleMalformed, "$", "input is not valid UTF-8"}
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	v, perr := parseValue(dec, "$")
	if perr != nil {
		return nil, perr
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, &Error{RuleMalformed, "$", "trailing data after the record"}
	}
	return v, nil
}

func parseValue(dec *json.Decoder, path string) (any, *Error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, &Error{RuleMalformed, path, err.Error()}
	}
	d, ok := tok.(json.Delim)
	if !ok {
		return tok, nil
	}
	switch d {
	case '{':
		m := map[string]any{}
		for dec.More() {
			kt, err := dec.Token()
			if err != nil {
				return nil, &Error{RuleMalformed, path, err.Error()}
			}
			k, _ := kt.(string)
			if _, dup := m[k]; dup {
				return nil, &Error{RuleDuplicateKey, path + "." + k, "key appears more than once"}
			}
			v, perr := parseValue(dec, path+"."+k)
			if perr != nil {
				return nil, perr
			}
			m[k] = v
		}
		if _, err := dec.Token(); err != nil {
			return nil, &Error{RuleMalformed, path, err.Error()}
		}
		return m, nil
	case '[':
		a := []any{}
		for dec.More() {
			v, perr := parseValue(dec, path+"["+strconv.Itoa(len(a))+"]")
			if perr != nil {
				return nil, perr
			}
			a = append(a, v)
		}
		if _, err := dec.Token(); err != nil {
			return nil, &Error{RuleMalformed, path, err.Error()}
		}
		return a, nil
	}
	return nil, &Error{RuleMalformed, path, "unexpected delimiter " + d.String()}
}

func kindOf(v any) string {
	switch v.(type) {
	case nil:
		return "null"
	case map[string]any:
		return "object"
	case []any:
		return "array"
	case string:
		return "string"
	case bool:
		return "boolean"
	case json.Number:
		return "number"
	}
	return fmt.Sprintf("%T", v)
}

// reader walks the generic tree with exact-key accessors. It keeps the first
// refusal; once one is recorded every accessor returns a zero value.
type reader struct {
	err *Error
	// declaring is the declaring record's identity, set once declared_by is
	// read; elided references expand against it.
	declaring *QID
}

func (r *reader) fail(rule, path, format string, a ...any) {
	if r.err == nil {
		r.err = &Error{Rule: rule, Path: path, Msg: fmt.Sprintf(format, a...)}
	}
}

// object checks v is an object carrying exactly keys: unknown keys first (in
// byte order), then missing keys (in declaration order).
func (r *reader) object(v any, path string, keys ...string) map[string]any {
	if r.err != nil {
		return nil
	}
	m, ok := v.(map[string]any)
	if !ok {
		r.fail(RuleWrongType, path, "want object, got %s", kindOf(v))
		return nil
	}
	known := make(map[string]bool, len(keys))
	for _, k := range keys {
		known[k] = true
	}
	var unknown []string
	for k := range m {
		if !known[k] {
			unknown = append(unknown, k)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		r.fail(RuleUnknownField, path+"."+unknown[0], "field is not defined by this version; a reader never drops a field it does not understand")
		return nil
	}
	for _, k := range keys {
		if _, ok := m[k]; !ok {
			r.fail(RuleMissingField, path+"."+k, "required field is absent")
			return nil
		}
	}
	return m
}

func (r *reader) str(m map[string]any, path, key string, re *regexp.Regexp, what string) string {
	if r.err != nil {
		return ""
	}
	return r.strValue(m[key], path+"."+key, re, what)
}

func (r *reader) strValue(v any, path string, re *regexp.Regexp, what string) string {
	if r.err != nil {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		r.fail(RuleWrongType, path, "want string, got %s", kindOf(v))
		return ""
	}
	if re != nil && !re.MatchString(s) {
		r.fail(RuleBadValue, path, "%q is not a %s", s, what)
		return ""
	}
	return s
}

func (r *reader) optStr(m map[string]any, path, key string, re *regexp.Regexp, what string) *string {
	if r.err != nil || m[key] == nil {
		return nil
	}
	s := r.str(m, path, key, re, what)
	if r.err != nil {
		return nil
	}
	return &s
}

func (r *reader) enum(m map[string]any, path, key string, allowed ...string) string {
	s := r.str(m, path, key, nil, "")
	if r.err != nil {
		return ""
	}
	for _, a := range allowed {
		if s == a {
			return s
		}
	}
	r.fail(RuleBadValue, path+"."+key, "%q is not one of %v", s, allowed)
	return ""
}

func (r *reader) constant(m map[string]any, path, key, want, rule string) string {
	s := r.str(m, path, key, nil, "")
	if r.err == nil && s != want {
		r.fail(rule, path+"."+key, "%q is not %q", s, want)
	}
	return s
}

func (r *reader) boolean(m map[string]any, path, key string) bool {
	if r.err != nil {
		return false
	}
	b, ok := m[key].(bool)
	if !ok {
		r.fail(RuleWrongType, path+"."+key, "want boolean, got %s", kindOf(m[key]))
	}
	return b
}

// array returns the elements and their paths; min is the minimum count.
func (r *reader) array(m map[string]any, path, key string, min int) ([]any, []string) {
	if r.err != nil {
		return nil, nil
	}
	p := path + "." + key
	a, ok := m[key].([]any)
	if !ok {
		r.fail(RuleWrongType, p, "want array, got %s", kindOf(m[key]))
		return nil, nil
	}
	if len(a) < min {
		r.fail(RuleBadValue, p, "want at least %d item(s), got %d", min, len(a))
		return nil, nil
	}
	paths := make([]string, len(a))
	for i := range a {
		paths[i] = p + "[" + strconv.Itoa(i) + "]"
	}
	return a, paths
}

func (r *reader) strArray(m map[string]any, path, key string, min int, re *regexp.Regexp, what string) []string {
	a, paths := r.array(m, path, key, min)
	out := make([]string, 0, len(a))
	for i, v := range a {
		out = append(out, r.strValue(v, paths[i], re, what))
	}
	return out
}

// qid reads one brief reference and returns its expanded four-segment form.
func (r *reader) qidValue(v any, path string) string {
	s := r.strValue(v, path, nil, "")
	if r.err != nil {
		return ""
	}
	q, err := ResolveQualifiedID(s, r.declaring)
	if err != nil {
		r.fail(RuleQualifiedID, path, "%v", err)
		return ""
	}
	return q.String()
}

func (r *reader) qid(m map[string]any, path, key string) string {
	if r.err != nil {
		return ""
	}
	return r.qidValue(m[key], path+"."+key)
}

func (r *reader) qidArray(m map[string]any, path, key string) []string {
	a, paths := r.array(m, path, key, 0)
	out := make([]string, 0, len(a))
	for i, v := range a {
		out = append(out, r.qidValue(v, paths[i]))
	}
	return out
}

// declaredBy reads the declaring identity, which is always the full form, and
// makes it the expansion context for every later reference.
func (r *reader) declaredBy(m map[string]any, path string) string {
	if r.err != nil {
		return ""
	}
	p := path + ".declared_by"
	s := r.strValue(m["declared_by"], p, nil, "")
	if r.err != nil {
		return ""
	}
	q, err := ResolveQualifiedID(s, nil)
	if err != nil {
		r.fail(RuleQualifiedID, p, "%v", err)
		return ""
	}
	r.declaring = &q
	return q.String()
}

// patch reads an amendment's candidate patch (base revision and digest).
func (r *reader) patch(v any, path string) *Patch {
	if r.err != nil {
		return nil
	}
	m := r.object(v, path, "base_revision", "digest")
	if r.err != nil {
		return nil
	}
	p := &Patch{
		BaseRevision: r.str(m, path, "base_revision", revisionRe, "revision (7-64 lower-case hex)"),
		Digest:       r.str(m, path, "digest", digestRe, "digest (sha256:<64 hex>)"),
	}
	if r.err != nil {
		return nil
	}
	return p
}

// DecodeHandoff strictly decodes, expands and validates a context-handoff-v1
// record. The returned record carries every brief reference in the canonical
// four-segment form.
//
// semantic: S-context-handoff
func DecodeHandoff(data []byte) (*Handoff, error) {
	tree, perr := parseStrict(data)
	if perr != nil {
		return nil, perr
	}
	r := &reader{}
	h := r.handoff(tree)
	if r.err != nil {
		return nil, r.err
	}
	if err := validateHandoff(h); err != nil {
		return nil, err
	}
	return h, nil
}

func (r *reader) handoff(v any) *Handoff {
	const p = "$"
	m := r.object(v, p, "schema", "declared_by", "target", "discovery", "impacts", "amendments", "history", "active_work", "readiness")
	h := &Handoff{}
	h.Schema = r.enum(m, p, "schema", HandoffSchema, HandoffSchemaV2)
	if r.err != nil && r.err.Path == "$.schema" && r.err.Rule == RuleBadValue {
		r.err.Rule = RuleSchemaVersion
	}
	h.DeclaredBy = r.declaredBy(m, p)
	h.Target = r.target(m["target"], p+".target")
	h.Discovery = r.discovery(m["discovery"], p+".discovery")

	items, paths := r.array(m, p, "impacts", 0)
	h.Impacts = make([]Impact, 0, len(items))
	for i, it := range items {
		h.Impacts = append(h.Impacts, r.impact(it, paths[i], h.Schema))
	}
	items, paths = r.array(m, p, "amendments", 0)
	h.Amendments = make([]Amendment, 0, len(items))
	for i, it := range items {
		h.Amendments = append(h.Amendments, r.amendment(it, paths[i]))
	}
	items, paths = r.array(m, p, "history", 0)
	h.History = make([]History, 0, len(items))
	for i, it := range items {
		h.History = append(h.History, r.history(it, paths[i]))
	}
	items, paths = r.array(m, p, "active_work", 0)
	h.ActiveWork = make([]ActiveWork, 0, len(items))
	for i, it := range items {
		h.ActiveWork = append(h.ActiveWork, r.activeWork(it, paths[i]))
	}
	h.Readiness = r.readiness(m["readiness"], p+".readiness")
	return h
}

func (r *reader) target(v any, p string) Target {
	m := r.object(v, p, "state", "revision", "approval", "intent", "contracts", "transition", "acceptance_source")
	t := Target{
		State:    r.enum(m, p, "state", TargetDraft, TargetProposed, TargetApproved),
		Revision: r.str(m, p, "revision", revisionRe, "revision (7-64 lower-case hex)"),
	}
	if r.err == nil && m["approval"] != nil {
		am := r.object(m["approval"], p+".approval", "revision", "reference")
		ap := &Approval{
			Revision:  r.str(am, p+".approval", "revision", revisionRe, "revision (7-64 lower-case hex)"),
			Reference: r.str(am, p+".approval", "reference", locatorRe, "locator"),
		}
		if r.err != nil && r.err.Path == p+".approval.revision" {
			// An approval without a recorded revision is an approval refusal,
			// whichever way the revision is missing.
			r.err.Rule = RuleApproval
		}
		t.Approval = ap
	}
	t.Intent = r.str(m, p, "intent", textRe, "non-blank text")
	items, paths := r.array(m, p, "contracts", 0)
	t.Contracts = make([]ContractChange, 0, len(items))
	for i, it := range items {
		cm := r.object(it, paths[i], "name", "from", "to")
		t.Contracts = append(t.Contracts, ContractChange{
			Name: r.str(cm, paths[i], "name", nameRe, "name"),
			From: r.optStr(cm, paths[i], "from", locatorRe, "locator"),
			To:   r.str(cm, paths[i], "to", locatorRe, "locator"),
		})
	}
	t.Transition = r.optStr(m, p, "transition", textRe, "non-blank text")
	t.AcceptanceSource = r.str(m, p, "acceptance_source", locatorRe, "locator")
	return t
}

func (r *reader) discovery(v any, p string) Discovery {
	if r.err == nil {
		if dm, ok := v.(map[string]any); ok {
			if c, present := dm["coverage"]; !present || c == nil {
				// A receipt that does not state its coverage is a coverage
				// refusal, never a default of "complete".
				r.fail(RuleCoverage, p+".coverage", "a discovery receipt must state its coverage: what was enumerated and what was unavailable")
				return Discovery{}
			}
		}
	}
	m := r.object(v, p, "sources", "queries", "dependency_slice", "coverage")
	d := Discovery{}
	items, paths := r.array(m, p, "sources", 0)
	d.Sources = make([]Source, 0, len(items))
	for i, it := range items {
		sm := r.object(it, paths[i], "id", "revision", "membership", "reason")
		d.Sources = append(d.Sources, Source{
			ID:         r.str(sm, paths[i], "id", nameRe, "name"),
			Revision:   r.optStr(sm, paths[i], "revision", revisionRe, "revision (7-64 lower-case hex)"),
			Membership: r.enum(sm, paths[i], "membership", "included", "excluded"),
			Reason:     r.str(sm, paths[i], "reason", textRe, "non-blank text"),
		})
	}
	items, paths = r.array(m, p, "queries", 0)
	d.Queries = make([]Query, 0, len(items))
	for i, it := range items {
		qm := r.object(it, paths[i], "rule", "version", "parameters", "results", "truncated")
		q := Query{
			Rule:    r.str(qm, paths[i], "rule", nameRe, "name"),
			Version: r.str(qm, paths[i], "version", locatorRe, "locator"),
		}
		q.Parameters = map[string]string{}
		if r.err == nil {
			pm, ok := qm["parameters"].(map[string]any)
			if !ok {
				r.fail(RuleWrongType, paths[i]+".parameters", "want object, got %s", kindOf(qm["parameters"]))
			}
			keys := make([]string, 0, len(pm))
			for k := range pm {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				q.Parameters[k] = r.strValue(pm[k], paths[i]+".parameters."+k, nil, "")
			}
		}
		q.Results = r.strArray(qm, paths[i], "results", 0, locatorRe, "locator")
		q.Truncated = r.boolean(qm, paths[i], "truncated")
		d.Queries = append(d.Queries, q)
	}
	d.DependencySlice = r.qidArray(m, p, "dependency_slice")

	cp := p + ".coverage"
	clean := r.err == nil
	cm := r.object(m["coverage"], cp, "state", "enumerated", "unavailable")
	if clean && r.err != nil && r.err.Rule == RuleMissingField {
		// A coverage that omits its state or either accounting list has not
		// stated its gaps.
		r.err.Rule = RuleCoverage
	}
	d.Coverage.State = r.enum(cm, cp, "state", "complete", "partial")
	d.Coverage.Enumerated = r.strArray(cm, cp, "enumerated", 0, nameRe, "name")
	items, paths = r.array(cm, cp, "unavailable", 0)
	d.Coverage.Unavailable = make([]Unavailable, 0, len(items))
	for i, it := range items {
		um := r.object(it, paths[i], "source", "reason")
		d.Coverage.Unavailable = append(d.Coverage.Unavailable, Unavailable{
			Source: r.str(um, paths[i], "source", nameRe, "name"),
			Reason: r.str(um, paths[i], "reason", textRe, "non-blank text"),
		})
	}
	return d
}

func (r *reader) impact(v any, p, schema string) Impact {
	m := r.object(v, p, "subject", "lifecycle", "evidence", "relationship", "disposition", "reason")
	lifecycles := []string{"todo", "in-progress", "implemented", "verified", "done", "blocked", "unknown"}
	if schema == HandoffSchemaV2 {
		lifecycles = append(lifecycles, "withdrawn")
	}
	im := Impact{
		Subject:   r.qid(m, p, "subject"),
		Lifecycle: r.enum(m, p, "lifecycle", lifecycles...),
		Evidence:  r.optStr(m, p, "evidence", locatorRe, "locator"),
	}
	rp := p + ".relationship"
	rm := r.object(m["relationship"], rp, "basis", "edge", "locator")
	im.Relationship = Relationship{
		Basis:   r.enum(rm, rp, "basis", "declared", "observed", "inferred"),
		Edge:    r.enum(rm, rp, "edge", edges...),
		Locator: r.str(rm, rp, "locator", locatorRe, "locator"),
	}
	im.Disposition = r.enum(m, p, "disposition", dispositions...)
	im.Reason = r.str(m, p, "reason", textRe, "non-blank text")
	return im
}

var (
	edges        = []string{"provides", "consumes", "owns", "enforces", "persists", "changes", "assumes", "realizes", "replaces", "migrates", "requires", "checked-by"}
	dispositions = []string{"retain", "amend", "split", "replace", "retire", "defer", "unresolved"}
	actions      = []string{"amend", "split", "replace", "retire"}
)

func (r *reader) amendment(v any, p string) Amendment {
	m := r.object(v, p, "id", "subject", "action", "changes", "replaced_by", "candidate")
	a := Amendment{
		ID:      r.str(m, p, "id", nameRe, "name"),
		Subject: r.qid(m, p, "subject"),
		Action:  r.enum(m, p, "action", actions...),
	}
	items, paths := r.array(m, p, "changes", 1)
	a.Changes = make([]Change, 0, len(items))
	for i, it := range items {
		cm := r.object(it, paths[i], "aspect", "before", "after")
		a.Changes = append(a.Changes, Change{
			Aspect: r.enum(cm, paths[i], "aspect", "task", "dependencies", "consumers", "delivery", "verify", "scope"),
			Before: r.str(cm, paths[i], "before", nil, ""),
			After:  r.str(cm, paths[i], "after", nil, ""),
		})
	}
	a.ReplacedBy = r.qidArray(m, p, "replaced_by")
	if r.err == nil && m["candidate"] != nil {
		a.Candidate = r.patch(m["candidate"], p+".candidate")
	}
	return a
}

func (r *reader) history(v any, p string) History {
	m := r.object(v, p, "kind", "subject", "locator", "digest", "correction")
	h := History{
		Kind:    r.enum(m, p, "kind", "evidence", "source-account", "decision-account", "source-identity"),
		Subject: r.qid(m, p, "subject"),
		Locator: r.str(m, p, "locator", locatorRe, "locator"),
		Digest:  r.str(m, p, "digest", digestRe, "digest (sha256:<64 hex>)"),
	}
	if r.err == nil && m["correction"] != nil {
		cp := p + ".correction"
		cm := r.object(m["correction"], cp, "rationale", "source", "review", "replacement_digest")
		h.Correction = &Correction{
			Rationale:         r.str(cm, cp, "rationale", textRe, "non-blank text"),
			Source:            r.str(cm, cp, "source", locatorRe, "locator"),
			Review:            r.str(cm, cp, "review", locatorRe, "locator"),
			ReplacementDigest: r.str(cm, cp, "replacement_digest", digestRe, "digest (sha256:<64 hex>)"),
		}
	}
	return h
}

func (r *reader) activeWork(v any, p string) ActiveWork {
	m := r.object(v, p, "subject", "claim", "brief_revision", "observed_at", "owner", "checkpoint", "treatment", "acknowledged")
	return ActiveWork{
		Subject:       r.qid(m, p, "subject"),
		Claim:         r.str(m, p, "claim", locatorRe, "locator"),
		BriefRevision: r.str(m, p, "brief_revision", revisionRe, "revision (7-64 lower-case hex)"),
		ObservedAt:    r.str(m, p, "observed_at", timestampRe, "UTC timestamp (YYYY-MM-DDTHH:MM:SSZ)"),
		Owner:         r.str(m, p, "owner", nameRe, "role name"),
		Checkpoint:    r.str(m, p, "checkpoint", textRe, "non-blank text"),
		Treatment:     r.enum(m, p, "treatment", "adjust", "cancel", "reconcile"),
		Acknowledged:  r.boolean(m, p, "acknowledged"),
	}
}

func (r *reader) readiness(v any, p string) Readiness {
	m := r.object(v, p, "unresolved", "deciding_role", "next_act", "prerequisites", "facts")
	rd := Readiness{
		Unresolved:    r.qidArray(m, p, "unresolved"),
		DecidingRole:  r.str(m, p, "deciding_role", nameRe, "role name"),
		NextAct:       r.enum(m, p, "next_act", "explore", "approve", "publish", "dispatch"),
		Prerequisites: r.strArray(m, p, "prerequisites", 0, textRe, "non-blank text"),
	}
	fp := p + ".facts"
	fm := r.object(m["facts"], fp, "source_available", "proposal_approved", "deployment_qualified")
	rd.Facts = Facts{
		SourceAvailable:     r.boolean(fm, fp, "source_available"),
		ProposalApproved:    r.boolean(fm, fp, "proposal_approved"),
		DeploymentQualified: r.boolean(fm, fp, "deployment_qualified"),
	}
	return rd
}

// DecodeManifest strictly decodes, expands and validates an
// amendment-manifest-v1 record on its own. CheckManifest binds it to its
// handoff.
func DecodeManifest(data []byte) (*Manifest, error) {
	tree, perr := parseStrict(data)
	if perr != nil {
		return nil, perr
	}
	r := &reader{}
	mf := r.manifest(tree)
	if r.err != nil {
		return nil, r.err
	}
	if err := validateManifest(mf); err != nil {
		return nil, err
	}
	return mf, nil
}

func (r *reader) manifest(v any) *Manifest {
	const p = "$"
	m := r.object(v, p, "schema", "declared_by", "handoff", "entries")
	mf := &Manifest{}
	mf.Schema = r.constant(m, p, "schema", ManifestSchema, RuleSchemaVersion)
	mf.DeclaredBy = r.declaredBy(m, p)
	mf.Handoff = r.str(m, p, "handoff", digestRe, "digest (sha256:<64 hex>)")
	items, paths := r.array(m, p, "entries", 0)
	mf.Entries = make([]Entry, 0, len(items))
	for i, it := range items {
		mf.Entries = append(mf.Entries, r.entry(it, paths[i]))
	}
	return mf
}

func (r *reader) entry(v any, p string) Entry {
	if r.err == nil {
		if em, ok := v.(map[string]any); ok {
			if _, present := em["binding"]; !present {
				// An absent binding is read as no binding; validateManifest
				// refuses it with the same rule as an empty or null one.
				filled := make(map[string]any, len(em)+1)
				for k, val := range em {
					filled[k] = val
				}
				filled["binding"] = nil
				v = filled
			}
		}
	}
	m := r.object(v, p, "obligation", "brief", "action", "delivery_paths", "contract_version", "depends", "test_commands", "active_work", "effort", "binding")
	e := Entry{
		Obligation:      r.str(m, p, "obligation", nameRe, "name"),
		Brief:           r.qid(m, p, "brief"),
		Action:          r.enum(m, p, "action", actions...),
		DeliveryPaths:   r.deliveryPaths(m, p),
		ContractVersion: r.optStr(m, p, "contract_version", locatorRe, "locator"),
		Depends:         r.qidArray(m, p, "depends"),
		TestCommands:    r.strArray(m, p, "test_commands", 0, textRe, "non-blank text"),
		ActiveWork:      r.str(m, p, "active_work", textRe, "non-blank text"),
		Effort:          r.enum(m, p, "effort", "S", "M", "L"),
	}
	if r.err == nil {
		e.Binding = r.binding(m["binding"], p+".binding")
	}
	return e
}

// binding decodes what the binding carries, structurally only. Whether it is
// bound (exactly one of a patch with a base revision and digest, or a
// non-empty follow-up brief) is decided in one place, checkBinding, so a
// decoded manifest and one built in code meet the same rule.
func (r *reader) binding(v any, p string) Binding {
	if r.err != nil || v == nil {
		return Binding{}
	}
	bm, ok := v.(map[string]any)
	if !ok {
		r.fail(RuleAmendUnbound, p, "binding is %s, not a patch or follow-up binding", kindOf(v))
		return Binding{}
	}
	if k := firstUnknown(bm, "patch", "follow_up"); k != "" {
		r.fail(RuleUnknownField, p+"."+k, "field is not defined by this version; a reader never drops a field it does not understand")
		return Binding{}
	}
	var b Binding
	if pv, present := bm["patch"]; present {
		pm, ok := pv.(map[string]any)
		if !ok {
			r.fail(RuleAmendUnbound, p+".patch", "patch is %s, not a base revision and patch digest", kindOf(pv))
			return Binding{}
		}
		if k := firstUnknown(pm, "base_revision", "digest"); k != "" {
			r.fail(RuleUnknownField, p+".patch."+k, "field is not defined by this version; a reader never drops a field it does not understand")
			return Binding{}
		}
		b.Patch = &Patch{
			BaseRevision: r.bindingStr(pm["base_revision"], p+".patch.base_revision"),
			Digest:       r.bindingStr(pm["digest"], p+".patch.digest"),
		}
	}
	if fv, present := bm["follow_up"]; present {
		if fv == nil {
			r.fail(RuleAmendUnbound, p+".follow_up", "follow-up must be a brief reference, not null")
			return Binding{}
		}
		s := r.bindingStr(fv, p+".follow_up")
		if s != "" {
			s = r.qidValue(fv, p+".follow_up")
		}
		b.FollowUp = &s
	}
	return b
}

// bindingStr reads a binding string, where absent reads as empty and any
// non-string is unbound.
func (r *reader) bindingStr(v any, path string) string {
	if r.err != nil || v == nil {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		r.fail(RuleAmendUnbound, path, "want string, got %s", kindOf(v))
	}
	return s
}

func firstUnknown(m map[string]any, keys ...string) string {
	var unknown []string
	for k := range m {
		known := false
		for _, want := range keys {
			known = known || k == want
		}
		if !known {
			unknown = append(unknown, k)
		}
	}
	if len(unknown) == 0 {
		return ""
	}
	sort.Strings(unknown)
	return unknown[0]
}

// deliveryPaths checks only lexical record data. Filesystem confinement (for
// example, symlinks) remains the responsibility of the eventual writer.
func (r *reader) deliveryPaths(m map[string]any, p string) []string {
	paths := r.strArray(m, p, "delivery_paths", 1, locatorRe, "repo-relative path")
	for i, path := range paths {
		if !isDeliveryPath(path) {
			r.fail(RuleBadValue, idx(p+".delivery_paths", i), "%q is not a repo-relative path", path)
		}
	}
	return paths
}

func isDeliveryPath(path string) bool {
	if path == "" || strings.HasPrefix(path, "/") || strings.HasPrefix(path, "~") ||
		strings.Contains(path, "\\") || schemeRe.MatchString(path) {
		return false
	}
	for _, segment := range strings.Split(path, "/") {
		if segment == ".." {
			return false
		}
	}
	for _, c := range path {
		if c < 0x20 || c >= 0x7f && c <= 0x9f || c == 0x061c || c == 0x200e || c == 0x200f ||
			c >= 0x202a && c <= 0x202e || c >= 0x2066 && c <= 0x2069 {
			return false
		}
	}
	return true
}

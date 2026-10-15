package main

// instance.go — the workflow-instance-v1 contract and `statusgen instance
// validate --file PATH [--root DIR] [--prior PATH] [--cell CELL] [--json]`
// (graph-execution/09).
//
// WHAT AN INSTANCE IS. A workflow-pattern-v1 file (patterns.go) states the shape
// of a kind of work. An instance binds one reviewed pattern to ONE canonical work
// item in ONE Cell: the pattern's name, version and file digest, the acceptance
// version and scope, the policy and control-profile revisions, immutable input
// references, a dependency manifest, a budget reference and one node instance per
// pattern node, each carrying its resolved actor and the effects it may perform.
// spec/workflow-instance-v1.md is the normative text; schemas/workflow-instance-v1.json
// is the shape.
//
// THREE LAYERS OF ONE DOCUMENT, WITH DIFFERENT CHANGE RULES.
//   - The CONTRACT (everything but attempts, decisions, assignment and
//     execution-state) is immutable. Its sha256 is the contract digest; a change to
//     any of it is a NEW instance that names the old one in `supersedes`.
//   - Attempts and decisions are APPEND-ONLY records. A later revision of the same
//     instance may add to them, never change or drop one (checkInstanceRevision).
//   - Assignment and execution-state are a versioned PROJECTION keyed by
//     generation: a backend change advances the generation, never reuses it.
//
// IDENTITIES. The canonical work identifier `<stream>/<NN>` stays the work's
// identity. The instance, node instance (`<instance>.<node>`) and attempt
// (`<node instance>.a<N>`) identifiers are ADDITIONAL identities this file is the
// single owner of; later records key on them rather than minting a second scheme.
//
// WHAT THIS FILE REUSES, NOT RE-IMPLEMENTS. Shape is conform.go's minimal JSON
// Schema validator (parseSchema/validateValue). The pattern is resolved and linted
// with patterns.go's lintPatternFile/parsePatternDoc, and the role table is
// patterns.go's patternRoleEffectPermissions. Acceptance currency reads the work
// item's brief-v2 `version:` through the stream loader, and the eligibility verdict
// reported beside the result is eligibility.go's eligibilityForStreams — reported,
// never folded into the instance verdict (admission is a separate predicate).
//
// NO AUTHORITY HERE. An instance records actor capabilities a roster binding
// already holds; it never grants one and never carries credential material.
// Human decisions are references to external authenticated acts, not a model
// role with its own token. The role packet (projectPacket, below) is a view
// projected from this record, never a second run record; the handoff a role
// returns is bound back to the same instance, node and attempt identifiers.
//
// THREE-STATE, SAME CONTRACT AS patterns/conform: checked-clean (0) /
// checked-failed (1) / could-not-check (2), fail-closed.

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// embeddedInstanceSchemaFS holds the committed workflow-instance-v1 schema, for
// the same reason patterns.go embeds the pattern schema: a pinned binary runs
// against an arbitrary --root. TestInstanceSchemaParity pins this copy identical to
// the canonical repo-root file.
//
//go:embed schemas/workflow-instance-v1.json
var embeddedInstanceSchemaFS embed.FS

const embeddedInstanceSchemaName = "schemas/workflow-instance-v1.json"

const instanceSchemaMarker = "workflow-instance-v1"

const (
	instanceExitClean      = 0
	instanceExitFailed     = 1
	instanceExitCouldNot   = 2
	instanceExitUsageError = 2
)

// Rule tags. Findings (checked-failed) and refusals (could-not-check) both carry
// one, so a reader can tell which class fired.
const (
	ruleInstanceSchema          = "instance-schema-violation"
	ruleInstanceCapability      = "instance-unsupported-capability"
	ruleInstanceProjection      = "instance-unsupported-projection"
	ruleInstanceCredential      = "instance-credential-material"
	ruleInstanceIdentity        = "instance-identity-mismatch"
	ruleInstanceCrossCell       = "instance-cross-cell"
	ruleInstanceCapMinted       = "instance-capability-minted"
	ruleInstanceEffectActor     = "instance-effect-exceeds-actor"
	ruleInstanceEffectPattern   = "instance-effect-not-declared"
	ruleInstanceNodeMismatch    = "instance-node-mismatch"
	ruleInstancePatternUnbound  = "instance-pattern-unbound"
	ruleInstancePatternInvalid  = "instance-pattern-invalid"
	ruleInstancePatternDrift    = "instance-pattern-drift"
	ruleInstanceAcceptStale     = "instance-acceptance-stale"
	ruleInstanceAcceptUnknown   = "instance-acceptance-unresolved"
	ruleInstanceTrust           = "instance-trust-not-authoritative"
	ruleInstanceGeneration      = "instance-stale-generation"
	ruleInstanceTruncation      = "instance-silent-truncation"
	ruleInstanceDecisionNode    = "instance-decision-node-unknown"
	ruleInstanceRewrite         = "instance-rewrite-in-place"
	ruleInstanceRecordRewritten = "instance-record-rewritten"
	ruleInstanceSupersession    = "instance-supersession-broken"
	ruleInstanceSubstituted     = "instance-assignment-substituted"
	ruleInstanceSeal            = "instance-seal-mismatch"
	ruleInstanceUnreadable      = "instance-unreadable"
)

// instanceReaderCapabilities is what THIS reader implements. A document whose
// `requires:` names anything else is refused, never evaluated with the unknown
// field ignored.
var instanceReaderCapabilities = map[string]bool{
	"packet-view@1":     true,
	"assignment@1":      true,
	"execution-state@1": true,
}

// instanceWriterRequires is what instantiate() declares in `requires:`: the
// capabilities a document it writes depends on, so an older reader refuses it
// instead of reading the projection and packet view as optional.
var instanceWriterRequires = []string{"packet-view@1", "assignment@1", "execution-state@1"}

// instanceProjectionVersions is the execution-state projection versions this
// reader understands.
var instanceProjectionVersions = map[int]bool{1: true}

// ---- document model ----

type workflowInstance struct {
	Schema         string                 `json:"schema"`
	Digest         string                 `json:"digest,omitempty"`
	Requires       []string               `json:"requires,omitempty"`
	Extensions     map[string]any         `json:"extensions,omitempty"`
	Instance       instanceIdentity       `json:"instance"`
	Work           instanceWork           `json:"work"`
	Pattern        instancePatternRef     `json:"pattern"`
	Acceptance     instanceAcceptance     `json:"acceptance"`
	Policy         instancePolicy         `json:"policy"`
	Budget         instanceBudget         `json:"budget"`
	Assignment     instanceAssignment     `json:"assignment"`
	ExecutionState instanceExecutionState `json:"execution-state"`
	Inputs         []instanceInput        `json:"inputs"`
	Dependencies   []instanceDependency   `json:"dependencies"`
	Nodes          []instanceNode         `json:"nodes"`
	Decisions      []instanceDecision     `json:"decisions,omitempty"`
}

type instanceIdentity struct {
	ID         string              `json:"id"`
	Supersedes *instanceSupersedes `json:"supersedes,omitempty"`
}

type instanceSupersedes struct {
	ID             string `json:"id"`
	ContractDigest string `json:"contract-digest"`
}

type instanceWork struct {
	ID     string `json:"id"`
	Cell   string `json:"cell"`
	Intent string `json:"intent"`
}

type instancePatternRef struct {
	Name    string `json:"name"`
	Version int    `json:"version"`
	Digest  string `json:"digest"`
}

type instanceAcceptance struct {
	Version    int      `json:"version"`
	Ref        string   `json:"ref"`
	Rows       []string `json:"rows"`
	Exclusions []string `json:"exclusions,omitempty"`
}

type instancePolicy struct {
	Revision       string `json:"revision"`
	ControlProfile string `json:"control-profile"`
}

type instanceBudget struct {
	Ref string `json:"ref"`
}

type instanceAssignment struct {
	Backend       string `json:"backend"`
	Generation    int    `json:"generation"`
	RunnerProfile string `json:"runner-profile"`
}

type instanceExecutionState struct {
	ProjectionVersion int    `json:"projection-version"`
	Generation        int    `json:"generation"`
	State             string `json:"state"`
}

type instanceInput struct {
	Name   string        `json:"name"`
	Ref    string        `json:"ref"`
	Digest string        `json:"digest"`
	Origin string        `json:"origin"`
	Trust  instanceTrust `json:"trust"`
}

type instanceTrust struct {
	Disposition   string `json:"disposition"`
	Source        string `json:"source"`
	ReportedLabel string `json:"reported-label,omitempty"`
}

type instanceDependency struct {
	Ref         string   `json:"ref"`
	Kind        string   `json:"kind"`
	Fingerprint string   `json:"fingerprint"`
	Supports    []string `json:"supports"`
}

type instanceNode struct {
	ID       string            `json:"id"`
	Node     string            `json:"node"`
	Role     string            `json:"role"`
	Actor    instanceActor     `json:"actor"`
	Effects  []instanceEffect  `json:"effects"`
	Evidence []instanceClaim   `json:"evidence"`
	Attempts []instanceAttempt `json:"attempts,omitempty"`
}

type instanceActor struct {
	Binding      string   `json:"binding"`
	Cell         string   `json:"cell"`
	Capabilities []string `json:"capabilities"`
}

type instanceEffect struct {
	Kind   string `json:"kind"`
	Target string `json:"target"`
}

type instanceClaim struct {
	Claim     string `json:"claim"`
	Mandatory bool   `json:"mandatory"`
}

type instanceAttempt struct {
	ID             string                `json:"id"`
	Ordinal        int                   `json:"ordinal"`
	Observed       []instanceObservation `json:"observed,omitempty"`
	Artifacts      []instanceArtifact    `json:"artifacts,omitempty"`
	Evidence       []instanceEvidence    `json:"evidence,omitempty"`
	Omissions      []instanceOmission    `json:"omissions,omitempty"`
	Completed      []string              `json:"completed,omitempty"`
	NextSafeAction string                `json:"next-safe-action,omitempty"`
}

type instanceObservation struct {
	Ref         string `json:"ref"`
	Fingerprint string `json:"fingerprint"`
}

type instanceArtifact struct {
	Ref           string `json:"ref"`
	Digest        string `json:"digest"`
	Bytes         int    `json:"bytes"`
	IncludedBytes *int   `json:"included-bytes,omitempty"`
	Index         string `json:"index,omitempty"`
}

type instanceEvidence struct {
	Claim    string `json:"claim"`
	Artifact string `json:"artifact"`
}

type instanceOmission struct {
	Ref    string `json:"ref"`
	Reason string `json:"reason"`
}

type instanceDecision struct {
	ID      string      `json:"id"`
	Node    string      `json:"node"`
	Outcome string      `json:"outcome"`
	Act     instanceAct `json:"act"`
}

type instanceAct struct {
	Kind string `json:"kind"`
	Ref  string `json:"ref"`
}

// instanceFinding is one tagged message: a checked-failed finding or a
// could-not-check refusal, depending on which list carries it.
type instanceFinding struct {
	Rule    string `json:"rule"`
	Message string `json:"message"`
}

func (f instanceFinding) String() string { return fmt.Sprintf("[%s] %s", f.Rule, f.Message) }

func instFinding(rule, format string, a ...any) instanceFinding {
	return instanceFinding{Rule: rule, Message: fmt.Sprintf(format, a...)}
}

func embeddedInstanceSchemaBytes() []byte {
	b, err := embeddedInstanceSchemaFS.ReadFile(embeddedInstanceSchemaName)
	if err != nil {
		panic(fmt.Sprintf("statusgen: embedded %s unreadable: %v", embeddedInstanceSchemaName, err))
	}
	return b
}

func instanceSchema() (*schemaNode, error) {
	return parseSchema(embeddedInstanceSchemaBytes())
}

// ---- decoding: shape, capability refusal, credential exclusion ----

// decodeInstance parses raw (YAML or JSON — JSON is a YAML subset) into the
// document model. It applies, in order: the old-reader capability refusal (a
// document requiring something this reader does not implement is never
// evaluated further), the schema, and the credential scan over every key and
// string value. findings are checked-failed; refusals are could-not-check.
//
// Every message it returns crosses redactFindings first: a schema finding
// quotes the offending value (a pattern or enum violation), so a
// credential-shaped value in such a field would otherwise be echoed into the
// report, its JSON form and the import error. Redacting here, at the one exit,
// covers every message the decode produces, present and future.
func decodeInstance(raw []byte, schema *schemaNode) (workflowInstance, []instanceFinding, []instanceFinding) {
	inst, findings, refusals, data := decodeInstanceBody(raw, schema)
	lits := credentialLiterals(data)
	return inst, redactFindings(findings, lits), redactFindings(refusals, lits)
}

// parseSyntaxLine and parseTypeLine match the parser's OWN position prefix and
// nothing else: a syntax or duplicate-key failure is one string that begins
// "yaml: line N:", and an unmarshal failure is a TypeError whose entries each
// begin "line N:". A "line N" that appears later inside an entry is part of the
// quoted scalar (or of the "already defined at line M" tail) and is never read.
var (
	parseSyntaxLine = regexp.MustCompile(`^yaml: line (\d+):`)
	parseTypeLine   = regexp.MustCompile(`^line (\d+):`)
)

// parseFailureMessage is the one message a YAML/JSON parse failure produces.
// The parser's own text quotes the offending scalar (a duplicate key, a tag
// mismatch), and at that point no decoded document exists to supply the
// literals redaction needs, so the text is never forwarded: the message names
// the position only, read from the position prefix the parser itself wrote.
// TypeError entries are separate strings, so a scalar holding a newline and a
// "line N:" cannot start a forged entry.
func parseFailureMessage(err error) string {
	var lines []string
	seen := map[string]bool{}
	take := func(re *regexp.Regexp, text string) {
		if m := re.FindStringSubmatch(text); m != nil && !seen[m[1]] && len(lines) < 3 {
			seen[m[1]] = true
			lines = append(lines, m[1])
		}
	}
	var te *yaml.TypeError
	if errors.As(err, &te) {
		for _, e := range te.Errors {
			take(parseTypeLine, e)
		}
	} else {
		take(parseSyntaxLine, err.Error())
	}
	msg := "the document is not parseable as YAML or JSON"
	if len(lines) > 0 {
		msg += " (near line " + strings.Join(lines, ", ") + ")"
	}
	return msg + "; the parser's detail is withheld because it can quote a value"
}

// decodeInstanceBody is decodeInstance before redaction; only decodeInstance
// calls it.
func decodeInstanceBody(raw []byte, schema *schemaNode) (inst workflowInstance, findings, refusals []instanceFinding, data map[string]any) {
	if err := yaml.Unmarshal(raw, &data); err != nil {
		return inst, nil, []instanceFinding{instFinding(ruleInstanceUnreadable, "%s", parseFailureMessage(err))}, nil
	}
	if data == nil {
		return inst, nil, []instanceFinding{instFinding(ruleInstanceUnreadable, "empty document")}, nil
	}
	if marker, _ := data["schema"].(string); marker != instanceSchemaMarker {
		return inst, nil, []instanceFinding{instFinding(ruleInstanceUnreadable,
			"`schema:` is %q, not %q — an instance must declare its own schema", marker, instanceSchemaMarker)}, data
	}

	// Old-reader refusal FIRST: the schema below would also flag an unknown
	// top-level field, but a capability the document says is mandatory is a
	// statement that this reader cannot evaluate it — could-not-check, not a
	// shape defect to be reported alongside a verdict.
	if reqs, ok := data["requires"].([]any); ok {
		for _, r := range reqs {
			s, _ := r.(string)
			if !instanceReaderCapabilities[s] {
				refusals = append(refusals, instFinding(ruleInstanceCapability,
					"the document requires capability %q, which this reader does not implement — refusing rather than ignoring a mandatory field", s))
			}
		}
	}
	if es, ok := data["execution-state"].(map[string]any); ok {
		if pv, ok := es["projection-version"].(int); ok && pv >= 1 && !instanceProjectionVersions[pv] {
			refusals = append(refusals, instFinding(ruleInstanceProjection,
				"execution-state projection-version %d is not one this reader understands", pv))
		}
	}
	if len(refusals) > 0 {
		return inst, nil, refusals, data
	}

	for _, v := range validateValue(schema, data, "") {
		findings = append(findings, instFinding(ruleInstanceSchema, "%s", v))
	}
	findings = append(findings, scanCredentialMaterial(data, "")...)

	// Decode the typed view. A document that failed the schema may not decode;
	// the findings above already say why, so a decode error is only reported
	// when the shape was otherwise clean.
	b, err := json.Marshal(data)
	if err == nil {
		dec := json.NewDecoder(bytes.NewReader(b))
		dec.DisallowUnknownFields()
		err = dec.Decode(&inst)
	}
	if err != nil && len(findings) == 0 {
		findings = append(findings, instFinding(ruleInstanceSchema, "does not decode: %v", err))
	}
	return inst, findings, nil, data
}

// Credential-shaped content. These recognise the SHAPE of well-known token and
// key formats and of authorization headers; an instance carries references to
// credentials held elsewhere, never the material. The test suite builds its
// synthetic secrets at run time so no secret-shaped literal lives in the tree.
var (
	instanceCredentialValue = []*regexp.Regexp{
		regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9]{20,}`),
		regexp.MustCompile(`\bgithub_pat_[A-Za-z0-9_]{20,}`),
		regexp.MustCompile(`\bglpat-[A-Za-z0-9_-]{20,}`),
		regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`),
		regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{20,}`),
		regexp.MustCompile(`\bxox[abpr]-[A-Za-z0-9-]{10,}`),
		regexp.MustCompile(`-{5}BEGIN [A-Z ]*PRIVATE KEY`),
		regexp.MustCompile(`(?i)\b(bearer|basic)\s+[A-Za-z0-9._~+/=-]{8,}`),
		regexp.MustCompile(`(?i)^\s*authorization\s*:`),
		regexp.MustCompile(`(?i)://[^/\s:@]+:[^/\s@]+@`),
	}
	// instanceCredentialInMessage is the value list's authorization-header
	// pattern without its line anchor: inside a message the header is not at
	// the start of the string. It is for redacting messages only; the scan
	// keeps the anchored form so prose that mentions "authorization:" is not
	// a finding.
	instanceCredentialInMessage = regexp.MustCompile(`(?i)\bauthorization\s*:\s*[^"'\x60\n]*`)
	instanceCredentialKey       = regexp.MustCompile(`(?i)(^|[-_])(token|password|passwd|secret|api-?key|private-?key|authorization|credentials?)($|[-_])`)
)

// instanceRedacted stands in for a credential-shaped value or key in any
// message: findings name where the material is, never what it is.
const instanceRedacted = "[redacted]"

func credentialShaped(s string) bool {
	for _, re := range instanceCredentialValue {
		if re.MatchString(s) {
			return true
		}
	}
	return false
}

// scanCredentialMaterial walks every key and string value of a decoded
// document. A credential-named key, a credential-shaped key or a
// credential-shaped value is a finding. A credential-shaped key is reported as
// a redacted path segment, so the path itself never carries the material.
func scanCredentialMaterial(v any, path string) []instanceFinding {
	var out []instanceFinding
	switch t := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			seg := k
			if credentialShaped(k) {
				seg = instanceRedacted
			}
			p := path + "/" + seg
			switch {
			case seg == instanceRedacted:
				out = append(out, instFinding(ruleInstanceCredential,
					"%s: a field name has the shape of credential material — replace it with an authorized reference", p))
			case instanceCredentialKey.MatchString(k):
				out = append(out, instFinding(ruleInstanceCredential,
					"%s: a field named like credential material — carry an authorized reference to it, never the value", p))
			}
			out = append(out, scanCredentialMaterial(t[k], p)...)
		}
	case []any:
		for i, e := range t {
			out = append(out, scanCredentialMaterial(e, path+"/"+strconv.Itoa(i))...)
		}
	case string:
		if credentialShaped(t) {
			out = append(out, instFinding(ruleInstanceCredential,
				"%s: value has the shape of credential material — replace it with an authorized reference", path))
		}
	}
	return out
}

// credentialLiterals collects every credential-shaped key and string value of
// a decoded document — the exact text redactFindings removes from messages.
func credentialLiterals(v any) []string {
	var out []string
	var walk func(any)
	walk = func(v any) {
		switch t := v.(type) {
		case map[string]any:
			for k, e := range t {
				if credentialShaped(k) {
					out = append(out, k)
				}
				walk(e)
			}
		case []any:
			for _, e := range t {
				walk(e)
			}
		case string:
			if credentialShaped(t) {
				out = append(out, t)
			}
		}
	}
	walk(v)
	// Longest first, so a literal containing another is removed whole.
	sort.Slice(out, func(i, j int) bool { return len(out[i]) > len(out[j]) })
	return out
}

// redactFindings removes credential material from messages: each literal the
// document carries, raw and in its quoted (%q) form, and then anything else
// with a credential shape. A finding keeps its rule and its location.
func redactFindings(fs []instanceFinding, literals []string) []instanceFinding {
	if fs == nil {
		return nil
	}
	out := make([]instanceFinding, len(fs))
	for i, f := range fs {
		out[i] = instanceFinding{Rule: f.Rule, Message: redactText(f.Message, literals)}
	}
	return out
}

func redactText(msg string, literals []string) string {
	for _, l := range literals {
		if q := strconv.Quote(l); len(q) > 2 {
			msg = strings.ReplaceAll(msg, q[1:len(q)-1], instanceRedacted)
		}
		msg = strings.ReplaceAll(msg, l, instanceRedacted)
	}
	for _, pat := range instanceCredentialValue {
		msg = pat.ReplaceAllString(msg, instanceRedacted)
	}
	return instanceCredentialInMessage.ReplaceAllString(msg, instanceRedacted)
}

// ---- digests ----

func sha256Ref(b []byte) string {
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// instanceContract is the immutable part of an instance: everything but the
// export seal, the append-only records and the generation-keyed projection.
func instanceContract(in workflowInstance) workflowInstance {
	c := canonicalInstance(in)
	c.Digest = ""
	c.Assignment = instanceAssignment{}
	c.ExecutionState = instanceExecutionState{}
	c.Decisions = nil
	for i := range c.Nodes {
		c.Nodes[i].Attempts = nil
	}
	return c
}

// contractDigest is the instance's content identity: sha256 over the canonical
// JSON of its contract. Two documents with one instance id and different
// contract digests are an in-place rewrite.
func contractDigest(in workflowInstance) string {
	b, err := json.Marshal(instanceContract(in))
	if err != nil {
		panic(fmt.Sprintf("statusgen: instance contract does not marshal: %v", err))
	}
	return sha256Ref(b)
}

// sealDigest is the export seal over the whole canonical document. It is an
// UNKEYED content digest: it detects accidental alteration, never deliberate
// alteration, since anyone holding the document can recompute it. It
// authenticates nothing; authenticity comes from the store or transport.
func sealDigest(in workflowInstance) string {
	c := canonicalInstance(in)
	c.Digest = ""
	b, err := json.Marshal(c)
	if err != nil {
		panic(fmt.Sprintf("statusgen: instance does not marshal: %v", err))
	}
	return sha256Ref(b)
}

// canonicalInstance is the explicit, field-by-field copy export serialises.
// Every reference is carried exactly; nothing is recomputed or defaulted, so an
// export→import round trip returns the same references the instantiator wrote.
func canonicalInstance(in workflowInstance) workflowInstance {
	out := workflowInstance{
		Schema:     in.Schema,
		Digest:     in.Digest,
		Requires:   append([]string(nil), in.Requires...),
		Extensions: in.Extensions,
		Instance:   instanceIdentity{ID: in.Instance.ID},
		Work:       in.Work,
		Pattern:    in.Pattern,
		Acceptance: instanceAcceptance{
			Version:    in.Acceptance.Version,
			Ref:        in.Acceptance.Ref,
			Rows:       append([]string(nil), in.Acceptance.Rows...),
			Exclusions: append([]string(nil), in.Acceptance.Exclusions...),
		},
		Policy: in.Policy,
		Budget: in.Budget,
		Assignment: instanceAssignment{
			Backend:       in.Assignment.Backend,
			Generation:    in.Assignment.Generation,
			RunnerProfile: in.Assignment.RunnerProfile,
		},
		ExecutionState: in.ExecutionState,
	}
	if in.Instance.Supersedes != nil {
		s := *in.Instance.Supersedes
		out.Instance.Supersedes = &s
	}
	for _, x := range in.Inputs {
		out.Inputs = append(out.Inputs, x)
	}
	for _, d := range in.Dependencies {
		d.Supports = append([]string{}, d.Supports...)
		out.Dependencies = append(out.Dependencies, d)
	}
	for _, n := range in.Nodes {
		cn := n
		cn.Actor.Capabilities = append([]string{}, n.Actor.Capabilities...)
		cn.Effects = append([]instanceEffect{}, n.Effects...)
		cn.Evidence = append([]instanceClaim{}, n.Evidence...)
		cn.Attempts = nil
		for _, a := range n.Attempts {
			ca := a
			ca.Observed = append([]instanceObservation(nil), a.Observed...)
			ca.Artifacts = append([]instanceArtifact(nil), a.Artifacts...)
			ca.Evidence = append([]instanceEvidence(nil), a.Evidence...)
			ca.Omissions = append([]instanceOmission(nil), a.Omissions...)
			ca.Completed = append([]string(nil), a.Completed...)
			cn.Attempts = append(cn.Attempts, ca)
		}
		out.Nodes = append(out.Nodes, cn)
	}
	out.Decisions = append([]instanceDecision(nil), in.Decisions...)
	if out.Inputs == nil {
		out.Inputs = []instanceInput{}
	}
	if out.Dependencies == nil {
		out.Dependencies = []instanceDependency{}
	}
	return out
}

// ---- export / import ----

// exportInstance serialises an instance as sealed canonical JSON.
func exportInstance(in workflowInstance) ([]byte, error) {
	c := canonicalInstance(in)
	c.Digest = sealDigest(c)
	return json.MarshalIndent(c, "", "  ")
}

// importInstance is the lower boundary every consumer crosses: it re-reads the
// bytes through the same shape, capability and credential checks as validate,
// requires and recomputes the export seal, refuses an instance whose stated Cell
// is not the importing Cell, and re-runs the contract rules that need no
// repository. It returns an error for every refusal; it never repairs a
// document. The seal is unkeyed, so a document altered and re-sealed passes the
// seal check: import checks what the document states (its Cell, its trust
// provenance, its contract) and cannot authenticate where it came from — that is
// the authenticated store or transport's job.
func importInstance(raw []byte, expectCell string) (workflowInstance, error) {
	schema, err := instanceSchema()
	if err != nil {
		return workflowInstance{}, fmt.Errorf("embedded instance schema: %w", err)
	}
	inst, findings, refusals := decodeInstance(raw, schema)
	if len(refusals) > 0 {
		return workflowInstance{}, findingsError("import refused", refusals)
	}
	if len(findings) > 0 {
		return workflowInstance{}, findingsError("import rejected", findings)
	}
	if inst.Digest == "" {
		return workflowInstance{}, findingsError("import rejected", []instanceFinding{instFinding(ruleInstanceSeal,
			"the document carries no export seal — only an exported instance can be imported")})
	}
	if got := sealDigest(inst); got != inst.Digest {
		return workflowInstance{}, findingsError("import rejected", []instanceFinding{instFinding(ruleInstanceSeal,
			"export seal %s does not match the content (%s) — the document was altered after export", inst.Digest, got)})
	}
	if inst.Work.Cell != expectCell {
		return workflowInstance{}, findingsError("import rejected", []instanceFinding{instFinding(ruleInstanceCrossCell,
			"instance %s belongs to Cell %q; this boundary imports for Cell %q", inst.Instance.ID, inst.Work.Cell, expectCell)})
	}
	if fs := checkInstanceRules(inst); len(fs) > 0 {
		return workflowInstance{}, findingsError("import rejected", fs)
	}
	return inst, nil
}

func findingsError(prefix string, fs []instanceFinding) error {
	parts := make([]string, 0, len(fs))
	for _, f := range fs {
		parts = append(parts, f.String())
	}
	return errors.New(prefix + ": " + strings.Join(parts, "; "))
}

// ---- contract rules that need no repository ----

// checkInstanceRules applies every rule the document alone can decide.
func checkInstanceRules(inst workflowInstance) []instanceFinding {
	var out []instanceFinding
	id := inst.Instance.ID

	if s := inst.Instance.Supersedes; s != nil && s.ID == id {
		out = append(out, instFinding(ruleInstanceSupersession, "instance %s names itself in supersedes", id))
	}

	// Generation agreement: the execution-state projection is of THIS assignment.
	if inst.ExecutionState.Generation != inst.Assignment.Generation {
		out = append(out, instFinding(ruleInstanceGeneration,
			"execution-state is projected at generation %d but the assignment is at generation %d",
			inst.ExecutionState.Generation, inst.Assignment.Generation))
	}

	// Trust provenance: only the trust gate decides a disposition.
	for _, in := range inst.Inputs {
		if in.Trust.Source == "reporter" && in.Trust.Disposition != "unresolved" {
			out = append(out, instFinding(ruleInstanceTrust,
				"input %q: a reporter-sourced trust label is recorded as disposition %q — only the trust gate decides a disposition; keep the label in reported-label and the disposition unresolved",
				in.Name, in.Trust.Disposition))
		}
	}

	rows := map[string]bool{}
	for _, r := range inst.Acceptance.Rows {
		rows[r] = true
	}
	for _, d := range inst.Dependencies {
		for _, s := range d.Supports {
			if !rows[s] {
				out = append(out, instFinding(ruleInstanceIdentity,
					"dependency %s supports acceptance row %q, which is not in the acceptance scope", d.Ref, s))
			}
		}
	}

	// One manifest entry per ref: acceptance and the packet bind each
	// dependency once, so a repeated ref could never be current.
	seenDep := map[string]bool{}
	for _, d := range inst.Dependencies {
		if seenDep[d.Ref] {
			out = append(out, instFinding(ruleInstanceIdentity,
				"dependency %s appears more than once in the manifest — one entry per ref", d.Ref))
		}
		seenDep[d.Ref] = true
	}

	nodeIDs := map[string]bool{}
	seenNode := map[string]bool{}
	for _, n := range inst.Nodes {
		want := id + "." + n.Node
		if n.ID != want {
			out = append(out, instFinding(ruleInstanceIdentity, "node instance %q must be %q (instance id . pattern node id)", n.ID, want))
		}
		if seenNode[n.Node] {
			out = append(out, instFinding(ruleInstanceNodeMismatch, "pattern node %q is instantiated more than once", n.Node))
		}
		seenNode[n.Node] = true
		nodeIDs[n.ID] = true

		// One Cell: every actor acts in the instance's own Cell.
		if n.Actor.Cell != inst.Work.Cell {
			out = append(out, instFinding(ruleInstanceCrossCell,
				"node %s: actor %s acts in Cell %q, the instance is in Cell %q — a cross-Cell substitution",
				n.ID, n.Actor.Binding, n.Actor.Cell, inst.Work.Cell))
		}

		// The actor's recorded capabilities never exceed what its role holds:
		// an instance records capability, it does not mint it.
		perm := patternRoleEffectPermissions[n.Role]
		caps := map[string]bool{}
		for _, c := range n.Actor.Capabilities {
			caps[c] = true
			if !perm[c] {
				out = append(out, instFinding(ruleInstanceCapMinted,
					"node %s: actor capability %q is not held by role %q — an instance cannot grant a capability", n.ID, c, n.Role))
			}
		}
		for _, e := range n.Effects {
			if !caps[e.Kind] {
				out = append(out, instFinding(ruleInstanceEffectActor,
					"node %s: effect %s→%s is not among the actor's resolved capabilities %v", n.ID, e.Kind, e.Target, n.Actor.Capabilities))
			}
		}

		for i, a := range n.Attempts {
			wantA := n.ID + ".a" + strconv.Itoa(a.Ordinal)
			if a.ID != wantA {
				out = append(out, instFinding(ruleInstanceIdentity, "attempt %q must be %q (node instance id . a<ordinal>)", a.ID, wantA))
			}
			if a.Ordinal != i+1 {
				out = append(out, instFinding(ruleInstanceIdentity, "node %s: attempt %q has ordinal %d at position %d — attempts are append-only and numbered from 1", n.ID, a.ID, a.Ordinal, i+1))
			}
			for _, art := range a.Artifacts {
				if art.IncludedBytes != nil && *art.IncludedBytes < art.Bytes && art.Index == "" {
					out = append(out, instFinding(ruleInstanceTruncation,
						"attempt %s: artifact %s is excerpted (%d of %d bytes) with no coverage index — record the omission, never truncate silently",
						a.ID, art.Ref, *art.IncludedBytes, art.Bytes))
				}
			}
		}
	}
	for _, d := range inst.Decisions {
		if !nodeIDs[d.Node] {
			out = append(out, instFinding(ruleInstanceDecisionNode, "decision %s names node %q, which is not a node instance of %s", d.ID, d.Node, id))
		}
	}
	return out
}

// ---- rules that need the pattern ----

// checkInstanceAgainstPattern: one node instance per pattern node, roles and
// evidence claims copied exactly, and every effect declared by the pattern node.
func checkInstanceAgainstPattern(inst workflowInstance, pat patternDoc) []instanceFinding {
	var out []instanceFinding
	have := map[string]bool{}
	for _, n := range inst.Nodes {
		have[n.Node] = true
		pn, ok := pat.NodeByID[n.Node]
		if !ok {
			out = append(out, instFinding(ruleInstanceNodeMismatch, "node %s instantiates %q, which pattern %s does not declare", n.ID, n.Node, pat.Name))
			continue
		}
		if n.Role != pn.Role {
			out = append(out, instFinding(ruleInstanceNodeMismatch, "node %s binds role %q; pattern node %q declares role %q", n.ID, n.Role, pn.ID, pn.Role))
		}
		declared := map[string]bool{}
		for _, e := range pn.Effects {
			declared[e.Kind+"\x00"+e.Target] = true
		}
		for _, e := range n.Effects {
			if !declared[e.Kind+"\x00"+e.Target] {
				out = append(out, instFinding(ruleInstanceEffectPattern,
					"node %s: effect %s→%s is not declared by pattern node %q", n.ID, e.Kind, e.Target, pn.ID))
			}
		}
		if !sameClaims(n.Evidence, pn.Evidence) {
			out = append(out, instFinding(ruleInstanceNodeMismatch, "node %s: evidence claims differ from pattern node %q's", n.ID, pn.ID))
		}
	}
	for _, pn := range pat.Nodes {
		if !have[pn.ID] {
			out = append(out, instFinding(ruleInstanceNodeMismatch, "pattern node %q has no node instance", pn.ID))
		}
	}
	return out
}

func sameClaims(got []instanceClaim, want []patternEvidence) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i].Claim != want[i].Claim || got[i].Mandatory != want[i].Mandatory {
			return false
		}
	}
	return true
}

// instancePatternBinding is a pattern resolved from a root for an instance.
type instancePatternBinding struct {
	Path    string
	Doc     patternDoc
	Version int
	Digest  string
}

// resolveInstancePattern finds the pattern file named `name` under
// root/spec/workflow-patterns, lints it with the pattern lint, and returns its
// parsed document, version and file digest. A missing directory is a refusal; a
// directory that holds no pattern of that name is a FINDING — a declared
// instance with no binding cannot fall back to no-pattern behaviour.
func resolveInstancePattern(root, name string) (instancePatternBinding, []instanceFinding, []instanceFinding) {
	var b instancePatternBinding
	dir := filepath.Join(root, "spec", "workflow-patterns")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return b, nil, []instanceFinding{instFinding(ruleInstancePatternUnbound, "cannot read %s: %v", dir, err)}
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var data map[string]any
		if yaml.Unmarshal(raw, &data) != nil || data == nil {
			continue
		}
		if n, _ := data["pattern"].(string); n != name {
			continue
		}
		schema, err := parseSchema(embeddedPatternSchemaBytes())
		if err != nil {
			return b, nil, []instanceFinding{instFinding(ruleInstancePatternInvalid, "embedded pattern schema: %v", err)}
		}
		state, msgs := lintPatternFile(path, schema)
		switch state {
		case patternStateFailed:
			return b, []instanceFinding{instFinding(ruleInstancePatternInvalid, "pattern %s does not pass the pattern lint: %s", name, strings.Join(msgs, "; "))}, nil
		case patternStateCouldNot:
			return b, nil, []instanceFinding{instFinding(ruleInstancePatternInvalid, "pattern %s could not be linted: %s", name, strings.Join(msgs, "; "))}
		}
		doc, err := parsePatternDoc(data)
		if err != nil {
			return b, []instanceFinding{instFinding(ruleInstancePatternInvalid, "pattern %s: %v", name, err)}, nil
		}
		v, _ := data["version"].(int)
		return instancePatternBinding{Path: path, Doc: doc, Version: v, Digest: sha256Ref(raw)}, nil, nil
	}
	return b, []instanceFinding{instFinding(ruleInstancePatternUnbound,
		"pattern %q is not present under %s — a declared instance with a missing binding is refused, never run as no-pattern work", name, dir)}, nil
}

// ---- rules that need the repository ----

// instanceAcceptanceCurrent compares the instance's acceptance version with the
// work item's brief-v2 `version:` in root.
func instanceAcceptanceCurrent(inst workflowInstance, streams []*Stream) ([]instanceFinding, []instanceFinding) {
	stream, num, ok := strings.Cut(inst.Work.ID, "/")
	if !ok {
		return nil, []instanceFinding{instFinding(ruleInstanceAcceptUnknown, "work id %q is not <stream>/<NN>", inst.Work.ID)}
	}
	for _, s := range streams {
		if s.Name != stream {
			continue
		}
		path := findBriefFile(s.Dir, num)
		if path == "" {
			break
		}
		bf, isBrief, err := parseBriefFile(path)
		if err != nil || !isBrief || bf.Version < 1 {
			return nil, []instanceFinding{instFinding(ruleInstanceAcceptUnknown,
				"work item %s has no readable brief-v2 version (%s)", inst.Work.ID, filepath.Base(path))}
		}
		if bf.Version != inst.Acceptance.Version {
			return []instanceFinding{instFinding(ruleInstanceAcceptStale,
				"instance is bound to acceptance version %d; work item %s is at version %d — bind a superseding instance to the current acceptance",
				inst.Acceptance.Version, inst.Work.ID, bf.Version)}, nil
		}
		return nil, nil
	}
	return nil, []instanceFinding{instFinding(ruleInstanceAcceptUnknown, "work item %s is not present in this root", inst.Work.ID)}
}

// checkInstanceRevision compares two documents with the SAME instance id: the
// contract is immutable, records are append-only, and the assignment projection
// moves forward by generation.
func checkInstanceRevision(prior, next workflowInstance) []instanceFinding {
	var out []instanceFinding
	if pc, nc := contractDigest(prior), contractDigest(next); pc != nc {
		out = append(out, instFinding(ruleInstanceRewrite,
			"instance %s: contract digest moved from %s to %s under the same id — a changed binding must be a new instance that supersedes this one",
			next.Instance.ID, pc, nc))
	}
	nextNodes := map[string]instanceNode{}
	for _, n := range next.Nodes {
		nextNodes[n.ID] = n
	}
	for _, pn := range prior.Nodes {
		nn := nextNodes[pn.ID]
		for i, pa := range pn.Attempts {
			if i >= len(nn.Attempts) {
				out = append(out, instFinding(ruleInstanceRecordRewritten, "attempt %s was dropped", pa.ID))
				continue
			}
			if !jsonEqual(pa, nn.Attempts[i]) {
				out = append(out, instFinding(ruleInstanceRecordRewritten, "attempt %s was rewritten", pa.ID))
			}
		}
	}
	nextDec := map[string]instanceDecision{}
	for _, d := range next.Decisions {
		nextDec[d.ID] = d
	}
	for _, pd := range prior.Decisions {
		nd, ok := nextDec[pd.ID]
		if !ok {
			out = append(out, instFinding(ruleInstanceRecordRewritten, "decision %s was dropped", pd.ID))
		} else if !jsonEqual(pd, nd) {
			out = append(out, instFinding(ruleInstanceRecordRewritten, "decision %s was rewritten", pd.ID))
		}
	}
	pa, na := prior.Assignment, next.Assignment
	switch {
	case na.Generation < pa.Generation:
		out = append(out, instFinding(ruleInstanceSubstituted, "assignment generation went backwards (%d → %d)", pa.Generation, na.Generation))
	case na.Generation == pa.Generation && (na.Backend != pa.Backend || na.RunnerProfile != pa.RunnerProfile):
		out = append(out, instFinding(ruleInstanceSubstituted,
			"assignment changed (%s/%s → %s/%s) without advancing generation %d — a substitution, not a transfer",
			pa.Backend, pa.RunnerProfile, na.Backend, na.RunnerProfile, pa.Generation))
	}
	return out
}

// checkSupersession: a superseding instance names the prior by id and contract
// digest, keeps the work identity and Cell, and carries none of the prior's
// decisions as its own (they stay in the prior's history).
func checkSupersession(prior, next workflowInstance) []instanceFinding {
	var out []instanceFinding
	s := next.Instance.Supersedes
	if s == nil || s.ID != prior.Instance.ID || s.ContractDigest != contractDigest(prior) {
		out = append(out, instFinding(ruleInstanceSupersession,
			"instance %s does not name %s at contract digest %s in supersedes", next.Instance.ID, prior.Instance.ID, contractDigest(prior)))
	}
	if next.Work.ID != prior.Work.ID || next.Work.Cell != prior.Work.Cell {
		out = append(out, instFinding(ruleInstanceSupersession,
			"a superseding instance keeps the work identity and Cell (%s in %s), got %s in %s",
			prior.Work.ID, prior.Work.Cell, next.Work.ID, next.Work.Cell))
	}
	priorDec := map[string]bool{}
	for _, d := range prior.Decisions {
		priorDec[d.ID] = true
	}
	for _, d := range next.Decisions {
		if priorDec[d.ID] {
			out = append(out, instFinding(ruleInstanceSupersession,
				"decision %s belongs to the superseded instance %s; it stays in that history and is not carried forward", d.ID, prior.Instance.ID))
		}
	}
	return out
}

// checkAgainstPrior dispatches on whether next is a revision of prior (same id)
// or a successor (different id).
func checkAgainstPrior(prior, next workflowInstance) []instanceFinding {
	if prior.Instance.ID == next.Instance.ID {
		return checkInstanceRevision(prior, next)
	}
	return checkSupersession(prior, next)
}

func jsonEqual(a, b any) bool {
	ab, err1 := json.Marshal(a)
	bb, err2 := json.Marshal(b)
	return err1 == nil && err2 == nil && bytes.Equal(ab, bb)
}

// ---- instantiation ----

// instanceBinding is what an instantiator resolves before it writes an
// instance: the work, the Cell, the pinned references and one resolved actor
// per role the pattern uses.
type instanceBinding struct {
	ID           string
	WorkID       string
	Cell         string
	Intent       string
	Acceptance   instanceAcceptance
	Policy       instancePolicy
	Budget       instanceBudget
	Assignment   instanceAssignment
	Inputs       []instanceInput
	Dependencies []instanceDependency
	Actors       map[string]instanceActor // role -> resolved actor
	Supersedes   *instanceSupersedes
}

// instantiate writes the instance for pattern pb under binding b. Every node
// instance carries exactly the pattern node's effects; an actor whose resolved
// capabilities do not cover them is a refusal — the instantiator never widens an
// actor to fit a pattern.
func instantiate(pb instancePatternBinding, b instanceBinding) (workflowInstance, error) {
	inst := workflowInstance{
		Schema:     instanceSchemaMarker,
		Requires:   append([]string(nil), instanceWriterRequires...),
		Instance:   instanceIdentity{ID: b.ID, Supersedes: b.Supersedes},
		Work:       instanceWork{ID: b.WorkID, Cell: b.Cell, Intent: b.Intent},
		Pattern:    instancePatternRef{Name: pb.Doc.Name, Version: pb.Version, Digest: pb.Digest},
		Acceptance: b.Acceptance,
		Policy:     b.Policy,
		Budget:     b.Budget,
		Assignment: b.Assignment,
		ExecutionState: instanceExecutionState{
			ProjectionVersion: 1,
			Generation:        b.Assignment.Generation,
			State:             "pending",
		},
		Inputs:       b.Inputs,
		Dependencies: b.Dependencies,
	}
	for _, pn := range pb.Doc.Nodes {
		actor, ok := b.Actors[pn.Role]
		if !ok {
			return workflowInstance{}, fmt.Errorf("no resolved actor for role %q (pattern node %q)", pn.Role, pn.ID)
		}
		caps := map[string]bool{}
		for _, c := range actor.Capabilities {
			caps[c] = true
		}
		n := instanceNode{
			ID:    b.ID + "." + pn.ID,
			Node:  pn.ID,
			Role:  pn.Role,
			Actor: actor,
		}
		for _, e := range pn.Effects {
			if !caps[e.Kind] {
				return workflowInstance{}, fmt.Errorf("actor %s for role %q lacks capability %q that pattern node %q declares", actor.Binding, pn.Role, e.Kind, pn.ID)
			}
			n.Effects = append(n.Effects, instanceEffect{Kind: e.Kind, Target: e.Target})
		}
		if n.Effects == nil {
			n.Effects = []instanceEffect{}
		}
		for _, ev := range pn.Evidence {
			n.Evidence = append(n.Evidence, instanceClaim{Claim: ev.Claim, Mandatory: ev.Mandatory})
		}
		if n.Evidence == nil {
			n.Evidence = []instanceClaim{}
		}
		inst.Nodes = append(inst.Nodes, n)
	}
	return canonicalInstance(inst), nil
}

// ---- validate ----

// instanceReport is the --json output of `statusgen instance validate`.
type instanceReport struct {
	File           string            `json:"file"`
	State          string            `json:"state"` // checked-clean | checked-failed | could-not-check
	InstanceID     string            `json:"instance-id,omitempty"`
	Work           string            `json:"work,omitempty"`
	Cell           string            `json:"cell,omitempty"`
	ContractDigest string            `json:"contract-digest,omitempty"`
	Findings       []instanceFinding `json:"findings"`
	CouldNotCheck  []instanceFinding `json:"could-not-check"`
	Eligibility    *Eligibility      `json:"eligibility,omitempty"`
	EligibilityGap string            `json:"eligibility-could-not-check,omitempty"`
}

type instanceValidateOptions struct {
	File  string
	Root  string
	Prior string
	Cell  string
}

// validateInstanceFile is the whole validate pipeline, shared by the CLI and the
// tests: decode (capability, shape, credentials) → document rules → pattern
// resolution and pattern rules → acceptance currency → prior revision or
// supersession → Cell expectation. Eligibility is attached for the reader and
// never changes the state.
func validateInstanceFile(opts instanceValidateOptions) instanceReport {
	rep := instanceReport{File: opts.File, Findings: []instanceFinding{}, CouldNotCheck: []instanceFinding{}}
	finish := func() instanceReport {
		sortFindings(rep.Findings)
		sortFindings(rep.CouldNotCheck)
		switch {
		case len(rep.Findings) > 0:
			rep.State = "checked-failed"
		case len(rep.CouldNotCheck) > 0:
			rep.State = "could-not-check"
		default:
			rep.State = "checked-clean"
		}
		return rep
	}
	schema, err := instanceSchema()
	if err != nil {
		rep.CouldNotCheck = append(rep.CouldNotCheck, instFinding(ruleInstanceUnreadable, "embedded instance schema: %v", err))
		return finish()
	}
	raw, err := os.ReadFile(opts.File)
	if err != nil {
		rep.CouldNotCheck = append(rep.CouldNotCheck, instFinding(ruleInstanceUnreadable, "%v", err))
		return finish()
	}
	inst, fs, refusals := decodeInstance(raw, schema)
	rep.Findings = append(rep.Findings, fs...)
	rep.CouldNotCheck = append(rep.CouldNotCheck, refusals...)
	if len(refusals) > 0 || len(fs) > 0 {
		return finish()
	}
	rep.InstanceID, rep.Work, rep.Cell = inst.Instance.ID, inst.Work.ID, inst.Work.Cell
	rep.ContractDigest = contractDigest(inst)
	if inst.Digest != "" && sealDigest(inst) != inst.Digest {
		rep.Findings = append(rep.Findings, instFinding(ruleInstanceSeal, "export seal %s does not match the content — altered after export", inst.Digest))
	}
	rep.Findings = append(rep.Findings, checkInstanceRules(inst)...)

	if opts.Cell != "" && inst.Work.Cell != opts.Cell {
		rep.Findings = append(rep.Findings, instFinding(ruleInstanceCrossCell,
			"instance %s belongs to Cell %q; validated for Cell %q", inst.Instance.ID, inst.Work.Cell, opts.Cell))
	}

	root := opts.Root
	if root == "" {
		root = "."
	}
	pb, pfs, prefs := resolveInstancePattern(root, inst.Pattern.Name)
	rep.Findings = append(rep.Findings, pfs...)
	rep.CouldNotCheck = append(rep.CouldNotCheck, prefs...)
	if len(pfs) == 0 && len(prefs) == 0 {
		if pb.Version != inst.Pattern.Version || pb.Digest != inst.Pattern.Digest {
			rep.Findings = append(rep.Findings, instFinding(ruleInstancePatternDrift,
				"instance pins pattern %s v%d at %s; the root holds v%d at %s — rebind with a superseding instance",
				inst.Pattern.Name, inst.Pattern.Version, inst.Pattern.Digest, pb.Version, pb.Digest))
		}
		rep.Findings = append(rep.Findings, checkInstanceAgainstPattern(inst, pb.Doc)...)
	}

	streams, _, serr := loadHydratedStreams(root)
	if serr != nil {
		rep.CouldNotCheck = append(rep.CouldNotCheck, instFinding(ruleInstanceAcceptUnknown, "cannot load streams under %s: %v", root, serr))
		rep.EligibilityGap = serr.Error()
	} else {
		afs, arefs := instanceAcceptanceCurrent(inst, streams)
		rep.Findings = append(rep.Findings, afs...)
		rep.CouldNotCheck = append(rep.CouldNotCheck, arefs...)
		if e, ok := eligibilityForStreams(streams)[inst.Work.ID]; ok {
			rep.Eligibility = &e
		} else {
			rep.EligibilityGap = "work item " + inst.Work.ID + " has no eligibility verdict in this root"
		}
	}

	if opts.Prior != "" {
		praw, err := os.ReadFile(opts.Prior)
		if err != nil {
			rep.CouldNotCheck = append(rep.CouldNotCheck, instFinding(ruleInstanceUnreadable, "prior: %v", err))
			return finish()
		}
		prior, pfs, prefs := decodeInstance(praw, schema)
		if len(pfs) > 0 || len(prefs) > 0 {
			rep.CouldNotCheck = append(rep.CouldNotCheck, instFinding(ruleInstanceUnreadable, "prior %s is not a valid instance", opts.Prior))
			return finish()
		}
		rep.Findings = append(rep.Findings, checkAgainstPrior(prior, inst)...)
	}
	return finish()
}

func sortFindings(fs []instanceFinding) {
	sort.SliceStable(fs, func(i, j int) bool {
		if fs[i].Rule != fs[j].Rule {
			return fs[i].Rule < fs[j].Rule
		}
		return fs[i].Message < fs[j].Message
	})
}

// runInstance is the `statusgen instance` entry point.
func runInstance(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "validate" {
		fmt.Fprintln(stderr, "statusgen instance: want `validate --file PATH [--root DIR] [--prior PATH] [--cell CELL] [--json]`")
		return instanceExitUsageError
	}
	flags := flag.NewFlagSet("instance validate", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var opts instanceValidateOptions
	flags.StringVar(&opts.File, "file", "", "instance document to validate (YAML or JSON)")
	flags.StringVar(&opts.Root, "root", ".", "repository root holding spec/workflow-patterns and docs/streams")
	flags.StringVar(&opts.Prior, "prior", "", "an earlier document: a prior revision of the same instance, or the instance this one supersedes")
	flags.StringVar(&opts.Cell, "cell", "", "the Cell the caller validates for; a different instance Cell is a finding")
	jsonOut := flags.Bool("json", false, "print the report as JSON")
	if err := flags.Parse(args[1:]); err != nil {
		return instanceExitUsageError
	}
	if opts.File == "" {
		fmt.Fprintln(stderr, "statusgen instance validate: --file is required")
		return instanceExitUsageError
	}
	rep := validateInstanceFile(opts)
	if *jsonOut {
		b, err := json.MarshalIndent(rep, "", "  ")
		if err != nil {
			fmt.Fprintln(stderr, "statusgen instance:", err)
			return instanceExitCouldNot
		}
		fmt.Fprintln(stdout, string(b))
	} else {
		for _, f := range rep.Findings {
			fmt.Fprintf(stdout, "checked-failed: %s: %s\n", rep.File, f)
		}
		for _, f := range rep.CouldNotCheck {
			fmt.Fprintf(stdout, "could-not-check: %s: %s\n", rep.File, f)
		}
		fmt.Fprintf(stdout, "instance: %s %s\n", rep.File, rep.State)
	}
	switch rep.State {
	case "checked-failed":
		return instanceExitFailed
	case "could-not-check":
		return instanceExitCouldNot
	default:
		return instanceExitClean
	}
}

// ---- role packet view and handoff (work-input amendment WI-1/WI-2) ----

// instancePacket is a role-specific VIEW of one node's latest attempt in a
// canonical instance. It carries identifiers and authorized references only —
// never credential material, never an authority of its own. Runner, control
// profile, budget and assignment appear as references for the executor that
// holds the real authority.
type instancePacket struct {
	View              string               `json:"view"`
	ProjectionVersion int                  `json:"projection-version"`
	Instance          string               `json:"instance"`
	ContractDigest    string               `json:"contract-digest"`
	Work              string               `json:"work"`
	Cell              string               `json:"cell"`
	Node              string               `json:"node"`
	PatternNode       string               `json:"pattern-node"`
	Attempt           string               `json:"attempt"`
	Role              string               `json:"role"`
	Intent            string               `json:"intent"`
	Acceptance        instanceAcceptance   `json:"acceptance"`
	Policy            instancePolicy       `json:"policy"`
	Inputs            []instanceInput      `json:"inputs"`
	Unresolved        []string             `json:"unresolved"`
	Dependencies      []instanceDependency `json:"dependencies"`
	Coverage          packetCoverage       `json:"coverage"`
	RequiredEvidence  []packetEvidence     `json:"required-evidence"`
	Artifacts         []instanceArtifact   `json:"artifacts"`
	Omissions         []instanceOmission   `json:"omissions"`
	Completed         []string             `json:"completed"`
	Remaining         []string             `json:"remaining"`
	Delta             packetDelta          `json:"delta"`
	NextSafeAction    string               `json:"next-safe-action"`
	References        packetReferences     `json:"references"`
}

type packetCoverage struct {
	Supported []packetRowSupport `json:"supported"`
	// Unknown lists acceptance rows no dependency entry supports. Missing
	// coverage is stated as unknown, never assumed.
	Unknown []string `json:"unknown"`
}

type packetRowSupport struct {
	Row string   `json:"row"`
	By  []string `json:"by"`
}

type packetEvidence struct {
	Claim     string `json:"claim"`
	Mandatory bool   `json:"mandatory"`
	Artifact  string `json:"artifact,omitempty"`
	State     string `json:"state"` // present | omitted | missing
}

type packetDelta struct {
	Since    string         `json:"since"` // the prior attempt id, or "instantiation"
	Changed  []packetChange `json:"changed"`
	Affected []string       `json:"affected"`
}

type packetChange struct {
	Ref  string `json:"ref"`
	Kind string `json:"kind"`
	From string `json:"from"`
	To   string `json:"to"`
}

type packetReferences struct {
	Backend        string `json:"backend"`
	Generation     int    `json:"generation"`
	RunnerProfile  string `json:"runner-profile"`
	ControlProfile string `json:"control-profile"`
	Budget         string `json:"budget"`
}

const packetUnknownFingerprint = "unknown"

func findNode(inst workflowInstance, nodeID string) (instanceNode, bool) {
	for _, n := range inst.Nodes {
		if n.ID == nodeID {
			return n, true
		}
	}
	return instanceNode{}, false
}

// projectPacket projects the role packet for node nodeID's latest attempt,
// given the dependency fingerprints observed now. A dependency with no current
// observation is unknown, which counts as changed (conservative invalidation).
// It refuses an instance that fails the document rules, a node with no attempt,
// an excerpt with no coverage index, and any credential-shaped content. The
// credential check runs first, before any message can quote a field.
func projectPacket(inst workflowInstance, nodeID string, observed map[string]string) (instancePacket, error) {
	if err := credentialFree(inst); err != nil {
		return instancePacket{}, err
	}
	if fs := checkInstanceRules(inst); len(fs) > 0 {
		return instancePacket{}, findingsError("cannot project a packet from an invalid instance", fs)
	}
	n, ok := findNode(inst, nodeID)
	if !ok {
		return instancePacket{}, fmt.Errorf("instance %s has no node %s", inst.Instance.ID, nodeID)
	}
	if len(n.Attempts) == 0 {
		return instancePacket{}, fmt.Errorf("node %s has no attempt to project a packet for", nodeID)
	}
	cur := n.Attempts[len(n.Attempts)-1]

	pkt := instancePacket{
		View:              "role-packet",
		ProjectionVersion: 1,
		Instance:          inst.Instance.ID,
		ContractDigest:    contractDigest(inst),
		Work:              inst.Work.ID,
		Cell:              inst.Work.Cell,
		Node:              n.ID,
		PatternNode:       n.Node,
		Attempt:           cur.ID,
		Role:              n.Role,
		Intent:            inst.Work.Intent,
		Acceptance:        canonicalInstance(inst).Acceptance,
		Policy:            inst.Policy,
		Inputs:            append([]instanceInput{}, inst.Inputs...),
		Unresolved:        unresolvedInputs(inst),
		Dependencies:      []instanceDependency{},
		RequiredEvidence:  []packetEvidence{},
		Artifacts:         []instanceArtifact{},
		Omissions:         append([]instanceOmission{}, cur.Omissions...),
		Completed:         []string{},
		Remaining:         []string{},
		References: packetReferences{
			Backend:        inst.Assignment.Backend,
			Generation:     inst.Assignment.Generation,
			RunnerProfile:  inst.Assignment.RunnerProfile,
			ControlProfile: inst.Policy.ControlProfile,
			Budget:         inst.Budget.Ref,
		},
	}

	// Dependencies as observed now, and the delta against the prior attempt's
	// observations (or the manifest pinned at instantiation).
	baseline := map[string]string{}
	pkt.Delta.Since = "instantiation"
	for _, d := range inst.Dependencies {
		baseline[d.Ref] = d.Fingerprint
	}
	if len(n.Attempts) > 1 {
		prev := n.Attempts[len(n.Attempts)-2]
		pkt.Delta.Since = prev.ID
		for _, o := range prev.Observed {
			baseline[o.Ref] = o.Fingerprint
		}
	}
	affected := map[string]bool{}
	pkt.Delta.Changed = []packetChange{}
	for _, d := range inst.Dependencies {
		now, ok := observed[d.Ref]
		if !ok {
			now = packetUnknownFingerprint
		}
		dd := d
		dd.Fingerprint = now
		dd.Supports = append([]string{}, d.Supports...)
		pkt.Dependencies = append(pkt.Dependencies, dd)
		if now != baseline[d.Ref] {
			pkt.Delta.Changed = append(pkt.Delta.Changed, packetChange{Ref: d.Ref, Kind: d.Kind, From: baseline[d.Ref], To: now})
			for _, r := range d.Supports {
				affected[r] = true
			}
		}
	}

	// Coverage, completed and remaining, in acceptance-row order. An affected
	// row is never reported completed: it must be revalidated.
	completed := map[string]bool{}
	for _, r := range cur.Completed {
		completed[r] = true
	}
	pkt.Delta.Affected = []string{}
	for _, row := range inst.Acceptance.Rows {
		var by []string
		for _, d := range inst.Dependencies {
			for _, s := range d.Supports {
				if s == row {
					by = append(by, d.Ref)
				}
			}
		}
		if len(by) == 0 {
			pkt.Coverage.Unknown = append(pkt.Coverage.Unknown, row)
		} else {
			pkt.Coverage.Supported = append(pkt.Coverage.Supported, packetRowSupport{Row: row, By: by})
		}
		if affected[row] {
			pkt.Delta.Affected = append(pkt.Delta.Affected, row)
		}
		if completed[row] && !affected[row] {
			pkt.Completed = append(pkt.Completed, row)
		} else {
			pkt.Remaining = append(pkt.Remaining, row)
		}
	}
	if pkt.Coverage.Unknown == nil {
		pkt.Coverage.Unknown = []string{}
	}
	if pkt.Coverage.Supported == nil {
		pkt.Coverage.Supported = []packetRowSupport{}
	}

	// Required evidence from the node's claims, with what the attempt holds. A
	// claim is present only when it cites an artifact the attempt itself holds
	// — the same test acceptHandoff applies to a returned handoff.
	held := map[string]bool{}
	for _, a := range cur.Artifacts {
		held[a.Ref] = true
	}
	for _, c := range n.Evidence {
		pe := packetEvidence{Claim: c.Claim, Mandatory: c.Mandatory, State: "missing"}
		for _, e := range cur.Evidence {
			if e.Claim == c.Claim && e.Artifact != "" && held[e.Artifact] {
				pe.State, pe.Artifact = "present", e.Artifact
			}
		}
		if pe.State == "missing" {
			for _, o := range cur.Omissions {
				if o.Ref == c.Claim {
					pe.State = "omitted"
				}
			}
		}
		pkt.RequiredEvidence = append(pkt.RequiredEvidence, pe)
	}

	// Prior artifacts of this node, every attempt, by reference and digest.
	for _, a := range n.Attempts {
		for _, art := range a.Artifacts {
			if art.IncludedBytes != nil && *art.IncludedBytes < art.Bytes && art.Index == "" {
				return instancePacket{}, fmt.Errorf("[%s] artifact %s is excerpted with no coverage index", ruleInstanceTruncation, art.Ref)
			}
			pkt.Artifacts = append(pkt.Artifacts, art)
		}
	}

	pkt.NextSafeAction = cur.NextSafeAction
	if len(pkt.Delta.Affected) > 0 {
		pkt.NextSafeAction = "revalidate acceptance rows " + strings.Join(pkt.Delta.Affected, ", ") +
			" against the changed dependencies before any further effect"
	}
	if err := credentialFree(pkt); err != nil {
		return instancePacket{}, err
	}
	return pkt, nil
}

// credentialFree runs the credential scan over any serialisable value.
func credentialFree(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	var generic any
	if err := json.Unmarshal(b, &generic); err != nil {
		return err
	}
	if fs := scanCredentialMaterial(generic, ""); len(fs) > 0 {
		return findingsError("refused", fs)
	}
	return nil
}

// unresolvedInputs lists every input not decided `trusted` by the trust gate —
// source data, never an instruction. A reporter's label is named as a label.
func unresolvedInputs(inst workflowInstance) []string {
	out := []string{}
	for _, in := range inst.Inputs {
		if in.Trust.Disposition == "trusted" && in.Trust.Source == "trust-gate" {
			continue
		}
		u := fmt.Sprintf("input %s: disposition %s (source %s)", in.Name, in.Trust.Disposition, in.Trust.Source)
		if in.Trust.ReportedLabel != "" {
			u += fmt.Sprintf("; reporter label %q is not a disposition", in.Trust.ReportedLabel)
		}
		out = append(out, u)
	}
	return out
}

// staleDependencies checks a packet's or handoff's dependency list against the
// instance's own manifest, never against the list alone: every manifest
// dependency must be bound exactly once, at the fingerprint observed now. A
// dependency trimmed from the list, or the whole list emptied, is named rather
// than skipped, and a missing observation is stale. A bound entry the manifest
// does not hold is named too.
func staleDependencies(inst workflowInstance, bound []instanceDependency, observed map[string]string) []string {
	byRef := map[string][]instanceDependency{}
	for _, d := range bound {
		byRef[d.Ref] = append(byRef[d.Ref], d)
	}
	var stale []string
	for _, m := range inst.Dependencies {
		b := byRef[m.Ref]
		delete(byRef, m.Ref)
		if len(b) != 1 {
			stale = append(stale, m.Ref+" (not bound)")
			continue
		}
		now, ok := observed[m.Ref]
		if !ok || now != b[0].Fingerprint {
			stale = append(stale, m.Ref)
		}
	}
	extra := make([]string, 0, len(byRef))
	for ref := range byRef {
		extra = append(extra, ref+" (not in the manifest)")
	}
	sort.Strings(extra)
	return append(stale, extra...)
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func instanceHolds[T comparable](have, want []T) bool {
	set := map[T]bool{}
	for _, h := range have {
		set[h] = true
	}
	for _, w := range want {
		if !set[w] {
			return false
		}
	}
	return true
}

// packetIsCurrent says whether a previously projected packet still describes the
// instance as it is now: same instance contract, same attempt, same acceptance,
// and no dependency moved. A packet that is not current must be re-projected;
// it can never stand in for the current one.
func packetIsCurrent(pkt instancePacket, inst workflowInstance, observed map[string]string) error {
	if err := credentialFree(pkt); err != nil {
		return err
	}
	if pkt.Instance != inst.Instance.ID || pkt.ContractDigest != contractDigest(inst) {
		return fmt.Errorf("packet is bound to instance %s at %s; the instance is %s at %s", pkt.Instance, pkt.ContractDigest, inst.Instance.ID, contractDigest(inst))
	}
	n, ok := findNode(inst, pkt.Node)
	if !ok || len(n.Attempts) == 0 || n.Attempts[len(n.Attempts)-1].ID != pkt.Attempt {
		return fmt.Errorf("packet attempt %s is not the latest attempt of %s", pkt.Attempt, pkt.Node)
	}
	if pkt.Acceptance.Version != inst.Acceptance.Version {
		return fmt.Errorf("packet acceptance version %d, instance %d", pkt.Acceptance.Version, inst.Acceptance.Version)
	}
	if stale := staleDependencies(inst, pkt.Dependencies, observed); len(stale) > 0 {
		return fmt.Errorf("dependencies moved since the packet was projected: %s", strings.Join(stale, ", "))
	}
	return nil
}

// handoffResult is what a role returns for its attempt; buildHandoff binds it to
// the packet's identifiers mechanically.
type handoffResult struct {
	Scope          []string
	Evidence       []instanceEvidence
	Artifacts      []instanceArtifact
	Omissions      []instanceOmission
	Unresolved     []string
	NextSafeAction string
}

// instanceHandoff is the returned result, bound to the same instance, node and
// attempt identifiers and the same acceptance revision and dependency
// fingerprints as the packet it answers.
type instanceHandoff struct {
	Instance          string               `json:"instance"`
	ContractDigest    string               `json:"contract-digest"`
	Work              string               `json:"work"`
	Cell              string               `json:"cell"`
	Node              string               `json:"node"`
	Attempt           string               `json:"attempt"`
	AcceptanceVersion int                  `json:"acceptance-version"`
	AcceptanceRef     string               `json:"acceptance-ref"`
	Scope             []string             `json:"scope"`
	Exclusions        []string             `json:"exclusions"`
	Evidence          []instanceEvidence   `json:"evidence"`
	Artifacts         []instanceArtifact   `json:"artifacts"`
	Omissions         []instanceOmission   `json:"omissions"`
	Dependencies      []instanceDependency `json:"dependencies"`
	Unresolved        []string             `json:"unresolved"`
	NextSafeAction    string               `json:"next-safe-action"`
}

// buildHandoff assembles the handoff's mechanical fields from the packet; only
// the result's own content comes from the role.
func buildHandoff(pkt instancePacket, res handoffResult) (instanceHandoff, error) {
	if err := credentialFree(pkt); err != nil {
		return instanceHandoff{}, err
	}
	if err := credentialFree(res); err != nil {
		return instanceHandoff{}, err
	}
	rows := map[string]bool{}
	for _, r := range pkt.Acceptance.Rows {
		rows[r] = true
	}
	for _, r := range res.Scope {
		if !rows[r] {
			return instanceHandoff{}, fmt.Errorf("result scope row %q is outside the acceptance scope", r)
		}
	}
	for _, art := range res.Artifacts {
		if art.IncludedBytes != nil && *art.IncludedBytes < art.Bytes && art.Index == "" {
			return instanceHandoff{}, fmt.Errorf("[%s] artifact %s is excerpted with no coverage index", ruleInstanceTruncation, art.Ref)
		}
	}
	h := instanceHandoff{
		Instance:          pkt.Instance,
		ContractDigest:    pkt.ContractDigest,
		Work:              pkt.Work,
		Cell:              pkt.Cell,
		Node:              pkt.Node,
		Attempt:           pkt.Attempt,
		AcceptanceVersion: pkt.Acceptance.Version,
		AcceptanceRef:     pkt.Acceptance.Ref,
		Scope:             append([]string{}, res.Scope...),
		Exclusions:        append([]string{}, pkt.Acceptance.Exclusions...),
		Evidence:          append([]instanceEvidence{}, res.Evidence...),
		Artifacts:         append([]instanceArtifact{}, res.Artifacts...),
		Omissions:         append(append([]instanceOmission{}, pkt.Omissions...), res.Omissions...),
		Dependencies:      append([]instanceDependency{}, pkt.Dependencies...),
		Unresolved:        append(append([]string{}, pkt.Unresolved...), res.Unresolved...),
		NextSafeAction:    res.NextSafeAction,
	}
	if err := credentialFree(h); err != nil {
		return instanceHandoff{}, err
	}
	return h, nil
}

// acceptHandoff is the acceptance boundary: it re-checks the binding at the
// moment of acceptance rather than trusting the packet. The handoff must answer
// this instance's contract, the node's latest attempt and the current acceptance
// version; its scope must lie inside the acceptance rows and it must carry the
// acceptance exclusions, the attempt's omissions and the unresolved inputs
// forward; no manifest dependency may have moved since the packet; and every
// mandatory claim of the node must carry evidence backed by a returned
// artifact. None of this trusts buildHandoff to have produced the handoff.
//
// The dependency fingerprints a handoff carries are its own statement of what
// it was bound to; they are not tied to a packet digest. A handoff rewritten to
// the current observations passes this check — binding a handoff to the packet
// it answers is the authenticated transport's job, not this boundary's.
func acceptHandoff(inst workflowInstance, h instanceHandoff, observed map[string]string) error {
	if err := credentialFree(h); err != nil {
		return err
	}
	if h.Instance != inst.Instance.ID || h.ContractDigest != contractDigest(inst) || h.Cell != inst.Work.Cell || h.Work != inst.Work.ID {
		return fmt.Errorf("handoff is bound to %s/%s in Cell %q; the instance is %s/%s in Cell %q",
			h.Instance, h.ContractDigest, h.Cell, inst.Instance.ID, contractDigest(inst), inst.Work.Cell)
	}
	n, ok := findNode(inst, h.Node)
	if !ok || len(n.Attempts) == 0 || n.Attempts[len(n.Attempts)-1].ID != h.Attempt {
		return fmt.Errorf("handoff attempt %s is not the latest attempt of node %s", h.Attempt, h.Node)
	}
	if h.AcceptanceVersion != inst.Acceptance.Version || h.AcceptanceRef != inst.Acceptance.Ref {
		return fmt.Errorf("handoff answers acceptance v%d (%s); the instance is at v%d (%s)", h.AcceptanceVersion, h.AcceptanceRef, inst.Acceptance.Version, inst.Acceptance.Ref)
	}
	if !instanceHolds(inst.Acceptance.Rows, h.Scope) {
		return fmt.Errorf("handoff scope %v is not inside the acceptance rows %v", h.Scope, inst.Acceptance.Rows)
	}
	if !sameStrings(h.Exclusions, inst.Acceptance.Exclusions) {
		return fmt.Errorf("handoff exclusions %v differ from the acceptance exclusions %v", h.Exclusions, inst.Acceptance.Exclusions)
	}
	if cur := n.Attempts[len(n.Attempts)-1]; !instanceHolds(h.Omissions, cur.Omissions) {
		return fmt.Errorf("handoff drops an omission the attempt records — omissions travel to the handoff")
	}
	if !instanceHolds(h.Unresolved, unresolvedInputs(inst)) {
		return fmt.Errorf("handoff drops an unresolved input — an input the trust gate has not decided stays unresolved")
	}
	if stale := staleDependencies(inst, h.Dependencies, observed); len(stale) > 0 {
		return fmt.Errorf("dependencies moved since the packet was projected: %s — re-project and revalidate the affected rows", strings.Join(stale, ", "))
	}
	arts := map[string]bool{}
	for _, a := range h.Artifacts {
		arts[a.Ref] = true
	}
	for _, c := range n.Evidence {
		if !c.Mandatory {
			continue
		}
		found := false
		for _, e := range h.Evidence {
			if e.Claim == c.Claim && e.Artifact != "" && arts[e.Artifact] {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("mandatory evidence %q is omitted from the handoff", c.Claim)
		}
	}
	return nil
}

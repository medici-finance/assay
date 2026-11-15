package main

// newbriefv4.go — negotiated brief-v4 authoring for `statusgen newbrief`
// (milestone-support/04).
//
// newbrief keeps writing the stream's detected legacy schema by default. The
// prefixed brief-v4 identifier/schema pair is selected only on an explicit
// `--schema brief-v4`, and only after the shared readercaps contract assesses
// statusgen's own advertisement AND every supplied required reader's
// advertisement as Supported for the exact requirement the written record will
// carry. Any Unsupported or could-not-check reader refuses before a single file
// is written; there is no silent fallback to a legacy schema that would drop a
// requested `--requires-milestone` gate. `--schema legacy` is the explicitly
// selected supported legacy mode.

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/medici-finance/assay/statusgen/readercaps"
	"github.com/medici-finance/assay/statusgen/workref"
)

const (
	newBriefSchemaAuto   = ""
	newBriefSchemaLegacy = "legacy"
)

// newBriefUUID mints the brief-v4 `id:`. A package-level seam so tests pin it;
// it is called once per written brief, never for an alias or selector.
var newBriefUUID = newUUIDv4

// repeatedFlag is a repeatable string flag.
type repeatedFlag struct {
	name   string
	values []string
}

func (r *repeatedFlag) String() string { return strings.Join(r.values, ", ") }
func (r *repeatedFlag) Set(v string) error {
	v = strings.TrimSpace(v)
	if v == "" {
		return fmt.Errorf("empty --%s value", r.name)
	}
	r.values = append(r.values, v)
	return nil
}

// newBriefV4Requirements is the exact requirement a brief-v4 record carries in
// its reader-requirements envelope, and therefore exactly what every required
// reader must be assessed Supported for before it is written.
func newBriefV4Requirements() readercaps.Requirements {
	return readercaps.Requirements{Schema: readercaps.RequirementsSchema, Required: []readercaps.Operation{
		{Operation: "syntax", Schemas: []string{briefSchemaV4}, Semantics: []string{}},
	}}
}

// decodeReaderAdvertisement accepts a bare reader-capabilities-v1 document or a
// tool's `--version --json` document carrying it under readerCapabilities.
func decodeReaderAdvertisement(raw []byte) (readercaps.Advertisement, error) {
	adv, err := readercaps.DecodeAdvertisement(raw)
	if err == nil {
		return adv, nil
	}
	var wrapped struct {
		ReaderCapabilities json.RawMessage `json:"readerCapabilities"`
	}
	if jerr := json.Unmarshal(raw, &wrapped); jerr != nil || len(wrapped.ReaderCapabilities) == 0 {
		return readercaps.Advertisement{}, err
	}
	return readercaps.DecodeAdvertisement(wrapped.ReaderCapabilities)
}

// newBriefNegotiateV4 assesses statusgen and every supplied reader against the
// brief-v4 requirement. It performs no write. A reader that cannot be read or
// decoded is could-not-check; zero supplied readers is could-not-check too —
// statusgen alone never establishes that the other required consumers read it.
func newBriefNegotiateV4(capFiles []string) readercaps.Result {
	need := newBriefV4Requirements()
	if r := readercaps.Assess(need, readercaps.Observation{State: readercaps.Current, Advertisement: statusgenReaderCaps()}); r.State != readercaps.Supported {
		return readercaps.Result{State: r.State, Reason: "statusgen: " + r.Reason}
	}
	if len(capFiles) == 0 {
		return readercaps.Result{State: readercaps.CouldNotCheck, Reason: "no required reader advertisement supplied (--reader-caps FILE, one per required consumer) — statusgen alone does not establish that the other readers support brief-v4"}
	}
	for _, f := range capFiles {
		raw, err := os.ReadFile(f)
		if err != nil {
			return readercaps.Result{State: readercaps.CouldNotCheck, Reason: f + ": unreadable advertisement: " + err.Error()}
		}
		adv, err := decodeReaderAdvertisement(raw)
		if err != nil {
			return readercaps.Result{State: readercaps.CouldNotCheck, Reason: f + ": malformed advertisement: " + err.Error()}
		}
		if r := readercaps.Assess(need, readercaps.Observation{State: readercaps.Current, Advertisement: adv}); r.State != readercaps.Supported {
			return readercaps.Result{State: r.State, Reason: f + " (" + adv.Provider + "): " + r.Reason}
		}
	}
	return readercaps.Result{State: readercaps.Supported}
}

// newBriefV4Plan carries the negotiated brief-v4 rendering inputs.
type newBriefV4Plan struct {
	typedID            string   // <cell>:<repo>:<stream>:b-<NN>
	uuid               string   // minted once for this new brief
	depends            []string // rendered <stream>:b-<NN>
	requiresMilestones []string
}

// newBriefSelectSchema resolves the schema to write. detected is the stream's
// own signal (newBriefStreamSchema). It returns the schema, or a refusal
// message; it writes nothing.
func newBriefSelectSchema(detected, selected string, capFiles, milestones []string) (string, string) {
	switch selected {
	case briefSchemaV4:
		r := newBriefNegotiateV4(capFiles)
		if r.State != readercaps.Supported {
			return "", fmt.Sprintf("--schema brief-v4 refused (%s): %s — nothing was written; select --schema legacy to write the stream's supported legacy schema instead", r.State, r.Reason)
		}
		return briefSchemaV4, ""
	case newBriefSchemaAuto, newBriefSchemaLegacy:
		if len(milestones) > 0 {
			return "", "--requires-milestone needs --schema brief-v4 — a legacy schema cannot carry a milestone start requirement, and newbrief never silently drops a requested gate"
		}
		if len(capFiles) > 0 {
			return "", "--reader-caps is only meaningful with --schema brief-v4"
		}
		if detected == briefSchemaV4 {
			if selected == newBriefSchemaAuto {
				return "", "this stream already carries brief-v4 briefs — select --schema brief-v4 (negotiated against every required reader) or --schema legacy (the supported brief-v2 form) explicitly"
			}
			return briefSchemaV2, ""
		}
		return detected, ""
	default:
		return "", fmt.Sprintf("invalid --schema %q (want legacy or brief-v4; empty keeps the stream's detected legacy schema)", selected)
	}
}

// newBriefV4Deps normalizes --depends values for a brief-v4 write: each must be
// a same-root brief reference (legacy or prefixed, no revision selector — a new
// brief binds its dependency at the selected snapshot). It returns the
// <stream>/<NN> keys the wave derivation reads and the canonical rendered forms.
func newBriefV4Deps(deps []string) (keys, rendered []string, msg string) {
	seen := map[string]bool{}
	for _, d := range deps {
		ref, err := workref.Parse(d, workref.Typed)
		if err != nil {
			return nil, nil, fmt.Sprintf("dependency %q is not a brief reference: %v", d, err)
		}
		if ref.Kind != workref.Brief {
			return nil, nil, fmt.Sprintf("dependency %q names a milestone — use --requires-milestone for a milestone start requirement", d)
		}
		if ref.Cell != "" || ref.Repo != "" {
			return nil, nil, fmt.Sprintf("dependency %q is qualified — --depends names a brief in this root as <stream>:b-<NN> (or <stream>/<NN>)", d)
		}
		if ref.Revision != 0 {
			return nil, nil, fmt.Sprintf("dependency %q pins a revision — newbrief binds the dependency at the selected snapshot; pin by editing the brief", d)
		}
		key := ref.Stream + "/" + ref.Number
		if seen[key] {
			return nil, nil, fmt.Sprintf("dependency %q names brief %s twice", d, key)
		}
		seen[key] = true
		keys = append(keys, key)
		rendered = append(rendered, ref.Stream+":b-"+ref.Number)
	}
	return keys, rendered, ""
}

// newBriefV4Milestones validates each --requires-milestone against the shared
// resolver and this root's registry: colon-only, exact version, known root.
func newBriefV4Milestones(root string, ms []string, cell, alias string) ([]string, string) {
	if len(ms) == 0 {
		return nil, ""
	}
	reg, ok, err := loadGraphRepos(root)
	if err != nil || !ok || reg == nil {
		return nil, "docs/streams/graph-repos.yaml is required to resolve --requires-milestone"
	}
	wr, err := briefV4Registry(reg)
	if err != nil {
		return nil, fmt.Sprintf("docs/streams/graph-repos.yaml cannot serve as a typed-reference registry: %v", err)
	}
	seen := map[string]bool{}
	var out []string
	for _, m := range ms {
		if strings.Contains(m, "/") {
			return nil, fmt.Sprintf("--requires-milestone %q must be colon-only (<stream>:m-<NN>@<version>)", m)
		}
		res, err := workref.Resolve(m, &workref.Context{Cell: cell, Repo: alias}, wr, workref.TypedMilestone)
		if err != nil {
			return nil, fmt.Sprintf("--requires-milestone %q: %v", m, err)
		}
		if seen[res.LogicalKey] {
			return nil, fmt.Sprintf("--requires-milestone names milestone %s more than once", res.LogicalKey)
		}
		seen[res.LogicalKey] = true
		out = append(out, m)
	}
	return out, ""
}

// newBriefV4Inverse is the unblocks: entry written into a dependency: the
// prefixed same-root form into a brief-v4 dependency, the legacy <stream>/<NN>
// form every legacy reader accepts into any other.
func newBriefV4Inverse(depSchema, stream, num string) string {
	if depSchema == briefSchemaV4 {
		return stream + ":b-" + num
	}
	return stream + "/" + num
}

// renderBriefV4Keys writes the brief-v4-only frontmatter keys after `version:`.
func renderBriefV4Keys(b io.StringWriter, p *newBriefV4Plan) {
	_, _ = b.WriteString("id: " + p.uuid + "\n")
	if len(p.requiresMilestones) > 0 {
		items := make([]string, 0, len(p.requiresMilestones))
		for _, m := range p.requiresMilestones {
			items = append(items, fmt.Sprintf("%q", m))
		}
		_, _ = b.WriteString("requires-milestones: [" + strings.Join(items, ", ") + "]\n")
	}
	need := newBriefV4Requirements()
	_, _ = b.WriteString("reader-requirements:\n")
	_, _ = b.WriteString("  schema: " + need.Schema + "\n")
	_, _ = b.WriteString("  required:\n")
	for _, op := range need.Required {
		_, _ = b.WriteString("    - operation: " + op.Operation + "\n")
		_, _ = b.WriteString("      schemas: [" + strings.Join(op.Schemas, ", ") + "]\n")
		_, _ = b.WriteString("      semantics: [" + strings.Join(op.Semantics, ", ") + "]\n")
	}
}

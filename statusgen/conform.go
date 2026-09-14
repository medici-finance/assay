package main

// conform — validate brief frontmatter against the machine-readable brief-schema
// contracts (`schemas/brief-v1.json`, `schemas/brief-v2.json`), a positional
// subcommand intercepted before flag parsing (like verifyrun/shardcheck/init).
//
// # Why a schema surface at all
//
// The brief-v1 contract has lived in two places that can silently disagree: the
// reference validator in brieffile.go (the code that runs in `--lint`) and the
// prose the field tables describe. The pinned binary proves WHICH validator runs;
// nothing proved the validator and a consumer still agreed on the CONTRACT, and a
// consumer whose pinned validator quietly stopped checking a field would never
// notice. `conform` emits that contract as a versioned, machine-readable artifact
// and validates against it, so drift becomes a red check in the consumer's own CI
// the day it happens rather than a discovery made later.
//
// # conform is NOT --lint
//
//	--lint     methodology rules — the full reference validator, including the
//	           cross-file and cross-field rules a schema cannot express (a brief's
//	           id matching its filename, its wave matching the stream README row,
//	           dependency-reference resolution, the verifier floor, the
//	           risk-answer/gate coupling).
//	conform    the schema contract — the per-file, frontmatter-shape rules only:
//	           required keys, field types, and closed value sets. Plus schema
//	           VERSION reporting: a file whose `schema:` marker is a brief-schema
//	           NONE of this binary's embedded schemas describe (a future brief-v3,
//	           …) is reported as a VERSION mismatch, not a field error — the
//	           deliberate-migration signal on a pin bump, kept distinct from a
//	           malformed field so the two never blur.
//
// # One contract per schema version, selected by the file's own marker
//
// A tree mid-migration holds brief-v1 and brief-v2 files side by side, so the
// contract is chosen PER FILE from its `schema:` marker rather than per run:
// every embedded contract validates the files that declare it, and a marker no
// embedded contract describes is the version mismatch above. Before this,
// brief-v1 was the only embedded contract, so a MIGRATED tree — every file on
// brief-v2, exactly what `statusgen migrate brief-v1-to-v2` produces — reported
// could-not-check for every brief and exited 2, reddening the schema-contract
// check on the very PR that lands the flag day. The gap was in the contract, not
// in the gate; widening the gate to swallow the exit 2 would have turned an
// honest could-not-check into a silent pass on every brief in the tree.
//
// Each schema and brieffile.go are held in lockstep by TestBriefV1SchemaCoverage
// and TestBriefV2SchemaCoverage, which derive the required-key and value sets from
// brieffile.go's own tables and assert the committed schemas encode exactly those.
// Neither can drift from the other without the assay repo's own CI failing.
//
// # Three-state, and which way it fails
//
// checked-clean (0) / checked-failed (1) / could-not-check (2), the same contract
// and the same numbers verifyrun and shardcheck use in this binary. A brief that
// violates the schema is checked-failed and names the file and field; an
// unreadable file, unparseable frontmatter, a missing streams tree, or a
// schema-version mismatch is could-not-check and fails CLOSED (never rounded up to
// clean). exit 1 dominates exit 2: any checked-failed exits 1, else any
// could-not-check exits 2, else 0.

import (
	"embed"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// embeddedSchemaFS holds the committed brief-schema contracts, compiled into the
// binary so `conform --emit-schema` reproduces an artifact from any pinned build
// and `conform` validates without reading a file from the tree. The embed
// directive cannot cross the module boundary (go.mod is rooted at statusgen/), so
// the canonical repo-root schemas/*.json are mirrored here byte-for-byte;
// TestEmbeddedSchemaMatchesCommitted pins each pair identical.
//
//go:embed schemas/brief-v1.json schemas/brief-v2.json
var embeddedSchemaFS embed.FS

// embeddedBriefV1SchemaName / embeddedBriefV2SchemaName are the embedded paths of
// the two brief-schema contracts this build carries.
const (
	embeddedBriefV1SchemaName = "schemas/brief-v1.json"
	embeddedBriefV2SchemaName = "schemas/brief-v2.json"
)

// embeddedBriefSchemaNames maps each brief-schema MARKER to the embedded artifact
// that describes it. It is the one place the set of contracts this binary knows is
// declared: conform dispatches on it, `--emit-schema --schema <marker>` selects
// from it, and the version-mismatch message enumerates it — so adding a future
// brief-v3 contract is one entry plus its artifact, with no message or dispatch
// arm to remember. Iterate it via embeddedBriefSchemaMarkers for stable order.
var embeddedBriefSchemaNames = map[string]string{
	briefSchemaCurrent: embeddedBriefV1SchemaName, // brief-v1
	briefSchemaV2:      embeddedBriefV2SchemaName, // brief-v2
}

// embeddedBriefSchemaMarkers returns the known markers in sorted order (brief-v1,
// brief-v2, …) so messages and emitted listings are deterministic.
func embeddedBriefSchemaMarkers() []string {
	out := make([]string, 0, len(embeddedBriefSchemaNames))
	for marker := range embeddedBriefSchemaNames {
		out = append(out, marker)
	}
	sort.Strings(out)
	return out
}

// conform exit codes — the same three-state contract, and the same numbers, as
// verifyrun and shardcheck in this same binary.
const (
	conformExitClean      = 0 // every brief-v1 file validates
	conformExitFailed     = 1 // at least one brief-v1 file violates the schema
	conformExitCouldNot   = 2 // at least one file could not be checked (fail-closed)
	conformExitUsageError = 2 // usage/refusal shares the could-not-check code
)

// embeddedSchemaBytes returns the raw bytes of one embedded schema artifact.
func embeddedSchemaBytes(name string) []byte {
	b, err := embeddedSchemaFS.ReadFile(name)
	if err != nil {
		// A build that compiled has the file embedded; a read failure here is a
		// programming error, not a runtime condition.
		panic(fmt.Sprintf("statusgen: embedded %s unreadable: %v", name, err))
	}
	return b
}

// embeddedBriefV1Schema returns the raw bytes of the embedded brief-v1 schema.
func embeddedBriefV1Schema() []byte { return embeddedSchemaBytes(embeddedBriefV1SchemaName) }

// embeddedBriefV2Schema returns the raw bytes of the embedded brief-v2 schema.
func embeddedBriefV2Schema() []byte { return embeddedSchemaBytes(embeddedBriefV2SchemaName) }

// parseEmbeddedBriefSchemas parses every embedded contract into a marker-keyed
// map, the form conformFile dispatches on. A schema that does not parse is a
// build-level fault and is returned as an error so the caller can report
// could-not-check rather than validate against a half-built contract set.
func parseEmbeddedBriefSchemas() (map[string]*schemaNode, error) {
	out := make(map[string]*schemaNode, len(embeddedBriefSchemaNames))
	for _, marker := range embeddedBriefSchemaMarkers() {
		name := embeddedBriefSchemaNames[marker]
		node, err := parseSchema(embeddedSchemaBytes(name))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		out[marker] = node
	}
	return out, nil
}

// runConform is the `statusgen conform` entry point. It returns the process exit
// code. --emit-schema prints one embedded schema (brief-v1 unless --schema names
// another) and returns 0; otherwise every brief file under each --root is
// validated against the contract its own `schema:` marker declares.
func runConform(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("conform", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var roots rootFlags
	flags.Var(&roots, "root", "repository root to scan (repeatable; default \".\")")
	emitSchema := flags.Bool("emit-schema", false, "print one embedded brief schema to stdout and exit")
	emitWhich := flags.String("schema", briefSchemaCurrent,
		"with --emit-schema: which embedded contract to print ("+strings.Join(embeddedBriefSchemaMarkers(), ", ")+")")
	if err := flags.Parse(args); err != nil {
		return conformExitUsageError
	}

	if *emitSchema {
		name, known := embeddedBriefSchemaNames[*emitWhich]
		if !known {
			fmt.Fprintf(stderr, "conform: --schema %q is not a contract this binary embeds (have: %s)\n",
				*emitWhich, strings.Join(embeddedBriefSchemaMarkers(), ", "))
			return conformExitUsageError
		}
		if _, err := stdout.Write(embeddedSchemaBytes(name)); err != nil {
			fmt.Fprintf(stderr, "conform: writing schema: %v\n", err)
			return conformExitCouldNot
		}
		return conformExitClean
	}

	resolvedRoots, err := resolveRoots(roots)
	if err != nil {
		fmt.Fprintln(stderr, "conform:", err)
		return conformExitUsageError
	}

	schemas, err := parseEmbeddedBriefSchemas()
	if err != nil {
		fmt.Fprintf(stderr, "conform: embedded schema is not valid JSON: %v\n", err)
		return conformExitCouldNot
	}

	var clean, failed, couldNot, versionMismatch, exempt, scanned int
	for _, root := range resolvedRoots {
		streamsDir := filepath.Join(root, "docs", "streams")
		info, statErr := os.Stat(streamsDir)
		if statErr != nil || !info.IsDir() {
			// A root with no streams tree is could-not-look, not a clean pass — the
			// three-state instrument rule forbids reading "nothing scanned" as
			// "nothing wrong".
			couldNot++
			fmt.Fprintf(stdout, "could-not-check: %s: no docs/streams directory to scan (%v)\n", root, statErr)
			continue
		}
		walkErr := filepath.WalkDir(streamsDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				couldNot++
				fmt.Fprintf(stdout, "could-not-check: %s: %v\n", path, err)
				return nil
			}
			if d.IsDir() || !briefNameRe.MatchString(d.Name()) {
				return nil
			}
			scanned++
			state, msg := conformFile(path, schemas)
			switch state {
			case conformStateClean:
				clean++
			case conformStateFailed:
				failed++
				fmt.Fprintf(stdout, "checked-failed: %s\n", msg)
			case conformStateVersion:
				versionMismatch++
				couldNot++
				fmt.Fprintf(stdout, "could-not-check: %s\n", msg)
			case conformStateCouldNot:
				couldNot++
				fmt.Fprintf(stdout, "could-not-check: %s\n", msg)
			case conformStateExempt:
				exempt++
			}
			return nil
		})
		if walkErr != nil {
			couldNot++
			fmt.Fprintf(stdout, "could-not-check: %s: walk error: %v\n", streamsDir, walkErr)
		}
	}

	fmt.Fprintf(stdout,
		"conform: %d checked-clean, %d checked-failed, %d could-not-check (of which %d schema-version mismatch), %d exempt (%d brief files scanned)\n",
		clean, failed, couldNot, versionMismatch, exempt, scanned)

	switch {
	case failed > 0:
		return conformExitFailed
	case couldNot > 0:
		return conformExitCouldNot
	default:
		return conformExitClean
	}
}

type conformState int

const (
	conformStateClean conformState = iota
	conformStateFailed
	conformStateCouldNot
	conformStateVersion
	conformStateExempt
)

// conformFile classifies one brief-*.md file against the embedded contract its
// own `schema:` marker declares. It reproduces parseBriefFile's opt-in gate
// (frontmatter present + a recognized `schema:` marker) but validates
// independently against the schema rather than re-running the reference
// validator, so the schema surface does the work.
func conformFile(path string, schemas map[string]*schemaNode) (conformState, string) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return conformStateCouldNot, fmt.Sprintf("%s: %v", path, err)
	}
	content := strings.ReplaceAll(string(raw), "\r\n", "\n")

	// A brief opts in only via YAML frontmatter. No leading `---` → legacy, exempt.
	first, _, _ := strings.Cut(content, "\n")
	if strings.TrimSpace(first) != "---" {
		return conformStateExempt, ""
	}
	fmRaw, _, splitErr := splitFrontmatter(content)
	if splitErr != nil {
		return conformStateCouldNot, fmt.Sprintf("%s: %v", path, splitErr)
	}
	var data map[string]any
	if err := yaml.Unmarshal([]byte(fmRaw), &data); err != nil {
		return conformStateCouldNot, fmt.Sprintf("%s: frontmatter: %v", path, err)
	}

	schemaVal, hasSchema := data["schema"]
	if !hasSchema {
		return conformStateExempt, "" // legacy (no schema marker) → exempt
	}
	s, isStr := schemaVal.(string)
	if !isStr {
		return conformStateExempt, "" // non-string marker → not a brief this contract covers
	}
	schema, described := schemas[s]
	switch {
	case described:
		// A marker this binary embeds a contract for → validate against THAT
		// contract below. Selecting per file, not per run, is what lets a tree
		// mid-migration hold brief-v1 and brief-v2 files side by side and have
		// each validated against the version it actually declares.
	case strings.HasPrefix(s, briefSchemaFamilyPrefix):
		// A brief-schema-family version no embedded contract describes (a future
		// brief-v3, …). Report it as a VERSION mismatch, not a field error: this is
		// the deliberate-migration signal on a pin bump, and it fails CLOSED so a
		// newer-than-the-binary brief is never validated green against the wrong
		// contract.
		return conformStateVersion, fmt.Sprintf(
			"%s: schema marker %q is newer than this binary's brief contracts (%s) — upgrade statusgen to a build whose embedded schema describes %q (schema-version mismatch, not a field error)",
			path, s, strings.Join(embeddedBriefSchemaMarkers(), ", "), s)
	default:
		return conformStateExempt, "" // a different document kind (contract-v1, …) → exempt
	}

	violations := validateValue(schema, data, "")
	if len(violations) > 0 {
		sort.Strings(violations)
		return conformStateFailed, fmt.Sprintf("%s: %s", path, strings.Join(violations, "; "))
	}
	return conformStateClean, ""
}

// ---------------------------------------------------------------------------
// Minimal JSON Schema (draft 2020-12) validator.
//
// This validates the SUBSET of keywords the committed brief schemas use — type,
// const, enum, required, properties, additionalProperties (boolean), items,
// minItems, minimum, pattern — against a value decoded from YAML frontmatter (so
// ints are int/int64,
// `yes`/`no` are strings, lists are []any, maps are map[string]any: exactly what
// the reference validator sees). It is deliberately small and closed rather than a
// general engine: the schema it validates is fixed and committed, and a
// hand-rolled subset keeps the binary dependency-free and fully offline. If the
// schema ever grows a keyword this validator does not implement,
// TestValidatorCoversSchemaKeywords fails until the keyword is handled here.
// ---------------------------------------------------------------------------

// schemaNode is a parsed schema (object) plus the raw map for keyword coverage.
type schemaNode struct {
	raw map[string]any
}

func parseSchema(b []byte) (*schemaNode, error) {
	var m map[string]any
	if err := yaml.Unmarshal(b, &m); err != nil { // JSON is a subset of YAML; reuse the one decoder.
		return nil, err
	}
	return &schemaNode{raw: m}, nil
}

// schemaKeywords is the closed set of keywords this validator implements. The
// coverage test asserts the committed schema uses no keyword outside it.
var schemaKeywords = map[string]bool{
	"$schema": true, "$id": true, "title": true, "description": true,
	"type": true, "const": true, "enum": true, "required": true,
	"properties": true, "additionalProperties": true, "items": true,
	"minItems": true, "minimum": true, "pattern": true,
}

// validateValue validates v against schema, returning human-readable violations
// each prefixed with the JSON-pointer-ish path to the offending value.
func validateValue(schema *schemaNode, v any, path string) []string {
	return validateNode(schema.raw, v, path)
}

func validateNode(node map[string]any, v any, path string) []string {
	var out []string
	label := path
	if label == "" {
		label = "(root)"
	}

	// type
	if t, ok := node["type"]; ok && !typeMatches(t, v) {
		out = append(out, fmt.Sprintf("%s: wrong type (want %s, got %s)", label, typeWant(t), goTypeName(v)))
		// A wrong type makes the sub-keyword checks below meaningless; stop here.
		return out
	}

	// const
	if c, ok := node["const"]; ok && !scalarEqual(c, v) {
		out = append(out, fmt.Sprintf("%s: must equal %v", label, c))
	}

	// enum
	if e, ok := node["enum"].([]any); ok {
		matched := false
		for _, cand := range e {
			if scalarEqual(cand, v) {
				matched = true
				break
			}
		}
		if !matched {
			out = append(out, fmt.Sprintf("%s: %v is not one of the allowed values %v", label, v, e))
		}
	}

	// minimum (numeric only) — the inclusive lower bound. Applied only to a value
	// that is actually numeric; a wrong type already returned above.
	if mn, ok := node["minimum"]; ok {
		if want, wantOK := numericValue(mn); wantOK {
			if got, gotOK := numericValue(v); gotOK && got < want {
				out = append(out, fmt.Sprintf("%s: %v is below the minimum %v", label, v, mn))
			}
		}
	}

	// pattern (string only)
	if p, ok := node["pattern"].(string); ok {
		if sv, isStr := v.(string); isStr {
			if re, err := regexp.Compile(p); err == nil && !re.MatchString(sv) {
				out = append(out, fmt.Sprintf("%s: %q does not match required pattern %s", label, sv, p))
			}
		}
	}

	// object keywords
	if m, isObj := v.(map[string]any); isObj {
		out = append(out, validateObject(node, m, path)...)
	}

	// array keywords
	if arr, isArr := v.([]any); isArr {
		out = append(out, validateArray(node, arr, path)...)
	}

	return out
}

func validateObject(node map[string]any, m map[string]any, path string) []string {
	var out []string

	if req, ok := node["required"].([]any); ok {
		for _, rk := range req {
			key, _ := rk.(string)
			if _, present := m[key]; !present {
				out = append(out, fmt.Sprintf("%s: missing required key %q", joinPath(path, ""), key))
			}
		}
	}

	props, _ := node["properties"].(map[string]any)
	// additionalProperties: only the boolean form is used by this schema.
	addlAllowed := true
	if ap, ok := node["additionalProperties"].(bool); ok {
		addlAllowed = ap
	}

	// stable key order for deterministic messages
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		sub, defined := props[k]
		if !defined {
			if !addlAllowed {
				out = append(out, fmt.Sprintf("%s: unknown key %q is not allowed here", joinPath(path, k), k))
			}
			continue
		}
		subMap, _ := sub.(map[string]any)
		out = append(out, validateNode(subMap, m[k], joinPath(path, k))...)
	}
	return out
}

func validateArray(node map[string]any, arr []any, path string) []string {
	var out []string
	if mi, ok := node["minItems"]; ok {
		if n := toInt(mi); len(arr) < n {
			out = append(out, fmt.Sprintf("%s: needs at least %d item(s), got %d", joinPath(path, ""), n, len(arr)))
		}
	}
	if items, ok := node["items"].(map[string]any); ok {
		for i, el := range arr {
			out = append(out, validateNode(items, el, fmt.Sprintf("%s[%d]", joinPath(path, ""), i))...)
		}
	}
	return out
}

// typeMatches reports whether v satisfies a schema `type` (a string or a list of
// strings — the union form).
func typeMatches(t any, v any) bool {
	switch tv := t.(type) {
	case string:
		return jsonTypeMatch(tv, v)
	case []any:
		for _, one := range tv {
			if s, ok := one.(string); ok && jsonTypeMatch(s, v) {
				return true
			}
		}
		return false
	}
	return false
}

func jsonTypeMatch(name string, v any) bool {
	switch name {
	case "object":
		_, ok := v.(map[string]any)
		return ok
	case "array":
		_, ok := v.([]any)
		return ok
	case "string":
		_, ok := v.(string)
		return ok
	case "integer":
		switch v.(type) {
		case int, int64:
			return true
		}
		return false
	case "number":
		switch v.(type) {
		case int, int64, float64:
			return true
		}
		return false
	case "boolean":
		_, ok := v.(bool)
		return ok
	case "null":
		return v == nil
	}
	return false
}

func typeWant(t any) string {
	switch tv := t.(type) {
	case string:
		return tv
	case []any:
		parts := make([]string, 0, len(tv))
		for _, one := range tv {
			if s, ok := one.(string); ok {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, " or ")
	}
	return fmt.Sprintf("%v", t)
}

func goTypeName(v any) string {
	if v == nil {
		return "null"
	}
	switch v.(type) {
	case map[string]any:
		return "object"
	case []any:
		return "array"
	case string:
		return "string"
	case int, int64:
		return "integer"
	case float64:
		return "number"
	case bool:
		return "boolean"
	}
	return reflect.TypeOf(v).String()
}

// scalarEqual compares two scalars across the int/int64/float64 spread YAML and
// JSON decode numbers into, and by value for strings/bools.
func scalarEqual(a, b any) bool {
	if reflect.DeepEqual(a, b) {
		return true
	}
	an, aok := numericValue(a)
	bn, bok := numericValue(b)
	if aok && bok {
		return an == bn
	}
	return false
}

func numericValue(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case float64:
		return n, true
	}
	return 0, false
}

func toInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	}
	return 0
}

func joinPath(path, key string) string {
	switch {
	case path == "" && key == "":
		return "(root)"
	case path == "":
		return key
	case key == "":
		return path
	default:
		return path + "." + key
	}
}

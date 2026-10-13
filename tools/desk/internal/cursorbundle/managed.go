package cursorbundle

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

const StatePath = ".cursor/assay-managed.json"

type inventory struct {
	Schema         int               `json:"schema"`
	Files          map[string]string `json:"files"`
	BindingsSHA256 string            `json:"bindingsSha256"`
}

func digest(b []byte) string { return fmt.Sprintf("%x", sha256.Sum256(b)) }
func validateBindings(raw []byte) error {
	s := string(raw)
	a, b := strings.Count(s, bindingsBegin), strings.Count(s, bindingsEnd)
	if a != b || a > 1 || (a == 1 && strings.Index(s, bindingsEnd) < strings.Index(s, bindingsBegin)) {
		return fmt.Errorf("malformed or repeated Assay bindings delimiters in AGENTS.md")
	}
	return nil
}
func bindings(raw []byte) []byte {
	s := string(raw)
	start := strings.Index(s, bindingsBegin)
	if start < 0 {
		return nil
	}
	end := strings.Index(s, bindingsEnd)
	if end < start {
		return nil
	}
	return raw[start : end+len(bindingsEnd)]
}
func generatedNotice(rel string, raw []byte) []byte {
	rel = filepath.ToSlash(rel)
	if !strings.HasSuffix(rel, ".md") && !strings.HasSuffix(rel, ".mdc") {
		return raw
	}
	source := rel
	if rel == "rules/assay.mdc" {
		source = "cursor/assay.mdc"
	}
	notice := []byte("<!-- GENERATED from plugins/assay/" + source + " by deskinstall --harness cursor. Do not edit. -->\n")
	if bytes.HasPrefix(raw, []byte("---\n")) {
		if end := bytes.Index(raw[4:], []byte("\n---\n")); end >= 0 {
			n := 4 + end + 5
			return append(append(append([]byte{}, raw[:n]...), notice...), raw[n:]...)
		}
	}
	return append(notice, raw...)
}

// managedPath is deliberately narrower than a general safe relative path: an
// inventory cannot retire files outside the generated Cursor payload roots.
func managedPath(rel string) bool {
	if rel != path.Clean(rel) || strings.ContainsAny(rel, `\:`) || strings.HasPrefix(rel, "/") {
		return false
	}
	parts := strings.Split(rel, "/")
	if len(parts) >= 4 && parts[0] == ".cursor" && parts[1] == "skills" && parts[2] != "" {
		return true
	}
	return len(parts) == 3 && parts[0] == ".cursor" && ((parts[1] == "references" && strings.HasSuffix(parts[2], ".md")) || (parts[1] == "rules" && parts[2] == "assay.mdc"))
}
func safeDestination(root, rel string) (string, error) {
	if rel != "AGENTS.md" && rel != StatePath && !managedPath(rel) {
		return "", fmt.Errorf("invalid managed destination %q", rel)
	}
	current := root
	for i, part := range strings.Split(rel, "/") {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("refusing symlinked destination %s", current)
		}
		if i < len(strings.Split(rel, "/"))-1 && !info.IsDir() {
			return "", fmt.Errorf("destination parent is not a directory: %s", current)
		}
		if i == len(strings.Split(rel, "/"))-1 && !info.Mode().IsRegular() {
			return "", fmt.Errorf("destination is not a regular file: %s", current)
		}
	}
	return current, nil
}
func readDestination(root, rel string) ([]byte, error) {
	full, err := safeDestination(root, rel)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(full)
	if os.IsNotExist(err) {
		return nil, nil
	}
	return raw, err
}
func readInventory(root string) (inventory, []byte, error) {
	raw, err := readDestination(root, StatePath)
	if err != nil {
		return inventory{}, nil, err
	}
	if raw == nil {
		return inventory{Schema: 1, Files: map[string]string{}}, nil, nil
	}
	var inv inventory
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&inv); err != nil {
		return inv, nil, fmt.Errorf("invalid Cursor ownership inventory: %w", err)
	}
	var extra any
	if err = decoder.Decode(&extra); err != io.EOF {
		return inv, nil, fmt.Errorf("trailing Cursor ownership inventory data")
	}
	if inv.Schema != 1 || len(inv.Files) == 0 || !shaPattern.MatchString(inv.BindingsSHA256) {
		return inv, nil, fmt.Errorf("invalid Cursor ownership inventory schema/content")
	}
	seen := map[string]bool{}
	for rel, sha := range inv.Files {
		if !managedPath(rel) || !shaPattern.MatchString(sha) || seen[strings.ToLower(rel)] {
			return inv, nil, fmt.Errorf("invalid or aliased Cursor ownership entry %q", rel)
		}
		seen[strings.ToLower(rel)] = true
	}
	return inv, raw, nil
}

// Plan is a preflighted local update. All collision/ownership checks happen
// before writes. Apply rechecks observed bytes; it is not a cross-file transaction.
type Plan struct {
	root     string
	files    map[string][]byte
	stale    []string
	observed map[string][]byte
	notice   string
}

func desiredFiles(opts Options) (map[string][]byte, string, error) {
	raw, err := buildCursorPlan(opts)
	if err != nil {
		return nil, "", err
	}
	files := map[string][]byte{}
	inv := inventory{Schema: 1, Files: map[string]string{}}
	seen := map[string]bool{}
	for _, f := range raw.files {
		rel := filepath.ToSlash(f.rel)
		if rel == "AGENTS.md" {
			inv.BindingsSHA256 = digest(bindings(f.content))
		} else {
			rel = ".cursor/" + rel
			inv.Files[rel] = digest(f.content)
		}
		if seen[strings.ToLower(rel)] {
			return nil, "", fmt.Errorf("aliased generated destination %s", rel)
		}
		seen[strings.ToLower(rel)] = true
		files[rel] = f.content
	}
	data, err := json.MarshalIndent(inv, "", "  ")
	if err != nil {
		return nil, "", err
	}
	files[StatePath] = append(data, '\n')
	return files, raw.ruleNotice, nil
}

func Prepare(opts Options) (*Plan, error) {
	files, notice, err := desiredFiles(opts)
	if err != nil {
		return nil, err
	}
	old, state, err := readInventory(opts.RepoRoot)
	if err != nil {
		return nil, err
	}
	p := &Plan{root: opts.RepoRoot, files: files, observed: map[string][]byte{StatePath: state}, notice: notice}
	for rel, want := range files {
		actual, err := readDestination(opts.RepoRoot, rel)
		if err != nil {
			return nil, err
		}
		p.observed[rel] = actual
		if actual == nil || bytes.Equal(actual, want) || rel == StatePath {
			continue
		}
		if rel == "AGENTS.md" {
			if state != nil && digest(bindings(actual)) != old.BindingsSHA256 && !bytes.Equal(bindings(actual), bindings(want)) {
				return nil, fmt.Errorf("locally modified Assay bindings in AGENTS.md; reconcile before sync")
			}
			continue
		}
		previous, owned := old.Files[rel]
		if !owned || digest(actual) != previous {
			return nil, fmt.Errorf("unmanaged or locally modified destination %s; reconcile before sync", rel)
		}
	}
	for rel, previous := range old.Files {
		if _, kept := files[rel]; kept {
			continue
		}
		actual, err := readDestination(opts.RepoRoot, rel)
		if err != nil {
			return nil, err
		}
		p.observed[rel] = actual
		if actual != nil && digest(actual) != previous {
			return nil, fmt.Errorf("locally modified stale generated file %s; refusing removal", rel)
		}
		p.stale = append(p.stale, rel)
	}
	sort.Strings(p.stale)
	return p, nil
}
func (p *Plan) Report(out io.Writer) {
	keys := make([]string, 0, len(p.files))
	for rel, want := range p.files {
		if !bytes.Equal(want, p.observed[rel]) {
			keys = append(keys, rel)
		}
	}
	sort.Strings(keys)
	for _, rel := range keys {
		fmt.Fprintf(out, "would write %s\n", rel)
	}
	for _, rel := range p.stale {
		fmt.Fprintf(out, "would retire unchanged generated file %s\n", rel)
	}
}
func (p *Plan) Apply(out io.Writer) error {
	if out == nil {
		out = io.Discard
	}
	for rel, before := range p.observed {
		actual, err := readDestination(p.root, rel)
		if err != nil {
			return err
		}
		if !bytes.Equal(actual, before) {
			return fmt.Errorf("destination changed after preview: %s; re-plan before applying", rel)
		}
	}
	keys := make([]string, 0, len(p.files))
	for rel := range p.files {
		if rel != StatePath {
			keys = append(keys, rel)
		}
	}
	sort.Strings(keys)
	for _, rel := range keys {
		dst, err := safeDestination(p.root, rel)
		if err != nil {
			return err
		}
		if bytes.Equal(p.files[rel], p.observed[rel]) {
			continue
		}
		if err = os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err = os.WriteFile(dst, p.files[rel], 0o644); err != nil {
			return err
		}
	}
	for _, rel := range p.stale {
		dst, err := safeDestination(p.root, rel)
		if err != nil {
			return err
		}
		if err = os.Remove(dst); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	dst, err := safeDestination(p.root, StatePath)
	if err != nil {
		return err
	}
	if err = os.WriteFile(dst, p.files[StatePath], 0o644); err != nil {
		return err
	}
	if p.notice != "" {
		fmt.Fprintln(out, p.notice)
	}
	fmt.Fprintf(out, "placed cursor harness: %d managed files; %d retired; unrelated files and AGENTS.md outside Assay bindings preserved\n", len(keys), len(p.stale))
	return nil
}
func Place(opts Options, out io.Writer) error {
	p, err := Prepare(opts)
	if err != nil {
		return err
	}
	return p.Apply(out)
}
func Check(opts Options) ([]DriftFinding, error) {
	files, _, err := desiredFiles(opts)
	if err != nil {
		return nil, err
	}
	old, _, err := readInventory(opts.RepoRoot)
	if err != nil {
		return nil, err
	}
	var findings []DriftFinding
	for rel, want := range files {
		actual, err := readDestination(opts.RepoRoot, rel)
		if err != nil {
			return nil, err
		}
		if actual == nil {
			findings = append(findings, DriftFinding{DriftMissing, rel})
		} else if !bytes.Equal(actual, want) {
			findings = append(findings, DriftFinding{DriftContentDiffers, rel})
		}
	}
	for rel := range old.Files {
		if _, kept := files[rel]; !kept {
			findings = append(findings, DriftFinding{DriftExtra, rel})
		}
	}
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Kind != findings[j].Kind {
			return findings[i].Kind < findings[j].Kind
		}
		return findings[i].Path < findings[j].Path
	})
	return findings, nil
}

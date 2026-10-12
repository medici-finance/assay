package cursorbundle

// Release bundles are integrity-checked against the selected umbrella's published
// composition. This is not build provenance; the binary installer's mandatory
// Sigstore verification is a separate, unchanged contract.
import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const AssetName = "cursor-bundle.tar.gz"
const MaxArchiveBytes = 16 << 20
const maxExpandedBytes = 64 << 20

var versionPattern = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)
var shaPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

type archiveManifest struct {
	Schema  int               `json:"schema"`
	Version string            `json:"version"`
	Files   map[string]string `json:"files"`
}

func sourceNames(plan *cursorPlan) []string {
	names := []string{"cursor/packaging.md", "codex/AGENTS-assay.md", ".claude-plugin/plugin.json"}
	for _, f := range plan.files {
		rel := filepath.ToSlash(f.rel)
		if rel == "AGENTS.md" {
			continue
		}
		if rel == "rules/assay.mdc" {
			rel = "cursor/assay.mdc"
		}
		names = append(names, rel)
	}
	sort.Strings(names)
	return names
}
func checkVersion(raw []byte, version string) error {
	var plugin struct {
		Version string `json:"version"`
	}
	if !versionPattern.MatchString(version) {
		return fmt.Errorf("Cursor bundle version must be strict vX.Y.Z")
	}
	if err := json.Unmarshal(raw, &plugin); err != nil {
		return fmt.Errorf("Cursor plugin metadata: %w", err)
	}
	if plugin.Version != strings.TrimPrefix(version, "v") {
		return fmt.Errorf("Cursor plugin version %q does not match selected release %s", plugin.Version, version)
	}
	return nil
}

// Pack generates a deterministic archive from the exact placement source set.
// It consumes already-generated canonical rule/resident files; CI owns freshness.
func Pack(bundleDir, version string, out io.Writer) error {
	repo, err := os.MkdirTemp("", "cursor-package-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(repo)
	plan, err := buildCursorPlan(Options{BundleDir: bundleDir, RepoRoot: repo, Forge: "github"})
	if err != nil {
		return err
	}
	if plan.ruleNotice != "" {
		return fmt.Errorf("release Cursor bundle requires cursor/assay.mdc")
	}
	files := map[string][]byte{}
	m := archiveManifest{Schema: 1, Version: version, Files: map[string]string{}}
	for _, name := range sourceNames(plan) {
		if err := validateSource(bundleDir, name); err != nil {
			return err
		}
		full := filepath.Join(bundleDir, filepath.FromSlash(name))
		info, err := os.Lstat(full)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("non-regular Cursor source %s", name)
		}
		data, err := os.ReadFile(full)
		if err != nil {
			return err
		}
		files[name] = data
		m.Files[name] = digest(data)
	}
	if err = checkVersion(files[".claude-plugin/plugin.json"], version); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	files["manifest.json"] = append(raw, '\n')
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	gz := gzip.NewWriter(out)
	tw := tar.NewWriter(gz)
	for _, name := range names {
		data := files[name]
		if err = tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(data)), Typeflag: tar.TypeReg}); err != nil {
			return err
		}
		if _, err = tw.Write(data); err != nil {
			return err
		}
	}
	if err = tw.Close(); err != nil {
		return err
	}
	return gz.Close()
}

// VerifiedBundle can only be constructed by the hash/closed-manifest verifier.
type VerifiedBundle struct{ files map[string][]byte }

func VerifyArchive(raw []byte, version, expectedSHA string) (*VerifiedBundle, error) {
	if !shaPattern.MatchString(expectedSHA) || digest(raw) != expectedSHA {
		return nil, fmt.Errorf("Cursor archive SHA-256 mismatch or absent expected digest")
	}
	if !versionPattern.MatchString(version) || len(raw) > MaxArchiveBytes {
		return nil, fmt.Errorf("invalid Cursor version or oversized archive")
	}
	gz, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	limited := &io.LimitedReader{R: gz, N: maxExpandedBytes + 1}
	tr := tar.NewReader(limited)
	files := map[string][]byte{}
	seen := map[string]bool{}
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if !fs.ValidPath(h.Name) || strings.ContainsAny(h.Name, `\:`) || h.Typeflag != tar.TypeReg || h.Size < 0 || h.Size > 8<<20 || seen[strings.ToLower(h.Name)] || len(files) >= 2048 {
			return nil, fmt.Errorf("invalid, duplicate, or non-regular Cursor archive entry %q", h.Name)
		}
		for _, part := range strings.Split(h.Name, "/") {
			if strings.HasSuffix(part, ".") || strings.HasSuffix(part, " ") {
				return nil, fmt.Errorf("nonportable Cursor archive path %q", h.Name)
			}
		}
		seen[strings.ToLower(h.Name)] = true
		data, err := io.ReadAll(tr)
		if err != nil {
			return nil, err
		}
		files[h.Name] = data
	}
	// Drain the gzip stream so a corrupt footer or expansion after tar EOF refuses.
	if _, err = io.Copy(io.Discard, limited); err != nil {
		return nil, err
	}
	if limited.N <= 0 {
		return nil, fmt.Errorf("Cursor archive exceeds expanded size limit")
	}
	manifest, ok := files["manifest.json"]
	if !ok {
		return nil, fmt.Errorf("Cursor archive has no closed manifest")
	}
	var m archiveManifest
	decoder := json.NewDecoder(bytes.NewReader(manifest))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&m); err != nil {
		return nil, err
	}
	var trailing any
	if decoder.Decode(&trailing) != io.EOF {
		return nil, fmt.Errorf("trailing Cursor manifest data")
	}
	if m.Schema != 1 || m.Version != version || len(m.Files)+1 != len(files) {
		return nil, fmt.Errorf("Cursor manifest schema/version/file set mismatch")
	}
	for name, sha := range m.Files {
		data, ok := files[name]
		if !ok || name == "manifest.json" || !shaPattern.MatchString(sha) || digest(data) != sha {
			return nil, fmt.Errorf("Cursor manifest content mismatch for %s", name)
		}
	}
	if err = checkVersion(files[".claude-plugin/plugin.json"], version); err != nil {
		return nil, err
	}
	delete(files, "manifest.json")
	return &VerifiedBundle{files: files}, nil
}

// Materialize writes verified bytes only into a caller-owned empty temporary
// directory. The returned source feeds the same planner as checkout generation.
func (v *VerifiedBundle) Materialize(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	if len(entries) != 0 {
		return fmt.Errorf("Cursor materialization directory must be empty")
	}
	for name, data := range v.files {
		dst := filepath.Join(dir, filepath.FromSlash(name))
		if err = os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err = os.WriteFile(dst, data, 0o644); err != nil {
			return err
		}
	}
	repo, err := os.MkdirTemp("", "cursor-validate-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(repo)
	plan, err := buildCursorPlan(Options{BundleDir: dir, RepoRoot: repo, Forge: "github"})
	if err != nil {
		return err
	}
	if plan.ruleNotice != "" {
		return fmt.Errorf("Cursor archive lacks generated rule")
	}
	names := sourceNames(plan)
	if len(names) != len(v.files) {
		return fmt.Errorf("Cursor archive contains files outside its placement source set")
	}
	for _, name := range names {
		if _, ok := v.files[name]; !ok {
			return fmt.Errorf("Cursor archive lacks source %s", name)
		}
	}
	return nil
}

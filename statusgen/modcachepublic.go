package main

// The host bracket's one repository-specific exception. This is deliberately
// absent from the cluster stager: that lane retains its owner-wide denial.
import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"
)

const publicAssayRepo = "medici-finance/assay"
const publicAssayRepoID int64 = 1321833604
const publicAssayOwnerID int64 = 298844171
const publicAssayAPI = "https://api.github.com/repos/" + publicAssayRepo
const publicAssayModulePrefix = "github.com/" + publicAssayRepo

// No token, cookies, ambient proxy, alternate endpoint or redirects. --module-cache
// is the existing explicit network opt-in; its proof target is announced first.
var publicAssayHTTP httpDoer = &http.Client{
	Timeout:       15 * time.Second,
	Transport:     &http.Transport{Proxy: nil},
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

type publicAssayModule struct {
	Root     string `json:"root"`
	Path     string `json:"path"`
	Version  string `json:"version"`
	Sum      string `json:"sum"`
	GoModSum string `json:"go_mod_sum"`
}
type receiptPublicAssay struct {
	Repository     string              `json:"repository"`
	RepositoryID   int64               `json:"repository_id"`
	OwnerID        int64               `json:"owner_id"`
	MetadataSHA256 string              `json:"metadata_sha256"`
	Modules        []publicAssayModule `json:"modules"`
}

func isPublicAssayModule(p string) bool {
	if p != publicAssayModulePrefix && !strings.HasPrefix(p, publicAssayModulePrefix+"/") {
		return false
	}
	for _, part := range strings.Split(p, "/") {
		if part == "" || strings.HasPrefix(part, ".") || strings.HasSuffix(part, ".") {
			return false
		}
		for _, c := range part {
			if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '.') {
				return false
			}
		}
	}
	return true
}
func operatorPrivateModule(p string, private []string) bool {
	for _, pat := range private {
		if matchModulePattern(pat, p) {
			return true
		}
	}
	return false
}
func validModuleSum(s string) bool {
	if !strings.HasPrefix(s, "h1:") {
		return false
	}
	b, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(s, "h1:"))
	return err == nil && len(b) == sha256.Size
}

// Every exception version, including go.sum-only versions, must have both
// committed checksums. Nothing is fetched to discover a missing version or sum.
func publicAssayBindings(dir, root string, env []string) ([]publicAssayModule, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "go.sum"))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	// A root with no dependencies need not carry go.sum. A public Assay
	// requirement below still refuses unless both committed checksums exist.
	sums := map[string]string{}
	versions := map[string]bool{}
	for _, line := range strings.Split(string(raw), "\n") {
		f := strings.Fields(line)
		if len(f) == 0 || !isPublicAssayModule(f[0]) {
			continue
		}
		if len(f) != 3 || !validModuleSum(f[2]) {
			return nil, fmt.Errorf("public Assay module has an invalid committed go.sum entry")
		}
		key := f[0] + " " + f[1]
		if old, ok := sums[key]; ok && old != f[2] {
			return nil, fmt.Errorf("public Assay module has conflicting committed checksums")
		}
		sums[key] = f[2]
		versions[f[0]+" "+strings.TrimSuffix(f[1], "/go.mod")] = true
	}
	parsed, err := goModEditJSON(filepath.Join(dir, "go.mod"), env)
	if err != nil {
		return nil, err
	}
	var mod struct {
		Require []struct{ Path, Version string }
		Replace []struct {
			Old, New struct{ Path, Version string }
		}
	}
	if err := json.Unmarshal(parsed, &mod); err != nil {
		return nil, err
	}
	// A require is bound unless a directory replaces it. A require replaced by
	// another module version is still bound: go.sum then holds
	// the replacement's checksums, not the Assay version's, so the
	// root refuses. That false refusal is deliberate and fails closed; the
	// request screen (moduleRequestPaths) also still lists the original path,
	// and dropping its binding would admit that path with no proof at all.
	// TestPublicAssayForkReplaceRefuses pins it.
	for _, r := range mod.Require {
		local := false
		for _, rp := range mod.Replace {
			if rp.Old.Path == r.Path && (rp.Old.Version == "" || rp.Old.Version == r.Version) && rp.New.Version == "" {
				local = true
			}
		}
		if !local && isPublicAssayModule(r.Path) {
			versions[r.Path+" "+r.Version] = true
		}
	}
	for _, r := range mod.Replace {
		if r.New.Version != "" && isPublicAssayModule(r.New.Path) {
			versions[r.New.Path+" "+r.New.Version] = true
		}
	}
	keys := make([]string, 0, len(versions))
	for k := range versions {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var out []publicAssayModule
	for _, k := range keys {
		p, v, ok := strings.Cut(k, " ")
		if !ok || !strings.HasPrefix(v, "v") || !validModuleSum(sums[k]) || !validModuleSum(sums[k+"/go.mod"]) {
			return nil, fmt.Errorf("public Assay module %s needs exact committed module and go.mod checksums", k)
		}
		out = append(out, publicAssayModule{Root: root, Path: p, Version: v, Sum: sums[k], GoModSum: sums[k+"/go.mod"]})
	}
	return out, nil
}

func provePublicAssay(modules []publicAssayModule, stdout io.Writer) (*receiptPublicAssay, error) {
	fmt.Fprintf(stdout, "module-public-proof GET %s (unauthenticated, redirects denied)\n", publicAssayAPI)
	req, err := http.NewRequest(http.MethodGet, publicAssayAPI, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	res, err := publicAssayHTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could-not-check — public Assay repository proof unavailable; nothing was sent to the proxy: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("could-not-check — public Assay repository proof returned HTTP %d; nothing was sent to the proxy", res.StatusCode)
	}
	const limit = 64 * 1024
	raw, err := io.ReadAll(io.LimitReader(res.Body, limit+1))
	if err != nil || len(raw) > limit {
		return nil, fmt.Errorf("could-not-check — public Assay repository proof unreadable or oversized; nothing was sent to the proxy")
	}
	var meta struct {
		ID         int64
		FullName   string `json:"full_name"`
		Private    *bool
		Visibility string
		HTMLURL    string `json:"html_url"`
		Owner      struct {
			ID    int64
			Login string
		}
	}
	if err := json.Unmarshal(raw, &meta); err != nil || meta.ID != publicAssayRepoID || meta.FullName != publicAssayRepo || meta.Private == nil || *meta.Private || meta.Visibility != "public" || meta.HTMLURL != "https://github.com/"+publicAssayRepo || meta.Owner.ID != publicAssayOwnerID || meta.Owner.Login != "medici-finance" {
		return nil, fmt.Errorf("could-not-check — public Assay repository identity/visibility is not proven; nothing was sent to the proxy")
	}
	digest := sha256.Sum256(raw)
	return &receiptPublicAssay{Repository: publicAssayRepo, RepositoryID: meta.ID, OwnerID: meta.Owner.ID, MetadataSHA256: hex.EncodeToString(digest[:]), Modules: modules}, nil
}

// Recheck the source binding at container admission and close without any
// network call. The outer receipt also binds the entire admitted home digest.
func publicAssayReceiptProblem(home string, roots []string, proof *receiptPublicAssay) error {
	var want []publicAssayModule
	for _, r := range roots {
		b, err := publicAssayBindings(filepath.Join(home, filepath.FromSlash(r)), r, moduleDownloadEnv("", ""))
		if err != nil {
			return fmt.Errorf("public Assay receipt binding: %w", err)
		}
		want = append(want, b...)
	}
	if len(want) == 0 && proof == nil {
		return nil
	}
	if proof == nil || len(want) == 0 || proof.Repository != publicAssayRepo || proof.RepositoryID != publicAssayRepoID || proof.OwnerID != publicAssayOwnerID || !slices.Equal(want, proof.Modules) {
		return fmt.Errorf("public Assay receipt proof does not match the admitted source module/version/checksums")
	}
	b, err := hex.DecodeString(proof.MetadataSHA256)
	if err != nil || len(b) != sha256.Size {
		return fmt.Errorf("public Assay receipt lacks its repository metadata digest")
	}
	return nil
}

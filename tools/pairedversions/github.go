package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// checksumsAsset is the file the pairing manifest's header names as the ONE
// authority for a pinned hash.
const checksumsAsset = "checksums.txt"

// ghAPI resolves releases over the GitHub REST API.
//
// It speaks to api.github.com directly rather than shelling out to `gh`: the
// self-hosted runners this repo's CI uses do not all ship the CLI, and a checker
// that silently skips when its tool is missing is the fail-OPEN shape this guard
// exists to prevent. A token is used when one is in the environment (needed for
// rate limit headroom, and for a private release home); the public release home
// resolves without one.
type ghAPI struct {
	baseURL string
	client  *http.Client
	token   string
}

func newGHAPI() *ghAPI {
	base := strings.TrimRight(os.Getenv("GITHUB_API_URL"), "/")
	if base == "" {
		base = "https://api.github.com"
	}
	tok := os.Getenv("GH_TOKEN")
	if tok == "" {
		tok = os.Getenv("GITHUB_TOKEN")
	}
	return &ghAPI{
		baseURL: base,
		client:  &http.Client{Timeout: 60 * time.Second},
		token:   tok,
	}
}

type ghAsset struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Digest string `json:"digest"`
}

type ghRelease struct {
	TagName    string    `json:"tag_name"`
	Draft      bool      `json:"draft"`
	Prerelease bool      `json:"prerelease"`
	Assets     []ghAsset `json:"assets"`
}

func (g *ghAPI) do(url, accept string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", accept)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if g.token != "" {
		req.Header.Set("Authorization", "Bearer "+g.token)
	}
	return g.client.Do(req)
}

// Release implements releaseSource against the GitHub REST API.
func (g *ghAPI) Release(home, tag string) (*release, error) {
	url := fmt.Sprintf("%s/repos/%s/releases/tags/%s", g.baseURL, home, tag)
	resp, err := g.do(url, "application/vnd.github+json")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		// Checked-FAILED, not could-not-check: the release home answered, and
		// its answer is that this tag names no release.
		return nil, errNoRelease
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: HTTP %d", url, resp.StatusCode)
	}
	var gr ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&gr); err != nil {
		return nil, fmt.Errorf("decoding release %s of %s: %w", tag, home, err)
	}

	rel := &release{
		Tag:        gr.TagName,
		Draft:      gr.Draft,
		Prerelease: gr.Prerelease,
		Checksums:  map[string]string{},
	}
	var sums *ghAsset
	for i := range gr.Assets {
		rel.Assets = append(rel.Assets, gr.Assets[i].Name)
		if gr.Assets[i].Name == checksumsAsset {
			sums = &gr.Assets[i]
		}
	}
	if sums == nil {
		return nil, fmt.Errorf("release %s of %s publishes no %s — the hash authority the pairing manifest names is absent", tag, home, checksumsAsset)
	}

	body, err := g.asset(home, sums.ID)
	if err != nil {
		return nil, err
	}
	// The API reports each asset's own digest. Verifying the bytes we just read
	// against it costs nothing and means a truncated or substituted transfer
	// cannot quietly become the hash authority the pins are compared to.
	if want, ok := strings.CutPrefix(sums.Digest, "sha256:"); ok {
		got := sha256.Sum256(body)
		if hex.EncodeToString(got[:]) != want {
			return nil, fmt.Errorf("%s of release %s does not match the digest the API reports for it (got %s, want %s)", checksumsAsset, tag, hex.EncodeToString(got[:]), want)
		}
	}
	sumsMap, err := parseChecksums(body)
	if err != nil {
		return nil, fmt.Errorf("parsing %s of release %s of %s: %w", checksumsAsset, tag, home, err)
	}
	rel.Checksums = sumsMap
	return rel, nil
}

func (g *ghAPI) asset(home string, id int64) ([]byte, error) {
	url := fmt.Sprintf("%s/repos/%s/releases/assets/%d", g.baseURL, home, id)
	resp, err := g.do(url, "application/octet-stream")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: HTTP %d", url, resp.StatusCode)
	}
	// A checksums.txt is a few hundred bytes; the cap keeps a wrong-URL HTML
	// body from being read into memory unbounded.
	return io.ReadAll(io.LimitReader(resp.Body, 1<<20))
}

// parseChecksums reads the `<sha256>  <name>` lines a checksums.txt carries.
// A line it cannot read is an error, not a skip: a partially-parsed authority
// would silently drop the very artifact whose hash is wrong.
func parseChecksums(b []byte) (map[string]string, error) {
	out := map[string]string{}
	sc := bufio.NewScanner(bytes.NewReader(b))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Fields(line)
		if len(f) != 2 {
			return nil, fmt.Errorf("line %q is not the `<sha256>  <name>` shape", line)
		}
		sum := f[0]
		name := strings.TrimPrefix(f[1], "*") // shasum's binary-mode marker
		if !sha256Re.MatchString(sum) {
			return nil, fmt.Errorf("line %q does not start with a bare lowercase 64-hex sha256", line)
		}
		out[name] = sum
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no checksum lines found")
	}
	return out, nil
}

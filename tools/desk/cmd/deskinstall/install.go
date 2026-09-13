package main

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"gopkg.in/yaml.v3"
)

// Fetcher downloads a release asset by URL. Injected in tests so the
// acquire→verify→place flow runs fully offline; nil in production means
// httpFetch (net/http, no external tool dependency).
type Fetcher func(url string) ([]byte, error)

// Options drives one install run.
type Options struct {
	ManifestPath string  // path to the pin manifest (paired-versions.yaml)
	DestDir      string  // PATH-resolvable directory the verified binaries land in
	Platform     string  // "windows-amd64" etc.; empty => detect from this host
	Fetch        Fetcher // nil => httpFetch
	Out          io.Writer
}

// componentManifest is one component's (statusgen / desk-tools) pin block in the
// manifest: a release home, a single pinned tag, and per-platform pin lines.
type componentManifest struct {
	ReleaseHome string            `yaml:"release_home"`
	Tag         string            `yaml:"tag"`
	Platforms   map[string]string `yaml:"platforms"`
}

// pairedVersions is the parsed manifest. Only the fields this installer needs.
type pairedVersions struct {
	Statusgen componentManifest `yaml:"statusgen"`
	DeskTools componentManifest `yaml:"desk-tools"`
}

// pin is a resolved, validated per-platform pin: the asset name, the pinned tag,
// and the expected sha256 the download is checked against.
type pin struct {
	component string // "statusgen" / "desk-tools"
	asset     string // e.g. statusgen-windows-amd64.exe
	tag       string // e.g. v0.26.0
	sha256    string // 64 lowercase hex
	home      string // release home, e.g. medici-finance/assay
}

var (
	reSHA256 = regexp.MustCompile(`^[0-9a-f]{64}$`)
	reTag    = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+`)
)

// detectPlatform reports this host's "<os>-<arch>" token (e.g. windows-amd64).
func detectPlatform() string { return runtime.GOOS + "-" + runtime.GOARCH }

// resolvePin extracts and VALIDATES the pin for one platform from a component
// block. It REFUSES (never guesses) when the platform line is absent, malformed,
// tag-mismatched, or carries a non-conforming sha256 — the security value the
// verify step compares against must itself be a well-formed, pinned digest.
func resolvePin(component string, m componentManifest, platform string) (pin, error) {
	line, ok := m.Platforms[platform]
	if !ok {
		return pin{}, fmt.Errorf("%s: no pin line for platform %q in the manifest — refusing to guess", component, platform)
	}
	fields := strings.Fields(line)
	if len(fields) != 3 {
		return pin{}, fmt.Errorf("%s/%s: malformed pin %q — want '<asset> <tag> <sha256>'", component, platform, line)
	}
	asset, tag, sum := fields[0], fields[1], fields[2]
	if !reTag.MatchString(tag) {
		return pin{}, fmt.Errorf("%s/%s: pin tag %q is not a pinned semver tag", component, platform, tag)
	}
	if tag != m.Tag {
		return pin{}, fmt.Errorf("%s/%s: pin tag %q disagrees with component tag %q", component, platform, tag, m.Tag)
	}
	if !reSHA256.MatchString(sum) {
		return pin{}, fmt.Errorf("%s/%s: sha256 %q is not 64 lowercase hex", component, platform, sum)
	}
	if m.ReleaseHome == "" {
		return pin{}, fmt.Errorf("%s: manifest has no release_home", component)
	}
	return pin{component: component, asset: asset, tag: tag, sha256: sum, home: m.ReleaseHome}, nil
}

// assetURL builds the release-download URL for a resolved pin.
func assetURL(p pin) string {
	return fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", p.home, p.tag, p.asset)
}

// verifySHA256 is the single load-bearing control: it computes the sha256 of the
// downloaded bytes and REFUSES on any mismatch. This is the ONE check between a
// substituted release asset and unverified bytes on disk. It never warns-and-
// continues: a mismatch is a returned error, and the caller places nothing.
func verifySHA256(got []byte, wantHex string) error {
	sum := sha256.Sum256(got)
	gotHex := hex.EncodeToString(sum[:])
	if gotHex != wantHex {
		return fmt.Errorf("sha256 mismatch: got %s, pinned %s — refusing to install unverified bytes", gotHex, wantHex)
	}
	return nil
}

// Install runs the full acquire→verify→place flow. It is two-phase by design:
// EVERY component is fetched and sha256-verified FIRST; only if all verify does
// anything get written to DestDir. A mismatch on any component therefore leaves
// nothing installed — the negative-path guarantee the security row asserts.
func Install(opts Options) error {
	out := opts.Out
	if out == nil {
		out = io.Discard
	}
	platform := opts.Platform
	if platform == "" {
		platform = detectPlatform()
	}
	fetch := opts.Fetch
	if fetch == nil {
		fetch = httpFetch
	}

	raw, err := os.ReadFile(opts.ManifestPath)
	if err != nil {
		return fmt.Errorf("reading manifest %s: %w", opts.ManifestPath, err)
	}
	var pv pairedVersions
	if err := yaml.Unmarshal(raw, &pv); err != nil {
		return fmt.Errorf("parsing manifest %s: %w", opts.ManifestPath, err)
	}

	// Resolve both pins up front — an absent/malformed pin refuses before any download.
	pins := []componentManifest{pv.Statusgen, pv.DeskTools}
	names := []string{"statusgen", "desk-tools"}
	resolved := make([]pin, 0, len(pins))
	for i, m := range pins {
		p, err := resolvePin(names[i], m, platform)
		if err != nil {
			return err
		}
		resolved = append(resolved, p)
	}

	// Phase 1 — fetch + VERIFY every component. Nothing is written yet.
	type verified struct {
		pin   pin
		bytes []byte
	}
	staged := make([]verified, 0, len(resolved))
	for _, p := range resolved {
		body, err := fetch(assetURL(p))
		if err != nil {
			return fmt.Errorf("%s: downloading %s: %w", p.component, p.asset, err)
		}
		if err := verifySHA256(body, p.sha256); err != nil {
			// Load-bearing refusal: FIRST post-download step, before any placement.
			return fmt.Errorf("%s (%s): %w", p.component, p.asset, err)
		}
		staged = append(staged, verified{pin: p, bytes: body})
	}

	// Phase 2 — place only verified bytes. Reached only when every hash matched.
	if err := os.MkdirAll(opts.DestDir, 0o755); err != nil {
		return fmt.Errorf("creating dest dir %s: %w", opts.DestDir, err)
	}
	for _, v := range staged {
		if err := place(opts.DestDir, v.pin, v.bytes); err != nil {
			return fmt.Errorf("%s: placing %s: %w", v.pin.component, v.pin.asset, err)
		}
		// Success line — brief 04's CI smoke and brief 05's doc assert on this shape.
		fmt.Fprintf(out, "installed %s %s sha256:%s\n", v.pin.component, v.pin.tag, v.pin.sha256)
	}
	// The binaries this mode places are INSIDE the boundary
	// (component-model.md §5 lists "the installed binaries" inside), so this
	// install writes no ledger line of its own — but the ledger path is still
	// worth naming: it is where a LATER outside step (a label, a ruleset
	// entry, an App installation — none of which this binary-only installer
	// performs) would be recorded, and it is what `deskdisable` reads when
	// reversing a component that does.
	fmt.Fprintf(out, "ledger: %s (this install has only inside-boundary effects; nothing to record)\n", deskkit.LedgerRelPath)
	return nil
}

// place writes the verified bytes into destDir: a single-binary .exe is written
// directly (0755); a .tar.gz is unpacked, each regular file placed executable.
func place(destDir string, p pin, body []byte) error {
	if strings.HasSuffix(p.asset, ".tar.gz") {
		return extractTarGz(destDir, body)
	}
	dst := filepath.Join(destDir, p.asset)
	return os.WriteFile(dst, body, 0o755)
}

// extractTarGz unpacks a desk-tools tarball into destDir. It places regular
// files only, flattened to their base name, and rejects any entry whose path
// would escape destDir (zip-slip guard).
func extractTarGz(destDir string, body []byte) error {
	gz, err := gzip.NewReader(strings.NewReader(string(body)))
	if err != nil {
		return fmt.Errorf("gzip: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("tar: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		base := path.Base(hdr.Name)
		if base == "." || base == ".." || base == "" || strings.Contains(base, "..") {
			return fmt.Errorf("tar: refusing suspicious entry %q", hdr.Name)
		}
		dst := filepath.Join(destDir, base)
		f, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
		if err != nil {
			return err
		}
		if _, err := io.Copy(f, tr); err != nil { //nolint:gosec // bounded by tar entry
			f.Close()
			return err
		}
		if err := f.Close(); err != nil {
			return err
		}
	}
}

// httpFetch is the production downloader: net/http, no external tool dependency,
// so the Go path carries no `gh`/shell requirement.
func httpFetch(url string) ([]byte, error) {
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: status %s", url, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

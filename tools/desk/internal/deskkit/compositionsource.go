package deskkit

// compositionsource.go — where a composition manifest COMES FROM when the adopter
// has not materialised one by hand.
//
// composition.go reads `releases/<umbrella>.yaml`, a file the version marker and
// `upgrade-assay` were written against — but one the release home never publishes.
// A release ships `checksums.txt` (one `<sha256>  <asset>` line per asset), the
// per-platform binaries, and a validation report; nothing materialises a manifest
// under an adopter's `releases/`, so every adopter hit "no such published release"
// on the sanctioned re-pin verb. This file closes that gap WITHOUT a second
// resolver: the composition is DERIVED from `checksums.txt`, the record the release
// actually publishes and the same file an adopter copies digests from by hand.
//
// The derivation is exact, not inferred. Under the plain `vX.Y.Z` scheme every
// asset on umbrella release T was built at tag T, so the composition of T is
// "one entry per component that ships an asset, each at tag T", and the per-asset
// digests are the ones `checksums.txt` lists — the digests channel E pins and
// hash-verifies the download against. Nothing is guessed: an asset name that is
// not `<component>-<os>-<arch>[.exe|.tar.gz]` is not a pinnable artifact and is
// skipped, a malformed digest refuses the whole file, and a checksums file that
// yields no component at all is refused rather than read as an empty composition.
//
// PRECEDENCE — explicit beats derived, present-but-broken beats absent:
//
//  1. `<releases>/<umbrella>.yaml`            — a hand-authored manifest WINS when
//     present; if it is present but unreadable/unparsable that is a refusal, never
//     a silent fall-through to derivation.
//  2. `<releases>/<umbrella>.checksums.txt`   — the release's checksums.txt,
//     materialised locally (`gh release download <tag> --pattern checksums.txt
//     -O releases/<tag>.checksums.txt`): the offline / air-gapped path.
//  3. the release home                        — `https://github.com/<home>/releases/
//     download/<umbrella>/checksums.txt`, fetched for exactly that tag, only when
//     the caller supplies a Fetch. Reaching the network is OPT-IN: both verbs
//     supply a Fetch only under an explicit `--fetch`, and every fetch announces
//     its exact URL on Announce (stderr) immediately before contact, whatever the
//     outcome — a tool never probes a remote by default or in silence.
//  4. nothing                                 — (‑, found=false, nil): the umbrella
//     is not a published release as far as this adopter can tell. The caller
//     refuses; this reader never invents a composition.
//
// A network failure that is NOT a clean "no such asset" (DNS down, a 5xx, a
// timeout) is an error, not found=false: "could not check" and "checked: absent"
// stay distinct, per the three-state rule.

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// DefaultReleaseHome is the `<owner>/<repo>` whose GitHub releases publish the
// umbrella assets — the same home `statusgen init`'s generated CI and
// docs/adopting-assay.md download from. Overridable per call (CompositionSource.
// ReleaseHome) for a mirror or a fork.
const DefaultReleaseHome = "medici-finance/assay"

// ChecksumsAsset is the release asset the composition is derived from.
const ChecksumsAsset = "checksums.txt"

// ChecksumsFileName is the local name a materialised checksums.txt takes under a
// releases dir: `<umbrella>.checksums.txt`, with `/` mapped to `-` exactly as the
// manifest name is (LoadComposition).
func ChecksumsFileName(umbrellaTag string) string {
	return strings.ReplaceAll(umbrellaTag, "/", "-") + ".checksums.txt"
}

// ManifestFileName is the local name of a hand-authored composition manifest under
// a releases dir — the name LoadComposition reads.
func ManifestFileName(umbrellaTag string) string {
	return strings.ReplaceAll(umbrellaTag, "/", "-") + ".yaml"
}

// ChecksumsURL is the release-home URL of a tag's checksums.txt.
func ChecksumsURL(home, umbrellaTag string) string {
	return fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", home, umbrellaTag, ChecksumsAsset)
}

// Fetcher downloads one URL. ErrNotFound (wrapped) means the server answered a
// clean 404 — "checked, absent" — as opposed to a transport failure.
type Fetcher func(url string) ([]byte, error)

// ErrNotFound is the sentinel a Fetcher wraps for an HTTP 404.
var ErrNotFound = errors.New("not found")

// HTTPFetch is the production Fetcher: a plain GET with a bounded timeout, no
// external tool. A 404 is reported as ErrNotFound so the caller can tell "no such
// release asset" from "could not reach the release home".
func HTTPFetch(url string) ([]byte, error) {
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("GET %s: %w", url, ErrNotFound)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: status %s", url, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 1<<20))
}

// ParseChecksums derives umbrellaTag's composition from the bytes of its
// checksums.txt. Every line is `<sha256>  <asset>` (sha256sum's output; a leading
// `*` binary marker on the asset is tolerated); blank lines and `#` comments are
// skipped. Each pinnable asset (`<component>-<os>-<arch>` with an optional `.exe`
// / `.tar.gz`) contributes its component at tag umbrellaTag and its digest under
// the pin-line name(s) the asset is pinned as: the asset name with `.tar.gz`
// stripped (`desk-tools-linux-amd64`) AND, when different, the full asset name
// (`desk-tools-windows-amd64.tar.gz`, the form the Windows docs show).
//
// Fail-closed: a line with the wrong field count, a digest that is not 64 lowercase
// hex, or a duplicate asset refuses the whole file; a file that yields no component
// is refused as well — an empty composition would let the marker report "known"
// on nothing.
func ParseChecksums(umbrellaTag string, raw []byte) (Composition, error) {
	c := Composition{Umbrella: umbrellaTag, Derived: true, AssetSHA256: map[string]string{}}
	seen := map[string]bool{}
	components := map[string]bool{}
	sc := bufio.NewScanner(bytes.NewReader(raw))
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			return Composition{}, Unverifiable(fmt.Sprintf(
				"checksums for %s: line %d is not `<sha256>  <asset>`: %q", umbrellaTag, lineNo, line), nil)
		}
		digest, asset := fields[0], strings.TrimPrefix(fields[1], "*")
		if !sha256Pattern.MatchString(digest) {
			return Composition{}, Unverifiable(fmt.Sprintf(
				"checksums for %s: line %d digest %q is not a 64-hex sha256", umbrellaTag, lineNo, digest), nil)
		}
		if seen[asset] {
			return Composition{}, Unverifiable(fmt.Sprintf(
				"checksums for %s: asset %q listed twice", umbrellaTag, asset), nil)
		}
		seen[asset] = true
		component := ComponentOf(asset)
		if component == asset {
			continue // not a <component>-<os>-<arch> asset: not a pinnable artifact
		}
		components[component] = true
		pinName := strings.TrimSuffix(asset, ".tar.gz")
		c.AssetSHA256[pinName] = digest
		c.AssetSHA256[asset] = digest
	}
	if err := sc.Err(); err != nil {
		return Composition{}, Unverifiable("checksums for "+umbrellaTag+": unreadable", err)
	}
	if len(components) == 0 {
		return Composition{}, Unverifiable(
			"checksums for "+umbrellaTag+" name no `<component>-<os>-<arch>` asset — nothing to derive a composition from", nil)
	}
	for name := range components {
		c.Artifacts = append(c.Artifacts, CompositionArtifact{Artifact: name, Tag: umbrellaTag})
	}
	sort.Slice(c.Artifacts, func(i, j int) bool { return c.Artifacts[i].Artifact < c.Artifacts[j].Artifact })
	return c, nil
}

// CompositionSource is the ordered set of places a composition can come from
// (see the file comment for the precedence). A zero ReleaseHome means
// DefaultReleaseHome; a nil Fetch means OFFLINE — only the two local files are
// consulted.
type CompositionSource struct {
	ReleasesDir string
	ReleaseHome string
	Fetch       Fetcher
	// Announce receives one `fetching <url>` line immediately BEFORE every Fetch
	// call, regardless of its outcome. nil means os.Stderr. It is never silenced:
	// an operator must be able to see every remote the tool contacted.
	Announce io.Writer
}

// announce prints the URL about to be fetched. Called before, never after, the
// fetch — a failed or refused fetch is still an attempted contact.
func (s CompositionSource) announce(url string) {
	w := s.Announce
	if w == nil {
		w = os.Stderr
	}
	fmt.Fprintf(w, "fetching %s\n", url)
}

func (s CompositionSource) home() string {
	if s.ReleaseHome == "" {
		return DefaultReleaseHome
	}
	return s.ReleaseHome
}

// Load resolves umbrellaTag's composition through the precedence above.
//
//   - (c, true, nil)   — a composition was read (hand-authored or derived).
//   - (‑, false, nil)  — nothing anywhere names the umbrella: not a published
//     release as far as this adopter can tell.
//   - (‑, false, err)  — a source was present but broken, or the release home could
//     not be checked. Fail-closed; never demoted to "absent".
func (s CompositionSource) Load(umbrellaTag string) (Composition, bool, error) {
	manifest := filepath.Join(s.ReleasesDir, ManifestFileName(umbrellaTag))
	if _, err := os.Stat(manifest); err == nil {
		c, lerr := LoadComposition(s.ReleasesDir, umbrellaTag)
		if lerr != nil {
			return Composition{}, false, lerr
		}
		return c, true, nil
	} else if !os.IsNotExist(err) {
		return Composition{}, false, Unverifiable("cannot stat composition manifest "+manifest, err)
	}

	local := filepath.Join(s.ReleasesDir, ChecksumsFileName(umbrellaTag))
	if raw, err := os.ReadFile(local); err == nil {
		c, perr := ParseChecksums(umbrellaTag, raw)
		if perr != nil {
			return Composition{}, false, Unverifiable("materialised "+local+": "+perr.Error(), perr)
		}
		c.Origin = local
		return c, true, nil
	} else if !os.IsNotExist(err) {
		return Composition{}, false, Unverifiable("cannot read materialised checksums "+local, err)
	}

	if s.Fetch == nil {
		return Composition{}, false, nil
	}
	url := ChecksumsURL(s.home(), umbrellaTag)
	s.announce(url)
	raw, err := s.Fetch(url)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return Composition{}, false, nil
		}
		return Composition{}, false, Unverifiable(
			"could not check the release home for "+umbrellaTag+" ("+url+")", err)
	}
	c, perr := ParseChecksums(umbrellaTag, raw)
	if perr != nil {
		return Composition{}, false, Unverifiable("fetched "+url+": "+perr.Error(), perr)
	}
	c.Origin = url
	return c, true, nil
}

// Describe names, for a refusal message, every place Load looked for umbrellaTag.
func (s CompositionSource) Describe(umbrellaTag string) string {
	parts := []string{
		filepath.Join(s.ReleasesDir, ManifestFileName(umbrellaTag)),
		filepath.Join(s.ReleasesDir, ChecksumsFileName(umbrellaTag)),
	}
	if s.Fetch != nil {
		parts = append(parts, ChecksumsURL(s.home(), umbrellaTag))
	} else {
		parts = append(parts, "(release home not consulted: pass --fetch to allow it)")
	}
	return strings.Join(parts, ", ")
}

// LocalUmbrellas lists the bare-umbrella tags materialised under ReleasesDir as
// either a manifest or a checksums file — the offline "published set". Only files
// whose stem is a bare vX.Y.Z and that actually load count; a broken file is
// silently not a release (it refuses loudly the moment it is named as a target).
func (s CompositionSource) LocalUmbrellas(isBare func(string) bool) (map[string]bool, error) {
	out := map[string]bool{}
	entries, err := os.ReadDir(s.ReleasesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		var tag string
		switch {
		case strings.HasSuffix(e.Name(), ".checksums.txt"):
			tag = strings.TrimSuffix(e.Name(), ".checksums.txt")
		case strings.HasSuffix(e.Name(), ".yaml"):
			tag = strings.TrimSuffix(e.Name(), ".yaml")
		default:
			continue
		}
		if !isBare(tag) || out[tag] {
			continue
		}
		offline := CompositionSource{ReleasesDir: s.ReleasesDir}
		if _, found, lerr := offline.Load(tag); lerr == nil && found {
			out[tag] = true
		}
	}
	return out, nil
}

// LatestReleaseTag asks the release home which release it marks latest
// (`/repos/<home>/releases/latest` → `tag_name`). ("", false, nil) when the home
// has no latest release; an error when it could not be asked. Used only by the
// "move to latest stable" verb when nothing is materialised locally; a nil Fetch
// returns ("", false, nil) — offline callers resolve latest from local files only.
func (s CompositionSource) LatestReleaseTag() (string, bool, error) {
	if s.Fetch == nil {
		return "", false, nil
	}
	url := "https://api.github.com/repos/" + s.home() + "/releases/latest"
	s.announce(url)
	raw, err := s.Fetch(url)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return "", false, nil
		}
		return "", false, Unverifiable("could not ask the release home for its latest release ("+url+")", err)
	}
	var body struct {
		TagName string `json:"tag_name"`
	}
	if err := json.Unmarshal(raw, &body); err != nil || body.TagName == "" {
		return "", false, Unverifiable("latest-release answer from "+url+" carries no tag_name", err)
	}
	return body.TagName, true, nil
}

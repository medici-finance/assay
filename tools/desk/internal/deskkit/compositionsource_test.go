package deskkit

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// checksumsFixture is a checksums.txt in the exact shape the umbrella release
// publishes (one line per asset, sha256sum output), with placeholder digests.
func checksumsFixture() string {
	d := func(c byte) string { return strings.Repeat(string(c), 64) }
	return strings.Join([]string{
		d('1') + "  statusgen-darwin-arm64",
		d('2') + "  statusgen-darwin-amd64",
		d('3') + "  statusgen-linux-amd64",
		d('4') + "  statusgen-windows-amd64.exe",
		d('5') + "  statusgen-windows-arm64.exe",
		d('6') + "  qualgen-darwin-arm64",
		d('7') + "  qualgen-linux-amd64",
		d('8') + "  desk-tools-darwin-arm64.tar.gz",
		d('9') + "  desk-tools-linux-amd64.tar.gz",
		d('a') + "  desk-tools-windows-amd64.tar.gz",
		d('b') + "  tool-validation-report.json", // not a pinnable asset: skipped
		"",
	}, "\n")
}

func TestParseChecksums_DerivesComposition(t *testing.T) {
	c, err := ParseChecksums("v1.0.8", []byte(checksumsFixture()))
	if err != nil {
		t.Fatal(err)
	}
	if !c.Derived || c.Umbrella != "v1.0.8" {
		t.Errorf("derived=%v umbrella=%q", c.Derived, c.Umbrella)
	}
	var names []string
	for _, a := range c.Artifacts {
		names = append(names, a.Artifact)
		if a.Tag != "v1.0.8" {
			t.Errorf("%s tag = %q, want the umbrella tag", a.Artifact, a.Tag)
		}
	}
	if got := strings.Join(names, ","); got != "desk-tools,qualgen,statusgen" {
		t.Errorf("components = %s", got)
	}
	for name, want := range map[string]string{
		"statusgen-darwin-arm64":          strings.Repeat("1", 64),
		"statusgen-windows-amd64.exe":     strings.Repeat("4", 64),
		"desk-tools-linux-amd64":          strings.Repeat("9", 64), // .tar.gz stripped
		"desk-tools-linux-amd64.tar.gz":   strings.Repeat("9", 64), // and the full asset name
		"desk-tools-windows-amd64.tar.gz": strings.Repeat("a", 64),
	} {
		if got := c.AssetSHA256[name]; got != want {
			t.Errorf("AssetSHA256[%s] = %q, want %q", name, got, want)
		}
	}
	if _, ok := c.AssetSHA256["tool-validation-report.json"]; ok {
		t.Error("a non-pinnable asset must not enter the digest map")
	}
}

func TestParseChecksums_FailClosed(t *testing.T) {
	for name, body := range map[string]string{
		"short digest":    "abc  statusgen-linux-amd64\n",
		"uppercase hex":   strings.ToUpper(strings.Repeat("a", 64)) + "  statusgen-linux-amd64\n",
		"wrong fields":    strings.Repeat("a", 64) + "\n",
		"duplicate asset": strings.Repeat("a", 64) + "  statusgen-linux-amd64\n" + strings.Repeat("b", 64) + "  statusgen-linux-amd64\n",
		"no component":    strings.Repeat("a", 64) + "  checksums.txt\n",
		"empty":           "",
	} {
		if _, err := ParseChecksums("v1.0.8", []byte(body)); err == nil || !IsUnverifiable(err) {
			t.Errorf("%s: want an Unverifiable refusal, got %v", name, err)
		}
	}
}

// fakeFetch serves a fixed URL→body map; anything else is a 404, and a URL in
// down is a transport failure.
func fakeFetch(bodies map[string]string, down ...string) Fetcher {
	return func(url string) ([]byte, error) {
		for _, d := range down {
			if url == d {
				return nil, errors.New("dial tcp: connection refused")
			}
		}
		if b, ok := bodies[url]; ok {
			return []byte(b), nil
		}
		return nil, fmt.Errorf("GET %s: %w", url, ErrNotFound)
	}
}

func TestCompositionSource_Precedence(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	url := ChecksumsURL(DefaultReleaseHome, "v1.0.8")
	fetched := CompositionSource{ReleasesDir: dir, Fetch: fakeFetch(map[string]string{url: checksumsFixture()})}

	// 3. nothing local → fetched from the release home, origin recorded.
	c, found, err := fetched.Load("v1.0.8")
	if err != nil || !found || !c.Derived || c.Origin != url {
		t.Fatalf("fetch path: found=%v derived=%v origin=%q err=%v", found, c.Derived, c.Origin, err)
	}
	// offline with nothing local: checked-absent, not an error.
	if _, found, err := (CompositionSource{ReleasesDir: dir}).Load("v1.0.8"); err != nil || found {
		t.Errorf("offline + nothing local: found=%v err=%v, want (false, nil)", found, err)
	}

	// 2. a materialised checksums.txt beats the network.
	write("v1.0.8.checksums.txt", checksumsFixture())
	c, found, err = fetched.Load("v1.0.8")
	if err != nil || !found || c.Origin != filepath.Join(dir, "v1.0.8.checksums.txt") {
		t.Fatalf("materialised path: found=%v origin=%q err=%v", found, c.Origin, err)
	}

	// 1. a hand-authored manifest beats both.
	write("v1.0.8.yaml", "umbrella: v1.0.8\nartifacts:\n  - artifact: statusgen\n    tag: v1.0.8\n")
	c, found, err = fetched.Load("v1.0.8")
	if err != nil || !found || c.Derived || len(c.Artifacts) != 1 {
		t.Fatalf("manifest path: found=%v derived=%v n=%d err=%v", found, c.Derived, len(c.Artifacts), err)
	}

	// A present-but-broken manifest REFUSES; it never falls through to derivation.
	write("v1.0.8.yaml", "umbrella: [\n")
	if _, _, err := fetched.Load("v1.0.8"); err == nil {
		t.Error("broken manifest must refuse, not fall through to checksums")
	}
	// Likewise a present-but-broken materialised checksums file.
	os.Remove(filepath.Join(dir, "v1.0.8.yaml"))
	write("v1.0.8.checksums.txt", "garbage\n")
	if _, _, err := fetched.Load("v1.0.8"); err == nil {
		t.Error("broken materialised checksums must refuse, not fall through to the network")
	}
}

func TestCompositionSource_NetworkStates(t *testing.T) {
	dir := t.TempDir()
	url := ChecksumsURL("example-org/mirror", "v9.9.9")
	// 404 → checked-absent.
	src := CompositionSource{ReleasesDir: dir, ReleaseHome: "example-org/mirror", Fetch: fakeFetch(nil)}
	if _, found, err := src.Load("v9.9.9"); err != nil || found {
		t.Errorf("404: found=%v err=%v, want (false, nil)", found, err)
	}
	// transport failure → error, never demoted to absent.
	src.Fetch = fakeFetch(nil, url)
	if _, found, err := src.Load("v9.9.9"); err == nil || found {
		t.Errorf("transport failure: found=%v err=%v, want an error", found, err)
	}
	// a fetched body that does not parse → error.
	src.Fetch = fakeFetch(map[string]string{url: "nonsense"})
	if _, _, err := src.Load("v9.9.9"); err == nil {
		t.Error("unparsable fetched checksums must refuse")
	}
	if !strings.Contains(src.Describe("v9.9.9"), url) {
		t.Errorf("Describe must name the URL consulted: %s", src.Describe("v9.9.9"))
	}
}

// TestCompositionSource_AnnouncesBeforeContact — every fetch prints its exact URL
// on Announce before the fetcher is called, whatever the outcome; and a source with
// no Fetch never announces anything (nothing to contact).
func TestCompositionSource_AnnouncesBeforeContact(t *testing.T) {
	dir := t.TempDir()
	var announced bytes.Buffer
	url := ChecksumsURL(DefaultReleaseHome, "v1.0.8")
	api := "https://api.github.com/repos/" + DefaultReleaseHome + "/releases/latest"
	seen := func(want string) Fetcher {
		return func(u string) ([]byte, error) {
			if !strings.HasSuffix(announced.String(), "fetching "+want+"\n") {
				t.Errorf("fetch of %s not announced first; announced so far: %q", u, announced.String())
			}
			return nil, errors.New("dial tcp: connection refused")
		}
	}
	src := CompositionSource{ReleasesDir: dir, Fetch: seen(url), Announce: &announced}
	if _, _, err := src.Load("v1.0.8"); err == nil {
		t.Error("transport failure must surface as an error")
	}
	src.Fetch = seen(api)
	if _, _, err := src.LatestReleaseTag(); err == nil {
		t.Error("transport failure must surface as an error")
	}
	if got := announced.String(); got != "fetching "+url+"\n"+"fetching "+api+"\n" {
		t.Errorf("announcements = %q", got)
	}
	announced.Reset()
	offline := CompositionSource{ReleasesDir: dir, Announce: &announced}
	offline.Load("v1.0.8")
	offline.LatestReleaseTag()
	if announced.Len() != 0 {
		t.Errorf("an offline source announced a fetch: %q", announced.String())
	}
	if !strings.Contains(offline.Describe("v1.0.8"), "--fetch") {
		t.Errorf("Describe must name the --fetch opt-in when offline: %s", offline.Describe("v1.0.8"))
	}
}

func TestCompositionSource_LocalUmbrellasAndLatest(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("v1.0.7.yaml", "umbrella: v1.0.7\nartifacts:\n  - artifact: statusgen\n    tag: v1.0.7\n")
	write("v1.0.8.checksums.txt", checksumsFixture())
	write("v1.0.9.checksums.txt", "broken\n") // not loadable: not a release
	write("statusgen-v0.1.0.yaml", "umbrella: statusgen/v0.1.0\n")
	bare := func(s string) bool { return strings.HasPrefix(s, "v") && !strings.Contains(s, "/") }
	got, err := (CompositionSource{ReleasesDir: dir}).LocalUmbrellas(bare)
	if err != nil {
		t.Fatal(err)
	}
	if !got["v1.0.7"] || !got["v1.0.8"] || got["v1.0.9"] || len(got) != 2 {
		t.Errorf("LocalUmbrellas = %v, want v1.0.7 (manifest) + v1.0.8 (checksums) only", got)
	}
	if got, err := (CompositionSource{ReleasesDir: filepath.Join(dir, "nonesuch")}).LocalUmbrellas(bare); err != nil || len(got) != 0 {
		t.Errorf("missing dir: %v %v", got, err)
	}

	api := "https://api.github.com/repos/" + DefaultReleaseHome + "/releases/latest"
	src := CompositionSource{ReleasesDir: dir, Fetch: fakeFetch(map[string]string{api: `{"tag_name":"v1.0.8","name":"v1.0.8"}`})}
	if tag, ok, err := src.LatestReleaseTag(); err != nil || !ok || tag != "v1.0.8" {
		t.Errorf("latest = (%q,%v,%v)", tag, ok, err)
	}
	if tag, ok, err := (CompositionSource{ReleasesDir: dir}).LatestReleaseTag(); err != nil || ok || tag != "" {
		t.Errorf("offline latest must be (\"\", false, nil), got (%q,%v,%v)", tag, ok, err)
	}
	if _, ok, err := (CompositionSource{ReleasesDir: dir, Fetch: fakeFetch(nil)}).LatestReleaseTag(); err != nil || ok {
		t.Errorf("404 latest must be (false, nil), got ok=%v err=%v", ok, err)
	}
}

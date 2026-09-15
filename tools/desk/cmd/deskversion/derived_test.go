package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// TestDeskversion_DerivedFromChecksums — the issue's adopter: per-platform pin
// lines only and no composition manifest, with the release's checksums.txt
// materialised under releases/. Known at exit 0, naming the derivation.
func TestDeskversion_DerivedFromChecksums(t *testing.T) {
	root := t.TempDir()
	rel := filepath.Join(root, deskkit.ReleasesDir)
	if err := os.MkdirAll(rel, 0o755); err != nil {
		t.Fatal(err)
	}
	checksums := strings.Repeat("1", 64) + "  statusgen-linux-amd64\n" +
		strings.Repeat("2", 64) + "  statusgen-darwin-arm64\n" +
		strings.Repeat("3", 64) + "  qualgen-linux-amd64\n" +
		strings.Repeat("4", 64) + "  desk-tools-linux-amd64.tar.gz\n"
	if err := os.WriteFile(filepath.Join(rel, "v1.0.8.checksums.txt"), []byte(checksums), 0o644); err != nil {
		t.Fatal(err)
	}
	pins := "assay v1.0.8\n" +
		"statusgen-linux-amd64 v1.0.8 " + strings.Repeat("1", 64) + "\n" +
		"statusgen-darwin-arm64 v1.0.8 " + strings.Repeat("2", 64) + "\n" +
		"desk-tools-linux-amd64 v1.0.8 " + strings.Repeat("4", 64) + "\n"
	if err := os.WriteFile(filepath.Join(root, deskkit.AssayVersionsFile), []byte(pins), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errb bytes.Buffer
	code := run([]string{"--root", root}, &out, &errb)
	if code != deskkit.ExitOK {
		t.Fatalf("exit %d, want 0\nstdout=%s\nstderr=%s", code, out.String(), errb.String())
	}
	for _, want := range []string{"state: known", "umbrella: v1.0.8", "statusgen v1.0.8", "desk-tools v1.0.8", "not pinned here", "qualgen", "composition derived from"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output missing %q:\n%s", want, out.String())
		}
	}

	// Without the materialised file and without --fetch: could-not-determine at the
	// unverifiable exit, naming the path it looked for and the --fetch opt-in — and
	// the fetcher is NEVER called (no default probe of a remote).
	os.Remove(filepath.Join(rel, "v1.0.8.checksums.txt"))
	prev := fetchFunc
	fetchFunc = func(url string) ([]byte, error) {
		t.Fatalf("fetcher called without --fetch: %s", url)
		return nil, nil
	}
	t.Cleanup(func() { fetchFunc = prev })
	out.Reset()
	code = run([]string{"--root", root}, &out, &errb)
	if code != deskkit.ExitUnverifiable || !strings.Contains(out.String(), "v1.0.8.checksums.txt") || !strings.Contains(out.String(), "--fetch") {
		t.Errorf("no source, no --fetch: exit %d\n%s", code, out.String())
	}

	// With --fetch and a fetcher that serves the tag, the same root reads known again,
	// and the exact URL was announced on stderr BEFORE the fetcher was contacted.
	url := deskkit.ChecksumsURL(deskkit.DefaultReleaseHome, "v1.0.8")
	errb.Reset()
	fetchFunc = func(u string) ([]byte, error) {
		if !strings.Contains(errb.String(), "fetching "+url+"\n") {
			t.Errorf("URL not announced before contact; stderr so far:\n%s", errb.String())
		}
		return []byte(checksums), nil
	}
	out.Reset()
	if code := run([]string{"--root", root, "--fetch"}, &out, &errb); code != deskkit.ExitOK || !strings.Contains(out.String(), url) {
		t.Errorf("fetched: exit %d\n%s", code, out.String())
	}
}

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// checksumsFor builds a checksums.txt in the release's published shape for tag,
// with placeholder digests derived from d so two tags' digests differ. The host's
// own statusgen asset is included so the bare `statusgen` line can take its digest.
func checksumsFor(d byte) string {
	sum := func(n int) string { return strings.Repeat(string(d), 63) + fmt.Sprint(n%10) }
	host := deskkit.HostPlatformAssets("statusgen")[0]
	lines := []string{
		sum(1) + "  " + host,
		sum(2) + "  statusgen-plan9-mips",
		sum(3) + "  qualgen-plan9-mips",
		sum(4) + "  desk-tools-linux-amd64.tar.gz",
		sum(5) + "  desk-tools-plan9-mips.tar.gz",
	}
	return strings.Join(lines, "\n") + "\n"
}

// writeDerivedFixture is the issue's adopter: per-platform pin lines (plus one bare
// statusgen line, as `statusgen init` now scaffolds), a source pin, an umbrella
// line — and NO composition manifest: only the two releases' checksums.txt,
// materialised under releases/.
func writeDerivedFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write := func(rel, body string) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	host := deskkit.HostPlatformAssets("statusgen")[0]
	write(".assay-versions", ""+
		"# adopter pin file\n"+
		"assay v0.12.0\n"+
		"statusgen v0.12.0 "+strings.Repeat("a", 64)+"\n"+
		host+" v0.12.0 "+strings.Repeat("a", 64)+"  # this host\n"+
		"desk-tools-linux-amd64 v0.12.0 "+strings.Repeat("b", 64)+"  # CI runners\n"+
		"desk-tools-source v0.12.0 "+strings.Repeat("c", 40)+"\n")
	write("releases/v0.12.0.checksums.txt", checksumsFor('1'))
	write("releases/v0.13.0.checksums.txt", checksumsFor('2'))
	return root
}

func TestDerived_DryRunAndApply(t *testing.T) {
	root := writeDerivedFixture(t)
	host := deskkit.HostPlatformAssets("statusgen")[0]

	code, out := run2(t, "--root", root, "--to", "v0.13.0", "--dry-run")
	if code != exitOK {
		t.Fatalf("dry-run: exit %d\n%s", code, out)
	}
	for _, want := range []string{"DRY-RUN", "derived from", "v0.13.0.checksums.txt", "statusgen v0.12.0 -> v0.13.0", "desk-tools v0.12.0 -> v0.13.0", "qualgen v0.12.0 -> v0.13.0 (not pinned here"} {
		if !strings.Contains(out, want) {
			t.Errorf("dry-run output missing %q:\n%s", want, out)
		}
	}
	before, _ := os.ReadFile(filepath.Join(root, ".assay-versions"))

	code, out = run2(t, "--root", root, "--to", "v0.13.0")
	if code != exitOK {
		t.Fatalf("apply: exit %d\n%s", code, out)
	}
	after, _ := os.ReadFile(filepath.Join(root, ".assay-versions"))
	if string(after) == string(before) {
		t.Fatal("apply must rewrite the pin file")
	}
	got := string(after)
	hostDigest := strings.Repeat("2", 63) + "1"
	for _, want := range []string{
		"assay v0.13.0\n",
		"statusgen v0.13.0 " + hostDigest + "\n",                            // bare line: this host's asset digest
		host + " v0.13.0 " + hostDigest + "\n",                              // platform line: its own asset digest
		"desk-tools-linux-amd64 v0.13.0 " + strings.Repeat("2", 63) + "4\n", // tarball digest, .tar.gz stripped
		"desk-tools-source v0.12.0 " + strings.Repeat("c", 40) + "\n",       // source pin untouched
		"# adopter pin file\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("re-pinned file missing %q:\n%s", want, got)
		}
	}
	// The marker now reads known at the target, from the derived composition.
	m := deskkit.ReadMarker(root, filepath.Join(root, deskkit.ReleasesDir))
	if m.State != deskkit.MarkerKnown || m.Umbrella != "v0.13.0" {
		t.Errorf("post-apply marker: %s %s (%s)", m.State, m.Umbrella, m.Reason)
	}
	// Idempotent: a second run is the already-at no-op.
	if code, out := run2(t, "--root", root, "--to", "v0.13.0"); code != exitOK || !strings.Contains(out, "already at") {
		t.Errorf("second apply: exit %d\n%s", code, out)
	}
}

func TestDerived_LatestFromLocalMaterialisation(t *testing.T) {
	root := writeDerivedFixture(t)
	code, out := run2(t, "--root", root, "--dry-run")
	if code != exitOK || !strings.Contains(out, "v0.12.0 -> v0.13.0") {
		t.Errorf("latest (offline) must resolve to the highest materialised umbrella: exit %d\n%s", code, out)
	}
}

func TestDerived_UnpublishedTargetRefuses(t *testing.T) {
	root := writeDerivedFixture(t)
	code, out := run2(t, "--root", root, "--to", "v0.14.0")
	if code != exitUnknownTarget {
		t.Fatalf("exit %d, want %d\n%s", code, exitUnknownTarget, out)
	}
	if !strings.Contains(out, "v0.14.0.checksums.txt") || !strings.Contains(out, "--fetch") {
		t.Errorf("refusal must name where it looked and the --fetch opt-in:\n%s", out)
	}
	if strings.Contains(out, "fetching ") {
		t.Errorf("no fetch may be announced without --fetch:\n%s", out)
	}
	for _, bad := range []string{"assum", "nearest", "latest"} {
		if strings.Contains(strings.ToLower(out), bad) {
			t.Errorf("unknown-target message must not contain %q:\n%s", bad, out)
		}
	}
}

// TestDerived_FetchesFromReleaseHome — nothing materialised for the target: the
// verb fetches that tag's checksums.txt (and, for latest, asks the release home).
func TestDerived_FetchesFromReleaseHome(t *testing.T) {
	root := writeDerivedFixture(t)
	url := deskkit.ChecksumsURL(deskkit.DefaultReleaseHome, "v0.14.0")
	api := "https://api.github.com/repos/" + deskkit.DefaultReleaseHome + "/releases/latest"
	served := map[string]string{url: checksumsFor('3'), api: `{"tag_name":"v0.14.0"}`}
	prev := fetchFunc
	fetchFunc = func(u string) ([]byte, error) {
		if b, ok := served[u]; ok {
			return []byte(b), nil
		}
		return nil, fmt.Errorf("GET %s: %w", u, deskkit.ErrNotFound)
	}
	t.Cleanup(func() { fetchFunc = prev })

	code, out := run2(t, "--root", root, "--to", "v0.14.0", "--dry-run", "--fetch")
	if code != exitOK || !strings.Contains(out, "derived from "+url) {
		t.Errorf("named fetched target: exit %d\n%s", code, out)
	}
	if !strings.Contains(out, "fetching "+url+"\n") {
		t.Errorf("the exact URL must be announced before contact:\n%s", out)
	}
	code, out = run2(t, "--root", root, "--dry-run", "--fetch")
	if code != exitOK || !strings.Contains(out, "v0.12.0 -> v0.14.0") || !strings.Contains(out, "fetching "+api+"\n") {
		t.Errorf("latest via the release home, announced: exit %d\n%s", code, out)
	}
	// A transport failure is a refusal that says so — never demoted to "not published" —
	// and the attempted URL was still announced.
	fetchFunc = func(u string) ([]byte, error) { return nil, errors.New("dial tcp: connection refused") }
	code, out = run2(t, "--root", root, "--to", "v0.14.0", "--dry-run", "--fetch")
	if code != exitUnknownTarget || !strings.Contains(out, "could not check the release home") || !strings.Contains(out, "fetching "+url) {
		t.Errorf("transport failure: exit %d\n%s", code, out)
	}
}

// TestHandAuthoredManifestStillWins — the original fixture (manifests with a
// per-component sha256:) behaves exactly as before alongside the new path.
func TestHandAuthoredManifestStillWins(t *testing.T) {
	root := writeFixture(t)
	if err := os.WriteFile(filepath.Join(root, "releases", "v0.13.0.checksums.txt"), []byte(checksumsFor('9')), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out := run2(t, "--root", root, "--to", "v0.13.0")
	if code != exitOK || strings.Contains(out, "derived from") {
		t.Fatalf("manifest must win over checksums: exit %d\n%s", code, out)
	}
	after, _ := os.ReadFile(filepath.Join(root, ".assay-versions"))
	if !strings.Contains(string(after), "statusgen v0.13.0 "+strings.Repeat("c", 64)) {
		t.Errorf("bare line must carry the manifest's sha256:\n%s", after)
	}
}

// TestNoFetchByDefault — without --fetch the verb never calls the fetcher, even
// when the target is unknown locally (no default probe of a remote).
func TestNoFetchByDefault(t *testing.T) {
	root := writeDerivedFixture(t)
	prev := fetchFunc
	fetchFunc = func(u string) ([]byte, error) {
		t.Fatalf("fetcher called without --fetch: %s", u)
		return nil, nil
	}
	t.Cleanup(func() { fetchFunc = prev })
	if code, _ := run2(t, "--root", root, "--to", "v0.14.0", "--dry-run"); code != exitUnknownTarget {
		t.Errorf("unknown target without --fetch: exit %d, want %d", code, exitUnknownTarget)
	}
	if code, _ := run2(t, "--root", root, "--dry-run"); code != exitOK {
		t.Errorf("latest from local materialisation without --fetch: exit %d", code)
	}
}

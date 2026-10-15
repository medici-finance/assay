package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// sumOf is checksumsFor's digest for asset index n under digit d.
func sumOf(d byte, n string) string { return strings.Repeat(string(d), 63) + n }

// stockPins is the pin file a stock install writes: per-artifact lines on one bare
// tag and NO `assay` umbrella line. Every digest is the one v1.0.29's checksums.txt
// (checksumsFor('1')) publishes for that asset.
func stockPins(statusgenSHA string) string {
	return "" +
		"# adopter pin file\n" +
		"statusgen v1.0.29 " + statusgenSHA + "  # linux-amd64 (CI-built)\n" +
		"statusgen-darwin-arm64 v1.0.29 " + sumOf('1', "1") + "\n" +
		"desk-tools-linux-amd64 v1.0.29 " + sumOf('1', "4") + "  # CI runners\n" +
		"desk-tools-source v1.0.29 " + strings.Repeat("c", 40) + "\n"
}

// writeAdoptFixture materialises an adopter with no umbrella line and the
// checksums.txt of v1.0.29 (what it is pinned at) and v1.0.30 (the target).
func writeAdoptFixture(t *testing.T, pins string) string {
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
	write(".assay-versions", pins)
	write("releases/v1.0.29.checksums.txt", checksumsFor('1'))
	write("releases/v1.0.30.checksums.txt", checksumsFor('2'))
	return root
}

func readPins(t *testing.T, root string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, ".assay-versions"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// Without the flag nothing changes: exit 6, and the refusal names the flag.
func TestAdopt_WithoutFlagRefusesAndNamesFlag(t *testing.T) {
	pins := stockPins(sumOf('1', "6"))
	root := writeAdoptFixture(t, pins)
	code, out := run2(t, "--root", root, "--to", "v1.0.30", "--dry-run")
	if code != exitUndetermined {
		t.Fatalf("exit %d, want %d\n%s", code, exitUndetermined, out)
	}
	for _, want := range []string{"could not be determined", "--adopt-umbrella-from-pins"} {
		if !strings.Contains(out, want) {
			t.Errorf("refusal missing %q:\n%s", want, out)
		}
	}
	if readPins(t, root) != pins {
		t.Error("a refusal must not touch the pin file")
	}
}

func TestAdopt_DryRunShowsEvidenceAndWritesNothing(t *testing.T) {
	pins := stockPins(sumOf('1', "6"))
	root := writeAdoptFixture(t, pins)
	code, out := run2(t, "--root", root, "--to", "v1.0.30", "--dry-run", "--adopt-umbrella-from-pins")
	if code != exitOK {
		t.Fatalf("dry-run: exit %d\n%s", code, out)
	}
	for _, want := range []string{
		"umbrella v1.0.29 inferred from the pin file: 3 pin(s) verified against",
		"v1.0.29.checksums.txt",
		"statusgen v1.0.29 matches statusgen-linux-amd64",
		"skipped:  line 5 desk-tools-source",
		"DRY-RUN — would move umbrella v1.0.29 -> v1.0.30",
		"would add the umbrella line `assay v1.0.30`",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("dry-run output missing %q:\n%s", want, out)
		}
	}
	if readPins(t, root) != pins {
		t.Error("a dry-run must not touch the pin file")
	}
}

func TestAdopt_ApplyWritesUmbrellaAndNewDigests(t *testing.T) {
	root := writeAdoptFixture(t, stockPins(sumOf('1', "6")))
	code, out := run2(t, "--root", root, "--to", "v1.0.30", "--adopt-umbrella-from-pins")
	if code != exitOK {
		t.Fatalf("apply: exit %d\n%s", code, out)
	}
	want := "" +
		"# adopter pin file\n" +
		"assay v1.0.30\n" +
		"statusgen v1.0.30 " + sumOf('2', "6") + "  # linux-amd64 (CI-built)\n" +
		"statusgen-darwin-arm64 v1.0.30 " + sumOf('2', "1") + "\n" +
		"desk-tools-linux-amd64 v1.0.30 " + sumOf('2', "4") + "  # CI runners\n" +
		"desk-tools-source v1.0.29 " + strings.Repeat("c", 40) + "\n"
	if got := readPins(t, root); got != want {
		t.Fatalf("re-pinned file:\n%s\nwant:\n%s", got, want)
	}
	// The marker now reads known at the target, and a plain re-run is the no-op.
	m := deskkit.ReadMarker(root, filepath.Join(root, deskkit.ReleasesDir))
	if m.State != deskkit.MarkerKnown || m.Umbrella != "v1.0.30" {
		t.Errorf("post-apply marker: %s %s (%s)", m.State, m.Umbrella, m.Reason)
	}
	if code, out := run2(t, "--root", root, "--to", "v1.0.30"); code != exitOK || !strings.Contains(out, "already at") {
		t.Errorf("second run: exit %d\n%s", code, out)
	}
	// The flag on a file that now has the line is a stated no-effect, not an error.
	if code, out := run2(t, "--root", root, "--to", "v1.0.30", "--adopt-umbrella-from-pins"); code != exitOK || !strings.Contains(out, "has no effect") {
		t.Errorf("flag on a known marker: exit %d\n%s", code, out)
	}
}

// Adopting AT the target is not the already-at no-op: the umbrella line is missing,
// and the apply writes it without changing any digest.
func TestAdopt_AtTargetWritesOnlyTheUmbrellaLine(t *testing.T) {
	pins := stockPins(sumOf('1', "6"))
	root := writeAdoptFixture(t, pins)
	code, out := run2(t, "--root", root, "--to", "v1.0.29", "--adopt-umbrella-from-pins")
	if code != exitOK {
		t.Fatalf("apply: exit %d\n%s", code, out)
	}
	want := strings.Replace(pins, "# adopter pin file\n", "# adopter pin file\nassay v1.0.29\n", 1)
	if got := readPins(t, root); got != want {
		t.Fatalf("pin file:\n%s\nwant:\n%s", got, want)
	}
}

// One pinned digest that differs from the published one is exit 5, naming the
// line and both digests, in a dry-run and an apply alike; the file stays
// byte-identical.
func TestAdopt_DigestMismatchRefusesInconsistent(t *testing.T) {
	bad := strings.Repeat("e", 64)
	pins := stockPins(bad)
	root := writeAdoptFixture(t, pins)
	for _, mode := range [][]string{{"--dry-run"}, nil} {
		args := append([]string{"--root", root, "--to", "v1.0.30", "--adopt-umbrella-from-pins"}, mode...)
		code, out := run2(t, args...)
		if code != exitInconsistent {
			t.Fatalf("%v: exit %d, want %d\n%s", mode, code, exitInconsistent, out)
		}
		for _, want := range []string{"line 2 statusgen v1.0.29", bad, sumOf('1', "6"), "statusgen-linux-amd64"} {
			if !strings.Contains(out, want) {
				t.Errorf("%v: refusal missing %q:\n%s", mode, want, out)
			}
		}
		if readPins(t, root) != pins {
			t.Fatalf("%v: the pin file changed on a refusal", mode)
		}
	}
}

// A pin set that does not prove one umbrella refuses could-not-determine.
func TestAdopt_UnprovableRefusesUndetermined(t *testing.T) {
	mixed := stockPins(sumOf('1', "6")) + "statusgen-darwin-amd64 v1.0.30 " + sumOf('2', "2") + "\n"
	root := writeAdoptFixture(t, mixed)
	code, out := run2(t, "--root", root, "--to", "v1.0.30", "--adopt-umbrella-from-pins")
	if code != exitUndetermined || !strings.Contains(out, "mixed tags") {
		t.Fatalf("mixed tags: exit %d\n%s", code, out)
	}
	if readPins(t, root) != mixed {
		t.Error("the pin file changed on a refusal")
	}
}

// The flag resolves ONLY the no-umbrella-line cause; any other could-not-determine
// still refuses. The refusal must come from the command's own gate, before inference
// runs: InferUmbrellaFromPins also refuses a file with an umbrella line, so asserting
// only the exit code would pass with the gate removed.
func TestAdopt_OtherUndeterminedCauseStillRefuses(t *testing.T) {
	pins := "assay v1.0.28\n" + stockPins(sumOf('1', "6")) // no v1.0.28 composition
	root := writeAdoptFixture(t, pins)
	code, out := run2(t, "--root", root, "--to", "v1.0.30", "--adopt-umbrella-from-pins")
	if code != exitUndetermined || !strings.Contains(out, "this cause is not that one") ||
		strings.Contains(out, "could not prove one umbrella") {
		t.Fatalf("exit %d\n%s", code, out)
	}
	if readPins(t, root) != pins {
		t.Error("the pin file changed on a refusal")
	}
}

// repinContent inserts the missing umbrella line before the first data line, and at
// the end (before the trailing newline) of a file with none.
func TestRepinContent_InsertsMissingUmbrella(t *testing.T) {
	toComp, _, err := deskkit.CompositionSource{ReleasesDir: t.TempDir()}.Load("v9.9.9")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ in, want string }{
		{"# only a header\n", "# only a header\nassay v9.9.9\n"},
		{"", "assay v9.9.9\n"},
		{"# h\n\nsomething-else v1.0.0 " + strings.Repeat("f", 64) + "\n",
			"# h\n\nassay v9.9.9\nsomething-else v1.0.0 " + strings.Repeat("f", 64) + "\n"},
	} {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, ".assay-versions"), []byte(c.in), 0o644); err != nil {
			t.Fatal(err)
		}
		_, out, err := repinContent(root, filepath.Join(root, "releases"), "v9.9.9", toComp, io.Discard)
		if err != nil {
			t.Fatal(err)
		}
		if string(out) != c.want {
			t.Errorf("in %q: got %q, want %q", c.in, out, c.want)
		}
	}
}

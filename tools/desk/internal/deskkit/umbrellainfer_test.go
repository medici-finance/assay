package deskkit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// inferRoot materialises an adopter root with NO umbrella line and the v1.0.8
// checksums fixture under releases/, returning the root and an offline source.
func inferRoot(t *testing.T, pins string) (string, CompositionSource) {
	t.Helper()
	return derivedRoot(t, pins) // checksumsFixture() as releases/v1.0.8.checksums.txt
}

func d64(c byte) string { return strings.Repeat(string(c), 64) }

func TestInferUmbrella_AllLinesMatch(t *testing.T) {
	root, src := inferRoot(t, "# adopter pins\n"+
		"statusgen-darwin-arm64 v1.0.8 "+d64('1')+"\n"+
		"statusgen-linux-amd64  v1.0.8 "+d64('3')+"  # CI\n"+
		"statusgen v1.0.8 "+d64('3')+"  # bare line: the linux-amd64 asset\n"+
		"desk-tools-linux-amd64 v1.0.8 "+d64('9')+"\n"+
		"statusgen-windows-amd64.exe v1.0.8 "+d64('4')+"\n")
	inf, err := InferUmbrellaFromPins(root, src)
	if err != nil {
		t.Fatalf("InferUmbrellaFromPins: %v", err)
	}
	if inf.Umbrella != "v1.0.8" || len(inf.Verified) != 5 {
		t.Fatalf("got umbrella %q, %d verified; want v1.0.8, 5:\n%s", inf.Umbrella, len(inf.Verified), inf.Evidence())
	}
	ev := inf.Evidence()
	for _, want := range []string{"5 pin(s) verified against", "v1.0.8.checksums.txt", "statusgen v1.0.8 matches statusgen-linux-amd64"} {
		if !strings.Contains(ev, want) {
			t.Errorf("evidence missing %q:\n%s", want, ev)
		}
	}
	if !inf.Composition.Derived || inf.Composition.Umbrella != "v1.0.8" {
		t.Errorf("composition not carried: %+v", inf.Composition)
	}
}

// One mismatched digest is exit 5, naming the line, the pinned and the published
// digest; the pin file is not touched (this reader never writes).
func TestInferUmbrella_MismatchIsInconsistent(t *testing.T) {
	pins := "statusgen-darwin-arm64 v1.0.8 " + d64('1') + "\n" +
		"statusgen-linux-amd64 v1.0.8 " + d64('e') + "\n"
	root, src := inferRoot(t, pins)
	_, err := InferUmbrellaFromPins(root, src)
	if ExitCodeOf(err) != ExitRefused {
		t.Fatalf("exit %d, want %d: %v", ExitCodeOf(err), ExitRefused, err)
	}
	for _, want := range []string{"known-inconsistent", "line 2 statusgen-linux-amd64", d64('e'), d64('3')} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal missing %q: %v", want, err)
		}
	}
	after, _ := os.ReadFile(filepath.Join(root, AssayVersionsFile))
	if string(after) != pins {
		t.Errorf("the pin file changed:\n%s", after)
	}
}

// A bare line is compared with <component>-linux-amd64, never another platform.
func TestInferUmbrella_BareLineUsesLinuxAmd64(t *testing.T) {
	root, src := inferRoot(t, "statusgen v1.0.8 "+d64('1')+"\n") // darwin-arm64's digest
	_, err := InferUmbrellaFromPins(root, src)
	if ExitCodeOf(err) != ExitRefused || !strings.Contains(err.Error(), "statusgen-linux-amd64") {
		t.Fatalf("bare line with a non-linux-amd64 digest: exit %d, %v", ExitCodeOf(err), err)
	}
}

func TestInferUmbrella_Refusals(t *testing.T) {
	for _, c := range []struct {
		name, pins, want string
	}{
		{"mixed tags",
			"statusgen-darwin-arm64 v1.0.8 " + d64('1') + "\nstatusgen-linux-amd64 v1.0.7 " + d64('3') + "\n",
			"mixed tags"},
		{"legacy per-artifact tag",
			"statusgen-linux-amd64 statusgen/v1.0.8 " + d64('3') + "\n",
			"legacy per-artifact tag"},
		{"unlisted asset",
			"statusgen-linux-amd64 v1.0.8 " + d64('3') + "\nstatusgen-plan9-mips v1.0.8 " + d64('3') + "\n",
			"lists no asset for line 2 statusgen-plan9-mips"},
		{"zero eligible lines",
			"# only comments\nstatusgen-source v1.0.8 " + strings.Repeat("c", 40) + "\n",
			"no eligible per-artifact pin line"},
		{"empty file", "# nothing pinned\n", "no per-artifact lines"},
		{"composition not loadable",
			"statusgen-linux-amd64 v1.0.9 " + d64('3') + "\n",
			"no composition for it was found"},
		{"umbrella line present",
			"assay v1.0.8\nstatusgen-linux-amd64 v1.0.8 " + d64('3') + "\n",
			"already carries an umbrella line"},
	} {
		t.Run(c.name, func(t *testing.T) {
			root, src := inferRoot(t, c.pins)
			_, err := InferUmbrellaFromPins(root, src)
			if ExitCodeOf(err) != ExitUnverifiable {
				t.Fatalf("exit %d, want %d: %v", ExitCodeOf(err), ExitUnverifiable, err)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("refusal %q missing %q", err, c.want)
			}
		})
	}
}

// A broken materialised checksums file is a could-not-determine, not a pass.
func TestInferUmbrella_BrokenComposition(t *testing.T) {
	root, src := inferRoot(t, "statusgen-linux-amd64 v1.0.8 "+d64('3')+"\n")
	if err := os.WriteFile(filepath.Join(src.ReleasesDir, "v1.0.8.checksums.txt"), []byte("not a checksums line\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := InferUmbrellaFromPins(root, src)
	if ExitCodeOf(err) != ExitUnverifiable || !strings.Contains(err.Error(), "could not be loaded") {
		t.Fatalf("exit %d, %v", ExitCodeOf(err), err)
	}
}

// `-source` lines and `same-tag: exempt` lines are skipped, even when their tag and
// digest would otherwise refuse, and the skip is recorded in the evidence.
func TestInferUmbrella_SkipsSourceAndExemptLines(t *testing.T) {
	root, src := inferRoot(t, ""+
		"statusgen-linux-amd64 v1.0.8 "+d64('3')+"\n"+
		"desk-tools-source v0.2.0 "+strings.Repeat("c", 40)+"\n"+
		"example-reconciler v2.4.1 "+d64('f')+"  # same-tag: exempt — separate release cadence\n")
	inf, err := InferUmbrellaFromPins(root, src)
	if err != nil {
		t.Fatalf("InferUmbrellaFromPins: %v", err)
	}
	if inf.Umbrella != "v1.0.8" || len(inf.Verified) != 1 || len(inf.Skipped) != 2 {
		t.Fatalf("got %+v", inf)
	}
	ev := inf.Evidence()
	if !strings.Contains(ev, "desk-tools-source") || !strings.Contains(ev, "same-tag: exempt") {
		t.Errorf("evidence must name the skipped lines:\n%s", ev)
	}
}

// The marker tells "no umbrella line" apart from every other could-not-determine
// cause, and its three states are unchanged.
func TestMarker_NoUmbrellaLineFlag(t *testing.T) {
	root, src := inferRoot(t, "statusgen-linux-amd64 v1.0.8 "+d64('3')+"\n")
	m := ReadMarkerFrom(root, src)
	if m.State != MarkerCouldNotDetermine || !m.NoUmbrellaLine {
		t.Fatalf("no umbrella line: state %s, NoUmbrellaLine %v", m.State, m.NoUmbrellaLine)
	}
	if strings.Contains(m.Report(), "NoUmbrellaLine") {
		t.Errorf("the report must not change:\n%s", m.Report())
	}
	// No pin file at all: could-not-determine, but not the no-umbrella-line cause.
	if m := ReadMarkerFrom(t.TempDir(), src); m.State != MarkerCouldNotDetermine || m.NoUmbrellaLine {
		t.Errorf("no file: state %s, NoUmbrellaLine %v", m.State, m.NoUmbrellaLine)
	}
	// An umbrella line with no composition: could-not-determine, other cause.
	root2, src2 := inferRoot(t, "assay v1.0.9\nstatusgen-linux-amd64 v1.0.9 "+d64('3')+"\n")
	if m := ReadMarkerFrom(root2, src2); m.State != MarkerCouldNotDetermine || m.NoUmbrellaLine {
		t.Errorf("missing composition: state %s, NoUmbrellaLine %v", m.State, m.NoUmbrellaLine)
	}
}

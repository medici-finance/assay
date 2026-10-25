package deskkit

// Flow tests for the run-bound prerequisite record (example-stream/50): what one
// verifier run executes against is admitted as a record, a missing input is
// could-not-check, a contradicted pin is a hold, neither is a code FAIL, an
// empty declaration is a successful empty scan, and the record admitted with a
// run is rechecked before its result may land.

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type world50Fake struct {
	revs map[string]string    // checkout path -> HEAD
	inst map[string][2]string // instrument -> {path, sha256}
	tool string
	host string
}

func (f *world50Fake) probes() PreflightProbes {
	return PreflightProbes{
		Revision: func(dir string) (string, error) {
			if r, ok := f.revs[dir]; ok {
				return r, nil
			}
			return "", errors.New("not a git checkout")
		},
		Instrument: func(name string) (string, string, error) {
			if v, ok := f.inst[name]; ok {
				return v[0], v[1], nil
			}
			return "", "", errors.New("executable file not found in $PATH")
		},
		RunningTool:  func() string { return f.tool },
		Runtime:      func() string { return f.host },
		DirExists:    func(d string) (bool, error) { _, ok := f.revs[d]; return ok, nil },
		SiblingRoots: func() ([]RootConfig, error) { return nil, nil },
	}
}

const (
	world50Rev     = "1111111111111111111111111111111111111111"
	world50Sib     = "2222222222222222222222222222222222222222"
	world50Digest  = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	world50Drifted = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

// world50Root builds a root with a pin file and a sibling, and a fake world
// in which every input is present and matches its pin.
func world50Root(t *testing.T) (string, string, *world50Fake) {
	t.Helper()
	parent := t.TempDir()
	root := filepath.Join(parent, "home")
	sib := filepath.Join(parent, "tracker")
	for _, d := range []string{root, sib} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	pins := "desk-tools-linux-amd64 desk-tools/v1.2.3 " + world50Digest + "\n" +
		"statusgen-linux-amd64 statusgen/v0.9.0 " + world50Digest + "\n"
	if err := os.WriteFile(filepath.Join(root, AssayVersionsFile), []byte(pins), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, sib, &world50Fake{
		revs: map[string]string{root: world50Rev, sib: world50Sib},
		inst: map[string][2]string{"statusgen": {"/opt/bin/statusgen", world50Digest}},
		tool: "desk-tools/v1.2.3",
		host: "linux-amd64",
	}
}

var world50Capability = Check{Name: CheckAppScopes, State: CheckedClean, Detail: "verifier App holds its duties"}

func world50Spec(root string) WorldSpec {
	return WorldSpec{Root: root, Repo: "example-org/one", Siblings: []string{"../tracker"},
		Instruments: []string{"statusgen"}, Capability: world50Capability}
}

func world50State(t *testing.T, a WorldAdmission, key string) CheckState {
	t.Helper()
	for _, c := range a.Checks {
		if c.Name == key {
			return c.State
		}
	}
	t.Fatalf("no %s check in %+v", key, a.Checks)
	return CouldNotCheck
}

// regression: example-stream/50 — a run's prerequisites were never recorded, so a
// missing instrument, drifted pin, moved sibling or denied capability spent a
// Verify row and read as the code's result.
func TestFlowWorld(t *testing.T) {
	t.Run("ready record names every input", func(t *testing.T) {
		root, sib, fake := world50Root(t)
		a := AdmitWorld(world50Spec(root), fake.probes())
		if !a.Ready() || a.Reason() != "" || a.Err() != nil {
			t.Fatalf("healthy world not ready: %s", a.Reason())
		}
		w := a.World
		if w.Inputs[WorldSource] != world50Rev || w.Inputs["sibling:../tracker"] != world50Sib ||
			w.Inputs["instrument:statusgen"] != "sha256:"+world50Digest || w.Inputs[WorldTool] != "desk-tools/v1.2.3" ||
			w.Inputs[WorldRuntime] != "linux-amd64" || w.Inputs[WorldCapability] == "" || w.Scanned != 2 {
			t.Fatalf("record incomplete: %+v", w)
		}
		if w.Paths["sibling:../tracker"] != sib || len(w.Digest()) != 64 {
			t.Fatalf("record paths/digest wrong: %+v %q", w.Paths, w.Digest())
		}
		if c := a.Check(); c.Name != CheckRunWorld || c.State != CheckedClean {
			t.Fatalf("ready admission check = %+v", c)
		}
	})

	t.Run("instrument missing is could-not-check", func(t *testing.T) {
		root, _, fake := world50Root(t)
		delete(fake.inst, "statusgen")
		a := AdmitWorld(world50Spec(root), fake.probes())
		if a.Ready() || world50State(t, a, "instrument:statusgen") != CouldNotCheck {
			t.Fatalf("missing instrument admitted: %+v", a.Checks)
		}
		if !strings.HasPrefix(a.Reason(), "environment could-not-check: ") || !strings.Contains(a.Reason(), "statusgen") {
			t.Fatalf("reason %q", a.Reason())
		}
		var de *DeskError
		if !errors.As(a.Err(), &de) || de.Code != ExitUnverifiable {
			t.Fatalf("missing instrument is not could-not-run: %v", a.Err())
		}
	})

	t.Run("pin drift is a hold, not could-not-check", func(t *testing.T) {
		root, _, fake := world50Root(t)
		fake.inst["statusgen"] = [2]string{"/opt/bin/statusgen", world50Drifted}
		a := AdmitWorld(world50Spec(root), fake.probes())
		if a.Ready() || world50State(t, a, "instrument:statusgen") != CheckedFailed ||
			!strings.HasPrefix(a.Reason(), "environment hold: ") || !strings.Contains(a.Reason(), "pin drift") {
			t.Fatalf("drifted instrument: %q", a.Reason())
		}
		var de *DeskError
		if !errors.As(a.Err(), &de) || de.Code != ExitUnverifiable {
			t.Fatalf("a hold must be could-not-run, never FAIL: %v", a.Err())
		}
		fake.inst["statusgen"] = [2]string{"/opt/bin/statusgen", world50Digest}
		fake.tool = "desk-tools/v1.2.2"
		if a = AdmitWorld(world50Spec(root), fake.probes()); world50State(t, a, WorldTool) != CheckedFailed {
			t.Fatalf("desk-tools pin drift admitted: %+v", a.Checks)
		}
		fake.tool = "dev"
		if a = AdmitWorld(world50Spec(root), fake.probes()); world50State(t, a, WorldTool) != CouldNotCheck {
			t.Fatalf("unstamped build vs pin must be could-not-check: %+v", a.Checks)
		}
		fake.tool, fake.host = "desk-tools/v1.2.3", "plan9-mips"
		if a = AdmitWorld(world50Spec(root), fake.probes()); world50State(t, a, WorldTool) != CheckedFailed ||
			!strings.Contains(a.Reason(), "unsupported runtime") {
			t.Fatalf("unpinned host runtime admitted: %q", a.Reason())
		}
	})

	t.Run("denied capability holds; unread capability is could-not-check", func(t *testing.T) {
		root, _, fake := world50Root(t)
		spec := world50Spec(root)
		spec.Capability = Check{Name: CheckAppScopes, State: CheckedFailed, Detail: "contents is read, duty needs write"}
		a := AdmitWorld(spec, fake.probes())
		if a.Ready() || world50State(t, a, WorldCapability) != CheckedFailed || !strings.Contains(a.Reason(), "denied App capability") {
			t.Fatalf("denied capability admitted: %q", a.Reason())
		}
		spec.Capability = Check{}
		if a = AdmitWorld(spec, fake.probes()); world50State(t, a, WorldCapability) != CouldNotCheck {
			t.Fatalf("unread capability admitted: %+v", a.Checks)
		}
		base := AdmitWorld(world50Spec(root), fake.probes()).World
		spec.Base = &base
		if a = AdmitWorld(spec, fake.probes()); !a.Ready() {
			t.Fatalf("capability carried from the root record was not used: %q", a.Reason())
		}
	})

	t.Run("zero scanned paths is a successful empty scan", func(t *testing.T) {
		root, _, fake := world50Root(t)
		spec := world50Spec(root)
		spec.Siblings, spec.Instruments = nil, nil
		a := AdmitWorld(spec, fake.probes())
		if !a.Ready() || a.World.Scanned != 0 || a.Check().State != CheckedClean {
			t.Fatalf("empty declaration is not a clean empty scan: %q %+v", a.Reason(), a.Check())
		}
		if a := (WorldAdmission{}); a.Ready() || a.Check().State != CouldNotCheck {
			t.Fatal("an admission that read nothing reads green")
		}
	})

	t.Run("changed sibling after admission invalidates the run", func(t *testing.T) {
		root, sib, fake := world50Root(t)
		w := AdmitWorld(world50Spec(root), fake.probes()).World
		if err := w.Recheck(fake.probes()); err != nil {
			t.Fatalf("unchanged world refused: %v", err)
		}
		fake.revs[sib] = "3333333333333333333333333333333333333333"
		err := w.Recheck(fake.probes())
		var de *DeskError
		if !errors.As(err, &de) || de.Code != ExitRefused || !strings.Contains(err.Error(), "sibling:../tracker") {
			t.Fatalf("moved sibling was not a refusal naming it: %v", err)
		}
		fake.revs[sib] = world50Sib
		fake.tool = "desk-tools/v1.2.4"
		if err = w.Recheck(fake.probes()); err == nil || !strings.Contains(err.Error(), WorldTool) {
			t.Fatalf("changed tool was not refused: %v", err)
		}
		fake.tool = "desk-tools/v1.2.3"
		delete(fake.revs, root)
		if err = w.Recheck(fake.probes()); !errors.As(err, &de) || de.Code != ExitUnverifiable {
			t.Fatalf("unreadable source must be could-not-check: %v", err)
		}
	})

	t.Run("second root with unreadable prerequisites reddens its preflight", func(t *testing.T) {
		root, _, fake := world50Root(t)
		p := okProbes()
		wp := fake.probes()
		p.Revision, p.Instrument, p.RunningTool, p.Runtime = wp.Revision, wp.Instrument, wp.RunningTool, wp.Runtime
		healthy := PreflightRequest{Role: pfRole, Root: root, Probes: p, World: &WorldSpec{}}.Run()
		if !healthy.Green() || healthy.World == nil || healthy.World.Inputs[WorldCapability] == "" {
			t.Fatalf("healthy root not admitted: %s", healthy.SummaryLine())
		}
		other := t.TempDir()
		held := PreflightRequest{Role: pfRole, Root: other, Probes: p, World: &WorldSpec{}}.Run()
		if held.Green() || held.World != nil || pfCheck(t, held, CheckRunWorld).State != CouldNotCheck ||
			!strings.Contains(held.Err().Error(), "cannot read the root revision") {
			t.Fatalf("unreadable root admitted or reason lost: %v", held.Err())
		}
		if plain := (PreflightRequest{Role: pfRole, Root: other, Probes: p}).Run(); !plain.Green() || len(plain.Checks) != 6 {
			t.Fatalf("the six boot checks changed for callers that ask for no record: %v", pfNames(plain))
		}
	})

	t.Run("bound record is rechecked by attestation admission", func(t *testing.T) {
		root, f := verifierFixture(t, "gpt-6-astra")
		if _, err := IssueVerifierAttestation(root, f); err != nil {
			t.Fatal(err)
		}
		sib := t.TempDir()
		fake := &world50Fake{revs: map[string]string{sib: world50Sib}, tool: "v", host: "linux-amd64"}
		w := RunWorld{Version: runWorldVersion, Root: root, Inputs: map[string]string{"sibling:../tracker": world50Sib},
			Paths: map[string]string{"sibling:../tracker": sib}}
		saved := worldRecheckProbes
		worldRecheckProbes = fake.probes()
		t.Cleanup(func() { worldRecheckProbes = saved })
		if _, err := BindVerifierWorld(root, w); err != nil {
			t.Fatal(err)
		}
		other := w
		other.Inputs = map[string]string{"sibling:../tracker": "4444444444444444444444444444444444444444"}
		if _, err := BindVerifierWorld(root, other); err == nil {
			t.Fatal("a bound run was silently rebound to a different record")
		}
		got, err := CheckVerifierAttestationWithForge(root, "brief.md", f)
		if err != nil || got.World == nil || got.World.Digest() != w.Digest() {
			t.Fatalf("bound record not admitted back: %+v %v", got.World, err)
		}
		fake.revs[sib] = "3333333333333333333333333333333333333333"
		if _, err = CheckVerifierAttestationWithForge(root, "brief.md", f); err == nil ||
			!strings.Contains(err.Error(), "environment changed since admission") {
			t.Fatalf("moved sibling admitted at landing: %v", err)
		}
	})

	t.Run("instruments are pinned command-position programs", func(t *testing.T) {
		root, _, _ := world50Root(t)
		got := WorldInstruments(root, []string{"cd tools && GOWORK=off statusgen --consumers", "go test ./x | grep ok", "./bin/statusgen -v"})
		if strings.Join(got, ",") != "statusgen" {
			t.Fatalf("instruments = %v", got)
		}
		if got := WorldInstruments(t.TempDir(), []string{"statusgen"}); got != nil {
			t.Fatalf("no pin file still named instruments: %v", got)
		}
	})
}

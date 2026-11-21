package deskkit

import (
	"errors"
	"net/http"
	"strings"
	"testing"
)

const (
	upgFrom = "v1.0.89"
	upgTo   = "v1.0.90"
)

// upgWorld is a record admitted under upgFrom with one sibling, and a fake in
// which the sibling has not moved and upgTo is running.
func upgWorld(t *testing.T) (RunWorld, *world50Fake, string) {
	t.Helper()
	sib := t.TempDir()
	fake := &world50Fake{revs: map[string]string{sib: world50Sib}, tool: upgTo, host: "linux-amd64"}
	w := RunWorld{Version: runWorldVersion, Root: t.TempDir(),
		Inputs: map[string]string{WorldTool: upgFrom, "sibling:../tracker": world50Sib},
		Paths:  map[string]string{"sibling:../tracker": sib}}
	return w, fake, sib
}

func pinOnMain(tag string) MainToolPin {
	return func() (string, string, error) {
		return tag, "example-org/one@main:.assay-versions desk-tools", nil
	}
}

func upgCode(t *testing.T, err error, code int, want string) {
	t.Helper()
	var de *DeskError
	if !errors.As(err, &de) || de.Code != code || !strings.Contains(err.Error(), want) {
		t.Fatalf("want exit %d naming %q, got %v", code, want, err)
	}
}

// regression: #7353 — a record admitted under one desk-tools release could not be
// rechecked for landing by any later release, so retiring a superseded record of
// the run's own brief needed a full re-dispatch.
func TestToolUpgradeRecheck(t *testing.T) {
	t.Run("an upgrade to main's pin is accepted and returned", func(t *testing.T) {
		w, fake, _ := upgWorld(t)
		u, err := w.RecheckLanding(fake.probes(), pinOnMain(upgTo))
		if err != nil || u == nil || u.From != upgFrom || u.To != upgTo || !strings.Contains(u.Pin, "@main:") {
			t.Fatalf("upgrade not accepted: %+v %v", u, err)
		}
		if got := u.Trailer(); got != "Tool-Upgrade: v1.0.89 -> v1.0.90 (pin example-org/one@main:.assay-versions desk-tools)" {
			t.Fatalf("trailer = %q", got)
		}
		fake.tool = upgFrom
		if u, err = w.RecheckLanding(fake.probes(), pinOnMain(upgTo)); err != nil || u != nil {
			t.Fatalf("an unchanged tool reported an upgrade: %+v %v", u, err)
		}
	})

	t.Run("control 1: an unstamped or pre-release build refuses", func(t *testing.T) {
		w, fake, _ := upgWorld(t)
		for _, running := range []string{"dev", "v1.0.91-rc1", "1.0.91"} {
			fake.tool = running
			_, err := w.RecheckLanding(fake.probes(), pinOnMain(running))
			upgCode(t, err, ExitRefused, "is not a stamped release")
		}
	})

	t.Run("control 2: a release that is not strictly newer refuses", func(t *testing.T) {
		w, fake, _ := upgWorld(t)
		for _, running := range []string{"v1.0.88", "v1.0.89+hotfix-abc123", "v0.9.99"} {
			fake.tool = running
			_, err := w.RecheckLanding(fake.probes(), pinOnMain(running))
			upgCode(t, err, ExitRefused, "is not newer than the admitted")
		}
	})

	t.Run("control 3: a newer release that is not main's pin refuses", func(t *testing.T) {
		w, fake, _ := upgWorld(t)
		fake.tool = "v1.0.91"
		_, err := w.RecheckLanding(fake.probes(), pinOnMain(upgTo))
		upgCode(t, err, ExitRefused, "is not the v1.0.90 pinned at")
	})

	t.Run("control 4: main unreadable is could-not-check and refuses", func(t *testing.T) {
		w, fake, _ := upgWorld(t)
		unread := func() (string, string, error) { return "", "", Unverifiable("cannot read main", errors.New("503")) }
		u, err := w.RecheckLanding(fake.probes(), unread)
		upgCode(t, err, ExitUnverifiable, "tool upgrade: cannot read main")
		if u != nil {
			t.Fatalf("an unread main returned an upgrade: %+v", u)
		}
	})

	t.Run("control 5: every other input still refuses, and plain Recheck stays exact", func(t *testing.T) {
		w, fake, sib := upgWorld(t)
		if err := w.Recheck(fake.probes()); err == nil || !strings.Contains(err.Error(), "changed since admission: tool") {
			t.Fatalf("plain Recheck accepted a tool change: %v", err)
		}
		fake.revs[sib] = "3333333333333333333333333333333333333333"
		_, err := w.RecheckLanding(fake.probes(), pinOnMain(upgTo))
		upgCode(t, err, ExitRefused, "changed since admission: sibling:../tracker")
		if _, err = w.RecheckLanding(fake.probes(), nil); err == nil {
			t.Fatal("a landing recheck with no main-pin reader admitted")
		}
	})
}

// upgForge serves ReadFile from files and delegates the rest.
type upgForge struct {
	*verifierForge
	files map[string]*FileContent
	err   error
	reads []string
}

func (f *upgForge) ReadFile(_ ForgeRepo, in ReadFileInput) (*FileContent, error) {
	f.reads = append(f.reads, in.Ref+":"+in.File)
	if f.err != nil {
		return nil, f.err
	}
	if fc, ok := f.files[in.File]; ok {
		return fc, nil
	}
	return nil, &ForgeAPIError{Status: http.StatusNotFound, Method: http.MethodGet, Path: in.File}
}

func regular(body string) *FileContent {
	return &FileContent{Content: []byte(body), SHA: "x", Exists: true, Kind: FileKindRegular}
}

// regression: #7353 — the pin a tool upgrade must equal is read from main of the
// tracking repo through the landing's forge, never from the local checkout.
func TestForgeMainToolPin(t *testing.T) {
	repo := ForgeRepo{Owner: "example-org", Name: "one"}
	read := func(f *upgForge) (string, string, error) {
		return ForgeMainToolPin(f, repo, "example-org/one", "linux-amd64")()
	}
	t.Run("pin file on main", func(t *testing.T) {
		f := &upgForge{files: map[string]*FileContent{AssayVersionsFile: regular("statusgen v1 x\ndesk-tools-linux-amd64.tar.gz v1.0.90 " + world50Digest + "\n")}}
		tag, src, err := read(f)
		if err != nil || tag != upgTo || src != "example-org/one@main:.assay-versions desk-tools-linux-amd64.tar.gz" {
			t.Fatalf("got %q %q %v", tag, src, err)
		}
		if len(f.reads) != 1 || f.reads[0] != "main:"+AssayVersionsFile {
			t.Fatalf("reads = %v", f.reads)
		}
	})
	t.Run("source tree falls back to the release manifest", func(t *testing.T) {
		f := &upgForge{files: map[string]*FileContent{PairedVersionsPath: regular("schema: x\nstatusgen:\n  tag: v9.9.9\ndesk-tools:\n  tag: v1.0.90\n")}}
		tag, src, err := read(f)
		if err != nil || tag != upgTo || !strings.HasSuffix(src, PairedVersionsPath+" desk-tools.tag") {
			t.Fatalf("got %q %q %v", tag, src, err)
		}
	})
	t.Run("a pin file with no desk-tools line refuses, never falls back", func(t *testing.T) {
		f := &upgForge{files: map[string]*FileContent{AssayVersionsFile: regular("statusgen v1 x\n"),
			PairedVersionsPath: regular("desk-tools:\n  tag: v1.0.90\n")}}
		_, _, err := read(f)
		upgCode(t, err, ExitRefused, "no desk-tools pin on main")
	})
	t.Run("no pin anywhere on main refuses", func(t *testing.T) {
		_, _, err := read(&upgForge{})
		upgCode(t, err, ExitRefused, "no desk-tools pin on main")
	})
	t.Run("an unreadable or non-regular main is could-not-check", func(t *testing.T) {
		_, _, err := read(&upgForge{err: errors.New("connection reset")})
		upgCode(t, err, ExitUnverifiable, "cannot read .assay-versions on main")
		link := &FileContent{Exists: true, Kind: FileKindSymlink}
		_, _, err = read(&upgForge{files: map[string]*FileContent{AssayVersionsFile: link}})
		upgCode(t, err, ExitUnverifiable, "is not a regular file")
		_, _, err = ForgeMainToolPin(nil, repo, "example-org/one", "linux-amd64")()
		upgCode(t, err, ExitUnverifiable, "no forge")
	})
}

// regression: #7353 — Evidence admission accepts the upgrade and hands it to the
// landing to record; row-execution admission still binds the admitted release.
func TestToolUpgradeAdmission(t *testing.T) {
	root, vf := verifierFixture(t, "gpt-6-astra")
	if _, err := IssueVerifierAttestation(root, vf); err != nil {
		t.Fatal(err)
	}
	fake := &world50Fake{revs: map[string]string{}, tool: upgFrom, host: "linux-amd64"}
	saved := worldRecheckProbes
	worldRecheckProbes = fake.probes()
	t.Cleanup(func() { worldRecheckProbes = saved })
	w := RunWorld{Version: runWorldVersion, Root: root, Inputs: map[string]string{WorldTool: upgFrom}}
	if _, err := BindVerifierWorld(root, w); err != nil {
		t.Fatal(err)
	}
	f := &upgForge{verifierForge: vf, files: map[string]*FileContent{AssayVersionsFile: regular("desk-tools v1.0.90 " + world50Digest + "\n")}}

	got, err := CheckVerifierEvidenceWithForge(root, "example-org/one", "brief.md", f)
	if err != nil || got.ToolUpgrade != nil {
		t.Fatalf("unchanged tool: %+v %v", got.ToolUpgrade, err)
	}
	fake.tool = upgTo
	got, err = CheckVerifierEvidenceWithForge(root, "example-org/one", "brief.md", f)
	if err != nil || got.ToolUpgrade == nil || got.ToolUpgrade.To != upgTo || got.ToolUpgrade.From != upgFrom {
		t.Fatalf("Evidence admission did not accept and return the upgrade: %+v %v", got.ToolUpgrade, err)
	}
	if _, err = CheckVerifierAttestationWithForge(root, "brief.md", f); err == nil ||
		!strings.Contains(err.Error(), "environment changed since admission: tool") {
		t.Fatalf("row-execution admission accepted a tool change: %v", err)
	}
	f.err = errors.New("connection reset")
	if _, err = CheckVerifierEvidenceWithForge(root, "example-org/one", "brief.md", f); ExitCodeOf(err) != ExitUnverifiable {
		t.Fatalf("unread main at landing was not could-not-check: %v", err)
	}
}

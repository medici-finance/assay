package deskkit

// overrideform_test.go — a refusal names the override form of the caller that hit it
// (#6059): the --force-scan-override flag on a flag-taking verb, the
// DESKPUSHGUARD_SCAN_OVERRIDE variable in the pre-push hook. A non-overridable refusal
// advertises neither.

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const (
	flagRerun = "re-run with --" + ScanOverrideFlag
	envRerun  = "re-run the push with " + EnvPushGuardScanOverride + "="
)

// formCaller sets the process context the way one caller does, with reason "" or a reason.
type formCaller struct {
	name  string
	set   func(t *testing.T, reason string)
	named string // the form's spelling its refusals must name
	other string // the other form's spelling, which they must never name
}

func formCallers() []formCaller {
	return []formCaller{
		{"deskfile", func(t *testing.T, reason string) {
			SetOutboundContext(OutboundContext{Tool: "deskfile", Verb: "new", OverrideReason: reason})
		}, "--" + ScanOverrideFlag, EnvPushGuardScanOverride},
		{"deskpushguard", func(t *testing.T, reason string) {
			t.Setenv(EnvPushGuardScanOverride, reason)
			if _, err := SetPushGuardOutboundContext("deskpushguard", "pre-push"); err != nil {
				t.Fatalf("SetPushGuardOutboundContext(%q): %v", reason, err)
			}
		}, EnvPushGuardScanOverride, ScanOverrideFlag},
	}
}

// regression: #6059
func TestRefusalNamesCallerForm(t *testing.T) {
	obRoster(t)
	defer SetOutboundNoticeWriter(&bytes.Buffer{})()
	write := func(repo, text string) error {
		return OutboundCheck(OutboundWrite{Repo: repo, Kind: OutboundKindFile,
			Fields: []OutboundField{{Name: "notes.txt", Text: text}}})
	}
	pii := "the vendor contact is " + obEmailOutside + " per the thread"
	for _, c := range formCallers() {
		t.Run(c.name+"/overridable", func(t *testing.T) {
			setup(t)
			c.set(t, "")
			err := write(obPrivate, pii)
			if ExitCodeOf(err) != ExitRefused {
				t.Fatalf("want a refusal, got %v", err)
			}
			want := flagRerun
			if c.named == EnvPushGuardScanOverride {
				want = envRerun
			}
			if msg := err.Error(); !strings.Contains(msg, want) || strings.Contains(msg, c.other) {
				t.Fatalf("refusal must name %q and never %q:\n%s", want, c.other, msg)
			}
		})
		for _, nc := range []struct{ rule, repo, text string }{
			{RuleWithheldIdentifier, obPublic, "mirrors the " + obWithheld + " stream"},
			{RuleVoiceRulingClaim, obPrivate, obRulingClaim},
		} {
			t.Run(c.name+"/"+nc.rule, func(t *testing.T) {
				setup(t)
				c.set(t, "")
				err := write(nc.repo, nc.text)
				if ExitCodeOf(err) != ExitRefused || !strings.Contains(err.Error(), nc.rule) {
					t.Fatalf("want a %s refusal, got %v", nc.rule, err)
				}
				for _, ad := range []string{ScanOverrideFlag, EnvPushGuardScanOverride, "re-run"} {
					if strings.Contains(err.Error(), ad) {
						t.Fatalf("a non-overridable refusal advertises %q:\n%s", ad, err)
					}
				}
				// With a reason supplied it says the caller's own form does not apply.
				c.set(t, "the operator believes this one is fine")
				err = write(nc.repo, nc.text)
				msg := ""
				if err != nil {
					msg = err.Error()
				}
				if !strings.Contains(msg, c.named+" does not apply to "+nc.rule) ||
					strings.Contains(msg, c.other) || strings.Contains(msg, "re-run") {
					t.Fatalf("want %q naming only %s, no re-run hint:\n%s", "does not apply", c.named, msg)
				}
			})
		}
	}
}

// regression: #6059
func TestDeskprRefusalNamesFlag(t *testing.T) {
	obRoster(t)
	setup(t)
	body := []byte("token " + scanGHToken)
	err := HandleScanRefusal(ScanOverride{Tool: "deskpr", Verb: "create", Surface: "PR body",
		Content: body}, ScanSurface("PR body", body))
	if ExitCodeOf(err) != ExitRefused || !strings.Contains(err.Error(), flagRerun) ||
		strings.Contains(err.Error(), EnvPushGuardScanOverride) {
		t.Fatalf("deskpr refusal must name only %q:\n%v", flagRerun, err)
	}
	ruling := []byte(obRulingClaim)
	scan := ScanSurface("PR body", ruling)
	if scan == nil {
		t.Fatal("the ruling-claim fixture no longer refuses")
	}
	err = HandleScanRefusal(ScanOverride{Tool: "deskpr", Verb: "create", Surface: "PR body",
		Content: ruling}, scan)
	for _, ad := range []string{ScanOverrideFlag, EnvPushGuardScanOverride, "re-run"} {
		if err == nil || strings.Contains(err.Error(), ad) {
			t.Fatalf("a non-overridable deskpr refusal advertises %q: %v", ad, err)
		}
	}
}

// regression: #6059
func TestPushGuardMalformedNamesEnv(t *testing.T) {
	setup(t)
	t.Setenv(EnvPushGuardScanOverride, "fp")
	_, err := SetPushGuardOutboundContext("deskpushguard", "pre-push")
	if ExitCodeOf(err) != ExitRefused || !strings.Contains(err.Error(), "refused: "+EnvPushGuardScanOverride) ||
		strings.Contains(err.Error(), ScanOverrideFlag) {
		t.Fatalf("a malformed hook reason must refuse naming %s only: %v", EnvPushGuardScanOverride, err)
	}
}

// reEnvOverrideRead matches a read of the hook's override variable: any Getenv/LookupEnv
// whose line names it by constant (deskkit's or the hook's alias) or literal.
var reEnvOverrideRead = regexp.MustCompile(`(Getenv|LookupEnv)\(.*(PushGuardScanOverride|EnvScanOverride|SCAN_OVERRIDE)`)

func envOverrideReads(src string) (lines []int) {
	for i, ln := range strings.Split(src, "\n") {
		if reEnvOverrideRead.MatchString(ln) {
			lines = append(lines, i+1)
		}
	}
	return lines
}

// TestPushGuardEnvReadOnce keeps the hook's override variable read in ONE place,
// SetPushGuardOutboundContext, which sets the env form along with the reason. A second
// reader would carry the reason with the zero-value flag form, and its refusals would name
// the flag a push does not take — the #6059 defect at a new site.
//
// regression: #6059
func TestPushGuardEnvReadOnce(t *testing.T) {
	// Positive control: the matcher flags a planted reader in either spelling.
	planted := "x := os.Getenv(deskkit.EnvPushGuardScanOverride)\ny := os.LookupEnv(EnvScanOverride)"
	if got := envOverrideReads(planted); len(got) != 2 {
		t.Fatalf("the matcher missed a planted reader: flagged lines %v, want [1 2]", got)
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	const home = "internal/deskkit/outboundpush.go"
	seenHome := false
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, werr error) error {
		if werr != nil {
			return werr
		}
		if d.IsDir() && (d.Name() == "testdata" || d.Name() == "vendor") {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		src, rerr := os.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		reads := envOverrideReads(string(src))
		if rel == home {
			seenHome = len(reads) == 1
			return nil
		}
		for _, ln := range reads {
			t.Errorf("%s:%d reads %s outside SetPushGuardOutboundContext; route it through that function",
				rel, ln, EnvPushGuardScanOverride)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !seenHome {
		t.Fatalf("%s no longer holds exactly one read of %s — the guard's anchor moved", home, EnvPushGuardScanOverride)
	}
}

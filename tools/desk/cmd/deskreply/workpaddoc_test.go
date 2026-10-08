package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// This is the installed-binary path: read the template workers actually see,
// then run the real deskreply --dry-run with the existing offline forge fixture.
func documentedWorkpad(text string) (string, bool) {
	const start = "cat > \"$WORKPAD\" <<'WORKPAD_BODY'\n"
	_, after, ok := strings.Cut(text, start)
	if !ok {
		return "", false
	}
	body, _, ok := strings.Cut(after, "\nWORKPAD_BODY")
	return body, ok
}

func TestDocTemplateControl(t *testing.T) {
	const body = "known body"
	const prefix = "cat > \"$WORKPAD\" <<'WORKPAD_BODY'\n"
	if got, ok := documentedWorkpad(prefix + body + "\nWORKPAD_BODY"); !ok || got != body {
		t.Fatalf("positive control: %q, %v", got, ok)
	}
	for _, text := range []string{body, prefix + body} {
		if _, ok := documentedWorkpad(text); ok {
			t.Fatal("incomplete template accepted")
		}
	}
}

func TestDocWorkpadDryRun(t *testing.T) {
	body, ok := documentedWorkpad(usage)
	if !ok {
		t.Fatal("help omits the runnable workpad template")
	}
	w, ok := deskkit.Parse(body)
	if !ok || body != deskkit.Render(w) {
		t.Fatal("documented template is not canonical Render output")
	}
	work := newBaseFixture(t)
	withEnv(t, work)
	swapWorkpadSeams(t, func(deskkit.Forge, deskkit.ForgeRepo, int, string) ([]workpadCandidate, error) { return nil, nil }, nil)
	for _, marked := range []bool{true, false} {
		input := body
		if !marked {
			input = strings.Replace(input, deskkit.WorkpadMarker+"\n", "", 1)
		}
		bf := bodyFileWith(t, input)
		rc := run([]string{"example-org/tracker", "7", "--workpad", "--body-file", bf, "--dry-run"})
		want := deskkit.ExitOK
		if !marked {
			want = deskkit.ExitRefused
		}
		if rc != want {
			t.Fatalf("marked=%v: rc=%d want=%d", marked, rc, want)
		}
		if forgeRec(t).posted() {
			t.Fatal("documented rehearsal wrote to the forge")
		}
	}
}

// The documentation defect class is an incomplete body-construction protocol
// on any shipped instruction surface, rather than a relaxed body validator.
// The help holds the one copyable template; the kits carry the shape and point at it.
func TestWorkpadDocSurfaces(t *testing.T) {
	helpBody, ok := documentedWorkpad(usage)
	if !ok {
		t.Fatal("help omits template")
	}
	for _, heading := range []string{"## Plan", "## Acceptance criteria", "## Validation", "## Notes"} {
		if !strings.Contains(helpBody, "\n"+heading+"\n") {
			t.Errorf("help template omits %q", heading)
		}
	}
	for _, phrase := range []string{"requires a source checkout", "./examples/workpad-render", "--dry-run", "DESK_LOOP=worker-desk"} {
		if !strings.Contains(usage, phrase) {
			t.Errorf("help: missing %q", phrase)
		}
	}
	for _, path := range []string{"../deskdispatch/references/common-clauses.md", "../deskdispatch/references/worker-prompt-objective.md"} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, dup := documentedWorkpad(string(data)); dup {
			t.Errorf("%s: carries a second copy of the help template", path)
		}
		for _, phrase := range []string{"`" + deskkit.WorkpadMarker + "`", "`deskreply --help`", "WORKPAD BODY", "`## Acceptance criteria`", "--dry-run"} {
			if !strings.Contains(string(data), phrase) {
				t.Errorf("%s: missing %q", path, phrase)
			}
		}
	}
}

// The refusal is the surface a worker reads at the moment it fails (#2122 review F1): it
// must send the caller to the installed-binary template, never to the internal library.
func TestWorkpadRefusalNamesHelp(t *testing.T) {
	work := newBaseFixture(t)
	withEnv(t, work)
	bf := bodyFileWith(t, "an ordinary reply body with no workpad marker")
	out := captureStderr(t, func() {
		if rc := run([]string{"example-org/tracker", "7", "--workpad", "--body-file", bf}); rc != deskkit.ExitRefused {
			t.Fatalf("rc = %d, want refused", rc)
		}
	})
	if !strings.Contains(out, "deskreply --help") || !strings.Contains(out, "WORKPAD BODY") {
		t.Errorf("refusal does not name the help template: %s", out)
	}
	if strings.Contains(out, "deskkit.") {
		t.Errorf("refusal points at an internal library: %s", out)
	}
}

func TestSourceWorkpadRender(t *testing.T) {
	cmd := exec.Command("go", "run", "./examples/workpad-render")
	cmd.Dir = filepath.Join("..", "..")
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOPROXY=off")
	cmd.Stdin = strings.NewReader(`{"Stamp":"example@abc1234","Plan":"- one","Acceptance":"- two","Validation":"not run","Notes":"none"}`)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("documented source renderer: %v: %s", err, out)
	}
	want := deskkit.Render(deskkit.Workpad{Stamp: "example@abc1234", Plan: "- one", Acceptance: "- two", Validation: "not run", Notes: "none"})
	if string(out) != want {
		t.Fatalf("renderer output differs: %s", out)
	}
	bad := exec.Command("go", "run", "./examples/workpad-render")
	bad.Dir, bad.Env = cmd.Dir, cmd.Env
	bad.Stdin = strings.NewReader(`{"Acceptance criteria":"- two"}`)
	if out, err := bad.CombinedOutput(); err == nil {
		t.Fatalf("unknown JSON key accepted: %s", out)
	}
}

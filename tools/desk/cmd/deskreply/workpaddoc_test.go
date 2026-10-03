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
func TestWorkpadDocSurfaces(t *testing.T) {
	helpBody, ok := documentedWorkpad(usage)
	if !ok {
		t.Fatal("help omits template")
	}
	for _, path := range []string{"../deskdispatch/references/common-clauses.md", "../deskdispatch/references/worker-prompt-objective.md"} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		body, ok := documentedWorkpad(string(data))
		if !ok || body != helpBody {
			t.Errorf("%s: workpad template missing or differs from help", path)
		}
		for _, phrase := range []string{"requires a source checkout", "./examples/workpad-render", "--dry-run", "DESK_LOOP=worker-desk"} {
			if !strings.Contains(string(data), phrase) {
				t.Errorf("%s: missing %q", path, phrase)
			}
		}
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
}

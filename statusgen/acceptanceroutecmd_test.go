package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestAcceptanceRoutePrepareValidatesBeforeRendering(t *testing.T) {
	o := vfDefaults()
	o.gate = "human"
	root, _ := vfFixture(t, o)
	bf, err := locateAcceptanceBrief(root, "vf/01")
	if err != nil || bf.Path != vfBrief(root) {
		t.Fatalf("lookup returned %+v (%v)", bf, err)
	}
	for _, c := range []struct {
		name, input, want string
	}{
		{"unknown field", `{"ruling":"approve"}`, "unknown field"},
		{"extra document", `{} {}`, "exactly one input document"},
		{"no origin", `{"runs":[],"dispositions":[]}`, "github.com origin"},
	} {
		t.Run(c.name, func(t *testing.T) {
			input := filepath.Join(t.TempDir(), "input.json")
			putFile(t, input, c.input)
			var out, stderr bytes.Buffer
			got := runAcceptanceRoute([]string{"prepare", "--root", root, "--brief", "vf/01", "--input", input}, &out, &stderr)
			if got != 1 || out.Len() != 0 || !strings.Contains(stderr.String(), c.want) {
				t.Fatalf("exit=%d out=%q err=%q", got, &out, &stderr)
			}
		})
	}
}

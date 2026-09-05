package deskkit

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeV2Tree writes a brief-v2 tree (or brief-v1 when v2==false) under a temp
// dir and returns its root.
func writeBriefTree(t *testing.T, v2 bool) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "streams", "svc")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	schema := "brief-v1"
	if v2 {
		schema = "brief-v2"
	}
	brief := "---\nbrief: svc/01\nschema: " + schema + "\n---\n\n# Brief\n"
	if err := os.WriteFile(filepath.Join(dir, "brief-01-a.md"), []byte(brief), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestRefuseIfTreeV2BelowV1(t *testing.T) {
	v2 := writeBriefTree(t, true)
	v1 := writeBriefTree(t, false)

	// Stamped below v1.0.0 (bare) on a v2 tree → refuse with exit 6.
	var errb bytes.Buffer
	if code := RefuseIfTreeV2BelowV1([]string{v2}, "v0.13.0", "deskboard", &errb); code != ExitUnverifiable {
		t.Fatalf("bare v0.13.0 on v2 tree: exit=%d, want %d", code, ExitUnverifiable)
	}
	if !strings.Contains(errb.String(), "tree is brief-v2") || !strings.Contains(errb.String(), "deskboard") {
		t.Errorf("refusal message wrong: %s", errb.String())
	}

	// Namespaced release stamp below v1.0.0 → also refuse (real-release shape).
	if code := RefuseIfTreeV2BelowV1([]string{v2}, "desk-tools/v0.13.0", "deskpr", &bytes.Buffer{}); code != ExitUnverifiable {
		t.Errorf("namespaced desk-tools/v0.13.0 should refuse, exit=%d", code)
	}

	// v1.0.0+ never gated.
	if code := RefuseIfTreeV2BelowV1([]string{v2}, "v1.0.0", "deskboard", &bytes.Buffer{}); code != 0 {
		t.Errorf("v1.0.0 must not be gated, exit=%d", code)
	}
	if code := RefuseIfTreeV2BelowV1([]string{v2}, "desk-tools/v1.2.0", "deskboard", &bytes.Buffer{}); code != 0 {
		t.Errorf("desk-tools/v1.2.0 must not be gated, exit=%d", code)
	}

	// dev / empty (unstamped) treated as latest, never gated.
	if code := RefuseIfTreeV2BelowV1([]string{v2}, "dev", "deskboard", &bytes.Buffer{}); code != 0 {
		t.Errorf("dev build must not be gated, exit=%d", code)
	}
	if code := RefuseIfTreeV2BelowV1([]string{v2}, "", "deskboard", &bytes.Buffer{}); code != 0 {
		t.Errorf("empty version must not be gated, exit=%d", code)
	}

	// v1 tree never gated, even on an old binary.
	if code := RefuseIfTreeV2BelowV1([]string{v1}, "v0.13.0", "deskboard", &bytes.Buffer{}); code != 0 {
		t.Errorf("v1 tree must not be gated, exit=%d", code)
	}
}

func TestRootsFromArgs(t *testing.T) {
	cases := []struct {
		args []string
		want []string
	}{
		{[]string{"actions"}, []string{"."}},
		{[]string{"--root", "/a"}, []string{"/a"}},
		{[]string{"--root", "/a", "--root", "/b"}, []string{"/a", "/b"}},
		{[]string{"--root=/c"}, []string{"/c"}},
	}
	for _, c := range cases {
		got := RootsFromArgs(c.args)
		if strings.Join(got, ",") != strings.Join(c.want, ",") {
			t.Errorf("RootsFromArgs(%v)=%v, want %v", c.args, got, c.want)
		}
	}
}

func TestEffectiveToolVersion(t *testing.T) {
	if got := EffectiveToolVersion("v0.13.0"); got != "v0.13.0" {
		t.Errorf("main.version stamp should win, got %q", got)
	}
	// With no main.version, falls back to ReleaseTagOrDev ("dev" here).
	if got := EffectiveToolVersion(""); got != "dev" {
		t.Errorf("empty main.version with no ReleaseTag should be dev, got %q", got)
	}
}

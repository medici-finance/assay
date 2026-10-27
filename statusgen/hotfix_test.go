package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// regression: #4405
// TestGateParseVersionHotfix: a hotfix is gated exactly as its pin. On a brief-v2 tree a
// `v0.9.0+hotfix-<sha12>` build refuses with exit 6 just as `v0.9.0` does; `dev` still
// proceeds (an unstamped build is latest, unchanged).
func TestGateParseVersionHotfix(t *testing.T) {
	root := migrateFixtureTree(t, true)
	var out, errb bytes.Buffer
	if code := runMigrate([]string{"brief-v1-to-v2", "--root", root}, &out, &errb); code != 0 {
		t.Fatalf("setup migrate failed: %s", errb.String())
	}
	hot := "v0.9.0+hotfix-0123456789ab"
	if v, ok := gateParseVersion(hot); !ok || v != [3]int{0, 9, 0} {
		t.Fatalf("gateParseVersion(%q) = %v, %v; want [0 9 0], true", hot, v, ok)
	}
	var pinErr, hotErr bytes.Buffer
	pinCode := refuseIfTreeTooNew([]string{root}, "v0.9.0", &pinErr)
	hotCode := refuseIfTreeTooNew([]string{root}, hot, &hotErr)
	if pinCode != statusgenExitTreeTooNew || hotCode != pinCode {
		t.Fatalf("v0.9.0 exit %d, hotfix exit %d; want both %d", pinCode, hotCode, statusgenExitTreeTooNew)
	}
	if !strings.Contains(hotErr.String(), "tree is brief-v2") {
		t.Fatalf("hotfix refusal message: %q", hotErr.String())
	}
	if code := refuseIfTreeTooNew([]string{root}, "dev", &bytes.Buffer{}); code != 0 {
		t.Fatalf("dev build gated: exit %d", code)
	}
	if code := refuseIfTreeTooNew([]string{root}, "v1.0.41+hotfix-0123456789ab", &bytes.Buffer{}); code != 0 {
		t.Fatalf("a hotfix over a v1 pin gated: exit %d", code)
	}
	if _, ok := gateParseVersion("v0.9.0+"); ok {
		t.Fatal("an empty build suffix parses")
	}
}

// regression: #4405
// TestBoardProvenanceHotfix: a red produced by a hotfix build names it as a HOTFIX build
// over its pin and still sends the reader back to the pinned release.
func TestBoardProvenanceHotfix(t *testing.T) {
	saved := statusgenVersion
	t.Cleanup(func() { statusgenVersion = saved })
	statusgenVersion = "v1.0.41+hotfix-0123456789ab"
	got := boardProvenanceLine(2)
	for _, want := range []string{"2 PROBLEM(s)", "HOTFIX build v1.0.41+hotfix-0123456789ab over pin v1.0.41", ".assay-versions"} {
		if !strings.Contains(got, want) {
			t.Errorf("hotfix provenance line missing %q\n  got: %s", want, got)
		}
	}
}

// hotfixMatrix is the shared reader fixture (tools/desk/internal/deskkit/testdata/
// hotfix-readers.json); statusgen is its own module, so it reads the file directly.
type hotfixMatrix struct {
	Schema  string            `json:"schema"`
	Pin     string            `json:"pin"`
	Readers map[string]string `json:"readers"`
	Cases   []struct {
		Tag      string            `json:"tag"`
		Verdicts map[string]string `json:"verdicts"`
	} `json:"cases"`
}

// regression: #4405
// TestStatusgenHotfixReaders: statusgen's version gate reaches the shared matrix's verdict
// on every tag — `gate:<base>` when it gates on that base, `latest` when it does not gate.
func TestStatusgenHotfixReaders(t *testing.T) {
	path := os.Getenv("ASSAY_HOTFIX_READERS_MATRIX")
	if path == "" {
		path = filepath.Join("..", "tools", "desk", "internal", "deskkit", "testdata", "hotfix-readers.json")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var m hotfixMatrix
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if m.Schema != "hotfix-readers-v1" {
		t.Fatalf("matrix schema %q", m.Schema)
	}
	if _, ok := m.Readers["statusgen"]; !ok {
		t.Fatal("statusgen is not in the matrix inventory")
	}
	for _, c := range m.Cases {
		got := "latest"
		if v, ok := gateParseVersion(c.Tag); ok {
			got = fmt.Sprintf("gate:v%d.%d.%d", v[0], v[1], v[2])
		}
		want, ok := c.Verdicts["statusgen"]
		if !ok {
			t.Fatalf("case %q has no statusgen verdict", c.Tag)
		}
		if got != want {
			t.Errorf("statusgen reads %q as %s, matrix says %s", c.Tag, got, want)
		}
	}
}

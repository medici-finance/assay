package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/statusgen/briefdoc"
)

func TestDeclaredFilesAuthoringLint(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.work"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	reg := &graphRepos{Aliases: map[string]graphRepoEntry{"app": {}}}
	for _, tc := range []struct {
		name, declaration, want, absent string
	}{
		{"root file", "files: `go.work`", "", "missing root path"},
		{"absent basename", "files:\n- `src/a.go`, `b.go`", "missing root path \"b.go\"", ""},
		{"per-path planned", "files:\n- `a.go` (new), `b.go`", "missing root path \"b.go\"", "missing root path \"a.go\""},
		{"planned inside span", "files: `future.go (planned)`", "", "missing root path"},
		{"planned in description", "files: `a.go` — next edit (planned)", "missing root path \"a.go\"", ""},
		{"grammar refusal", "files:\n- `one/a.go` invalid tail", "line 9: refused:", ""},
		{"alias missing", "files:\n- [other] `src/a.go`", "unknown repository alias `other`", ""},
		{"alias found", "files:\n- [app] `src/a.go`", "", "unknown repository alias"},
		{"fenced alias", "facts:\n```\nfiles: [other] `src/a.go`\n```", "", "unknown repository alias"},
		{"readers disagree", "files: README", "different paths", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := "## Context\n" + tc.declaration + "\n\nfacts: none\n"
			bf := &BriefFile{Body: body, bodyLineOffset: 6}
			bf.DeclaredPaths, _ = extractContextDeclaredPaths(body)
			got := strings.Join(declaredFilesNotices(root, "brief.md", bf, reg, briefdoc.BriefFilesTags(body)), "\n")
			if !strings.Contains(got, tc.want) || tc.absent != "" && strings.Contains(got, tc.absent) {
				t.Fatalf("findings: %q, want %q, exclude %q", got, tc.want, tc.absent)
			}
		})
	}
}

func TestDeclaredAliasRatchet(t *testing.T) {
	notice := func(tag string) string {
		return "[declared-file-alias] " + ratchetBriefA + ": unknown repository alias `" + tag + "` in Context files"
	}
	carried, introduced := notice("old"), notice("other")
	advisory := ratchetBriefA + ": [declared-files] line 8: refused: unsupported declaration"
	unknown := "[declared-file-alias] " + ratchetBriefA + ": COULD-NOT-CHECK repository alias `other` without a readable graph-repos.yaml"
	changed := map[string]string{ratchetBriefA: ratchetBriefA}
	res := runRatchet(t, ratchetCfg([]string{carried}, []string{carried, introduced, advisory, unknown}, changed, nil))
	if len(res.ratchetIntroduced) != 1 || !strings.Contains(res.ratchetIntroduced[0], "`other`") || res.ratchetCarried != 1 {
		t.Fatalf("changed brief: promoted %q, carried %d", res.ratchetIntroduced, res.ratchetCarried)
	}
	res = runRatchet(t, ratchetCfg(nil, []string{introduced}, nil, nil))
	if len(res.ratchetIntroduced) != 0 {
		t.Fatalf("untouched brief promoted: %q", res.ratchetIntroduced)
	}
}

func TestDeclaredFilesReachBriefLint(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/briefschema")); err != nil {
		t.Fatal(err)
	}
	streams, _, err := loadStreams(root)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "docs/streams/alpha/brief-01-valid.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	raw = append(raw, []byte("\n## Context\nfiles:\n- `missing-root.go`\n")...)
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	_, notices := checkBriefFiles(streams, streams)
	line := strings.Count(string(raw), "\n")
	if !hasProblem(notices, "brief-01-valid.md:", fmt.Sprintf("line %d:", line), "missing root path", "missing-root.go") {
		t.Fatal("brief lint did not report the declared file and its source line")
	}
}

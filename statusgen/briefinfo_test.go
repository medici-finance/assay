package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

const briefInfoFixture = "testdata/briefinfo"

// runBriefInfoCapture drives runBriefInfo and returns (exitCode, stdout, stderr).
func runBriefInfoCapture(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	code := runBriefInfo(args, &out, &errb)
	return code, out.String(), errb.String()
}

// TestBriefInfoResolvesFrontmatterAndRow — Verify row 2. The happy-path fields
// (schema, gate, risk, exec-tier, effort) and the board row's status match the
// fixture, and the emitted file path is RELATIVE (no leading "/"), so the output
// carries no machine path.
func TestBriefInfoResolvesFrontmatterAndRow(t *testing.T) {
	code, out, errb := runBriefInfoCapture("--root", briefInfoFixture, "sample/01")
	if code != briefInfoExitOK {
		t.Fatalf("exit = %d, want 0; stderr:\n%s", code, errb)
	}

	var got briefInfo
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("output is not a single JSON object: %v\n%s", err, out)
	}

	if got.Key != "sample/01" {
		t.Errorf("key = %q, want sample/01", got.Key)
	}
	if got.File != "docs/streams/sample/brief-01-first.md" {
		t.Errorf("file = %q, want the RELATIVE brief path", got.File)
	}
	// The pre-mortem "output carries an absolute path" detection lives here.
	if strings.HasPrefix(got.File, "/") {
		t.Errorf("file %q is absolute — output must carry no machine path", got.File)
	}
	if got.Schema != "brief-v1" {
		t.Errorf("schema = %q, want brief-v1", got.Schema)
	}
	if got.Gate != "model" {
		t.Errorf("gate = %q, want model", got.Gate)
	}
	if got.Effort != "M" {
		t.Errorf("effort = %q, want M", got.Effort)
	}
	if got.Wave != 1 {
		t.Errorf("wave = %d, want 1", got.Wave)
	}
	if got.ExecTier != "strong" {
		t.Errorf("exec_tier = %q, want strong", got.ExecTier)
	}
	if got.ExecTierWhy == "" {
		t.Errorf("exec_tier_why is empty, want the fixture rationale")
	}
	for k, want := range map[string]string{
		"regulatory": "no", "customer": "no", "irreversible": "no", "sensitive-data": "no",
	} {
		if got.Risk[k] != want {
			t.Errorf("risk[%s] = %q, want %q", k, got.Risk[k], want)
		}
	}
	if len(got.Depends) != 1 || got.Depends[0] != "sample/00" {
		t.Errorf("depends = %v, want [sample/00]", got.Depends)
	}
	if len(got.Unblocks) != 1 || got.Unblocks[0] != "sample/09" {
		t.Errorf("unblocks = %v, want [sample/09]", got.Unblocks)
	}
	if len(got.Issues) != 1 || got.Issues[0] != 42 {
		t.Errorf("issues = %v, want [42]", got.Issues)
	}
	if got.Row == nil {
		t.Fatalf("row is null, want the board row")
	}
	if got.Row.Status != "implemented" {
		t.Errorf("row.status = %q, want implemented", got.Row.Status)
	}
	if got.Row.Effort != "M" {
		t.Errorf("row.effort = %q, want M", got.Row.Effort)
	}
}

// TestBriefInfoDuplicatePrefixIsAnError — Verify row 3. Two `brief-03-*` files
// make `sample/03` ambiguous: exit 2, both files named on stderr, and NO JSON
// body on stdout.
func TestBriefInfoDuplicatePrefixIsAnError(t *testing.T) {
	code, out, errb := runBriefInfoCapture("--root", briefInfoFixture, "sample/03")
	if code != briefInfoExitResolve {
		t.Fatalf("exit = %d, want 2; stderr:\n%s", code, errb)
	}
	if strings.TrimSpace(out) != "" {
		t.Errorf("stdout must be empty on a resolution failure (no partial JSON); got:\n%s", out)
	}
	if !strings.Contains(errb, "brief-03-alpha.md") || !strings.Contains(errb, "brief-03-beta.md") {
		t.Errorf("stderr must name BOTH colliding files; got:\n%s", errb)
	}
}

// TestBriefInfoLegacyAndMissingRow — Verify row 4. A legacy brief resolves with
// `schema: legacy`; a brief with no README row resolves with `row: null`; both
// exit 0.
func TestBriefInfoLegacyAndMissingRow(t *testing.T) {
	t.Run("legacy", func(t *testing.T) {
		code, out, errb := runBriefInfoCapture("--root", briefInfoFixture, "sample/02")
		if code != briefInfoExitOK {
			t.Fatalf("exit = %d, want 0; stderr:\n%s", code, errb)
		}
		var got briefInfo
		if err := json.Unmarshal([]byte(out), &got); err != nil {
			t.Fatalf("bad JSON: %v\n%s", err, out)
		}
		if got.Schema != "legacy" {
			t.Errorf("schema = %q, want legacy", got.Schema)
		}
		if got.Title != "" || got.Gate != "" || got.Wave != 0 {
			t.Errorf("legacy brief must have empty frontmatter fields; got title=%q gate=%q wave=%d", got.Title, got.Gate, got.Wave)
		}
		// A legacy brief can still carry a board row (the fixture gives sample/02 one).
		if got.Row == nil || got.Row.Status != "done" {
			t.Errorf("legacy brief's row = %+v, want status done", got.Row)
		}
	})

	t.Run("missing row", func(t *testing.T) {
		code, out, errb := runBriefInfoCapture("--root", briefInfoFixture, "sample/04")
		if code != briefInfoExitOK {
			t.Fatalf("exit = %d, want 0; stderr:\n%s", code, errb)
		}
		// `row` must be literally null — never invented as a status.
		if !strings.Contains(out, "\"row\": null") {
			t.Errorf("missing row must render as \"row\": null; got:\n%s", out)
		}
		var got briefInfo
		if err := json.Unmarshal([]byte(out), &got); err != nil {
			t.Fatalf("bad JSON: %v\n%s", err, out)
		}
		if got.Row != nil {
			t.Errorf("row = %+v, want null", got.Row)
		}
		if got.Schema != "brief-v1" {
			t.Errorf("schema = %q, want brief-v1", got.Schema)
		}
	})
}

// TestBriefInfoUnknownKeyIsAnError — an unknown key (no file) and a malformed key
// (bad grammar) both exit 2.
func TestBriefInfoUnknownKeyIsAnError(t *testing.T) {
	t.Run("no file", func(t *testing.T) {
		code, out, _ := runBriefInfoCapture("--root", briefInfoFixture, "sample/99")
		if code != briefInfoExitResolve {
			t.Fatalf("exit = %d, want 2", code)
		}
		if strings.TrimSpace(out) != "" {
			t.Errorf("no JSON body on failure; got:\n%s", out)
		}
	})
	t.Run("bad grammar", func(t *testing.T) {
		code, _, errb := runBriefInfoCapture("--root", briefInfoFixture, "not-a-key")
		if code != briefInfoExitResolve {
			t.Fatalf("exit = %d, want 2; stderr:\n%s", code, errb)
		}
	})
}

// TestBriefInfoMultiKeyPartialFailure — Verify row 5. One bad key among three:
// exit 2, EVERY key's outcome reported, and no partial JSON array on stdout.
func TestBriefInfoMultiKeyPartialFailure(t *testing.T) {
	t.Run("all good — array in argument order", func(t *testing.T) {
		code, out, errb := runBriefInfoCapture("--root", briefInfoFixture, "sample/04", "sample/01", "sample/02")
		if code != briefInfoExitOK {
			t.Fatalf("exit = %d, want 0; stderr:\n%s", code, errb)
		}
		var arr []briefInfo
		if err := json.Unmarshal([]byte(out), &arr); err != nil {
			t.Fatalf("multi-key output must be a JSON array: %v\n%s", err, out)
		}
		if len(arr) != 3 {
			t.Fatalf("array len = %d, want 3", len(arr))
		}
		wantOrder := []string{"sample/04", "sample/01", "sample/02"}
		for i, w := range wantOrder {
			if arr[i].Key != w {
				t.Errorf("array[%d].key = %q, want %q (argument order)", i, arr[i].Key, w)
			}
		}
	})

	t.Run("one bad key fails the whole call", func(t *testing.T) {
		code, out, errb := runBriefInfoCapture("--root", briefInfoFixture, "sample/01", "sample/99", "sample/02")
		if code != briefInfoExitResolve {
			t.Fatalf("exit = %d, want 2; stderr:\n%s", code, errb)
		}
		if strings.TrimSpace(out) != "" {
			t.Errorf("no partial array on failure; got:\n%s", out)
		}
		// Every key's outcome is reported: the two resolvable ones AND the bad one.
		for _, want := range []string{"sample/01", "sample/99", "sample/02"} {
			if !strings.Contains(errb, want) {
				t.Errorf("stderr must report every key's outcome; missing %q in:\n%s", want, errb)
			}
		}
	})
}

// TestBriefInfoTextRendersEveryKey — Verify row 3's `--text` counterpart: --text
// renders every key the JSON carries.
func TestBriefInfoText(t *testing.T) {
	code, out, errb := runBriefInfoCapture("--root", briefInfoFixture, "--text", "sample/01")
	if code != briefInfoExitOK {
		t.Fatalf("exit = %d, want 0; stderr:\n%s", code, errb)
	}
	for _, field := range []string{
		"key: sample/01",
		"file: docs/streams/sample/brief-01-first.md",
		"schema: brief-v1",
		"gate: model",
		"effort: M",
		"exec_tier: strong",
		"risk: ",
		"depends: sample/00",
		"unblocks: sample/09",
		"issues: 42",
		"row.status: implemented",
	} {
		if !strings.Contains(out, field) {
			t.Errorf("--text output missing %q; got:\n%s", field, out)
		}
	}

	// --text on a missing row prints `row: null`, matching the JSON.
	_, out2, _ := runBriefInfoCapture("--root", briefInfoFixture, "--text", "sample/04")
	if !strings.Contains(out2, "row: null") {
		t.Errorf("--text missing-row must print `row: null`; got:\n%s", out2)
	}
}

// TestBriefInfoLeavesFixtureByteIdentical — Verify row 7's byte-identical
// assertion for this subcommand: a resolve run writes nothing under the tree.
func TestBriefInfoLeavesFixtureByteIdentical(t *testing.T) {
	before := hashTree(t, briefInfoFixture)
	if code, _, errb := runBriefInfoCapture("--root", briefInfoFixture, "sample/01", "sample/02", "sample/04"); code != briefInfoExitOK {
		t.Fatalf("exit = %d, want 0; stderr:\n%s", code, errb)
	}
	after := hashTree(t, briefInfoFixture)
	if len(before) != len(after) {
		t.Fatalf("file count changed: before %d, after %d", len(before), len(after))
	}
	for p, h := range before {
		if after[p] != h {
			t.Errorf("fixture file changed: %s", p)
		}
	}
}

func hashTree(t *testing.T, root string) map[string]string {
	t.Helper()
	m := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(b)
		m[rel] = string(sum[:])
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// Deterministic iteration is not required for correctness, but sorting keeps a
	// failure message stable.
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return m
}

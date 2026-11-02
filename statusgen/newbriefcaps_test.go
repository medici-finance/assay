package main

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/statusgen/readercaps"
	"github.com/medici-finance/assay/statusgen/readercaps/record"
	"gopkg.in/yaml.v3"
)

// ms04Snapshot records every file under root, so a refusal can prove it wrote
// nothing at all (no new brief, no README row, no inverse edge).
func ms04Snapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		raw, err := os.ReadFile(p)
		out[p] = string(raw)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func ms04WriteCaps(t *testing.T, name string, v any) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	var raw []byte
	if s, ok := v.(string); ok {
		raw = []byte(s)
	} else {
		var err error
		if raw, err = json.Marshal(v); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(p, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func v4ReaderCaps(provider string) readercaps.Advertisement {
	return readercaps.Advertisement{Schema: readercaps.AdvertisementSchema, Provider: provider, Version: "test",
		Operations: []readercaps.Operation{{Operation: "syntax", Schemas: []string{"brief-v1", "brief-v2", "brief-v3", briefSchemaV4}, Semantics: []string{}}}}
}

// regression: milestone-support/04
func TestNewbriefCaps(t *testing.T) {
	saved := newBriefUUID
	t.Cleanup(func() { newBriefUUID = saved })
	newBriefUUID = func() (string, error) { return "44444444-4444-4444-8444-444444444444", nil }

	supported := ms04WriteCaps(t, "supported.json", v4ReaderCaps("next-reader"))
	wrapped := ms04WriteCaps(t, "wrapped.json", map[string]any{"version": "test", "readerCapabilities": v4ReaderCaps("wrapped-reader")})
	legacy := ms04WriteCaps(t, "legacy.json", legacyReaderCaps())
	malformed := ms04WriteCaps(t, "malformed.json", `{"schema": "reader-capabilities-v1", "provider": "x"`)
	risk := []string{"--regulatory", "no", "--customer", "no", "--irreversible", "no", "--sensitive-data", "no", "--offline"}
	args := func(root string, extra ...string) []string {
		return append(append([]string{"--root", root, "--stream", "demo4", "--title", "Negotiated"}, risk...), extra...)
	}

	refusals := []struct {
		name  string
		extra []string
		want  string
	}{
		{"no reader advertisement", []string{"--schema", "brief-v4"}, "could-not-check"},
		{"tolerant legacy reader", []string{"--schema", "brief-v4", "--reader-caps", legacy}, "unsupported required feature for syntax: brief-v4"},
		{"one unsupported among supported", []string{"--schema", "brief-v4", "--reader-caps", supported, "--reader-caps", legacy}, "unsupported"},
		{"malformed advertisement", []string{"--schema", "brief-v4", "--reader-caps", malformed}, "could-not-check"},
		{"unreadable advertisement", []string{"--schema", "brief-v4", "--reader-caps", filepath.Join(t.TempDir(), "absent.json")}, "could-not-check"},
		{"unselected write into a v4 stream", nil, "select --schema brief-v4"},
		{"milestone gate without brief-v4", []string{"--schema", "legacy", "--requires-milestone", "demo4:m-01@1"}, "never silently drops a requested gate"},
		{"milestone gate with detected schema", []string{"--requires-milestone", "demo4:m-01@1"}, "never silently drops a requested gate"},
		{"unknown schema", []string{"--schema", "brief-v5"}, "invalid --schema"},
		{"milestone in depends", []string{"--schema", "brief-v4", "--reader-caps", supported, "--depends", "demo4:m-01@1"}, "names a milestone"},
		{"qualified depends", []string{"--schema", "brief-v4", "--reader-caps", supported, "--depends", "assay:demo4:b-01"}, "is qualified"},
		{"pinned depends", []string{"--schema", "brief-v4", "--reader-caps", supported, "--depends", "demo4:b-01@2"}, "pins a revision"},
		{"aliased depends twice", []string{"--schema", "brief-v4", "--reader-caps", supported, "--depends", "demo4/01", "--depends", "demo4:b-01"}, "twice"},
		{"unknown milestone root", []string{"--schema", "brief-v4", "--reader-caps", supported, "--requires-milestone", "other:demo4:m-01@1"}, "unknown owning root"},
		{"unversioned milestone", []string{"--schema", "brief-v4", "--reader-caps", supported, "--requires-milestone", "demo4:m-01"}, "--requires-milestone"},
	}
	for _, tc := range refusals {
		t.Run(tc.name, func(t *testing.T) {
			root := ms04Tree(t, nil)
			before := ms04Snapshot(t, root)
			code, _, se := nbRun(t, args(root, tc.extra...)...)
			if code != newBriefExitRefuse {
				t.Fatalf("want exit %d, got %d; stderr=%s", newBriefExitRefuse, code, se)
			}
			if !strings.Contains(se, tc.want) {
				t.Errorf("refusal %q missing %q", se, tc.want)
			}
			after := ms04Snapshot(t, root)
			if len(after) != len(before) {
				t.Errorf("a refusal created files: %d -> %d", len(before), len(after))
			}
			for p, b := range before {
				if after[p] != b {
					t.Errorf("a refusal modified %s", p)
				}
			}
		})
	}

	t.Run("explicit legacy mode writes brief-v2", func(t *testing.T) {
		root := ms04Tree(t, nil)
		code, _, se := nbRun(t, args(root, "--schema", "legacy", "--depends", "demo4/01")...)
		if code != newBriefExitOK {
			t.Fatalf("want exit 0, got %d; stderr=%s", code, se)
		}
		body := ms04Read(t, root, "brief-04-negotiated.md")
		for _, want := range []string{"\nbrief: assay:assay:demo4:04\n", "\nschema: brief-v2\n", "\ndepends: [\"demo4/01\"]\n"} {
			if !strings.Contains(body, want) {
				t.Errorf("legacy body missing %q:\n%s", want, body)
			}
		}
		if strings.Contains(body, "reader-requirements") || strings.Contains(body, ":b-04") {
			t.Errorf("legacy mode must not write brief-v4 keys:\n%s", body)
		}
		if b := ms04Read(t, root, "brief-01-base.md"); !strings.Contains(b, `"demo4/04"`) {
			t.Errorf("legacy dependency inverse must be demo4/04:\n%s", b)
		}
		if problems, _, _ := ms04Check(t, root); len(problems) != 0 {
			t.Errorf("legacy write must lint clean: %v", problems)
		}
	})

	t.Run("negotiated brief-v4 writes id/schema pair and inverses", func(t *testing.T) {
		root := ms04Tree(t, nil)
		code, so, se := nbRun(t, args(root, "--schema", "brief-v4", "--reader-caps", supported, "--reader-caps", wrapped,
			"--depends", "demo4/01", "--depends", "demo4:b-02", "--requires-milestone", "demo4:m-01@1")...)
		if code != newBriefExitOK {
			t.Fatalf("want exit 0, got %d; stderr=%s", code, se)
		}
		body := ms04Read(t, root, "brief-04-negotiated.md")
		for _, want := range []string{
			"\nbrief: assay:assay:demo4:b-04\n", "\nschema: brief-v4\n", "\nversion: 1\n",
			"\nid: 44444444-4444-4444-8444-444444444444\n", "\nwave: 3\n",
			"\ndepends: [\"demo4:b-01\", \"demo4:b-02\"]\n",
			"\nrequires-milestones: [\"demo4:m-01@1\"]\n",
			"\nreader-requirements:\n  schema: reader-requirements-v1\n  required:\n    - operation: syntax\n      schemas: [brief-v4]\n      semantics: []\n",
		} {
			if !strings.Contains(body, want) {
				t.Errorf("brief-v4 body missing %q:\n%s", want, body)
			}
		}
		// Inverses: the legacy dependency gets the legacy-readable id, the
		// brief-v4 dependency the prefixed same-root form.
		if b := ms04Read(t, root, "brief-01-base.md"); !strings.Contains(b, `"demo4/04"`) || strings.Contains(b, "b-04") {
			t.Errorf("legacy dependency inverse must be the legacy id:\n%s", b)
		}
		if b := ms04Read(t, root, "brief-02-prefixed.md"); !strings.Contains(b, "unblocks: [\"demo4:b-04\"]") {
			t.Errorf("brief-v4 dependency inverse must be demo4:b-04:\n%s", b)
		}
		if !strings.Contains(so, "brief assay:assay:demo4:b-04") {
			t.Errorf("stdout must name the typed id: %s", so)
		}
		problems, notices, streams := ms04Check(t, root)
		if len(problems) != 0 {
			t.Errorf("the generated tree must lint clean; problems:\n%s", strings.Join(problems, "\n"))
		}
		if hasProblem(notices, "brief-04-negotiated.md", "one-sided") || hasProblem(notices, "brief-04", "is unknown") {
			t.Errorf("generated edges must be reciprocated and bound; notices:\n%s", strings.Join(notices, "\n"))
		}
		if e := eligibilityForStreams(streams)["demo4/04"]; e.Verdict != VerdictHeld || !hasHold(e, "demo4:m-01@1", StateCouldNotCheck) {
			t.Errorf("the generated brief must be held on its milestone requirement: %+v", e)
		}
		raw := []byte(body)
		schemas, err := parseEmbeddedBriefSchemas()
		if err != nil {
			t.Fatal(err)
		}
		if st, msg := conformFile(filepath.Join(root, "docs/streams/demo4/brief-04-negotiated.md"), schemas); st != conformStateClean {
			t.Errorf("generated brief must conform to brief-v4: %v %s", st, msg)
		}
		// A tolerant YAML decoder reads the record without error — which is
		// exactly why tolerance is never support: the record's own envelope
		// makes a legacy reader refuse it.
		fm, _, err := splitFrontmatter(body)
		if err != nil {
			t.Fatal(err)
		}
		var loose map[string]any
		if err := yaml.Unmarshal([]byte(fm), &loose); err != nil {
			t.Fatalf("tolerant decode: %v", err)
		}
		if r := record.Check(raw, legacyReaderCaps()); r.State != readercaps.Unsupported {
			t.Errorf("a legacy reader must be Unsupported for the generated record: %+v", r)
		}
		if r := record.Check(raw, statusgenReaderCaps()); r.State != readercaps.Supported {
			t.Errorf("statusgen must be Supported for the generated record: %+v", r)
		}
	})
}

func ms04Read(t *testing.T, root, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, "docs", "streams", "demo4", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

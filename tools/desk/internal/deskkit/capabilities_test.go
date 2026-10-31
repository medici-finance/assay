package deskkit

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/statusgen/readercaps"
)

// regression: #4531
func TestDeskCapsPreflight(t *testing.T) {
	for _, target := range []string{"README.md", "brief-01-sample.md"} {
		t.Run(target, func(t *testing.T) {
			root := writeBriefTree(t, false)
			dir := filepath.Join(root, "docs", "streams", "svc")
			file := filepath.Join(dir, target)
			for _, version := range []string{"dev", "v0.13.0", "v999.0.0"} {
				for _, op := range []string{"syntax", "assessment", "dispatch", "acceptance", "future-operation"} {
					raw := []byte("---\nstream: svc\nreader-requirements: {schema: reader-requirements-v1, required: [{operation: " + op + ", schemas: [milestone-v1], semantics: [unknown-v1]}]}\n---\n")
					if err := os.WriteFile(file, raw, 0644); err != nil {
						t.Fatal(err)
					}
					var diag bytes.Buffer
					if code := RefuseIfTreeV2BelowV1([]string{root}, version, "deskboard", &diag); code != ExitRefused || !strings.Contains(diag.String(), "unsupported required") {
						t.Fatalf("%s %s admitted: %d %s", version, op, code, &diag)
					}
					after, err := os.ReadFile(file)
					if err != nil || !bytes.Equal(after, raw) {
						t.Fatalf("preflight mutated source: %v", err)
					}
				}
			}
			if err := os.WriteFile(file, []byte("---\nreader-requirements: {schema: reader-requirements-v1, required: null}\n---\n"), 0644); err != nil {
				t.Fatal(err)
			}
			if code := RefuseIfTreeV2BelowV1([]string{root}, "dev", "deskboard", &bytes.Buffer{}); code != ExitUnverifiable {
				t.Fatalf("malformed became supported: %d", code)
			}
			if err := os.WriteFile(file, []byte("---\nstream: svc\n---\n"), 0644); err != nil {
				t.Fatal(err)
			}
			if code := RefuseIfTreeV2BelowV1([]string{root}, "dev", "deskboard", &bytes.Buffer{}); code != 0 {
				t.Fatalf("legacy behavior changed: %d", code)
			}
		})
	}
	t.Run("decoded-record", func(t *testing.T) {
		root := writeBriefTree(t, false)
		path := filepath.Join(root, "docs", "streams", "svc", "README.md")
		for _, tc := range []struct {
			raw  string
			want int
		}{
			{"---\n\"reader-\\x72equirements\": {schema: \"reader-\\x72equirements-v1\", required: [{operation: dispatch, schemas: [], semantics: [future-v1]}]}\n---\n", ExitRefused},
			{"---\nreader-requirements: {schema: reader-requirements-v1, required: [{operation: syntax, schemas: [milestone-v1], ſchemas: [], semantics: []}]}\n---\n", ExitUnverifiable},
		} {
			if err := os.WriteFile(path, []byte(tc.raw), 0644); err != nil {
				t.Fatal(err)
			}
			if got := RefuseIfTreeV2BelowV1([]string{root}, "dev", "deskboard", &bytes.Buffer{}); got != tc.want {
				t.Fatalf("decoded requirements admitted: got %d want %d", got, tc.want)
			}
		}
	})
	t.Run("advertised-syntax", func(t *testing.T) {
		for _, schema := range []string{"brief-v1", "brief-v2", "brief-v3", "milestone-v1"} {
			required := readercaps.Requirements{Schema: readercaps.RequirementsSchema, Required: []readercaps.Operation{{Operation: "syntax", Schemas: []string{schema}, Semantics: []string{}}}}
			want := readercaps.Supported
			if schema == "milestone-v1" {
				want = readercaps.Unsupported
			}
			if got := readercaps.Assess(required, readercaps.Observation{State: readercaps.Current, Advertisement: ReaderCapabilities("dev", "deskboard")}); got.State != want {
				t.Fatalf("%s: %+v want %s", schema, got, want)
			}
		}
	})

}

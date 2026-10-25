package workref_test

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/statusgen/workref"
)

// regression: #4219
func TestWorkrefGrammarAndAliases(t *testing.T) {
	ctx := &workref.Context{Cell: "example", Repo: "tools"}
	id := workref.Identity{Kind: workref.Brief, Cell: "example", Repo: "tools", Stream: "build", Number: "11"}
	old := id
	old.Stream = "prior"
	roots := []workref.Root{{Cell: "example", Repo: "tools", Aliases: []string{"alias"}}, {Cell: "example", Repo: "other"}, {Cell: "elsewhere", Repo: "tools"}}
	bindings := []workref.Binding{{Identity: id, Key: "stable-key", Aliases: []workref.Identity{old}}}
	reg, err := workref.NewRegistry(roots, bindings)
	if err != nil {
		t.Fatal(err)
	}
	// Mutating host slices after construction cannot change the supplied registry.
	roots[0].Aliases[0] = "changed"
	bindings[0].Aliases[0].Stream = "changed"
	bindings[0].Key = "changed"
	for _, ref := range []string{
		"build/11", "build:11", "tools:build:11", "example:tools:build:11",
		"tools:build/11", "example:tools:build/11", "build:b-11", "tools:build:b-11", "example:tools:build:b-11",
		"build/b-11", "example:tools:build/b-11", "alias:build:11", "prior:11",
	} {
		got, err := workref.Resolve(ref, ctx, reg, workref.TypedBrief)
		if err != nil || got.Identity != id || got.LogicalKey != "stable-key" || got.Revision != 0 || got.Spelling != ref || got.Compatibility != "example:tools:build:11" {
			t.Errorf("%s = %+v, %v", ref, got, err)
		}
	}
	for _, ref := range []string{"build:b-11@2", "example:tools:build/b-11@2", "prior:b-11@2"} {
		got, err := workref.Resolve(ref, ctx, reg, workref.Typed)
		if err != nil || got.Identity != id || got.Revision != 2 || got.LogicalKey != "stable-key" {
			t.Errorf("exact brief revision %s = %+v, %v", ref, got, err)
		}
	}
	for _, ref := range []string{"build:m-03@1", "tools:build:m-03@1", "example:tools:build:m-03@1", "build/m-03@1", "example:tools:build/m-03@1"} {
		got, err := workref.Resolve(ref, ctx, reg, workref.TypedMilestone)
		if err != nil || got.Kind != workref.Milestone || got.Number != "03" || got.Revision != 1 || got.Compatibility != "" {
			t.Errorf("milestone %s = %+v, %v", ref, got, err)
		}
	}
	// Only the final segment determines kind, never a prefix-shaped stream or root.
	for _, ref := range []string{"m-build:11", "b-build:m-03@1", "m-cell:b-repo:m-stream:b-12a"} {
		got, err := workref.Parse(ref, workref.Typed)
		want := workref.Brief
		if ref == "b-build:m-03@1" {
			want = workref.Milestone
		}
		if err != nil || got.Kind != want {
			t.Errorf("final-segment kind %s = %+v, %v", ref, got, err)
		}
	}
	for _, ref := range []string{"", "build:11@1", "build:b-11@0", "build:b-11@-1", "build:b-11@+1", "build:b-11@1.0", "build:b-11@1..2", "build:b-11@1@2", "build:b-11@18446744073709551616", "build:m-18446744073709551616@1", "build:m-00@1", "build:m-3@1", "build:m-03", "build:m-03a@1", "build:b-1", "build:b-12345", "build:x-11", "a:b:c:d:b-11", "build//11", "example/tools:build:11", "tools:build/b-11", "tools:build/m-03@1", "build: 11", "build:b-11\t", "build:\u00a011", "build:\ufeff11", "Build:11", "build:11/extra", "#11"} {
		if got, err := workref.Parse(ref, workref.Typed); err == nil {
			t.Errorf("malformed %q accepted as %+v", ref, got)
		}
	}
	for _, ref := range []string{"build:m-03@1", "build/m-03@1"} {
		if _, err := workref.Resolve(ref, ctx, reg, workref.TypedBrief); err == nil {
			t.Errorf("wrong kind %s accepted", ref)
		}
	}
	if _, err := workref.Parse("build:b-11", workref.TypedMilestone); err == nil {
		t.Error("brief became milestone")
	}
	for _, tc := range []struct {
		ref string
		ctx *workref.Context
	}{
		{"build:11", nil}, {"tools:build:11", nil}, {"unknown:build:11", ctx}, {"unknown:tools:build:11", ctx}, {"build:11", &workref.Context{Cell: "example"}},
	} {
		if _, err := workref.Resolve(tc.ref, tc.ctx, reg, workref.Typed); err == nil {
			t.Errorf("missing context/root %s accepted", tc.ref)
		}
	}
	if _, err := workref.Resolve("example:tools:build:11", nil, reg, workref.Typed); err != nil {
		t.Error(err)
	}
	if got, err := workref.Resolve("elsewhere:tools:build:11", nil, reg, workref.Typed); err != nil || got.Cell != "elsewhere" || got.LogicalKey == "stable-key" {
		t.Errorf("explicit root was searched away: %+v, %v", got, err)
	}
	// Supplied aliases must not merge different stable keys or cross work types.
	other := id
	other.Number = "12"
	for _, b := range [][]workref.Binding{
		{{Identity: id, Key: "one"}, {Identity: id, Key: "two"}},
		{{Identity: id, Key: "same"}, {Identity: other, Key: "same"}},
		{{Identity: id, Key: "one"}, {Identity: other, Key: "two", Aliases: []workref.Identity{id}}},
		{{Identity: id, Key: "one", Aliases: []workref.Identity{{Kind: workref.Milestone, Cell: "example", Repo: "tools", Stream: "build", Number: "03"}}}},
	} {
		if _, err := workref.NewRegistry([]workref.Root{{Cell: "example", Repo: "tools"}}, b); err == nil {
			t.Error("identity namespace collision accepted")
		}
	}
	if _, err := workref.NewRegistry([]workref.Root{{Cell: "example", Repo: "tools", Aliases: []string{"same"}}, {Cell: "example", Repo: "other", Aliases: []string{"same"}}}, nil); err == nil {
		t.Error("ambiguous root accepted")
	}
	// Large selectors retain exact values; this layer never substitutes a known revision.
	got, err := workref.Resolve("build:b-11@18446744073709551615", ctx, reg, workref.Typed)
	if err != nil || got.Revision != ^uint64(0) || strings.Contains(got.String(), "@") {
		t.Errorf("identity/selector conflated: %+v, %v", got, err)
	}
}

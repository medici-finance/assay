package deskkit

import (
	"reflect"
	"testing"
)

// partialdelivery_test.go — #6751: the desk's reading of a brief's `delivery:` list, held to
// statusgen's rules (statusgen/briefdelivery.go). Aliases, repos and PR numbers are synthetic.

var partialAliases = ParseDeliveryAliases([]byte(`schema: graph-repos-v1
cell: example
repos:
  example: {cell: example, repo: example-org/example-repo}
  other:   {cell: example, repo: example-org/other-repo}
  hidden:  {cell: example, repo: null, unpublished: true}
`))

func partialFM(delivery string) string {
	return "---\nbrief: example:example:example-partial:15\ntitle: T\n" + delivery + "schema: brief-v2\n---\n\n# T\n"
}

func TestDeclaredPartialDeliveries(t *testing.T) {
	const repo = "example-org/example-repo"
	cases := []struct {
		name     string
		delivery string
		want     map[int]bool
		unsure   bool
	}{
		{"none", "", nil, false},
		{"one partial", "delivery:\n  - {in: \"example#5127\", covers: partial, note: \"n\"}\n", map[int]bool{5127: true}, false},
		{"two partials", "delivery:\n  - {in: \"example#5127\", covers: partial, note: \"n\"}\n  - {in: \"example#6001\", covers: partial, note: \"n\"}\n", map[int]bool{5127: true, 6001: true}, false},
		{"partial then full", "delivery:\n  - {in: \"example#5127\", covers: partial, note: \"n\"}\n  - {in: \"example#5127\", covers: full}\n", nil, false},
		{"other repo", "delivery:\n  - {in: \"other#5127\", covers: partial, note: \"n\"}\n", nil, false},
		{"withheld alias", "delivery:\n  - {in: \"hidden#5127\", covers: partial, note: \"n\"}\n", nil, true},
		{"unknown alias", "delivery:\n  - {in: \"missing#5127\", covers: partial, note: \"n\"}\n", nil, true},
		{"malformed ref", "delivery:\n  - {in: \"example-5127\", covers: partial, note: \"n\"}\n", nil, true},
		{"not a list", "delivery: example#5127\n", nil, true},
		{"unknown covers ignored", "delivery:\n  - {in: \"missing#1\", covers: some}\n", nil, false},
		// A resolvable alias with an unknown or wrong-case covers word: statusgen counts that PR as
		// delivery, so it must not be set aside (a typo'd `Partial` is not `partial`).
		{"unknown covers on a resolvable alias", "delivery:\n  - {in: \"example#5127\", covers: Partial}\n", nil, false},
	}
	for _, c := range cases {
		got, why := DeclaredPartialDeliveries(partialFM(c.delivery), "example-partial/15", repo, partialAliases)
		if (why != "") != c.unsure {
			t.Errorf("%s: why=%q, want unsure=%v", c.name, why, c.unsure)
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestDeclaredPartialsNoRegistry(t *testing.T) {
	got, why := DeclaredPartialDeliveries(partialFM("delivery:\n  - {in: \"example#1\", covers: partial, note: \"n\"}\n"),
		"example-partial/15", "example-org/example-repo", LoadDeliveryAliases(t.TempDir()))
	if got != nil || why == "" {
		t.Fatalf("no registry is could-not-check, never a set-aside: %v %q", got, why)
	}
}

// TestPartialBriefIDBound: the `delivery:` list is evidence only about the brief the file itself
// names. A different, missing or unparseable frontmatter `brief:` id is could-not-check and sets
// nothing aside; the same id in either the hierarchical or the slash spelling, any case, is honoured.
//
// regression: #6751
func TestPartialBriefIDBound(t *testing.T) {
	const repo = "example-org/example-repo"
	delivery := "delivery:\n  - {in: \"example#5127\", covers: partial, note: \"n\"}\n"
	body := func(idLine string) string {
		return "---\n" + idLine + "title: T\n" + delivery + "schema: brief-v2\n---\n\n# T\n"
	}
	cases := []struct {
		name, idLine, item string
		want               map[int]bool
	}{
		{"same id", "brief: example:example:example-partial:15\n", "example-partial/15", map[int]bool{5127: true}},
		{"same id, slash and case", "brief: Example-Partial/15\n", "example-partial/15", map[int]bool{5127: true}},
		{"different brief", "brief: example:example:example-other:02\n", "example-partial/15", nil},
		{"different number", "brief: example:example:example-partial:16\n", "example-partial/15", nil},
		{"no id", "", "example-partial/15", nil},
		{"unparseable id", "brief: \"not an id\"\n", "example-partial/15", nil},
		{"no item id", "brief: example:example:example-partial:15\n", "", nil},
	}
	for _, c := range cases {
		got, why := DeclaredPartialDeliveries(body(c.idLine), c.item, repo, partialAliases)
		if (why == "") != (c.want != nil) {
			t.Errorf("%s: why=%q, want could-not-check=%v", c.name, why, c.want == nil)
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestDropDeclaredPartials(t *testing.T) {
	list := []RepresentedPR{{Number: 3, Merged: true}, {Number: 2, Merged: false}, {Number: 1, Merged: true}}
	got := DropDeclaredPartials(list, map[int]bool{1: true, 2: true})
	want := []RepresentedPR{{Number: 3, Merged: true}, {Number: 2, Merged: false}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("an open PR is never set aside and order is kept: got %+v, want %+v", got, want)
	}
}

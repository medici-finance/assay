package main

import "testing"

func TestSharedFrontmatter(t *testing.T) {
	fm := parseFrontmatter("---\nrisk: {regulatory: \"yes\"}\ngate: human\n---\n")
	if !fm.Risk.Regulatory || fm.ParseError != "" {
		t.Fatal(fm)
	}
	bad := parseFrontmatter("---\nrisk: [yes]\n---\n")
	if bad.ParseError == "" || bad.Gate != "human" || !bad.Risk.Regulatory {
		t.Fatal("unreadable input was treated as safe", bad)
	}
	for _, misplaced := range []string{"risks:\n  regulatory: yes\n", "meta:\n  customer: yes\n", "\u200bcustomer: yes\n", "\ufeff--- {customer: yes}\n", "\u2800customer: yes\n", "\u0301customer: yes\n", "\u00a0--- {customer: yes}\n", "cu\u017ftomer: yes\n", "notes: |\n  customer: yes\n"} {
		fm := parseFrontmatter("---\ngate: model\nrisk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\n" + misplaced + "---\n")
		if fm.ParseError == "" || fm.Gate != "human" || !fm.Risk.Regulatory || !fm.Risk.Customer {
			t.Fatalf("misplaced risk answer %q read as safe: %+v", misplaced, fm)
		}
	}
	for _, second := range []string{"...\nregulatory: yes\n", "--- {customer: yes}\n", "...\ngate: human\n"} {
		fm := parseFrontmatter("---\ngate: model\nrisk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\n" + second + "---\n")
		if fm.ParseError == "" || fm.Gate != "human" || !fm.Risk.Regulatory || !fm.Risk.Customer {
			t.Errorf("second YAML document %q read as safe: %+v", second, fm)
		}
	}
	for _, hidden := range continuationLines {
		fm := parseFrontmatter("---\n" + hidden + "\n---\n")
		if fm.ParseError == "" || fm.Gate != "human" || fm.ExecTier != "strong" {
			t.Errorf("continuation line %q read as %+v", hidden, fm)
		}
	}
}

// continuationLines hide a column-0 policy line inside a multi-line value
// opened on an earlier line: the shared parser reads the line as text in that
// value, while a first-match line reader reads it as the key.
var continuationLines = []string{
	"meta: {a: 1,\ngate: human\n}\ngate: model",
	"meta: {a:1,\ngate: human\n}",
	"tags: [a,\ngate: human\n]\ngate: model",
	"tags: [a,\nexec-tier: strong]\nexec-tier: any",
	"title: \"first\ngate: human\n  end\"\ngate: model",
	"title: 'first\nexec-tier: strong\n  end'",
	"meta: {a: 1,\ndecision-issue: 12\n}",
}

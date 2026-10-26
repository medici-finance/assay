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
	for _, misplaced := range []string{"risks:\n  regulatory: yes\n", "meta:\n  customer: yes\n", "\u200bcustomer: yes\n", "\ufeff--- {customer: yes}\n"} {
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
}

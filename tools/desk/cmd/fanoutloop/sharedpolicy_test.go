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
}

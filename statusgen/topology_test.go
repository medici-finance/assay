package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// vgBrief01 is the verifygate fixture's brief-01 path under root.
func vgBrief01(root string) string {
	return filepath.Join(root, "docs", "streams", "vg", "brief-01-human-verified.md")
}

// plantVGRefs rewrites vg/01's sources line to carry src (a YAML flow list body), adds a
// consumers list, and appends ev to its Evidence section.
func plantVGRefs(t *testing.T, root, src, cons, ev string) {
	t.Helper()
	p := vgBrief01(root)
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	old := `sources: ["fixture: human-gated, verified, eligible"]`
	if !strings.Contains(s, old) {
		t.Fatal("fixture drift: vg/01 sources line not found")
	}
	repl := "sources: [" + src + "]"
	if cons != "" {
		repl += "\nconsumers: [" + cons + "]"
	}
	s = strings.Replace(s, old, repl, 1)
	s = strings.Replace(s, "\n## Review\n", "\n"+ev+"\n\n## Review\n", 1)
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

func runLint(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := runLintNamed(args, &out, &errb)
	return code, out.String(), errb.String()
}

// TestCitationAliasCheck pins `statusgen lint --check citation-alias-resolution`: every aliased
// ref in sources/consumers/Evidence resolves through the registry, an unregistered alias is a
// PROBLEM naming it, a tree with no registry cannot vouch for any alias, and in-repo refs never
// need the registry.
func TestCitationAliasCheck(t *testing.T) {
	const chk = "citation-alias-resolution"

	t.Run("clean: registered aliases resolve", func(t *testing.T) {
		root, _ := loadVGStreams(t)
		writeVGRegistry(t, root)
		plantVGRefs(t, root, `"app:vg/02", "other#12"`, `"example:other:ab/03"`,
			"Cited by app:vg/03 and other#7; in-repo vg/04 and #9 need no registry.")
		code, out, errs := runLint(t, "--root", root, "--check", chk)
		if code != 0 {
			t.Fatalf("exit %d, want 0\nstdout: %s\nstderr: %s", code, out, errs)
		}
		if !strings.Contains(out, "5 aliased ref(s)") {
			t.Errorf("summary must count the 5 aliased refs; got %q", out)
		}
	})

	cases := []struct{ name, src, cons, ev, alias, field string }{
		{"sources", `"ghost:vg/02"`, "", "", "ghost", "sources"},
		{"consumers", `"app:vg/02"`, `"ghost:ab/03"`, "", "ghost", "consumers"},
		{"evidence issue", `"app:vg/02"`, "", "Closes ghost#44.", "ghost", "Evidence"},
		{"evidence cross-cell", `"app:vg/02"`, "", "See example:ghost:ab/05.", "ghost", "Evidence"},
	}
	for _, tc := range cases {
		t.Run("unknown alias in "+tc.name, func(t *testing.T) {
			root, _ := loadVGStreams(t)
			writeVGRegistry(t, root)
			plantVGRefs(t, root, tc.src, tc.cons, tc.ev)
			code, _, errs := runLint(t, "--root", root, "--check", chk)
			if code != 1 {
				t.Fatalf("exit %d, want 1 (unregistered alias)\nstderr: %s", code, errs)
			}
			if !strings.Contains(errs, `alias "`+tc.alias+`"`) || !strings.Contains(errs, tc.field+" ref") {
				t.Errorf("PROBLEM must name the alias %q and the field %q; got:\n%s", tc.alias, tc.field, errs)
			}
		})
	}

	t.Run("no registry: aliased ref is a problem", func(t *testing.T) {
		root, _ := loadVGStreams(t)
		plantVGRefs(t, root, `"app:vg/02"`, "", "")
		code, _, errs := runLint(t, "--root", root, "--check", chk)
		if code != 1 || !strings.Contains(errs, "no docs/streams/graph-repos.yaml") {
			t.Fatalf("exit %d, want 1 naming the missing registry; stderr:\n%s", code, errs)
		}
	})

	t.Run("no registry, in-repo refs only: clean", func(t *testing.T) {
		root, _ := loadVGStreams(t)
		plantVGRefs(t, root, `"vg/02"`, "", "Follows vg/03 and #9.")
		if code, out, errs := runLint(t, "--root", root, "--check", chk); code != 0 {
			t.Fatalf("exit %d, want 0\nstdout: %s\nstderr: %s", code, out, errs)
		}
	})

	t.Run("unparseable brief: could-not-check", func(t *testing.T) {
		root, _ := loadVGStreams(t)
		writeVGRegistry(t, root)
		p := vgBrief01(root)
		b, _ := os.ReadFile(p)
		broken := strings.Replace(string(b), "schema: brief-v1\n", "schema: brief-v1\nsources: [unterminated\n", 1)
		broken = strings.Replace(broken, `sources: ["fixture: human-gated, verified, eligible"]`+"\n", "", 1)
		if err := os.WriteFile(p, []byte(broken), 0o644); err != nil {
			t.Fatal(err)
		}
		code, _, errs := runLint(t, "--root", root, "--check", chk)
		if code != 6 || !strings.Contains(errs, "UNCHECKED:") {
			t.Fatalf("exit %d, want 6 with an UNCHECKED line; stderr:\n%s", code, errs)
		}
	})
}

// TestLintNamedUsage: `statusgen lint` runs named checks only, and a name it does not know is a
// usage error (exit 2), never a pass — so an acceptance command naming a check that does not
// exist yet fails loudly.
func TestLintNamedUsage(t *testing.T) {
	root, _ := loadVGStreams(t)
	for _, args := range [][]string{
		{"--root", root},
		{"--root", root, "--check", "deliverable-repo-declared"},
		{"--root", root, "--check", "citation-alias-resolution", "--check", "bogus"},
		{"--root", root, "--check", "citation-alias-resolution", "stray"},
	} {
		if code, _, errs := runLint(t, args...); code != 2 {
			t.Errorf("lint %v: exit %d, want 2; stderr: %s", args, code, errs)
		}
	}
}

func runVGC(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := runVerifyGateClose(args, &out, &errb)
	return code, out.String(), errb.String()
}

// TestVerifyGateCloseVerb pins `statusgen verify-gate-close --ref`: exit 5 for an alias the
// registry refuses (unknown, or another repo), 6 when there is no registry to ask, 0 with nothing
// written under --dry-run, and a real flip for this tree's own alias.
func TestVerifyGateCloseVerb(t *testing.T) {
	readme := func(root string) string {
		b, err := os.ReadFile(filepath.Join(root, "docs", "streams", "vg", "README.md"))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}

	for _, tc := range []struct {
		ref, want string
		reg       bool
		code      int
	}{
		{"stray:vg/01", `"stray"`, true, 5},
		{"other:vg/01", `this tree is alias "app"`, true, 5},
		{"example:other:vg:01", "repo unpublished in this registry", true, 5},
		{"app:vg/01", "carries no docs/streams/graph-repos.yaml", false, 6},
		{"vg/04", "verified", true, 1},
	} {
		t.Run(tc.ref, func(t *testing.T) {
			root, _ := loadVGStreams(t)
			if tc.reg {
				writeVGRegistry(t, root)
			}
			before := readme(root)
			code, _, errs := runVGC(t, "--root", root, "--ref", tc.ref)
			if code != tc.code || !strings.Contains(errs, tc.want) {
				t.Fatalf("exit %d, want %d mentioning %q; stderr: %s", code, tc.code, tc.want, errs)
			}
			if readme(root) != before {
				t.Error("a refused close must write nothing")
			}
		})
	}

	t.Run("dry-run own alias", func(t *testing.T) {
		root, _ := loadVGStreams(t)
		writeVGRegistry(t, root)
		before := readme(root)
		code, out, errs := runVGC(t, "--root", root, "--ref", "example:app:vg:01", "--dry-run")
		if code != 0 || !strings.Contains(out, "resolved example:app:vg:01 -> vg/01") || !strings.Contains(out, "would mark vg/01 done") {
			t.Fatalf("exit %d; stdout: %s; stderr: %s", code, out, errs)
		}
		if readme(root) != before {
			t.Error("--dry-run must write nothing")
		}
	})

	t.Run("flips own alias", func(t *testing.T) {
		root, _ := loadVGStreams(t)
		writeVGRegistry(t, root)
		if code, out, errs := runVGC(t, "--root", root, "--ref", "app:vg/01"); code != 0 {
			t.Fatalf("exit %d; stdout: %s; stderr: %s", code, out, errs)
		}
		if got := vgRowStatus(t, root, "01"); got != "done" {
			t.Errorf("vg/01 = %q, want done", got)
		}
	})

	t.Run("usage", func(t *testing.T) {
		root, _ := loadVGStreams(t)
		for _, args := range [][]string{{"--root", root}, {"--root", root, "vg/01"}} {
			if code, _, _ := runVGC(t, args...); code != 2 {
				t.Errorf("%v: exit %d, want 2", args, code)
			}
		}
	})
}

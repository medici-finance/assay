package main

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// A self-route is `follow-up <stream>/<NN>` naming the brief that carries the
// entry. Its target always exists and always references back, so before #4560
// it corroborated from the brief text alone: the same summary at the real
// delivery, at an unrelated delivery and at the pre-delivery base.

// selfRouteRun pins a delivery and returns the exit code and the summary line.
func selfRouteRun(t *testing.T, root, base, delivery string) (int, string) {
	t.Helper()
	var code int
	var out string
	_ = captureStderr(t, func() {
		out = captureStdout(t, func() { code = runDeliveryConsumers(root, base, "alpha/01", delivery) })
	})
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "summary: ") {
			return code, strings.TrimSuffix(strings.TrimPrefix(line, "summary: "), ", 0 brief(s) claiming nothing")
		}
	}
	return code, "(no summary)\n" + out
}

// selfRouteHistory builds: initial tree, a pre-delivery anchor, the real
// delivery (touches web/fixed.go), then an unrelated change (touches only
// web/excluded.go). It returns the anchor, the real delivery and the unrelated one.
func selfRouteHistory(t *testing.T, routes []string) (root, anchor, real, unrelated string) {
	t.Helper()
	root = consumersFixture(t, routes, map[string]string{"web/fixed.go": "before\n", "web/untouched.go": "before\n", "web/excluded.go": "before\n"})
	mustGitInit(t, root)
	mustWrite(t, filepath.Join(root, "notes.txt"), "the commit before the delivery\n")
	mustGitCommitAll(t, root, "pre-delivery anchor")
	anchor = mustGitOut(t, root, "rev-parse", "HEAD")
	mustWrite(t, filepath.Join(root, "web/fixed.go"), "delivered\n")
	mustGitCommitAll(t, root, "the brief's delivery")
	real = mustGitOut(t, root, "rev-parse", "HEAD")
	mustWrite(t, filepath.Join(root, "web/excluded.go"), "someone else's change\n")
	mustGitCommitAll(t, root, "an unrelated delivery")
	unrelated = mustGitOut(t, root, "rev-parse", "HEAD")
	return root, anchor, real, unrelated
}

// regression: #4560
func TestSelfRouteNeedsDelivery(t *testing.T) {
	root, anchor, real, unrelated := selfRouteHistory(t, []string{"web/fixed.go: follow-up alpha/01", "web/excluded.go: out-of-scope (unchanged reader contract)"})
	for _, tc := range []struct {
		name, base, delivery string
		code                 int
		summary              string
	}{
		{"real delivery", anchor, real, 0, "1 corroborated, 0 disproved, 1 unchecked"},
		// The unrelated change edits the out-of-scope site, so it also disproves.
		{"unrelated delivery", real, unrelated, 1, "0 corroborated, 1 disproved, 1 unchecked"},
		{"pre-delivery base", mustGitOut(t, root, "rev-parse", anchor+"^1"), anchor, 2, "0 corroborated, 0 disproved, 2 unchecked"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, summary := selfRouteRun(t, root, tc.base, tc.delivery)
			if code != tc.code || summary != tc.summary {
				t.Fatalf("got exit %d %q, want exit %d %q", code, summary, tc.code, tc.summary)
			}
		})
	}
}

// regression: #4560
func TestSelfRouteOnlyBrief(t *testing.T) {
	// The desktools-v2 shape: every entry is a self-route, nothing out-of-scope.
	// Only the real delivery corroborates; the others corroborate nothing and
	// fail COULD-NOT-CHECK rather than pass.
	root, anchor, real, unrelated := selfRouteHistory(t, []string{"web/fixed.go: follow-up alpha/01 (the change)", "web/untouched.go: follow-up alpha/01 (kept as is)"})
	for _, tc := range []struct {
		name, base, delivery string
		code                 int
		summary              string
	}{
		{"real delivery", anchor, real, 0, "1 corroborated, 0 disproved, 1 unchecked"},
		{"unrelated delivery", real, unrelated, 2, "0 corroborated, 0 disproved, 2 unchecked"},
		{"pre-delivery base", mustGitOut(t, root, "rev-parse", anchor+"^1"), anchor, 2, "0 corroborated, 0 disproved, 2 unchecked"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, summary := selfRouteRun(t, root, tc.base, tc.delivery)
			if code != tc.code || summary != tc.summary {
				t.Fatalf("got exit %d %q, want exit %d %q", code, summary, tc.code, tc.summary)
			}
		})
	}
}

// regression: #4560
func TestSelfRouteSiteShapes(t *testing.T) {
	// Every site shape the diff cannot fully show comes out UNCHECKED, never
	// CORROBORATED; a self-route is never DISPROVED by an untouched site. A
	// follow-up to ANOTHER brief keeps its rule: existence plus reference-back.
	C, U := stateCorroborated, stateUnchecked
	for _, tc := range []struct {
		route  string
		states []string
	}{
		{"web/fixed.go: follow-up alpha/01", []string{C}},
		{"web/: follow-up alpha/01", []string{C}},                                    // directory, a file under it changed
		{"web/{fixed,untouched}.go: follow-up alpha/01", []string{U}},                // a set, only part of it touched
		{"web/untouched.go: follow-up alpha/01", []string{U}},                        // untouched
		{"web/not-yet-created.go: follow-up alpha/01", []string{U}},                  // declared, not created
		{"web/gone.go: follow-up alpha/01", []string{C}},                             // deleted by the diff
		{"the reader of the web API: follow-up alpha/01", []string{U}},               // prose site
		{"[other-repo] web/fixed.go: follow-up alpha/01", []string{U}},               // foreign site
		{"web/untouched.go: follow-up alpha/02", []string{C}},                        // not a self-route
		{"web/untouched.go: follow-up alpha/02, follow-up alpha/01", []string{C, U}}, // per target
	} {
		t.Run(tc.route, func(t *testing.T) {
			root := consumersFixture(t, []string{tc.route}, map[string]string{"web/fixed.go": "x\n", "web/untouched.go": "x\n"})
			target := filepath.Join(root, "docs", "streams", "alpha", "brief-02-target.md")
			mustWrite(t, target, readFileT(t, target)+"\nPicks up the web readers deferred by alpha/01.\n")
			streams, _, err := loadStreams(root)
			if err != nil {
				t.Fatal(err)
			}
			bf, ok, err := parseBriefFile(filepath.Join(root, "docs", "streams", "alpha", "brief-01-claims.md"))
			if err != nil || !ok {
				t.Fatalf("fixture must parse: %v", err)
			}
			verdicts := corroborateBrief(root, streams, map[string]bool{"web/fixed.go": true, "web/gone.go": true}, nil, bf)
			var got []string
			for _, v := range verdicts {
				got = append(got, v.State)
			}
			if strings.Join(got, ",") != strings.Join(tc.states, ",") {
				t.Fatalf("%q: got %v, want %v: %+v", tc.route, got, tc.states, verdicts)
			}
		})
	}
}

// regression: #4560
func TestSelfRouteSpelling(t *testing.T) {
	// A brief-v2 id must not dodge the self-route check by spelling.
	for _, bf := range []*BriefFile{
		{Brief: "alpha/01", Path: "docs/streams/alpha/brief-01-claims.md"},
		{Brief: "cell:repo:alpha:01", Path: "docs/streams/alpha/brief-01-claims.md"},
		{Brief: "cell:repo:alpha:01", Path: "elsewhere/notabrief.md"},
	} {
		if !isSelfRoute("alpha/01", bf) {
			t.Fatalf("%+v: alpha/01 is a self-route", bf)
		}
		if isSelfRoute("alpha/02", bf) || isSelfRoute("beta/01", bf) {
			t.Fatalf("%+v: a different brief is not a self-route", bf)
		}
	}
}

// regression: #4560
func TestSelfRouteRealHistory(t *testing.T) {
	// The two briefs named on #4560, pinned to their real deliveries, to an
	// unrelated delivery and to the pre-delivery base. Before #4560 all three
	// printed 7/0 (desktools-v2/23) and 13/0 (desktools-v2/47).
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	const unrelatedDelivery = "d7c556ca374934c58f0eda35e079a41ee2d3bd69"
	cases := []struct{ brief, base, delivery, want string }{
		{"desktools-v2/23", "90da708cb3a192d0fd32b7b85185c0c3fead5392", "3f15da841717fa597b7473eb8c0efbbd46e594c3", "3 corroborated, 0 disproved, 4 unchecked"},
		{"desktools-v2/47", "eeaa65d08810d6b9db1422b9ffb2f4bdc779cb8b", "0ce631327963a306078f6965a595685c8944b1e4", "5 corroborated, 0 disproved, 8 unchecked"},
	}
	need := []string{unrelatedDelivery + "^1", unrelatedDelivery}
	for _, c := range cases {
		need = append(need, c.base+"^1", c.base, c.delivery)
	}
	if out, err := exec.Command("git", "-C", root, "rev-parse", "--is-shallow-repository").Output(); err != nil || strings.TrimSpace(string(out)) != "false" {
		t.Skipf("needs this repository's full history (shallow or no git: %v %s)", err, out)
	}
	for _, rev := range need {
		if err := exec.Command("git", "-C", root, "cat-file", "-e", rev+"^{commit}").Run(); err != nil {
			t.Skipf("needs this repository's history: %s is not present", rev)
		}
	}
	unrelatedBase := mustGitOut(t, root, "rev-parse", unrelatedDelivery+"^1")
	for _, c := range cases {
		run := func(base, delivery string) (int, string) {
			var code int
			var out string
			_ = captureStderr(t, func() {
				out = captureStdout(t, func() { code = runDeliveryConsumers(root, base, c.brief, delivery) })
			})
			for _, line := range strings.Split(out, "\n") {
				if strings.HasPrefix(line, "summary: ") {
					return code, strings.TrimSuffix(strings.TrimPrefix(line, "summary: "), ", 0 brief(s) claiming nothing")
				}
			}
			return code, ""
		}
		t.Run(c.brief, func(t *testing.T) {
			if code, got := run(c.base, c.delivery); code != 0 || got != c.want {
				t.Fatalf("real delivery: exit %d %q, want exit 0 %q", code, got, c.want)
			}
			if code, got := run(unrelatedBase, unrelatedDelivery); code == 0 || strings.HasPrefix(got, strings.SplitN(c.want, ",", 2)[0]) {
				t.Fatalf("unrelated delivery must fail: exit %d %q", code, got)
			}
			if code, got := run(mustGitOut(t, root, "rev-parse", c.base+"^1"), c.base); code == 0 {
				t.Fatalf("pre-delivery base must fail: exit %d %q", code, got)
			}
		})
	}
}

package evaluation

import (
	"reflect"
	"strings"
	"testing"
)

// witness_review_test.go — the review-round guards for the v2 reader
// (statusgen/36): the `inputs:` bookkeeping filter, the option-1 network-tool
// reading through wrappers, and the order-independent tree digest.

// regression: #4409 review F2 — a declared token that is a prefix of
// `docs/streams/` (docs/, docs, d) is a real declared path, never bookkeeping.
func TestWitnessInputsKeepsDocsPrefix(t *testing.T) {
	cases := []struct {
		name, ctx string
		want      []string
		problem   bool
	}{
		{"docs/ and statusgen/", "inputs: `docs/` `statusgen/`", []string{"docs/", "statusgen/"}, false},
		{"docs without slash", "inputs: `docs` `statusgen/x.go`", []string{"docs", "statusgen/x.go"}, false},
		{"single letter d", "inputs: d statusgen/x.go", []string{"d", "statusgen/x.go"}, false},
		{"docs/s", "inputs: docs/s statusgen/x.go", []string{"docs/s", "statusgen/x.go"}, false},
		{"docs/streams/ stays bookkeeping", "inputs: `docs/streams/` `statusgen/`", []string{"statusgen/"}, true},
		{"docs/streams/x stays bookkeeping", "inputs: `docs/streams/x/y.md` `statusgen/`", []string{"statusgen/"}, true},
		{"STATUS.md stays bookkeeping", "inputs: `STATUS.md` `statusgen/`", []string{"statusgen/"}, true},
	}
	for _, tc := range cases {
		got, found, problem := ParseInputsDeclaration(tc.ctx)
		if !found || !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: entries %v found=%v, want %v", tc.name, got, found, tc.want)
		}
		if (problem != "") != tc.problem {
			t.Errorf("%s: problem %q, want problem=%v", tc.name, problem, tc.problem)
		}
	}
}

// regression: #4409 review F3 — option 1 reads through wrappers, `sh -c`
// strings and find -exec; an opaque runner is unreadable, never "no network".
func TestWitnessNetworkWrappers(t *testing.T) {
	cases := []struct {
		cmd               string
		network, readable bool
	}{
		// the ruled shapes: not a network tool, readable
		{"go test ./pkg/", false, true},
		{"cat tools/x/a.go", false, true},
		{"bash tools/check.sh", false, true},
		{"env FOO=bar go test ./pkg/", false, true},
		{"timeout 30 go test ./pkg/", false, true},
		{"go run ./cmd/tool", false, true},
		{"bash -c 'go test ./pkg/'", false, true},
		{"find . -name '*.go' -exec cat {} +", false, false}, // braces: the lexer refuses
		// first-word shapes that always held
		{"curl -s x", true, true},
		{"git fetch origin", true, true},
		{"go install ./cmd/x", true, true},
		{"cat http://example.invalid/x", true, true},
		// wrappers
		{"env curl x", true, true},
		{"env -i FOO=bar curl x", true, true},
		{"env -u HOME wget x", true, true},
		{"timeout 30 curl -s x", true, true},
		{"timeout -k 5 30 curl x", true, true},
		{"nice -n 5 curl x", true, true},
		{"nohup curl x", true, true},
		{"sudo wget x", true, true},
		{"sudo -u root curl x", true, true},
		{"command curl x", true, true},
		{"exec curl x", true, true},
		{"time curl x", true, true},
		{"xargs curl", true, true},
		{"xargs -n 1 wget", true, true},
		{"FOO=bar curl x", true, true},
		{"env timeout 30 sudo curl x", true, true},
		{"cat a.go | xargs curl", true, true},
		{"go test ./pkg/ && env curl x", true, true},
		// shell strings
		{"bash -c 'curl x'", true, true},
		{"sh -c \"wget x\"", true, true},
		{"bash -lc 'curl x'", true, true},
		{"sudo bash -c 'curl x'", true, true},
		{"bash -c 'sh -c \"curl x\"'", true, true},
		{"bash -c", false, false},
		// networked go commands
		{"go run example.invalid/mod/cmd/tool@latest", true, true},
		{"go run -mod=mod example.invalid/m@v1.2.3", true, true},
		{"go get example.invalid/m", true, true},
		{"go mod download", true, true},
		{"go mod tidy", true, true},
		// regression: #4409 review SEC-3 — env short flags bundled before S
		// (or before a value flag) are read the way env reads them; an
		// unknown flag in a bundle is unreadable, never "no network".
		{"env -iS 'curl x'", true, true},
		{"env -ivS'wget x'", true, true},
		{"env -0vS curl", true, true},
		{"env -iS 'go test' ./pkg/", false, true},
		{"env -iu HOME curl x", true, true},
		{"env -iu HOME go test ./pkg/", false, true},
		{"env -iC tools curl x", true, true},
		{"env -iv go test ./pkg/", false, true},
		{"env -x curl x", false, false},
		{"env -ix go test ./pkg/", false, false},
		// regression: #4409 review F13 — split strings and go -C
		{"env -S 'curl x'", true, true},
		{"env -S'curl x'", true, true},
		{"env --split-string='wget x'", true, true},
		{"env -S 'go test' ./pkg/", false, true},
		{"env -S 'make' test", false, false},
		{"go -C tools/x get example.invalid/m", true, true},
		{"go -C tools/x test ./pkg/", false, true},
		// find -exec
		{"find tools -name x -exec curl {} ;", false, false}, // braces: the lexer refuses
		{"find tools -name x -exec curl y ;", true, true},
		// opaque runners
		{"make fetch", false, false},
		{"make test", false, false},
		{"just ci", false, false},
		{"env FOO=1 make", false, false},
		{"eval run", false, false},
		{"timeout 30 make test", false, false},
		{"bash -c 'make test'", false, false},
	}
	for _, tc := range cases {
		network, readable := commandNamesNetworkTool(tc.cmd, "")
		if network != tc.network || readable != tc.readable {
			t.Errorf("%q: network=%v readable=%v, want network=%v readable=%v", tc.cmd, network, readable, tc.network, tc.readable)
		}
	}
	// A shell other than sh is unreadable whatever it says.
	if n, r := commandNamesNetworkTool("echo hi", "bash"); n || r {
		t.Errorf("a bash-shell row is unreadable, got network=%v readable=%v", n, r)
	}
}

// regression: #4409 review F6 — the tree digest does not depend on input
// order, including for paths holding a space and sharing a suffix.
func TestWitnessTreeDigestOrder(t *testing.T) {
	a := TreeBlob{Mode: "100644", OID: "1111111111111111111111111111111111111111", Path: "a x"}
	b := TreeBlob{Mode: "100644", OID: "2222222222222222222222222222222222222222", Path: "b x"}
	c := TreeBlob{Mode: "100755", OID: "3333333333333333333333333333333333333333", Path: "c/d"}
	orders := [][]TreeBlob{{a, b, c}, {b, a, c}, {c, b, a}, {b, c, a}}
	want := TreeDigest(orders[0])
	for i, o := range orders {
		if got := TreeDigest(o); got != want {
			t.Errorf("order %d: digest %s != %s", i, got, want)
		}
	}
	if TreeDigest([]TreeBlob{a, b}) == TreeDigest([]TreeBlob{a, c}) {
		t.Error("different listings digest equal")
	}
	// Untracked worktree entries come out in path order, not map order.
	wt := map[string]string{"u/3": "a", "u/1": "b", "u/2": "c", "u/0": "d"}
	var first []TreeBlob
	for i := 0; i < 20; i++ {
		l, _ := WorktreeListing([]TreeBlob{a}, wt)
		if i == 0 {
			first = l
		} else if !reflect.DeepEqual(first, l) {
			t.Fatalf("worktree listing order varies: %v vs %v", first, l)
		}
	}
}

// regression: #4409 review SEC-1 — a commit object's parents are its
// top-level `parent` headers; its message is everything after the first
// blank line.
func TestWitnessParseCommitObject(t *testing.T) {
	tree, p1, p2 := strings.Repeat("1", 40), strings.Repeat("2", 40), strings.Repeat("3", 40)
	who := "author A <a@example.test> 1700000000 +0000\ncommitter A <a@example.test> 1700000000 +0000\n"
	obj := "tree " + tree + "\nparent " + p1 + "\nparent " + p2 + "\n" + who +
		"gpgsig -----BEGIN SIGNATURE-----\n parent " + strings.Repeat("4", 40) + "\n -----END SIGNATURE-----\n" +
		"\nMerge pull request #12 from example-org/feat\n\nparent " + strings.Repeat("5", 40) + "\n"
	parents, msg, ok := parseCommitObject([]byte(obj))
	if !ok || len(parents) != 2 || parents[0] != p1 || parents[1] != p2 {
		t.Fatalf("parents %v ok=%v, want [%s %s]", parents, ok, p1, p2)
	}
	if !strings.HasPrefix(msg, "Merge pull request #12 from example-org/feat\n") || !strings.Contains(msg, "parent "+strings.Repeat("5", 40)) {
		t.Errorf("message %q", msg)
	}
	// regression: #4409 review SEC-2 — git reads parents only in the run of
	// `parent` headers directly after `tree`; a `parent` header after another
	// header, or a continuation line right after the parents, is not a parent.
	for name, obj := range map[string]string{
		"a parent header after the author": "tree " + tree + "\nparent " + p1 + "\n" + who + "parent " + p2 + "\n\nMerge pull request #12 from example-org/feat\n",
		"a parent header after a header":   "tree " + tree + "\nparent " + p1 + "\nencoding UTF-8\nparent " + p2 + "\n" + who + "\nmsg\n",
		"a continuation after the parents": "tree " + tree + "\nparent " + p1 + "\n parent " + p2 + "\n" + who + "\nmsg\n",
	} {
		parents, _, ok := parseCommitObject([]byte(obj))
		if !ok || len(parents) != 1 || parents[0] != p1 {
			t.Errorf("%s: parents %v ok=%v, want [%s]", name, parents, ok, p1)
		}
	}
	for name, bad := range map[string]string{
		"no tree header":    "parent " + p1 + "\n" + who + "\nmsg\n",
		"a short parent id": "tree " + tree + "\nparent " + p1[:39] + "\n" + who + "\nmsg\n",
		"a short tree id":   "tree " + tree[:39] + "\n" + who + "\nmsg\n",
	} {
		if _, _, ok := parseCommitObject([]byte(bad)); ok {
			t.Errorf("%s: want a refusal", name)
		}
	}
}

// regression: #4409 review SEC-1 — an addition whose commit is not a full
// object id, or whose commit object does not read as one, releases nothing.
func TestWitnessLandingCommitObjectRequired(t *testing.T) {
	runV2Cases(t, []v2case{
		{name: "an adding commit that is not an object id", mut: func(fx *v2fx) {
			fx.refs = func(fx *v2fx) {
				fx.repo.additions = func(adds []WitnessAddition) []WitnessAddition {
					for i := range adds {
						adds[i].Commit = strings.ToUpper(adds[i].Commit)
					}
					return adds
				}
			}
		}, gap: GapUnestablished, has: []string{"not an object id"}},
		{name: "an adding commit missing from the object store", mut: func(fx *v2fx) {
			fx.refs = func(fx *v2fx) {
				fx.repo.additions = func(adds []WitnessAddition) []WitnessAddition {
					for i := range adds {
						adds[i].Commit = strings.Repeat("9", 40)
					}
					return adds
				}
			}
		}, gap: GapUnestablished, has: []string{"not in the object store"}},
		// regression: #4409 review F15 — an id that reads as another object
		// type is named as such, not as a shallow clone.
		{name: "an adding commit id that reads as a non-commit object", mut: func(fx *v2fx) {
			fx.refs = func(fx *v2fx) { fx.repo.objectType = map[string]string{fx.landing: "tag"} }
		}, gap: GapUnestablished, has: []string{"does not name a commit object"}},
		{name: "the commit object's parents decide, not the walked graph", mut: func(fx *v2fx) {
			fx.refs = func(fx *v2fx) { fx.repo.objectParents = map[string][]string{fx.landing: {fx.base}} }
		}, gap: GapCRUnverified, has: []string{"parent"}},
	})
}

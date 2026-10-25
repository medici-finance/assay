package evaluation

import (
	"reflect"
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

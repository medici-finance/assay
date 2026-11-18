package deskkit

// forgebodycap_guard_test.go — the source guard for the forge body cap. It scans the shipped
// files of the roots below (forgeBodyGuardRoots) for a response body read with io.ReadAll,
// copied, or decoded (bare, or wrapped in an io.LimitReader that silently truncates) instead of
// going through ReadCappedForgeBody / ReadForgeBodyUpTo, for a body read whose error is
// DISCARDED, and for a truncating reader handed to a capped reader (the capped reader then sees
// a body that already ends at the wrapper's bound, so its over-cap refusal can never fire). The
// planted fixture proves the scanner sees each shape, so a scanner that sees nothing cannot
// certify everything. scanForgeBodyReads states exactly which functions are scanned and which
// forms are recognised.
//
// What it does NOT claim: the guard is syntactic (go/ast, no type information) and NOT
// exhaustive. A read in a file outside the roots is not held to the cap. TestForgeBodyOutsideRoots
// inventories every tree of the repository outside the roots (from the repository root down;
// hidden directories and testdata are skipped) and pins each one's hits in forgeBodyGuardGaps or
// forgeBodyNotForge, so such a read is listed, never silent. Some ways of reaching a body through
// indirection are not seen;
// those known blind spots are tracked outside this file. A clean run means "no shape this guard
// recognises", never "every body read in the tree is capped".

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// forgeBodyGuardRoots are the trees whose forge body reads must go through the cap: the forge
// library, the credential paths, the lock-holding write verbs, and the commands that read
// the forge directly.
var forgeBodyGuardRoots = []string{
	"internal/deskkit",
	"internal/credential",
	"cmd/deskevidence",
	"cmd/deskpost",
	"cmd/deskrelease",
	"cmd/deskadvisory",
	"cmd/deskinbox",
	"cmd/deskgitlab",
	"cmd/desktoken",
	"cmd/deskfleet",
	"cmd/deskapps",
	"internal/forgelabels",
	// The release-pin checker is its own module (tools/pairedversions) and cannot import
	// deskkit; it keeps a local readCapped with the same refuse-over-limit rule.
	"../pairedversions",
}

// forgeBodyNoTransportCap are the roots scanned in strict mode: every response body handed to
// any call other than a capped reader is a hit there (uncapped-body-handoff). It is NOT the
// list of roots without a transport-level body cap. Several other roots also build a plain
// http.Client with a timeout and no transport cap — for example cmd/deskadvisory,
// cmd/deskinbox (detail.go), cmd/deskfleet (main.go), cmd/deskapps, cmd/deskgitlab,
// internal/credential (gitlab.go) and internal/deskkit (hotfix.go, hotfixseam.go,
// compositionsource.go). Strict mode is off there because the handoff shape matches by name
// and also reports Body fields of already-capped answers. Every body on those paths goes
// through a capped reader today, but a body handed to a plain io.Reader helper there would be
// neither seen by this guard nor bounded by a transport cap.
var forgeBodyNoTransportCap = map[string]string{
	"../pairedversions": "its http.Client is a plain client with a timeout; every read is a direct readCapped call",
}

// forgeBodyReadPermits are the readers that keep their own, narrower bound AND refuse an
// over-bound body (they read limit+1 and check). Key: "<root>/<file>:<func> <kind>" — the
// hit kind is part of the key, and a permit covers ONE hit: a second read added to the same
// function is still reported, whether it is of another kind or of the permitted kind.
var forgeBodyReadPermits = map[string]string{
	"internal/deskkit/compositionsource.go:httpFetchBounded uncapped-body-read": "reads limit+1 and refuses a body over its own limit",
	"internal/deskkit/forge_github.go:GitHubForge.runLog uncapped-body-copy": "streams into runLogSink, which refuses an archive past runLogArchiveCap; " +
		"the client transport caps the same body one byte past that bound",
	"cmd/deskfleet/pushrotate.go:pushRun.provePush uncapped-body-copy": "drains a status-only probe answer to io.Discard under maxResponseBytes; " +
		"the body is never read as an answer",
}

// bodyReadFuncs are the calls whose error result is a body read's error.
var bodyReadFuncs = map[string]bool{
	"ReadCappedForgeBody": true, "ReadForgeBodyUpTo": true, "readCapped": true,
	"readCappedResponse": true, "readCappedN": true, "ReadForgeBody": true,
}

// regression: #6730
func TestForgeBodyReadsCapped(t *testing.T) {
	var found []string
	for _, r := range forgeBodyGuardRoots {
		hits, err := scanForgeBodyReads(filepath.Join(deskTreeRoot, r), r)
		if err != nil {
			t.Fatalf("the forge-body guard could not scan %s: %v — could-not-check, NOT clean", r, err)
		}
		found = append(found, hits...)
	}
	bad, unused := unpermittedBodyReads(found, forgeBodyReadPermits)
	for _, h := range bad {
		t.Errorf("%s — read forge response bodies through deskkit.ReadCappedForgeBody (or ReadForgeBodyUpTo "+
			"for a narrower bound), hand it the response body itself (never a truncating wrapper), and never "+
			"discard the read error: an uncapped or truncating read buffers a hostile body whole or hands back "+
			"a prefix as the answer (#6730)", h)
	}
	for _, key := range unused {
		t.Errorf("permit %s matches no read any more — drop it", key)
	}
}

// unpermittedBodyReads splits the guard's hits into the ones no permit covers and the permits
// no hit used. A permit covers exactly one "<file>:<func> <kind>" hit — the first; every
// further hit with the same key is reported, so two same-kind reads under one permit fail.
func unpermittedBodyReads(hits []string, permits map[string]string) (bad, unused []string) {
	used := map[string]bool{}
	for _, h := range hits {
		if _, ok := permits[h]; ok && !used[h] {
			used[h] = true
			continue
		}
		bad = append(bad, h)
	}
	for key := range permits {
		if !used[key] {
			unused = append(unused, key)
		}
	}
	sort.Strings(unused)
	return bad, unused
}

// A permit granted for one read in a function does not cover a second read of another kind in
// the same function: planted.go's permitThenRead drains a status probe (the permitted copy)
// and then reads the body whole, and only the drain may pass.
//
// regression: #6755
func TestForgeBodyPermitIsPerKind(t *testing.T) {
	hits, err := scanForgeBodyReads(filepath.Join("testdata", "forgebodycap"), "planted")
	if err != nil {
		t.Fatal(err)
	}
	permits := map[string]string{"planted/planted.go:permitThenRead uncapped-body-copy": "the status-probe drain"}
	bad, unused := unpermittedBodyReads(hits, permits)
	if len(unused) != 0 {
		t.Fatalf("the permit matched no hit: %v", unused)
	}
	var seen bool
	for _, h := range bad {
		switch h {
		case "planted/planted.go:permitThenRead uncapped-body-read":
			seen = true
		case "planted/planted.go:permitThenRead uncapped-body-copy":
			t.Errorf("the permitted drain was still reported: %s", h)
		}
	}
	if !seen {
		t.Errorf("a second read in a permitted function passed under the drain's permit; reported:\n%s",
			strings.Join(bad, "\n"))
	}
}

// A permit covers ONE hit: planted.go's permitTwice drains the body twice under one permit,
// and the second drain must still be reported.
//
// regression: #6755
func TestForgeBodyPermitCoversOne(t *testing.T) {
	hits, err := scanForgeBodyReads(filepath.Join("testdata", "forgebodycap"), "planted")
	if err != nil {
		t.Fatal(err)
	}
	const key = "planted/planted.go:permitTwice uncapped-body-copy"
	bad, unused := unpermittedBodyReads(hits, map[string]string{key: "one drain"})
	if len(unused) != 0 {
		t.Fatalf("the permit matched no hit: %v", unused)
	}
	n := 0
	for _, h := range bad {
		if h == key {
			n++
		}
	}
	if n != 1 {
		t.Errorf("permitTwice's two drains under one permit reported %d times, want 1 (the second drain)", n)
	}
}

// The planted fixture holds one of each shape the guard must catch, and two it must not
// (cappedRead, wrappedCapped).
//
// regression: #6730
func TestForgeBodyGuardSeesPlanted(t *testing.T) {
	hits, err := scanForgeBodyReads(filepath.Join("testdata", "forgebodycap"), "planted")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"planted/planted.go:aliasedRead uncapped-body-read",
		"planted/planted.go:bodyAliasRead uncapped-body-read",
		"planted/planted.go:copyRead uncapped-body-copy",
		"planted/planted.go:decodeRead uncapped-body-decode",
		"planted/planted.go:discardedCapped discarded-read-error",
		"planted/planted.go:discardedRead discarded-read-error",
		"planted/planted.go:discardedRead uncapped-body-read",
		"planted/planted.go:permitThenRead uncapped-body-copy",
		"planted/planted.go:permitThenRead uncapped-body-read",
		"planted/planted.go:permitTwice uncapped-body-copy",
		"planted/planted.go:permitTwice uncapped-body-copy",
		"planted/planted.go:reassignedCapped truncated-capped-read",
		"planted/planted.go:teeTruncCapped truncated-capped-read",
		"planted/planted.go:truncatedAliasCapped truncated-capped-read",
		"planted/planted.go:truncatedCapped truncated-capped-read",
		"planted/planted.go:truncatedLimitedCapped truncated-capped-read",
		"planted/planted.go:truncatingRead uncapped-body-read",
		"planted/planted.go:wrappedAliasCapped truncated-capped-read",
		"planted/planted.go:wrappedTruncCapped truncated-capped-read",
		"planted/sub/nested.go:nestedRead uncapped-body-read",
	}
	sort.Strings(hits)
	if strings.Join(hits, "\n") != strings.Join(want, "\n") {
		t.Errorf("planted hits:\n%s\nwant:\n%s", strings.Join(hits, "\n"), strings.Join(want, "\n"))
	}
}

// regression: #6775
func TestForgeBodyGuardSeesGaps(t *testing.T) {
	hits, err := scanForgeBodyReads(filepath.Join("testdata", "forgebodygaps"), "gaps")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"gaps/gaps.go:a.drain uncapped-body-copy",
		// aliasByName reads a byte slice; the name-not-scope match reports it (fail-closed).
		"gaps/gaps.go:aliasByName uncapped-body-read",
		"gaps/gaps.go:b.drain uncapped-body-copy",
		"gaps/gaps.go:bodyReassignCapped truncated-capped-read",
		"gaps/gaps.go:helper2TruncCapped truncated-capped-read",
		"gaps/gaps.go:helperTruncCapped truncated-capped-read",
		"gaps/gaps.go:loopHopCapped truncated-capped-read",
		"gaps/gaps.go:pkgLitRead uncapped-body-read",
		"gaps/gaps.go:pkgTable uncapped-body-read",
		"gaps/gaps.go:twoHopCapped truncated-capped-read",
	}
	sort.Strings(hits)
	if strings.Join(hits, "\n") != strings.Join(want, "\n") {
		t.Errorf("gap plant hits:\n%s\nwant:\n%s", strings.Join(hits, "\n"), strings.Join(want, "\n"))
	}
}

// The strict mode is switched on by the root's label, the way TestForgeBodyReadsCapped scans
// the real roots: the handoff fixture scanned under the pairedversions root's label must report
// the handoff, and under any other label must not.
//
// regression: #6775
func TestForgeBodyHandoffStrict(t *testing.T) {
	dir := filepath.Join("testdata", "forgebodyhandoff")
	const root = "../pairedversions"
	if !slicesContains(forgeBodyGuardRoots, root) {
		t.Fatalf("%s is no longer a guard root", root)
	}
	hits, err := scanForgeBodyReads(dir, root)
	if err != nil {
		t.Fatal(err)
	}
	want := root + "/handoff.go:handedOff uncapped-body-handoff"
	if strings.Join(hits, "\n") != want {
		t.Errorf("handoff hits under the %s label:\n%s\nwant:\n%s — the root is not scanned strict",
			root, strings.Join(hits, "\n"), want)
	}
	// Outside a strict root the handoff shape is off.
	if hits, err := scanForgeBodyReads(dir, "handoff"); err != nil || len(hits) != 0 {
		t.Errorf("non-strict handoff hits = %v, %v; want none", hits, err)
	}
	for r := range forgeBodyNoTransportCap {
		if !slicesContains(forgeBodyGuardRoots, r) {
			t.Errorf("forgeBodyNoTransportCap names %s, which is not a guard root", r)
		}
	}
}

// forgeBodyInventoryParents are the directories, relative to deskTreeRoot, whose children
// TestForgeBodyOutsideRoots scans: each child directory (with its subdirectories) and each
// shipped .go file directly under a parent is one tree. A child that is a guard root, or is
// itself a parent, is skipped. The first parent is the repository root and every other parent
// is a child of an earlier one (TestForgeBodyInventoryWhole), so the trees cover the whole
// repository outside the roots: a reader added in any module is inventoried.
var forgeBodyInventoryParents = []string{
	forgeBodyRepoRoot,
	"..",
	".",
	"cmd",
	"internal",
	"../../deskd",
	"../../deskd/cmd",
	"../../deskd/internal",
	"../../statusgen",
}

// forgeBodyRepoRoot is the repository root, relative to deskTreeRoot.
const forgeBodyRepoRoot = "../.."

// forgeBodyGuardGaps are trees, relative to deskTreeRoot, that read forge response bodies but
// are NOT guard roots: their reads are not held to the cap by this guard. Each is kept out by
// decision — widening a root to a client with no transport cap means rewriting its reads
// first — and listed so the gap is visible. Each entry pins the tree's exact hit set
// ("<file>:<func> <kind>", relative to the tree). Some of those hits are not forge response
// reads (a server's own request body, for one); they are pinned all the same. A tree whose
// hits change fails TestForgeBodyOutsideRoots: a new read is reported, a fixed read forces
// the entry to be updated, and a gap with no hit left moves into forgeBodyGuardRoots. An entry
// pins at least one hit: an empty entry is refused whatever its tree scans as.
var forgeBodyGuardGaps = map[string][]string{
	"../../deskd/internal/adapter": {
		"github.go:Client.conditionalGet uncapped-body-read",
		"github.go:Client.graphQL uncapped-body-read",
		"permission.go:PermissionRouter.Permission uncapped-body-read",
		"permission.go:PermissionRouter.RepoPrivate uncapped-body-read",
		"readonly.go:drainBody uncapped-body-read",
	},
	"../../deskd/internal/answer": {
		"github.go:HumanWriter.CurrentState uncapped-body-read",
		"github.go:HumanWriter.mutate discarded-read-error",
		"github.go:HumanWriter.mutate uncapped-body-read",
	},
	"../../deskd/internal/comment": {
		"github.go:UserWriter.Comment discarded-read-error",
		"github.go:UserWriter.Comment uncapped-body-read",
	},
	"../../deskd/internal/contract": {
		"client.go:Client.get uncapped-body-read",
		"client.go:Client.openStream discarded-read-error",
		"client.go:Client.openStream uncapped-body-read",
		"client.go:Client.postAction uncapped-body-read",
		"client.go:Client.postNoCred discarded-read-error",
		"client.go:Client.postNoCred uncapped-body-read",
		"client.go:Client.viewerAction uncapped-body-read",
	},
	"../../deskd/internal/identity": {
		"auth.go:Validator.check uncapped-body-decode",
		"oauth.go:Auth.currentLogin uncapped-body-read",
		"oauth.go:Auth.exchange uncapped-body-read",
	},
	"../../deskd/internal/mcp": {
		"github_filer.go:GitHubFiler.doRecord uncapped-body-read",
		"github_filer.go:GitHubFiler.post uncapped-body-read",
		"mcp.go:Server.Handler uncapped-body-decode",
	},
	"../../statusgen/claimdecay_github.go": {
		"claimdecay_github.go:githubGraphQLPost uncapped-body-read",
	},
	"../../statusgen/claimdecay_gitlab.go": {
		"claimdecay_gitlab.go:gitlabAPI.getMRPage uncapped-body-read",
	},
	"../../statusgen/ghfetch.go": {
		"ghfetch.go:ghClient.get uncapped-body-read",
	},
	"cmd/deskinstall": {
		"install.go:httpGet uncapped-body-read",
	},
}

// forgeBodyNotForge are trees outside the roots whose hits are not forge response reads, or
// are reads that keep their own bound. They are not gaps, but their hit sets are pinned the
// same way, so a new read in one of them is still reported. Like a gap, an entry pins at least
// one hit.
var forgeBodyNotForge = map[string]notForgeEntry{
	"../../deskd/internal/chat": {"a model backend's answer, not a forge response", []string{
		"httpbackend.go:HTTPBackend.Complete uncapped-body-read",
	}},
	"../../deskd/internal/serve": {"the local server's own request bodies", []string{
		"v1_actions.go:decodeActionBody uncapped-body-read",
		"v1_actions.go:peekIntent discarded-read-error",
		"v1_actions.go:peekIntent uncapped-body-read",
		"v1_overrides.go:Overrides.handlePost uncapped-body-decode",
		"v1_terminal_actions.go:decodeStrict uncapped-body-decode",
	}},
	"../../forgeread": {"the forge-service client's answer, read to limit+1 and refused over its limit", []string{
		"client/client.go:Client.Do uncapped-body-read",
	}},
	"../../statusgen/modcachepublic.go": {"reads limit+1 and refuses a body over its limit", []string{
		"modcachepublic.go:provePublicAssay uncapped-body-read",
	}},
	"../../statusgen/telemetry.go": {"drains a telemetry endpoint's answer, never read as data", []string{
		"telemetry.go:sendTelemetry uncapped-body-copy",
	}},
	"cmd/deskboard": {"a cache transport that reads limit+1 and hands an oversized body on whole", []string{
		"readcost.go:boardTransport.RoundTrip uncapped-body-copy",
		"readcost.go:boardTransport.RoundTrip uncapped-body-read",
	}},
	"internal/kubecompat": {"a test fixture server reading its own request bodies", []string{
		"testfixture/server.go:Server.serve uncapped-body-decode",
	}},
	"internal/reviewtest": {"a test helper transport", []string{
		"http.go:Transport.RoundTrip discarded-read-error",
		"http.go:Transport.RoundTrip uncapped-body-read",
	}},
}

// notForgeEntry is one forgeBodyNotForge entry: why the tree is not a forge reader, and its
// pinned hits.
type notForgeEntry struct {
	why  string
	hits []string
}

// Every tree outside the roots that the inventory reaches must show exactly the hits its
// forgeBodyGuardGaps or forgeBodyNotForge entry pins: an unlisted tree with a hit, a listed
// tree whose hits changed, and a listed tree the inventory never reaches all fail.
//
// regression: #6775
func TestForgeBodyOutsideRoots(t *testing.T) {
	listed, problems := forgeBodyListed(forgeBodyGuardGaps, forgeBodyNotForge)
	for _, p := range problems {
		t.Error(p)
	}
	trees, err := forgeBodyInventoryIn(deskTreeRoot, forgeBodyInventoryParents, forgeBodyGuardRoots)
	if err != nil {
		t.Fatal(err)
	}
	scanned := map[string]bool{}
	for _, tree := range trees {
		hits, err := scanForgeBodyReads(filepath.Join(deskTreeRoot, tree), tree)
		if errors.Is(err, errNoGoSource) {
			continue
		}
		if err != nil {
			t.Errorf("the forge-body guard could not scan %s: %v — could-not-check, NOT clean", tree, err)
			continue
		}
		scanned[tree] = true
		got := make([]string, 0, len(hits))
		for _, h := range hits {
			got = append(got, strings.TrimPrefix(h, tree+"/"))
		}
		sort.Strings(got)
		pinned, isListed := listed[tree]
		want := append([]string(nil), pinned...)
		sort.Strings(want)
		if strings.Join(got, "\n") == strings.Join(want, "\n") {
			continue
		}
		switch {
		case !isListed:
			t.Errorf("%s reads response bodies outside every guard root and is not listed — move it into "+
				"forgeBodyGuardRoots, or list it in forgeBodyGuardGaps (or forgeBodyNotForge, with a reason):\n%s",
				tree, strings.Join(got, "\n"))
		case len(got) == 0:
			t.Errorf("%s shows no uncapped body read any more — move it into forgeBodyGuardRoots and drop "+
				"its entry", tree)
		default:
			t.Errorf("%s hits changed:\n%s\nlisted:\n%s", tree, strings.Join(got, "\n"), strings.Join(want, "\n"))
		}
	}
	for tree := range listed {
		if !scanned[tree] {
			t.Errorf("%s is listed outside the roots, but the inventory never scans it (a guard root, gone, or "+
				"not a tree the inventory reaches) — drop or move its entry", tree)
		}
	}
}

// forgeBodyListed merges the two inventory tables into one tree→pinned-hits map and reports
// every entry the tables must not hold: a tree in both tables, a forgeBodyNotForge entry with no
// reason, and an entry in either table that pins no hit. An empty entry is refused here, before
// any scan: compared against a tree with no hits it would match and pass, so a tree with no
// uncapped read left could keep its entry instead of moving into forgeBodyGuardRoots.
func forgeBodyListed(gaps map[string][]string, notForge map[string]notForgeEntry) (map[string][]string, []string) {
	listed := map[string][]string{}
	var problems []string
	for tree, hits := range gaps {
		if len(hits) == 0 {
			problems = append(problems, fmt.Sprintf("forgeBodyGuardGaps entry %s pins no hit — a tree with no "+
				"uncapped body read moves into forgeBodyGuardRoots; drop the entry", tree))
		}
		listed[tree] = hits
	}
	for tree, nf := range notForge {
		if _, dup := listed[tree]; dup {
			problems = append(problems, fmt.Sprintf("%s is listed both as a gap and as not a forge reader", tree))
		}
		if nf.why == "" {
			problems = append(problems, fmt.Sprintf("forgeBodyNotForge entry %s gives no reason", tree))
		}
		if len(nf.hits) == 0 {
			problems = append(problems, fmt.Sprintf("forgeBodyNotForge entry %s pins no hit — a tree with no "+
				"body read needs no entry; drop it", tree))
		}
		listed[tree] = nf.hits
	}
	sort.Strings(problems)
	return listed, problems
}

// An entry that pins no hit is refused in both tables, even for a tree that scans clean, where
// comparing the scan with the pin alone would pass it.
//
// regression: #6794
func TestForgeBodyEmptyEntry(t *testing.T) {
	gaps := map[string][]string{"gapdir": {}, "nilgap": nil, "kept": {"a.go:f uncapped-body-read"}}
	notForge := map[string]notForgeEntry{
		"nfdir":  {"a reason", []string{}},
		"nfkept": {"a reason", []string{"b.go:g uncapped-body-read"}},
	}
	listed, problems := forgeBodyListed(gaps, notForge)
	for _, tree := range []string{"gapdir", "nilgap", "nfdir"} {
		n := 0
		for _, p := range problems {
			if strings.Contains(p, " "+tree+" pins no hit") {
				n++
			}
		}
		if n != 1 {
			t.Errorf("an empty entry for %s was reported %d times, want 1; problems:\n%s",
				tree, n, strings.Join(problems, "\n"))
		}
	}
	if len(problems) != 3 {
		t.Errorf("problems = %d, want 3 (the three empty entries only):\n%s", len(problems), strings.Join(problems, "\n"))
	}
	if len(listed) != 5 {
		t.Errorf("listed %d trees, want 5", len(listed))
	}
}

// forgeBodyInventoryIn lists the trees under parents (relative to base), roots excluded. A
// child's label is its path relative to base, so a parent reached from another parent (".."
// from "../..", "." from "..") is recognised whatever spelling reaches it.
func forgeBodyInventoryIn(base string, parents, roots []string) ([]string, error) {
	absBase, err := filepath.Abs(base)
	if err != nil {
		return nil, fmt.Errorf("the forge-body inventory could not resolve %s: %v — could-not-check", base, err)
	}
	var trees []string
	for _, parent := range parents {
		dir := filepath.Join(absBase, parent)
		ents, err := os.ReadDir(dir)
		if err != nil {
			return nil, fmt.Errorf("the forge-body inventory could not read %s: %v — could-not-check", parent, err)
		}
		for _, e := range ents {
			name := e.Name()
			rel, err := filepath.Rel(absBase, filepath.Join(dir, name))
			if err != nil {
				return nil, fmt.Errorf("the forge-body inventory could not label %s: %v — could-not-check", name, err)
			}
			tree := filepath.ToSlash(rel)
			switch {
			case strings.HasPrefix(name, "."), name == "testdata":
			case slicesContains(roots, tree), slicesContains(parents, tree):
			case e.IsDir():
				trees = append(trees, tree)
			case strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go"):
				trees = append(trees, tree)
			}
		}
	}
	if len(trees) == 0 {
		return nil, errors.New("the forge-body inventory found no trees — could-not-check, NOT clean")
	}
	return trees, nil
}

// forgeBodyInventoryBounds reports every way parents fails to cover the whole tree under
// repoRoot: repoRoot is not the first parent, or a parent is not a child of an earlier parent
// (its siblings would then be scanned by nobody, or its own tree scanned twice).
func forgeBodyInventoryBounds(parents []string, repoRoot string) []string {
	var problems []string
	if len(parents) == 0 || parents[0] != repoRoot {
		problems = append(problems, fmt.Sprintf("the first inventory parent must be the repository root %s — "+
			"a reader outside the listed parents would not be inventoried", repoRoot))
	}
	for i, p := range parents {
		if i == 0 {
			continue
		}
		up := filepath.ToSlash(filepath.Join(p, ".."))
		if !slicesContains(parents[:i], up) {
			problems = append(problems, fmt.Sprintf("inventory parent %s is not a child of an earlier parent — the "+
				"trees beside it are not inventoried", p))
		}
	}
	return problems
}

// The inventory is bounded by the repository, not by a short list of parents: the real parents
// start at the repository root and chain down, and a reader planted in a module beside the
// desk module (testdata/forgebodyinventory) is reached and reported.
//
// regression: #6794
func TestForgeBodyInventoryWhole(t *testing.T) {
	for _, p := range forgeBodyInventoryBounds(forgeBodyInventoryParents, forgeBodyRepoRoot) {
		t.Error(p)
	}
	// The bounds check itself refuses a list that starts below the root, or skips a level.
	if len(forgeBodyInventoryBounds([]string{".", "cmd"}, forgeBodyRepoRoot)) == 0 {
		t.Error("a parent list that starts at the desk module passed the bounds check")
	}
	if len(forgeBodyInventoryBounds([]string{forgeBodyRepoRoot, "../../deskd/internal"}, forgeBodyRepoRoot)) == 0 {
		t.Error("a parent list that skips a level passed the bounds check")
	}

	// The planted repository: tools/desk is the desk module; tools/guarded is a guard root with a
	// read; elsewhere is a module beside it with a read.
	base := filepath.Join("testdata", "forgebodyinventory", "tools", "desk")
	parents := []string{forgeBodyRepoRoot, "..", "."}
	if p := forgeBodyInventoryBounds(parents, forgeBodyRepoRoot); len(p) != 0 {
		t.Fatalf("the planted parents are not whole: %v", p)
	}
	trees, err := forgeBodyInventoryIn(base, parents, []string{"../guarded"})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(trees)
	if want := "../../elsewhere\ncmd"; strings.Join(trees, "\n") != want {
		t.Fatalf("planted inventory:\n%s\nwant:\n%s", strings.Join(trees, "\n"), want)
	}
	hits, err := scanForgeBodyReads(filepath.Join(base, "../../elsewhere"), "../../elsewhere")
	if err != nil {
		t.Fatal(err)
	}
	if want := "../../elsewhere/reader.go:fetch uncapped-body-read"; strings.Join(hits, "\n") != want {
		t.Errorf("planted reader hits:\n%s\nwant:\n%s", strings.Join(hits, "\n"), want)
	}
}

func slicesContains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// scanForgeBodyReads parses every shipped .go file under path, subdirectories included (no
// _test.go, no testdata), and returns one "<label>/<file>:<func> <kind>" per offending read.
// path may also name one .go file, which is scanned alone. <func> is the function's name, a
// method's receiver type and name (T.name), or, for a function literal outside every function
// declaration, the package-level name it is bound to.
//
// It is a syntactic scan without type information, and it sees exactly these shapes:
//   - io.ReadAll, json.NewDecoder or io.Copy/io.CopyN whose reader is a response body, named
//     directly (resp.Body) or through a local alias assigned from one (r := resp.Body);
//   - a discarded body-read error;
//   - a truncating reader handed to one of the capped readers (truncated-capped-read): the
//     capped reader then sees a body that already ends at the wrapper's bound, so its
//     over-cap refusal can never fire. See buildsTruncating for the forms it recognises;
//   - in a root listed in forgeBodyNoTransportCap, a response body handed to any call other
//     than a capped reader (uncapped-body-handoff).
//
// Every function declaration is scanned, with the function literals inside it, and so is
// every function literal outside all of them (a package-level func value, or a func field in
// a package-level composite literal).
//
// Names are matched by name, not by scope or order: a name assigned a body (or a truncating
// reader) anywhere in a function makes every use of that name in the function count, and a
// helper's name matches any call of that name in its package. That can only add hits, so it
// errs toward failing the build, and it is kept that way by decision.
//
// A body handed to a helper that takes a plain io.Reader is out of its sight in the roots
// that are not strict. Where such a root reads through the forge library's GitHub and GitLab
// clients, their transport cap bounds the body (forgebodytransport.go, guarded by
// TestForgeClientsCapped); a root on a plain client has no such bound (see
// forgeBodyNoTransportCap).
func scanForgeBodyReads(path, label string) ([]string, error) {
	_, strict := forgeBodyNoTransportCap[label]
	return scanForgeBodyReadsIn(path, label, strict)
}

// errNoGoSource is the scan error for a path that holds no shipped Go file.
var errNoGoSource = errors.New("no Go source")

// scanForgeBodyReadsIn is scanForgeBodyReads with the handoff shape switched on or off.
func scanForgeBodyReadsIn(path, label string, strict bool) ([]string, error) {
	type parsed struct {
		f   *ast.File
		rel string
	}
	byDir := map[string][]parsed{}
	var dirs []string
	fset := token.NewFileSet()
	files := 0
	add := func(file, rel string) error {
		f, err := parser.ParseFile(fset, file, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		files++
		d := filepath.Dir(file)
		if _, seen := byDir[d]; !seen {
			dirs = append(dirs, d)
		}
		byDir[d] = append(byDir[d], parsed{f, label + "/" + filepath.ToSlash(rel)})
		return nil
	}
	st, err := os.Stat(path)
	switch {
	case err != nil:
		return nil, err
	case !st.IsDir():
		err = add(path, filepath.Base(path))
	default:
		err = filepath.WalkDir(path, func(p string, e os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			name := e.Name()
			if e.IsDir() {
				if name == "testdata" || (p != path && strings.HasPrefix(name, ".")) {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				return nil
			}
			rel, _ := filepath.Rel(path, p)
			return add(p, rel)
		})
	}
	if err != nil {
		return nil, err
	}
	if files == 0 {
		return nil, fmt.Errorf("%w under %s — a guard that scans nothing certifies nothing", errNoGoSource, path)
	}
	var out []string
	for _, d := range dirs {
		pkg := map[*ast.File]string{}
		for _, p := range byDir[d] {
			pkg[p.f] = p.rel
		}
		helpers := truncatingHelpers(pkg)
		for _, p := range byDir[d] {
			out = append(out, scanBodyReadFile(p.f, p.rel, helpers, strict)...)
		}
	}
	return out, nil
}

// funcBodies yields every function body in f the guard scans, with the name it reports it
// under: each function declaration (a method as T.name), and each function literal outside
// every declaration, named for the package-level name it is bound to.
func funcBodies(f *ast.File, yield func(name string, body *ast.BlockStmt)) {
	for _, d := range f.Decls {
		switch x := d.(type) {
		case *ast.FuncDecl:
			if x.Body != nil {
				yield(funcDeclName(x), x.Body)
			}
		case *ast.GenDecl:
			for _, s := range x.Specs {
				vs, ok := s.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, v := range vs.Values {
					name := vs.Names[0].Name
					if i < len(vs.Names) {
						name = vs.Names[i].Name
					}
					ast.Inspect(v, func(n ast.Node) bool {
						lit, ok := n.(*ast.FuncLit)
						if !ok {
							return true
						}
						yield(name, lit.Body)
						return false // the outer literal's scan covers literals inside it
					})
				}
			}
		}
	}
}

// funcDeclName is a function's name, or T.name for a method on T or *T.
func funcDeclName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return fn.Name.Name
	}
	t := fn.Recv.List[0].Type
	for {
		switch x := t.(type) {
		case *ast.StarExpr:
			t = x.X
			continue
		case *ast.IndexExpr:
			t = x.X
			continue
		case *ast.IndexListExpr:
			t = x.X
			continue
		case *ast.ParenExpr:
			t = x.X
			continue
		}
		break
	}
	if id, ok := t.(*ast.Ident); ok {
		return id.Name + "." + fn.Name.Name
	}
	return fn.Name.Name
}

func scanBodyReadFile(f *ast.File, label string, helpers map[string]bool, strict bool) []string {
	ioName := importName(f, "io", "io")
	jsonName := importName(f, "encoding/json", "json")
	var out []string
	funcBodies(f, func(name string, body *ast.BlockStmt) {
		where := fmt.Sprintf("%s:%s", label, name)
		aliases := bodyAliases(body)
		truncating := truncatingAliases(body, ioName, helpers)
		isTruncating := func(e ast.Expr) bool { return buildsTruncating(e, ioName, truncating, helpers) }
		isBody := func(e ast.Expr) bool {
			if mentionsBody(e) {
				return true
			}
			id, ok := e.(*ast.Ident)
			return ok && aliases[id.Name]
		}
		ast.Inspect(body, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.CallExpr:
				switch {
				case ioName != "" && isPkgCall(x, ioName, "ReadAll") && len(x.Args) == 1 && isBody(x.Args[0]):
					out = append(out, where+" uncapped-body-read")
				case ioName != "" && (isPkgCall(x, ioName, "Copy") || isPkgCall(x, ioName, "CopyN")) && len(x.Args) >= 2 && isBody(x.Args[1]):
					out = append(out, where+" uncapped-body-copy")
				case jsonName != "" && isPkgCall(x, jsonName, "NewDecoder") && len(x.Args) == 1 && isBody(x.Args[0]):
					out = append(out, where+" uncapped-body-decode")
				case isCappedReadCall(x) && len(x.Args) >= 1 && isTruncating(x.Args[0]):
					out = append(out, where+" truncated-capped-read")
				case strict && !isCappedReadCall(x) && anyArg(x, isBody):
					out = append(out, where+" uncapped-body-handoff")
				}
			case *ast.AssignStmt:
				if len(x.Lhs) == 2 && len(x.Rhs) == 1 && isBlank(x.Lhs[1]) {
					if c, ok := x.Rhs[0].(*ast.CallExpr); ok && isBodyReadCall(c, ioName) {
						out = append(out, where+" discarded-read-error")
					}
				}
			}
			return true
		})
	})
	return out
}

// anyArg reports whether any argument of c satisfies pred.
func anyArg(c *ast.CallExpr, pred func(ast.Expr) bool) bool {
	for _, a := range c.Args {
		if pred(a) {
			return true
		}
	}
	return false
}

// bodyAliases: local names assigned a bare response body (r := resp.Body, var r io.Reader =
// resp.Body). A call result (a capped reader, a wrapper) is not an alias.
func bodyAliases(body *ast.BlockStmt) map[string]bool {
	out := map[string]bool{}
	bare := func(e ast.Expr) bool {
		s, ok := e.(*ast.SelectorExpr)
		return ok && s.Sel.Name == "Body"
	}
	ast.Inspect(body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.AssignStmt:
			if len(x.Lhs) == len(x.Rhs) {
				for i, r := range x.Rhs {
					if id, ok := x.Lhs[i].(*ast.Ident); ok && bare(r) {
						out[id.Name] = true
					}
				}
			}
		case *ast.ValueSpec:
			if len(x.Names) == len(x.Values) {
				for i, v := range x.Values {
					if bare(v) {
						out[x.Names[i].Name] = true
					}
				}
			}
		}
		return true
	})
	return out
}

// truncatingReader reports whether e builds a reader that ENDS at a bound instead of
// refusing past it: io.LimitReader(...), an io.LimitedReader literal (or its address), or a
// call of a helper named in helpers (a function of the same package that returns one).
func truncatingReader(e ast.Expr, ioName string, helpers map[string]bool) bool {
	if u, ok := e.(*ast.UnaryExpr); ok && u.Op == token.AND {
		e = u.X
	}
	switch x := e.(type) {
	case *ast.CallExpr:
		if ioName != "" && isPkgCall(x, ioName, "LimitReader") {
			return true
		}
		switch f := x.Fun.(type) {
		case *ast.Ident:
			return helpers[f.Name]
		case *ast.SelectorExpr:
			return helpers[f.Sel.Name]
		}
	case *ast.CompositeLit:
		sel, ok := x.Type.(*ast.SelectorExpr)
		if ioName == "" || !ok || sel.Sel.Name != "LimitedReader" {
			return false
		}
		id, ok := sel.X.(*ast.Ident)
		return ok && id.Name == ioName
	}
	return false
}

// buildsTruncating reports whether e is, or holds anywhere inside it, a truncating reader
// (truncatingReader) or a name in aliases: a local (lr) or a field path (resp.Body). The
// recognised forms, as the capped reader's reader argument:
//   - an io.LimitReader(...) call or an io.LimitedReader literal (or its address), whether it
//     is the whole argument or sits anywhere inside it (bufio.NewReader(io.LimitReader(...)),
//     io.TeeReader(io.LimitReader(...), w));
//   - a call of a helper in the same package whose return statement returns such an
//     expression, followed through helpers that return a call of another (truncatingHelpers);
//   - a local name, or a field path such as resp.Body, assigned such an expression in the same
//     function by :=, var or a plain = (a reassignment such as r = io.LimitReader(r, n), or
//     resp.Body = io.NopCloser(io.LimitReader(resp.Body, n)), included), followed through any
//     number of further assignments (truncatingAliases), named alone or inside the argument.
//
// The Sel of a selector is a field or method name, never a local, so a selector that is not
// itself an alias has only its X searched.
func buildsTruncating(e ast.Expr, ioName string, aliases, helpers map[string]bool) bool {
	seen := false
	ast.Inspect(e, func(n ast.Node) bool {
		switch x := n.(type) {
		case nil:
			return false
		case *ast.SelectorExpr:
			if k := selectorKey(x); k != "" && aliases[k] {
				seen = true
			}
			seen = seen || buildsTruncating(x.X, ioName, aliases, helpers)
			return false
		case *ast.Ident:
			if aliases[x.Name] {
				seen = true
			}
		case ast.Expr:
			if truncatingReader(x, ioName, helpers) {
				seen = true
			}
		}
		return !seen
	})
	return seen
}

// selectorKey spells a name or a field path (resp.Body, c.resp.Body) as written, or "" for any
// other expression.
func selectorKey(e ast.Expr) string {
	switch x := e.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.SelectorExpr:
		if k := selectorKey(x.X); k != "" {
			return k + "." + x.Sel.Name
		}
	}
	return ""
}

// truncatingAliases: the names and field paths assigned, anywhere in body, an expression that
// is or holds a truncating reader (lr := io.LimitReader(...), br := bufio.NewReader(lr),
// resp.Body = io.NopCloser(io.LimitReader(resp.Body, n))). It repeats until nothing new is
// found, so a name assigned another such name (r2 := lr) is followed however many hops deep.
func truncatingAliases(body *ast.BlockStmt, ioName string, helpers map[string]bool) map[string]bool {
	out := map[string]bool{}
	for grew := true; grew; {
		grew = false
		mark := func(k string, v ast.Expr) {
			if k != "" && !out[k] && buildsTruncating(v, ioName, out, helpers) {
				out[k] = true
				grew = true
			}
		}
		ast.Inspect(body, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.AssignStmt:
				if len(x.Lhs) == len(x.Rhs) {
					for i, r := range x.Rhs {
						mark(selectorKey(x.Lhs[i]), r)
					}
				}
			case *ast.ValueSpec:
				if len(x.Names) == len(x.Values) {
					for i, v := range x.Values {
						mark(x.Names[i].Name, v)
					}
				}
			}
			return true
		})
	}
	return out
}

// truncatingHelpers names the functions of one package (files maps each parsed file to its
// label) that return a truncating reader: a return statement in the function's own body (not
// in a literal inside it) whose result is or holds one, by buildsTruncating with the function's
// own aliases. It repeats until nothing new is found, so a helper returning another helper's
// call is followed. A method is named by its method name alone, and a function literal bound
// at package level by the name it is bound to.
func truncatingHelpers(files map[*ast.File]string) map[string]bool {
	out := map[string]bool{}
	for grew := true; grew; {
		grew = false
		for f := range files {
			ioName := importName(f, "io", "io")
			funcBodies(f, func(name string, body *ast.BlockStmt) {
				name = name[strings.LastIndex(name, ".")+1:]
				if out[name] {
					return
				}
				local := truncatingAliases(body, ioName, out)
				ast.Inspect(body, func(n ast.Node) bool {
					switch x := n.(type) {
					case *ast.FuncLit:
						return false
					case *ast.ReturnStmt:
						for _, r := range x.Results {
							if !out[name] && buildsTruncating(r, ioName, local, out) {
								out[name] = true
								grew = true
							}
						}
					}
					return true
				})
			})
		}
	}
	return out
}

// isCappedReadCall: a call to one of the capped body readers (by name, as bodyReadFuncs lists
// them), whatever package qualifies it.
func isCappedReadCall(c *ast.CallExpr) bool {
	switch f := c.Fun.(type) {
	case *ast.Ident:
		return bodyReadFuncs[f.Name]
	case *ast.SelectorExpr:
		return bodyReadFuncs[f.Sel.Name]
	}
	return false
}

func isPkgCall(c *ast.CallExpr, pkg, fn string) bool {
	sel, ok := c.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != fn {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	return ok && id.Name == pkg
}

// isBodyReadCall: io.ReadAll(<anything with .Body>) or one of the capped body readers.
func isBodyReadCall(c *ast.CallExpr, ioName string) bool {
	if ioName != "" && isPkgCall(c, ioName, "ReadAll") && len(c.Args) == 1 && mentionsBody(c.Args[0]) {
		return true
	}
	return isCappedReadCall(c)
}

// mentionsBody reports whether e contains a selector ending in .Body (resp.Body, r.Body, …).
func mentionsBody(e ast.Expr) bool {
	seen := false
	ast.Inspect(e, func(n ast.Node) bool {
		if s, ok := n.(*ast.SelectorExpr); ok && s.Sel.Name == "Body" {
			seen = true
		}
		return !seen
	})
	return seen
}

func isBlank(e ast.Expr) bool {
	id, ok := e.(*ast.Ident)
	return ok && id.Name == "_"
}

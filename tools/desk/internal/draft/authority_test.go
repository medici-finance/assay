package draft

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// THE AUTHORITY SUITE
// -------------------
// Every test here is named TestAuthority* so that the brief's original Verify
// row — `go test ./... -run TestAuthority` — selects exactly this set. What it
// proves, in order:
//
//   - the package's imports are a closed, computation-only allow-list, so
//     there is no capability here to post with;
//   - no I/O call site exists in shipped source;
//   - no exported name is transmit-shaped;
//   - the scanners above actually report a bent tree (positive controls);
//   - an attempted auto-post is REFUSED at run time, including through a
//     destination whose posting tool is measurably present in this module;
//   - the refusal does not depend on the substrate staying absent;
//   - the transitive dependency closure holds no I/O package either, which is
//     the half a source scan of this directory alone cannot see;
//   - the register's Present flags are measurements, held true against the
//     tree.

// allowedImports is the closed set this package may import. It is
// computation only: hashing, encoding, error wrapping, formatting, pattern
// matching, sorting, string handling. Nothing here can open a socket, start a
// process, or touch a file — which is why "this layer cannot post" is a fact
// about the import list rather than a promise about behaviour.
//
// Widening this set is the reviewable event. A commit that adds "os/exec" or
// "net/http" to the package must also edit this line, in the same diff.
var allowedImports = map[string]bool{
	"crypto/sha256": true,
	"encoding/hex":  true,
	"errors":        true,
	"fmt":           true,
	"regexp":        true,
	"sort":          true,
	"strings":       true,
}

// forbiddenCallBases are package identifiers whose calls perform I/O. The
// import allow-list already makes these unreachable; this is the second arm,
// because a single check is a single point of silent failure.
var forbiddenCallBases = map[string]string{
	"exec":    "starts a subprocess",
	"os":      "touches the filesystem or the process environment",
	"io":      "moves bytes to somewhere this package does not own",
	"ioutil":  "moves bytes to somewhere this package does not own",
	"net":     "opens a network connection",
	"http":    "makes an HTTP request",
	"syscall": "reaches the kernel directly",
	"rpc":     "calls out of process",
	"smtp":    "sends mail",
}

// transmitShapes are the words an API that moves something somewhere is
// named with. No exported identifier in this package may contain one: a
// reader scanning the package's exported surface must be unable to find a
// send button, and a future one must collide with this list.
var transmitShapes = []string{
	"Post", "Send", "Submit", "Publish", "Deliver", "Push", "Comment",
	"Reply", "Transmit", "Upload", "Emit", "Dispatch", "Notify", "Mail",
}

// violation is one thing a scan found. Kind is a stable token so the positive
// controls can assert on the class rather than on prose.
type violation struct {
	File   string
	Kind   string
	Detail string
}

func (v violation) String() string { return fmt.Sprintf("%s: %s — %s", v.File, v.Kind, v.Detail) }

// scanGoSource is the ONE scanner. Both the real scan of this package and the
// positive controls below run through it, so a control that passes proves the
// production path can see the same violation. A control with its own private
// matcher proves nothing about the scan that ships.
//
// It returns the violations and the number of imports it actually parsed, so a
// caller can tell "clean" from "saw nothing" — a scanner that silently parsed
// zero declarations is the failure mode this count exists to expose.
func scanGoSource(filename, src string) (found []violation, imports int, err error) {
	fset := token.NewFileSet()
	f, perr := parser.ParseFile(fset, filename, src, parser.SkipObjectResolution)
	if perr != nil {
		return nil, 0, perr
	}
	for _, im := range f.Imports {
		path, uerr := strconv.Unquote(im.Path.Value)
		if uerr != nil {
			return nil, imports, uerr
		}
		imports++
		if !allowedImports[path] {
			found = append(found, violation{filename, "undeclared-import",
				fmt.Sprintf("imports %q, which is not in the computation-only allow-list (%s)", path, sortedSet(allowedImports))})
		}
	}
	ast.Inspect(f, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.CallExpr:
			sel, ok := node.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			base, ok := sel.X.(*ast.Ident)
			if !ok {
				return true
			}
			if why, bad := forbiddenCallBases[base.Name]; bad {
				found = append(found, violation{filename, "io-call-site",
					fmt.Sprintf("calls %s.%s, which %s", base.Name, sel.Sel.Name, why)})
			}
		case *ast.FuncDecl:
			name := node.Name.Name
			if !ast.IsExported(name) {
				return true
			}
			for _, s := range transmitShapes {
				if strings.Contains(name, s) {
					found = append(found, violation{filename, "transmit-shaped-export",
						fmt.Sprintf("exports %q, whose name contains %q — this package's exported surface must hold no send button", name, s)})
				}
			}
		}
		return true
	})
	return found, imports, nil
}

func sortedSet(m map[string]bool) string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return strings.Join(out, " ")
}

// shippedSources returns this package's non-test Go files.
func shippedSources(t *testing.T) map[string]string {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	out := map[string]string{}
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() || !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") {
			continue
		}
		b, rerr := os.ReadFile(n)
		if rerr != nil {
			t.Fatalf("read %s: %v", n, rerr)
		}
		out[n] = string(b)
	}
	if len(out) == 0 {
		t.Fatal("no shipped source found — every scan below would pass vacuously")
	}
	return out
}

func scanShipped(t *testing.T, kind string) []violation {
	t.Helper()
	var hits []violation
	var imports int
	for name, src := range shippedSources(t) {
		vs, n, err := scanGoSource(name, src)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		imports += n
		for _, v := range vs {
			if kind == "" || v.Kind == kind {
				hits = append(hits, v)
			}
		}
	}
	if imports == 0 {
		t.Fatal("the scan parsed zero imports across the whole package — clean and blind look identical here, so this is a failure")
	}
	return hits
}

// TestAuthorityImportsAreAClosedComputationOnlySet is the load-bearing check.
// A package that imports nothing capable of I/O cannot post, cannot write and
// cannot connect, regardless of what any function in it says.
func TestAuthorityImportsAreAClosedComputationOnlySet(t *testing.T) {
	for _, v := range scanShipped(t, "undeclared-import") {
		t.Errorf("%s", v)
	}
}

// TestAuthorityNoIOCallSiteInShippedSource is the second arm.
func TestAuthorityNoIOCallSiteInShippedSource(t *testing.T) {
	for _, v := range scanShipped(t, "io-call-site") {
		t.Errorf("%s", v)
	}
}

// TestAuthorityNoTransmitShapedExportedAPI holds the exported surface free of
// anything a caller could mistake for a send button.
func TestAuthorityNoTransmitShapedExportedAPI(t *testing.T) {
	for _, v := range scanShipped(t, "transmit-shaped-export") {
		t.Errorf("%s", v)
	}
}

// TestAuthorityScannerReportsABentTree is the positive control for all three
// scans above. Each case is source that BREAKS one rule; the scanner must
// report it, with the right class. A scanner never observed failing is an
// assertion, not a check.
func TestAuthorityScannerReportsABentTree(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "a subprocess capability arrives by import",
			src:  "package draft\n\nimport \"os/exec\"\n\nfunc x() { _ = exec.Command }\n",
			want: "undeclared-import",
		},
		{
			name: "a network capability arrives by import",
			src:  "package draft\n\nimport \"net/http\"\n\nvar c = http.DefaultClient\n",
			want: "undeclared-import",
		},
		{
			name: "an I/O call site",
			src:  "package draft\n\nfunc x() { os.WriteFile(\"a\", nil, 0) }\n",
			want: "io-call-site",
		},
		{
			name: "a transmit-shaped exported function",
			src:  "package draft\n\nfunc PostDraft() {}\n",
			want: "transmit-shaped-export",
		},
		{
			name: "a transmit-shaped exported method",
			src:  "package draft\n\ntype D struct{}\n\nfunc (D) SendToInbox() {}\n",
			want: "transmit-shaped-export",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			vs, _, err := scanGoSource("bent.go", c.src)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			var got []string
			for _, v := range vs {
				got = append(got, v.Kind)
				if v.Kind == c.want {
					return
				}
			}
			t.Fatalf("POSITIVE CONTROL FAILED: bent source did not produce a %s violation; scanner reported %v", c.want, got)
		})
	}
	// The control in the other direction: clean source must produce nothing,
	// or the scans above would be satisfied by a matcher that fires on
	// everything.
	clean := "package draft\n\nimport \"strings\"\n\nfunc Render(s string) string { return strings.TrimSpace(s) }\n"
	vs, n, err := scanGoSource("clean.go", clean)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if n != 1 {
		t.Fatalf("the scanner counted %d imports in single-import source — it is not reading what it claims to read", n)
	}
	if len(vs) != 0 {
		t.Fatalf("POSITIVE CONTROL FAILED in the clean direction: clean source reported %v", vs)
	}
}

// TestAuthorityAutoPostAttemptIsRefused is the brief's positive control made
// executable: an attempted auto-post, proven refused, at run time rather than
// by reading the source.
func TestAuthorityAutoPostAttemptIsRefused(t *testing.T) {
	d := mustCompose(t)

	var transmitting, transmittingAndPresent int
	for _, dest := range Destinations() {
		if !dest.Transmits {
			continue
		}
		transmitting++
		if dest.Present {
			transmittingAndPresent++
		}
		pkt, err := d.HandOff(dest.ID)
		if !errors.Is(err, ErrNoAutoPost) {
			t.Errorf("HandOff(%q) returned %v, want ErrNoAutoPost — this destination transmits", dest.ID, err)
		}
		if !pkt.Zero() {
			t.Errorf("HandOff(%q) returned a non-zero packet alongside its refusal — a caller that drops the error would be holding %q", dest.ID, pkt.Destination)
		}
		if !strings.Contains(err.Error(), dest.Identity) {
			t.Errorf("the refusal for %q does not name the identity a transmission would carry", dest.ID)
		}
		if !strings.Contains(err.Error(), dest.Authority) {
			t.Errorf("the refusal for %q does not name the authority a transmission would require", dest.ID)
		}
	}

	// Non-vacuity, both ways. Without these, deleting every transmitting
	// destination from the register would make this test pass.
	if transmitting < 3 {
		t.Fatalf("the register declares %d transmitting destinations — this control is near-vacuous below 3", transmitting)
	}
	if transmittingAndPresent < 1 {
		t.Fatal("no transmitting destination is PRESENT in this tree, so this control only proves that absent things cannot be reached. At least one must be a tool that actually exists and is refused anyway")
	}

	// The control in the other direction: a non-transmitting, present
	// destination must SUCCEED, or "refuse everything" would satisfy the test.
	pkt, err := d.HandOff("operator-render")
	if err != nil {
		t.Fatalf("HandOff(operator-render) = %v, want success — a layer that refuses every exit has no exit, and a draft nobody can read is not a draft", err)
	}
	if pkt.Zero() || !strings.Contains(pkt.Body, DraftBanner) {
		t.Fatalf("the permitted hand-off produced no usable packet: %+v", pkt)
	}

	// And an undeclared destination is default-denied rather than passed
	// through.
	if _, err := d.HandOff("some-new-route"); !errors.Is(err, ErrUnknownDestination) {
		t.Errorf("an undeclared destination returned %v, want ErrUnknownDestination", err)
	}
}

// TestAuthorityPresenceNeverGrants proves the order of checks in HandOff. The
// substrate for two transmitting destinations is absent today. If the refusal
// depended on that absence, the invariant would expire the day desk-console/10
// lands. So: flip Present to true on a transmitting destination and confirm
// the refusal is unchanged.
func TestAuthorityPresenceNeverGrants(t *testing.T) {
	var idx = -1
	for i := range destinations {
		if destinations[i].Transmits && !destinations[i].Present {
			idx = i
			break
		}
	}
	if idx < 0 {
		t.Fatal("no transmitting-and-absent destination to mutate — this control needs one")
	}
	orig := destinations[idx].Present
	destinations[idx].Present = true
	t.Cleanup(func() { destinations[idx].Present = orig })

	d := mustCompose(t)
	pkt, err := d.HandOff(destinations[idx].ID)
	if !errors.Is(err, ErrNoAutoPost) {
		t.Fatalf("with its substrate present, %q returned %v — the refusal depended on absence, which means it expires the day the substrate lands", destinations[idx].ID, err)
	}
	if !pkt.Zero() {
		t.Fatal("a refused hand-off produced a packet")
	}
}

// TestAuthorityNoAuthorizationTypeExists holds the design decision that this
// package ships no way to represent a granted authority. The sibling ruling
// this follows: dormancy is fine for a mechanism whose worst case is a wrong
// recommendation; it is not fine for one whose worst case is an unattended
// post, because the gap between a flag and a grant is one commit.
func TestAuthorityNoAuthorizationTypeExists(t *testing.T) {
	shapes := []string{
		"type Authorization", "type Authorized", "type Grant", "type Approval",
		"type Consent", "type Token", "type Credential", "Authorize(", "Approve(",
	}
	for name, src := range shippedSources(t) {
		for _, s := range shapes {
			if strings.Contains(src, s) {
				t.Errorf("%s declares %q — an authorization value plus a transmitting code path is a post waiting on a flag flip; this package has neither and must keep neither", name, s)
			}
		}
	}
	// Positive control: the matcher must see the shape it forbids.
	if !strings.Contains("type Authorization struct{}", "type Authorization") {
		t.Fatal("POSITIVE CONTROL FAILED: the authorization-shape matcher does not match an authorization shape")
	}
}

// TestAuthorityTransitiveClosureHoldsNoIOPackage is the half a scan of this
// directory cannot see: an allowed import could itself import os/exec. This is
// why the package accepts a [Claim] interface instead of importing the answer
// layer — that layer runs guarded subprocesses, so importing it would have put
// os/exec in this closure and the check below would be red.
func TestAuthorityTransitiveClosureHoldsNoIOPackage(t *testing.T) {
	cmd := exec.Command("go", "list", "-deps", "./internal/draft")
	cmd.Dir = moduleRootDir(t)
	out, err := cmd.Output()
	if err != nil {
		// Not skipped: a missing toolchain here would render as a pass, and a
		// check that cannot fail is not a check.
		t.Fatalf("go list -deps failed, so this property is unproven rather than satisfied: %v", err)
	}
	deps := strings.Fields(string(out))
	if len(deps) < 5 {
		t.Fatalf("go list reported %d dependencies — too few to be a real closure, so this scan is blind", len(deps))
	}
	// THE BOUND ON THIS CHECK, STATED.
	// `os` and `syscall` are in the closure and cannot be got out of one: the
	// standard library's `fmt` imports `os`, and `time` reaches `syscall`. Any
	// Go package that can format a string has them. Banning them here would
	// have made this check red on day one and it would have been deleted, so
	// the ban is the set that is genuinely absent and genuinely acquirable —
	// the transports. The arm that does exclude `os` is the DIRECT-import
	// allow-list above, which is why that one is primary and this one is
	// corroborating: this package cannot NAME os, and cannot reach any
	// transport even transitively.
	banned := map[string]string{
		"os/exec":  "starts subprocesses",
		"net":      "opens sockets",
		"net/http": "makes HTTP requests",
		"net/rpc":  "calls out of process",
		"net/smtp": "sends mail",
		"net/url":  "is only ever here to build a request target",
	}
	for _, d := range deps {
		if why, bad := banned[d]; bad {
			t.Errorf("the transitive closure of this package includes %q, which %s — the no-post claim rests on this closure, not on this directory alone", d, why)
		}
	}
	// Non-vacuity, three ways. The closure must contain this package's own
	// declared imports, must contain the package it is bound to, and must
	// contain `os` — the last of those is what proves the scan is reading a
	// real transitive closure and not a short list it happens to agree with.
	seen := map[string]bool{}
	for _, d := range deps {
		seen[d] = true
	}
	for _, must := range []string{"strings", "crypto/sha256", "regexp", "os",
		"github.com/medici-finance/assay/tools/desk/internal/draft"} {
		if !seen[must] {
			t.Errorf("the reported closure does not contain %q — go list resolved a different target, so this scan proved nothing", must)
		}
	}
}

func moduleRootDir(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for i := 0; i < 10; i++ {
		if _, serr := os.Stat(filepath.Join(dir, "go.mod")); serr == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatal("no module root above the package directory")
	return ""
}

// TestAuthorityPresentFlagsMatchTheTree holds the register's measurements true.
// Present is a claim about this tree; a claim nobody re-measures is a claim
// that silently rots. Each probe below is the measurement itself, not a
// restatement of it.
func TestAuthorityPresentFlagsMatchTheTree(t *testing.T) {
	root := moduleRootDir(t)
	probes := map[string]func() (bool, string){
		"operator-render": func() (bool, string) {
			// Present iff this package renders. Read the shipped source for
			// the method rather than calling it, so the probe measures the
			// tree the way the others do.
			b, err := os.ReadFile("draft.go")
			return err == nil && strings.Contains(string(b), "func (d Draft) Render() string"), "draft.go declares Draft.Render"
		},
		"issue-comment-reply": func() (bool, string) {
			_, err := os.Stat(filepath.Join(root, "cmd", "deskreply"))
			return err == nil, "cmd/deskreply exists in this module"
		},
		"pr-comment": func() (bool, string) {
			_, err := os.Stat(filepath.Join(root, "cmd", "deskpost"))
			return err == nil, "cmd/deskpost exists in this module"
		},
		"inbox-answer-commit": func() (bool, string) {
			return anyDirExists(root, "internal/chat", "internal/console", "internal/inbox"), "a console/inbox package exists in this module"
		},
		"priority-steering-write": func() (bool, string) {
			return anyDirExists(root, "internal/steering", "internal/priority"), "a steering package exists in this module"
		},
		"chat-transcript-persist": func() (bool, string) {
			return anyDirExists(root, "internal/chat", "internal/draftstore"), "a draft store exists in this module"
		},
	}
	all := Destinations()
	if len(all) != len(probes) {
		t.Fatalf("the register declares %d destinations and this test probes %d — a destination with no probe is an unmeasured claim", len(all), len(probes))
	}
	for _, d := range all {
		probe, ok := probes[d.ID]
		if !ok {
			t.Errorf("destination %q has no measurement probe", d.ID)
			continue
		}
		got, what := probe()
		if got != d.Present {
			t.Errorf("destination %q declares Present=%v but the tree says %v (%s)", d.ID, d.Present, got, what)
		}
	}
}

func anyDirExists(root string, rel ...string) bool {
	for _, r := range rel {
		if fi, err := os.Stat(filepath.Join(root, filepath.FromSlash(r))); err == nil && fi.IsDir() {
			return true
		}
	}
	return false
}

// TestAuthorityUnsupportedTargetsNamesEveryOne — no silent caps on the
// hand-off surface. A caller must be able to enumerate what this layer will
// not do, by name and with the reason, rather than discovering it as an error
// at the moment it matters.
func TestAuthorityUnsupportedTargetsNamesEveryOne(t *testing.T) {
	got := UnsupportedTargets()
	var want int
	for _, d := range Destinations() {
		if d.Transmits || !d.Present {
			want++
		}
	}
	if len(got) != want {
		t.Fatalf("UnsupportedTargets listed %d of %d unsupported destinations", len(got), want)
	}
	if want == 0 {
		t.Fatal("nothing is unsupported, which would mean this layer supports posting")
	}
	for _, d := range Destinations() {
		if !d.Transmits && d.Present {
			continue
		}
		var found bool
		for _, line := range got {
			if strings.HasPrefix(line, d.ID+" ") {
				found = true
			}
		}
		if !found {
			t.Errorf("%q is unsupported and is not named in UnsupportedTargets — that is a silent cap", d.ID)
		}
	}
}

// TestAuthorityEveryDestinationStatesItsIdentityAndAuthority — a hand-off
// whose identity or authority is blank is a hand-off nobody can review. Tool
// identity in this suite is NOT uniform: the two present posting routes carry
// two different App roles.
func TestAuthorityEveryDestinationStatesItsIdentityAndAuthority(t *testing.T) {
	identities := map[string]bool{}
	for _, d := range Destinations() {
		if strings.TrimSpace(d.Identity) == "" {
			t.Errorf("%q states no identity", d.ID)
		}
		if strings.TrimSpace(d.Authority) == "" {
			t.Errorf("%q states no authority requirement", d.ID)
		}
		if strings.TrimSpace(d.Substrate) == "" {
			t.Errorf("%q names no substrate", d.ID)
		}
		if strings.TrimSpace(d.Note) == "" {
			t.Errorf("%q carries no note", d.ID)
		}
		if d.Transmits {
			identities[d.Identity] = true
		}
	}
	if len(identities) < 2 {
		t.Fatalf("the transmitting destinations declare %d distinct identities — the identity split is the finding, and a register that collapses it is wrong", len(identities))
	}
	// AuthorityRequired must answer for every declared destination and refuse
	// for an undeclared one.
	for _, d := range Destinations() {
		if _, ok := AuthorityRequired(d.ID); !ok {
			t.Errorf("AuthorityRequired(%q) reports unknown", d.ID)
		}
	}
	if _, ok := AuthorityRequired("not-a-destination"); ok {
		t.Error("AuthorityRequired answered for an undeclared destination")
	}
}

// TestAuthorityNoKillSwitchOfTheConcatenatedShape — this package holds no stop
// flag, and the reason is measured next door: internal/deskkit/killswitch.go
// builds a per-loop flag name by concatenating an environment variable onto a
// prefix with no allow-list, so a rename leaves a held stop flag inert with
// nothing failing loudly. A layer that cannot act needs no flag; if one is
// ever added here it must not be that shape.
func TestAuthorityNoKillSwitchOfTheConcatenatedShape(t *testing.T) {
	// Line comments are stripped first: the package header CITES the
	// neighbouring instance by name, and a scanner that trips on the prose
	// explaining it is a scanner nobody keeps.
	for name, src := range shippedSources(t) {
		code := stripLineComments(src)
		for _, s := range []string{"STOP.", "Getenv", "LookupEnv", "killswitch", "KillSwitch"} {
			if strings.Contains(code, s) {
				t.Errorf("%s names %q in code — this layer holds no stop flag, and an env-concatenated one is the shape that goes silently inert on a rename", name, s)
			}
		}
	}
	// Positive control on the stripper, both directions: a violation in a
	// comment is not a violation, and one in code still is.
	if strings.Contains(stripLineComments(`// killswitch`), "killswitch") {
		t.Error("comment stripping failed — the package header's own citation would be reported")
	}
	if !strings.Contains(stripLineComments("x := \"killswitch\" // a comment"), "killswitch") {
		t.Error("comment stripping removed real code — a real stop flag would go unreported")
	}
	// Positive control, including proof the neighbouring instance is real: the
	// concatenation this test exists to avoid is still in the tree.
	root := moduleRootDir(t)
	b, err := os.ReadFile(filepath.Join(root, "internal", "deskkit", "killswitch.go"))
	if err != nil {
		t.Fatalf("the cited neighbouring file could not be read, so the reason for this test is unverified: %v", err)
	}
	if !strings.Contains(string(b), `"STOP."+name`) {
		t.Error("the concatenated stop-flag shape this test cites is no longer in killswitch.go — re-point or delete this rationale rather than leaving a stale citation")
	}
}

// stripLineComments removes text after a `//` on each line. It is used by the
// scans whose own rationale prose names the tokens they forbid.
func stripLineComments(src string) string {
	var b strings.Builder
	for _, line := range strings.Split(src, "\n") {
		code := line
		if i := strings.Index(code, "//"); i >= 0 {
			code = code[:i]
		}
		b.WriteString(code)
		b.WriteByte('\n')
	}
	return b.String()
}

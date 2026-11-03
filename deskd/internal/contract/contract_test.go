package contract

import (
	"fmt"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

// frozenNamedActions is the allow-list as the human gate ratified it
// (the service contract, option 1 on the decision issue: "NamedActions = exactly
// {answer, ack, send, session} … allow-list frozen behind tests"). It is a
// SECOND literal on purpose: growing [NamedActions] and [Routes] together would
// keep the route-vs-literal compare green, so the literal itself is pinned
// here, and a fifth action is a test edit a reviewer sees — never a one-line
// addition to contract.go.
var frozenNamedActions = []Action{ActionAnswer, ActionAck, ActionSend, ActionSession}

// deleteRoute is the ONE route that may use DELETE: closing a custom terminal
// session. No other resource grows a delete.
const deleteRoute = Prefix + "/sessions/{id}"

// TestNamedActionSetIsClosed is the custody-(b) closed-set rule as a build
// check: the human-authored write routes declared in Routes must equal the
// enumerated NamedActions literal, and the literal must equal the ratified
// four. Adding a route typed as a human write without adding it to the literal
// (or vice versa), or growing both together, fails here, so growing the
// surface is a deliberate contract change, never a config toggle.
func TestNamedActionSetIsClosed(t *testing.T) {
	if !sameActionSet(NamedActions, frozenNamedActions) {
		t.Fatalf("NamedActions = %v, but the ratified allow-list is exactly %v — the set is frozen behind this test "+
			"(the service contract); a change to it is a new human gate, not an edit", NamedActions, frozenNamedActions)
	}
	if v := closedSetViolations(Routes, NamedActions); len(v) > 0 {
		t.Fatalf("the closed named-action set is violated:\n  %s", strings.Join(v, "\n  "))
	}
}

// closedSetViolations is the whole closed-set rule over a route table: the
// deduplicated named actions the table declares must equal allow; every
// named-action route must name an allow-listed action and use POST — except
// DELETE on exactly [deleteRoute] for `session`; and no other route may carry
// an Action (a mislabelled route is a smuggled write). It returns one message
// per violation so the positive controls below can prove each clause bites.
func closedSetViolations(routes []Route, allow []Action) []string {
	var out []string
	declared := humanWriteActions(routes)
	if !sameActionSet(declared, allow) {
		out = append(out, fmt.Sprintf("human-authored write routes declare %v, but the enumerated allow-list is %v — "+
			"a human write route outside the allow-list (or an allow-list entry with no route) is a defect; "+
			"the named-action set is CLOSED (custody rule (b))", declared, allow))
	}
	allowed := map[Action]bool{}
	for _, a := range allow {
		allowed[a] = true
	}
	for _, r := range routes {
		switch r.Kind {
		case KindNamedAction:
			if !allowed[r.Action] {
				out = append(out, fmt.Sprintf("route %s %s is a named action but %q is not in the allow-list", r.Method, r.Path, r.Action))
			}
			switch {
			case r.Method == POST:
			case r.Method == DELETE && r.Path == deleteRoute && r.Action == ActionSession:
			default:
				out = append(out, fmt.Sprintf("named-action route %s %s must be POST (the one DELETE is %s for %q), got %s",
					r.Method, r.Path, deleteRoute, ActionSession, r.Method))
			}
		default:
			if r.Action != "" {
				out = append(out, fmt.Sprintf("route %s %s (kind %s) names action %q — only named-action routes may name an action", r.Method, r.Path, r.Kind, r.Action))
			}
			if r.Method == DELETE {
				out = append(out, fmt.Sprintf("route %s %s (kind %s) uses DELETE — the only DELETE in the contract is %s", r.Method, r.Path, r.Kind, deleteRoute))
			}
		}
	}
	return out
}

// TestNamedActionSetIsClosedCatchesAStrayWrite is the positive control: a route
// set that DID contain a human write outside the allow-list must be rejected by
// sameActionSet, so the check above is load-bearing.
func TestNamedActionSetIsClosedCatchesAStrayWrite(t *testing.T) {
	strayed := append(HumanWriteActions(), Action("close"))
	if sameActionSet(strayed, NamedActions) {
		t.Fatal("sameActionSet accepted a stray 'close' write outside the allow-list — the closed-set check is not load-bearing")
	}
}

// TestNamedActionSetIsClosedCatchesAFifthAction is the fifth-action control
// (the service contract Verify row 1): a fifth named action fails the rule every
// way it could arrive — as a route outside the literal, as a literal entry
// with no route, grown into BOTH together (the frozen literal catches that),
// or as a second DELETE — so the four-action freeze is load-bearing.
func TestNamedActionSetIsClosedCatchesAFifthAction(t *testing.T) {
	fifth := Action("exec")
	withRoute := append(append([]Route(nil), Routes...), Route{POST, Prefix + "/desks/{id}/terminal/exec", KindNamedAction, fifth, "a fifth action"})
	if v := closedSetViolations(withRoute, NamedActions); len(v) == 0 {
		t.Error("a fifth named-action ROUTE outside the literal passed the closed-set rule")
	}
	grownLiteral := append(append([]Action(nil), NamedActions...), fifth)
	if v := closedSetViolations(Routes, grownLiteral); len(v) == 0 {
		t.Error("a fifth LITERAL entry with no route passed the closed-set rule")
	}
	if v := closedSetViolations(withRoute, grownLiteral); len(v) != 0 {
		t.Fatalf("the route-vs-literal rule should agree once both grew (the freeze below is what catches that): %v", v)
	}
	if sameActionSet(grownLiteral, frozenNamedActions) {
		t.Error("a literal grown to five matched the frozen four — the freeze does not bite")
	}
	strayDelete := append(append([]Route(nil), Routes...), Route{DELETE, Prefix + "/desks/{id}/terminal", KindNamedAction, ActionSend, "a second delete"})
	if v := closedSetViolations(strayDelete, NamedActions); len(v) == 0 {
		t.Error("a second DELETE route passed the closed-set rule — the one-DELETE rule does not bite")
	}
}

func sameActionSet(a, b []Action) bool {
	if len(a) != len(b) {
		return false
	}
	seen := map[Action]int{}
	for _, x := range a {
		seen[x]++
	}
	for _, x := range b {
		seen[x]--
	}
	for _, n := range seen {
		if n != 0 {
			return false
		}
	}
	return true
}

// credentialShapedNames are the field name / json-tag substrings that a wire
// body must never carry: the per-request credential rides the Authorization
// header, and the signed intent carries a signature over a record, not a token.
var credentialShapedNames = []string{"token", "secret", "password", "credential", "authorization"}

// TestNoCredentialShapedFields walks every exported wire type (WireTypes) by
// reflection and fails on any field whose name or json tag looks like a
// credential. This is the contract-level half of custody (b): the shape has no
// place to persist a human credential in a body. "signature" is explicitly
// allowed — a signature over a record is not a credential.
func TestNoCredentialShapedFields(t *testing.T) {
	for _, v := range WireTypes() {
		walkForCredentialFields(t, reflect.TypeOf(v), map[reflect.Type]bool{})
	}
}

func walkForCredentialFields(t *testing.T, typ reflect.Type, seen map[reflect.Type]bool) {
	t.Helper()
	for typ.Kind() == reflect.Ptr || typ.Kind() == reflect.Slice || typ.Kind() == reflect.Array {
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Struct || seen[typ] {
		return
	}
	seen[typ] = true
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		tag := strings.ToLower(strings.Split(f.Tag.Get("json"), ",")[0])
		name := strings.ToLower(f.Name)
		for _, bad := range credentialShapedNames {
			if strings.Contains(name, bad) || strings.Contains(tag, bad) {
				t.Errorf("%s.%s (json:%q) is credential-shaped (%q): a wire body must never carry a credential — "+
					"the per-request credential rides the Authorization header; the signed intent carries a signature over a record",
					typ.Name(), f.Name, f.Tag.Get("json"), bad)
			}
		}
		walkForCredentialFields(t, f.Type, seen)
	}
}

// targetAddressNames are field name / json-tag substrings that would let a
// client NAME or SELECT a target — a container name, a pod/namespace, a local
// command line, a host, a tmux pane. A wire type must carry none of them: the
// driver a desk id resolves to is chosen by the registry from the parsed
// cells.yaml `target:`, NEVER by the caller (spec §5.1, SPOF layer (a)). If a
// client could pass a target in a body, it could attach to any container on
// the host.
//
// `target_kind` is DELIBERATELY not here: it is a CATEGORY the service derives
// and returns (container|pod|local), not an address a client sends — the same
// way `state` is derived, not sent. Only address-shaped names are forbidden.
var targetAddressNames = []string{"container", "namespace", "argv", "selector", "pane", "targetname", "targetref", "targethost", "targetcommand"}

// TestNoTargetShapedRequest walks every exported wire type (WireTypes) by
// reflection and fails on any field whose name or json tag could carry a
// target NAME/ADDRESS. It is the contract-level half of SPOF layer (a): the
// wire shape has no place a client could name the target it wants to attach
// to, so the registry — not the client — is the only thing that chooses a
// driver. It recurses only into CONTRACT structs (a target selector would be a
// contract field; domain content is out of scope), which also keeps the
// address substrings from false-matching unrelated domain fields.
func TestNoTargetShapedRequest(t *testing.T) {
	for _, v := range WireTypes() {
		for _, f := range targetShapeFindings(reflect.TypeOf(v), map[reflect.Type]bool{}) {
			t.Error(f)
		}
	}
}

// targetShapeFindings returns one message per field that could name/select a
// target. It recurses only into CONTRACT structs (a target selector would be a
// contract field; domain content is out of scope), which also keeps the
// address substrings from false-matching unrelated domain fields.
func targetShapeFindings(typ reflect.Type, seen map[reflect.Type]bool) []string {
	for typ.Kind() == reflect.Ptr || typ.Kind() == reflect.Slice || typ.Kind() == reflect.Array {
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Struct || seen[typ] {
		return nil
	}
	seen[typ] = true
	var out []string
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		tag := strings.ToLower(strings.Split(f.Tag.Get("json"), ",")[0])
		name := strings.ToLower(f.Name)
		// The one allowed field carrying "target": the derived category label.
		isKind := name == "targetkind" || tag == "target_kind"
		if !isKind && (strings.Contains(name, "target") || strings.Contains(tag, "target")) {
			out = append(out, fmt.Sprintf("%s.%s (json:%q) is target-shaped: a wire body must never carry a field a client could use to NAME a target — "+
				"the driver a desk id resolves to is chosen by the registry from the parsed cells.yaml target:, never by the caller (spec §5.1). "+
				"target_kind (a derived category) is the only permitted 'target' field.", typ.Name(), f.Name, f.Tag.Get("json")))
		}
		for _, bad := range targetAddressNames {
			if strings.Contains(name, bad) || strings.Contains(tag, bad) {
				out = append(out, fmt.Sprintf("%s.%s (json:%q) is target-address-shaped (%q): a client must not be able to name a container/pod/namespace/command — "+
					"the registry resolves the target from cells.yaml, not the request (spec §5.1, SPOF (a))",
					typ.Name(), f.Name, f.Tag.Get("json"), bad))
			}
		}
		// Recurse only into contract structs (skip domain and time.Time).
		ft := f.Type
		for ft.Kind() == reflect.Ptr || ft.Kind() == reflect.Slice || ft.Kind() == reflect.Array {
			ft = ft.Elem()
		}
		if ft.Kind() == reflect.Struct && strings.HasSuffix(ft.PkgPath(), "/internal/contract") {
			out = append(out, targetShapeFindings(ft, seen)...)
		}
	}
	return out
}

// TestTargetShapeCheckLoadBearing is the positive control: a
// wire type that DID carry a target-address field must be rejected, so the
// walk above is load-bearing rather than vacuous.
func TestTargetShapeCheckLoadBearing(t *testing.T) {
	type badRequest struct {
		Container string `json:"container"`
	}
	if findings := targetShapeFindings(reflect.TypeOf(badRequest{}), map[reflect.Type]bool{}); len(findings) == 0 {
		t.Fatal("targetShapeFindings accepted a field named 'container' — the target-shape check is not load-bearing")
	}
}

// TestLeafImports asserts the contract package is a LEAF with respect to the
// identity-bearing packages: neither the service nor a client can be dragged
// toward the other's identity by importing the contract. go list -deps is the
// real transitive graph, not a grep, so a leak three imports deep is caught.
func TestLeafImports(t *testing.T) {
	forbidden := []string{
		"github.com/medici-finance/assay/deskd/internal/answer",
		"github.com/medici-finance/assay/deskd/internal/custody",
		"github.com/medici-finance/assay/deskd/internal/mcp",
	}
	deps := goListDeps(t)
	for _, f := range forbidden {
		if deps[f] {
			t.Errorf("internal/contract transitively imports %s — the contract must be a leaf both binaries import, "+
				"so importing it never drags one toward the other's identity", f)
		}
	}
	// Positive control: the package DOES reach internal/domain, so the check is
	// not vacuous on an empty/mis-parsed dep set.
	if !deps["github.com/medici-finance/assay/deskd/internal/domain"] {
		t.Fatal("internal/contract does not reach internal/domain — the dep set is empty or mis-parsed, which would make the leaf check vacuous")
	}
}

func goListDeps(t *testing.T) map[string]bool {
	t.Helper()
	cmd := exec.Command("go", "list", "-deps", ".")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps failed: %v\n%s", err, out)
	}
	deps := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		deps[strings.TrimSpace(line)] = true
	}
	return deps
}

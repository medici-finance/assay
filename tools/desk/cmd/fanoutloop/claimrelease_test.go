package main

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

// claimrelease_test.go — the two properties of the claim RELEASE that no single-component
// test could see before: that the sink obtains its forge from the resolver rather than
// holding a nil one (the claim-layer brief's Verify row 5), and that a release which did not
// happen is never reported as one (row 6).
//
// Both are about a failure mode that is invisible on the happy path. A claim leaks silently:
// nothing errors, no line is printed, the item simply never becomes dispatchable again, and
// the only observable is a slot that stopped coming back — days later, on another machine.
// So these tests assert the NEGATIVE half explicitly: what must NOT be returned, and what
// must NOT be printed.

// staticResolver adapts a single Forge into a resolver, for the cases where WHICH repo
// resolved is not the property under test.
func staticResolver(f deskkit.Forge) forgeResolver {
	return func(deskkit.ForgeRepo) (deskkit.Forge, error) { return f, nil }
}

// --- row 5: the sink is constructed from the resolver -----------------------------------

// TestSinkResolvesForge pins that the PRODUCTION path yields a sink whose forge comes from
// the resolver, and that no path yields one holding nothing.
//
// The disease this closes is specific and was load bearing: newForgeDispatchSink's only
// caller was a test, the production field was nil, and the sink's own answer to that was a
// runtime error string. A release path that is dead in production is not a release path —
// it is a claim leak with a unit test.
func TestSinkResolvesForge(t *testing.T) {
	// 1. The default (nothing injected, not dry-run) IS the releasing sink, and it carries a
	//    resolver. A dryRunSink here would mean the production path still releases nothing.
	loop := &FanoutLoop{}
	s, err := loop.sink()
	if err != nil {
		t.Fatalf("the production sink could not be constructed: %v", err)
	}
	fds, ok := s.(*forgeDispatchSink)
	if !ok {
		t.Fatalf("the production sink is %T, not the releasing forgeDispatchSink — the claim would never "+
			"be released in production", s)
	}
	if fds.resolve == nil {
		t.Fatal("the production sink holds no forge resolver — this is the nil-forge state in a new shape")
	}

	// 2. The dry-run sink is reached only by ASKING for it, never as a fallback.
	dry, derr := (&FanoutLoop{DryRun: true}).sink()
	if derr != nil {
		t.Fatalf("dry-run sink: %v", derr)
	}
	if _, ok := dry.(dryRunSink); !ok {
		t.Fatalf("DryRun did not select the printing sink, got %T", dry)
	}

	// 3. A sink cannot be built without a resolver at all: the nil case is refused at
	//    construction rather than surviving to become a runtime message.
	if _, err := newForgeDispatchSink(io.Discard, nil); err == nil {
		t.Fatal("newForgeDispatchSink accepted a nil resolver — a sink that cannot obtain a forge must " +
			"not be constructible")
	}

	// 4. The resolver is asked for the ITEM's target repo, not for some repo the loop was
	//    booted under: a batch spans repos, and deleting the claim in the wrong project
	//    reports a release that freed nothing.
	var asked []deskkit.ForgeRepo
	rec := &recordingForge{}
	spy, serr := newForgeDispatchSink(io.Discard, func(r deskkit.ForgeRepo) (deskkit.Forge, error) {
		asked = append(asked, r)
		return rec, nil
	})
	if serr != nil {
		t.Fatalf("newForgeDispatchSink: %v", serr)
	}
	it := loopengine.Item{ID: "other/07", Payload: map[string]string{"repo": "other-owner/other-repo"}}
	if err := spy.ReleaseDispatchClaim(it); err != nil {
		t.Fatalf("ReleaseDispatchClaim: %v", err)
	}
	want := deskkit.ForgeRepo{Owner: "other-owner", Name: "other-repo"}
	if len(asked) != 1 || asked[0] != want {
		t.Fatalf("the resolver was asked for %+v, want exactly one ask for %+v", asked, want)
	}
	if rec.repo != want {
		t.Fatalf("the delete addressed %+v, want %+v", rec.repo, want)
	}
}

// TestSinkNamespaceIsTheSharedConstant pins that this command's namespace constant — the one
// Verify row 8 reads every other mention of the namespace against — is DERIVED from the
// single definition in deskkit rather than being a second spelling of it.
func TestSinkNamespaceIsTheSharedConstant(t *testing.T) {
	if dispatchRefNamespace != deskkit.ClaimRefNamespace {
		t.Fatalf("the sink's namespace %q is not deskkit's %q — a writer and a reader that disagree about "+
			"where a claim lives is the double-dispatch failure",
			dispatchRefNamespace, deskkit.ClaimRefNamespace)
	}
	ref, err := deskkit.ClaimRefPath("repo--stream--01")
	if err != nil {
		t.Fatalf("ClaimRefPath: %v", err)
	}
	if !strings.HasPrefix("refs/"+ref, deskkit.ClaimRefsPrefix) {
		t.Fatalf("the claim ref %q does not sit under the listed prefix %q", ref, deskkit.ClaimRefsPrefix)
	}
}

// --- row 6: a failed release is never reported as a release ------------------------------

// failingForge answers DeleteRef with a fixed error and records that it was asked.
type failingForge struct {
	deskkit.Forge // embedded nil — any other method panics
	err           error
	calls         int
}

func (f *failingForge) DeleteRef(deskkit.ForgeRepo, string) error {
	f.calls++
	return f.err
}

// TestReleaseFailureIsNotReportedReleased is the negative path the whole brief turns on.
//
// A leaked claim must be LOUD. For each way the release can fail, the sink must (a) return a
// non-nil error, (b) name the claim key in it so the operator knows which slot is stuck, and
// (c) print NOTHING — a "released" line on a failed release is a false record that survives
// in a log after the error scrolls away.
func TestReleaseFailureIsNotReportedReleased(t *testing.T) {
	const key = "fixture--02" // claimKey("fixture/02")
	it := loopengine.Item{ID: "fixture/02", Payload: map[string]string{"repo": "medici-finance/assay"}}

	cases := []struct {
		name    string
		resolve forgeResolver
	}{
		{
			// The forge refused the delete. A 403 is the dangerous one: it looks like a
			// transient and is not — the ref is still there.
			name: "delete refused by the forge",
			resolve: staticResolver(&failingForge{
				err: &deskkit.ForgeAPIError{Status: http.StatusForbidden, Method: "DELETE", Path: "/x"},
			}),
		},
		{
			// The credential expired mid-batch. Nothing was deleted.
			name: "delete refused for authentication",
			resolve: staticResolver(&failingForge{
				err: &deskkit.ForgeAPIError{Status: http.StatusUnauthorized, Method: "DELETE", Path: "/x"},
			}),
		},
		{
			// The forge served the namespace with a could-not-check — the exact shape a
			// backend that cannot delete this ref returns. It is NOT a release.
			name: "backend cannot serve the namespace",
			resolve: staticResolver(&failingForge{
				err: deskkit.Unverifiable("could-not-check: this backend cannot delete that ref", nil),
			}),
		},
		{
			// No forge could be resolved for the repo at all.
			name: "resolver refuses",
			resolve: func(deskkit.ForgeRepo) (deskkit.Forge, error) {
				return nil, deskkit.Refused("no custody file for this role")
			},
		},
		{
			// The resolver answered with neither a forge nor an error. Nothing may be
			// inferred from that — least of all that the claim is free.
			name: "resolver yields nothing at all",
			resolve: func(deskkit.ForgeRepo) (deskkit.Forge, error) {
				return nil, nil
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			s, serr := newForgeDispatchSink(&out, tc.resolve)
			if serr != nil {
				t.Fatalf("newForgeDispatchSink: %v", serr)
			}
			err := s.ReleaseDispatchClaim(it)
			if err == nil {
				t.Fatal("a release that did not happen was reported as success — the slot leaks silently")
			}
			if !strings.Contains(err.Error(), key) {
				t.Fatalf("the failure does not name the claim key %q, so the stuck slot is unidentifiable: %v",
					key, err)
			}
			if out.Len() != 0 {
				t.Fatalf("a failed release still emitted output %q — no record of a release may survive a "+
					"release that did not occur", out.String())
			}
		})
	}

	// The mirror image, so the assertions above are not vacuously satisfied by a sink that
	// never prints: a release that DID happen is recorded, names the ref and the repo, and
	// returns nil.
	var out bytes.Buffer
	ok, serr := newForgeDispatchSink(&out, staticResolver(&recordingForge{}))
	if serr != nil {
		t.Fatalf("newForgeDispatchSink: %v", serr)
	}
	if err := ok.ReleaseDispatchClaim(it); err != nil {
		t.Fatalf("a successful release must return nil: %v", err)
	}
	line := out.String()
	for _, must := range []string{"released", deskkit.ClaimRefsPrefix + key, "medici-finance/assay"} {
		if !strings.Contains(line, must) {
			t.Fatalf("the success record %q does not carry %q", line, must)
		}
	}
}

// TestReleaseAlreadyGoneIsNotAFailure keeps the no-op half honest while row 6 tightens the
// failure half: a claim someone else already released is not an error, and it is not
// announced either — nothing happened.
func TestReleaseAlreadyGoneIsNotAFailure(t *testing.T) {
	it := loopengine.Item{ID: "fixture/02", Payload: map[string]string{"repo": "medici-finance/assay"}}
	for _, gone := range []error{
		&deskkit.ForgeAPIError{Status: http.StatusNotFound, Method: "DELETE", Path: "/x"},
		&deskkit.ForgeAPIError{Status: http.StatusUnprocessableEntity, Method: "DELETE", Path: "/x"},
	} {
		var out bytes.Buffer
		s, serr := newForgeDispatchSink(&out, staticResolver(&failingForge{err: gone}))
		if serr != nil {
			t.Fatalf("newForgeDispatchSink: %v", serr)
		}
		if err := s.ReleaseDispatchClaim(it); err != nil {
			t.Fatalf("an already-released claim must be a no-op, got %v", err)
		}
		if out.Len() != 0 {
			t.Fatalf("an already-gone claim announced a release it did not perform: %q", out.String())
		}
	}
	// And the sentinel test itself must be meaningful: refAlreadyGone must NOT swallow a
	// could-not-check, which carries no status at all.
	if refAlreadyGone(deskkit.Unverifiable("could-not-check", nil)) {
		t.Fatal("a could-not-check was treated as 'the ref is already gone' — that is the fail-open")
	}
	if refAlreadyGone(errors.New("connection reset")) {
		t.Fatal("a transport error was treated as 'the ref is already gone'")
	}
}

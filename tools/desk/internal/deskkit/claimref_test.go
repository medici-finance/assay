package deskkit

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"testing"
)

// claimref_test.go — the claim layer's round trip and its guard.
//
// The claim ref is a distributed mutual-exclusion primitive, and its two failure modes are
// both silent. A claim that cannot be RELEASED never frees its slot; a claim released
// against a namespace nobody lists frees a slot that was never held. Neither shows up in a
// happy-path test of either backend on its own, which is why the round trip below drives
// BOTH backends through the SAME scenario list against recorded fixtures: a gap between them
// reads as a named failing scenario rather than as silence.
//
// What is fixture-recorded here and what is not. Taking and listing a claim is plain git —
// a push and an `ls-remote`, forge-neutral by construction — so the fixture models the ref
// STORE those operations act on, and the forge's part of the lifecycle (the delete, the only
// half that has ever needed a forge at all) runs against the recorded HTTP surface of each
// backend. The assertion that matters is post-condition, not request shape: after the
// delete, the store must no longer list the ref. A backend that answered 200 and left the
// ref in place would pass a request-shape check and leak every slot.

// --- the ref store both fixtures model ---------------------------------------------------

// refStore is the fixture's repository refs. It is deliberately a plain map keyed by the
// fully-qualified ref name — the same thing `git ls-remote` prints — so "create" and "list"
// are what they are in production (plain git), and only the delete travels over a backend.
type refStore struct {
	refs map[string]string // refs/heads/... -> sha
}

func newRefStore() *refStore { return &refStore{refs: map[string]string{}} }

// create is the claim-taking half: `git push origin <sha>:<ref>`.
func (s *refStore) create(ref, sha string) { s.refs[ref] = sha }

// list is the claim-reading half: `git ls-remote origin '<pattern>'`, reduced to the claim
// namespace.
func (s *refStore) listClaims() []string {
	var out []string
	for name := range s.refs {
		if key, ok := ClaimKeyFromRef(name); ok {
			out = append(out, key)
		}
	}
	sort.Strings(out)
	return out
}

func (s *refStore) has(ref string) bool { _, ok := s.refs[ref]; return ok }

// --- the two recorded backends ------------------------------------------------------------

// githubRefServer records GitHub's git-data ref API: DELETE /repos/{o}/{r}/git/refs/{ref},
// which serves any namespace. It removes the ref from the store, or answers 404 when the ref
// is not there — the shape a caller distinguishes with IsForgeNotFound.
func githubRefServer(t *testing.T, store *refStore) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		const pfx = "/repos/medici-finance/assay/git/refs/"
		if r.Method != http.MethodDelete || !strings.HasPrefix(r.URL.EscapedPath(), pfx) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"not found"}`))
			return
		}
		ref := "refs/" + strings.TrimPrefix(r.URL.EscapedPath(), pfx)
		if !store.has(ref) {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"Reference does not exist"}`))
			return
		}
		delete(store.refs, ref)
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// gitlabRefServer records GitLab's Branches API: DELETE
// /api/v4/projects/{urlencoded}/repository/branches/{urlencoded}, Free tier, the ONLY
// ref-delete surface Community Edition exposes. It is reached with the branch name — the
// claim ref minus its "refs/heads/" prefix — and answers 204 on success, 404 when the branch
// is not there.
func gitlabRefServer(t *testing.T, store *refStore) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		const pfx = "/api/v4/projects/medici-finance%2Fassay/repository/branches/"
		if r.Method != http.MethodDelete || !strings.HasPrefix(r.URL.EscapedPath(), pfx) {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]any{"message": "404 Not Found"})
			return
		}
		esc := strings.TrimPrefix(r.URL.EscapedPath(), pfx)
		branch, uerr := url.PathUnescape(esc)
		if uerr != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		ref := "refs/heads/" + branch
		if !store.has(ref) {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]any{"message": "404 Branch Not Found"})
			return
		}
		delete(store.refs, ref)
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// backend is one recorded forge: a name, a fixture server, and the Forge that talks to it.
type backend struct {
	name  string
	build func(t *testing.T, store *refStore) Forge
}

func claimBackends() []backend {
	return []backend{
		{
			name: "github",
			build: func(t *testing.T, store *refStore) Forge {
				srv := githubRefServer(t, store)
				return &GitHubForge{Token: "test-token", BaseURL: srv.URL, Client: srv.Client()}
			},
		},
		{
			name: "gitlab",
			build: func(t *testing.T, store *refStore) Forge {
				srv := gitlabRefServer(t, store)
				return &GitLabForge{Token: glTestToken, BaseURL: srv.URL, Client: srv.Client()}
			},
		},
	}
}

// TestClaimRefRoundTripBothBackends drives create → list → delete → list over the CHOSEN
// claim namespace against both recorded backends, under the same scenario names.
//
// The scenario that matters is "release": the assertion is not that the backend emitted a
// request, it is that the ref is GONE from the store afterwards and no longer appears in the
// listing. That is the only check that separates a real release from a call that returns
// success and deletes nothing — the precise way a claim leaks while every log line looks
// healthy.
func TestClaimRefRoundTripBothBackends(t *testing.T) {
	repo := ForgeRepo{Owner: "medici-finance", Name: "assay"}
	const (
		keyA = "example--example-stream--05"
		keyB = "assay--issue-1234"
	)

	scenarios := []struct {
		name string
		run  func(t *testing.T, f Forge, store *refStore)
	}{
		{
			// The whole lifecycle on one claim. Every step is asserted from the STORE, so a
			// backend that no-ops passes none of it.
			name: "claim round trip: take, list, release, list",
			run: func(t *testing.T, f Forge, store *refStore) {
				ref, err := ClaimRefPath(keyA)
				if err != nil {
					t.Fatalf("ClaimRefPath(%q): %v", keyA, err)
				}
				full := "refs/" + ref

				// take
				store.create(full, "0123456789abcdef0123456789abcdef01234567")
				if !store.has(full) {
					t.Fatalf("the claim %q was not taken", full)
				}
				// list — the reader must see exactly the key that was taken
				if got := store.listClaims(); len(got) != 1 || got[0] != keyA {
					t.Fatalf("listing after take = %v, want [%s]", got, keyA)
				}
				// release, through the forge
				if err := f.DeleteRef(repo, ref); err != nil {
					t.Fatalf("DeleteRef(%q): %v — this backend cannot release a claim, which means a "+
						"claim taken on it is a slot lost", ref, err)
				}
				// the post-condition: gone from the store AND gone from the listing
				if store.has(full) {
					t.Fatalf("DeleteRef reported success but %q is still present — a release that deletes "+
						"nothing is the silent leak this row exists to catch", full)
				}
				if got := store.listClaims(); len(got) != 0 {
					t.Fatalf("listing after release = %v, want empty", got)
				}
			},
		},
		{
			// A second live claim is untouched. A delete that took out the namespace, or the
			// wrong key, would free a slot someone else is holding — the mirror failure of a
			// leak, and the one that produces two workers on one item.
			name: "release frees only the named claim",
			run: func(t *testing.T, f Forge, store *refStore) {
				refA, _ := ClaimRefPath(keyA)
				refB, _ := ClaimRefPath(keyB)
				store.create("refs/"+refA, "aaa")
				store.create("refs/"+refB, "bbb")

				if err := f.DeleteRef(repo, refA); err != nil {
					t.Fatalf("DeleteRef(%q): %v", refA, err)
				}
				if got := store.listClaims(); len(got) != 1 || got[0] != keyB {
					t.Fatalf("listing after releasing one of two claims = %v, want [%s]", got, keyB)
				}
			},
		},
		{
			// Releasing a claim that is already gone reports not-found, which the caller may
			// treat as a no-op. The seam does not decide that for it — but it MUST be
			// distinguishable, so IsForgeNotFound is asserted rather than "some error".
			name: "release of an absent claim is a distinguishable not-found",
			run: func(t *testing.T, f Forge, store *refStore) {
				ref, _ := ClaimRefPath(keyA)
				err := f.DeleteRef(repo, ref)
				if err == nil {
					t.Fatal("deleting an absent claim reported success")
				}
				if !IsForgeNotFound(err) {
					t.Fatalf("the absent-claim error is not recognisable as not-found (%v), so a caller "+
						"cannot tell 'already released' from 'could not release'", err)
				}
			},
		},
	}

	for _, sc := range scenarios {
		for _, be := range claimBackends() {
			t.Run(sc.name+"/"+be.name, func(t *testing.T) {
				store := newRefStore()
				sc.run(t, be.build(t, store), store)
			})
		}
	}
}

// TestRefPathStillRejectsAPIPaths is the negative path: moving the claim into a namespace
// both forges can serve must not have widened what DeleteRef's one path-shaped argument may
// be.
//
// DeleteRef is the single frozen operation whose argument is a string that LOOKS like a
// path, which is exactly the shape an arbitrary-endpoint escape hatch takes — the call it
// replaced was `gh api -X DELETE repos/<o>/<r>/git/refs/...`, which could as easily have
// addressed a branch-protection endpoint. The guard is what makes it an operation on a ref
// instead of a request to wherever the caller pointed it, so every widening of the namespace
// has to be shown NOT to be a widening of the guard.
func TestRefPathStillRejectsAPIPaths(t *testing.T) {
	refused := []struct {
		why string
		ref string
	}{
		{"an API path with a traversal out of the ref namespace", "heads/../../branches/main/protection"},
		{"a traversal hidden inside the claim namespace", ClaimRefNamespace + "/../../../repos/o/r"},
		{"an absolute URL", "https://api.github.com/repos/o/r/git/refs/heads/x"},
		{"a scheme-relative URL", "//api.github.com/repos/o/r"},
		{"a bare unnamespaced component", "main"},
		{"a bare unnamespaced claim key", "example--example-stream--05"},
		{"an empty ref", ""},
		{"a bare namespace with no name under it", "heads/"},
		{"a leading separator", "/heads/x"},
		{"a query string smuggled onto the ref", "heads/x?ref=main"},
		{"a fragment smuggled onto the ref", "heads/x#main"},
		{"a percent escape that could reshape the request", "heads/x%2f..%2fprotection"},
		{"a shell metacharacter", "heads/x;rm -rf /"},
		{"a component that would read as an option", "heads/-force"},
		{"a newline that could inject a second request line", "heads/x\nDELETE /repos/o/r"},
	}
	for _, tc := range refused {
		t.Run(tc.why, func(t *testing.T) {
			if got, err := ValidateRefPath(tc.ref); err == nil {
				t.Fatalf("ValidateRefPath(%q) accepted it as %q — %s must be refused", tc.ref, got, tc.why)
			}
		})
	}

	// The claim-key grammar is bounded by the same guard, one level up: a key is ONE
	// component under the namespace, so a key that would nest, traverse, or escape it is
	// refused before a ref exists. A key accepted here that produced a deeper ref would be a
	// claim nothing lists — held forever, invisible.
	badKeys := []struct {
		why string
		key string
	}{
		{"a key carrying a path separator", "example/example-stream/05"},
		{"a key traversing upward", ".."},
		{"a key escaping into another namespace", "../heads/main"},
		{"an empty key", ""},
		{"a whitespace-only key", "   "},
		{"a key that would read as an option", "-force"},
		{"a key with a forbidden character", "assay--x?y"},
	}
	for _, tc := range badKeys {
		t.Run("claim key: "+tc.why, func(t *testing.T) {
			if got, err := ClaimRefPath(tc.key); err == nil {
				t.Fatalf("ClaimRefPath(%q) produced %q — %s must be refused", tc.key, got, tc.why)
			}
		})
	}

	// And the guard is not vacuous: the shapes the desk actually uses still pass, and each
	// resolves to exactly the ref the readers list.
	for _, key := range []string{"example--example-stream--05", "example--issue-1234", "example-stream--05"} {
		ref, err := ClaimRefPath(key)
		if err != nil {
			t.Fatalf("ClaimRefPath(%q) refused a legitimate claim key: %v", key, err)
		}
		if want := ClaimRefNamespace + "/" + key; ref != want {
			t.Fatalf("ClaimRefPath(%q) = %q, want %q", key, ref, want)
		}
		back, ok := ClaimKeyFromRef("refs/" + ref)
		if !ok || back != key {
			t.Fatalf("ClaimKeyFromRef(refs/%s) = (%q, %v), want (%q, true)", ref, back, ok, key)
		}
	}

	// A ref one level DEEPER than the namespace is not a claim: reading its last segment as
	// a key is how a reader invents a claim key that names nothing.
	for _, notAClaim := range []string{
		ClaimRefsPrefix + "nested/key",
		"refs/heads/main",
		"refs/tags/" + ClaimRefNamespace + "/x",
		"refs/dispatch/legacy--key",
	} {
		if key, ok := ClaimKeyFromRef(notAClaim); ok {
			t.Fatalf("ClaimKeyFromRef(%q) claimed key %q — only a single component under %q is a claim",
				notAClaim, key, ClaimRefsPrefix)
		}
	}
}

// TestClaimNamespaceIsServedByBothBackends states the property the namespace was chosen FOR,
// as a proposition rather than as a consequence of the round trip: the release must be
// expressible on the backend whose ref-delete surface is the narrower of the two.
//
// It is a separate test because the round trip would still pass if GitLab's refusal branch
// were simply deleted; this one asserts that the refusal branch is INTACT (a foreign
// namespace is still refused, with zero requests) and that the claim namespace is on the
// served side of it.
func TestClaimNamespaceIsServedByBothBackends(t *testing.T) {
	repo := ForgeRepo{Owner: "medici-finance", Name: "assay"}
	store := newRefStore()
	srv := gitlabRefServer(t, store)
	f := &GitLabForge{Token: glTestToken, BaseURL: srv.URL, Client: srv.Client()}

	// The claim namespace is inside what the Branches API serves.
	if !strings.HasPrefix(ClaimRefNamespace+"/", "heads/") {
		t.Fatalf("the claim namespace %q is outside refs/heads, which GitLab Community Edition cannot "+
			"delete at any tier — a claim taken there could never be released", ClaimRefNamespace)
	}

	// The refusal for a namespace it genuinely cannot serve is unchanged, and names the gap.
	err := f.DeleteRef(repo, "dispatch/legacy--key")
	if err == nil {
		t.Fatal("a ref outside refs/heads was reported deleted — GitLab CE has no endpoint that could " +
			"have done it")
	}
	if ExitCodeOf(err) != ExitUnverifiable {
		t.Fatalf("the refusal is exit %d, want could-not-check (%d): a namespace this backend cannot "+
			"serve is unknowable, not merely disallowed", ExitCodeOf(err), ExitUnverifiable)
	}
	for _, must := range []string{"heads/<branch>", ClaimRefsPrefix} {
		if !strings.Contains(err.Error(), must) {
			t.Fatalf("the refusal does not mention %q, so it does not say where a claim belongs: %v", must, err)
		}
	}

	// A backend that refuses must have emitted nothing at all: a refusal that already sent a
	// request is a refusal after the fact.
	if got := fmt.Sprint(store.refs); got != "map[]" {
		t.Fatalf("the refused delete disturbed the store: %s", got)
	}
}

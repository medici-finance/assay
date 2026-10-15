package deskkit

// claimstore_test.go — ResolveClaimStore (the claim-store spec §5, Verify V8 release-N half).
//
// Every case goes through the REAL roster loader path (a 0600 roster.env under a private
// config home), so what passes here is the shipped read, not a shortcut around it.

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// installForgeRefOpener installs a forge-ref opener that records whether it was called and
// returns a stand-in store, and uninstalls it after the test.
func installForgeRefOpener(t *testing.T) (called *int, stand ClaimStore) {
	t.Helper()
	n := 0
	s := newMemForgeClaimStore(nil)
	SetForgeRefClaimStoreOpener(func(string) (ClaimStore, error) { n++; return s, nil })
	t.Cleanup(func() { SetForgeRefClaimStoreOpener(nil) })
	return &n, s
}

// wantRefusal asserts err is an exit-6 refusal whose message carries every fragment.
func wantRefusal(t *testing.T, res ClaimStoreResolution, err error, fragments ...string) {
	t.Helper()
	if err == nil {
		t.Fatalf("resolved %q (legacy=%t) — want a refusal", res.Name, res.Legacy)
	}
	if code := ExitCodeOf(err); code != ExitUnverifiable {
		t.Fatalf("refusal exit = %d, want %d (%v)", code, ExitUnverifiable, err)
	}
	if res.Name != "" || res.Store != nil || res.Legacy {
		t.Fatalf("a refusal still carried a store: name=%q legacy=%t store=%v", res.Name, res.Legacy, res.Store)
	}
	for _, f := range fragments {
		if !strings.Contains(err.Error(), f) {
			t.Errorf("refusal does not name %q:\n%v", f, err)
		}
	}
}

// wantUnshippedFallback asserts a valid but unshipped store resolved to the legacy forge-ref
// store (exit 0) under a NOTICE naming the configured store and the release that ships it, with
// provenance recording both the configured value and the fallback (medici-finance/assay#2753).
func wantUnshippedFallback(t *testing.T, res ClaimStoreResolution, err error, value, ships string) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s=%s (valid, not shipped) refused — want the legacy resolution: %v", EnvClaimStore, value, err)
	}
	if res.Name != ClaimStoreForgeRef || !res.Legacy || !res.NeedsForgeCredential {
		t.Fatalf("%s=%s resolved name=%q legacy=%t credential=%t, want %s legacy with a forge credential",
			EnvClaimStore, value, res.Name, res.Legacy, res.NeedsForgeCredential, ClaimStoreForgeRef)
	}
	if res.Notice == ClaimStoreLegacyNotice {
		t.Fatalf("%s=%s printed the UNSET notice — the operator did set the key", EnvClaimStore, value)
	}
	for _, f := range []string{"NOTICE", EnvClaimStore + "=" + value, "does not ship", ClaimStoreForgeRef,
		"takes effect in", ships} {
		if !strings.Contains(res.Notice, f) {
			t.Errorf("unshipped NOTICE does not name %q: %s", f, res.Notice)
		}
	}
	for _, f := range []string{EnvClaimStore + "=", "→ " + EnvClaimStore + "=" + value + " is not shipped in this build", ships,
		"legacy resolution: " + ClaimStoreForgeRef} {
		if !strings.Contains(res.Provenance, f) {
			t.Errorf("provenance does not record %q:\n%s", f, res.Provenance)
		}
	}
}

func TestResolveClaimStore(t *testing.T) {
	const repo = "example-org/tracker"

	t.Run("unset key resolves to the legacy forge-ref store with the removal NOTICE", func(t *testing.T) {
		withRoster(t, goldenRoster())
		called, stand := installForgeRefOpener(t)
		res, err := ResolveClaimStore(repo)
		if err != nil {
			t.Fatalf("unset key refused: %v", err)
		}
		if res.Name != ClaimStoreForgeRef || !res.Legacy || !res.NeedsForgeCredential {
			t.Fatalf("unset key resolved name=%q legacy=%t credential=%t, want %s legacy with a forge credential",
				res.Name, res.Legacy, res.NeedsForgeCredential, ClaimStoreForgeRef)
		}
		if res.Store != stand || *called != 1 {
			t.Fatalf("the legacy store was not opened through the installed opener (called %d times)", *called)
		}
		if got := res.Label(); got != "forge-ref (legacy)" {
			t.Errorf("Label() = %q, want %q", got, "forge-ref (legacy)")
		}
		if res.Notice != ClaimStoreLegacyNotice {
			t.Fatalf("the legacy resolution carries notice %q, want the removal NOTICE", res.Notice)
		}
		for _, f := range []string{"NOTICE", EnvClaimStore, ClaimStoreFile, ClaimStoreService,
			"release " + ClaimStoreLegacyRemovalRelease} {
			if !strings.Contains(res.Notice, f) {
				t.Errorf("removal NOTICE does not name %q: %s", f, res.Notice)
			}
		}
	})

	t.Run("an absent roster is an unset key", func(t *testing.T) {
		withNoRoster(t)
		res, err := ResolveClaimStore(repo)
		if err != nil || !res.Legacy || res.Notice == "" {
			t.Fatalf("absent roster: res=%+v err=%v — want the legacy resolution with its NOTICE", res, err)
		}
	})

	t.Run("without an installed opener the legacy resolution names the store and opens nothing", func(t *testing.T) {
		withRoster(t, goldenRoster())
		SetForgeRefClaimStoreOpener(nil)
		res, err := ResolveClaimStore(repo)
		if err != nil || res.Name != ClaimStoreForgeRef || res.Store != nil {
			t.Fatalf("res=%+v err=%v — want name %s and no store", res, err, ClaimStoreForgeRef)
		}
	})

	t.Run("forge-ref set explicitly is refused printing file and service", func(t *testing.T) {
		withRoster(t, map[string]string{EnvClaimStore: ClaimStoreForgeRef})
		called, _ := installForgeRefOpener(t)
		res, err := ResolveClaimStore(repo)
		wantRefusal(t, res, err, EnvClaimStore, ClaimStoreFile, ClaimStoreService, "cannot be selected")
		if *called != 0 {
			t.Fatal("an explicit forge-ref value opened the forge-ref store")
		}
	})

	t.Run("an unknown value is refused printing file and service", func(t *testing.T) {
		for _, v := range []string{"forge", "FILE", "files", "git-ref", "service,file"} {
			withRoster(t, map[string]string{EnvClaimStore: v})
			called, _ := installForgeRefOpener(t)
			res, err := ResolveClaimStore(repo)
			wantRefusal(t, res, err, EnvClaimStore, ClaimStoreFile, ClaimStoreService)
			if *called != 0 {
				t.Fatalf("value %q opened the forge-ref store", v)
			}
		}
	})

	t.Run("file and service are valid but unshipped: legacy store, NOTICE names the shipping release", func(t *testing.T) {
		for v, brief := range map[string]string{ClaimStoreFile: "ships the file store", ClaimStoreService: "ships the served store"} {
			withRoster(t, map[string]string{EnvClaimStore: v, EnvClaimSingleHost: "yes"})
			called, stand := installForgeRefOpener(t)
			res, err := ResolveClaimStore(repo)
			wantUnshippedFallback(t, res, err, v, brief)
			if res.Store != stand || *called != 1 {
				t.Fatalf("%s: the legacy store was not opened through the installed opener (called %d times)", v, *called)
			}
		}
	})

	t.Run("the per-repo form resolves the named repo and refuses an unnamed one", func(t *testing.T) {
		withRoster(t, map[string]string{EnvClaimStore: "example-org/tracker=file,example-org/agents=service"})
		called, _ := installForgeRefOpener(t)
		res, err := ResolveClaimStore("example-org/console")
		wantRefusal(t, res, err, "example-org/console", "names no store")
		if *called != 0 {
			t.Fatal("a per-repo key that does not name the repo fell through to the forge-ref store")
		}
		res, err = ResolveClaimStore("Example-Org/Tracker")
		wantUnshippedFallback(t, res, err, ClaimStoreFile, "ships the file store")
	})

	t.Run("malformed keys are refused, never defaulted", func(t *testing.T) {
		cases := []struct {
			vals map[string]string
			want string
		}{
			{map[string]string{EnvClaimStore: "file,example-org/tracker=service"}, "mixes a cell-wide value"},
			{map[string]string{EnvClaimStore: "tracker=file"}, "not a full owner/name slug"},
			{map[string]string{EnvClaimStore: "example-org/tracker=file,example-org/tracker=service"}, "more than once"},
			{map[string]string{EnvClaimStore: "example-org/tracker=forge-ref"}, "cannot be selected"},
			{map[string]string{EnvClaimDir: "relative/claims"}, "not an absolute path"},
			{map[string]string{EnvClaimDir: "/a,/b"}, "ONE directory"},
			{map[string]string{EnvClaimSingleHost: "true"}, "the only value is yes"},
		}
		for _, c := range cases {
			withRoster(t, c.vals)
			called, _ := installForgeRefOpener(t)
			res, err := ResolveClaimStore(repo)
			wantRefusal(t, res, err, c.want)
			if *called != 0 {
				t.Fatalf("malformed %v opened the forge-ref store", c.vals)
			}
		}
	})

	t.Run("an unusable roster is could-not-check, never an unset key", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("the roster permission rule is an ACL check on windows; the unix mode bits are what this drives")
		}
		home := withRoster(t, goldenRoster())
		if err := os.Chmod(filepath.Join(home, ".config", "assay", "roster.env"), 0o666); err != nil {
			t.Fatal(err)
		}
		called, _ := installForgeRefOpener(t)
		res, err := ResolveClaimStore(repo)
		wantRefusal(t, res, err, "roster cannot be read", "never treated as unset")
		if *called != 0 {
			t.Fatal("an unreadable roster resolved to the forge-ref store")
		}
	})

	t.Run("a forge-ref opener failure is could-not-check", func(t *testing.T) {
		withRoster(t, goldenRoster())
		SetForgeRefClaimStoreOpener(func(string) (ClaimStore, error) { return nil, errors.New("no forge token") })
		t.Cleanup(func() { SetForgeRefClaimStoreOpener(nil) })
		res, err := ResolveClaimStore(repo)
		wantRefusal(t, res, err, "cannot be opened", "no forge token")
	})

	t.Run("provenance names every input and its source", func(t *testing.T) {
		home := withRoster(t, map[string]string{EnvClaimDir: "/srv/cell/claims", EnvClaimSingleHost: "yes"})
		res, err := ResolveClaimStore(repo)
		if err != nil {
			t.Fatal(err)
		}
		src := filepath.Join(home, ".config", "assay", "roster.env")
		for _, f := range []string{
			EnvClaimStore + "=(unset) (config file " + src + ")",
			EnvClaimDir + "=/srv/cell/claims (config file " + src + ")",
			EnvClaimSingleHost + "=yes (config file " + src + ")",
			"legacy resolution: " + ClaimStoreForgeRef,
		} {
			if !strings.Contains(res.Provenance, f) {
				t.Errorf("provenance does not carry %q:\n%s", f, res.Provenance)
			}
		}
		withRoster(t, goldenRoster())
		res, _ = ResolveClaimStore(repo)
		if !strings.Contains(res.Provenance, EnvClaimDir+"="+defaultClaimDir()+" (default)") {
			t.Errorf("an unset %s is not reported as its default:\n%s", EnvClaimDir, res.Provenance)
		}
	})

	t.Run("a roster carrying the claim-store keys is not refused as unknown keys", func(t *testing.T) {
		roster := goldenRoster()
		roster[EnvClaimStore] = ClaimStoreFile
		roster[EnvClaimDir] = "/srv/cell/claims"
		roster[EnvClaimSingleHost] = "yes"
		withRoster(t, roster)
		if cfg := EffectiveConfig(); !cfg.Configured() {
			t.Fatalf("a roster carrying the claim-store keys was refused: %v", cfg.Problems)
		}
	})

	t.Run("no exported symbol and no flag accepts a store choice", func(t *testing.T) {
		assertNoClaimStoreSelector(t)
	})
}

// assertNoClaimStoreSelector is the no-selector check: the store is the resolver's answer, and
// nothing a caller supplies can choose it.
//
//  1. ResolveClaimStore takes exactly one string, the repo.
//  2. No exported function or method in this package takes a parameter NAMED like a store
//     choice (store, storeName, backend, kind…) — the shape by which a selector would reach a
//     caller-reachable surface. The forge-ref opener hook takes a constructor, not a choice.
//  3. No flag in the claim tool or the dispatcher's source mentions a store.
func assertNoClaimStoreSelector(t *testing.T) {
	t.Helper()
	fset := token.NewFileSet()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	selectorParam := regexp.MustCompile(`(?i)^(store|storename|storekind|claimstore|backend|backendname|kind|choice)$`)
	checked := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, perr := parser.ParseFile(fset, name, nil, 0)
		if perr != nil {
			t.Fatalf("parsing %s: %v", name, perr)
		}
		checked++
		ast.Inspect(f, func(n ast.Node) bool {
			fd, ok := n.(*ast.FuncDecl)
			if !ok || !fd.Name.IsExported() || fd.Type.Params == nil {
				return true
			}
			if fd.Name.Name == "ResolveClaimStore" {
				params := fd.Type.Params.List
				if len(params) != 1 || len(params[0].Names) != 1 || params[0].Names[0].Name != "repo" {
					t.Errorf("%s: ResolveClaimStore must take exactly (repo string)", fset.Position(fd.Pos()))
				} else if id, ok := params[0].Type.(*ast.Ident); !ok || id.Name != "string" {
					t.Errorf("%s: ResolveClaimStore's one parameter must be the repo string", fset.Position(fd.Pos()))
				}
			}
			if strings.Contains(fd.Name.Name, "Claim") || strings.Contains(fd.Name.Name, "Store") {
				for _, p := range fd.Type.Params.List {
					for _, pn := range p.Names {
						if selectorParam.MatchString(pn.Name) {
							t.Errorf("%s: exported func %s has a parameter %q — a caller could choose the claim store",
								fset.Position(fd.Pos()), fd.Name.Name, pn.Name)
						}
					}
				}
			}
			return true
		})
	}
	if checked == 0 {
		t.Fatal("no source files were scanned — this check would pass vacuously")
	}

	flagMentionsStore := regexp.MustCompile(`"--?[A-Za-z-]*store[A-Za-z-]*"|fs\.\w+\(\s*"[A-Za-z-]*store[A-Za-z-]*"`)
	for _, dir := range []string{"../../cmd/deskclaim-ref", "../../cmd/deskdispatch"} {
		files, err := filepath.Glob(filepath.Join(dir, "*.go"))
		if err != nil || len(files) == 0 {
			t.Fatalf("could not list %s (%v) — the flag check would pass vacuously", dir, err)
		}
		for _, file := range files {
			if strings.HasSuffix(file, "_test.go") {
				continue
			}
			b, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			if m := flagMentionsStore.FindString(string(b)); m != "" {
				t.Errorf("%s declares or parses a store flag (%s) — no flag may select the claim store", file, m)
			}
		}
	}
}

// TestResolveClaimStoreNeverFallsBack — an EXPLICIT store this build SHIPS whose precondition
// fails is exit 6, and the resolved name is never another store (Verify row 3; the mutation
// claimstore-unmet-precondition-falls-through reddens it). A valid store this build does NOT
// ship holds no claim here, so it resolves to the legacy store under its own NOTICE
// (amended 2026-10-13, medici-finance/assay#2753).
// regression: #2753
func TestResolveClaimStoreNeverFallsBack(t *testing.T) {
	const repo = "example-org/tracker"

	check := func(t *testing.T, why string) {
		t.Helper()
		called, _ := installForgeRefOpener(t)
		res, err := ResolveClaimStore(repo)
		if err == nil {
			t.Fatalf("%s: resolved %q (legacy=%t) — an unmet precondition must refuse", why, res.Label(), res.Legacy)
		}
		if code := ExitCodeOf(err); code != ExitUnverifiable {
			t.Fatalf("%s: exit %d, want %d: %v", why, code, ExitUnverifiable, err)
		}
		if res.Name != "" || res.Legacy || res.Store != nil {
			t.Fatalf("%s: the refusal carried store %q (legacy=%t)", why, res.Name, res.Legacy)
		}
		if *called != 0 {
			t.Fatalf("%s: the forge-ref store was opened — the resolver fell back", why)
		}
	}

	for _, v := range []string{ClaimStoreFile, ClaimStoreService} {
		t.Run(v+" not shipped in this build", func(t *testing.T) {
			withRoster(t, map[string]string{EnvClaimStore: v, EnvClaimSingleHost: "yes"})
			called, _ := installForgeRefOpener(t)
			res, err := ResolveClaimStore(repo)
			wantUnshippedFallback(t, res, err, v, claimStoreBackends[v].shippedBy)
			if *called != 1 {
				t.Fatalf("%s (not shipped): the legacy store was opened %d times, want 1", v, *called)
			}
		})
		t.Run(v+" shipped but its precondition fails", func(t *testing.T) {
			withRoster(t, map[string]string{EnvClaimStore: v})
			prev := claimStoreBackends[v]
			claimStoreBackends[v] = claimStoreBackend{shippedBy: prev.shippedBy,
				open: func(string, claimStoreKeys) (ClaimStore, error) {
					return nil, errors.New("precondition not met")
				}}
			t.Cleanup(func() { claimStoreBackends[v] = prev })
			check(t, v+" (precondition fails)")
		})
	}
}

// TestLegacyNoticeSaysSetNow — the unset-key NOTICE tells the operator the keys may be set
// now and take effect on upgrade; it never tells them to set a value this build would refuse.
// regression: #2753
func TestLegacyNoticeSaysSetNow(t *testing.T) {
	for _, f := range []string{"may be set in the roster now", "takes effect on upgrade",
		EnvClaimStore + "=" + ClaimStoreFile, EnvClaimStore + "=" + ClaimStoreService, EnvClaimSingleHost + "=yes",
		"release " + ClaimStoreLegacyRemovalRelease} {
		if !strings.Contains(ClaimStoreLegacyNotice, f) {
			t.Errorf("legacy NOTICE does not say %q: %s", f, ClaimStoreLegacyNotice)
		}
	}
	// The value it names must be one this build resolves without refusing.
	withRoster(t, map[string]string{EnvClaimStore: ClaimStoreFile, EnvClaimSingleHost: "yes"})
	SetForgeRefClaimStoreOpener(nil)
	if _, err := ResolveClaimStore("example-org/tracker"); err != nil {
		t.Fatalf("the legacy NOTICE names %s=%s, which this build refuses: %v", EnvClaimStore, ClaimStoreFile, err)
	}
}

// TestClaimSingleHostNoEffect — ASSAY_CLAIM_SINGLE_HOST=yes is accepted and changes nothing
// while the store is unset or unshipped: same store, same legacy flag, same NOTICE; only the
// provenance records the declaration.
// regression: #2753
func TestClaimSingleHostNoEffect(t *testing.T) {
	const repo = "example-org/tracker"
	for _, store := range []string{"", ClaimStoreFile, ClaimStoreService} {
		resolve := func(singleHost bool) ClaimStoreResolution {
			t.Helper()
			vals := map[string]string{}
			if store != "" {
				vals[EnvClaimStore] = store
			}
			if singleHost {
				vals[EnvClaimSingleHost] = "yes"
			}
			withRoster(t, vals)
			_, stand := installForgeRefOpener(t)
			res, err := ResolveClaimStore(repo)
			if err != nil {
				t.Fatalf("store %q single-host=%t refused: %v", store, singleHost, err)
			}
			if res.Store != stand {
				t.Fatalf("store %q single-host=%t did not open the legacy store", store, singleHost)
			}
			return res
		}
		without, with := resolve(false), resolve(true)
		if with.Name != without.Name || with.Legacy != without.Legacy ||
			with.NeedsForgeCredential != without.NeedsForgeCredential || with.Notice != without.Notice {
			t.Errorf("store %q: %s=yes changed the resolution:\n without: %+v\n with:    %+v",
				store, EnvClaimSingleHost, without, with)
		}
		if !strings.Contains(with.Provenance, EnvClaimSingleHost+"=yes") {
			t.Errorf("store %q: provenance does not record %s=yes:\n%s", store, EnvClaimSingleHost, with.Provenance)
		}
	}
}

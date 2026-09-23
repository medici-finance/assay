package deskkit

// claimstore.go — the dispatch-claim store seam, and ResolveClaimStore, the ONE place that
// decides where a process keeps its dispatch claims (forge-neutral/21; the spec is
// docs/streams/forge-neutral/reviewer-write-boundary.md §4 and §5).
//
// THE SEAM. ClaimStore is the storage surface the claim tool (cmd/deskclaim-ref) always drove,
// lifted here unchanged from that tool's main package so that more than one backend can sit
// behind it: seven methods, three-state results, and a compare-and-swap token on every write.
// The verbs above it — acquire, progress, release, steal, show, list — stay pure decision
// logic over these results, so their output (the `show` / `list` wire contract other tools
// parse) is a function of what a store returns and nothing else.
//
// THE RESOLVER. ResolveClaimStore mirrors ForgeFor's contract (forgeresolve.go):
//
//  1. NO CALLER CHOICE. The store is never chosen by a caller. ResolveClaimStore takes the
//     repo and nothing else; no exported symbol in this package takes a store name; and no
//     command-line flag selects one. The only input is the roster (ASSAY_CLAIM_STORE).
//  2. REFUSAL IS THE ONLY FALLBACK. A configured store whose preconditions do not hold is a
//     refusal (exit 6) naming the store, the missing precondition and the remedy. It never
//     moves on to another store — a resolver that fell back instead of refusing would
//     double-dispatch without failing a single happy-path test.
//  3. THE LEGACY RESOLUTION. For one release window, and ONLY when ASSAY_CLAIM_STORE is
//     unset, the answer is today's forge-ref store with a NOTICE naming the key, its two
//     valid values and the release in which the unset key stops resolving. `forge-ref` is
//     not a valid VALUE: set explicitly it is refused like any unknown value, so the legacy
//     store is what silence means, never something that can be selected.
//
// SINGLE-POINT-OF-FAILURE. The resolver's order is the one control between a wrong
// resolution and two holders of one claim. Behind it: each store's own atomic create (a
// wrong resolution still cannot yield two holders INSIDE one store) and, during the window,
// the mixed-store refusal (forge-neutral/23), which detects two live stores for one repo from
// the forge side — a different signal in a different component.

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// ClaimReadStatus is the three-state result of a claim read.
type ClaimReadStatus int

const (
	// ClaimReadHeld — a holder exists.
	ClaimReadHeld ClaimReadStatus = iota
	// ClaimReadFree — no claim exists for the key.
	ClaimReadFree
	// ClaimReadUnverifiable — the read itself failed. Fail closed: never "free".
	ClaimReadUnverifiable
)

// ClaimWriteOutcome is the three-state result of a claim write. A rejection is the store's
// compare-and-swap losing (the claim already exists on a create, or its version moved under
// an update/steal) — an expected race the caller acts on, never a could-not-check.
type ClaimWriteOutcome int

const (
	// ClaimWriteApplied — the store applied the write.
	ClaimWriteApplied ClaimWriteOutcome = iota
	// ClaimWriteRejected — the store refused: the compare-and-swap old value no longer matches.
	ClaimWriteRejected
	// ClaimWriteUnverifiable — a transport, auth or I/O failure. Fail closed.
	ClaimWriteUnverifiable
)

// ClaimStoreRecord is a held claim as a store returns it.
type ClaimStoreRecord struct {
	// Version is the store's compare-and-swap token for the claim as read — the value the next
	// UpdateFrom must carry as its old value (the forge store: the claim tag object's sha).
	Version string
	// Msg is the holder encoding, byte-for-byte as written:
	// `dispatch-claim <id> owner=… state=… branch=…[ note=…]`.
	Msg string
	// Date is when the claim was placed or last advanced, RFC3339.
	Date string
}

// ClaimStore is the storage surface every dispatch-claim backend implements. Every method is
// three-state: a result the store could not establish is Unverifiable, never "free" and never
// "applied".
type ClaimStore interface {
	// Read returns the claim held under id and its status.
	Read(id string) (ClaimStoreRecord, ClaimReadStatus)
	// CreateIfAbsent places a claim carrying msg (stamped now) only if none exists.
	// ClaimWriteRejected == a holder already exists.
	CreateIfAbsent(id, msg string) ClaimWriteOutcome
	// UpdateFrom replaces the claim with one carrying msg, only if it still holds
	// oldVersion. ClaimWriteRejected == it moved (advanced or stolen) under this caller.
	UpdateFrom(id, oldVersion, msg string) ClaimWriteOutcome
	// Remove deletes the claim. ClaimWriteApplied == deleted OR already absent (a release is
	// idempotent); existed reports which.
	Remove(id string) (outcome ClaimWriteOutcome, existed bool)
	// List enumerates the ids of the present claims.
	List() ([]string, ClaimReadStatus)
	// BranchExists reports whether the branch a holder names exists on the remote
	// (branch-as-claim); verifiable=false is could-not-check.
	BranchExists(branch string) (exists, verifiable bool)
	// TransportCause reports "<where>: <error>" for the store's most recent transport
	// failure, or "" when the last operation did not fail at the transport layer, so a
	// fail-closed message can say where the store reached and why it failed.
	TransportCause() string
}

// The claim store names. ClaimStoreFile and ClaimStoreService are the two VALID values of
// ASSAY_CLAIM_STORE. ClaimStoreForgeRef is the name the legacy resolution REPORTS; it is not
// a valid value of the key, and setting it is refused like any unknown value.
const (
	ClaimStoreFile     = "file"
	ClaimStoreService  = "service"
	ClaimStoreForgeRef = "forge-ref"
)

// ClaimStoreLegacyRemovalRelease is the release in which an unset ASSAY_CLAIM_STORE stops
// resolving to the forge-ref store. It is ONE constant, and the removal NOTICE is rendered
// from it rather than restating it in prose. It holds the placeholder "N+1" until release N —
// the release that ships the file store and the serve mode — is cut; that cut sets it to the
// concrete release tag, forge-neutral/30 records the release it names, and forge-neutral/32
// (the deletion) targets exactly that release.
const ClaimStoreLegacyRemovalRelease = "N+1"

// ClaimStoreLegacyNotice is the removal NOTICE printed on every run of a dispatching role that
// resolved the legacy store. It names the key, the two valid values and the removal release.
const ClaimStoreLegacyNotice = "NOTICE: " + EnvClaimStore + " is unset, so dispatch claims use the legacy " +
	ClaimStoreForgeRef + " store, which needs repository write; an unset " + EnvClaimStore +
	" stops resolving in release " + ClaimStoreLegacyRemovalRelease + " — set " + EnvClaimStore + "=" +
	ClaimStoreFile + " or " + EnvClaimStore + "=" + ClaimStoreService + " in the roster."

// ClaimStoreResolution is ResolveClaimStore's answer: the store, its name, and where every
// input to the decision came from.
type ClaimStoreResolution struct {
	// Store is the resolved backend. For the legacy resolution it is built by the forge-ref
	// opener this process installed (SetForgeRefClaimStoreOpener); a process that drives the
	// claim tool as a child rather than opening the store itself installs none and gets nil
	// here — it needs only the name and whether a forge credential is required.
	Store ClaimStore
	// Name is the resolved store's name (ClaimStoreForgeRef for the legacy resolution).
	Name string
	// Legacy is true only for the one-window legacy resolution of an unset key.
	Legacy bool
	// NeedsForgeCredential reports whether writing a claim to this store needs a forge
	// credential — true for the forge-ref store, which writes refs on the forge.
	NeedsForgeCredential bool
	// Provenance names each input to the decision and its source.
	Provenance string
	// Notice is the NOTICE the caller must print on every run for this resolution, or "".
	Notice string
}

// Label renders the store for a report line: the name, marked "(legacy)" for the legacy
// resolution — e.g. `forge-ref (legacy)`.
func (r ClaimStoreResolution) Label() string {
	if r.Legacy {
		return r.Name + " (legacy)"
	}
	return r.Name
}

// forgeRefOpenerMu guards forgeRefOpener.
var forgeRefOpenerMu sync.Mutex

// forgeRefOpener builds the forge-ref store for a repo. The forge store's transport lives in
// the claim tool (cmd/deskclaim-ref), which owns its own credential handling, so it installs
// this rather than deskkit constructing it — the same hook shape as SetGitHubCustodyMinter.
var forgeRefOpener func(repo string) (ClaimStore, error)

// SetForgeRefClaimStoreOpener installs how THIS process opens the forge-ref store, should the
// resolver resolve to it. Installing an opener selects nothing: the resolver alone decides
// whether it is used. nil uninstalls it.
func SetForgeRefClaimStoreOpener(open func(repo string) (ClaimStore, error)) {
	forgeRefOpenerMu.Lock()
	defer forgeRefOpenerMu.Unlock()
	forgeRefOpener = open
}

// claimStoreBackend is one valid value of ASSAY_CLAIM_STORE as this build knows it. A backend
// whose open is nil is VALID but not shipped in this build: resolving to it is a refusal that
// names the brief that ships it, so a key set early fails loudly instead of quietly resolving
// to something else.
type claimStoreBackend struct {
	shippedBy string
	open      func(repo string, keys claimStoreKeys) (ClaimStore, error)
}

// claimStoreBackends is keyed by the two valid values and nothing else.
var claimStoreBackends = map[string]claimStoreBackend{
	ClaimStoreFile:    {shippedBy: "forge-neutral/23"},
	ClaimStoreService: {shippedBy: "forge-neutral/24"},
}

// ResolveClaimStore decides where the dispatch claims for repo are kept (spec §5). It reads
// only the roster; nothing a caller passes but the repo takes part.
//
// Order:
//  1. a claim-store key that cannot be read or does not parse → refusal;
//  2. ASSAY_CLAIM_STORE set for repo → that store, or a refusal when its preconditions do
//     not hold — never another store;
//  3. ASSAY_CLAIM_STORE unset → the legacy forge-ref store plus ClaimStoreLegacyNotice.
//
// Every refusal is exit 6 and says which store, what is missing and the remedy. The caller
// resolves BEFORE it cuts a worktree or mints a credential.
func ResolveClaimStore(repo string) (ClaimStoreResolution, error) {
	vals, source, err := readClaimStoreKeys()
	if err != nil {
		return ClaimStoreResolution{}, Unverifiable(fmt.Sprintf(
			"claim store for %s cannot be resolved: the roster cannot be read (%s), so %s cannot be read — an "+
				"unreadable key is never treated as unset. Fix the roster and re-run", repo, source, EnvClaimStore), err)
	}
	keys, perr := parseClaimStoreKeys(vals)
	if perr != nil {
		return ClaimStoreResolution{}, Unverifiable(fmt.Sprintf(
			"claim store for %s cannot be resolved: %v (from %s)", repo, perr, source), nil)
	}
	prov := keys.provenance(source)

	value, set, verr := keys.storeFor(repo)
	if verr != nil {
		return ClaimStoreResolution{}, Unverifiable(fmt.Sprintf(
			"claim store for %s cannot be resolved: %v (from %s)", repo, verr, source), nil)
	}
	if set {
		res, rerr := resolveConfiguredClaimStore(repo, value, keys, prov)
		if rerr != nil {
			return ClaimStoreResolution{}, rerr
		}
		return res, nil
	}
	return resolveLegacyClaimStore(repo, prov)
}

// resolveConfiguredClaimStore resolves an explicitly configured store. Every unmet
// precondition is a refusal; this function has no path that returns a different store.
func resolveConfiguredClaimStore(repo, value string, keys claimStoreKeys, prov string) (ClaimStoreResolution, error) {
	b, ok := claimStoreBackends[value]
	if !ok {
		// Unreachable: parsing admits only the backend names. Kept fail-closed.
		return ClaimStoreResolution{}, Unverifiable(fmt.Sprintf(
			"claim store for %s: %s=%s is not a store this build knows — valid values are %s and %s",
			repo, EnvClaimStore, value, ClaimStoreFile, ClaimStoreService), nil)
	}
	if b.open == nil {
		return ClaimStoreResolution{}, Unverifiable(fmt.Sprintf(
			"claim store for %s: %s=%s is a valid store, but this build does not ship it — it ships with "+
				"%s. The configured store is never replaced by another one: upgrade to a release that "+
				"ships it, or remove the key until then (%s)",
			repo, EnvClaimStore, value, b.shippedBy, prov), nil)
	}
	s, err := b.open(repo, keys)
	if err != nil {
		return ClaimStoreResolution{}, Unverifiable(fmt.Sprintf(
			"claim store for %s: %s=%s cannot be used — refusing rather than using another store (%s)",
			repo, EnvClaimStore, value, prov), err)
	}
	return ClaimStoreResolution{Store: s, Name: value, Provenance: prov}, nil
}

// resolveLegacyClaimStore is the one-window legacy resolution of an UNSET key.
func resolveLegacyClaimStore(repo, prov string) (ClaimStoreResolution, error) {
	res := ClaimStoreResolution{
		Name:                 ClaimStoreForgeRef,
		Legacy:               true,
		NeedsForgeCredential: true,
		Provenance:           prov + " → legacy resolution: " + ClaimStoreForgeRef,
		Notice:               ClaimStoreLegacyNotice,
	}
	forgeRefOpenerMu.Lock()
	open := forgeRefOpener
	forgeRefOpenerMu.Unlock()
	if open == nil {
		return res, nil
	}
	s, err := open(repo)
	if err != nil {
		return ClaimStoreResolution{}, Unverifiable(fmt.Sprintf(
			"claim store for %s: the %s store cannot be opened", repo, ClaimStoreForgeRef), err)
	}
	res.Store = s
	return res, nil
}

// claimStoreKeyNames are the roster keys the resolver reads, in the order it reports them.
var claimStoreKeyNames = []string{EnvClaimStore, EnvClaimDir, EnvClaimSingleHost}

// readClaimStoreKeys reads the raw claim-store keys from the source this process's tool class
// reads the roster from (rosterconfig.go's split by action class). It reads them directly
// rather than through the cached trust configuration, so a trust-surface problem elsewhere in
// the roster neither hides these keys nor is hidden by them.
//
// An ABSENT roster file is a legitimate empty: no key is set, which is exactly today's
// configuration. A roster file that exists but cannot be used (permissions, ownership, read
// error) is an error — the keys are then unknown, and unknown is never unset.
func readClaimStoreKeys() (map[string]string, string, error) {
	cfgMu.Lock()
	class := cfgClass
	cfgMu.Unlock()

	fromEnv := func() map[string]string {
		m := map[string]string{}
		for _, k := range claimStoreKeyNames {
			if v := strings.TrimSpace(os.Getenv(k)); v != "" {
				m[k] = v
			}
		}
		return m
	}
	pick := func(all map[string]string) map[string]string {
		m := map[string]string{}
		for _, k := range claimStoreKeyNames {
			if v, ok := all[k]; ok {
				m[k] = v
			}
		}
		return m
	}

	switch class {
	case ClassCI:
		return fromEnv(), "environment (repository/organization Actions variables)", nil
	case ClassReadOnly:
		m := fromEnv()
		file, path, ferr := readConfigFile()
		switch {
		case ferr == nil:
			for k, v := range pick(file) {
				if _, ok := m[k]; !ok {
					m[k] = v
				}
			}
			return m, "environment, then config file " + path, nil
		case errors.Is(ferr, os.ErrNotExist):
			return m, "environment (config file " + path + " absent)", nil
		default:
			return nil, "config file " + path + " (unusable: " + ferr.Error() + ")", ferr
		}
	default: // ClassWrite — the config-home file ONLY, never the environment.
		file, path, ferr := readConfigFile()
		switch {
		case ferr == nil:
			return pick(file), "config file " + path, nil
		case errors.Is(ferr, os.ErrNotExist):
			return map[string]string{}, "config file " + path + " (absent)", nil
		default:
			return nil, "config file " + path + " (unusable: " + ferr.Error() + ")", ferr
		}
	}
}

// defaultClaimDir is the file store's directory when ASSAY_CLAIM_DIR is unset:
// `<config home>/dispatch-claims`. Under a cell launcher the config home is already the
// cell's own.
func defaultClaimDir() string {
	return filepath.Join(filepath.Dir(ConfigHomePath()), "dispatch-claims")
}

// provenance renders each claim-store input and its source, in claimStoreKeyNames order.
func (k claimStoreKeys) provenance(source string) string {
	parts := make([]string, 0, len(claimStoreKeyNames))
	store := "(unset)"
	if k.single != "" {
		store = k.single
	} else if len(k.perRepo) > 0 {
		entries := make([]string, 0, len(k.perRepo))
		for repo, v := range k.perRepo {
			entries = append(entries, repo+"="+v)
		}
		sort.Strings(entries)
		store = strings.Join(entries, ",")
	}
	parts = append(parts, EnvClaimStore+"="+store+" ("+source+")")
	if k.dir != "" {
		parts = append(parts, EnvClaimDir+"="+k.dir+" ("+source+")")
	} else {
		parts = append(parts, EnvClaimDir+"="+defaultClaimDir()+" (default)")
	}
	single := "(unset)"
	if k.singleHost {
		single = "yes"
	}
	parts = append(parts, EnvClaimSingleHost+"="+single+" ("+source+")")
	return strings.Join(parts, "; ")
}

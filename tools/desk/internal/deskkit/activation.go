package deskkit

// activation.go — component ACTIVATION (the component-manifest spec §6, this brief).
//
// A component is ACTIVE iff every `inject.required` key resolves to an ACTIVE
// provider (§6.1): for a roster key, "resolves" means the loader's own per-key
// validation (rosterconfig.go's TRUST/EXTENSION split) says the key is usable;
// for any other key, it means some ACTIVE component's `provides` list carries
// it. Otherwise the component is INACTIVE and its verbs/hooks refuse in the
// three-state `could-not-check` form (brief-v1 §8) rather than run with a
// missing input.
//
// WHY THIS FILE DOES NOT IMPORT cmd/deskmanifest. brief 00's `deskmanifest
// lint` (tools/desk/cmd/deskmanifest) owns the FULL manifest contract —
// version ranges, cycle REPORTING, the CI-blocking lint output. `cmd/…` is a
// `main` package and is not importable, and this file needs only the
// declaring / providing / requiring shape (the component-manifest spec §2) to compute
// activation. So it parses that minimal shape itself, the same
// documented-duplicate pattern rosterconfig.go already uses against
// statusgen/rosterconfig.go (see that file's package comment): two readers of
// the same declarations, deliberately not sharing code, each simple enough to
// read on its own.
//
// SCOPE. This file computes activation and formats the refusal string;
// CheckVerbActivation is the ONE call site every desk command's main() makes
// (right after deskkit.EchoEffectiveConfig — "the verb entrypoint shared by
// the desk commands" the brief edits), so a verb belonging to an INACTIVE
// component refuses before doing any work. It is best-effort about WHERE it
// looks: a desk binary can run from any working directory, including outside
// any checkout at all, and manifest discovery failing must never itself
// become a refusal — see CheckVerbActivation's doc comment.

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/gitcore"
	"gopkg.in/yaml.v3"
)

// componentManifest is the minimal shape activation needs from a
// component.yaml (the component-manifest spec §2): its name, what it provides, and
// what it requires or optionally accepts. Version ranges, apply/intercept and
// config are deskmanifest lint's business, not this file's.
type componentManifest struct {
	Component string   `yaml:"component"`
	Provides  []string `yaml:"provides"`
	Inject    struct {
		Required []manifestInjectKey `yaml:"required"`
		Optional []manifestInjectKey `yaml:"optional"`
	} `yaml:"inject"`
}

type manifestInjectKey struct {
	Key string `yaml:"key"`
}

// DiscoverManifests walks root for every component.yaml (skipping .git,
// mirroring cmd/deskmanifest's own discover()) and parses the
// activation-relevant fields. A manifest that fails to parse, or carries no
// `component:` name, is SKIPPED and named in skipped rather than aborting the
// whole discovery — `deskmanifest lint` is the tool whose job is to fail
// loudly on a broken manifest; a caller here getting a partial view because
// one manifest is malformed is still more useful than refusing every
// component's activation over it.
func DiscoverManifests(root string) (manifests []componentManifest, skipped []string, err error) {
	info, statErr := os.Stat(root)
	if statErr != nil {
		return nil, nil, statErr
	}
	if !info.IsDir() {
		return nil, nil, fmt.Errorf("root %s is not a directory", root)
	}
	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, werr error) error {
		if werr != nil {
			return werr
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() != "component.yaml" {
			return nil
		}
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			skipped = append(skipped, path+": unreadable: "+rerr.Error())
			return nil
		}
		var m componentManifest
		if yerr := yaml.Unmarshal(data, &m); yerr != nil {
			skipped = append(skipped, path+": unparseable: "+yerr.Error())
			return nil
		}
		if strings.TrimSpace(m.Component) == "" {
			skipped = append(skipped, path+": carries no component: name")
			return nil
		}
		manifests = append(manifests, m)
		return nil
	})
	if walkErr != nil {
		return nil, skipped, fmt.Errorf("could not walk %s: %w", root, walkErr)
	}
	sort.Slice(manifests, func(i, j int) bool { return manifests[i].Component < manifests[j].Component })
	sort.Strings(skipped)
	return manifests, skipped, nil
}

// ActivationResult is one component's computed state (the component-manifest
// spec §6.1). Reason and Key are populated only when Active is false: Key is the
// SPECIFIC unresolved inject.required entry (the first found, in manifest
// declaration order), Reason is why it did not resolve.
type ActivationResult struct {
	Active bool
	Key    string
	Reason string
}

// ComputeActivation computes ACTIVE/INACTIVE for every component named in
// manifests.
//
//   - "assay.roster.trust" resolves iff trustConfigured — the caller passes
//     EffectiveConfig().Configured(); this file never reads the roster itself,
//     so a caller (or a test) can feed it any bool.
//   - a key with the "assay.roster.ext." prefix resolves iff
//     ext[name].Status != ExtInvalid, where name is the suffix after that
//     prefix and ext is keyed exactly as Config.Ext is (ExtKeyName's return
//     value). A name absent from ext (never validated at all) resolves — the
//     same "unset is a complete configuration" rule every extension key
//     documents.
//   - any other key resolves iff some component whose OWN activation is
//     already established ACTIVE lists it under `provides`. Resolution is a
//     fixed point over the manifest set: a component reached while it is
//     still being resolved (a cycle among inject.required) is treated as
//     unresolved for that edge, which leaves every member of the cycle
//     INACTIVE — matching §6.1's cycle rule. `deskmanifest lint` is the tool
//     that REPORTS a cycle as a CI problem; this file only refuses to
//     activate one.
//   - a key with NO declared provider anywhere in manifests is unresolved
//     (INACTIVE) rather than assumed fine — `deskmanifest lint`'s
//     checkResolution is what should have caught that as a problem before
//     this ever runs in production, but this file fails closed on it
//     regardless of whether the lint ran.
func ComputeActivation(manifests []componentManifest, ext map[string]ExtKeyResult, trustConfigured bool) map[string]ActivationResult {
	providers := map[string]string{} // key -> the FIRST component that provides it
	for _, m := range manifests {
		for _, p := range m.Provides {
			if _, dup := providers[p]; !dup {
				providers[p] = m.Component
			}
		}
	}
	byName := map[string]*componentManifest{}
	for i := range manifests {
		byName[manifests[i].Component] = &manifests[i]
	}

	results := map[string]ActivationResult{}
	resolving := map[string]bool{} // cycle guard: components on the current resolve stack

	var resolve func(component string) ActivationResult
	resolve = func(component string) ActivationResult {
		if r, done := results[component]; done {
			return r
		}
		if resolving[component] {
			r := ActivationResult{Reason: "cycle among inject.required"}
			return r // NOT cached here: the component that closes the cycle still
			// gets its own cached (false) result from the frame that opened it.
		}
		m, known := byName[component]
		if !known {
			r := ActivationResult{Reason: "no manifest declares this component"}
			results[component] = r
			return r
		}
		resolving[component] = true
		defer delete(resolving, component)

		for _, req := range m.Inject.Required {
			key := req.Key
			switch {
			case key == "assay.roster.trust":
				if !trustConfigured {
					r := ActivationResult{Key: key, Reason: "unset or malformed"}
					results[component] = r
					return r
				}
			case strings.HasPrefix(key, "assay.roster.ext."):
				name := strings.TrimPrefix(key, "assay.roster.ext.")
				if res, ok := ext[name]; ok && res.Status == ExtInvalid {
					reason := res.Reason
					if reason == "" {
						reason = "invalid"
					}
					r := ActivationResult{Key: key, Reason: reason}
					results[component] = r
					return r
				}
			default:
				provider, has := providers[key]
				if !has {
					r := ActivationResult{Key: key, Reason: "no provider declares " + key}
					results[component] = r
					return r
				}
				if pr := resolve(provider); !pr.Active {
					reason := pr.Reason
					if reason == "" {
						reason = "inactive"
					}
					r := ActivationResult{Key: key, Reason: "provider " + provider + " is INACTIVE: " + reason}
					results[component] = r
					return r
				}
			}
		}
		r := ActivationResult{Active: true}
		results[component] = r
		return r
	}

	for _, m := range manifests {
		resolve(m.Component)
	}
	return results
}

// VerbComponent resolves which component OWNS the desk verbs: the manifest
// whose `provides` includes "assay.desk.verbs" — today a SINGLE
// tools/desk/component.yaml covering every desk binary
// (the component-manifest spec §2 treats the whole binary set as one unit; a future
// manifest MAY split verb-level components without this lookup changing,
// since it never hardcodes a binary name). "" means no manifest owns any
// verb at all — the facts note's "a verb with no owning component injects
// assay.roster.trust only" case.
func VerbComponent(manifests []componentManifest) string {
	for _, m := range manifests {
		for _, p := range m.Provides {
			if p == "assay.desk.verbs" {
				return m.Component
			}
		}
	}
	return ""
}

// VerbActivationRefusal computes the three-state could-not-check refusal
// (brief-v1 §8) for the desk-verbs component, given manifests discovered
// under root, or "" when the verb may proceed. It returns "" — proceed —
// in every case this mechanism cannot positively refuse:
//
//   - the desk-verbs component IS active;
//   - no manifest anywhere under root provides assay.desk.verbs at all (falls
//     back to trust-only, already enforced by each write path's own
//     RosterUnconfiguredError());
//   - manifests could not be discovered under root (a verb run outside a
//     checkout degrades to "no activation information", never a refusal the
//     mechanism cannot see the reason for).
func VerbActivationRefusal(root string) string {
	manifests, _, err := DiscoverManifests(root)
	if err != nil || len(manifests) == 0 {
		return ""
	}
	component := VerbComponent(manifests)
	if component == "" {
		return ""
	}
	cfg := EffectiveConfig()
	results := ComputeActivation(manifests, cfg.Ext, cfg.Configured())
	r := results[component]
	if r.Active {
		return ""
	}
	key := r.Key
	if key == "" {
		key = "(unresolved)"
	}
	reason := r.Reason
	if reason == "" {
		reason = "inactive"
	}
	return fmt.Sprintf("could-not-check: assay/%s inactive — %s %s", component, key, reason)
}

// verbActivationRoot resolves the tree root CheckVerbActivation discovers
// component.yaml under. It is best-effort on purpose: a desk verb can run
// from any working directory, including one outside any git checkout at all
// (an operator's home directory, a CI runner's tool cache), and that must
// degrade to "no activation information" rather than a refusal — see
// CheckVerbActivation.
func verbActivationRoot() (string, bool) {
	wd, err := os.Getwd()
	if err != nil {
		return "", false
	}
	top, err := gitcore.Toplevel(wd)
	if err != nil {
		return "", false
	}
	return top, true
}

// CheckVerbActivation is the call every desk command's main() makes right
// after deskkit.EchoEffectiveConfig, before dispatching to its own run(): the
// "verb entrypoint shared by the desk commands" this brief edits. It
// prints the three-state could-not-check refusal to w and returns false when
// the calling verb's owning component is INACTIVE; true otherwise —
// including every case VerbActivationRefusal cannot see a reason to refuse
// (root not discoverable, no owning manifest, manifests unreadable) — a
// caller getting true does NOT mean "verified active", only "not refused
// here"; the trust surface's own RosterUnconfiguredError() callers remain the
// authority on trust itself.
func CheckVerbActivation(w io.Writer) bool {
	root, ok := verbActivationRoot()
	if !ok {
		return true
	}
	if msg := VerbActivationRefusal(root); msg != "" {
		fmt.Fprintln(w, msg)
		return false
	}
	return true
}

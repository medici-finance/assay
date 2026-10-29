// Package attestation owns offline verifier-record judgments, with explicit capability profiles.
package attestation

import (
	"encoding/json"
	"fmt"
	"github.com/medici-finance/assay/deskcore/domain"
	"path"
	"regexp"
	"sort"
	"strings"
)

type Profile uint8

const (
	ContainerV1 Profile = iota
	GeneralV1V2
)
const Title = "[verification-attestation] "
const Preamble = "Dispatcher-owned pre-work attestation. This record contains NO verification result.\n\n"

var hex64Re = regexp.MustCompile(`^[0-9a-f]{64}$`)
var creditHexCommitRe = regexp.MustCompile(`^([0-9a-f]{40}|[0-9a-f]{64})$`)
var creditCheckoutRe = regexp.MustCompile(`^autocrlf=(true|false|input) eol=(lf|crlf|native)$`)
var siblingRepoRe = regexp.MustCompile(`^[A-Za-z0-9._-]+/[A-Za-z0-9._-]+$`)

func CheckoutOK(s string) bool { return creditCheckoutRe.MatchString(s) }

type Binding struct {
	Version                                                                              int
	SourceRepo, TrackingSource, TrackingHomeSHA256, TrackingCheckout                     string `json:",omitempty"`
	Run, Repo, Source, Brief, BriefSHA256, PlanSHA256, HomeSHA256, Model, Tier, Checkout string
	// SiblingSources binds one fetched origin/main commit per further sibling
	// repository the brief declares (#4876): "owner/repo@<40-hex>" entries,
	// comma-joined in FormatSiblingSources order. A string, not a map, so the
	// binding stays comparable; empty, it renders exactly as before. The '@'
	// keeps each commit a bare git SHA to the outbound body scanner.
	SiblingSources string `json:",omitempty"`
}

// ParseSiblingSources decodes SiblingSources. It refuses a malformed entry, a
// repeated repository name, or any spelling FormatSiblingSources would not
// produce, so one set of sibling sources has exactly one rendering.
func ParseSiblingSources(s string) (map[string]string, error) {
	out := map[string]string{}
	if s == "" {
		return out, nil
	}
	names := map[string]bool{}
	for _, entry := range strings.Split(s, ",") {
		repo, commit, ok := strings.Cut(entry, "@")
		_, name, _ := strings.Cut(repo, "/")
		if !ok || !siblingRepoRe.MatchString(repo) || len(commit) != 40 || !creditHexCommitRe.MatchString(commit) {
			return nil, fmt.Errorf("sibling source %q is not owner/repo@<40-hex commit>", entry)
		}
		if names[strings.ToLower(name)] {
			return nil, fmt.Errorf("sibling repository name %q is bound twice", name)
		}
		names[strings.ToLower(name)] = true
		out[repo] = commit
	}
	if FormatSiblingSources(out) != s {
		return nil, fmt.Errorf("sibling sources are not in canonical order")
	}
	return out, nil
}

// FormatSiblingSources renders sibling sources sorted by repository.
func FormatSiblingSources(m map[string]string) string {
	repos := make([]string, 0, len(m))
	for repo := range m {
		repos = append(repos, repo)
	}
	sort.Strings(repos)
	for i, repo := range repos {
		repos[i] = repo + "@" + m[repo]
	}
	return strings.Join(repos, ",")
}

// siblingSourcesProblem keeps every sibling distinct from both repositories
// already bound, by repository name: a coverage check matches a ../<name>/
// declaration to exactly one repo-source key by that name.
func siblingSourcesProblem(b Binding) string {
	siblings, err := ParseSiblingSources(b.SiblingSources)
	if err != nil {
		return "invalid sibling sources: " + err.Error()
	}
	for repo := range siblings {
		_, name, _ := strings.Cut(repo, "/")
		for _, bound := range []string{b.Repo, b.SourceRepo} {
			_, boundName, _ := strings.Cut(bound, "/")
			if strings.EqualFold(name, boundName) {
				return "a sibling source repeats a repository the binding already names"
			}
		}
	}
	return ""
}

func BindingProblem(b Binding, repo string, profile Profile, sourceAllowed bool) string {
	if profile != ContainerV1 && profile != GeneralV1V2 {
		return "unsupported attestation profile"
	}
	switch {
	case b.Version == 2 && profile == ContainerV1:
		return "version 2 (cross-repository) attestations are refused for container runs"
	case b.Version != 1 && b.Version != 2:
		return fmt.Sprintf("binding version %d is not accepted", b.Version)
	case b.Repo != repo:
		return fmt.Sprintf("the binding's repo %s is not the repository read, %s", b.Repo, repo)
	case !hex64Re.MatchString(b.Run), len(b.Source) != 40, !creditHexCommitRe.MatchString(b.Source),
		!hex64Re.MatchString(b.HomeSHA256), !hex64Re.MatchString(b.BriefSHA256), !hex64Re.MatchString(b.PlanSHA256):
		return "invalid binding (run, source or a digest is not hex)"
	case b.Version == 1 && (b.SourceRepo != "" || b.TrackingSource != "" || b.TrackingHomeSHA256 != "" || b.TrackingCheckout != "" || b.SiblingSources != ""):
		return "a same-repository binding carries cross-repository fields"
	}
	if b.Version == 2 {
		if !sourceAllowed || strings.EqualFold(b.SourceRepo, b.Repo) || len(b.TrackingSource) != 40 || !creditHexCommitRe.MatchString(b.TrackingSource) || !hex64Re.MatchString(b.TrackingHomeSHA256) {
			return "invalid cross-repository verifier binding"
		}
		if !CheckoutOK(b.TrackingCheckout) {
			return "invalid tracking checkout conversion"
		}
		if why := siblingSourcesProblem(b); why != "" {
			return why
		}
	}
	brief := strings.TrimSpace(strings.ReplaceAll(b.Brief, "\\", "/"))
	if brief != "" {
		brief = path.Clean(brief)
	}
	// A cleaned path of exactly ".." (or the root ".") names no file inside the
	// repository, so it is refused alongside an absolute or "../"-prefixed path.
	if brief == "" || brief == "." || brief == ".." || strings.HasPrefix(brief, "/") || strings.HasPrefix(brief, "../") {
		return fmt.Sprintf("the binding's brief %s is not a repository path", b.Brief)
	}
	if !ModelSlugOK(b.Model) {
		return fmt.Sprintf("the binding's model %q is not a model slug", b.Model)
	}
	if t := strings.ToLower(strings.TrimSpace(b.Tier)); t != "any" && t != "strong" {
		return fmt.Sprintf("the binding's tier %q is not an exec-tier value", b.Tier)
	}
	if !CheckoutOK(b.Checkout) {
		return "invalid binding checkout conversion"
	}
	return ""
}
func ModelSlugOK(slug string) bool {
	s := strings.ToLower(strings.TrimSpace(slug))
	alnum := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			alnum = true
		case r == '-' || r == '.':
		default:
			return false
		}
	}
	return alnum
}
func Body(b Binding) string {
	data, _ := json.MarshalIndent(b, "", "  ")
	return Preamble + string(data) + "\n"
}

// TargetProblem checks immutable content, leaving authority observation to its adapter.
func TargetProblem(present, isPR, authority bool, title, body, wantTitle, wantBody string) string {
	switch {
	case !present:
		return "missing run record"
	case isPR:
		return "run record is a pull request"
	case !authority:
		return "run record author is not the dispatcher"
	case title != wantTitle:
		return fmt.Sprintf("has title %q, not %q", title, wantTitle)
	case body != wantBody:
		return "has a body that is not the exact rendering of its binding"
	}
	return ""
}
func HistoryProblem(known, complete, edited bool) string {
	switch {
	case !known:
		return "body history was not reported (bodyHistoryKnown is false or absent)"
	case !complete:
		return "body history is incomplete"
	case edited:
		return "body was edited"
	}
	return ""
}

type LabelEvent struct {
	Label              string
	Removed, Authority bool
}

// Stamp consumes current labels and their complete, oldest-first standing history.
func Stamp(present []string, events []LabelEvent) (model, tier, why string) {
	norm := func(s string) string { return strings.ToLower(strings.TrimSpace(s)) }
	isStamp := func(n string) bool {
		return strings.HasPrefix(n, "dispatched-model:") || strings.HasPrefix(n, "dispatched-tier:")
	}
	standing := map[string]bool{}
	for _, e := range events {
		n := norm(e.Label)
		if !isStamp(n) {
			continue
		}
		if e.Removed {
			delete(standing, n)
		} else {
			standing[n] = e.Authority
		}
	}
	models, tiers := map[string]bool{}, map[string]bool{}
	seen := map[string]bool{}
	for _, l := range present {
		n := norm(l)
		if !isStamp(n) || seen[n] {
			continue
		}
		seen[n] = true
		auth, ok := standing[n]
		if !ok {
			return "", "", fmt.Sprintf("the stamp label %s has no standing application in the label history", n)
		}
		if !auth {
			return "", "", fmt.Sprintf("the stamp label %s was applied by an actor that is not the dispatcher", n)
		}
		if v, ok := strings.CutPrefix(n, "dispatched-model:"); ok {
			models[strings.TrimSpace(v)] = true
		} else {
			tiers[strings.TrimSpace(strings.TrimPrefix(n, "dispatched-tier:"))] = true
		}
	}
	if len(models) == 0 && len(tiers) == 0 {
		return "", "", "it carries no dispatcher model stamp"
	}
	if len(models) != 1 || len(tiers) != 1 {
		return "", "", "its dispatcher stamp is not one model and one tier"
	}
	for m := range models {
		model = m
	}
	for t := range tiers {
		tier = t
	}
	if model == "" || (tier != "any" && tier != "strong") {
		return "", "", "its dispatcher stamp is not one model and one tier"
	}
	return model, tier, ""
}

// ParseBody decodes binding data; TargetProblem separately authenticates exact rendering.
func ParseBody(body string) (Binding, error) {
	var b Binding
	_, raw, ok := strings.Cut(body, "\n\n")
	if !ok {
		return b, fmt.Errorf("body is not a binding")
	}
	if err := domain.DecodeStrict([]byte(raw), &b); err != nil {
		// Keep the established CLI vocabulary while the strict decoder owns rejection.
		if strings.Contains(err.Error(), "is not a field of this object") {
			return b, fmt.Errorf("unknown field: %w", err)
		}
		return b, err
	}
	return b, nil
}

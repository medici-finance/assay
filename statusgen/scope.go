package main

import (
	"path/filepath"
	"strings"
)

// scope.go — product-scoping for --lint.
//
// statusgen lints every stream across all products at once, so one product's
// problem can red-gate another product's PR (the PR class). The `serves:`
// stream-frontmatter tag (example-app | example-service | assay | platform)
// is the product axis; this file lets a lint run be restricted to a single
// product so a doc PR in one product is not gated by another's streams.
//
// Two entry points: an explicit --scope <product>, or auto-derivation from the
// CI-supplied --changed set (deriveScope). `platform` is the shared/cross-cutting
// bucket — it is ALWAYS included in a scoped run (a house-global invariant is
// never scoped away) and a platform-only change never narrows the scope.

// validServes is the canonical product set (mirrors roadmap.go's servesOrder,
// minus the "" untagged sentinel). --scope must be one of these.
var validServes = map[string]bool{
	"example-app":     true,
	"example-service": true,
	"assay":           true,
	"platform":        true,
}

// sharedServes is the cross-cutting bucket always included in a scoped run.
const sharedServes = "platform"

// filterStreamsByServes returns the streams in-scope for `scope`: those whose
// serves matches, plus every shared (platform) stream. Same underlying *Stream
// pointers — this narrows the per-stream CHECK set only, never the view build.
func filterStreamsByServes(streams []*Stream, scope string) []*Stream {
	out := make([]*Stream, 0, len(streams))
	for _, s := range streams {
		if s.Serves == scope || s.Serves == sharedServes {
			out = append(out, s)
		}
	}
	return out
}

// streamNameOf returns the stream name a changed repo-relative path belongs to
// (the <name> in docs/streams/<name>/...), and whether the path is a per-stream
// path at all. A register path (docs/streams/findings/…, …/intake/…, or a bare
// docs/streams/FINDINGS.md) is NOT a stream — it is house-global — so returns
// ("", false), same as any non-docs/streams path.
func streamNameOf(p string, known map[string]bool) (string, bool) {
	p = filepath.ToSlash(p)
	const pre = "docs/streams/"
	if !strings.HasPrefix(p, pre) {
		return "", false
	}
	rest := strings.TrimPrefix(p, pre)
	seg := rest
	if i := strings.IndexByte(rest, '/'); i >= 0 {
		seg = rest[:i]
	} else {
		// docs/streams/<file> — a top-level register file, not a stream dir.
		return "", false
	}
	if !known[seg] {
		return "", false
	}
	return seg, true
}

// deriveScope infers a product scope from the CI-supplied changed-set. It
// returns a product ONLY when the change is unambiguously confined to a single
// non-shared product's streams; otherwise it returns "" (run unscoped — today's
// behavior), which is the safe default:
//   - empty changed-set (main regen / no signal) → "" (whole house)
//   - any changed path outside a known stream dir (tooling, k8s/, a register,
//     a root doc) → "" (a shared surface can affect any product)
//   - any changed stream that is untagged (serves: "") → "" (unknown product)
//   - changed streams spanning >1 product, or only the shared bucket → ""
//   - exactly one non-shared product P across all changed streams → P
func deriveScope(streams []*Stream, changed []string) string {
	if len(changed) == 0 {
		return ""
	}
	known := make(map[string]bool, len(streams))
	serves := make(map[string]string, len(streams))
	for _, s := range streams {
		known[s.Name] = true
		serves[s.Name] = s.Serves
	}
	products := map[string]bool{}
	for _, p := range changed {
		name, ok := streamNameOf(p, known)
		if !ok {
			return "" // a non-stream / shared-surface change — cannot scope
		}
		sv := serves[name]
		if sv == "" {
			return "" // an untagged stream changed — unknown product
		}
		products[sv] = true
	}
	// One distinct product, and it is not the shared bucket → scope to it.
	if len(products) == 1 {
		for p := range products {
			if p == sharedServes {
				return ""
			}
			return p
		}
	}
	return ""
}

// servesCoverageNotices flags any stream missing a serves: tag — it can never
// be product-scoped, so the taxonomy would silently degrade as streams are
// added. Advisory (NOTICE), never a hard gate.
func servesCoverageNotices(streams []*Stream) []string {
	var out []string
	for _, s := range streams {
		if s.Serves == "" {
			out = append(out, s.Name+": no serves: product tag — cannot be product-scoped; tag it example-app|example-service|assay|platform")
		}
	}
	return out
}

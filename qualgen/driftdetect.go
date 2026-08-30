package main

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/object"
)

// errFoundStop is a control-flow sentinel used ONLY to short-circuit a tree
// walk once a match is found; it never escapes treeContainsElsewhere as a real
// error.
var errFoundStop = errors.New("qualgen: match found, stop iteration")

// driftdetect.go is the GENERIC source↔render drift-detection capability
// (spec §4.6): given a source artifact and the render/target artifact(s) it is
// supposed to track, resolved at a given commit, it reports a three-state
// drift verdict per target plus the evidence behind it. It is deliberately NOT
// tied to any one document pipeline — a doc referencing a file/symbol/typed ID
// is one source↔render pair; a template and its rendered output would be
// another — so any source↔render pair is checkable through this one capability
// rather than a second, parallel drift checker.
//
// instructionbrittle.go's trended reference-validity is this capability's
// measured, historical generalization: it calls driftdetect once per history
// window rather than growing its own resolution path (task 1/2, Verify row 6).

// TargetKind classifies what a source artifact points at and expects to still
// resolve in the render/target it tracks.
type TargetKind string

const (
	// TargetFilePath is a file-path reference, resolved against the tree.
	TargetFilePath TargetKind = "file-path"
	// TargetSymbol is a named code symbol, resolved by searching tracked blob
	// content elsewhere in the tree for the identifier.
	TargetSymbol TargetKind = "symbol"
	// TargetTypedID is a structured ID token (matched by a configured
	// pattern), resolved the same way as a symbol: a live referent is the
	// literal token appearing somewhere else in the tree.
	TargetTypedID TargetKind = "typed-id"
)

// Target is one thing a source artifact points at and expects to still be
// true of its render/target — a referenced file path, a named code symbol, or
// a typed ID. It carries no assumption about which document pipeline it came
// from.
type Target struct {
	Kind  TargetKind
	Value string // the literal token as the source recorded it
}

// Verdict is the three-state drift outcome for one Target at one commit.
// Distinct from the Measure[T] three-state (measured/measured-zero/
// could-not-measure, quality/01): driftdetect is a generic point-in-time
// capability, not itself a metric instrument. instructionbrittle.go is the
// caller that translates a Verdict into a Measure[bool] for its own scoring
// (verdictToValidity).
type Verdict string

const (
	// VerdictInSync: the target resolves — the render/target still matches
	// what the source expects.
	VerdictInSync Verdict = "in-sync"
	// VerdictDrifted: the target does not resolve — the source is stale.
	VerdictDrifted Verdict = "drifted"
	// VerdictCouldNotCheck: resolution itself failed (a tree lookup error, an
	// unregistered target kind) — distinct from a genuine drifted verdict.
	VerdictCouldNotCheck Verdict = "could-not-check"
)

// Result is the evidence for one resolved Target.
type Result struct {
	Target  Target
	Verdict Verdict
	// Reason explains the verdict: always populated for Drifted and
	// CouldNotCheck; empty is fine for InSync (the positive case needs no
	// explanation).
	Reason string
}

// Context is the render/target state a Target is resolved against: the tree
// at one commit, plus the source artifact's own path so a resolver can
// exclude the source's self-mention of a token from counting as evidence that
// the token is still "live" elsewhere.
type Context struct {
	Tree       *object.Tree
	CommitSHA  string
	SourcePath string
}

// driftdetect resolves a single Target against ctx using the kind-appropriate
// resolver. This is the one shared resolution path (Verify row 6): a target
// kind not registered here is reported could-not-check rather than silently
// skipped.
func driftdetect(ctx Context, target Target) Result {
	switch target.Kind {
	case TargetFilePath:
		return resolveFilePath(ctx, target)
	case TargetSymbol:
		return resolveWholeWordElsewhere(ctx, target, "symbol")
	case TargetTypedID:
		return resolveWholeWordElsewhere(ctx, target, "typed ID")
	default:
		return Result{
			Target:  target,
			Verdict: VerdictCouldNotCheck,
			Reason:  fmt.Sprintf("no resolver registered for target kind %q", target.Kind),
		}
	}
}

// resolveFilePath resolves a file-path Target directly against ctx.Tree.
func resolveFilePath(ctx Context, target Target) Result {
	path := strings.TrimPrefix(target.Value, "./")
	if _, err := ctx.Tree.FindEntry(path); err != nil {
		if err == object.ErrEntryNotFound || err == object.ErrDirectoryNotFound {
			return Result{
				Target:  target,
				Verdict: VerdictDrifted,
				Reason:  fmt.Sprintf("path %q does not resolve in the tree at %s", target.Value, ctx.CommitSHA),
			}
		}
		return Result{
			Target:  target,
			Verdict: VerdictCouldNotCheck,
			Reason:  fmt.Sprintf("tree lookup error for %q at %s: %v", target.Value, ctx.CommitSHA, err),
		}
	}
	return Result{Target: target, Verdict: VerdictInSync}
}

// resolveWholeWordElsewhere resolves a symbol or typed-ID Target by searching
// every tracked blob in ctx.Tree OTHER than ctx.SourcePath for the literal
// token as a whole word. A live referent means the token is DEFINED or
// recorded somewhere else in the tree, not merely mentioned by the doc that
// references it — the doc's own mention of its dead symbol/ID must never
// count as the evidence that keeps it alive.
func resolveWholeWordElsewhere(ctx Context, target Target, kindLabel string) Result {
	name := strings.TrimSuffix(target.Value, "()")
	re, err := regexp.Compile(`\b` + regexp.QuoteMeta(name) + `\b`)
	if err != nil {
		return Result{Target: target, Verdict: VerdictCouldNotCheck, Reason: fmt.Sprintf("could not build search pattern for %q: %v", name, err)}
	}
	found, err := treeContainsElsewhere(ctx.Tree, ctx.SourcePath, re)
	if err != nil {
		return Result{Target: target, Verdict: VerdictCouldNotCheck, Reason: fmt.Sprintf("tree scan failed for %q: %v", target.Value, err)}
	}
	if found {
		return Result{Target: target, Verdict: VerdictInSync}
	}
	return Result{
		Target:  target,
		Verdict: VerdictDrifted,
		Reason:  fmt.Sprintf("%s %q has no live referent elsewhere in the tree at %s", kindLabel, target.Value, ctx.CommitSHA),
	}
}

// treeContainsElsewhere reports whether any blob in tree, other than the one
// at excludePath, has content matching re. Binary/unreadable blobs are
// skipped rather than erroring the whole scan — a single unreadable file must
// not turn every reference resolution in the tree into could-not-check.
func treeContainsElsewhere(tree *object.Tree, excludePath string, re *regexp.Regexp) (bool, error) {
	iter := tree.Files()
	defer iter.Close()
	found := false
	err := iter.ForEach(func(f *object.File) error {
		if f.Name == excludePath {
			return nil
		}
		if bin, err := f.IsBinary(); err != nil || bin {
			return nil
		}
		content, err := f.Contents()
		if err != nil {
			return nil
		}
		if re.MatchString(content) {
			found = true
			return errFoundStop
		}
		return nil
	})
	if found {
		return true, nil
	}
	if err != nil && err != errFoundStop {
		return false, err
	}
	return false, nil
}

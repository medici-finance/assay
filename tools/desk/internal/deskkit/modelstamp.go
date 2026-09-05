package deskkit

// modelstamp.go — the `dispatched-model:<slug>` + `dispatched-tier:<strong|any>`
// dispatcher-attestation stamp, and the READER contract every consumer of it must use.
//
// WHAT THIS IS. In an agentic fleet the actor is a tuple — role-App × model (strength) ×
// area — and the model axis is the one currently unusable as a metric input, because
// which model actually ran is honor-system self-report (the exec-tier rule is explicit:
// "statusgen never verifies which model actually ran — pickup-side compliance is
// honor-system self-report"). This file declares the ONE input that is NOT self-report:
// the DISPATCHER knows exactly which model it launched, and its stamp on the worker's PR
// is other-actor attestation the trust rules already allow.
//
// TWO LABELS, applied by the DISPATCHER. Immediately after a worker's draft PR opens, the
// desk (the dispatcher — the desk App identity, the OTHER actor, never the worker session
// itself) applies two labels:
//
//	dispatched-model:<slug>   the lowercase model slug it launched (e.g. opus-4.8, kimi-3,
//	                          glm5.2). An OPEN vocabulary — new models appear — so the writer
//	                          validates slug SHAPE, not membership in a fixed list.
//	dispatched-tier:<tier>    the strength tier: one of the brief-schema `exec-tier:` values
//	                          (strong | any). A CLOSED vocabulary, validated on write.
//
// THE CLAIM IS PRECISE — ATTESTATION OF DISPATCH, NOT SURVEILLANCE OF EXECUTION. The
// dispatcher attests what it LAUNCHED. It does not claim to have observed every token the
// session later produced; a session that silently swapped models mid-run is outside what
// this stamp can see. That bound is the difference between this input and the exec-tier
// honor-system it repairs, and it is why the tier axis is worth trusting only as far as
// "the dispatcher launched a strong-tier runner", never "a strong model produced this
// diff".
//
// ONE DECLARED SOURCE FOR THE TIER VOCABULARY (derive-or-diff). The tier set is NOT a
// fresh list invented here. It IS the brief-schema `exec-tier:` value set — the same
// `{any, strong}` that `statusgen/brieffile.go`'s `validExecTier` enforces on every brief.
// A second hand-maintained tier list is exactly the drift the `raised-by` reader
// (raisedby.go) exists to prevent: the metric would keep reporting confidently against a
// vocabulary nobody was emitting. statusgen and this tool are SEPARATE Go modules with no
// dependency edge, so `validExecTier` (unexported, and across a module boundary) cannot be
// imported; this file therefore MIRRORS it, and pins the mirror with a test
// (TestModelStampTierVocabularyIsExecTierSet) so a divergence is loud at build time rather
// than silent in a dashboard. If the modules are ever unified, collapse this to a single
// declaration — the mirror is a boundary artifact, not a design choice.
//
// THREE-STATE, and it is the load-bearing part. A metric over these
// labels has THREE answers per PR, never two:
//
//	ModelStamped        one dispatched-model label naming a non-empty slug AND one
//	                    dispatched-tier label naming a valid tier — the complete stamp.
//	ModelUnknown        NO stamp. This is the ZERO VALUE, deliberately, and it is NEVER a
//	                    default model: an unstamped PR contributes no model-keyed signal,
//	                    full stop.
//	ModelIndeterminate  the stamp is present but unreadable — conflicting labels (two
//	                    different model slugs, or two different tiers), a malformed half
//	                    (empty slug, a tier outside the set), only ONE of the two labels
//	                    (an incomplete stamp), or a stamp applied by a NON-DISPATCHER
//	                    identity (see below).
//
// UNKNOWN IS NOT A DEFAULT MODEL. Backfill is out of scope: every pre-adoption PR reads
// ModelUnknown forever, and per-model history starts accumulating honestly at adoption.
// The entire standing corpus is unstamped, so a reader that collapsed Unknown into any
// default model would land the whole backlog in one bucket and report a confident wrong
// number. There is no default here to choose: the zero value is Unknown, so a consumer
// that forgets to handle the state gets the non-answer rather than an answer.
//
// SELF-REPORTED TAGS ARE WORTHLESS BY DESIGN. A `dispatched-model:` label applied by the
// WORKER's own App — or any identity that is not the dispatcher — reads Indeterminate, not
// Stamped. That is the whole point: a stamp anyone could apply to themselves is the
// exec-tier honor-system with extra steps. The label-content reader (ModelStampOf) cannot
// see WHO applied a label; the applier-aware reader (AttestedModelStampOf) takes a
// StampTimeline (the labels the PR CARRIES plus the ordered label events) and downgrades to
// Indeterminate when the STANDING application of a dispatched-* label came from a
// non-dispatcher. A consumer that needs the strong form MUST use the applier-aware reader;
// the content reader answers only "what do the labels say", not "may I trust them".
//
// STANDING, NOT HISTORICAL — and it is a correctness property, not a nicety. A GitHub
// timeline is APPEND-ONLY. A reader that took ANY historical `labeled` event as the applier
// made a foreign stamp PERMANENT: the real dispatcher could remove the labels and re-apply
// them under its own identity, and the reader still found the original foreign event and
// refused every authority-bearing write on that PR forever, with no repair available to
// anyone. The applier is therefore the actor of the LAST `labeled` event for each currently
// present label that is not followed by an `unlabeled` of the same name. A foreign stamp
// that is still standing is NOT laundered by this — only a genuine re-stamp is honoured.
//
// SINGLE-POINT-OF-FAILURE, and the layer behind it. The one fragile step is the dispatcher
// applying the labels reliably. The reader's ModelUnknown is fail-safe underneath it — a
// silently broken stamp step yields unstamped PRs that simply contribute no signal, never
// a wrong one — and a diagnostics reader can print the stamped-share so a broken
// step is visible in the dailies rather than silent.
//
// CONSUMER BOUNDARY. The per-model / per-tier failure-rate tables (the reputation-metrics
// family) and the demanded-tier-vs-dispatched-tier mismatch NOTICE
// (a later reader) are the readers this exists for, and NEITHER is implemented here. What
// they need from this file is: the two labels, the tier vocabulary, and the three states
// as first-class values they can report separately rather than fold into a denominator.

import (
	"sort"
	"strings"
)

// DispatchedModelPrefix carries the model slug the dispatcher launched. It is the ONE
// spelling of it: writers build the label through DispatchedModelLabel and readers match
// through ModelStampOf, so no consumer restates the string.
const DispatchedModelPrefix = "dispatched-model:"

// DispatchedTierPrefix carries the strength tier the dispatcher launched. One spelling,
// same discipline as the model prefix.
const DispatchedTierPrefix = "dispatched-tier:"

// execTierValues is the tier vocabulary — a MIRROR of the brief-schema `exec-tier:` value
// set (statusgen/brieffile.go `validExecTier` = {any, strong}). See the file comment for
// why it is mirrored rather than imported (separate Go modules) and why
// TestModelStampTierVocabularyIsExecTierSet pins it. It is declared ONCE here and every
// caller — writer and reader — projects from it, so there is no second in-package list to
// drift.
var execTierValues = []string{"any", "strong"}

// DispatchTiers returns the tier vocabulary, sorted, as a fresh slice a caller may not
// mutate the backing of. It is the whole set: rendering usage text and validating a tier
// argument both go through this rather than restating {any, strong}.
func DispatchTiers() []string {
	out := make([]string, len(execTierValues))
	copy(out, execTierValues)
	sort.Strings(out)
	return out
}

func isDispatchTier(tier string) bool {
	for _, t := range execTierValues {
		if t == tier {
			return true
		}
	}
	return false
}

// ModelState is the three-state answer to "which model (and tier) did the dispatcher
// attest for this PR?".
type ModelState int

const (
	// ModelUnknown is the ZERO VALUE and means NO stamp was found. It is a non-answer,
	// never a default model: an unstamped PR contributes no model-keyed signal. A
	// consumer that treats it as any particular model is asserting something the data
	// does not say.
	ModelUnknown ModelState = iota
	// ModelStamped means one dispatched-model label named a non-empty slug and one
	// dispatched-tier label named a valid tier — the complete stamp.
	ModelStamped
	// ModelIndeterminate is the could-not-check state: conflicting labels, a malformed
	// half (empty slug or an out-of-vocabulary tier), an incomplete stamp (only one of
	// the two labels), or a stamp applied by a non-dispatcher identity. The PR asserts a
	// dispatch attestation this package cannot resolve to one (model, tier).
	ModelIndeterminate
)

func (s ModelState) String() string {
	switch s {
	case ModelStamped:
		return "stamped"
	case ModelIndeterminate:
		return "indeterminate"
	default:
		return "unknown"
	}
}

// Answered reports whether the state names a (model, tier). It exists so a consumer writes
// `if !state.Answered()` rather than comparing against one of the two non-answers and
// forgetting the other — the enumeration that omits Indeterminate is the bug this
// prevents.
func (s ModelState) Answered() bool { return s == ModelStamped }

// ModelStamp is the (model, tier) a Stamped PR carries. Both fields are "" for every state
// except ModelStamped, so a consumer that reads the fields without checking the state
// still sees the non-answer rather than a partial one.
type ModelStamp struct {
	Model string // lowercase model slug, verbatim from the label
	Tier  string // "strong" | "any"
}

// DispatchedModelLabel validates a model slug's SHAPE and returns the label to apply. It
// is the ONLY admissible way to construct the model half of a stamp: a caller that formats
// the string itself can publish a slug carrying a space, an uppercase run, or a personal
// handle onto every PR it touches.
//
// The vocabulary is OPEN (new models appear), so this validates shape, not membership:
// lowercased, non-empty, and made only of [a-z0-9.-] with at least one alphanumeric rune.
// That is the same lowercase/dash shape RaisedByLabel enforces, for the same reason — a
// label is published, so a human name or id must not be able to reach one.
func DispatchedModelLabel(slug string) (string, error) {
	want := strings.ToLower(strings.TrimSpace(slug))
	if want == "" {
		return "", Refused("dispatched-model: an empty slug is not a stamp — " +
			"omit the label entirely if the dispatched model is genuinely unrecorded, so the PR reads " +
			"UNKNOWN rather than carrying a blank model")
	}
	alnum := false
	for _, r := range want {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			alnum = true
		case r == '-' || r == '.':
			// separators, fine
		default:
			return "", Refused("dispatched-model: slug " + StripControl(slug) +
				" carries a character outside [a-z0-9.-] — a model slug is lowercase letters, digits, " +
				"dot and dash only, so a space, an uppercase run or a personal handle cannot reach a " +
				"published label")
		}
	}
	if !alnum {
		return "", Refused("dispatched-model: slug " + StripControl(slug) +
			" has no alphanumeric character — a run of only separators is not a model slug")
	}
	return DispatchedModelPrefix + want, nil
}

// DispatchedTierLabel validates a tier against the exec-tier vocabulary and returns the
// label to apply. It is the ONLY admissible way to construct the tier half of a stamp: a
// caller that formats the string itself can stamp a tier outside {any, strong}, and a
// metric grouping by a tier nobody is bound to reports a category with one member forever.
//
// The refusal enumerates the vocabulary, because "invalid tier" without the set is a
// refusal the caller cannot act on.
func DispatchedTierLabel(tier string) (string, error) {
	want := strings.ToLower(strings.TrimSpace(tier))
	if want == "" {
		return "", Refused("dispatched-tier: an empty tier is not a stamp — " +
			"the tier vocabulary is the brief-schema exec-tier set (" + strings.Join(DispatchTiers(), " | ") +
			"); omit the label if the tier is genuinely unrecorded")
	}
	if !isDispatchTier(want) {
		return "", Refused("dispatched-tier: " + StripControl(tier) +
			" is not an exec-tier value. The vocabulary is the brief-schema exec-tier set: " +
			strings.Join(DispatchTiers(), " | ") + " — it is DERIVED from that one set, not a second " +
			"list, so a tier outside it groups the metric by a value nothing emits")
	}
	return DispatchedTierPrefix + want, nil
}

// ModelStampLabels builds BOTH halves at once — the two labels the dispatcher applies to a
// PR — validating each. It returns an error if either half is invalid, so a caller never
// applies a half-stamp from a bad input (the incomplete-stamp state exists for transport
// failures, not for a writer that should have refused up front).
func ModelStampLabels(slug, tier string) (labels []string, err error) {
	ml, err := DispatchedModelLabel(slug)
	if err != nil {
		return nil, err
	}
	tl, err := DispatchedTierLabel(tier)
	if err != nil {
		return nil, err
	}
	return []string{ml, tl}, nil
}

// ModelStampOf is the label-CONTENT reader: given a PR's label names, answer which model
// and tier the stamp names, in three states. It answers "what do the labels say", NOT "may
// I trust them" — a consumer that needs the self-report defence must use
// AttestedModelStampOf, which additionally checks WHO applied the labels.
//
// The slug/tier are returned lowercased and trimmed, verbatim from the labels; the slug is
// NOT re-validated for shape (a stamp is a historical fact about a filing, like
// RaisedByOf's role — re-validating at read time rewrites history as the vocabulary
// evolves). An empty half, a tier outside the set, a conflict, or an incomplete stamp all
// resolve to Indeterminate. Both returned fields are "" for every state except
// ModelStamped.
func ModelStampOf(labels []string) (ModelStamp, ModelState) {
	modelFound := ""
	modelCount := 0
	modelEmpty := false
	modelConflict := false

	tierFound := ""
	tierCount := 0
	tierEmpty := false
	tierBad := false
	tierConflict := false

	for _, l := range labels {
		name := strings.ToLower(strings.TrimSpace(l))
		switch {
		case strings.HasPrefix(name, DispatchedModelPrefix):
			modelCount++
			v := strings.TrimSpace(strings.TrimPrefix(name, DispatchedModelPrefix))
			if v == "" {
				modelEmpty = true
				continue
			}
			if modelFound != "" && modelFound != v {
				modelConflict = true
			}
			modelFound = v
		case strings.HasPrefix(name, DispatchedTierPrefix):
			tierCount++
			v := strings.TrimSpace(strings.TrimPrefix(name, DispatchedTierPrefix))
			if v == "" {
				tierEmpty = true
				continue
			}
			if !isDispatchTier(v) {
				tierBad = true
				continue
			}
			if tierFound != "" && tierFound != v {
				tierConflict = true
			}
			tierFound = v
		}
	}

	// No stamp at all — the zero value, and never a default.
	if modelCount == 0 && tierCount == 0 {
		return ModelStamp{}, ModelUnknown
	}

	// Any malformed or conflicting half is a could-not-check. So is a HALF stamp: the
	// stamp is two labels, and one label present is an incomplete attestation (a
	// transport failure between the two applies), not a readable one.
	switch {
	case modelConflict || tierConflict,
		modelEmpty || tierEmpty,
		tierBad,
		modelCount == 0 || tierCount == 0,
		modelFound == "" || tierFound == "":
		return ModelStamp{}, ModelIndeterminate
	}

	// modelCount/tierCount may exceed 1 here only when every label of that kind named the
	// SAME value (a duplicate label is not a conflict).
	return ModelStamp{Model: modelFound, Tier: tierFound}, ModelStamped
}

// LabelEvent is ONE label mutation from the issue/PR timeline — a `labeled` or an
// `unlabeled` event, and the login that performed it. AttestedModelStampOf needs the actor
// because a self-applied dispatched-* label is worthless by design; it needs the REMOVALS
// because a GitHub timeline is APPEND-ONLY, so without them a label applied once by a
// foreign login can never be repaired (see StampTimeline).
type LabelEvent struct {
	Name      string // the label name, e.g. "dispatched-model:opus-4.8"
	AppliedBy string // GitHub login of the actor who added (or removed) the label
	Removed   bool   // true for an `unlabeled` event: this application was superseded
}

// StampTimeline is the applier-aware reader's whole input: the labels CURRENTLY on the PR,
// and the ordered label events that put them there.
//
// WHY BOTH, AND WHY THE EVENTS ARE NOT ENOUGH. A GitHub timeline is APPEND-ONLY: a
// `labeled` event is never rewritten or deleted, so a reader that treats ANY historical
// `labeled` event for a dispatched-* label as "the applier" makes a foreign stamp permanent
// — the real dispatcher can remove the label and re-apply it under its own identity and the
// reader still sees the original foreign event, so the PR is refused forever and there is no
// repair. Present + ordered events is what makes a genuine RE-STAMP readable: for each label
// still on the PR, the applier is the actor of the LAST `labeled` event not followed by an
// `unlabeled` of the same name.
//
// WHY THE PRESENT SET IS SEPARATE AND AUTHORITATIVE. Presence is NOT derived from the
// events, because a truncated or partial timeline read would then make a present stamp look
// ABSENT — and absent proceeds on the NOTICE path. That is the one direction this floor must
// never fail. So the present set comes from the labels API, and a present dispatched-* label
// the events cannot attribute is could-not-check (UNREADABLE), never unstamped.
//
// Events must be OLDEST FIRST — the order both forge timeline reads return them in.
type StampTimeline struct {
	Present []string     // label names currently on the PR, from the labels read
	Events  []LabelEvent // `labeled`/`unlabeled` events, OLDEST FIRST
}

// normLabel is the ONE spelling of label-name normalization the stamp readers share, so the
// present set and the event stream cannot key on subtly different strings.
func normLabel(name string) string { return strings.ToLower(strings.TrimSpace(name)) }

// isStampLabelName reports whether a NORMALIZED label name is one half of a dispatch stamp.
func isStampLabelName(name string) bool {
	return strings.HasPrefix(name, DispatchedModelPrefix) || strings.HasPrefix(name, DispatchedTierPrefix)
}

// presentStampLabels returns the normalized, de-duplicated dispatched-* labels currently on
// the PR, sorted, so every caller enumerates the same set in the same order.
func presentStampLabels(present []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, l := range present {
		n := normLabel(l)
		if !isStampLabelName(n) || seen[n] {
			continue
		}
		seen[n] = true
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// resolveStampAppliers answers, for every dispatched-* label CURRENTLY present, WHO applied
// the standing application of it — and names the present labels no standing application
// could be found for.
//
// THE RULE, and it is the whole fix. Walk the events in order; a `labeled` event records its
// actor as the standing applier of that name, and a later `unlabeled` of the same name
// SUPERSEDES it (the application it recorded is gone). What survives the walk is, per label
// name, the actor of the last `labeled` event not followed by an `unlabeled` — which is
// exactly the application the PR is carrying now. A foreign stamp that was removed and
// re-applied by the dispatcher therefore reads as the DISPATCHER's; a foreign stamp that is
// still standing reads as foreign. The fix respects a real re-stamp without laundering
// history.
//
// unattributed lists present dispatched-* labels with no standing `labeled` event — an
// incomplete timeline read. It is returned SEPARATELY rather than folded into the applier
// map with an empty login, because "nobody applied this" and "an actor the timeline does not
// name applied this" have different remedies and the refusal has to say which one it found.
func resolveStampAppliers(tl StampTimeline) (appliers map[string]string, unattributed []string) {
	standing := map[string]string{}
	for _, e := range tl.Events {
		n := normLabel(e.Name)
		if !isStampLabelName(n) {
			continue
		}
		if e.Removed {
			delete(standing, n)
			continue
		}
		standing[n] = strings.TrimSpace(e.AppliedBy)
	}
	appliers = map[string]string{}
	for _, n := range presentStampLabels(tl.Present) {
		who, ok := standing[n]
		if !ok {
			unattributed = append(unattributed, n)
			continue
		}
		appliers[n] = who
	}
	return appliers, unattributed
}

// UnattributedStampLabels names every dispatched-* label the PR CARRIES that the timeline
// read cannot attribute to any applier — a present stamp with no standing `labeled` event,
// i.e. a timeline this reader did not see all of.
//
// IT EXISTS FOR THE REFUSAL MESSAGE, for the same reason NonDispatcherStampAppliers does:
// "unreadable" without the cause is a verdict the operator cannot act on, and the remedy
// here (re-read the timeline / re-stamp the PR) is not the remedy for a foreign applier.
func UnattributedStampLabels(tl StampTimeline) []string {
	_, unattributed := resolveStampAppliers(tl)
	return unattributed
}

// AttestedModelStampOf is the applier-aware reader — the STRONG form a consumer must use
// when it needs the self-report defence. It reads the same content as ModelStampOf but
// first requires that EVERY dispatched-* label was applied by a dispatcher identity: if
// any was applied by a non-dispatcher (or by an actor isDispatcher cannot vouch for),
// the whole stamp is Indeterminate, because a stamp anyone can self-apply is not
// attestation.
//
// isDispatcher answers "is this login the dispatcher?" — inject IsDispatcherLogin for the
// roster's desk App, or a test stub. A nil predicate cannot vouch for anyone, so a PR that
// carries any dispatched-* label with a nil predicate is Indeterminate (could-not-check),
// never Stamped: the strong reader fails safe rather than trusting an applier it cannot
// check. A PR with NO dispatched-* labels is Unknown regardless of the predicate.
func AttestedModelStampOf(tl StampTimeline, isDispatcher func(applier string) bool) (ModelStamp, ModelState) {
	if len(presentStampLabels(tl.Present)) == 0 {
		// No dispatched-* label ON THE PR: the applier question does not arise, and the
		// content reader will return Unknown. Historical stamp events that were later
		// removed are deliberately NOT consulted here — a stamp the PR no longer carries is
		// not an attestation it is making.
		return ModelStampOf(tl.Present)
	}
	appliers, unattributed := resolveStampAppliers(tl)
	if len(unattributed) > 0 {
		// A standing stamp the timeline read cannot attribute. Could-not-check, never
		// unstamped: the NOTICE path is the one direction this floor must not fail.
		return ModelStamp{}, ModelIndeterminate
	}
	for _, who := range appliers {
		if isDispatcher == nil || !isDispatcher(who) {
			// The STANDING application of a dispatched-* label came from someone the
			// predicate will not vouch for. Self-report is worthless by design.
			return ModelStamp{}, ModelIndeterminate
		}
	}
	return ModelStampOf(tl.Present)
}

// DispatcherRole is the desk ROLE whose App identity is the dispatcher — the ONE declared
// source of that answer, for the reader AND the writer.
//
// WHY IT IS A CONSTANT AND NOT A STRING IN TWO PLACES. The reader (IsDispatcherLogin) and
// the WRITER (the dispatch verb's stamp step) have to name the same identity or the whole
// mechanism inverts: a stamp applied under one identity and verified against another reads
// as a forged self-report, which the floor refuses — so a correctly dispatched, genuinely
// strong-tier PR is refused while an UNSTAMPED one proceeds on the NOTICE path. That is
// exactly the failure this constant exists to make impossible: the writer projects the
// role it must authenticate as from here, the reader projects the login it accepts from
// here, and a change moves both at once.
const DispatcherRole = "desk"

// IsDispatcherLogin reports whether a GitHub login is the fleet's DISPATCHER — the
// DispatcherRole App identity, the only actor whose dispatched-* stamp counts as
// attestation. It is the roster-derived answer (RoleAppLogin(DispatcherRole)): false for
// an unbound role, an unconfigured roster, or an empty login, so an unconfigured
// deployment vouches for nobody rather than defaulting to trust. Inject it as
// AttestedModelStampOf's predicate wherever the strong form is needed against the live
// roster.
func IsDispatcherLogin(login string) bool {
	want := strings.TrimSpace(login)
	if want == "" {
		return false
	}
	deskLogin, ok := RoleAppLogin(DispatcherRole)
	if !ok {
		return false
	}
	return strings.EqualFold(want, deskLogin)
}

// NonDispatcherStampAppliers names every identity holding the STANDING application of a
// dispatched-* label the predicate will not vouch for — sorted and de-duplicated, and EMPTY
// when every label the PR currently carries came from the dispatcher.
//
// STANDING, not historical. A login whose application was later removed and re-applied by
// the dispatcher is NOT listed: naming it would report a repaired PR as still foreign-
// stamped, which is the append-only trap this reader was corrected for.
//
// IT EXISTS FOR THE REFUSAL MESSAGE. "Applied by a non-dispatcher identity" is a verdict
// the operator cannot act on: the two remedies (re-stamp the PR under the dispatcher vs
// escalate the write to a strong-tier session) are different, and choosing between them
// needs the login that actually applied the stamp. Reconstructing it by hand from the
// timeline API is what the field report had to do.
//
// A nil predicate vouches for NOBODY, so every stamp applier is listed — the same
// fail-closed direction AttestedModelStampOf takes, so the message never claims a clean
// applier set it did not actually check.
func NonDispatcherStampAppliers(tl StampTimeline, isDispatcher func(applier string) bool) []string {
	appliers, _ := resolveStampAppliers(tl)
	seen := map[string]bool{}
	var out []string
	for _, who := range appliers {
		if isDispatcher != nil && isDispatcher(who) {
			continue
		}
		if who == "" {
			who = "(an actor the timeline does not name)"
		}
		if seen[who] {
			continue
		}
		seen[who] = true
		out = append(out, who)
	}
	sort.Strings(out)
	return out
}

package deskkit

import (
	"strings"
	"testing"
)

// modelstampReviewerRoster binds BOTH dispatching roles — the desk App and the reviewer App —
// plus a worker App that is neither. It is the fixture for the widened trusted-applier set:
// the reviewer App's stamp must be accepted, the worker App's must still be refused, so the
// widening is provably a widening of exactly one identity and not of "any App".
const modelstampReviewerRoster = `ASSAY_BLESS_LOGIN=ada:2001
ASSAY_TRUSTED_LOGINS=ada:2001
ASSAY_TRUSTED_BOT_SLUGS=desk=example-desk-app:300000001,reviewer=example-reviewer-app:300000002,worker=example-worker-app:300000006
ASSAY_ALLOWED_REPOS=example-org/one:ci:private
`

// --- Ruling 2: a stamp whose dispatch claim is released AGES OUT ---------------------------

// The defect in one test: a dead stamp was treated WORSE than no stamp. A foreign (or
// otherwise unreadable) stamp refuses every authority-bearing write, while the SAME PR with
// no stamp at all proceeds on the NOTICE path — so a PR the dispatching cycle abandoned was
// bricked with no repair anyone could perform. Once the claim is released the cycle is over
// and the stamp attests nothing about this write, so the PR must read UNSTAMPED.
func TestForeignStampAgesOutWhenClaimReleased(t *testing.T) {
	// Hermetic: the refusal/NOTICE wording renders the roster's dispatcher identities, so a test
	// that did not plant a roster would print the RUNNING deployment's App logins into its output.
	plantRoster(t, modelstampReviewerRoster)
	const disp = "the-dispatcher"
	// Standing stamp applied by someone the predicate will not vouch for: Indeterminate,
	// which is the branch that refuses.
	tl := tlOf(modelEvent("example-model-1", "a-stranger"), tierEvent("strong", "a-stranger"))

	if d := ModelCapabilityFloor(tl, dispatcherIs(disp), false, ClaimHeld); d.Outcome != FloorRefuse {
		t.Fatalf("claim HELD: outcome = %v, want FloorRefuse — a live cycle's untrusted stamp still refuses\n%s",
			d.Outcome, d.Message)
	}
	if d := ModelCapabilityFloor(tl, dispatcherIs(disp), false, ClaimLivenessUnknown); d.Outcome != FloorRefuse {
		t.Fatalf("claim UNKNOWN: outcome = %v, want FloorRefuse — could-not-check must never age a stamp out\n%s",
			d.Outcome, d.Message)
	}

	d := ModelCapabilityFloor(tl, dispatcherIs(disp), false, ClaimReleased)
	if d.Outcome != FloorNoticeAllow {
		t.Fatalf("claim RELEASED: outcome = %v, want FloorNoticeAllow — a dead cycle's stamp ages out\n%s",
			d.Outcome, d.Message)
	}
	if d.State != ModelUnknown {
		t.Errorf("State = %v, want ModelUnknown — an aged-out stamp means the PR READS unstamped, and a "+
			"consumer reporting the state must see that, not the state the dead labels would have had", d.State)
	}
	if d.Stamp != (ModelStamp{}) {
		t.Errorf("Stamp = %+v, want the zero stamp — an aged-out stamp is not an attestation to report", d.Stamp)
	}
	// The message must say WHY, and name the labels: "no attestation" on a PR that visibly
	// carries dispatched-* labels is the confusing line the `any` NOTICE already had to fix.
	if !strings.Contains(d.Message, "NOTICE") || !strings.Contains(d.Message, "RELEASED") {
		t.Errorf("the age-out message does not report itself as a NOTICE naming the released claim:\n%s", d.Message)
	}
	if !strings.Contains(d.Message, DispatchedTierPrefix+"strong") {
		t.Errorf("the age-out message does not name the stamp it aged out:\n%s", d.Message)
	}
	if strings.Contains(d.Message, "UNREADABLE") {
		t.Errorf("an aged-out stamp was reported with the UNREADABLE refusal wording — it is not a refusal:\n%s",
			d.Message)
	}
}

// A READABLE, dispatcher-applied, at-floor stamp ages out too. The rule is about the CYCLE,
// not about whether the labels parse: once the claim is released the stamp attests for a
// session that is gone, so it stops clearing the floor and the PR reads unstamped. Today both
// branches proceed, so this costs nothing; it is asserted because it stops being free the
// moment an unstamped branch is made to refuse for some class of PR.
func TestStrongStampAlsoAgesOut(t *testing.T) {
	// Hermetic: the refusal/NOTICE wording renders the roster's dispatcher identities, so a test
	// that did not plant a roster would print the RUNNING deployment's App logins into its output.
	plantRoster(t, modelstampReviewerRoster)
	const disp = "the-dispatcher"
	tl := tlOf(modelEvent("example-model-2", disp), tierEvent("strong", disp))

	if d := ModelCapabilityFloor(tl, dispatcherIs(disp), false, ClaimHeld); d.Outcome != FloorAllow {
		t.Fatalf("claim HELD: outcome = %v, want FloorAllow", d.Outcome)
	}
	d := ModelCapabilityFloor(tl, dispatcherIs(disp), false, ClaimReleased)
	if d.Outcome != FloorNoticeAllow || d.State != ModelUnknown {
		t.Fatalf("claim RELEASED: outcome = %v state = %v, want FloorNoticeAllow / ModelUnknown\n%s",
			d.Outcome, d.State, d.Message)
	}
}

// The age-out must not fire on a PR that carries NO stamp: there is nothing to age out, and
// reporting the age-out wording would tell an operator a stamp was ignored that never existed.
// Such a PR takes the ordinary unstamped NOTICE.
func TestAgeOutNeedsAStampToBePresent(t *testing.T) {
	// Hermetic: the refusal/NOTICE wording renders the roster's dispatcher identities, so a test
	// that did not plant a roster would print the RUNNING deployment's App logins into its output.
	plantRoster(t, modelstampReviewerRoster)
	d := ModelCapabilityFloor(StampTimeline{}, dispatcherIs("the-dispatcher"), false, ClaimReleased)
	if d.Outcome != FloorNoticeAllow {
		t.Fatalf("outcome = %v, want FloorNoticeAllow", d.Outcome)
	}
	if strings.Contains(d.Message, "RELEASED") {
		t.Errorf("an unstamped PR was reported as having AGED OUT a stamp it never carried:\n%s", d.Message)
	}
}

// The override outranks the age-out, as it outranks every other branch: an operator running
// incident recovery gets the loud override line, not a NOTICE that quietly looks like a pass.
func TestOverrideOutranksAgeOut(t *testing.T) {
	// Hermetic: the refusal/NOTICE wording renders the roster's dispatcher identities, so a test
	// that did not plant a roster would print the RUNNING deployment's App logins into its output.
	plantRoster(t, modelstampReviewerRoster)
	tl := tlOf(modelEvent("example-model-1", "a-stranger"), tierEvent("strong", "a-stranger"))
	d := ModelCapabilityFloor(tl, dispatcherIs("the-dispatcher"), true, ClaimReleased)
	if d.Outcome != FloorOverrideAllow || !strings.Contains(d.Message, ModelFloorOverrideMarker) {
		t.Fatalf("outcome = %v, want FloorOverrideAllow carrying %s\n%s",
			d.Outcome, ModelFloorOverrideMarker, d.Message)
	}
}

// ClaimLivenessFromRefPresence is the ONE reduction every verb's own presence read passes
// through. The direction that matters is the error one: a read that FAILED is could-not-check,
// never a release — treating it as a release is how a LIVE dispatch would have its attestation
// thrown away by a flaky API call.
func TestClaimLivenessFromRefPresence(t *testing.T) {
	if got := ClaimLivenessFromRefPresence(true, nil); got != ClaimHeld {
		t.Errorf("present ref = %v, want held", got)
	}
	if got := ClaimLivenessFromRefPresence(false, nil); got != ClaimReleased {
		t.Errorf("absent ref = %v, want released", got)
	}
	for _, present := range []bool{true, false} {
		if got := ClaimLivenessFromRefPresence(present, errFixture{}); got != ClaimLivenessUnknown {
			t.Errorf("failed read (present=%v) = %v, want unknown — a read that failed is not a release",
				present, got)
		}
	}
	if ClaimLivenessUnknown != 0 {
		t.Error("ClaimLivenessUnknown is not the zero value — a consumer that forgets to resolve liveness " +
			"must leave the stamp standing, not age it out")
	}
	if ClaimLivenessUnknown.AgesOutStamp() || ClaimHeld.AgesOutStamp() || !ClaimReleased.AgesOutStamp() {
		t.Error("AgesOutStamp does not answer for exactly ClaimReleased")
	}
}

type errFixture struct{}

func (errFixture) Error() string { return "the presence read failed" }

// The claim key the floor looks up MUST be the key the dispatcher took the claim under, or
// the reader looks in a place the writer never wrote and every PR reads released. These rows
// are the shapes deskdispatch's own claimKeyFor produces, in reverse.
func TestClaimKeyForPR(t *testing.T) {
	plantRoster(t, modelstampFixtureRoster)
	const repo = "example-org/one"

	cases := []struct {
		name string
		body string
		want string // "" means: no derivable key
	}{
		{"brief trailer", "does a thing\n\nBrief: example-stream/15\n", "one--example-stream--15"},
		{"issue trailer", "fixes it\n\nIssue: #486\n", "one--issue-486"},
		// ParseTrailers REFUSES a body carrying both link kinds, so such a body names no
		// single dispatch and yields no key. That is the right answer here, not a defect to
		// route around: a PR whose trailer set is malformed is one whose claim cannot be
		// identified, and Unknown leaves its stamp standing.
		{"both trailers is a malformed set", "both\n\nBrief: st/07\nIssue: #12\n", ""},
		{"no trailer at all", "a human opened this PR by hand\n", ""},
		{"unsplittable brief value", "x\n\nBrief: not-a-brief-ref\n", ""},
		{"empty body", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := ClaimKeyForPR(repo, c.body)
			if c.want == "" {
				if ok {
					t.Fatalf("ClaimKeyForPR = %q, ok — want NO derivable key; inventing one looks up a claim "+
						"nobody ever took, and its absence would read as a release", got)
				}
				return
			}
			if !ok || got != c.want {
				t.Fatalf("ClaimKeyForPR = %q (ok=%v), want %q", got, ok, c.want)
			}
			// The key has to be a legal claim ref path, or the lookup can never happen.
			if _, err := ClaimRefPath(got); err != nil {
				t.Fatalf("the derived key %q does not build a claim ref path: %v", got, err)
			}
		})
	}
}

// --- Ruling 4: the reviewer App is a dispatcher for the lane it dispatches ------------------

// pr-review-desk dispatches its reviewers itself rather than through the desk App, so before
// this a correctly-run review could never carry a floor-trusted stamp: the lane applied none,
// and the floor would not have accepted one. Admitting the reviewer App restores
// dispatcher==applier for that lane. What must NOT change is anything else — a worker App's
// self-applied stamp is still worthless, which is the property the whole mechanism rests on.
func TestReviewerAppStampIsTrustedAndWorkerAppIsNot(t *testing.T) {
	plantRoster(t, modelstampReviewerRoster)

	const reviewer = "example-reviewer-app[bot]"
	const desk = "example-desk-app[bot]"
	const worker = "example-worker-app[bot]"

	if !IsDispatcherLogin(desk) {
		t.Fatal("the desk App is no longer accepted as a dispatcher — the widening must ADD an identity, " +
			"never move the one that was already there")
	}
	if !IsDispatcherLogin(reviewer) {
		t.Fatal("the reviewer App is not accepted as a dispatcher — the review lane still cannot carry a " +
			"trustable stamp")
	}
	if IsDispatcherLogin(worker) {
		t.Fatal("the WORKER App is accepted as a dispatcher — the set was widened to 'any App', which " +
			"restores exactly the self-report the stamp exists to defeat")
	}
	if IsDispatcherLogin("") || IsDispatcherLogin("ada") {
		t.Fatal("an empty or unbound login is accepted as a dispatcher")
	}

	// End to end through the floor: a reviewer-applied strong stamp clears it.
	rev := tlOf(modelEvent("example-model-2", reviewer), tierEvent("strong", reviewer))
	if d := ModelCapabilityFloor(rev, IsDispatcherLogin, false, ClaimLivenessUnknown); d.Outcome != FloorAllow {
		t.Fatalf("a reviewer-App strong stamp: outcome = %v, want FloorAllow\n%s", d.Outcome, d.Message)
	}
	// And a worker-applied one still refuses, naming BOTH accepted identities so the operator
	// can see which dispatcher to re-stamp from.
	wrk := tlOf(modelEvent("example-model-2", worker), tierEvent("strong", worker))
	d := ModelCapabilityFloor(wrk, IsDispatcherLogin, false, ClaimLivenessUnknown)
	if d.Outcome != FloorRefuse {
		t.Fatalf("a worker-App stamp: outcome = %v, want FloorRefuse\n%s", d.Outcome, d.Message)
	}
	for _, want := range []string{worker, desk, reviewer} {
		if !strings.Contains(d.Message, want) {
			t.Errorf("the refusal does not name %s — it must name the applier it found and every identity "+
				"it would have accepted:\n%s", want, d.Message)
		}
	}
}

// DispatcherRoles is the ONE list reader, writer and refusal message project from. Pinning it
// keeps a role added in one place from being missed in the other two.
func TestDispatcherRolesIsTheOneList(t *testing.T) {
	got := DispatcherRoles()
	want := []string{"desk", "reviewer"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("DispatcherRoles() = %v, want %v", got, want)
	}
	if got[0] != DispatcherRole || got[1] != ReviewDispatcherRole {
		t.Fatal("DispatcherRoles does not project from the role constants — a second spelling has appeared")
	}
}

// A roster that binds NEITHER dispatching role vouches for nobody, and the refusal has to SAY
// that rather than name an identity this deployment does not carry.
func TestDispatcherLoginsForMessageOnUnboundRoster(t *testing.T) {
	plantRoster(t, `ASSAY_BLESS_LOGIN=ada:2001
ASSAY_TRUSTED_LOGINS=ada:2001
ASSAY_TRUSTED_BOT_SLUGS=worker=example-worker-app:300000006
ASSAY_ALLOWED_REPOS=example-org/one:ci:private
`)
	msg := DispatcherLoginsForMessage()
	if !strings.Contains(msg, "vouches for nobody") {
		t.Fatalf("an unbound roster does not report that it vouches for nobody: %s", msg)
	}
	if IsDispatcherLogin("example-worker-app[bot]") {
		t.Fatal("an unconfigured dispatcher roster defaulted to trusting an App")
	}
}

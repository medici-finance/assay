package deskkit

import (
	"strings"
	"testing"
)

// modelstampFixtureRoster binds a desk role so IsDispatcherLogin has a dispatcher identity
// to vouch for. The desk App slug here is the DISPATCHER — the only applier whose stamp is
// attestation.
const modelstampFixtureRoster = `ASSAY_BLESS_LOGIN=ada:2001
ASSAY_TRUSTED_LOGINS=ada:2001
ASSAY_TRUSTED_BOT_SLUGS=desk=example-desk-app:300000001,worker=example-worker-app:300000006
ASSAY_ALLOWED_REPOS=example-org/one:ci:private
`

// TestModelStampTierVocabularyIsExecTierSet is the derive-or-diff PIN: the tier vocabulary
// IS the brief-schema exec-tier value set {any, strong}. statusgen owns the canonical
// `validExecTier` in a separate module; this asserts the mirror has not drifted from it. A
// third tier value added to one side and not the other trips this at build time rather
// than reporting confidently against a vocabulary nothing emits.
func TestModelStampTierVocabularyIsExecTierSet(t *testing.T) {
	got := DispatchTiers()
	want := []string{"any", "strong"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("DispatchTiers() = %v, want %v — the tier vocabulary is the brief-schema exec-tier "+
			"value set (statusgen validExecTier); a divergence here is the second-list drift the "+
			"mirror is pinned to prevent", got, want)
	}
}

// TestModelStampTierVocabularyReturnsAFreshCopy — a caller must not be able to mutate the
// package's backing vocabulary through the returned slice.
func TestModelStampTierVocabularyReturnsAFreshCopy(t *testing.T) {
	a := DispatchTiers()
	if len(a) == 0 {
		t.Fatal("DispatchTiers() empty")
	}
	a[0] = "MUTATED"
	if DispatchTiers()[0] == "MUTATED" {
		t.Fatal("DispatchTiers() handed out its backing array — a caller mutated the vocabulary")
	}
}

// TestModelStampZeroValueIsUnknown pins the fail-safe default. A consumer that declares a
// ModelState and forgets to set it must hold the NON-ANSWER, and it must never be read as a
// default model.
func TestModelStampZeroValueIsUnknown(t *testing.T) {
	var zero ModelState
	if zero != ModelUnknown {
		t.Fatalf("the zero ModelState is %v, want ModelUnknown — the default must be the non-answer, "+
			"or a consumer that forgets the state gets a confident wrong model", zero)
	}
	if zero.Answered() {
		t.Fatal("the zero state reports Answered() — unknown is not an answer")
	}
	if ModelIndeterminate.Answered() {
		t.Fatal("indeterminate reports Answered() — a conflicting/self-applied stamp is a could-not-check")
	}
	if !ModelStamped.Answered() {
		t.Fatal("stamped does not report Answered()")
	}
}

// TestModelStampPrefixesPinned — the label prefixes are the wire format the per-model /
// per-tier metric groups on. Changing either silently orphans
// every stamp already applied.
func TestModelStampPrefixesPinned(t *testing.T) {
	if DispatchedModelPrefix != "dispatched-model:" {
		t.Fatalf("DispatchedModelPrefix = %q, want \"dispatched-model:\"", DispatchedModelPrefix)
	}
	if DispatchedTierPrefix != "dispatched-tier:" {
		t.Fatalf("DispatchedTierPrefix = %q, want \"dispatched-tier:\"", DispatchedTierPrefix)
	}
}

// TestModelStampLabelWriters — the ONE admissible construction path for each half, with
// case-folding, slug-shape validation, and tier-vocabulary validation.
func TestModelStampLabelWriters(t *testing.T) {
	plantRoster(t, modelstampFixtureRoster)

	t.Run("model happy path and case fold", func(t *testing.T) {
		for _, in := range []string{"opus-4.8", "OPUS-4.8", "  Opus-4.8  "} {
			got, err := DispatchedModelLabel(in)
			if err != nil {
				t.Fatalf("DispatchedModelLabel(%q): %v", in, err)
			}
			if got != "dispatched-model:opus-4.8" {
				t.Errorf("DispatchedModelLabel(%q) = %q, want dispatched-model:opus-4.8", in, got)
			}
		}
	})

	t.Run("model empty refuses", func(t *testing.T) {
		for _, in := range []string{"", "   "} {
			if _, err := DispatchedModelLabel(in); err == nil {
				t.Errorf("DispatchedModelLabel(%q) succeeded — a blank slug would stamp dispatched-model:", in)
			} else if ExitCodeOf(err) != ExitRefused {
				t.Errorf("DispatchedModelLabel(%q) exit = %d, want refused", in, ExitCodeOf(err))
			}
		}
	})

	t.Run("model with a bad character refuses", func(t *testing.T) {
		// A space, a slash, or anything that is not lowercase/dash/dot/digit — the shape
		// that keeps a personal handle out of a published label.
		for _, in := range []string{"opus 4.8", "opus/4.8", "Claude_Opus", "модель"} {
			if _, err := DispatchedModelLabel(in); err == nil {
				t.Errorf("DispatchedModelLabel(%q) succeeded — a slug outside [a-z0-9.-] reached a label", in)
			}
		}
	})

	t.Run("model of only separators refuses", func(t *testing.T) {
		if _, err := DispatchedModelLabel("--.-"); err == nil {
			t.Error("DispatchedModelLabel(\"--.-\") succeeded — a run of separators is not a slug")
		}
	})

	t.Run("tier happy path and case fold", func(t *testing.T) {
		for in, want := range map[string]string{
			"strong": "dispatched-tier:strong", "STRONG": "dispatched-tier:strong",
			"any": "dispatched-tier:any", "  Any  ": "dispatched-tier:any",
		} {
			got, err := DispatchedTierLabel(in)
			if err != nil {
				t.Fatalf("DispatchedTierLabel(%q): %v", in, err)
			}
			if got != want {
				t.Errorf("DispatchedTierLabel(%q) = %q, want %q", in, got, want)
			}
		}
	})

	t.Run("tier outside the exec-tier set refuses and enumerates", func(t *testing.T) {
		_, err := DispatchedTierLabel("turbo")
		if err == nil {
			t.Fatal("DispatchedTierLabel(turbo) succeeded — a tier outside the exec-tier set reached a label")
		}
		if ExitCodeOf(err) != ExitRefused {
			t.Errorf("exit = %d, want refused", ExitCodeOf(err))
		}
		for _, want := range []string{"any", "strong"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("refusal does not enumerate the exec-tier value %q: %s", want, err)
			}
		}
	})

	t.Run("both halves at once", func(t *testing.T) {
		got, err := ModelStampLabels("kimi-3", "any")
		if err != nil {
			t.Fatalf("ModelStampLabels: %v", err)
		}
		want := []string{"dispatched-model:kimi-3", "dispatched-tier:any"}
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("ModelStampLabels = %v, want %v", got, want)
		}
		// A bad half refuses the whole pair — no half-stamp from a bad input.
		if _, err := ModelStampLabels("kimi-3", "turbo"); err == nil {
			t.Fatal("ModelStampLabels applied a pair with an invalid tier")
		}
		if _, err := ModelStampLabels("", "any"); err == nil {
			t.Fatal("ModelStampLabels applied a pair with an empty slug")
		}
	})
}

// TestModelStampLabelCarriesNoPersonalIdentifier — the same naming trap raisedby guards:
// a label is published on every PR it touches, so the slug shape must be lowercase/dash/
// dot/digit and nothing that could be a human handle or given name.
func TestModelStampLabelCarriesNoPersonalIdentifier(t *testing.T) {
	plantRoster(t, modelstampFixtureRoster)
	for _, slug := range []string{"opus-4.8", "kimi-3", "glm5.2", "fable-5"} {
		label, err := DispatchedModelLabel(slug)
		if err != nil {
			t.Fatalf("DispatchedModelLabel(%q): %v", slug, err)
		}
		for _, r := range strings.TrimPrefix(label, DispatchedModelPrefix) {
			if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' && r != '.' {
				t.Errorf("label %q carries %q — model slugs are lowercase/dash/dot/digit only, so a "+
					"human handle cannot reach a published label", label, string(r))
			}
		}
	}
}

// TestModelStampOfThreeStates is the READER contract the family-F metric computes against —
// the label-CONTENT reader (applier trust is a separate axis, tested below). Subtest names
// carry the literal state words so `go test -run TestModelStamp -v` output names the
// ModelUnknown and ModelIndeterminate cases.
func TestModelStampOfThreeStates(t *testing.T) {
	cases := []struct {
		name      string
		labels    []string
		wantModel string
		wantTier  string
		wantState ModelState
	}{
		// --- ModelUnknown: no stamp, the zero value, never a default ---
		{"ModelUnknown on no labels at all", nil, "", "", ModelUnknown},
		{"ModelUnknown on labels but no stamp", []string{"bug", "urgent"}, "", "", ModelUnknown},
		{"ModelUnknown when a prefix is not at the start", []string{"not-dispatched-model:opus-4.8"}, "", "", ModelUnknown},

		// --- ModelStamped: complete stamp ---
		{"stamped happy path", []string{"bug", "dispatched-model:opus-4.8", "dispatched-tier:strong"},
			"opus-4.8", "strong", ModelStamped},
		{"stamped with odd case and spacing",
			[]string{" Dispatched-Model:Opus-4.8 ", " DISPATCHED-TIER:STRONG "}, "opus-4.8", "strong", ModelStamped},
		{"stamped with duplicate identical halves (not a conflict)",
			[]string{"dispatched-model:kimi-3", "dispatched-model:kimi-3", "dispatched-tier:any", "dispatched-tier:any"},
			"kimi-3", "any", ModelStamped},
		{"stamped with tier any", []string{"dispatched-model:glm5.2", "dispatched-tier:any"}, "glm5.2", "any", ModelStamped},

		// --- ModelIndeterminate: present but unreadable ---
		{"ModelIndeterminate on two different model labels",
			[]string{"dispatched-model:opus-4.8", "dispatched-model:kimi-3", "dispatched-tier:strong"}, "", "", ModelIndeterminate},
		{"ModelIndeterminate on two different tier labels",
			[]string{"dispatched-model:opus-4.8", "dispatched-tier:strong", "dispatched-tier:any"}, "", "", ModelIndeterminate},
		{"ModelIndeterminate on an empty slug",
			[]string{"dispatched-model:", "dispatched-tier:strong"}, "", "", ModelIndeterminate},
		{"ModelIndeterminate on an empty tier",
			[]string{"dispatched-model:opus-4.8", "dispatched-tier:"}, "", "", ModelIndeterminate},
		{"ModelIndeterminate on a tier outside the exec-tier set",
			[]string{"dispatched-model:opus-4.8", "dispatched-tier:turbo"}, "", "", ModelIndeterminate},
		{"ModelIndeterminate on an incomplete stamp — model only",
			[]string{"dispatched-model:opus-4.8"}, "", "", ModelIndeterminate},
		{"ModelIndeterminate on an incomplete stamp — tier only",
			[]string{"dispatched-tier:strong"}, "", "", ModelIndeterminate},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stamp, state := ModelStampOf(tc.labels)
			if state != tc.wantState || stamp.Model != tc.wantModel || stamp.Tier != tc.wantTier {
				t.Fatalf("ModelStampOf(%v) = (%+v, %v), want ({Model:%q Tier:%q}, %v)",
					tc.labels, stamp, state, tc.wantModel, tc.wantTier, tc.wantState)
			}
			if state != ModelStamped && (stamp.Model != "" || stamp.Tier != "") {
				t.Fatalf("a non-answer returned (%q,%q) — a consumer would read it as attribution",
					stamp.Model, stamp.Tier)
			}
		})
	}
}

// TestModelStampOfNeverSubstitutesADefault is the invariant stated as a test: NOTHING this
// package returns names a model for an unstamped PR. The only positive claim it can make is
// a slug that was literally on the PR.
func TestModelStampOfNeverSubstitutesADefault(t *testing.T) {
	for _, labels := range [][]string{nil, {}, {"bug"}, {"question", "needs-decision"}} {
		stamp, state := ModelStampOf(labels)
		if state != ModelUnknown {
			t.Fatalf("ModelStampOf(%v) = %v, want ModelUnknown", labels, state)
		}
		if stamp.Model != "" || stamp.Tier != "" {
			t.Fatalf("ModelStampOf(%v) attributed (%q,%q) to an unstamped PR", labels, stamp.Model, stamp.Tier)
		}
	}
}

// TestAttestedModelStampOf is the STRONG reader: a self-applied (non-dispatcher) stamp is
// Indeterminate by design — that is the difference between this attestation and the
// exec-tier honor-system it repairs.
func TestAttestedModelStampOf(t *testing.T) {
	const dispatcher = "example-desk-app[bot]"
	const worker = "example-worker-app[bot]"
	isDispatcher := func(applier string) bool { return applier == dispatcher }

	stampedEvents := []LabelEvent{
		{Name: "dispatched-model:opus-4.8", AppliedBy: dispatcher},
		{Name: "dispatched-tier:strong", AppliedBy: dispatcher},
	}

	t.Run("dispatcher-applied stamp is Stamped", func(t *testing.T) {
		stamp, state := AttestedModelStampOf(tlOf(stampedEvents...), isDispatcher)
		if state != ModelStamped || stamp.Model != "opus-4.8" || stamp.Tier != "strong" {
			t.Fatalf("got (%+v, %v), want opus-4.8/strong stamped", stamp, state)
		}
	})

	t.Run("worker-applied model label is Indeterminate (self-report is worthless)", func(t *testing.T) {
		events := []LabelEvent{
			{Name: "dispatched-model:opus-4.8", AppliedBy: worker}, // the worker stamping ITSELF
			{Name: "dispatched-tier:strong", AppliedBy: dispatcher},
		}
		_, state := AttestedModelStampOf(tlOf(events...), isDispatcher)
		if state != ModelIndeterminate {
			t.Fatalf("state = %v, want ModelIndeterminate — a self-applied dispatched-model label must "+
				"not read as attestation", state)
		}
	})

	t.Run("any non-dispatcher applier taints the whole stamp", func(t *testing.T) {
		events := []LabelEvent{
			{Name: "dispatched-model:opus-4.8", AppliedBy: dispatcher},
			{Name: "dispatched-tier:strong", AppliedBy: "some-random-user"},
		}
		if _, state := AttestedModelStampOf(tlOf(events...), isDispatcher); state != ModelIndeterminate {
			t.Fatalf("state = %v, want ModelIndeterminate for a non-dispatcher-applied tier", state)
		}
	})

	t.Run("nil predicate cannot vouch — Indeterminate when a stamp is present", func(t *testing.T) {
		if _, state := AttestedModelStampOf(tlOf(stampedEvents...), nil); state != ModelIndeterminate {
			t.Fatalf("state = %v, want ModelIndeterminate — a nil predicate vouches for nobody", state)
		}
	})

	t.Run("no stamp is Unknown regardless of predicate", func(t *testing.T) {
		events := []LabelEvent{{Name: "bug", AppliedBy: worker}}
		if _, state := AttestedModelStampOf(tlOf(events...), nil); state != ModelUnknown {
			t.Fatalf("state = %v, want ModelUnknown — no dispatched-* label present", state)
		}
	})

	t.Run("dispatcher-applied but conflicting content is still Indeterminate", func(t *testing.T) {
		events := []LabelEvent{
			{Name: "dispatched-model:opus-4.8", AppliedBy: dispatcher},
			{Name: "dispatched-model:kimi-3", AppliedBy: dispatcher},
			{Name: "dispatched-tier:strong", AppliedBy: dispatcher},
		}
		if _, state := AttestedModelStampOf(tlOf(events...), isDispatcher); state != ModelIndeterminate {
			t.Fatalf("state = %v, want ModelIndeterminate — trusted applier does not fix conflicting content", state)
		}
	})
}

// TestIsDispatcherLogin — the dispatcher identity is the desk role's App login, derived
// from the roster, and it fails closed (vouches for nobody) when the roster does not bind
// a desk role.
func TestIsDispatcherLogin(t *testing.T) {
	plantRoster(t, modelstampFixtureRoster)
	if !IsDispatcherLogin("example-desk-app[bot]") {
		t.Error("IsDispatcherLogin rejected the bound desk App login")
	}
	if !IsDispatcherLogin("EXAMPLE-DESK-APP[BOT]") {
		t.Error("IsDispatcherLogin is not case-insensitive on the login")
	}
	for _, notDispatcher := range []string{"", "example-worker-app[bot]", "some-human", "some-user"} {
		if IsDispatcherLogin(notDispatcher) {
			t.Errorf("IsDispatcherLogin(%q) vouched for a non-dispatcher", notDispatcher)
		}
	}

	// Unconfigured / desk-unbound roster: vouch for nobody rather than defaulting to trust.
	plantRoster(t, `ASSAY_BLESS_LOGIN=ada:2001
ASSAY_TRUSTED_LOGINS=ada:2001
ASSAY_TRUSTED_BOT_SLUGS=worker=example-worker-app:300000006
ASSAY_ALLOWED_REPOS=example-org/one:ci:private
`)
	if IsDispatcherLogin("example-desk-app[bot]") {
		t.Error("IsDispatcherLogin vouched for a login against a roster that binds no desk role")
	}
}

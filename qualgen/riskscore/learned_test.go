package riskscore

import (
	"fmt"
	"math"
	"math/rand"
	"testing"
	"time"
)

// day0 is the fixture epoch; changes land one day apart so the temporal split is
// unambiguous.
var day0 = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// syntheticCorpus builds a deterministic labeled corpus with a genuine, LEARNABLE
// defect signal carried by the Kamei JIT features, and §9.1 heuristic features
// that are only COARSE, NOISY proxies of a SUBSET of the drivers. The dominant
// driver (change entropy, plus lines-added and dev-count) is invisible to the
// hand-weighted heuristic — which is exactly why a learned model trained on the
// repo's own traced defects should out-predict the fixed heuristic on held-out
// LATER defects. The label of each change is generated FROM the very feature
// vector the model sees, so the signal is real, not injected noise.
//
// Every defect's LabelTime is set strictly AFTER its inducing change landed
// (blame looks backward), so the corpus also exercises the temporal machinery.
func syntheticCorpus(n int, seed int64) Corpus {
	rng := rand.New(rand.NewSource(seed))
	var changes []Change
	var defects []Defect

	for i := 0; i < n; i++ {
		landed := day0.AddDate(0, 0, i)

		// --- draw the latent Kamei drivers ---
		nf := 1 + rng.Intn(6)
		perFile := make([]int, nf)
		files := make([]string, nf)
		for k := 0; k < nf; k++ {
			perFile[k] = 1 + rng.Intn(40)
			files[k] = fmt.Sprintf("pkg%d/file%d.go", rng.Intn(5), rng.Intn(50))
		}
		la := 3 + rng.Intn(120)
		ld := rng.Intn(60)
		nuc := rng.Intn(30)
		npd := rng.Intn(6)
		ndev := 1 + rng.Intn(6)
		exp := rng.Intn(40)
		class := AuthorHuman
		switch rng.Intn(3) {
		case 1:
			class = AuthorAgent
		case 2:
			class = AuthorAutomation
		}

		ch := Change{
			ID:                  fmt.Sprintf("c%04d", i),
			MergeTime:           landed,
			CommitTime:          landed,
			Files:               files,
			ChangedLinesPerFile: perFile,
			LinesAdded:          la,
			LinesDeleted:        ld,
			LinesTotal:          200 + rng.Intn(2000),
			PriorChangesToFiles: nuc,
			PriorDefectsToFiles: npd,
			RecentChurn:         rng.Intn(50),
			NDev:                ndev,
			AgeDays:             float64(rng.Intn(200)),
			AuthorClass:         class,
			AuthorExp:           exp,
		}

		// --- true generative logit over the features the model actually sees ---
		f := ExtractJIT(ch)
		agentBump := 0.0
		if f.Class == AuthorAgent {
			agentBump = 0.6
		}
		logit := 2.4*f.Entropy +
			0.020*float64(f.LA) +
			0.30*float64(f.NDEV) +
			0.045*float64(f.NUC) +
			0.20*float64(f.NPD) +
			agentBump -
			3.1
		p := 1 / (1 + math.Exp(-logit))
		induced := rng.Float64() < p

		// --- §9.1 heuristic features: coarse, NOISY proxies of a SUBSET only ---
		// They can see the traced-defect history (npd) and the churn rank (nuc),
		// but NOT the dominant entropy / lines-added / dev-count drivers.
		ch.Heuristic = HeuristicFeatures{
			HotspotPercentile:         Measured(clamp01(float64(nuc)/30 + rng.NormFloat64()*0.28)),
			TracedDefectDensity:       Measured(math.Max(0, float64(npd)+rng.NormFloat64()*1.1)),
			TopIdentityOwnershipShare: Measured(clamp01(0.4 + rng.NormFloat64()*0.3)),
			MissingCouplingPartners:   Measured(math.Max(0, float64(rng.Intn(4)))),
		}
		changes = append(changes, ch)

		if induced {
			// The defect is discovered some days AFTER the inducing change
			// landed — its label time is strictly later, as blame-derived labels
			// always are.
			delay := 3 + rng.Intn(20)
			defects = append(defects, Defect{
				ID:               fmt.Sprintf("d%04d", i),
				InducingChangeID: ch.ID,
				LabelTime:        landed.AddDate(0, 0, delay),
			})
		}
	}
	return Corpus{Changes: changes, Defects: defects}
}

// TestLearned_BeatsHeuristicOnHeldOut is the DEREFERENCE test (Verify row 3):
// with a fixture defect corpus and a TEMPORAL train/test split, the learned
// model's held-out AUC on LATER defects is strictly greater than the §9.1
// heuristic baseline's on the same held-out set. This proves the model learned
// real signal — not merely that it produces a number.
func TestLearned_BeatsHeuristicOnHeldOut(t *testing.T) {
	corpus := syntheticCorpus(600, 42)
	// Temporal split at the midpoint landing time: train on the earlier half,
	// evaluate on the strictly later half.
	splitTime := day0.AddDate(0, 0, 300)

	traceRate := 0.72 // fixture's SZZ trace coverage, carried per honest-claims §10
	cmp, err := EvaluateTemporal(corpus, splitTime, DefaultConfig(), traceRate)
	if err != nil {
		t.Fatalf("EvaluateTemporal: %v", err)
	}
	t.Logf("held-out evaluation: %s", cmp.Claim())

	if cmp.HeldOutN < 100 {
		t.Fatalf("held-out set too small to be meaningful: %d", cmp.HeldOutN)
	}
	if !(cmp.LearnedAUC > cmp.HeuristicAUC) {
		t.Errorf("learned AUC %.3f must strictly beat heuristic AUC %.3f on held-out later defects",
			cmp.LearnedAUC, cmp.HeuristicAUC)
	}
	// Sanity: the learned model must actually rank (well above chance), and the
	// heuristic must not be secretly perfect (which would make the test vacuous).
	if cmp.LearnedAUC < 0.65 {
		t.Errorf("learned AUC %.3f suspiciously low — model may not have learned", cmp.LearnedAUC)
	}
	// Honest-claims: the rendered claim must carry the held-out metric AND the
	// corpus size / trace-rate, never a bare "more accurate".
	claim := cmp.Claim()
	for _, must := range []string{"held-out AUC", "trace-rate", "corpus size"} {
		if !contains(claim, must) {
			t.Errorf("honest-claims sentence missing %q: %s", must, claim)
		}
	}
}

// TestLearned_NoFutureLeakInTrainingSet is the leakage/negative test (Verify row
// 4): for a defect labeled at time T, the training set used to score any change
// AT OR BEFORE T contains NO record derived from that defect's own future; a
// fixture that injects a future-leaked label is excluded.
func TestLearned_NoFutureLeakInTrainingSet(t *testing.T) {
	// X landed day 0, known-defective by day 5 (D1); ALSO traced by a FUTURE
	// defect D2 discovered at day 100. Z landed day 2, traced ONLY by a future
	// defect D3 at day 50. Y landed day 10 (after the cutoff).
	corpus := Corpus{
		Changes: []Change{
			{ID: "X", MergeTime: day0.AddDate(0, 0, 0)},
			{ID: "Z", MergeTime: day0.AddDate(0, 0, 2)},
			{ID: "Y", MergeTime: day0.AddDate(0, 0, 10)},
		},
		Defects: []Defect{
			{ID: "D1", InducingChangeID: "X", LabelTime: day0.AddDate(0, 0, 5)},
			{ID: "D2", InducingChangeID: "X", LabelTime: day0.AddDate(0, 0, 100)}, // future leak
			{ID: "D3", InducingChangeID: "Z", LabelTime: day0.AddDate(0, 0, 50)},  // future leak
		},
	}
	asOf := day0.AddDate(0, 0, 8) // between D1 (day5) and D3/D2 (day50/100)

	examples := buildTrainingExamples(corpus, asOf)

	byID := map[string]Example{}
	for _, e := range examples {
		byID[e.Change.ID] = e
	}

	// Y landed after the cutoff → must not be a training row at all.
	if _, ok := byID["Y"]; ok {
		t.Errorf("change Y landed after the cutoff and must be excluded from training")
	}
	// X: known-defective via D1 (day5 <= day8); its FUTURE defect D2 must NOT
	// appear among the contributing labels.
	x, ok := byID["X"]
	if !ok {
		t.Fatalf("change X (landed before cutoff) missing from training set")
	}
	if !x.Label {
		t.Errorf("X should be labeled defect-inducing via D1 (labeled day 5, before cutoff)")
	}
	if containsStr(x.ContributingDefects, "D2") {
		t.Errorf("FUTURE-LEAK: D2 (labeled day 100) must not contribute to X's label at cutoff day 8; got %v", x.ContributingDefects)
	}
	// Z: its ONLY inducing defect D3 is in the future → Z must be labeled NEGATIVE
	// now. A naive labeler using the full-history trace would leak D3 backward and
	// mark Z positive.
	z, ok := byID["Z"]
	if !ok {
		t.Fatalf("change Z (landed before cutoff) missing from training set")
	}
	if z.Label {
		t.Errorf("FUTURE-LEAK: Z's only defect D3 is labeled day 50 (> cutoff day 8); Z must be negative now, got positive %v", z.ContributingDefects)
	}
	// Global invariant: EVERY contributing defect across the whole training set
	// has LabelTime <= asOf. No future record derived from a defect's own future.
	labelTime := map[string]time.Time{}
	for _, d := range corpus.Defects {
		labelTime[d.ID] = d.LabelTime
	}
	for _, e := range examples {
		for _, id := range e.ContributingDefects {
			if labelTime[id].After(asOf) {
				t.Errorf("LEAK: example %s labeled by future defect %s (labeled %v > cutoff %v)",
					e.Change.ID, id, labelTime[id], asOf)
			}
		}
	}
}

// TestLearned_ScoreCarriesHeuristicFeatures (Verify row 5, part 1): every learned
// score is emitted WITH its §9.1 heuristic feature decomposition — the score can
// always show its reasoning.
func TestLearned_ScoreCarriesHeuristicFeatures(t *testing.T) {
	corpus := syntheticCorpus(300, 7)
	// Score a change that lands late enough that the model has a full corpus to
	// train on as of its landing time.
	target := corpus.Changes[250]

	ls := ScoreChange(corpus, target, DefaultConfig())
	if ls.State != StateMeasured {
		t.Fatalf("expected a measured learned score for a well-seasoned corpus, got %q (%s)", ls.State, ls.Reason)
	}
	if ls.Score < 0 || ls.Score > 1 {
		t.Errorf("learned score out of [0,1]: %.3f", ls.Score)
	}
	if !ls.Explained() {
		t.Fatal("learned score does not carry its heuristic decomposition")
	}
	// All four §9.1 features must be present in the carried explanation, with a
	// state set on each (not a zero-value struct).
	h := ls.Heuristic.Features
	for name, f := range map[string]Feature{
		"hotspot_percentile":           h.HotspotPercentile,
		"traced_defect_density":        h.TracedDefectDensity,
		"top_identity_ownership_share": h.TopIdentityOwnershipShare,
		"missing_coupling_partners":    h.MissingCouplingPartners,
	} {
		if f.State == "" {
			t.Errorf("carried heuristic feature %q has no state — decomposition incomplete", name)
		}
	}
	// The carried decomposition must be the target's OWN features, not some other
	// change's — the explanation must actually explain THIS score.
	if h.HotspotPercentile != target.Heuristic.HotspotPercentile {
		t.Errorf("carried hotspot feature %+v != target's %+v", h.HotspotPercentile, target.Heuristic.HotspotPercentile)
	}
}

// TestLearned_UnderCorpusFallsBackToHeuristic (Verify row 5, part 2): below the
// minimum corpus the output is heuristic-only with a could-not-learn status —
// never a fabricated learned zero.
func TestLearned_UnderCorpusFallsBackToHeuristic(t *testing.T) {
	// A tiny corpus: far fewer changes than DefaultConfig().MinCorpus.
	small := syntheticCorpus(10, 3)
	target := small.Changes[9]

	ls := ScoreChange(small, target, DefaultConfig())
	if ls.State != StateCouldNotMeasure {
		t.Fatalf("under-corpus must be could-not-learn (could-not-measure), got %q", ls.State)
	}
	if ls.Reason == "" || !contains(ls.Reason, "could-not-learn") {
		t.Errorf("under-corpus result must explain itself as could-not-learn, got %q", ls.Reason)
	}
	// CRITICAL: no fabricated learned zero. Score must be the meaningless zero
	// value guarded by a non-measured state, NOT a measured 0.0.
	if ls.State == StateMeasured {
		t.Fatal("under-corpus must never emit a measured learned score")
	}
	if ls.Score != 0 {
		t.Errorf("under-corpus learned score must carry no value, got %.3f", ls.Score)
	}
	// The heuristic fallback must still ship and be usable.
	if !ls.Explained() {
		t.Fatal("under-corpus result must still carry the heuristic fallback")
	}
	if ls.Heuristic.State != StateMeasured && ls.Heuristic.State != StateMeasuredZero {
		t.Errorf("heuristic fallback should be usable, got state %q", ls.Heuristic.State)
	}
}

// --- small test helpers ---

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func containsStr(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}

package riskscore

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// Defect is one traced defect from the M2 corpus (quality/07): an SZZ trace from
// a fix back to the change(s) it identifies as inducing, plus the TIME the defect
// became known. LabelTime is the load-bearing field for leakage avoidance — a
// defect's label may only inform predictions made AT OR AFTER LabelTime.
type Defect struct {
	ID string `json:"id"`
	// InducingChangeID is the change SZZ blamed as inducing this defect.
	InducingChangeID string `json:"inducing_change_id"`
	// LabelTime is when the defect became known (the fix's landing / the report
	// time) — NOT when the inducing change landed. Blame looks backward, so a
	// defect's label time is always later than its inducing change's landing;
	// treating the two as the same is exactly the future-leak this design guards.
	LabelTime time.Time `json:"label_time"`
}

// Corpus is the labeled training/evaluation corpus: the repo's changes and the
// defects traced to them. It is self-contained and fixture-populatable, so this
// brief is testable without a live seasoned M2 corpus (brief precondition).
type Corpus struct {
	Changes []Change `json:"changes"`
	Defects []Defect `json:"defects"`
}

// Config tunes the learned layer. MinCorpus is the three-state under-corpus
// threshold: below this many labeled training examples the model does not train
// and the score is emitted heuristic-only with a could-not-learn status.
type Config struct {
	MinCorpus    int              // minimum training examples to train (spec §3.2/§11)
	Weights      HeuristicWeights // heuristic weighting (defaults if zero)
	Epochs       int              // gradient-descent epochs
	LearningRate float64          // gradient-descent step
	L2           float64          // L2 regularization strength
}

// DefaultConfig is the transparent default tuning.
func DefaultConfig() Config {
	return Config{
		MinCorpus:    40,
		Weights:      DefaultHeuristicWeights(),
		Epochs:       400,
		LearningRate: 0.1,
		L2:           1e-3,
	}
}

func (c Config) weights() HeuristicWeights {
	if (c.Weights == HeuristicWeights{}) {
		return DefaultHeuristicWeights()
	}
	return c.Weights
}

// Example is one labeled training row. Label is the defect-inducing outcome
// KNOWN AS OF the training cutoff; ContributingDefects lists the defect ids that
// set the label — every one of them has LabelTime <= the cutoff by construction,
// which is what the leakage test asserts.
type Example struct {
	Change              Change
	Label               bool
	ContributingDefects []string
}

// buildTrainingExamples builds the labeled training set for a model that will
// make predictions AS OF time asOf. Two temporal guards, together, forbid a
// change from being scored using knowledge from its own future:
//
//  1. Only changes that landed strictly BEFORE asOf are eligible — a future
//     change cannot be a training row for a past prediction.
//  2. A change is labeled defect-inducing ONLY by defects whose LabelTime is
//     <= asOf. A defect discovered later is invisible now; its label never
//     leaks backward into a training row it would inflate.
//
// A naive labeler that marks a change inducing whenever ANY defect (regardless
// of label time) traces to it would leak the change's own future — this builder
// is exactly the temporal correction of that.
func buildTrainingExamples(c Corpus, asOf time.Time) []Example {
	// Index defects known as-of asOf by inducing change.
	byChange := map[string][]string{}
	for _, d := range c.Defects {
		if d.LabelTime.After(asOf) {
			continue // future defect: invisible as of asOf
		}
		byChange[d.InducingChangeID] = append(byChange[d.InducingChangeID], d.ID)
	}
	var out []Example
	for _, ch := range c.Changes {
		if !ch.landedAt().Before(asOf) {
			continue // change landed at/after the cutoff: not yet training data
		}
		ids := byChange[ch.ID]
		sort.Strings(ids)
		out = append(out, Example{
			Change:              ch,
			Label:               len(ids) > 0,
			ContributingDefects: ids,
		})
	}
	return out
}

// finalLabel is the ground-truth defect-inducing label using the WHOLE corpus,
// with no as-of cutoff — used only to score a held-out set after the fact, never
// to build training rows. Evaluating held-out predictions against the eventual
// truth is legitimate; training on it is not.
func finalLabel(c Corpus, changeID string) bool {
	for _, d := range c.Defects {
		if d.InducingChangeID == changeID {
			return true
		}
	}
	return false
}

// Model is a trained logistic-regression JIT defect-prediction model over the
// standardized Kamei feature vector. Standardization stats are captured from the
// TRAINING set only (no evaluation data touches them) and stored so scoring a new
// change applies the identical transform.
type Model struct {
	weights []float64
	bias    float64
	mean    []float64
	std     []float64
	names   []string
	// TrainedAsOf records the temporal cutoff the model was trained at, so a
	// consumer can prove the model never saw data from after this time.
	TrainedAsOf time.Time
	// TrainN is how many labeled examples trained the model.
	TrainN int
}

// Train fits the model on the examples known as of asOf, using a temporal
// training set (buildTrainingExamples). It returns an error when there are too
// few labeled examples to train — the caller turns that into a could-not-learn
// status rather than a fabricated model. A degenerate single-class training set
// (all defective or none) is also refused: a model with no negative or no
// positive examples has learned nothing.
func Train(c Corpus, asOf time.Time, cfg Config) (*Model, error) {
	if cfg.Epochs == 0 {
		cfg = DefaultConfig()
	}
	examples := buildTrainingExamples(c, asOf)
	if len(examples) < cfg.MinCorpus {
		return nil, fmt.Errorf("under-corpus: %d labeled examples < minimum %d", len(examples), cfg.MinCorpus)
	}
	pos := 0
	for _, e := range examples {
		if e.Label {
			pos++
		}
	}
	if pos == 0 || pos == len(examples) {
		return nil, fmt.Errorf("degenerate corpus: %d/%d positive — model cannot learn a boundary", pos, len(examples))
	}

	// Assemble the raw feature matrix.
	names := FeatureNames()
	X := make([][]float64, len(examples))
	y := make([]float64, len(examples))
	for i, e := range examples {
		X[i] = ExtractJIT(e.Change).Vector()
		if e.Label {
			y[i] = 1
		}
	}

	// Standardize columns from the training set only.
	mean, std := standardizeStats(X)
	Xs := applyStandardize(X, mean, std)

	// Gradient descent on the logistic loss with L2.
	w := make([]float64, len(names))
	b := 0.0
	n := float64(len(Xs))
	for epoch := 0; epoch < cfg.Epochs; epoch++ {
		gradW := make([]float64, len(w))
		gradB := 0.0
		for i, xi := range Xs {
			p := sigmoid(dot(w, xi) + b)
			err := p - y[i]
			for j := range w {
				gradW[j] += err * xi[j]
			}
			gradB += err
		}
		for j := range w {
			gradW[j] = gradW[j]/n + cfg.L2*w[j]
			w[j] -= cfg.LearningRate * gradW[j]
		}
		b -= cfg.LearningRate * (gradB / n)
	}

	return &Model{
		weights:     w,
		bias:        b,
		mean:        mean,
		std:         std,
		names:       names,
		TrainedAsOf: asOf,
		TrainN:      len(examples),
	}, nil
}

// predict returns the model's defect probability for a change.
func (m *Model) predict(c Change) float64 {
	x := ExtractJIT(c).Vector()
	xs := standardizeRow(x, m.mean, m.std)
	return sigmoid(dot(m.weights, xs) + m.bias)
}

// LearnedScore is the output of the learned layer for one change. Its central
// invariant (spec §9.1): the learned score NEVER stands alone — Heuristic is
// ALWAYS populated with the §9.1 decomposition, whether the model trained or
// not. Score is meaningful ONLY when State is StateMeasured; a could-not-learn
// result carries NO learned number (never a fabricated zero), only the heuristic
// fallback and a reason.
type LearnedScore struct {
	State State `json:"state"`
	// Score is the learned defect probability in [0,1], meaningful only when
	// State is StateMeasured.
	Score float64 `json:"score,omitempty"`
	// Reason is set when State is StateCouldNotMeasure (could-not-learn).
	Reason string `json:"reason,omitempty"`
	// Heuristic is the §9.1 decomposition — the explanation AND the fallback. It
	// is always present; when the model could not learn, it is what ships.
	Heuristic HeuristicScore `json:"heuristic"`
	// TrainedAsOf / CorpusSize record what the model saw, so the score can be
	// audited for temporal correctness.
	TrainedAsOf time.Time `json:"trained_as_of,omitempty"`
	CorpusSize  int       `json:"corpus_size"`
}

// Explained reports whether the learned score carries its heuristic
// decomposition. It is always true — the type cannot represent a learned score
// without one — and exists so a consumer/test can assert the invariant by name.
func (s LearnedScore) Explained() bool {
	return s.Heuristic.Features != HeuristicFeatures{} || s.Heuristic.State != ""
}

// ScoreChange is the top-level entry point: it trains a model as of the change's
// landing time on the corpus's own traced defects and returns the learned score
// TOGETHER with the §9.1 heuristic decomposition. Below the minimum corpus (or
// when the model cannot train) it returns a could-not-learn status carrying the
// heuristic score as the fallback — the score that actually ships.
//
// Training as of the scored change's own landing time is the per-prediction
// temporal guarantee: the model that scores this change never saw this change,
// nor any defect discovered after it landed.
func ScoreChange(c Corpus, target Change, cfg Config) LearnedScore {
	if cfg.Epochs == 0 {
		cfg = DefaultConfig()
	}
	heur := Heuristic(target.Heuristic, cfg.weights())

	asOf := target.landedAt()
	model, err := Train(c, asOf, cfg)
	if err != nil {
		// Under-corpus / could-not-train: heuristic-only, could-not-learn. No
		// fabricated learned zero.
		return LearnedScore{
			State:      StateCouldNotMeasure,
			Reason:     "could-not-learn: " + err.Error(),
			Heuristic:  heur,
			CorpusSize: len(buildTrainingExamples(c, asOf)),
		}
	}
	return LearnedScore{
		State:       StateMeasured,
		Score:       model.predict(target),
		Heuristic:   heur,
		TrainedAsOf: asOf,
		CorpusSize:  model.TrainN,
	}
}

// Comparison is an honest-claims (spec §10) learned-vs-heuristic result. A
// "learned beats heuristic" claim may only be made through this type, which
// FORCES the held-out metric and the corpus size / trace-rate it was measured at
// to travel with the claim — never a bare "more accurate."
type Comparison struct {
	LearnedAUC   float64 `json:"learned_auc"`
	HeuristicAUC float64 `json:"heuristic_auc"`
	// HeldOutN is the size of the held-out (later) evaluation set.
	HeldOutN int `json:"held_out_n"`
	// CorpusSize / TraceRate are the honest-claims provenance (spec §10): the
	// number is meaningless without the corpus it was measured on and the SZZ
	// trace coverage that labeled it.
	CorpusSize int     `json:"corpus_size"`
	TraceRate  float64 `json:"trace_rate"`
}

// Claim renders the honest-claims sentence: never a bare comparison, always with
// the held-out metric and the corpus size / trace-rate attached (spec §10).
func (c Comparison) Claim() string {
	verdict := "does not beat"
	if c.LearnedAUC > c.HeuristicAUC {
		verdict = "beats"
	}
	return fmt.Sprintf(
		"learned %s heuristic: held-out AUC %.3f vs %.3f over %d later changes, at corpus size %d / trace-rate %.0f%%",
		verdict, c.LearnedAUC, c.HeuristicAUC, c.HeldOutN, c.CorpusSize, c.TraceRate*100,
	)
}

// EvaluateTemporal performs the full temporal-split evaluation (Verify row 3):
// it splits the corpus at splitTime, trains the learned model on the EARLY
// portion (known as of splitTime), and scores BOTH the learned model and the
// §9.1 heuristic on the held-out LATER changes, returning their held-out AUCs
// with the honest-claims provenance attached. traceRate is the SZZ trace
// coverage of the labeling, carried through per spec §10.
func EvaluateTemporal(c Corpus, splitTime time.Time, cfg Config, traceRate float64) (Comparison, error) {
	if cfg.Epochs == 0 {
		cfg = DefaultConfig()
	}
	model, err := Train(c, splitTime, cfg)
	if err != nil {
		return Comparison{}, fmt.Errorf("train on early split: %w", err)
	}

	// Held-out = changes that landed at/after the split. Their ground-truth
	// labels use the eventual (final) trace — legitimate for evaluation only.
	var learnedPreds, heurPreds []float64
	var labels []bool
	for _, ch := range c.Changes {
		if ch.landedAt().Before(splitTime) {
			continue
		}
		learnedPreds = append(learnedPreds, model.predict(ch))
		h := Heuristic(ch.Heuristic, cfg.weights())
		heurPreds = append(heurPreds, h.Score) // 0 for could-not-measure: honest, not favorable
		labels = append(labels, finalLabel(c, ch.ID))
	}

	lauc, err := auc(learnedPreds, labels)
	if err != nil {
		return Comparison{}, fmt.Errorf("learned AUC: %w", err)
	}
	hauc, err := auc(heurPreds, labels)
	if err != nil {
		return Comparison{}, fmt.Errorf("heuristic AUC: %w", err)
	}

	return Comparison{
		LearnedAUC:   lauc,
		HeuristicAUC: hauc,
		HeldOutN:     len(labels),
		CorpusSize:   model.TrainN,
		TraceRate:    traceRate,
	}, nil
}

// --- numeric helpers ---

func sigmoid(z float64) float64 {
	if z >= 0 {
		return 1 / (1 + math.Exp(-z))
	}
	// numerically stable form for very negative z
	ez := math.Exp(z)
	return ez / (1 + ez)
}

func dot(a, b []float64) float64 {
	var s float64
	for i := range a {
		s += a[i] * b[i]
	}
	return s
}

// standardizeStats returns per-column mean and std over the matrix. A zero-
// variance column gets std 1 so standardization leaves it at its (zero-centered)
// constant without dividing by zero.
func standardizeStats(X [][]float64) (mean, std []float64) {
	if len(X) == 0 {
		return nil, nil
	}
	cols := len(X[0])
	mean = make([]float64, cols)
	std = make([]float64, cols)
	for _, row := range X {
		for j, v := range row {
			mean[j] += v
		}
	}
	n := float64(len(X))
	for j := range mean {
		mean[j] /= n
	}
	for _, row := range X {
		for j, v := range row {
			d := v - mean[j]
			std[j] += d * d
		}
	}
	for j := range std {
		std[j] = math.Sqrt(std[j] / n)
		if std[j] == 0 {
			std[j] = 1
		}
	}
	return mean, std
}

func applyStandardize(X [][]float64, mean, std []float64) [][]float64 {
	out := make([][]float64, len(X))
	for i, row := range X {
		out[i] = standardizeRow(row, mean, std)
	}
	return out
}

func standardizeRow(row, mean, std []float64) []float64 {
	out := make([]float64, len(row))
	for j, v := range row {
		out[j] = (v - mean[j]) / std[j]
	}
	return out
}

// auc computes the area under the ROC curve via the Mann-Whitney rank statistic,
// with tie handling (average ranks). It needs at least one positive and one
// negative label. AUC 0.5 is chance; higher is a better ranker.
func auc(scores []float64, labels []bool) (float64, error) {
	if len(scores) != len(labels) {
		return 0, fmt.Errorf("scores/labels length mismatch")
	}
	type pair struct {
		score float64
		label bool
	}
	pairs := make([]pair, len(scores))
	pos, neg := 0, 0
	for i := range scores {
		pairs[i] = pair{scores[i], labels[i]}
		if labels[i] {
			pos++
		} else {
			neg++
		}
	}
	if pos == 0 || neg == 0 {
		return 0, fmt.Errorf("AUC undefined: need both classes (pos=%d neg=%d)", pos, neg)
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].score < pairs[j].score })

	// Assign average ranks (1-based), breaking ties by mean rank.
	ranks := make([]float64, len(pairs))
	for i := 0; i < len(pairs); {
		j := i
		for j < len(pairs) && pairs[j].score == pairs[i].score {
			j++
		}
		avg := float64(i+1+j) / 2 // mean of ranks i+1..j
		for k := i; k < j; k++ {
			ranks[k] = avg
		}
		i = j
	}
	var sumPosRanks float64
	for i, p := range pairs {
		if p.label {
			sumPosRanks += ranks[i]
		}
	}
	// Mann-Whitney U for positives, then AUC = U / (pos*neg).
	u := sumPosRanks - float64(pos*(pos+1))/2
	return u / float64(pos*neg), nil
}

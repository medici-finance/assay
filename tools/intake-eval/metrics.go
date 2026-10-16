package main

import (
	"fmt"
	"sort"
	"strings"
)

// ExitScore is one exit's confusion counts and derived metrics. A metric that
// is undefined for the data (division by zero) is nil — reported as
// "unmeasured", never as a pass and never as 0.
type ExitScore struct {
	Support   int      `json:"support"`   // scored fixtures whose expected exit is this one
	Predicted int      `json:"predicted"` // scored fixtures predicted into this exit
	TP        int      `json:"tp"`
	FP        int      `json:"fp"`
	FN        int      `json:"fn"`
	Precision *float64 `json:"precision"`
	Recall    *float64 `json:"recall"`
	F1        *float64 `json:"f1"`
}

// Score is the full result of scoring one prediction set against a gold set.
type Score struct {
	Scored   int                   `json:"scored"`
	Correct  int                   `json:"correct"`
	Accuracy *float64              `json:"accuracy"` // informational only; never the verdict
	Exits    map[string]*ExitScore `json:"exits"`
	// Scored fixtures with no prediction: each counts as a miss on its expected exit.
	Missing []string `json:"missing"`
	// Predictions for excluded (not-ground-truth) fixtures, ignored.
	IgnoredExcluded []string `json:"ignored_excluded"`
}

func ratio(num, den int) *float64 {
	if den == 0 {
		return nil
	}
	v := float64(num) / float64(den)
	return &v
}

// score computes per-exit precision, recall and F1 over the scored fixtures.
//
//	precision = TP / (TP + FP)  — unmeasured when nothing was predicted into the exit
//	recall    = TP / (TP + FN)  — unmeasured when the exit has no scored fixture
//	F1        = 2TP / (2TP + FP + FN) — unmeasured whenever recall is
//
// F1 uses the count form so an exit that has fixtures but receives no
// predictions (a collapsed class) scores F1 = 0 rather than undefined.
func score(gs *GoldSet, pred map[string]string) (*Score, error) {
	known := map[string]bool{}
	for _, f := range gs.Fixtures {
		known[f.ID] = true
	}
	scored := map[string]bool{}
	for _, f := range gs.Scored {
		scored[f.ID] = true
	}
	var unknown []string
	s := &Score{Exits: map[string]*ExitScore{}, Missing: []string{}, IgnoredExcluded: []string{}}
	for id := range pred {
		switch {
		case !known[id]:
			unknown = append(unknown, id)
		case !scored[id]:
			s.IgnoredExcluded = append(s.IgnoredExcluded, id)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return nil, fmt.Errorf("predictions name fixture ids not in the gold set (wrong gold set?): %s", strings.Join(unknown, ", "))
	}
	sort.Strings(s.IgnoredExcluded)

	for _, e := range exits {
		s.Exits[e] = &ExitScore{}
	}
	for _, f := range gs.Scored {
		s.Scored++
		want := f.ExpectedExit
		got, ok := pred[f.ID]
		s.Exits[want].Support++
		if !ok {
			s.Missing = append(s.Missing, f.ID)
			s.Exits[want].FN++
			continue
		}
		s.Exits[got].Predicted++
		if got == want {
			s.Correct++
			s.Exits[want].TP++
		} else {
			s.Exits[want].FN++
			s.Exits[got].FP++
		}
	}
	sort.Strings(s.Missing)
	s.Accuracy = ratio(s.Correct, s.Scored)
	for _, e := range exits {
		x := s.Exits[e]
		x.Precision = ratio(x.TP, x.TP+x.FP)
		x.Recall = ratio(x.TP, x.TP+x.FN)
		if x.Recall != nil {
			x.F1 = ratio(2*x.TP, 2*x.TP+x.FP+x.FN)
		}
	}
	return s, nil
}

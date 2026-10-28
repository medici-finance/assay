package reviewflow

import "time"

// BoardPlan is the shared legacy/observation plan assembly. Its verbs are only
// proposals; the returned idle verdict is scoped to reviewer dispatch depth.
type BoardPlan struct {
	Depths map[Disposition]int
	Verbs  []PlannedVerb
	Idle   IdleVerdict
}

// PlanBoard preserves the legacy cadence-based idle contract. New observations
// use their caller-owned freshness policy before reaching the same assembly.
func PlanBoard(b *Board, now time.Time) BoardPlan { return assemblePlan(b, Idle(b, now)) }
func assemblePlan(b *Board, idle IdleVerdict) BoardPlan {
	depths := map[Disposition]int{}
	if b != nil {
		depths = b.CountByDisposition()
	}
	return BoardPlan{Depths: depths, Verbs: Coalesce([]*Board{b}), Idle: idle}
}

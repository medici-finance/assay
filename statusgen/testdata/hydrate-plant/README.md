# hydrate-plant

Fixture for TestHydrateBudget, TestHydrateBudgetPlant and TestHydrateRealTree
(hydrate_budget_test.go).

`brief.md` is the real-shape brief: board status `implemented`, a recorded
`VERIFY: FAIL`, an issue number and a declared `files:` path. Every brief in
the generated corpus carries it, so each one is a "waiting brief" whose
stale-FAIL notice needs a query of main's git history (stalefail.go,
newestFailFix). The corpus is a real git repository with merge commits on main
after the FAIL date, which is what makes that query expensive.

The plant is the test hook `briefHistoryNoticesInHydration`: setting it makes
the hydrating loader compute those notices again, which is the cost statusgen/44
removed. TestHydrateBudgetPlant sets it and asserts the budget check fails.

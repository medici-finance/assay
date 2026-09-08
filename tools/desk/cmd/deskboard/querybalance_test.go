package main

// querybalance_test.go — the board's open-PR GraphQL read constant moved to the forge
// backend (internal/deskkit, ghOpenChangesQuery) when fetchOpenPRs was migrated onto the typed
// ListOpenChanges op (the read-verbs-on-the-seam migration), so its never-executed-string brace-typo guard moved
// with it (internal/deskkit/openchangesquery_test.go). No GraphQL query constant remains in
// this package to balance-check.

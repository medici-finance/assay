// Package deskkit is a FIXTURE for TestGitHubMinterGuardCatchesPlantedCallers: declarations
// that share a guarded link's spelling. A declared name is never a reference (the real tree's
// `var tokenMinter = ...` in roletoken.go is the case this pins), so the guard must report
// nothing here — only the names a declaration's type or value USES count.
package deskkit

var tokenMinter = func(role, owner string) (string, string, error) { return "", "", nil }

var (
	roleTokenMemo = map[roleOwnerKey]roleTokenMemoEntry{}
	custody       = "declared, not referenced"
)

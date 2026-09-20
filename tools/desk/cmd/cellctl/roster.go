package main

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// rosterParses answers `cellctl check`'s "roster parses under the cell home" row.
//
// The shell oracle shells out to `env HOME=<cell home> deskroster repos --scope scan` and reads
// its exit code. The port does NOT re-implement that shell-out: it asks deskkit the same
// question in-process through deskkit.LoadConfig, which is the very code the shelled-to binary
// runs. That is the brief's requirement, not a preference — a port that keeps the shell-out
// keeps a second copy of the roster's meaning.
//
// Only HOME is overridden, exactly as the oracle's `env HOME=…` does — everything else in the
// environment (ASSAY_CONFIG_HOME included) is left as the caller set it, because that is what
// the shelled-to binary would have inherited. Mirroring that is what keeps the two answers the
// same answer.
func (c *Cell) rosterParses() bool {
	restore := withHome(c.Home)
	defer restore()
	cfg := deskkit.LoadConfig(deskkit.ClassWrite)
	return cfg.Configured() && len(cfg.Problems) == 0
}

// rosterAllowedRepos is the RAW write-authorisation scope line the cell's own roster carries —
// the value `cellctl check`'s scrubbed exact-scope row compares against CELL_REPO_SLUG.
//
// The NAME says what it reads: ASSAY_ALLOWED_REPOS, the roster's write-authorisation set. An
// earlier revision renamed it to a scope-line spelling specifically so the P3 echo-coverage guard's
// substring detector would stop seeing cellctl — which is a rename that hides the property, not
// one that satisfies it. cellctl DOES consult a write-scope-bearing control surface, so it is in
// that guard's class and main declares its tool class and echoes its effective config like every
// other roster-reading main. The honest name is restored here, and the guard now sees this
// binary for the reason it should: because it reads this.
//
// The KEY is deskkit's own constant (deskkit.EnvAllowedRepos), never a string literal spelled
// again here: the roster variable's NAME has exactly one definition in this tree, and the port
// is not entitled to a second.
//
// The comparison is on the raw value, as the oracle's is. A typed read of the parsed repo set
// would answer a DIFFERENT question — "does the scope resolve to this one repo", which a policy
// suffix like `:no-ci:public` satisfies too — and silently widening the row this brief is
// porting is not a change a port gets to make. Narrowing or widening it is a brief of its own.
func (c *Cell) rosterAllowedRepos() string {
	f, err := os.Open(filepath.Join(c.Config, "roster.env"))
	if err != nil {
		return ""
	}
	defer f.Close()
	val := ""
	prefix := deskkit.EnvAllowedRepos + "="
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		if line := sc.Text(); strings.HasPrefix(line, prefix) {
			val = strings.TrimPrefix(line, prefix)
		}
	}
	return val
}

// withHome points HOME at the cell home for the duration of one in-process deskkit call and
// returns the undo. A deskkit read resolves its configuration from the process environment, so
// this is how a caller asks it "what does THIS home's roster say" — the in-process equivalent of
// the oracle's `env HOME=… <verb>`.
func withHome(home string) func() {
	prev, had := os.LookupEnv("HOME")
	_ = os.Setenv("HOME", home)
	return func() {
		if had {
			_ = os.Setenv("HOME", prev)
			return
		}
		_ = os.Unsetenv("HOME")
	}
}

package writepolicy

import (
	"strings"
	"testing"
	"time"
)

func ghFilterClassified(cmd string) bool {
	return Classify(Request{Tool: "Bash", Target: cmd}, LexicalContext{}).ReadsOutsideGhOutput()
}

// regression: #5078
func TestGhFilterReadsRefused(t *testing.T) {
	for _, cmd := range []string{
		// The names, on reads with no status word.
		`gh api repos/o/r --jq env.GH_X`,
		`gh api repos/o/r --jq '$ENV.GH_X'`,
		`gh pr view 1 --json number --jq 'env'`,
		`gh pr list --json number -q 'now'`,
		`gh pr list --json createdAt --jq '.[0].createdAt|fromdate|localtime'`,
		`gh pr list --json createdAt --jq '.[0].createdAt|fromdate|strflocaltime("%H")'`,
		`gh release view --json name -q '$ENV'`,
		// Every flag spelling gh accepts.
		`gh api x --jq=env.X`,
		`gh api x -q env.X`,
		`gh api x -q=env.X`,
		`gh api x -qenv.X`,
		`gh api x -iq env.X`,
		`gh api x -iqenv.X`,
		`gh api x "--jq" 'env.X'`,
		`gh api x '--jq=env.X'`,
		"gh api x --jq \\\n  env.X",
		// Where the name can sit in a filter.
		`gh api x --jq '.a|env'`,
		`gh api x --jq '{$ENV}'`,
		`gh api x --jq '{a:env}'`,
		`gh api x --jq '.[env.X]?'`,
		`gh api x --jq 'def f: env; f.X'`,
		`gh api x --jq 'env # note'`,
		`gh api x --jq '"\(env.X)"'`,
		`gh api x --jq '@text "a\(env.X)b"'`,
		`gh api x --jq '"\("\(env.X)")"'`,
		`gh api x --jq '"\((1)|env)"'`,
		`gh api x --jq '"a\("b")c" | env'`,
		`gh api x --jq '..env'`,
		// How the shell can spell it.
		`gh api x --jq "env.X"`,
		`gh api x --jq "\$ENV.X"`,
		`gh api x --jq e''nv.X`,
		`gh api x --jq e\nv.X`,
		`gh api x --jq $'env.X'`,
		`gh api x --jq $"env.X"`,
		`gh api x --jq "\"a\"|env"`,
		// Quoting that cannot be trusted gives a string no credit.
		`gh api x --jq '# "' --jq 'env'`,
		"gh api x --jq '# \"\nenv # \"'",
		`bash -c "gh api x --jq 'env.X'"`,
		`bash -c 'gh api x --jq "env.X"'`,
		`bash -c "gh api x --jq \"env.X\""`,
		`bash -c "gh api x --jq '\"a\"|env'"`,
		`bash -c 'gh api x --jq '\''env.X'\'''`,
		// Beside other commands, and on any gh path.
		`/usr/local/bin/gh api x --jq env.X`,
		`A=1 gh api x --jq env.X | cat`,
		`gh api x 2>&1 --jq env.X`,
		`echo "-q "; gh api x --jq env.X`,
		`gh alias set a 'api x --jq env.X'`,
		// Accepted false refusals: not a filter, or a key that is not quoted.
		`gh pr list --json title | grep -q env`,
		`gh pr view 1 | grep -iq now`,
		`gh api x --jq '{env:.a}'`,
		`gh api x --jq '{env}'`,
		`gh api x --jq '.[]|{now:.a}'`,
		`gh api x --jq '.a # env'`,
	} {
		if !ghFilterClassified(cmd) {
			t.Errorf("%q: a filter that reads outside the output must report true", cmd)
		}
	}
}

// regression: #5078
func TestGhFilterOrdinaryAllowed(t *testing.T) {
	for _, cmd := range []string{
		// Selection and formatting, including the recipes' own shapes.
		`gh api user --jq .login`,
		`gh api x -q .head.sha`,
		`gh pr view 1 --json state --jq .state`,
		`gh pr list --json number,title --jq '.[] | select(.title|test("x";"i")) | .number'`,
		`gh pr view 1 --json reviews --jq '[.reviews[] | {a:.author.login, s:.state}] | sort_by(.s) | last'`,
		`gh pr view 1 --json labels --jq '[.labels[].name] | join(",")'`,
		`gh pr list --json number --jq 'map(.number) | length'`,
		`gh pr list --json createdAt --jq '.[0].createdAt | fromdate | todate'`,
		`gh pr list --json createdAt --jq '.[0].createdAt | fromdate | strftime("%Y")'`,
		`gh api x --jq '.a as $x | $x.b'`,
		`gh api x --jq '.[] | "\(.number) \(.title)"'`,
		`gh api x --jq "[.[] | select(.number==$N)]"`,
		`gh api x --jq ".[] | select(.name==\"main\")"`,
		// A field, a key in quotes, a string, a jq variable: none reads anything.
		`gh api x --jq .env`,
		`gh api x --jq .a.env.now`,
		`gh api x --jq '.a?.env'`,
		`gh api x --jq '.[0].localtime'`,
		`gh api x --jq '.ENV'`,
		`gh api x --jq '."env"'`,
		`gh api x --jq '.["env"]'`,
		`gh api x --jq '{"env": .a}'`,
		`gh api x --jq 'select(.name=="env")'`,
		`gh api x --jq '.a|contains("now")'`,
		`gh api x --jq '"\(.a) env \(.b) now"'`,
		`gh api x --jq '"\("env")"'`,
		`gh api x --jq '. as $env | $env'`,
		`gh api x --jq 'environment, renv, env_x, know, nowhere'`,
		`gh api x --jq "select(.name==\"env\")"`,
		// Not a filter of gh's at all.
		`gh pr view 1`,
		`gh pr view 1 --json number`,
		`gh pr create -t env --body-file f`,
		`gh api x --template '{{.env}}'`,
		`gh api x -t 'env: {{.a}}'`,
		`gh pr list --jq-ish env`,
		`env A=1 gh pr view 1 --json a --jq .a`,
		`gh pr view 1 --json a | grep --quiet env`,
		`gh api graphql -f query='{ env }'`,
		// Stated limits: a plain shell print, a filter outside gh, a computed filter.
		`gh pr view 1 && printenv GH_X`,
		`gh pr view 1; echo "$GH_X"`,
		`gh pr view 1 --json a | jq 'env.GH_X'`,
		`f='env.GH_X'; gh api x --jq "$f"`,
		`gh api x --jq "$(cat f.jq)"`,
		// No gh word, no finding.
		`mytool --jq env.X`,
		`grep -q env notes.md`,
		`gh`,
		``,
		`--jq`,
		`-q`,
		`gh api x --jq`,
		`gh api x -q`,
	} {
		if ghFilterClassified(cmd) {
			t.Errorf("%q: must stay allowed", cmd)
		}
	}
}

func TestGhFilterScanWorkBounded(t *testing.T) {
	long := strings.Repeat("-q '.a' ", 200000)
	for _, cmd := range []string{
		"gh api x " + long,
		"gh api x --jq '" + strings.Repeat(`"\(`, 300000) + "'",
		"gh api x --jq '" + strings.Repeat("(", 1000000) + "'",
		"gh api x --jq '" + strings.Repeat("# ", 500000) + "'",
		"gh api x --jq " + strings.Repeat("-", 1000000),
		strings.Repeat("-q env ", 200000),
	} {
		start := time.Now()
		if ghFilterReads(cmd) {
			t.Errorf("%.40q…: holds no name that reads, or no gh", cmd)
		}
		if d := time.Since(start); d > 5*time.Second {
			t.Errorf("%.40q…: took %v", cmd, d)
		}
	}
}

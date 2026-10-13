package evaluation

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

// TestRowInputsGrammar — the closed grammar: each derivable row yields
// exactly the paths it reads; everything else is refused with a reason,
// never approximated.
func TestRowInputsGrammar(t *testing.T) {
	ok := []struct {
		cmd  string
		want []string
	}{
		{"grep -c a src/a.go", []string{"src/a.go"}},
		{"grep -q -e foo -e bar src/a.go src/b.go", []string{"src/a.go", "src/b.go"}},
		{"grep -qE 'a|b' -- src/a.go", []string{"src/a.go"}},
		{"cd tools && grep -c x a.go", []string{"tools/a.go"}},
		{"cd tools && cd sub && cat a.go", []string{"tools/sub/a.go"}},
		{"sed -n '1,5p' src/a.go \\| grep -c x", []string{"src/a.go"}},
		{"sed -n '/^x/,$p' src/a.go | wc -l", []string{"src/a.go"}},
		{"test -f src/a.go && echo ok", []string{"src/a.go"}},
		{"head -n 3 src/a.go | tail -1 | tr -d x", []string{"src/a.go"}},
		{"cat src/a.go | sort -u | uniq -c | cut -d, -f1", []string{"src/a.go"}},
		{"set -o pipefail; grep -c x \"src/a b.go\"", []string{"src/a b.go"}},
		{"grep -c x src/a.go || true", []string{"src/a.go"}},
		{"wc -l src && cat ./src/a.go", []string{"src", "src/a.go"}},
	}
	for _, c := range ok {
		got, err := deriveRowInputs(VerifyRow{Command: c.cmd})
		if err != nil {
			t.Errorf("%q: want %v, got the refusal %v", c.cmd, c.want, err)
			continue
		}
		if !reflect.DeepEqual(got.paths, c.want) {
			t.Errorf("%q: want %v, got %v", c.cmd, c.want, got.paths)
		}
	}
	refused := []struct{ cmd, shell, why string }{
		{"go test ./statusgen", "", "outside the set"},
		{"bash scripts/x.sh", "", "outside the set"},
		{"git diff --stat", "", "outside the set"},
		{"grep -r x src", "", "grep option -r"},
		{"grep -c x src/a.go -r", "", "grep option -r"},
		{"grep -f pats src/a.go", "", "grep option -f"},
		{"grep --include=x a src", "", "grep option"},
		{"grep -c x $(ls)", "", "metacharacter"},
		{"grep -c x \"$F\"", "", "inside double quotes"},
		{"cat src/*.go", "", "metacharacter"},
		{"cat src/a.go > out", "", "metacharacter"},
		{"cat < src/a.go", "", "metacharacter"},
		{"cat src/a.go &", "", "background"},
		{"cat ~/x", "", "metacharacter"},
		{"cat ../x", "", "climbs"},
		{"cat src/../../x", "", "climbs"},
		{"cat /etc/hosts", "", "plain relative"},
		{"cat .git/config", "", ".git"},
		{"cat .", "", "repository root"},
		{"cat $F", "", "metacharacter \"$\""},
		{"grep -c a src/a.go && sh x.sh", "", "outside the set"},
		{"cd src ; cat a.go", "", "joined by"},
		{"cd src && cat a.go ; cat b.go", "", "joined by"},
		{"true || cd src && cat a.go", "", "joined by"},
		{"cd src || cat a.go", "", "joined by"},
		{"cd src | cat a.go", "", "stage of its own"},
		{"cd src a && cat a.go", "", "stage of its own"},
		{"FOO=1 cat src/a.go", "", "assignment"},
		{"sed 's/a/b/' src/a.go", "", "sed other than"},
		{"sed -n '1w out' src/a.go", "", "sed other than"},
		{"sort -o out src/a.go", "", "sort option -o"},
		{"grep -c x", "", "standard input"},
		{"echo hi | cat src/a.go", "", "filter cat names a file"},
		{"cat src/a.go | uniq -c out", "", "uniq with an operand"},
		{"echo ok", "", "reads no repository path"},
		{"test -d src", "", "test other than"},
		{"set -x; cat src/a.go", "", "set other than"},
		{"cat 'src/a  b.go'", "", "repeated space"},
		{"cat 'src/a.go", "", "unterminated"},
		{"cat src/a.go && ", "", "empty command"},
		{"cat src/a.go", "pwsh", "runs under pwsh"},
	}
	for _, c := range refused {
		_, err := deriveRowInputs(VerifyRow{Command: c.cmd, Shell: c.shell})
		if err == nil || !errors.Is(err, errUnderivable) || !strings.Contains(err.Error(), c.why) {
			t.Errorf("%q: want a refusal naming %q, got %v", c.cmd, c.why, err)
		}
	}
}

// TestBookkeepingPathsNotStamps — the board's bookkeeping exemptions stay
// docs/streams/** and STATUS.md: a release stamp or changelog fragment is never
// exempt by name (#2026).
func TestBookkeepingPathsNotStamps(t *testing.T) {
	stamps := []string{"changelog/x.md", ".claude-plugin/marketplace.json", ".assay-versions", "README.md", "CHANGELOG.md"}
	for _, p := range stamps {
		if isBoardBookkeepingPath(p) || isVerifyWrittenPath(p) {
			t.Errorf("%s is exempt by name; a witness stops being stale only because its row does not read a path (#2026)", p)
		}
	}
}

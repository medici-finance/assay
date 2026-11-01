package writepolicy

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
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
		// The flag spellings.
		`gh api x --jq=env.X`,
		`gh api x -q env.X`,
		`gh api x -q=env.X`,
		`gh api x -qenv.X`,
		`gh api x -iq env.X`,
		`gh api x -iqenv.X`,
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
		`gh api x --jq $'env.X'`,
		`gh api x --jq $"env.X"`,
		// Quoting that cannot be trusted gives a string no credit.
		`gh api x --jq '# "' --jq 'env'`,
		"gh api x --jq '# \"\nenv # \"'",
		`bash -c "gh api x --jq 'env.X'"`,
		`bash -c 'gh api x --jq "env.X"'`,
		`bash -c "gh api x --jq \"env.X\""`,
		`bash -c "gh api x --jq '\"a\"|env'"`,
		// Beside other commands, and on gh paths.
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
		// A filter the shell computes is not read.
		`f='.a'; gh api x --jq "$f"`,
		`gh api x --jq "$(printf .a)"`,
		`gh api x --jq "${f}"`,
		"gh api x --jq `printf .a`",
		`gh api x --jq $f --paginate`,
		// Braces that are not an expansion, or that cannot produce the flag.
		`gh api repos/{owner}/{repo} --jq .a`,
		`gh api x --jq '{a,b}'`,
		`gh pr list --state={open,closed} --jq .a`,
		`gh api r-{a,b} --jq .a`,
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

// ghFilterStep is a line whose filter the listed shells hand to gh, and whether the
// check must refuse it; each shell's own reading of the line is checked too.
type ghFilterStep struct {
	line, filter string
	shells       []string
}

func ghFilterSteps() []ghFilterStep {
	all := []string{"sh", "dash", "bash", "zsh -f"}
	var rows []ghFilterStep
	add := func(line, filter string, shells ...string) {
		if shells == nil {
			shells = all
		}
		rows = append(rows, ghFilterStep{line, filter, shells})
	}
	// Text with a quote the reader takes as opening one, before an ordinary filter.
	for _, before := range []string{
		"# it's\n",
		"# \"\n",
		"true # '\n",
		"cat <<E >/dev/null\n'\nE\n",
		"cat <<'E' >/dev/null\nit's\nE\n",
		"echo ${u:-'}'} >/dev/null; ",
		`echo ${u:-"}"} >/dev/null; `,
		`echo ${u#'}'} >/dev/null; `,
	} {
		add(before+"gh api x --jq '.a|env'", ".a|env")
		add(before+`gh api x -q ".a|env.X"`, ".a|env.X")
		add(before+`eval "gh api x --jq '.a|env'"`, ".a|env")
		if strings.HasSuffix(before, "\n") {
			add(before+`eval gh\ api\ x\ --jq\ \'.a\|env\'`, ".a|env")
		}
		// The same text inside the string, with the filter escaped there too.
		add(`eval '`+strings.ReplaceAll(before+"gh api x --jq '.a|env'", "'", `'\''`)+`'`, ".a|env")
		add(`eval "`+strings.ReplaceAll(strings.ReplaceAll(before, `"`, `\"`), `$`, `\$`)+`gh api x --jq \".a|env\""`, ".a|env")
	}
	// A brace in quotes in an expansion beside the filter.
	add(`gh api x --jq ${u:+'}'}'.a|env'`, ".a|env")
	add(`gh api x --jq ${u:+"}"}'.a|env'`, ".a|env")
	add(`gh api x --jq "${u:+"}"}.a|env"`, ".a|env")
	// A `)` the reader takes as closing a substitution. The older bash some hosts
	// have as sh cannot parse these.
	add(`echo "$(case x in x) gh api y --jq ".a|env";; esac)"`, ".a|env", "dash", "bash", "zsh -f")
	add("echo \"$(true # )\ngh api y --jq \".a|env\")\"", ".a|env", "dash", "bash", "zsh -f")
	// The rest of a redirection operator.
	for _, f := range []string{"--jq", "-q"} {
		add("gh api x "+f+" <<- E env.X\nbody\nE", "env.X")
		add("gh api x "+f+" <<-E env.X\nbody\nE", "env.X")
		add("gh api x "+f+" >! o env.X", "env.X", "zsh -f")
		add("gh api x "+f+" >!o env.X", "env.X", "zsh -f")
		add("gh api x "+f+" >>! o env.X", "env.X", "zsh -f")
		add("gh api x "+f+" >&! o env.X", "env.X", "zsh -f")
		add("gh api x "+f+" &>! o env.X", "env.X", "zsh -f")
		add("gh api x "+f+" >! env.X", "env.X", "sh", "dash", "bash")
		// An argument, not a descriptor.
		add("gh api x "+f+" {a:env.X}>o", "{a:env.X}")
		add("gh api x "+f+" {env.X}>o", "{env.X}")
		// A carriage return is a byte of the word.
		add("gh api x "+f+" \renv.X", "\renv.X")
		// The shells differ on the `$` of `$\"`.
		add("gh api x "+f+` $"ENV.X"`, "$ENV.X", "dash", "zsh -f")
		add("gh api x "+f+` $'ENV.X'`, "$ENV.X", "dash")
		// Beside an expansion: the shell reads one digit, and an unset name gives nothing.
		add("gh api x "+f+" $1env.X", "env.X")
		add("gh api x "+f+` "$1env.X"`, "env.X")
		add("gh api x "+f+"$1 env.X", "env.X")
		add("gh api x "+f+"$u env.X", "env.X")
	}
	add("gh api x --jq=\renv.X", "\renv.X")
	add("gh api x -iq\renv.X", "\renv.X")
	add("gh api x -q=\renv.X", "\renv.X")
	add("gh api x --jq=$1env.X", "env.X")
	return rows
}

// regression: #5078
func TestGhFilterOutOfStepRefused(t *testing.T) {
	// A real shell hands each filter over, so none of these is a false refusal.
	want := map[string][]ghFilterStep{}
	for _, row := range ghFilterSteps() {
		if !ghFilterClassified(row.line) {
			t.Errorf("%q: gh gets filter %q, and the check allowed it", row.line, row.filter)
		}
		for _, sh := range row.shells {
			want[sh] = append(want[sh], row)
		}
	}
	for sh, rows := range want {
		t.Run(sh, func(t *testing.T) {
			lines := make([]string, len(rows))
			for i, r := range rows {
				lines[i] = r.line
			}
			for i, argv := range ghFilterShellArgv(t, strings.Fields(sh), lines, true) {
				if !slices.Contains(ghFilterValues(argv), rows[i].filter) {
					t.Errorf("%q: argv %q does not hand over %q", lines[i], argv, rows[i].filter)
				}
			}
		})
	}
}

// regression: #5078
func TestGhFilterOutOfStepAllowed(t *testing.T) {
	for _, cmd := range []string{
		"# it's\ngh api x --jq '.a|.b'",
		"# it's\ngh api x --jq '.a|select(.b==\"env\")'",
		"# \"\ngh api x --jq \".a|.b\"",
		"cat <<'E' >/dev/null\nit's\nE\ngh api x --jq '.a|.b'",
		`echo ${u:-'}'} >/dev/null; gh api x --jq '.a|.b'`,
		`gh api x --jq ${u:+'}'}'.a|.b'`,
		`gh api x --jq "${u:-'}".a`,
		"gh api x --jq <<- E .a\nbody\nE",
		"gh api x --jq >! o .a",
		"gh api x --jq >!o .a",
		"gh api x --jq &>! o .a",
		"gh api x --jq {x}>o .a",
		"gh api x --jq {x_1}>o .a",
		"gh api x --jq 2>o .a",
		"gh api x --jq .a\r\ngh api y --jq .b\r",
		`gh api x --jq $".a"`,
		`gh api x --jq $'.a'`,
		`gh api x --jq $"env_x"`,
		`gh api x --jq "$1.a"`,
		`gh api x --jq$u .a`,
		`gh api x -q$u .a`,
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
		"gh " + strings.Repeat("$(", 300000),
		"gh " + strings.Repeat(`"$(`, 200000),
		"gh " + strings.Repeat("${", 300000),
		"gh " + strings.Repeat("`", 1000001),
		"gh " + strings.Repeat(`'"'"`, 200000),
		"gh " + strings.Repeat(`\`, 1000000),
		"gh " + strings.Repeat("> ", 500000),
		"gh api x --jq " + strings.Repeat("2>o ", 200000),
		"gh api x --jq " + strings.Repeat("$f ", 300000),
		"gh " + strings.Repeat("{a,", 300000),
		// The reads that take no state from earlier text.
		"gh " + strings.Repeat("-q ' ", 200000),
		"gh " + strings.Repeat(`-q " `, 200000),
		"gh " + strings.Repeat(`"$( `, 200000),
		"gh " + strings.Repeat("- ", 500000) + "'",
		"gh " + strings.Repeat("$ ", 500000) + `"`,
		"gh " + strings.Repeat("-q 'a\n", 150000),
		"gh\n" + strings.Repeat("'\n", 500000),
		"gh " + strings.Repeat(`"-q `, 200000),
		"gh " + strings.Repeat(`'bash -c " `, 100000),
		"gh " + strings.Repeat(`\ `, 500000),
		"gh " + strings.Repeat(`-q\ `, 250000),
		"gh\n" + strings.Repeat(`\ `, 500000),
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

// regression: #5078
func TestGhFilterBudgetFailClosed(t *testing.T) {
	// Text that spends the budget is judged whole, with no string taken as one. These
	// rows show that judgment; a false refusal on such text is one cost of it.
	pad := "gh " + strings.Repeat(`\ `, 200000)
	for _, tail := range []string{"\n# it's\ngh api x --jq '.a|env'", ` --jq 'env.X'`, ` --jq '"env"'`} {
		start := time.Now()
		if !ghFilterReads(pad + tail) {
			t.Errorf("padding then %q: must report true", tail)
		}
		if d := time.Since(start); d > 5*time.Second {
			t.Errorf("padding then %q: took %v", tail, d)
		}
	}
	if ghFilterReads(pad + ` --jq '.a'`) {
		t.Errorf("padding then an ordinary filter: holds no name, must stay allowed")
	}
	if ghFilterReads(`gh api x --jq '"env"'`) {
		t.Errorf("this short call should not spend the budget, so its string keeps its credit")
	}
}

// regression: #5078
func TestGhFilterExpansionsNotRead(t *testing.T) {
	// The name sits only inside the expansion, and each expansion gives nothing.
	for _, n := range []string{"env", "now", "localtime", "strflocaltime", "ENV"} {
		for _, form := range []string{`"${%s}"`, `"$%s"`, `${%s}`, `$%s`, `"$(true %s)"`, `$(true %s)`, "\"`true %s`\"", `".[]|select(.t<${%s})"`} {
			value := fmt.Sprintf(form, n)
			for _, line := range []string{"gh api x --jq " + value, "gh api x --jq=" + value, "gh api x -q " + value} {
				if ghFilterClassified(line) {
					t.Errorf("%q: what the shell expands is not filter text", line)
				}
			}
		}
		// Where the shell does not expand it, the text is the filter.
		for _, form := range []string{`'${%s}'`, `"\${%s}"`, `'$(%s)'`} {
			line := "gh api x --jq " + fmt.Sprintf(form, n)
			if !ghFilterClassified(line) && n != "ENV" {
				t.Errorf("%q: this text reaches the filter", line)
			}
		}
		if n == "ENV" {
			continue
		}
		// A substituted command still runs, so the reader tries to read its own filter.
		// These rows show that in the forms they list.
		for _, form := range []string{`echo "$(gh api x --jq %s.X)"`, `echo $(gh api x --jq %s.X)`, "echo `gh api x --jq %s.X`", `echo "${a:-$(gh api x --jq %s.X)}"`, `gh api x --jq "$(gh api y --jq %s.X)"`} {
			line := fmt.Sprintf(form, n)
			if !ghFilterClassified(line) {
				t.Errorf("%q: a substituted command's filter must be read", line)
			}
		}
	}
}

// regression: #5078
func TestGhFilterStringCreditPlain(t *testing.T) {
	// A jq string is harmless text, but it is only taken as one when the flag and the
	// value are each written plainly; any other quoting gets no credit.
	str := `"env"`
	plain := []string{"--jq '" + str + "'", "-q '" + str + "'", "--jq='" + str + "'"}
	for _, v := range plain {
		if line := "gh api x " + v; ghFilterReads(line) {
			t.Errorf("%q: a plainly written string must stay allowed", line)
		}
	}
	for _, f := range []string{"--jq", "-q"} {
		for _, m := range ghFilterQuoted(f) {
			if line := "gh api x " + m + " '" + str + "'"; !ghFilterReads(line) {
				t.Errorf("%q: a quoted flag gets no string credit", line)
			}
		}
		for i := 1; i < len(str); i++ {
			line := "gh api x " + f + " '" + str[:i] + "''" + str[i:] + "'"
			if !ghFilterReads(line) {
				t.Errorf("%q: a value joined from pieces gets no string credit", line)
			}
		}
	}
}

// regression: #5078
func TestGhFilterDeepNesting(t *testing.T) {
	for _, v := range []struct {
		value string
		want  bool
	}{{"env.X", true}, {".a", false}} {
		line := "gh api x --jq " + v.value
		for level := 1; level <= 10; level++ {
			line = "eval '" + strings.ReplaceAll(line, "'", `'\''`) + "'"
			if got := ghFilterReads(line); got != v.want {
				t.Errorf("value %q at level %d: got %v", v.value, level, got)
			}
		}
	}
	for _, c := range []struct{ open, close string }{{"$(", ")"}, {"`", "`"}} {
		line := "gh " + strings.Repeat(c.open, 40) + "x --jq env.X" + strings.Repeat(c.close, 40)
		if !ghFilterReads(line) {
			t.Errorf("%.30q…: past the depth bound the text gets no credit", line)
		}
	}
}

// ghFilterRedirections lists redirection shapes, one blank-joined string each.
func ghFilterRedirections() []string {
	var out []string
	for _, fd := range []string{"", "2", "9", "{x}"} {
		for _, op := range []string{">", ">>", "<", "<>", ">|"} {
			for _, gap := range []string{"", " "} {
				out = append(out, fd+op+gap+"o")
			}
		}
	}
	out = append(out, "&>o", "&> o", "&>>o", "2>&1", ">&2", "2>o 9>o", "<o >o")
	return out
}

// ghFilterWords lists every string over alphabet up to length max.
func ghFilterWords(alphabet string, max int) []string {
	words, last := []string{}, []string{""}
	for n := 0; n < max; n++ {
		var next []string
		for _, w := range last {
			for _, c := range alphabet {
				next = append(next, w+string(c))
			}
		}
		words, last = append(words, next...), next
	}
	return words
}

// ghFilterMixes: the word's bytes in order, with up to n quoting bytes placed
// around and between them.
func ghFilterMixes(word string, n int) []string {
	mixes, seen := []string{word}, map[string]bool{word: true}
	for from := 0; n > 0; n-- {
		end := len(mixes)
		for _, m := range mixes[from:end] {
			for i := 0; i <= len(m); i++ {
				for _, q := range []string{`'`, `"`, `\`} {
					if w := m[:i] + q + m[i:]; !seen[w] {
						seen[w] = true
						mixes = append(mixes, w)
					}
				}
			}
		}
		from = end
	}
	return mixes
}

// ghFilterQuoted: the word's other writings that every listed shell still reads as
// the word, with one backslash or one pair of quotes placed in it.
func ghFilterQuoted(word string) []string {
	var out []string
	for i := range word {
		out = append(out, word[:i]+`\`+word[i:])
		for j := i; j <= len(word); j++ {
			for _, q := range []string{`'`, `"`} {
				out = append(out, word[:i]+q+word[i:j]+q+word[j:])
			}
		}
	}
	return out
}

// ghFilterClosed reports whether a shell reads w with no quote left open and no
// backslash left at its end.
func ghFilterClosed(w string) bool {
	for i := 0; i < len(w); i++ {
		switch w[i] {
		case '\\':
			i++
			if i == len(w) {
				return false
			}
		case '\'':
			j := strings.IndexByte(w[i+1:], '\'')
			if j < 0 {
				return false
			}
			i += j + 1
		case '"':
			for i++; i < len(w) && w[i] != '"'; i++ {
				if w[i] == '\\' {
					i++
				}
			}
			if i >= len(w) {
				return false
			}
		}
	}
	return true
}

// ghFilterShellArgv runs each line under the shell with gh as a function that
// records its arguments, and returns them by line; nil when no gh call ran. With
// alone set each line has its own subshell, so that one the shell cannot parse
// does not end the run.
func ghFilterShellArgv(t *testing.T, shell []string, lines []string, alone bool) [][]string {
	t.Helper()
	dir := t.TempDir()
	var b strings.Builder
	b.WriteString("exec 3>out.txt\ngh() { for a in \"$@\"; do printf '%s\\001' \"$a\" >&3; done; printf '\\n' >&3; }\n")
	for i, l := range lines {
		if alone {
			l = "(eval '" + strings.ReplaceAll(l, "'", `'\''`) + "') 2>/dev/null"
		}
		fmt.Fprintf(&b, "printf '#%d\\n' >&3\n%s\nwait\n", i, l)
	}
	path := filepath.Join(dir, "read.sh")
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "o"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	bin, err := exec.LookPath(shell[0])
	if err != nil {
		t.Skipf("could not check: %s is not on PATH: %v", shell[0], err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, append(append([]string{}, shell[1:]...), path)...)
	cmd.Dir, cmd.Env = dir, []string{"HOME=" + dir, "LC_ALL=C", "PATH=" + os.Getenv("PATH")}
	_ = cmd.Run() // a line that is no command is the shell's to reject; the markers tell
	out, err := os.ReadFile(filepath.Join(dir, "out.txt"))
	if err != nil {
		t.Fatal(err)
	}
	got, last := make([][]string, len(lines)), -1
	for _, l := range strings.Split(string(out), "\n") {
		if n, err := strconv.Atoi(strings.TrimPrefix(l, "#")); err == nil && strings.HasPrefix(l, "#") {
			last = n
		} else if l != "" && last >= 0 {
			got[last] = append(got[last], strings.Split(strings.TrimSuffix(l, "\x01"), "\x01")...)
		}
	}
	if last != len(lines)-1 {
		t.Fatalf("%v stopped after line %d of %d: %q", shell, last, len(lines), lines[max(last, 0)])
	}
	return got
}

var (
	ghFilterLongFlag  = regexp.MustCompile(`^--jq(=(.*))?$`)
	ghFilterShortFlag = regexp.MustCompile(`^-[a-zA-Z]*?q(=?)(.*)$`)
)

// ghFilterValues is what gh takes as filters from the arguments, worked out here
// from what a real shell passed and from nothing in the code under test.
func ghFilterValues(argv []string) []string {
	var vals []string
	for i, a := range argv {
		next := func() {
			if i+1 < len(argv) {
				vals = append(vals, argv[i+1])
			}
		}
		if m := ghFilterLongFlag.FindStringSubmatch(a); m != nil {
			if m[1] == "" {
				next()
			} else {
				vals = append(vals, m[2])
			}
		} else if m := ghFilterShortFlag.FindStringSubmatch(a); m != nil {
			if m[1] == "" && m[2] == "" {
				next()
			} else {
				vals = append(vals, m[2])
			}
		}
	}
	return vals
}

type ghFilterRow struct {
	line   string         // the command, with %s for each word
	words  []string       // the generated words
	reads  *regexp.Regexp // a filter matching this must be refused
	clears *regexp.Regexp // with every filter matching this, the line must be allowed
	alone  bool           // the shell may not parse every line
}

func ghFilterRows() []ghFilterRow {
	readsX := regexp.MustCompile(`^env\.X$`)
	reads := regexp.MustCompile(`^env$`)
	readsJoin := regexp.MustCompile(`^("a"|a),env$`)
	clears := regexp.MustCompile(`^\.a$`)
	var rows []ghFilterRow
	add := func(line string, words []string, r, c *regexp.Regexp, alone bool) {
		rows = append(rows, ghFilterRow{line, words, r, c, alone})
	}
	for _, f := range []string{"--jq", "-q", "-iq"} {
		add("gh api x %s env.X", ghFilterMixes(f, 3), readsX, nil, false)
		add("gh api x %s .a", ghFilterMixes(f, 2), nil, clears, false)
	}
	for _, f := range []string{"--jq=", "-q", "-iq="} {
		add("gh api x %s", ghFilterMixes(f+"env", 2), reads, nil, false)
		add("gh api x %s", ghFilterMixes(f+".a", 2), nil, clears, false)
	}
	// A value in the quoting ghFilterMixes places, after a flag in a sample of the
	// closed quotings.
	var forms []string
	for _, f := range []string{"--jq", "-q"} {
		for i, m := range ghFilterQuoted(f) {
			if i%5 == 0 {
				forms = append(forms, m)
			}
		}
	}
	for _, f := range forms {
		add("gh api x "+f+" %s", ghFilterMixes("env.X", 2), readsX, nil, false)
		add("gh api x "+f+" %s", ghFilterMixes(`"a",env`, 2), readsJoin, nil, false)
	}
	add("gh api x --jq %s", ghFilterMixes(".a", 3), nil, clears, false)
	add("gh api x --jq=%s", ghFilterMixes(".a", 3), nil, clears, false)
	// Every word over quoting bytes and a flag's or a name's bytes.
	add("gh api x %s env.X", ghFilterWords(`'"\-q`, 6), readsX, nil, false)
	add("gh api x %s .a", ghFilterWords(`'"\-q`, 5), nil, clears, false)
	add("gh api x --jq %s", ghFilterWords(`'"env`, 6), reads, nil, false)
	add("gh api x --jq=%s", ghFilterWords(`'"\env`, 5), reads, nil, false)
	// Every word over brace bytes: an expansion must not produce a flag or value unread.
	add("gh api x %s env.X", ghFilterWords("{,}-q", 4), readsX, nil, true)
	add("gh api x -%s env.X", ghFilterWords("{,}q", 4), readsX, nil, true)
	add("gh api x --jq e%s", ghFilterWords("{,}nv", 4), reads, nil, true)
	add("gh api x --jq=e%s", ghFilterWords("{,}nv", 4), reads, nil, true)
	add("gh api x -qe%s", ghFilterWords("{,}nv", 4), reads, nil, true)
	// Redirections between a flag and its value.
	redir := ghFilterRedirections()
	for _, f := range append(forms[:2:2], "--jq", "-q") {
		add("gh api x "+f+" %s env.X", redir, readsX, nil, true)
		add("gh api x "+f+" %s .a", redir, nil, clears, true)
	}
	// A word the shell removes entirely leaves the flag waiting for its value.
	gone := []string{"$u", "${u}", "$u$v", "$(true)", "`true`"}
	add("gh api x --jq %s env.X", gone, readsX, nil, false)
	add("gh api x -q %s env.X", gone, readsX, nil, false)
	// Nested in a string another shell reads.
	nest := func(cmd string) string { return "eval '" + strings.ReplaceAll(cmd, "'", `'\''`) + "'" }
	var nested []string
	for _, f := range []string{"--jq", "-q"} {
		for _, q := range []string{"%s", "'%s'", `"%s"`} {
			c := "gh api x " + f + " " + fmt.Sprintf(q, "env.X")
			nested = append(nested, nest(c), nest(nest(c)), nest(nest(nest(c))), `eval "`+strings.ReplaceAll(c, `"`, `\"`)+`"`)
		}
	}
	add("%s", nested, readsX, nil, false)
	return rows
}

// regression: #5078
func TestGhFilterWordsLikeShell(t *testing.T) {
	// A real shell reads each generated line, and what it passes alone says whether
	// a filter reached gh; the check must agree. A row with reads set may also refuse
	// lines that pass something else, an accepted false refusal, so only rows with
	// clears set assert allowed.
	shells := [][]string{{"sh"}, {"dash"}, {"bash"}, {"zsh", "-f"}, {"zsh", "-f", "-o", "rcquotes"}}
	for _, shell := range shells {
		t.Run(strings.Join(shell, " "), func(t *testing.T) {
			for _, row := range ghFilterRows() {
				var lines []string
				for _, w := range row.words {
					if ghFilterClosed(w) {
						lines = append(lines, fmt.Sprintf(row.line, w))
					}
				}
				refused, cleared := 0, 0
				for i, argv := range ghFilterShellArgv(t, shell, lines, row.alone) {
					got := ghFilterReads(lines[i])
					vals := ghFilterValues(argv)
					matched, harmless := false, row.clears != nil
					for _, v := range vals {
						matched = matched || row.reads != nil && row.reads.MatchString(v)
						harmless = harmless && row.clears.MatchString(v)
					}
					switch {
					case matched:
						refused++
						if !got {
							t.Errorf("%q: gh gets filter %q from argv %q, and the check allowed it", lines[i], vals, argv)
						}
					case harmless && argv != nil:
						cleared++
						if got {
							t.Errorf("%q: argv %q holds only harmless filters, and the check refused it", lines[i], argv)
						}
					}
				}
				if row.reads != nil && refused == 0 || row.clears != nil && cleared == 0 {
					t.Errorf("%q: %d refused, %d cleared, so this row proves nothing", row.line, refused, cleared)
				}
			}
		})
	}
}

// regression: #5078
func TestGhFilterBracesFailClosed(t *testing.T) {
	// The expansion is not worked out: these rows' words that could produce the flag,
	// and their filter values holding one, are refused whatever they expand to.
	long, short := "--jq", "-q"
	for _, line := range []string{
		"gh api x {" + long + ",.a}",
		"gh api x {" + short + ",.a}",
		"gh api x -{p..r} .a",
		"gh api x {-,}" + short[1:] + " .a",
		"gh api x " + long + " {.a,.b}",
		"gh api x " + long + "={.a,.b}",
		"gh api x " + short + " .{a..c}",
		"gh api x " + short + "{.a,.b}",
	} {
		if !ghFilterClassified(line) {
			t.Errorf("%q: a brace expansion in a flag or value must be refused", line)
		}
	}
}

package main

import (
	"os"
	"strings"
	"testing"
)

// actpin_test.go — the driver-token pin rule (actpin.go): every gh step an Act
// block runs stands behind a pin of the driver's token at the top of the act
// function. Each failing row is a block that runs a gh step on whatever
// GH_TOKEN, GITHUB_TOKEN or active account the driver's shell holds.

const (
	pinLine    = `  GH_TOKEN="$(gh auth token --user "<driver-login>")" || exit 1`
	exportLine = `  export GH_TOKEN; unset GITHUB_TOKEN`
)

// pinBlock builds an act block whose function body is the dry-run line then body.
func pinBlock(body ...string) []string {
	lines := []string{
		actBlockGuardLine,
		"# 0. the act as one function. Pasting this only prints. Type driver_act_1 live to act.",
		"driver_act_1() (",
		`  DRY_RUN=1; [ "${1-}" = live ] && DRY_RUN=0`,
		`  run() { if [ "$DRY_RUN" = 1 ]; then echo "would run: $*"; else "$@"; fi; }`,
	}
	lines = append(lines, body...)
	return append(lines, ")", "driver_act_1")
}

func TestActPinClean(t *testing.T) {
	for name, body := range map[string][]string{
		"pin then run gh": {pinLine, exportLine, `  run gh issue edit 1 -R o/r --remove-label x`},
		"bare exit":       {`  GH_TOKEN="$(gh auth token --user "<driver-login>")" || exit`, exportLine, `  run gh issue edit 1 -R o/r`},
		"exit group": {`  GH_TOKEN="$(gh auth token --user "<driver-login>")" || { echo "no driver token; nothing changed" >&2; exit 1; }`,
			exportLine, `  run gh issue edit 1 -R o/r`},
		"resolved login": {`  GH_TOKEN="$(gh auth token --user octocat)" || exit 1`, exportLine, `  run gh issue edit 1 -R o/r`},
		"separate lines": {pinLine, `  export GH_TOKEN`, `  unset GITHUB_TOKEN`, `  run gh issue edit 1 -R o/r`},
		"pipe and if": {pinLine, exportLine,
			`  if [ "$DRY_RUN" = 1 ]; then echo "would run: gh secret set X -R o/r"`,
			`  else printf '%s' "$V" | gh secret set X -R o/r; fi`},
		"echo only, no gh step":     {`  echo "would run: gh issue edit 1 -R o/r"`},
		"login probe is not a step": {pinLine, exportLine, `  run echo done`},
	} {
		blocks, issues := lintOne(t, "sh", pinBlock(body...))
		if blocks != 1 || len(issues) != 0 {
			t.Errorf("%s: blocks=%d issues=%+v, want 1 block and no issue", name, blocks, issues)
		}
	}
}

func TestActPinFlagsBareGh(t *testing.T) {
	for name, c := range map[string]struct {
		body []string
		want string
	}{
		"no pin at all":  {[]string{`  run gh issue edit 1 -R o/r`}, "no driver-token pin"},
		"bare gh":        {[]string{`  gh issue edit 1 -R o/r`}, "no driver-token pin"},
		"pipe to gh":     {[]string{`  printf x | gh secret set X -R o/r`}, "no driver-token pin"},
		"gh after and":   {[]string{`  true && gh issue edit 1 -R o/r`}, "no driver-token pin"},
		"quoted gh":      {[]string{`  "gh" issue edit 1 -R o/r`}, "no driver-token pin"},
		"escaped gh":     {[]string{`  \gh issue edit 1 -R o/r`}, "no driver-token pin"},
		"command gh":     {[]string{`  command gh issue edit 1 -R o/r`}, "no driver-token pin"},
		"gh path":        {[]string{`  /usr/local/bin/gh issue edit 1 -R o/r`}, "no driver-token pin"},
		"in if cond":     {[]string{`  if gh issue view 1 -R o/r; then echo y; fi`}, "no driver-token pin"},
		"in command sub": {[]string{`  X=$(gh issue view 1 -R o/r)`}, "no driver-token pin"},
		"pin fails open": {[]string{`  GH_TOKEN="$(gh auth token --user "<driver-login>")" || true`, exportLine,
			`  run gh issue edit 1 -R o/r`}, "no driver-token pin"},
		"pin without exit": {[]string{`  GH_TOKEN="$(gh auth token --user "<driver-login>")"`, exportLine,
			`  run gh issue edit 1 -R o/r`}, "no driver-token pin"},
		"pin group lacks exit": {[]string{`  GH_TOKEN="$(gh auth token --user "<driver-login>")" || { echo no; }`, exportLine,
			`  run gh issue edit 1 -R o/r`}, "no driver-token pin"},
		"pin in if": {[]string{`  if [ "$DRY_RUN" = 0 ]; then GH_TOKEN="$(gh auth token --user "<driver-login>")" || exit 1; fi`,
			exportLine, `  run gh issue edit 1 -R o/r`}, "no driver-token pin"},
		"pin in subshell": {[]string{`  ( GH_TOKEN="$(gh auth token --user "<driver-login>")" || exit 1 )`,
			exportLine, `  run gh issue edit 1 -R o/r`}, "no driver-token pin"},
		"pin after gh": {[]string{`  run gh issue view 1 -R o/r`, pinLine, exportLine}, "before the driver token is pinned"},
		"export combined": {[]string{`  export GH_TOKEN="$(gh auth token --user "<driver-login>")" || exit 1`, `  unset GITHUB_TOKEN`,
			`  run gh issue edit 1 -R o/r`}, "no driver-token pin"},
		"no export": {[]string{pinLine, `  unset GITHUB_TOKEN`, `  run gh issue edit 1 -R o/r`}, "separate `export GH_TOKEN`"},
		"no unset":  {[]string{pinLine, `  export GH_TOKEN`, `  run gh issue edit 1 -R o/r`}, "`unset GITHUB_TOKEN`"},
		"export in if": {[]string{pinLine, `  if [ "$DRY_RUN" = 0 ]; then export GH_TOKEN; fi`, `  unset GITHUB_TOKEN`,
			`  run gh issue edit 1 -R o/r`}, "separate `export GH_TOKEN`"},
		"inline swap": {[]string{pinLine, exportLine, `  GH_TOKEN="$other" gh issue edit 1 -R o/r`}, "assigns GH_TOKEN after"},
		"reassign":    {[]string{pinLine, exportLine, `  GH_TOKEN="$other"`, `  run gh issue edit 1 -R o/r`}, "assigns GH_TOKEN after"},
		"reset github token": {[]string{pinLine, exportLine, `  GITHUB_TOKEN="$other"`, `  run gh issue edit 1 -R o/r`},
			"assigns GITHUB_TOKEN after"},
		"unset pin": {[]string{pinLine, exportLine, `  unset GH_TOKEN`, `  run gh issue edit 1 -R o/r`}, "unsets GH_TOKEN after"},
	} {
		_, issues := lintOne(t, "sh", pinBlock(c.body...))
		if len(issues) == 0 {
			t.Errorf("%s: no issue, want one containing %q", name, c.want)
			continue
		}
		found := false
		for _, is := range issues {
			if strings.Contains(is.Msg, c.want) {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: issues=%+v, want one containing %q", name, issues, c.want)
		}
	}
}

// TestActPinFlagsStepOutsideFunction — a gh step after the function runs with no
// pin, wherever the function pinned its own steps.
func TestActPinFlagsStepOutsideFunction(t *testing.T) {
	lines := pinBlock(pinLine, exportLine, `  run gh issue edit 1 -R o/r`)
	lines = append(lines, "gh issue close 1 -R o/r")
	_, issues := lintOne(t, "sh", lines)
	if len(issues) != 1 || !strings.Contains(issues[0].Msg, "outside the act function") {
		t.Fatalf("issues=%+v, want one outside-the-function finding", issues)
	}
}

// TestActPinReportsGhLine — the finding stands on the first gh step's line.
func TestActPinReportsGhLine(t *testing.T) {
	lines := pinBlock(`  echo '1. edit.'`, `  run gh issue edit 1 -R o/r`)
	_, issues := lintOne(t, "sh", lines)
	// six header lines, then the block: the gh step is block line index 6.
	wantOneAt(t, issues, 7+6, "no driver-token pin")
}

// shapeRow is one block body and the finding it must draw.
type shapeRow struct {
	body []string
	want string
}

func wantFinding(t *testing.T, name string, rows map[string]shapeRow) {
	t.Helper()
	for n, c := range rows {
		_, issues := lintOne(t, "sh", pinBlock(c.body...))
		found := false
		for _, is := range issues {
			if strings.Contains(is.Msg, c.want) {
				found = true
			}
		}
		if !found {
			t.Errorf("%s/%s: issues=%+v, want one containing %q", name, n, issues, c.want)
		}
	}
}

const (
	editLine = `  run gh issue edit 1 -R o/r`
	pinCmd   = `GH_TOKEN="$(gh auth token --user "<driver-login>")" || exit 1`
)

// TestActPinFlagsDeclared — a declaring builtin carries the token variable
// after the pin as surely as a bare assignment does.
func TestActPinFlagsDeclared(t *testing.T) {
	rows := map[string]shapeRow{}
	for name, line := range map[string]string{
		"export":   `  export GH_TOKEN="$other"`,
		"declare":  `  declare -x GH_TOKEN="$other"`,
		"typeset":  `  typeset -x GH_TOKEN=$other`,
		"local":    `  local GH_TOKEN="$other"`,
		"readonly": `  readonly GH_TOKEN="$other"`,
		"append":   `  GH_TOKEN+="x"`,
		"second":   `  export A=1 GH_TOKEN="$other"`,
	} {
		rows[name] = shapeRow{[]string{pinLine, exportLine, line, editLine}, "assigns GH_TOKEN after"}
	}
	for name, line := range map[string]string{
		"export github":   `  export GITHUB_TOKEN="$other"`,
		"declare github":  `  declare -x GITHUB_TOKEN="$other"`,
		"readonly github": `  readonly GITHUB_TOKEN=$other`,
	} {
		rows[name] = shapeRow{[]string{pinLine, exportLine, line, editLine}, "assigns GITHUB_TOKEN after"}
	}
	for name, line := range map[string]string{
		"export -n":  `  export -n GH_TOKEN`,
		"declare +x": `  declare +x GH_TOKEN`,
		"typeset +x": `  typeset +x GH_TOKEN`,
	} {
		rows[name] = shapeRow{[]string{pinLine, exportLine, line, editLine}, "removes the export"}
	}
	wantFinding(t, "declared", rows)
}

// TestActPinFlagsPinPosition — a pin that is not its own statement does not
// bind: after || or && it may never run, in a pipe it runs in a subshell.
func TestActPinFlagsPinPosition(t *testing.T) {
	wantFinding(t, "position", map[string]shapeRow{
		"after true or": {[]string{`  true || ` + pinCmd, exportLine, editLine}, "own statement"},
		"after and":     {[]string{`  true && ` + pinCmd, exportLine, editLine}, "own statement"},
		"after bang":    {[]string{`  ! ` + pinCmd, exportLine, editLine}, "own statement"},
		"in a pipe":     {[]string{`  echo | ` + pinCmd, exportLine, editLine}, "own statement"},
		"after pipe nl": {[]string{`  echo |`, `  ` + pinCmd, exportLine, editLine}, "own statement"},
		"after or nl":   {[]string{`  true ||`, `  ` + pinCmd, exportLine, editLine}, "own statement"},
		"under time":    {[]string{`  time ` + pinCmd, exportLine, editLine}, "own statement"},
		"under env":     {[]string{`  env ` + pinCmd, exportLine, editLine}, "own statement"},
		"with a tail":   {[]string{`  ` + pinCmd + ` | cat`, exportLine, editLine}, "no driver-token pin"},
	})
}

// TestActPinFlagsWrappedGh — a wrapper's own options do not hide the gh step.
func TestActPinFlagsWrappedGh(t *testing.T) {
	rows := map[string]shapeRow{}
	for name, line := range map[string]string{
		"env -i":         `  env -i gh issue edit 1 -R o/r`,
		"env assign":     `  env FOO=1 gh issue edit 1 -R o/r`,
		"env -u":         `  env -u FOO gh issue edit 1 -R o/r`,
		"command -p":     `  command -p gh issue edit 1 -R o/r`,
		"time -p":        `  time -p gh issue edit 1 -R o/r`,
		"sudo":           `  sudo gh issue edit 1 -R o/r`,
		"sudo -u":        `  sudo -u root gh issue edit 1 -R o/r`,
		"sudo -E":        `  sudo -E gh issue edit 1 -R o/r`,
		"doas":           `  doas gh issue edit 1 -R o/r`,
		"run sudo":       `  run sudo gh issue edit 1 -R o/r`,
		"exec -a":        `  exec -a x gh issue edit 1 -R o/r`,
		"nohup dashdash": `  nohup -- gh issue edit 1 -R o/r`,
	} {
		rows[name] = shapeRow{[]string{line}, "no driver-token pin"}
	}
	wantFinding(t, "wrapped", rows)
}

// TestActPinFlagsEnvClears — env can drop the pinned token for one step.
func TestActPinFlagsEnvClears(t *testing.T) {
	rows := map[string]shapeRow{}
	for name, line := range map[string]string{
		"env -i":          `  env -i gh issue edit 1 -R o/r`,
		"env --ignore":    `  env --ignore-environment gh issue edit 1 -R o/r`,
		"env -u":          `  env -u GH_TOKEN gh issue edit 1 -R o/r`,
		"env -uGH_TOKEN":  `  env -uGH_TOKEN gh issue edit 1 -R o/r`,
		"env --unset eq":  `  env --unset=GH_TOKEN gh issue edit 1 -R o/r`,
		"env --unset sep": `  env --unset GH_TOKEN gh issue edit 1 -R o/r`,
		"sudo":            `  sudo gh issue edit 1 -R o/r`,
		"sudo -E":         `  sudo -E gh issue edit 1 -R o/r`,
		"sudo -u":         `  sudo -u root gh issue edit 1 -R o/r`,
		"doas":            `  doas gh issue edit 1 -R o/r`,
		"run env -i":      `  run env -i gh issue edit 1 -R o/r`,
		"env -i with set": `  env -i FOO=1 gh issue edit 1 -R o/r`,
	} {
		rows[name] = shapeRow{[]string{pinLine, exportLine, line}, "does not reach"}
	}
	wantFinding(t, "envclears", rows)
}

// TestActPinWrappersClean — wrappers that leave the pin alone, and lookups that
// are not a gh step, draw nothing.
func TestActPinWrappersClean(t *testing.T) {
	for name, body := range map[string][]string{
		"command -v":      {`  command -v gh >/dev/null || exit 1`},
		"command -V":      {`  command -V gh >/dev/null || exit 1`},
		"env no gh":       {`  env FOO=1 printenv FOO`},
		"pinned env -u":   {pinLine, exportLine, `  env -u FOO gh issue edit 1 -R o/r`},
		"pinned env set":  {pinLine, exportLine, `  env FOO=1 gh issue edit 1 -R o/r`},
		"pinned command":  {pinLine, exportLine, `  command -p gh issue edit 1 -R o/r`},
		"pinned time":     {pinLine, exportLine, `  time -p gh issue edit 1 -R o/r`},
		"pin after nl":    {`  true`, pinLine, exportLine, editLine},
		"pin after semi":  {`  true; ` + pinCmd, exportLine, editLine},
		"export github":   {pinLine, exportLine, `  export GITHUB_TOKEN`, editLine},
		"export other":    {pinLine, exportLine, `  export A=1 B`, editLine},
		"local other":     {pinLine, exportLine, `  local A="$x"`, editLine},
		"dash arg of cmd": {pinLine, exportLine, `  run echo -n done`},
	} {
		blocks, issues := lintOne(t, "sh", pinBlock(body...))
		if blocks != 1 || len(issues) != 0 {
			t.Errorf("%s: blocks=%d issues=%+v, want 1 block and no issue", name, blocks, issues)
		}
	}
}

// TestActPinFlagsLoginShape — the login must be one literal word. Unquoted, the
// placeholder is a redirect and the block fails to parse; an expansion can
// name any login.
func TestActPinFlagsLoginShape(t *testing.T) {
	wantFinding(t, "login", map[string]shapeRow{
		"unquoted placeholder": {[]string{`  GH_TOKEN="$(gh auth token --user <driver-login>)" || exit 1`, exportLine, editLine}, "no driver-token pin"},
		"variable":             {[]string{`  GH_TOKEN="$(gh auth token --user "$LOGIN")" || exit 1`, exportLine, editLine}, "expansion"},
		"bare variable":        {[]string{`  GH_TOKEN="$(gh auth token --user $LOGIN)" || exit 1`, exportLine, editLine}, "expansion"},
		"command sub":          {[]string{`  GH_TOKEN="$(gh auth token --user "$(whoami)")" || exit 1`, exportLine, editLine}, "no driver-token pin"},
		"backtick":             {[]string{"  GH_TOKEN=\"$(gh auth token --user `whoami`)\" || exit 1", exportLine, editLine}, "no driver-token pin"},
		"two words":            {[]string{`  GH_TOKEN="$(gh auth token --user a b)" || exit 1`, exportLine, editLine}, "no driver-token pin"},
		"empty login":          {[]string{`  GH_TOKEN="$(gh auth token --user "")" || exit 1`, exportLine, editLine}, "no driver-token pin"},
		"extra option":         {[]string{`  GH_TOKEN="$(gh auth token --user "<driver-login>" --hostname h)" || exit 1`, exportLine, editLine}, "no driver-token pin"},
	})
}

// TestActPinFlagsBraceBody — a { } body runs in the driver's shell: the pin's
// exit ends that shell, and the exported token outlives the call.
func TestActPinFlagsBraceBody(t *testing.T) {
	lines := []string{
		actBlockGuardLine,
		"# 0. the act as one function. Pasting this only prints. Type driver_act_1 live to act.",
		"driver_act_1() {",
		`  DRY_RUN=1; [ "${1-}" = live ] && DRY_RUN=0`,
		`  run() { if [ "$DRY_RUN" = 1 ]; then echo "would run: $*"; else "$@"; fi; }`,
		pinLine, exportLine, editLine,
		"}",
		"driver_act_1",
	}
	_, issues := lintOne(t, "sh", lines)
	found := false
	for _, is := range issues {
		if strings.Contains(is.Msg, "( ) subshell") {
			found = true
		}
	}
	if !found {
		t.Errorf("issues=%+v, want a finding naming the ( ) subshell body", issues)
	}
}

// TestActPinFlagsSpellings — the same refusals under other spellings: a |&
// pipe, a quoted or escaped builtin name, and env's short options in a cluster
// or abbreviated long. Each draws the finding its plain spelling draws.
func TestActPinFlagsSpellings(t *testing.T) {
	rows := map[string]shapeRow{
		"pin after |&":      {[]string{`  true |& ` + pinCmd, exportLine, editLine}, "own statement"},
		"pin after |& nl":   {[]string{`  true |&`, `  ` + pinCmd, exportLine, editLine}, "own statement"},
		"pin after echo |&": {[]string{`  echo hi |& ` + pinCmd, exportLine, editLine}, "own statement"},
	}
	for name, c := range map[string]struct{ line, want string }{
		"escaped unset":       {`  \unset GH_TOKEN`, "unsets GH_TOKEN after"},
		"single-quoted unset": {`  'unset' GH_TOKEN`, "unsets GH_TOKEN after"},
		"split-quoted unset":  {`  un"set" GH_TOKEN`, "unsets GH_TOKEN after"},
		"escaped unset -v":    {`  \unset -v GH_TOKEN`, "unsets GH_TOKEN after"},
		"builtin unset":       {`  builtin unset GH_TOKEN`, "unsets GH_TOKEN after"},
		"quoted export":       {`  "export" GH_TOKEN="$o"`, "assigns GH_TOKEN after"},
		"escaped export":      {`  \export GH_TOKEN=$o`, "assigns GH_TOKEN after"},
		"escaped declare":     {`  \declare -x GH_TOKEN=$o`, "assigns GH_TOKEN after"},
		"quoted local":        {`  "local" GITHUB_TOKEN=$o`, "assigns GITHUB_TOKEN after"},
		"escaped readonly":    {`  \readonly GH_TOKEN=$o`, "assigns GH_TOKEN after"},
		"escaped export -n":   {`  \export -n GH_TOKEN`, "removes the export"},
		"quoted export -n":    {`  "export" -n GH_TOKEN`, "removes the export"},
		"escaped declare +x":  {`  \declare +x GH_TOKEN`, "removes the export"},
		"quoted typeset +x":   {`  'typeset' +x GH_TOKEN`, "removes the export"},
	} {
		rows[name] = shapeRow{[]string{pinLine, exportLine, c.line, editLine}, c.want}
	}
	for name, line := range map[string]string{
		"env -vi":             `  env -vi gh issue edit 1 -R o/r`,
		"env -iv":             `  env -iv gh issue edit 1 -R o/r`,
		"env -0i":             `  env -0i gh issue edit 1 -R o/r`,
		"env -i0v":            `  env -i0v gh issue edit 1 -R o/r`,
		"env -vu name":        `  env -vu GH_TOKEN gh issue edit 1 -R o/r`,
		"env -vuGH_TOKEN":     `  env -vuGH_TOKEN gh issue edit 1 -R o/r`,
		"env -v -u":           `  env -v -u GH_TOKEN gh issue edit 1 -R o/r`,
		"env --ignore-env":    `  env --ignore-env gh issue edit 1 -R o/r`,
		"env --ignore-envir":  `  env --ignore-envir gh issue edit 1 -R o/r`,
		"env -vC dir -i":      `  env -vC /tmp -i gh issue edit 1 -R o/r`,
		"run env -vi":         `  run env -vi gh issue edit 1 -R o/r`,
		"env -vi with assign": `  env -vi FOO=1 gh issue edit 1 -R o/r`,
		"env -iu name":        `  env -iu GH_TOKEN gh issue edit 1 -R o/r`,
		"env -viu name":       `  env -viu GH_TOKEN gh issue edit 1 -R o/r`,
		"env -iuGH_TOKEN":     `  env -iuGH_TOKEN gh issue edit 1 -R o/r`,
		"env -iC dir":         `  env -iC /tmp gh issue edit 1 -R o/r`,
		"env -viC dir":        `  env -viC /tmp gh issue edit 1 -R o/r`,
		"env -iC/tmp":         `  env -iC/tmp gh issue edit 1 -R o/r`,
		"env -iaxu":           `  env -iaxu gh issue edit 1 -R o/r`,
		"env -iaC":            `  env -iaC gh issue edit 1 -R o/r`,
		"env -iP/menu":        `  env -iP/menu gh issue edit 1 -R o/r`,
		"env -iPmenu":         `  env -iPmenu gh issue edit 1 -R o/r`,
		"env -iS":             `  env -iS gh issue edit 1 -R o/r`,
		"env -0iu name":       `  env -0iu GH_TOKEN gh issue edit 1 -R o/r`,
	} {
		rows[name] = shapeRow{[]string{pinLine, exportLine, line}, "does not reach"}
	}
	for name, line := range map[string]string{
		"no pin env -iC dir":  `  env -iC /tmp gh issue edit 1 -R o/r`,
		"no pin env -iu name": `  env -iu FOO gh issue edit 1 -R o/r`,
		"no pin env -viu":     `  env -viu FOO gh issue edit 1 -R o/r`,
		"no pin env -iaxu":    `  env -iaxu gh issue edit 1 -R o/r`,
		"no pin env -iPmenu":  `  env -iPmenu gh issue edit 1 -R o/r`,
	} {
		rows[name] = shapeRow{[]string{line}, "no driver-token pin"}
	}
	wantFinding(t, "spellings", rows)
}

// TestActPinSpellingsClean — spellings that leave the pinned token alone draw
// nothing, so the cluster and quote handling does not over-refuse.
func TestActPinSpellingsClean(t *testing.T) {
	for name, body := range map[string][]string{
		"env -v":               {pinLine, exportLine, `  env -v gh issue edit 1 -R o/r`},
		"env -vu other":        {pinLine, exportLine, `  env -vu FOO gh issue edit 1 -R o/r`},
		"env -vuFOO":           {pinLine, exportLine, `  env -vuFOO gh issue edit 1 -R o/r`},
		"env -vC dir":          {pinLine, exportLine, `  env -vC /tmp gh issue edit 1 -R o/r`},
		"env -Ci dir":          {pinLine, exportLine, `  env -Ci gh issue edit 1 -R o/r`},
		"env -ai":              {pinLine, exportLine, `  env -ai gh issue edit 1 -R o/r`},
		"env --chdir":          {pinLine, exportLine, `  env --chdir /tmp gh issue edit 1 -R o/r`},
		"env --ignore-x":       {pinLine, exportLine, `  env --block-signal gh issue edit 1 -R o/r`},
		"escaped unset other":  {pinLine, exportLine, `  \unset FOO`, editLine},
		"quoted export other":  {pinLine, exportLine, `  "export" FOO=1`, editLine},
		"escaped export plain": {pinLine, exportLine, `  \export GITHUB_TOKEN`, editLine},
		"pin after |& text":    {`  echo "a |& b"`, pinLine, exportLine, editLine},
	} {
		blocks, issues := lintOne(t, "sh", pinBlock(body...))
		if blocks != 1 || len(issues) != 0 {
			t.Errorf("%s: blocks=%d issues=%+v, want 1 block and no issue", name, blocks, issues)
		}
	}
}

// actUnmodelledList is the one list of what rule 6 does not model. It must read
// the same at every site that states it, so no site claims more than the check
// does; § stands for a backtick. The PR body carries the same text and is
// checked by the author, outside this test.
const actUnmodelledList = "The lint refuses the shapes it models; the pin inside the function is the control. Not modelled (examples, not exhaustive; a §gh§ step in one can run with no pin, and a token change in one can undo the pin): a §gh§ run through §eval§, §sh -c§, §bash -c§, §xargs§, §nice§, §timeout§, §stdbuf§, §exec -c§ or a wrapper command the lint does not name, or one whose name is quoted, escaped or given by path (§\\env§, §\"sudo\"§, §\\builtin§, §/usr/bin/env -i gh§); a wrapper option that takes a separate argument and is not in the lint's list (§env -S§, say); a token set from a file by §env --env0-from=FILE§; an alias; a command name built from an expansion; a function that wraps §gh§, or one that shadows §export§ or §unset§; a pin inside a heredoc body; a §trap … DEBUG§; a §gh§ step inside a process substitution (§<( )§, §>( )§), after a leading redirect such as §2>&1§, or in a §case§ arm whose pattern opens with §(§; a §case§ pattern §)§ that no §esac§ closes; an §export§ or §unset§ of the token run under §||§, §&&§, §&§ or a pipe; a change of either token variable by §read§, §printf -v§, §for§, §select§, §let§, §mapfile§, §getopts§, §source§, §.§, a nameref, §eval§, zsh §integer§ or §float§, or zsh §${GH_TOKEN::=…}§; a token change by an §export§ option cluster holding §n§ (§-pn§), by a bare §local§, §declare§ or §typeset§ of the token with no value (zsh), or by zsh §unset -m§. The pin governs §gh§ only: a §git push§ uses git's credential helper, and §GH_HOST§ and the enterprise-token variables are left alone, which matters only against a GitHub Enterprise Server host."

// squashSpace drops the Go comment markers and folds every run of whitespace.
func squashSpace(s string) string {
	var b strings.Builder
	for _, l := range strings.Split(s, "\n") {
		l = strings.TrimSpace(l)
		l = strings.TrimSpace(strings.TrimPrefix(l, "//"))
		b.WriteString(l + " ")
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

// TestActPinUnmodelledListIdentical — the not-modelled list is word for word the
// same in the lint source, the README, the skill and the changelog fragment.
func TestActPinUnmodelledListIdentical(t *testing.T) {
	want := squashSpace(strings.ReplaceAll(actUnmodelledList, "§", "`"))
	for _, f := range []struct {
		path     string
		optional bool
	}{
		{"actpin.go", false},
		{"README.md", false},
		{"../../plugins/assay/skills/ask-decision/SKILL.md", false},
		{"../../changelog/wd1005-issue-2224.md", true},
	} {
		b, err := os.ReadFile(f.path)
		if err != nil {
			if f.optional && os.IsNotExist(err) {
				continue // a fragment consumed at release is no longer here
			}
			t.Errorf("%s: %v", f.path, err)
			continue
		}
		if n := strings.Count(squashSpace(string(b)), want); n != 1 {
			t.Errorf("%s states the not-modelled list %d times, want exactly once and word for word", f.path, n)
		}
	}
}

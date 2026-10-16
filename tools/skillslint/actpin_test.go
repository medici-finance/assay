package main

import (
	"strings"
	"testing"
)

// actpin_test.go — the driver-token pin rule (actpin.go): every gh step an Act
// block runs stands behind a pin of the driver's token at the top of the act
// function. Each failing row is a block that runs a gh step on whatever
// GH_TOKEN, GITHUB_TOKEN or active account the driver's shell holds.

const (
	pinLine    = `  GH_TOKEN="$(gh auth token --user <driver-login>)" || exit 1`
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
		"bare exit":       {`  GH_TOKEN="$(gh auth token --user <driver-login>)" || exit`, exportLine, `  run gh issue edit 1 -R o/r`},
		"exit group": {`  GH_TOKEN="$(gh auth token --user <driver-login>)" || { echo "no driver token; nothing changed" >&2; exit 1; }`,
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
		"pin fails open": {[]string{`  GH_TOKEN="$(gh auth token --user <driver-login>)" || true`, exportLine,
			`  run gh issue edit 1 -R o/r`}, "no driver-token pin"},
		"pin without exit": {[]string{`  GH_TOKEN="$(gh auth token --user <driver-login>)"`, exportLine,
			`  run gh issue edit 1 -R o/r`}, "no driver-token pin"},
		"pin group lacks exit": {[]string{`  GH_TOKEN="$(gh auth token --user <driver-login>)" || { echo no; }`, exportLine,
			`  run gh issue edit 1 -R o/r`}, "no driver-token pin"},
		"pin in if": {[]string{`  if [ "$DRY_RUN" = 0 ]; then GH_TOKEN="$(gh auth token --user <driver-login>)" || exit 1; fi`,
			exportLine, `  run gh issue edit 1 -R o/r`}, "no driver-token pin"},
		"pin in subshell": {[]string{`  ( GH_TOKEN="$(gh auth token --user <driver-login>)" || exit 1 )`,
			exportLine, `  run gh issue edit 1 -R o/r`}, "no driver-token pin"},
		"pin after gh": {[]string{`  run gh issue view 1 -R o/r`, pinLine, exportLine}, "before the driver token is pinned"},
		"export combined": {[]string{`  export GH_TOKEN="$(gh auth token --user <driver-login>)" || exit 1`, `  unset GITHUB_TOKEN`,
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
	pinCmd   = `GH_TOKEN="$(gh auth token --user <driver-login>)" || exit 1`
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

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

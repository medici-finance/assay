package main

// principal.go — statusgen's READ-ONLY mirror of the on-behalf-of principal (multi-
// principal/01). statusgen and tools/desk are separate Go modules that deliberately
// share no code (the documented-duplicate pattern this repo already uses for
// rosterconfig.go); this file is that duplication for the ONE piece of the on-behalf-of
// mechanism statusgen needs — resolving the roster's bless login and checking a login
// against the human map — never the full writer-side resolver (statusgen never writes a
// trailer; it only RENDERS one into a witness row it is already producing, and READS one
// back for the attribution lint). See tools/desk/internal/deskkit/principal.go for the
// writer-side original and the fuller design rationale.

import "strings"

// onBehalfOfPrincipalLogin resolves the roster's bless login for a witness row's
// on-behalf-of suffix, the SAME way the writer side does: EffectiveConfig-equivalent
// (scanEffectiveConfig, config-home file for a write-classed tool, never an env var for
// the login itself), validated against the human map. Returns ok=false when no principal
// can be resolved — the caller (verifyrun's row renderer) treats that as "say nothing"
// rather than refuse the whole witness write; the hard refusal lives on the writer side
// (deskpost/deskreply/deskpr/deskevidence/deskfile/deskflip), not here.
func onBehalfOfPrincipalLogin() (login string, ok bool) {
	c := scanEffectiveConfig()
	l := strings.ToLower(strings.TrimSpace(c.Bless.Login))
	if !c.Configured() || l == "" {
		return "", false
	}
	if _, present := c.Humans[l]; !present {
		return "", false
	}
	return l, true
}

// isHumanRunnerToken reports whether a witness/Evidence runner token is ALREADY a human
// stamp (`human:<name>`) — see humanRunnerName (attribution.go). A human-run row needs no
// on-behalf-of suffix: the runner already names the acting human directly.
func isHumanRunnerToken(token string) bool {
	_, ok := humanRunnerName(token)
	return ok
}

// isAppRunnerToken reports whether a witness/Evidence runner token names a shared App/bot
// identity — the GitHub-rendered `<slug>[bot]` convention executingRunner itself uses for
// a CI-actor runner (verifyrun.go's file comment: "GitHub Actions: GITHUB_ACTOR ...
// `<slug>[bot]`"). It is a LOWER BOUND, deliberately: a runner token this does not
// recognise as an App is simply not suffixed or lint-checked, never guessed either way.
func isAppRunnerToken(token string) bool {
	t := strings.TrimSpace(token)
	// A full Evidence Runner CELL (as opposed to a bare witness Runner token) carries
	// trailing "@ <tree>" / "(...)" material after the identity — take only the leading
	// identifier before the first space or "(" before checking the "[bot]" suffix, so
	// "assay-verifier-app[bot] @ abc1234 (on-behalf-of human:x)" is recognised exactly
	// like the bare "assay-verifier-app[bot]" witness token is.
	if i := strings.IndexAny(t, " ("); i >= 0 {
		t = t[:i]
	}
	return strings.HasSuffix(t, "[bot]")
}

// onBehalfOfSuffix renders the on-behalf-of annotation for a witness Runner cell —
// `on-behalf-of human:<login>` — when token names an App/bot identity and a principal
// resolves; "" (no annotation) for a human runner, or when no principal can be resolved
// (statusgen never blocks a witness write over this — the write-side refusal is the
// enforcement point).
func onBehalfOfSuffix(token string) string {
	if isHumanRunnerToken(token) || !isAppRunnerToken(token) {
		return ""
	}
	login, ok := onBehalfOfPrincipalLogin()
	if !ok {
		return ""
	}
	return "on-behalf-of human:" + login
}

// onBehalfOfPrincipalOf extracts the login named by a Runner/Evidence cell's
// `on-behalf-of human:<login>` annotation, wherever it appears in the cell text (it is
// rendered inside a trailing parenthetical, alongside RunnerSource — see verifyrun.go's
// row()). ok=false when the cell carries no such annotation at all.
//
// Takes the LAST occurrence, not the first, if the cell somehow carries more than one
// (defensive: verifyrun.go's row() only ever renders one, and the write-side
// AppendOnBehalfOf strips any caller-planted line before appending its own — see that
// function's comment — so this cell should never legitimately carry two; taking the
// last is the reading that cannot be shadowed by an earlier, spoofed occurrence).
func onBehalfOfPrincipalOf(cell string) (login string, ok bool) {
	const marker = "on-behalf-of human:"
	idx := strings.LastIndex(strings.ToLower(cell), marker)
	if idx < 0 {
		return "", false
	}
	rest := cell[idx+len(marker):]
	end := 0
	for end < len(rest) {
		c := rest[end]
		if c == ')' || c == ' ' || c == '\t' || c == '\n' || c == '|' {
			break
		}
		end++
	}
	login = strings.ToLower(strings.TrimSpace(rest[:end]))
	if login == "" {
		return "", false
	}
	return login, true
}

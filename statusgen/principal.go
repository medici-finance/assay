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
//
// The witness row lands in a brief's `## Evidence` table — a file in the repository — so
// it follows the writer side's visibility split: the neutral name on any target the roster
// does not state is `:private`, the login only on one it does (onBehalfOfSubject).

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

// onBehalfOfPublicForm reports whether an annotation written into repo must name the
// human's NEUTRAL form rather than the roster login — the witness-side mirror of
// deskkit's publicFormRequired, and stated in the same fail-closed direction: everything
// EXCEPT a repo the roster explicitly configures `:private` takes the public form. A repo
// the roster does not carry, one configured with no visibility token, and an empty repo
// (the witness could not tell which repo its brief lives in) all answer true. A wrong
// `true` costs audit precision (the name still maps back to the login through the
// roster); a wrong `false` puts a login into a world-readable Evidence table for good.
//
// KEEP IN SYNC with deskkit's publicFormRequired.
func onBehalfOfPublicForm(repo string) bool {
	policy, ok := scanEffectiveConfig().Repos[strings.TrimSpace(repo)]
	return !ok || !strings.HasSuffix(policy, ":private")
}

// humanNeutralName returns the roster's NEUTRAL form of login — the key under which
// ASSAY_HUMAN_LOGIN_MAP's `name:login` entry maps to it — and whether one is configured.
// Deterministic when a roster maps two names onto one login: the lexicographically first
// wins, so the same roster always renders the same annotation.
//
// KEEP IN SYNC with deskkit's HumanNeutralName.
func humanNeutralName(login string) (string, bool) {
	login = strings.ToLower(strings.TrimSpace(login))
	if login == "" {
		return "", false
	}
	best := ""
	for name, mapped := range scanEffectiveConfig().HumanLogins {
		if mapped != login {
			continue
		}
		if best == "" || name < best {
			best = name
		}
	}
	return best, best != ""
}

// onBehalfOfSubject resolves the form of the principal a witness annotation written into
// repo names: the login on a known-private repo, the neutral name everywhere else.
// ok=false when no principal resolves, OR when the target needs the neutral form and the
// roster carries none — the witness then says nothing rather than fall back to the login
// (the writer side refuses in that case; statusgen never blocks a witness write, so
// "say nothing" is its equivalent, and the attribution lint then reports the unannotated
// row exactly as it reports any other).
func onBehalfOfSubject(repo string) (string, bool) {
	login, ok := onBehalfOfPrincipalLogin()
	if !ok {
		return "", false
	}
	if !onBehalfOfPublicForm(repo) {
		return login, true
	}
	return humanNeutralName(login)
}

// onBehalfOfSuffix renders the on-behalf-of annotation for a witness Runner cell —
// `on-behalf-of human:<subject>` — when token names an App/bot identity and a principal
// resolves; "" (no annotation) for a human runner, or when no principal can be resolved
// (statusgen never blocks a witness write over this — the write-side refusal is the
// enforcement point).
//
// repo is the repo the witness row is written INTO (the brief's own repo); it decides
// which form of the human the annotation names (onBehalfOfSubject). It is a mandatory
// argument for the same reason it is on the writer side: no renderer may choose a form
// without saying where the text lands.
func onBehalfOfSuffix(token, repo string) string {
	if isHumanRunnerToken(token) || !isAppRunnerToken(token) {
		return ""
	}
	subject, ok := onBehalfOfSubject(repo)
	if !ok {
		return ""
	}
	return "on-behalf-of human:" + subject
}

// onBehalfOfPrincipalRecognised reports whether principal — the value an on-behalf-of
// annotation or trailer names — is a human this roster recognises, in EITHER of the two
// forms the writers render: a login in the roster's human set (the known-private form),
// or a neutral name that ASSAY_HUMAN_LOGIN_MAP maps onto such a login (the public form).
// It is not a widening to "any token": a name is accepted only through the map, and only
// when the login it maps to is itself a recognised human.
func onBehalfOfPrincipalRecognised(cfg scanConfig, principal string) bool {
	p := strings.ToLower(strings.TrimSpace(principal))
	if p == "" {
		return false
	}
	if _, ok := cfg.Humans[p]; ok {
		return true
	}
	if login, ok := cfg.HumanLogins[p]; ok {
		_, human := cfg.Humans[login]
		return human
	}
	return false
}

// onBehalfOfPrincipalOf extracts the principal named by a Runner/Evidence cell's
// `on-behalf-of human:<principal>` annotation — a login on a known-private repo, the
// roster's neutral name elsewhere (onBehalfOfSubject); onBehalfOfPrincipalRecognised
// accepts either — wherever it appears in the cell text (it is
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

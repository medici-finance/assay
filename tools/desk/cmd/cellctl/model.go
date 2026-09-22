package main

import (
	"strings"
)

// isOpusPin is true when a resolved model value is the `opus` alias or a `claude-opus…` id,
// case-insensitive (an operator could type Opus, OPUS, or a full id in any case). Any other
// value — including another full id or a different alias — is not an Opus pin.
func isOpusPin(m string) bool {
	l := strings.ToLower(m)
	return l == "opus" || strings.HasPrefix(l, "claude-opus")
}

// refuseOpusForTheDesk: the-desk is the one window that spends its tier on judgment, synthesis
// and arbitration across streams, so the methodology's model-tier rule pins it to the top tier
// available — a downgrade there is a stop condition, not an operator preference. `opus` is no
// longer the top tier, so this is called for the-desk ONLY, before it launches, INCLUDING on the
// DRY_RUN path: a dry run is a plan of exactly that launch, so it refuses on the same input.
func refuseOpusForTheDesk(m string) {
	if !isOpusPin(m) {
		return
	}
	die("the-desk runs on the top tier; an Opus pin is refused for the coordinator — resolved DESK_MODEL_the_desk=%s (from DESK_MODEL_the_desk or DESK_MODEL_DEFAULT); set DESK_MODEL_the_desk=fable (or another non-Opus id) in cell.env", m)
}

// roleTier is which TIER a role resolves at when it falls all the way through to the tier map
// (#986): the-desk spends its window on judgment/synthesis/arbitration across streams, so it is
// the one role pinned to the TOP tier by default; the mechanical loop roles get MID. FAST is not
// assigned to any role automatically; it exists for an operator to pin a role at directly.
func roleTier(role string) string {
	if role == "the-desk" {
		return "top"
	}
	return "mid"
}

// resolvedModel carries the model NAME and the cell.env key (or tier entry) that produced it.
type resolvedModel struct {
	Model string
	Src   string
	OK    bool
}

// resolveRoleModel is the per-harness NAMESPACE + TIER-MAP fallback resolution (#986).
//
// Order: (1) that harness's own per-role pin — DESK_MODEL_<role> on claude, CODEX_MODEL_<role>
// on codex — (2) that harness's own default — DESK_MODEL_DEFAULT / CODEX_MODEL_default — (3) the
// tier map, by this role's tier and the ACTIVE harness's column. On total failure Src names
// every place looked and OK is false.
//
// NEVER consulted for an explicit --model: that value passes through verbatim, bypassing this
// whole chain, on either harness.
func (c *Cell) resolveRoleModel(role, harness string) resolvedModel {
	rvar, dvar := "DESK_MODEL_"+underscore(role), "DESK_MODEL_DEFAULT"
	if harness == "codex" {
		rvar, dvar = "CODEX_MODEL_"+underscore(role), "CODEX_MODEL_default"
	}
	if v := c.Env.Get(rvar); v != "" {
		return resolvedModel{Model: v, Src: rvar, OK: true}
	}
	if v := c.Env.Get(dvar); v != "" {
		return resolvedModel{Model: v, Src: dvar, OK: true}
	}
	tier := roleTier(role)
	tvar := "TIER_MODEL_" + strings.ToUpper(tier) + "_" + strings.ToUpper(harness)
	if v := c.Env.Get(tvar); v != "" {
		return resolvedModel{Model: v, Src: "tier:" + tier + " (" + tvar + ")", OK: true}
	}
	return resolvedModel{Src: "no model resolves for role '" + role + "' harness '" + harness +
		"' — checked " + rvar + ", " + dvar + ", and the '" + tier + "' tier (" + tvar + "), all unset"}
}

func underscore(s string) string { return strings.ReplaceAll(s, "-", "_") }

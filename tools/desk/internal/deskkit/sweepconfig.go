package deskkit

// sweepconfig.go — the HOUSE-supplied half of the disclosure sweep's routed-away
// set.
//
// WHY A CONFIG POINTER AND NOT A HARD-CODED PATH. The S2 disclosure sweep routes
// itself AWAY from a handful of paths whose CONTENT deliberately names the private
// review channel — a private stream's README, for one, explains why the channel is
// withheld, so scanning it would flag the sweep against its own documented reason.
// But naming that stream in the SHIPPED sweep source is itself
// the leak the sweep exists to prevent: a copy of tools/desk published to a public
// repo (the #1316 flip) would then carry a map straight to the withheld stream. So
// the stream PATHS the sweep routes away from are a deployment VALUE the house
// supplies at run time, exactly as the other generic knobs work
// (ASSAY_WRITEGUARD_CALLOUT, ASSAY_CONFIG_HOME): the shipped binary carries the
// MECHANISM (route away, and keep the routed-away list honest), the deployment
// carries the POLICY (which of its own private streams to route).
//
// ONE BINARY, NO FORK. An adopter recreates their own withheld streams through the
// same knob; the public copy names none of ours and needs no exclude/fork. Unset is
// a COMPLETE configuration, not a degraded one — a tree with no withheld streams to
// route (the public copy, where docs/streams is not published at all) simply has an
// empty house-supplied set, and the operational exclusions the sweep still hard-codes
// (generated views, third-party trees) are unaffected.
//
// NAMESPACE. This is a GENERIC methodology knob — the MECHANISM of routing a
// self-scanning sweep away from streams whose content names the channel — so it
// wears the ASSAY_ prefix. The VALUES it carries are deployment-specific stream
// paths and are never compiled in.

import (
	"os"
	"strings"
)

// EnvSweepWithheldStreams names the environment variable carrying the repo-relative
// paths the disclosure sweep routes AWAY, comma-separated:
//
//	ASSAY_SWEEP_WITHHELD_STREAMS=docs/streams/<your-private-stream>,docs/streams/<another>
//
// Each path is a private stream whose content names the withheld channel by
// construction (its own README justifies the withholding), so the sweep must route
// past it rather than flag it — and the path must never be compiled into the shipped
// source. Unset yields the empty set.
const EnvSweepWithheldStreams = "ASSAY_SWEEP_WITHHELD_STREAMS"

// SweepWithheldStreamExclusions returns the house-configured routed-away stream
// paths from EnvSweepWithheldStreams, split on commas and trimmed, with empties
// dropped. Order is preserved; the caller de-dupes against its own base set. A nil
// return (the unset case) is the complete public/adopter configuration.
func SweepWithheldStreamExclusions() []string {
	raw := strings.TrimSpace(os.Getenv(EnvSweepWithheldStreams))
	if raw == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(raw, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

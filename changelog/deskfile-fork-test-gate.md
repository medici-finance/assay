### Added
- `deskfile new` requires a `### Fork test` block on every `needs-decision` filing (the
  workable options, the default, the human-held gate that catches a wrong guess, and the
  search proving the question was not already ruled), refusing a filing with fewer than two
  workable options and naming three `--no-fork` re-routes
  (`brief-contradicts-artifact` | `wrong-repo` | `tool-false-positive`) instead.
- Two workable options plus a gate the driver still holds can file on a NOTICE LANE
  (`desk-decided`, off the driver's queue, with the shared `desk-r3-decision v1` marker)
  rather than `needs-decision`. Admission fails closed: the item must carry a positive R-3
  reversible signal (`deskkit.ReversibleSignals`) AND no one-way signal — a one-way caller
  label (`human-only`, `security`, `gate:human`), a `deskkit.HumanOnlySignals` needle, or a
  `deskkit.OneWayPatterns` match (merge, ready-flip, main push, tag/release, weakening a
  security control, secrets/keys/PII, money, identity/auth, deleting or overwriting data,
  sending or publishing outside, live infrastructure, App permissions, approval authority,
  `gate:human`, direct writes to main, draft-to-ready, required reviews, 2FA/MFA, hooks and
  signatures, the trust gate and roster, owners and roles, archive/transfer/visibility,
  auto-closing driver-owned items, charges, and turning a control off). A one-way term
  outranks a reversible one, and the `ruled-check:` line is read for one-way terms (exempt
  only from the `ruling` needle). The same one-way check refuses `--no-fork` and withholds
  the re-routes from the fewer-than-two-options refusal. The patterns are a floor, not a
  guarantee: review of the filing and the digest's veto window are the layers around them.
  `deskdigest` lists notice-lane items in its desk-decisions section with their veto date,
  and drops a `desk-decided` item from the Queue only when that section lists it.
- `deskfile new` refuses a caller `--label desk-decided` and a caller body already carrying
  the `desk-r3-decision v1` marker (`--force-new` included): only the notice lane writes
  either.

### Changed
- The R-3 human-only and reversible keyword lists moved from `cmd/deskdigest` into
  `internal/deskkit` (`HumanOnlySignals`, `ReversibleSignals`) so `deskfile`'s notice-lane
  gate and `deskdigest`'s classifier consult one definition; `deskdigest` no longer reads
  the tool-written `## Desk-decided` block when classifying.
- The `desk-decided` label is created from one shared spec
  (`deskkit.DeskDecidedLabelColor` / `DeskDecidedLabelDescription`) by both `deskpr` and
  `deskfile`, whichever reaches a repo first.

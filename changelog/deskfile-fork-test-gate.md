### Added
- `deskfile new` requires a `### Fork test` block on every `needs-decision` filing (the
  workable options, the default, the human-held gate that catches a wrong guess, and the
  search proving the question was not already ruled), refusing a filing with fewer than two
  workable options and naming three `--no-fork` re-routes
  (`brief-contradicts-artifact` | `wrong-repo` | `tool-false-positive`) instead.
- Two workable options plus a gate the driver still holds can file on a NOTICE LANE
  (`desk-decided`, off the driver's queue, with the shared `desk-r3-decision v1` marker)
  rather than `needs-decision`. Admission fails closed: the item must carry a positive,
  content-bearing R-3 reversible signal (docs wording, typo, phrasing, a table column — never
  a shape-only `tool default` / `default value` / `flag default` / `rename the`, and never a
  `lint level` / `lint severity` / `notice or error` / `port-or-drop` example either, which is
  always a classification question about some check or job, named or not) AND no one-way
  signal — a one-way caller
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
  the `desk-r3-decision v1` marker in any spelling the digest reads, or broader
  (`--force-new` included): only the notice lane writes either. The marker's reader pattern
  is declared once, in `deskkit`.

### Fixed
- The notice lane's positive reversible-signal test now reads the fork-test block's own
  `subject:` line alone, never the issue title or body prose (security review sec-1688-S1,
  round 4): a title routinely carries more than one clause, and a scan of the whole thing
  admitted on whichever clause happened to carry a reversible needle rather than on what the
  filing was actually about. `subject:` is optional; its absence never admits.
- A `lint level`/`lint severity`/`notice or error`/`port-or-drop` example never admits the
  notice lane on its own any more, named CI check or not (security review sec-1688-S1, round
  5): each of those is, by construction, always a classification question about some check or
  job, so a rule that only refused when the subject named the check by a generic noun
  (check/job/workflow/pipeline, or this codebase's own `<word>-sweep`/`<word> check`
  compounds) still admitted a check named by its own name (`pin-consistency`, `skillslint`,
  `forge-surface`, `build-test`, `govulncheck`, `CodeQL`). The generic-noun check remains as a
  backstop for other reversible needles paired with an explicit check/job mention.
- The declared `subject:` line is now read only when the fork-test section carries exactly
  one such line and it is not `>`-quoted (security review sec-1688-S1, round 5): the section
  runs to the next heading or EOF, so a `>`-quoted line of trailing prose, or a second
  `subject:` line (an incidental one, or a leftover template placeholder), used to override an
  honest first subject because the parser kept only the last line seen. Two or more lines, or
  a lone quoted one, now leave no declared subject — the same fail-closed default as none at
  all.

### Changed
- The R-3 human-only and reversible keyword lists moved from `cmd/deskdigest` into
  `internal/deskkit` (`HumanOnlySignals`, `ReversibleSignals`) so `deskfile`'s notice-lane
  gate and `deskdigest`'s classifier consult one definition; `deskdigest` no longer reads
  the tool-written `## Desk-decided` block when classifying.
- The `desk-decided` label is created from one shared spec
  (`deskkit.DeskDecidedLabelColor` / `DeskDecidedLabelDescription`) by both `deskpr` and
  `deskfile`, whichever reaches a repo first.

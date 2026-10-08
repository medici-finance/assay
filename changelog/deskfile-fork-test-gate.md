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
- The fork-test block is read by a strict, fail-closed grammar (`tools/desk/README.md`):
  fenced code blocks and HTML comments are blanked first, following CommonMark for those two
  constructs (a fence closes only on the same character, at least as many repetitions and
  under 4 columns of indent; an unclosed comment hides everything from its line to the end,
  wherever on the line it opens; a blanked line stays as an empty line, so it never joins what
  came before it to what came after it). The heading must be followed directly by the block,
  and the block is the contiguous run of column-zero `<lowercase-key>: <content>` lines. Prose
  between the heading and the first key line is refused as a malformed block, naming that
  problem. The strip models only fences and comments; a backstop drops the declared subject
  when the raw body has more than one fork-test heading, when a reading of the raw body
  disagrees with the stripped reading, or when the raw body opens a raw-HTML block kind a
  renderer drops whole (processing instruction, declaration, CDATA). One known limit is
  stated: a `subject:` line placed directly under the block's last line, with no blank line,
  joins the block.
- The notice lane reads its reversible signal from the block's own `subject:` line alone,
  never the title or body prose, and only when the bounded block carries exactly one such
  line; none or several admit nothing. A shape-only needle (`lint level`, `lint severity`,
  `notice or error`, `port-or-drop`) never admits and vetoes any content needle beside it,
  in its plural spellings too (`-s`, `-es`, and `-ies` for a needle ending in `y`).
  Needles match on word boundaries, with runs of spaces, underscores, dots, slashes and
  hyphens treated as one space. The CI check/job backstop reads the subject the same way, and
  any non-ASCII character in the subject fails closed.

### Changed
- The R-3 human-only and reversible keyword lists moved from `cmd/deskdigest` into
  `internal/deskkit` (`HumanOnlySignals`, `ReversibleSignals`) so `deskfile`'s notice-lane
  gate and `deskdigest`'s classifier consult one definition; `deskdigest` no longer reads
  the tool-written `## Desk-decided` block when classifying.
- The `desk-decided` label is created from one shared spec
  (`deskkit.DeskDecidedLabelColor` / `DeskDecidedLabelDescription`) by both `deskpr` and
  `deskfile`, whichever reaches a repo first.

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
- The fork-test section itself is now BOUNDED (security review sec-1688-S1, round 6): it used
  to run to the next heading or EOF, so a `subject:`-shaped line anywhere afterward — inside a
  fenced or indented code block, in ordinary prose, or past a Markdown setext heading, none of
  which the old scan treated as ending anything — was still read as the declared subject when
  the block itself declared none. The section now ends at the first heading, fence, or blank
  line encountered after the block's last recognised grammar key line, never at EOF.
- A subject pairing a shape-only needle (`lint level`/`lint severity`/`notice or error`/
  `port-or-drop`/`port or drop`) with an unrelated content-bearing needle (docs wording, a
  typo, …) now refuses instead of admitting through the content needle (correctness re-review
  cor-1688-C7, residual; security review sec-1688-S1, round 6): the shape-only needle is a
  VETO, checked before any content needle is looked for, not merely skipped while scanning for
  one.
- Needle matches (the notice lane's admission scan) now respect word boundaries, so `wording`
  no longer matches inside `rewording`; and a Unicode hyphen/dash look-alike (U+2010, U+2011,
  U+2012, U+2013, U+2014, U+2212) in the subject is normalised to an ASCII hyphen before the
  `ciCheckOrJobRe` backstop and the hyphenated needles run, so a check name typed with a
  "fancy" hyphen still matches (round 6 advisories).
- **The subject-extraction grammar is now strict and fail-closed (round 7), replacing the
  round 1-6 shape-by-shape boundary markers with three rules that need no further enumeration**
  (security review sec-1688-S1, correctness re-review cor-1688-C11/C12; the driver's option-2
  ruling is unchanged — this is the same ruling, implemented once instead of patched six
  times): every fenced code block and every HTML comment is stripped from the whole body BEFORE
  any parsing runs, so nothing inside either — including a `subject:` line hidden inside an
  HTML comment between two of the block's own real key lines, and a `### Fork test` heading
  quoted inside an earlier fenced example — can ever be read as a key line or the heading
  itself; the `### Fork test` heading must be followed DIRECTLY by the block (blank lines are
  fine, any other text before the first key line is now a MALFORMED block, named as such,
  rather than a silently empty one); and the block is the CONTIGUOUS run of key lines
  (`<lowercase-key>: <content>` at column zero, no bullet/quote/indent decoration) starting
  there, ending at the first line that does not match — blank or not — with nothing past that
  line ever read as part of the block. This closes, at once: every prior round's trailing-
  content shape, the plain-trailing-line-after-one-blank-line residual security review
  5332392502 found still open, the fence-arm gap that had no test able to fail
  (cor-1688-C12/sec-1688-S5), the intro-prose-before-the-first-key-line case
  (cor-1688-C11), and the withheld hidden-HTML-comment-subject variant from
  review-notes#169. The dedicated `>`-quote exclusion and per-shape boundary markers
  (fence/heading/blank-run tracking) are retired: a decorated or indented line was never a key
  line under the new grammar, so it can neither start nor extend the block.
- The Unicode hyphen/dash normaliser (round 6) widens to four further look-alikes named in the
  withheld review-notes#169 detail: U+FF0D FULLWIDTH HYPHEN-MINUS, U+FE63 SMALL HYPHEN-MINUS,
  U+00AD SOFT HYPHEN (which renders as no visible character at all), and U+2043 HYPHEN BULLET.

### Changed
- The R-3 human-only and reversible keyword lists moved from `cmd/deskdigest` into
  `internal/deskkit` (`HumanOnlySignals`, `ReversibleSignals`) so `deskfile`'s notice-lane
  gate and `deskdigest`'s classifier consult one definition; `deskdigest` no longer reads
  the tool-written `## Desk-decided` block when classifying.
- The `desk-decided` label is created from one shared spec
  (`deskkit.DeskDecidedLabelColor` / `DeskDecidedLabelDescription`) by both `deskpr` and
  `deskfile`, whichever reaches a repo first.

<!-- GENERATED from plugins/assay/skills/pr-review-desk/references/verdict-format.md by deskinstall --harness cursor. Do not edit. -->
# Verdict mechanics and the verdict-body schema

The desk's verdict-posting interface. `pr-review-desk/SKILL.md` § The reviewer's bar points here;
`deskdispatch --kit review` §12 carries the generic wording the dispatched agent receives. The tool
reference is `tools/desk/README.md` — the public home of the desk tools.

## Posting a verdict

The verdict is a **real GitHub review by the reviewer App, not a text marker**:

```
deskpost review          <owner/repo> <pr> --verdict approve|request-changes --head <sha> --body-file F   # correctness lane
deskpost security-review <owner/repo> <pr> --verdict pass|fail               --head <sha> --body-file F   # security lane (pass = COMMENT event)
deskpost comment         <owner/repo> <pr> --body-file F                                                  # informational, NOT a verdict
```

The App-token mint is absorbed in-tool; `desktoken` is the only mint path anywhere (CLAUDE.md
§ Identity & posting). `--approve` on a pass, `--request-changes` on any blocker with the full
findings body. A plain `--comment` review is allowed for informational notes but is **NOT a
verdict** — only APPROVED / CHANGES_REQUESTED count, and only those move the board.

**Two lanes, two idempotency keys.** The correctness and security lanes of one PR run concurrently
and post independently: the correctness verdict keys on `review:correctness:<flag>` and the security
verdict on `review:security:<flag>` plus the body digest, so two lanes landing at the SAME head
never no-op each other. **A security pass goes through `deskpost security-review` ONLY** — it
submits as a COMMENT-event review, visible to the flip gate but invisible to GitHub's approval
reduction. The `deskpost review --verdict approve` shape is LEGACY for a security pass and is the
laundering shape: under the one shared reviewer App a later same-head security APPROVE erases an
at-head correctness CHANGES_REQUESTED (the 2026-08-15 laundering). A plain comment is the opposite
failure — invisible to the flip gate, so it moves nothing. Exactly one verdict kind per body, read
per-lane.

## The body schema — the BODY FILE must carry it; `--verdict` does NOT satisfy it

deskpost body-checks the file independently and refuses (exit 5) unless the body carries all four
(`tools/desk/README.md` § "Verdict format"):

1. at least one Markdown **H2 heading** (`## …`);
2. a **bare** verdict line — `Verdict: approve` / `Verdict: request-changes`, or for a security
   review `Security-Review: pass` / `Security-Review: fail`. Bare = only whitespace before the key:
   `## Verdict: APPROVE` is refused (the `## ` prefix, not the caps — case is fine), and so is
   `**Verdict: approve**`;
3. body **≤ 16384 bytes (16 KiB)** — over-cap is refused outright, never truncated: split or trim;
4. exactly ONE verdict kind — a body carrying both a `Verdict:` line and a `Security-Review:` line
   is refused; quote the other lane's line with a leading `> ` to reference it. (Read-side, a body
   carrying both `pass` and `fail` counts as `fail`.)

**A refused body costs the desk, not just the post.** Five consecutive non-progress attempts
(refused/noop) open deskpost's circuit breaker — 15 minutes, blocking every deskpost writer
(reviews, comments, ready flips), not only yours. Never retry a refused body unchanged: a
SCHEMA refusal (rules 1–4 above) is fixed by the body's author first; the refusal reason is in the
audit `detail`. A CONTENT-SCAN refusal (`refused: <rule id> at body:<line>`) is not a schema
defect — it is a STOP (`SKILL.md` § "A scan refusal on a verdict body is a STOP — never reword
it"). Any other exit-5 refusal that names no scan rule — a malformed typed finding block, for
one — is the body author's to fix, like a schema refusal. **Exit 5 is NEVER a fallback trigger** —
fall back only on exit 3 (disabled) / 6 (unverifiable).

## Coverage — the correctness verdict's per-class accounting block

A correctness verdict body ENDS with an `## Coverage` section: one line per class in the
correctness class register. The register's home is the dispatched review kit's `review-prompt`
reference, never this file; each line is exactly one of four shapes, one grammar — class
first, state second, colon before the detail, except the finding shape:

```
<class> — finding <finding-id>
<class> — checked-clean: <what was examined>
<class> — n/a: <one-sentence reason>
<class> — could-not-check: <what was missing>
```

A class that applied, was examined, and is sound is `checked-clean`; a class that genuinely
does not apply to this diff is `n/a` with its one-sentence reason — the two must never share
a state, and an examined-and-sound class written as `n/a` is precisely the leak this
contract exists to close. A class that could not be honestly `n/a`'d and was not checked is
`could-not-check`, never a silent pass; silence is not accounting, and a class with no line
is a gap in the accounting, not an answer. A could-not-check line is disclosure: an APPROVE
stands unless it rests on that class. The accounting covers the stated scope: a delta
round accounts for the classes the delta range's paths can touch; a full pass accounts for
every class in the register. A security verdict carries no Coverage block — the register is
the correctness lane's, and the two verdicts stay separate artifacts.

The block is small by construction: all 14 classes at one line each are a small fraction of
the 16 KiB body cap — cut prose elsewhere, never accounting.

`deskpost` neither requires nor refuses the block today. It rides the body as plain lines
under the schema above, and a body without it passes that schema exactly as before; whether
deskpost should refuse a correctness body carrying no Coverage block is a separate, later
ruling — this contract is doctrine, not a gate.

## When your ONLY finding is a red required check

A `request-changes` whose sole blocker is a required CHECK is a special case, because that
blocker can clear **without a code push** — a human applies `changelog:skip`, a flaked job is
re-run — and the flip gate's default is that an APPROVE at an unchanged head re-verifies
nothing and does not lift a standing CHANGES_REQUESTED. Left alone that costs either a no-op
push (which games the rule) or a human dismissing your review by hand.

Two extra lines, and only these two, open the narrow path out. Both are ordinary body lines —
neither is a verdict kind, so neither trips rule 4 above:

1. **On the CHANGES_REQUESTED**, declare the check and nothing else:

   `Blocked-On-Check: changelog`

   Write it **only when it is true** — when the check is genuinely the whole of your finding.
   The line is a claim about your own review, and the gate reads the claim, not your prose: it
   has no way to notice that the CR also carried three code findings. A CR with any other
   finding gets no such line.

2. **On the APPROVE that answers it**, cite the specific run that went green:

   `Cleared-Check-Run: 41234567890`

   The numeric **run id**, not the check's name — `gh api repos/<slug>/commits/<head>/check-runs`
   lists them. It identifies one execution, which is the point: it must be the run at *this*
   head, of the check you named, finished green (`success` / `neutral` / `skipped`) **after** you
   posted the CR. Cite the run you actually looked at; citing an older run of the same check, or
   a different check's run, is refused.

Both lines are read whole-line and never from inside a fenced code block — quoting the format to
explain it (as this file does) declares nothing. Two lines of the same marker that disagree
establish nothing. Miss any part and the flip refuses exactly as it does today, so there is no
way for a wrong citation to become a flip; but a **false** `Blocked-On-Check:` on a CR that
carried real findings is you clearing your own block, and nothing downstream will catch it.

Clearing the block is not an approval, either: the gate still needs your APPROVE to be the
governing verdict at head, so if you post a further CHANGES_REQUESTED afterwards the PR blocks
again.

## When your ONLY finding is the PR body

A `request-changes` whose sole blocker is the PR **body** (the description asserts something
false or stale) is answered by a body edit, and a body edit never moves the head. The flip gate
admits a same-head APPROVE over such a CR only in this documented, machine-checkable form:

1. **On the CHANGES_REQUESTED**, declare the finding and the body you blocked on:

   `Blocked-On-Body: f-body-1 <64-hex digest of the body as you read it>`

   Only when the body is genuinely the whole of your finding. A typed finding block on the same
   review that names any other blocking finding makes the CR mixed, and a CR also carrying
   `Blocked-On-Check:` or `External-Prereq-Only:` qualifies for neither class.

2. **On the APPROVE that answers it**, after re-reading the live body via the API:

   `Resolved-Body-Finding: f-body-1`
   `Body-Reread-Digest: <64-hex digest of the live body you re-read>`
   `CI-Green-At: <full 40-hex head sha>`

The digest is SHA-256 of the body with carriage returns removed and trailing newlines trimmed:
`printf '%s' "$(gh api repos/<slug>/pulls/<N> --jq .body | tr -d '\r')" | shasum -a 256`.
`deskflip` recomputes the live body's digest at flip time (and again just before the mutation)
and refuses unless it equals your re-read digest **and** differs from the CR's. It also reads the
forge's own record of the body's last edit and refuses unless that edit is later than your CR —
your recorded digests alone never establish that the body was edited. So a re-approve over an
unedited body, or over a body edited again after you read it, clears nothing. It still
judges CI green itself; your `CI-Green-At:` line is the documentation, not the check. The same
fence and ambiguity rules as the check-only lines apply.

### A security fail whose ONLY basis is the PR body

A `Security-Review: fail` is never cleared by a correctness APPROVE, and an unmarked one is
cleared only by a new commit. When the fail's **whole** basis is the PR body (the description
claims a control the diff does not add, say) and no code at head would change, mark it the same
way:

1. **On the `Security-Review: fail`**, add `Blocked-On-Body: f-body-1 <64-hex digest of the body
   as you read it>`. Never add it when any part of the fail rests on code, config or a check:
   that fail stays unmarked. `Blocked-On-Check:` and `External-Prereq-Only:` are not security-lane
   clearances, and a security fail carrying either one always blocks.
2. **After the author edits the body**, re-read it via the API and post a `Security-Review: pass`
   at the same head carrying `Body-Reread-Digest: <64-hex digest of the live body you re-read>`.
   It needs no `Resolved-Body-Finding:` or `CI-Green-At:` line, because the pass is the whole
   security lane's verdict and CI stays the gate's own check.

`deskflip` clears the fail only when your pass is the last security verdict you posted at that
head, it stands (a COMMENTED or APPROVED review; a dismissed or pending one clears nothing), its
digest equals the live body's and differs from every fail's, and the forge records a
body edit after the last fail. A later unmarked fail blocks again, and a clearance applies only
at its own head: after any head move the fail stands until a pass at the new head.

The gate cannot check that your marker is true. "The basis was only the body" is your claim; the
later pass and the forge's body-edit record back it up, and nothing else does.

## The secret scan

The scan refuses any run of 32+ base64ish characters, plus token prefixes, `AKIA…`, PEM and JWT
markers, and sops markers. Exempt: exactly-40/64-character lowercase-hex git SHAs, and
slash-separated paths built from word-shaped segments (`file:line` refs are fine). A 32+-character
run with NO slash is refused however word-shaped — long CamelCase identifiers (Go test names,
template names) fire this, and backticks do NOT help, because a backtick is outside the scanned
charset and the run inside stays contiguous. Break the identifier or shorten the reference. Quote
the numeric review `id` from `gh api repos/<slug>/pulls/<N>/reviews`; never paste prefixed digests
or base64 blobs.

Cite sops material, never quote it. A finding about an encrypted file names it by `path:line`
and describes it in words: the `sops` footer, its `mac` or `lastmodified` field, an `ENC[…]`
envelope missing its `iv`. A quoted footer or envelope is refused on every surface, inside a
code fence too. The audited `--force-scan-override` is a human act, and a reviewer never passes
it to get a verdict through.

That is authoring guidance for the FIRST post. Once the scan has refused a verdict body, the body
is evidence: the desk never rewords, re-encodes, splits or trims it, the scan override is the
maintainer's alone and exists only for the rules the tool lets it waive — no flag waives
`voice.ruling-claim` or `withheld.identifier`, so there the filing asks the maintainer for a
ruling, not an override — and the one re-issue is the reviewer's own, restating each refused span
as a `path:line` citation. Whether the reviewer may restate its OWN prose on those two rules is
still open, a maintainer decision (`SKILL.md` § "A scan refusal on a verdict body is a STOP —
never reword it").

## If a raw `gh pr review` is ever unavoidable

Run it **BARE and read `gh`'s OWN exit — never `gh pr review … | grep …`**: the
pipe's last stage owns `$?`, so a SUCCESSFUL post reads as failed and gets re-posted, and a
submitted review cannot be retracted, so the retry is permanent duplicate noise. If a pipe is
unavoidable, use `set -o pipefail` + `${PIPESTATUS[0]}` / `$pipestatus[1]`. Before any manual
retry, check the post already landed — `gh pr view <N> -R <slug> --json reviews` for an
`assay-reviewer-app[bot]` review at the current head — and skip if so. (`deskpost review` needs no
such care: it reads the App's live reviews at head and no-ops on a duplicate.)

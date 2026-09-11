# Verdict mechanics and the verdict-body schema

The desk's verdict-posting interface. `pr-review-desk/SKILL.md` § The reviewer's bar points here;
`deskdispatch --kit review` §9 carries the generic wording the dispatched agent receives. The tool
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
(reviews, comments, ready flips), not only yours. Never retry a refused body unchanged: fix it
first; the refusal reason is in the audit `detail`. **Exit 5 is NEVER a fallback trigger** — fall
back only on exit 3 (disabled) / 6 (unverifiable).

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

## The secret scan

The scan refuses any run of 32+ base64ish characters, plus token prefixes, `AKIA…`, PEM and JWT
markers, and sops markers. Exempt: exactly-40/64-character lowercase-hex git SHAs, and
slash-separated paths built from word-shaped segments (`file:line` refs are fine). A 32+-character
run with NO slash is refused however word-shaped — long CamelCase identifiers (Go test names,
template names) fire this, and backticks do NOT help, because a backtick is outside the scanned
charset and the run inside stays contiguous. Break the identifier or shorten the reference. Quote
the numeric review `id` from `gh api repos/<slug>/pulls/<N>/reviews`; never paste prefixed digests
or base64 blobs.

## If a raw `gh pr review` is ever unavoidable

Run it **BARE and read `gh`'s OWN exit — never `gh pr review … | grep …`**: the
pipe's last stage owns `$?`, so a SUCCESSFUL post reads as failed and gets re-posted, and a
submitted review cannot be retracted, so the retry is permanent duplicate noise. If a pipe is
unavoidable, use `set -o pipefail` + `${PIPESTATUS[0]}` / `$pipestatus[1]`. Before any manual
retry, check the post already landed — `gh pr view <N> -R <slug> --json reviews` for an
`assay-reviewer-app[bot]` review at the current head — and skip if so. (`deskpost review` needs no
such care: it reads the App's live reviews at head and no-ops on a duplicate.)

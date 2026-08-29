# Verdict mechanics and the verdict-body schema

The desk's verdict-posting interface. `pr-review-desk/SKILL.md` § The reviewer's bar points here;
`deskdispatch --kit review` §9 carries the generic wording the dispatched agent receives. The tool
reference is `tools/desk/README.md` — the public home of the desk tools.

## Posting a verdict

The verdict is a **real GitHub review by the reviewer App, not a text marker**:

```
deskpost review <owner/repo> <pr> --verdict approve|request-changes --head <sha> --body-file F
deskpost comment <owner/repo> <pr> --body-file F
```

The App-token mint is absorbed in-tool; `desktoken` is the only mint path anywhere (CLAUDE.md
§ Identity & posting). `--approve` on a pass, `--request-changes` on any blocker with the full
findings body. A plain `--comment` review is allowed for informational notes but is **NOT a
verdict** — only APPROVED / CHANGES_REQUESTED count, and only those move the board.

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

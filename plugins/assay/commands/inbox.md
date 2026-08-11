---
description: What's waiting on you across your configured repos — needs-decision, question, help wanted, and urgent issues, sorted urgency-then-age. Read-only, works from any terminal.
argument-hint: "[owner/repo ...]"
---

# assay:inbox

The inbox: a **derived query**, not a stored roll-up or a service. This command
shells `../scripts/assay-inbox.sh` — a plain `gh` + `jq` helper — across your configured
repos and prints what's waiting on a human decision, most urgent (then oldest) first.

**Read-only.** It never writes to, comments on, or closes any issue. **Terminal-agnostic.**
It is plain `git`/`gh` shell — it runs from any terminal.

## What it queries

The four escalation label tokens. `needs-decision` is created and owned by the escalation
contract; the other three are stock GitHub labels this inbox folds in alongside it:

- `urgent`
- `needs-decision`
- `question`
- `help wanted`

An issue carrying more than one of these is shown once, ranked by the most urgent label it
carries. Each row prints label, repo, `#number`, title, age, and the issue URL.

## A failed query is never shown as an empty inbox

The run always ends with a summary line — `assay-inbox: N item(s) across M repo(s)` — so
"nothing is waiting" is *positively* stated rather than inferred from silence. If any `gh`
query fails (expired token, missing repo, rate limit), the helper prints `gh`'s own
diagnostic to stderr, marks the summary `THIS INBOX IS INCOMPLETE`, and exits non-zero.

| Exit | Meaning |
|---|---|
| `0` | every query succeeded — the table is complete |
| `1` | precondition failure: `gh`/`jq` missing, or no repos resolvable |
| `2` | one or more queries FAILED — the table is **partial**, see stderr |

Per-repo-per-label fetch cap is `ASSAY_INBOX_LIMIT` (default 500); `gh issue list` would
otherwise default to 30 and silently discard the *oldest* items — the exact ones this
ordering exists to surface. Hitting the cap is reported, not swallowed.

## Repo resolution (in order)

1. **Arguments** — pass one or more `owner/repo` pairs: `/assay:inbox owner/repo1 owner/repo2`.
2. **`./.assay/repos.txt`** — if no arguments are given and this file exists in the current
   directory, it is read as a flat newline-delimited list of `owner/repo` (blank lines and
   `#`-comments ignored). This is the inbox's repo config — a flat list, nothing more.
3. **Origin default** — if neither of the above applies, the command falls back to the
   current repo's `origin` git remote.

## Usage

```
/assay:inbox                          # uses .assay/repos.txt, else origin
/assay:inbox example-org/app example-org/service   # one or more repos
```

## Instructions for the agent running this command

1. Resolve `$ARGUMENTS` as the repo list (may be empty).
2. Before running anything, check that every repo argument matches `owner/repo`
   (`[A-Za-z0-9._-]+/[A-Za-z0-9._-]+`). Refuse to run the command if any token does not
   match, and say why. Then pass each repo as its own separately-quoted argument — never a
   bare unquoted `$ARGUMENTS` expansion:
   ```
   bash "${CLAUDE_PLUGIN_ROOT}/scripts/assay-inbox.sh" "owner/repo1" "owner/repo2"
   ```
   (`CLAUDE_PLUGIN_ROOT` resolves to this plugin's installed root; if unset, resolve the
   script relative to this command file's own `../scripts/assay-inbox.sh`.)
3. Print the helper's table output verbatim to the user — do not summarize away rows, do not
   edit, close, or comment on any issue found. If the helper errors (missing `gh`/`jq` auth,
   no repos resolvable), surface the error message as-is; do not guess at issue state.
4. **Check the exit code.** A non-zero exit means the inbox you are looking at is incomplete
   or absent — say so plainly and relay the stderr diagnostic. Never report "nothing is
   waiting" on a non-zero exit; that is the failure mode this command exists to avoid.

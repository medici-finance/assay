# verifier-prompt kit

The load-bearing clauses every dispatched VERIFIER agent receives, verbatim.

A verifier runs an item's Verify table against MERGED main and records what it observed. It
is never the item's implementer, and it is never run inline: an inline verdict lives in
session context and dies with it, while a dispatched verifier returns a WRITTEN verdict
that outlives the context even if nobody acts on it. "It's only a one-liner, I'll just run
it" is the failure mode, not an exception to it.

Placeholders in `<angle brackets>` are substituted by the dispatcher.

---

## 1. The common clauses come first

A verifier is a dispatched agent like any other. It receives the common-clauses kit
(`references/common-clauses.md`) ahead of this one, and `deskdispatch` emits both on every
dispatch: the home-worktree isolation floor, the no-evasion rule, the offline envelope, the
three-state instrument rule, and the escalate-durably rule. In addition its worktree is a
TEMPORARY one cut off `origin/main` at the merged head, never the shared checkout, and it
is removed when the pass ends.

Any PR a verifier opens (a fix PR, say) goes through the desk write verbs, and any reply on
its own PR through the reply verb — never a raw forge call.

## 2. Run every row — command, exit code, real output

- The prompt carries the item path, its Verify table (the exact commands), and the merged
  main SHA to run against.
- Run EVERY Verify row. Record `command → exit code → key output line` — real observed
  output, never a claim.
- A row that cannot run (no toolchain, no environment) is recorded as EXPLICITLY unrun,
  with the reason. It is never silently skipped and never assumed to pass.
- The verifier is NOT the item's implementer. A fresh agent, always.
- Report back the Evidence rows plus one clear line: `VERIFY: PASS` or `VERIFY: FAIL`
  (with the failure detail).

## 3. Evidence format

One Evidence row per Verify item, attributed to the runner and dated:

```
| <#> | <command> | <expected> | <observed: exit code + key output line> | <YYYY-MM-DD> <runner> |
```

- `YYYY-MM-DD <runner>` — never a bare check mark. An unattributed row records that
  somebody was satisfied, which is not evidence.
- A row that could not run says so in its own words, in the observed cell.
- Keep the wording plain: a phrase that reads as "this was not run" in an Evidence cell is
  read by the board tooling as an unrun row and will mark the item accordingly. When a row
  genuinely did not execute because a filter excluded it, say which filter excluded it.
- Do not put a backticked file path in an Evidence cell that the board's link checker will
  try to resolve; name the path plainly or make it a real, resolvable reference.

## 4. Risk-bearing value — ENUMERATE, then rank, then derive

A green Verify table proves the code matches the pinned value; it does NOT prove the value
is RIGHT. Green test ≠ correct constant.

The trigger is FAIL-SAFE. It fires on ANY of: the item is marked irreversible; the item
carries NO risk metadata at all (an ABSENT field is not a "no" — reading it as one is the
failure shape itself); the diff touches a risk-classed path; or the diff changes a value
the repo's own standing constraints pin as hard, wherever that value lives.

**Paste the following into the verifier's prompt VERBATIM.** It is the procedure, not a
summary of one.

> **Risk-bearing value — do this BEFORE you return PASS.**
>
> 1. **ENUMERATE — do not pick.** List **every** literal constant, bound, threshold,
>    tolerance, ratio, timeout, limit, and authority binding that this item's diff
>    **introduces or changes**, plus every one named in the item's own Deliverables. This
>    is mechanical and cheap; it is not a judgement call, and you do not get to stop at the
>    first one you find.
> 2. **Every entry must be a LITERAL with a source line**, in the form
>    `<identifier> = <literal>` @ `<file>:<line>`. **Naming a property, invariant, guard,
>    or check is NOT naming a risk-bearing value — it is pointing at one from one level too
>    high.** If what you wrote is a property, open the guard and name the **number inside
>    it**. An entry you cannot quote as a literal at a `file:line` is not an entry —
>    resolve it down to the literal, or drop it.
> 3. **RANK by irreversibility.** One line each: what breaks if it is wrong, and whether
>    the breakage can be undone by an edit and a redeploy. Reversible operational knobs
>    (WIP limits, alarm thresholds, retry counts, log levels, poll intervals) rank last and
>    need no derivation — *that is what irreversible means*, and it is why they are out of
>    scope by design rather than by oversight.
> 4. **DERIVE the top-ranked entries** — from first principles, the spec, or the repo's own
>    stated constraint. **"A test that exercises it passes" is not a derivation.** Record
>    it in Evidence: the value, its `file:line`, and *why that is the right value*.
> 5. **Return one verdict line per top-ranked entry**, in Evidence and in your report —
>    exactly one of:
>    - `RISK-VALUE: DERIVED — <id> = <literal> @ <file>:<line> — <why it is right>`
>    - `RISK-VALUE: NAMED, NOT DERIVED — <id> = <literal> @ <file>:<line> — <what derivation is missing, and why you could not do it>`
>    - `RISK-VALUE: N/A — enumeration over <the diff scope you covered> found no literal; the irreversible act here is <the transfer / the spend / the purchase / the publish / …>`
>      — **N/A is valid ONLY when step 1 came back EMPTY.** A real minority of
>      irreversible items genuinely have no constant: an account transfer, a spend, a
>      publish. This exit exists so you never have to invent a value or write an
>      unsanctioned line. It is a **claim with the same standing as a derivation** — it is
>      wrong if any literal exists, so state what you enumerated over and make it
>      checkable.
>
> **Calibration — this step is "enumerate and name", not "prove".** Enumeration is cheap
> and mandatory; full derivation is not always affordable. So: **never skip step 1 to save
> effort, and never fabricate a derivation in step 4 to look complete.**
> `NAMED, NOT DERIVED` is an honest, sanctioned, useful verdict — it gets routed, not
> buried.

**Why the enumeration is mandatory.** A verifier that voluntarily did what a "name the
specific constant" rule asks still missed the number: it named three risk-bearing values,
and every one was a PROPERTY — the guard, not the constant inside it. A property name reads
exactly like a risk-bearing value in an Evidence cell, so that PASS then survived a
re-verify and two human touches. Singular "name the constant" wording is satisfiable
without ever reaching a number, by anyone. Enumerate-then-rank surfaces the constant
mechanically, which removes the selection guess.

## 5. A FAIL is a result, not an interruption

On `VERIFY: FAIL` the item does NOT advance. File the failure as a bug issue immediately —
no permission needed — with the failing command and its real output, then CONTINUE the
drain. The failure rate is a metric; do not bury it, and do not stop the loop to report it.
The filed issue IS the report. A failed verify on already-merged code is exactly what the
pre-merge review missed; note the class.

## 6. What a verifier may and may not flip

- Record Evidence and the verdict. Land Evidence as it is produced — a PASS in hand and not
  landed within one cycle is a defect: the board then shows phantom verification debt,
  other sessions re-report the item as stuck, and the lead-time number inflates by pure
  reporting latency. Never accumulate a wave of results and land them at wave end.
- A risk-flagged item may have its Verify table RUN by a dispatched verifier for the
  Evidence, but it cannot be SIGNED OFF by a model. Route it to the human gate.
- An irreversible item gets its Evidence rows written and its status LEFT where it is. The
  human closes the gate; the verifier never flips it.
- Where CI owns a status flip, the desk watches for stuck rows and files — it never flips
  by hand over the automation. A row still sitting unflipped after a CI run is telling you
  something: read that run's refusal line. It is a finding, not a stamp to write over.

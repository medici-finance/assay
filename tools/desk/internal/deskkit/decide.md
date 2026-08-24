# `deskkit.Decide` — the escape-valve consult contract

`Decide` is a bounded "ask-an-agent" primitive for the deterministic desk loops. A
loop reaches a point its own code cannot classify — *is this verify FAIL stale-artifact
rot or a real regression? is this refusal retryable or terminal?* — and instead of
guessing badly or stopping, it consults a read-only agent that must pick **exactly one
member of a fixed vocabulary the loop declared up front**.

The whole design is one sentence: **the agent advises, the loop acts, and the loop only
ever acts among moves it could already make.** Every rule below is a consequence of that
sentence.

## The contract

| Property | Rule |
|----------|------|
| Advise vs. act | The consult returns a vocabulary member. Acting on it is the loop's own code — the valve never enlarges the loop's action space. |
| Read-only agent | The `Advisor` is a pure function of the `Consultation`: structured context in, one enum member + a one-line justification out. It is never handed a shell or tools. |
| Reserved verbs | No vocabulary may contain a member equivalent to a human-gate action. Enforced at **construction** (`NewQuestion`), not at answer time. |
| Fail-closed default | An invalid answer, a timeout, an advisor error, a spent budget, or the kill switch all resolve to the pre-declared conservative **default**. The valve degrades to the loop's no-valve behaviour; it never spins. |
| Journalled | Every consult records question, a **digest** of the context (never the raw, possibly-untrusted context), the answer, its justification, elapsed time, and the outcome. An advised (non-default) answer is returned **only if its journal line was written**. |
| Optimization only | Every consumer must run correctly with the valve off. `DESK_DECIDE_DISABLED=1` makes every consult return its default without consulting anyone. |

### Single point of failure, and its two independent backstops

The enum validator is the single point of failure — it is what turns "whatever the agent
said" into "a member of a known-safe set". Behind it stand two backstops that fail for
**different reasons in different places**, so a defect in one does not disarm the other:

1. **Fail-closed default** — an invalid or absent answer never acts; it falls to the
   pre-declared conservative option.
2. **Per-item / per-hour budget** — a confused loop degrades to its no-valve behaviour
   rather than consulting in a spin.

### Injection posture

`Consult.Context` carries untrusted repo text (PR bodies, CI logs). Enum validation
bounds a prompt injection to picking a *wrong-but-safe* branch of the vocabulary;
anything the agent returns that is not a vocabulary member is malformed and lands on the
default. Because reserved verbs are refused at construction, no branch an injection could
steer toward is ever a human-gate move. The raw context is never journalled — only its
sha256 digest — so an untrusted body cannot be laundered into the audit log.

## Reserved verbs (refused at construction)

A vocabulary member is rejected by `NewQuestion` if any of its word-tokens equals a
reserved root, or its separators-removed form contains a reserved phrase. This is the
compiled-in embodiment of *"a strong model does not satisfy a human gate."*

Reserved roots: `approve` / `approved` / `approval`, `flip` / `flipped`, `merge` /
`merged`, `ready`, `sign` / `signed` / `signoff`.
Reserved joined phrases: `closegate`, `signoff`, `readyflip`.

So `merge`, `Merge-It`, `mark-approved`, `close-gate`, and `ready-flip` are all refused;
innocuous members that merely *contain* a reserved substring inside a larger token —
`already-done`, `signal` — are accepted (token match, not substring match).

## Signature

The brief's shape `Decide(question, context, vocabulary, default, budget) → member`
maps onto two Go types so that the vocabulary/default invariants are validated **once**,
at registration, and reused across many consults:

```go
q, err := deskkit.NewQuestion(prompt, vocabulary, def) // validates vocab + deny-list here
answer, _ := q.Decide(ctx, deskkit.Consult{
    Item:    "owner/repo#123",  // per-item budget key
    Detail:  "verify row 4",    // per-situation framing
    Context: untrustedText,      // digest-journalled, never stored raw
    Advisor: advisor,            // read-only agent (nil ⇒ valve off)
    Journal: journal,            // nil ⇒ shared desk-tools audit log
    Budget:  budget,             // nil ⇒ unbounded; share one across items
    Timeout: 30 * time.Second,
})
```

`Decide` always returns a vocabulary member and, on every operational path, a nil error:
kill switch, no advisor, spent budget, timeout, advisor error, invalid answer, and
un-journallable advice all resolve to the default so the loop proceeds.

## Worked example 1 — verify FAIL triage

A verification loop finds a FAIL. Is it stale-artifact rot (the tree moved under an
intact check → re-baseline), a genuine regression, a transient flake worth one retry, or
something a human must see?

```go
triage, _ := deskkit.NewQuestion(
    "Classify this verify FAIL.",
    []string{"REBASELINE", "REGRESSION", "RETRY", "ESCALATE"},
    "ESCALATE", // conservative default: when unsure, a human sees it
)
verdict, _ := triage.Decide(ctx, deskkit.Consult{
    Item:    prRef,
    Detail:  "row 4: link-check on a moved sibling path",
    Context: failOutput, // untrusted CI text
    Advisor: advisor,
    Budget:  budget,
})
// verdict ∈ {REBASELINE, REGRESSION, RETRY, ESCALATE}; the loop's code acts on it.
```

The default is `ESCALATE` — the branch that surrenders to a human — so every failure of
the valve routes the ambiguous FAIL to a person rather than auto-closing it.

## Worked example 2 — refusal handling

A poster loop's write is refused. Retry the same call, reroute it (to a human / another
lane), or escalate? This is the exact case the primitive exists for: retrying a refusal
five times is what tripped a per-repo breaker for fifteen minutes.

```go
refusal, _ := deskkit.NewQuestion(
    "How should this refusal be handled?",
    []string{"RETRY", "REROUTE", "ESCALATE"},
    "ESCALATE", // never the unbounded retry by default
)
move, _ := refusal.Decide(ctx, deskkit.Consult{
    Item:    prRef,
    Detail:  "deskpost refused (rc 5) on a quarantined PR",
    Context: refusalDetail,
    Advisor: advisor,
    Budget:  budget,
})
```

`RETRY` is a member of the vocabulary, but it is **not** the default — so a confused or
injected valve can never *default* into the retry that caused the incident; a human-safe
`ESCALATE` is where every fail-closed path lands.

## Kill switch and budgets

- **Kill switch:** `DESK_DECIDE_DISABLED=1` turns the valve off globally. It is separate
  from the desk-tools `DISABLED` switch because the valve is an optimization layered on
  loops that already run correctly without it — an operator can disable *judgement*
  without halting the *loops*.
- **Budgets** (`NewBudget(perItem, perHour)`, `0` = unbounded on that axis) are rolling
  one-hour windows. Share one `Budget` across a loop's items so the per-hour axis is a
  fleet-wide cap, not a per-item one.

## Consumers

This package lands the primitive and its contract only. Wiring specific loops
(verification triage, refusal handling) to consult it are separate follow-ups; each such
consumer must still run correctly with the valve disabled.

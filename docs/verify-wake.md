# Verify wake: when a held brief is re-run

A verify run that does not pass (`verify-fail` or `blocked`) lands an outcome record that
carries a wake receipt (`wake_schema: verify-wake-v1`). `deskevidence --outcome-record`
derives the receipt when it lands the record:

- `inputs` holds one `file:<path>` key per path in the brief's `## Context` `files:` list that
  exists at the record's sha. A listed directory contributes every file under it. The keys
  also include the brief itself, hashed as it lands, and the tool version.
- `blocker_kind` and `blocker_ref` come from the caller and are required. `blocker_ref` must be
  an issue or change reference (`#N`, `<owner>/<repo>#N`, or an issue or pull-request URL). A
  placeholder such as `to file` is refused with exit 5.

`verifyloop plan` then decides, per brief, whether its latest record is worth another run. It
makes two independent reads, and each one can return could-not-check:

- **Inputs.** Every declared input is hashed again at the current tree and compared with the
  receipt.
- **Blocker issue.** The issue named by `blocker_ref` is read through the forge as the verifier
  App. If there is no token, the token is rejected, the reference is not an issue, or the state
  is unknown, the read is could-not-check. It is never read as "closed". `plan --no-forge`
  makes no forge call, so every blocker read is could-not-check.

## The hold rule

First match wins.

| Receipt | Inputs | Blocker issue | Result |
|---|---|---|---|
| explicit-recheck receipt newer than the hold | any | any (not read) | dispatchable: "recheck: <reason>" |
| absent or incomplete | — | — | dispatchable once: "classification pass, writes a receipt" |
| complete | changed | any | dispatchable: "inputs changed: <path>" |
| complete | unchanged | closed | dispatchable: "blocker closed: <ref>" |
| complete | unchanged | open | **wait** — next actor from the blocker class |
| complete | could-not-check | any | held, surfaced as could-not-check |
| complete | unchanged | could-not-check | held, surfaced as could-not-check |

The explicit recheck is the first row so that it overrides every hold below it. A recheck
receipt older than the hold has been superseded by the hold.

Every held line names the blocker reference, the wake condition and the next actor (the
blocker class decides who acts next: `implementation` is the worker, `check-definition` is the
brief author, `human-action` is a human, `environment` is the operator, `unknown` is the
verifier). The plan ends with one summary line:

```
verify-desk plan: wait=<n> dispatchable=<n> could-not-check=<n>
```

`wait` counts briefs held on an open blocker. Briefs held because a read could not be made are
counted under `could-not-check`, never under `wait` or `dispatchable`.

Receipts whose predicate is `referenced-action-completed` or `declared-deadline-reached`
keep their own evaluation, which is unchanged by this rule.

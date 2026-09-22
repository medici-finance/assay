# review-finding/v1 — the persistent review-finding record

A review round loses continuity the moment the reviewing or working agent is
replaced: the successor rereads the whole pull request from prose, restates old
objections under fresh IDs, and the round counter resets. This record makes the
disputed state **durable on the forge** so a follow-up review — by any agent —
resumes from a ledger rather than from a reread.

Implemented in `tools/desk/internal/deskkit/reviewfinding.go`; derived and
rendered by `reviewloop`; validated at the write boundary by `deskpost review`
(reviewer role) and `deskreply` (worker role).

## Where it lives

The record is a **versioned block embedded additively in the forge review/reply
body**, inside an HTML comment so a legacy Markdown reader — and every human
reading the thread — sees nothing while a parser can find it unambiguously:

```
<!-- assay:review-finding:v1
{
  "schema": "review-finding/v1",
  "findings": [
    {
      "id": "F-lease-ttl",
      "class": "unbounded-lease",
      "severity": "blocking",
      "blocker": "code-content",
      "state": "open",
      "originHead": "<sha the finding was first raised against>",
      "evidenceHead": "<sha the evidence in THIS record was gathered at>",
      "failure": "concrete reproduction, or the acceptance obligation left unmet",
      "resolution": "what would resolve it",
      "evidence": ["<link or ref>"]
    }
  ]
}
-->
```

**Additive-record compatibility.** An older tool that never learned the block
skips it as a comment and reads the surrounding prose exactly as before. A body
with no block is the legacy case: it asserts nothing about the ledger, and a
clean finding set is **never inferred from unparseable prose**.

## The fields

| Field | Meaning |
|---|---|
| `id` | Stable identity across heads and agents. A newly discovered **occurrence** of the same proposition **reuses** this id; only a genuinely new proposition gets a new one. |
| `class` | The claim class. The round cap is **per class**: fixing one sentence never resets the class, and a sibling occurrence retains it. |
| `severity` | `blocking` or `advisory`. Only a reviewer may promote `advisory`→`blocking`, with changed impact or new evidence. |
| `blocker` | `code-content` (a defect in this branch) or `external-prerequisite` (a shared-CI leg or upstream fact — one shared repair, not a per-PR defect). |
| `state` | `open`, `fixed-awaiting-review`, `disputed`, `resolved`, `awaiting-arbitration`. |
| `originHead` / `evidenceHead` | The head first raised against; the head this record's evidence was gathered at. A resolution whose `evidenceHead` is not the current head cannot clear the finding. |
| `failure` / `explanation` / `evidence` | A **blocking** finding must carry a concrete reproduction, an evidence-based explanation, or an evidence link — a bare assertion cannot block. |
| `sharedRepair` | The one shared repair reference an external-prerequisite blocker points at, so N PRs cite one repair. |

## What the derivation guarantees

The ledger is a **pure function of the durable records**, folded in forge order
(`deskkit.DeriveLedger`). That is the whole point: re-deriving from the same
records after an agent is replaced or a process restarts yields the same IDs, the
same per-class round counts, and the same single arbiter packet. There is no
in-memory ledger to lose.

- **A worker cannot author a reviewer resolution.** Actor identity and head come
  from the authenticated forge event, not from prose. A worker record may move a
  finding to `fixed-awaiting-review` or `disputed`; only a reviewer clears a
  blocking finding, and only with current-head evidence. This is enforced twice:
  at the write boundary (`deskreply`'s role gate) and again in the derivation, so
  a record that reached the forge by a path that skipped the tool still cannot
  clear a blocker.
- **The existing round cap survives replacement.** A round is a durable
  review → worker-response → re-review transition — never a commit or a poll tick.
  At the existing three-round-per-class cap the derivation produces **one**
  deduplicated arbiter packet for the human decision lane and holds the class.
  Re-deriving the records refiles nothing; a duplicate reviewer sweep at the cap
  refiles nothing; a newly noticed sibling sentence keeps the class and is held,
  it does not start a fresh count. The cap threshold is **not changed** by this
  record, and the packet never overrules a reviewer — it is data for the existing
  human authority.
- **Continuity across heads.** A finding keeps its id and evidence when the head
  advances; a follow-up review targets both the fix and the changed surface. An
  approval recorded at an old head is **not** carried across the change.
- **Shared CI is not re-invented as N defects.** An `external-prerequisite`
  blocker citing a `sharedRepair` is one shared repair across every PR that cites
  it, not a content defect per PR. It still blocks a ready-flip — the applicable
  checks are still required — but it is not a per-branch correctness defect.

## Reactor consumption

`reviewloop plan --records <thread.json>` derives and renders the ledger for one
PR's review thread: the outstanding blocking findings, the per-class rounds
against the cap, the arbiter packets, and every could-not-check reason (a record
missing its authenticated role or head, a worker record that tried to clear a
blocker). "No findings" and "could not read the findings" stay distinct — an
unreadable thread is exit 6, never an empty finding set. The compact ledger is
what the desk injects into the reviewer and worker prompts so a replacement agent
resumes from it.

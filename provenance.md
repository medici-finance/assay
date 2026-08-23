# Provenance — the ideas Assay borrowed

Assay did not invent most of what it does. The methodology is largely a re-assembly of
older disciplines — industrial control, operations management, sense-making frameworks,
delivery research — applied to fleets of software agents, plus patterns taken gratefully
from the agent-tooling ecosystem. This file is the credit ledger for the borrowings that
ship **in this repository**: each entry names the idea, what we adopted, the source, and
the artifact in this tree where it shows up. When we converged on something
independently and only later found the prior art, we say so — the prior art still gets
the credit.

Scope note: this ledger covers only what is present in the Assay tree. Borrowings that
live elsewhere in the wider house are credited where they live, not here.

Corrections welcome. If we borrowed from you and this file doesn't say so, open an issue.

---

## Methodology & process frameworks

### SCADA / industrial control
- **The idea:** the supervisory display shows *measured* state, never *commanded* state
  (the discipline behind every power grid and pipeline HMI; the Three Mile Island valve
  indicator is the canonical failure). Also *point quality* — an instrument reading
  carries its own trustworthiness — and the alarm-management corollaries below.
- **What we adopted:** "status is a build artifact" — the STATUS board is derived from
  evidence artifacts, never hand-asserted, and an instrument that cannot measure says
  `could-not-check`, never `0`. We built this first-principles and found the lineage
  afterwards — convergent evolution, so the credit runs backwards, but it runs.
- **Source:** SCADA architecture and control-room practice (no single citation; EEMUA-191
  and ISA-18.2 below are the codified standards we then adopted deliberately).
- **Where in Assay:** [`statusgen/`](statusgen/) (the derived `STATUS.md` board; the
  SCADA lens is cited in `bottleneck.go`, `quality.go`, and `alarms.go`), and the
  [three-state instrument rule](docs/three-state-instrument-rule.md).

### EEMUA-191 — alarm systems and control-room design
- **The idea:** an operator has a bounded span of attention; an active-item list past
  that span degrades quality, and a silently truncated list is worse than an alarm.
- **What we adopted:** the Next-up queue overflow is itself an alarm — an explicit
  overflow line, never a silent truncation — and the active-alarm count is banded
  against operator capacity (the classic 7±2).
- **Source:** EEMUA Publication 191, *Alarm Systems — A Guide to Design, Management and
  Procurement* — <https://www.eemua.org/>
- **Where in Assay:** [`statusgen/emit.go`](statusgen/emit.go) (the Next-up overflow
  alarm, cited to EEMUA-191 in the rendered line itself) and
  [`statusgen/alarms.go`](statusgen/alarms.go) (alarm-count band).

### ISA-18.2 — alarm management
- **The idea:** an alarm system is only healthy if you measure it — alarm rate, standing
  (stale) alarms, alarm floods — and shelving an alarm must be a bounded snooze, never a
  mute.
- **What we adopted:** the same KPIs over the FINDINGS register (rate, standing-alarm
  age, flood detection) and bounded shelving for parked findings.
- **Source:** ANSI/ISA-18.2, *Management of Alarm Systems for the Process Industries* —
  <https://www.isa.org/>
- **Where in Assay:** [`statusgen/alarms.go`](statusgen/alarms.go) (`statusgen
  --alarms`; the standard is cited in the code and in the rendered park-expiry line).

### OODA loop (John Boyd)
- **The idea:** Observe → Orient → Decide → Act, with Boyd's insight that the loop's
  integrity — especially Orient — matters more than its speed: act on a corrupted
  picture and speed just gets you to the wrong place faster.
- **What we adopted:** the written invariant that **orient integrity is paramount** —
  the registers the desks orient on must be trustworthy before anything else is
  optimized. The alarm KPIs above exist to protect exactly that layer.
- **Source:** John Boyd's briefings (*Patterns of Conflict*, *Organic Design for Command
  and Control*); widely summarized — <https://en.wikipedia.org/wiki/OODA_loop>
- **Where in Assay:** the orient-integrity invariant carried in the
  [statusgen stream docs](docs/streams/statusgen/) (FINDINGS register state machine);
  the SCADA/OODA lineage note in [`statusgen/alarms.go`](statusgen/alarms.go).

### Theory of Constraints (Eliyahu Goldratt)
- **The idea:** a system has one constraint at a time; find it, exploit it, subordinate
  everything else to it — and when you relieve one station, the constraint moves, so
  keep watching.
- **What we adopted:** the daily bottleneck diagnostic over the brief lifecycle: which
  stage work queues behind, the prescribed ToC action per constraint stage, and
  explicit constraint-shift detection.
- **Source:** Eliyahu M. Goldratt, *The Goal* (1984) and the ToC literature.
- **Where in Assay:** [`statusgen/bottleneck.go`](statusgen/bottleneck.go) (`statusgen
  --bottleneck`; "Theory of Constraints" and the shift rule are cited in the code and
  the rendered report), and the verify-first priority rules in the
  [verify-desk skill](plugins/assay/skills/verify-desk/).

### Cynefin (Dave Snowden)
- **The idea:** situations differ by the knowability of cause and effect (Clear /
  Complicated / Complex / Chaotic / Disorder), and each domain wants a different
  diagnostic; driving Complex work with a single-constraint lens is a category error.
- **What we adopted:** a Cynefin-domain classification over active work, rendered as a
  board lens (never a gate), plus the domain-vs-management-style **mismatch**
  diagnostic — Complex work being driven with ordered tools is surfaced, not assumed
  fine. This is also the switch that says when the ToC diagnostics above apply.
- **Source:** Dave Snowden's Cynefin framework —
  <https://thecynefin.co/about-us/about-cynefin-framework/>
- **Where in Assay:** [`statusgen/cynefin.go`](statusgen/cynefin.go) and
  [`statusgen/cynefinmismatch.go`](statusgen/cynefinmismatch.go) (`statusgen
  --cynefin`; Snowden is cited in the code).

### DORA metrics
- **The idea:** measure delivery *outcomes*, not output — throughput (deployment
  frequency, change lead time) together with stability (change failure rate, recovery,
  rework), always as a system, never cherry-picked.
- **What we adopted:** the metrics mapped onto the brief lifecycle; the verify loop is
  explicitly the Change-Lead-Time fixer and the Change-Failure-Rate sensor (a
  post-merge Verify FAIL is a change failure that pre-merge review missed), and
  oldest-first verify priority is the lead-time signal acted on.
- **Source:** DORA (DevOps Research and Assessment) — <https://dora.dev/>
- **Where in Assay:** the [verify-desk skill](plugins/assay/skills/verify-desk/SKILL.md)
  (the DORA framing is written into its loop) and `statusgen`'s DORA views
  ([`statusgen/roadmapdora.go`](statusgen/roadmapdora.go), `--dora`).

---

## Agent & durable-execution patterns

### Temporal — SLA-timeout escalation
- **The idea:** durable workflow engines long ago named the failure mode of human-in-
  the-loop steps: a blocked decision left silent. Temporal's pattern — a reminder
  window, then an SLA timeout that escalates to a backup approver — makes "quietly
  stuck" a state the system refuses to hold.
- **What we adopted:** decision-owed issues age against an SLA measured from the last
  *human* response (bot activity never resets the clock); under the SLA they classify
  AWAIT, past it ESCALATE — a computed class that sorts the item to the top of the
  operator's lane rather than letting it sit amber.
- **Source:** Temporal — <https://temporal.io/> (its human-in-the-loop and SLA
  escalation patterns).
- **Where in Assay:** the [`issueboard`](tools/desk/cmd/issueboard/) tool
  (`--sla-days`, the AWAIT/ESCALATE classes).

### CIGAR — authority envelopes for delegated work
- **The idea:** when an agent delegates to another agent, the child's capabilities are
  *intersected* with the issuer's own scope — a result never amplifies authority; a
  capability the child requested but was not granted is *recorded as ungranted*, never
  silently absorbed or silently widened; and a selection over inputs records what it
  rejected, so a scoped view is never mistaken for evidence of absence.
- **What we adopted:** all three, as written rules for how dispatched agent work is
  granted, bounced, and read in the brief workflow.
- **Source:** CIGAR (Hashgraph Online) —
  <https://github.com/hashgraph-online/hol-cigar> (Apache-2.0): its handoff
  capability-intersection mechanism and selection manifests.
- **Where in Assay:** the authority-envelope rules in
  [docs/brief-rules.md](docs/brief-rules.md), which cite CIGAR as the source.

---

## Tooling

### gitleaks — secret scanning
- **What it is:** the standard open-source secret scanner.
- **What we adopted:** gitleaks runs the leak-sweep gate over this public tree; the
  repo carries its configuration at the root.
- **Source:** <https://github.com/gitleaks/gitleaks> (MIT).
- **Where in Assay:** [`.gitleaks.toml`](.gitleaks.toml) and the leak-sweep workflows
  in [`.github/workflows/`](.github/workflows/).

---

## Naming

### "Assay"
The name is the metallurgical assayer's verb: to test a precious metal for what it
actually contains, as opposed to what the stamp claims. That is the whole methodology
in one word — evidence over assertion — and we did not coin the word, we picked it for
the metaphor.

---

*Maintained by the Assay desk. Additions must satisfy the ledger's own rule: an entry
names the idea, what we adopted, the source (with link), and a real artifact in this
tree where it shows up.*

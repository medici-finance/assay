# Provenance — the ideas Assay borrowed

Assay did not invent most of what it does. The methodology is largely a re-assembly of
older disciplines — industrial control, operations management, sense-making frameworks,
delivery research — applied to fleets of software agents, plus patterns and code taken
gratefully from the agent-tooling ecosystem. This file is the credit ledger: each entry
names the idea, what we adopted, where it came from, and where it shows up here. When we
converged on something independently and only later found the prior art, we say so — the
prior art still gets the credit.

Corrections welcome. If we borrowed from you and this file doesn't say so, open an issue.

---

## Methodology & process frameworks

### SCADA / industrial control
- **The idea:** the supervisory display shows *measured* state, never *commanded* state
  (the discipline behind every power grid and pipeline HMI; the Three Mile Island valve
  indicator is the canonical failure). Also the *historian* (append-only trend of
  measured state) and *point quality* (an instrument reading carries its own
  trustworthiness).
- **What we adopted:** "status is a build artifact" — the STATUS board is derived from
  evidence artifacts, never hand-asserted; a trend historian; visibly distinct
  unverified-vs-evidence-backed states. We built this first-principles and found the
  lineage afterwards — convergent evolution, so the credit runs backwards but it runs.
- **Source:** SCADA architecture and control-room practice (no single citation; see
  EEMUA-191 and ISA-18.2 below for the codified standards).
- **Where in Assay:** `statusgen` (the derived `STATUS.md` board), the
  [three-state instrument rule](docs/three-state-instrument-rule.md), the evidence
  conventions in [docs/evidence-bundle.md](docs/evidence-bundle.md).

### EEMUA-191 — alarm systems and control-room design
- **The idea:** a human operator has a bounded span of control; an active-item list past
  that span degrades attention, and a silently truncated list is worse than an alarm.
- **What we adopted:** the Next-up queue carries a span-of-control cap, and overflow is
  itself an alarm — never a silent truncation. (The classic 7±2 human band is raised for
  an agent-worked queue, but the overflow-alarms rule is kept intact.)
- **Source:** EEMUA Publication 191, *Alarm Systems — A Guide to Design, Management and
  Procurement* — <https://www.eemua.org/>
- **Where in Assay:** `statusgen` Next-up (span cap + explicit overflow line and lint
  notice).

### ISA-18.2 — alarm management
- **The idea:** an alarm system is only healthy if you measure it: alarm rate, standing
  (stale) alarms, alarm floods.
- **What we adopted:** the same KPIs over the findings register — rate, standing-alarm
  age, flood detection.
- **Source:** ANSI/ISA-18.2, *Management of Alarm Systems for the Process Industries* —
  <https://www.isa.org/>
- **Where in Assay:** `statusgen --alarms` over the FINDINGS register.

### OODA loop (John Boyd)
- **The idea:** Observe → Orient → Decide → Act, with the insight that the loop's
  integrity — especially Orient — matters more than its speed.
- **What we adopted:** the desk cadence is an OODA loop over derived state, and two of
  its written invariants are Boyd restated: observation capacity must scale with action
  capacity, and Orient integrity (an unlied-to board) is paramount.
- **Source:** John Boyd's briefings (*Patterns of Conflict*, *Organic Design for Command
  and Control*); widely summarized — <https://en.wikipedia.org/wiki/OODA_loop>
- **Where in Assay:** the desk-loop structure in the [desk skills](plugins/assay/skills/)
  and the methodology's written invariants.

### Theory of Constraints (Eliyahu Goldratt)
- **The idea:** a system has one constraint at a time; find it, exploit it, subordinate
  everything else to it. Local optimization off the constraint is waste.
- **What we adopted:** the bottleneck diagnostic over the delivery pipeline (where work
  queues between lifecycle stages), and the habit of treating verification — not
  authoring — as the usual constraint of agentic delivery.
- **Source:** Eliyahu M. Goldratt, *The Goal* (1984) and the ToC literature.
- **Where in Assay:** `statusgen --bottleneck` (the factory-floor view); the
  verify-first priority rules in the [verify-desk skill](plugins/assay/skills/verify-desk/).

### Cynefin (Dave Snowden)
- **The idea:** situations differ by the knowability of cause and effect (Clear /
  Complicated / Complex / Chaotic / Disorder), and each domain wants a different
  diagnostic; driving Complex work with a single-constraint lens is a category error.
- **What we adopted:** a `domain:` tag on each unit of work and a board view of the
  distribution — operationalizing "which diagnostic applies": Ordered work gets ToC,
  Complex work gets probe-sense-respond and enabling constraints. The
  domain-vs-management-style mismatch is itself a rendered diagnostic.
- **Source:** Dave Snowden's Cynefin framework —
  <https://thecynefin.co/about-us/about-cynefin-framework/>
- **Where in Assay:** brief frontmatter `domain:`; `statusgen` Cynefin views
  (`cynefin.go`, `cynefinmismatch.go`).

### Estuarine mapping (Dave Snowden)
- **The idea:** in the Complex domain, map constraints and connectors by their cost and
  speed to shift; the cheap-to-shift ones are your levers.
- **What we adopted:** estuarine mapping as the Complex-domain analog of ToC constraint
  analysis — the lever-finding half of the Cynefin lens, so the domain tag produces
  actions, not just a label.
- **Source:** Dave Snowden, Estuarine framework —
  <https://thecynefin.co/estuarine-framework/>
- **Where in Assay:** the lever guidance attached to the Cynefin mismatch diagnostic in
  `statusgen`.

### DORA metrics
- **The idea:** measure delivery *outcomes*, not output — throughput (deployment
  frequency, change lead time) together with stability (change failure rate, recovery,
  rework), always as a system, never cherry-picked.
- **What we adopted:** the five metrics mapped onto the brief lifecycle; the verify loop
  is explicitly the Change-Lead-Time fixer and the Change-Failure-Rate sensor (a
  post-merge Verify FAIL is a change failure pre-merge review missed).
- **Source:** DORA (DevOps Research and Assessment) — <https://dora.dev/>
- **Where in Assay:** the [verify-desk skill](plugins/assay/skills/verify-desk/)
  (DORA framing is written into its loop); `statusgen`'s DORA roadmap view
  (`roadmapdora.go`).

### Temporal — durable-execution vocabulary
- **The idea:** durable workflow engines long ago named the failure modes of scheduled
  and long-running work: schedule **overlap policies** (Skip / BufferOne / AllowAll /
  CancelOther / …), declared retry policies, dedupe-at-start, and SLA-timeout
  escalation to a **backup approver** so a blocked decision cannot sit silent.
- **What we adopted:** the policy *vocabulary* — declaring each recurring loop's overlap
  policy instead of leaving races implicit (our status quo was an undeclared
  `AllowAll`), declared retries, and aged-decision escalation on the issue board.
- **Source:** Temporal — <https://temporal.io/> (schedule overlap policies, signals,
  SLA escalation patterns from its docs and HITL examples).
- **Where in Assay:** concurrency/overlap declarations on the recurring workflows;
  escalation classes in the `issueboard` tool.

### ORCHESTRA framework (Sarkar & Mohapatra)
- **The idea:** a management-literature checklist for leading human-agent work systems —
  nine elements across design, operation, and assurance (objectives, guardrails,
  capabilities, handoffs, escalation, supervision, telemetry, retrospectives,
  auditability).
- **What we adopted:** used as an external audit checklist against the methodology (the
  management lens alongside our throughput and harness lenses), not as an implemented
  system.
- **Source:** Sarkar & Mohapatra, *Leading Human-Agent Teams: The ORCHESTRA Framework
  for Accountable AI Work* — <https://ssrn.com/abstract=6762245> (SSRN preprint, not
  peer reviewed).
- **Where in Assay:** methodology self-review practice; no code artifact.

---

## Agent / LLM-systems patterns

### LangChain — "ambient agents" and the notify / question / review triad
- **The idea:** background ("ambient") agents need exactly three human-interaction
  verbs — *notify*, *question*, *review* — and "notify exists precisely because a
  blocked agent is otherwise silent." The invisible-blocked-agent problem was named and
  framed by LangChain before we got there.
- **What we adopted:** the triad as the basis for tiering agent→human notifications:
  review = gating decisions (always surface), question = collect for the human's next
  pass, notify = FYI stream.
- **Source:** LangChain, "Ambient Agents" —
  <https://blog.langchain.com/introducing-ambient-agents/>; the LangGraph Agent Inbox.
- **Where in Assay:** the notification-tier design of the house's human-oversight
  surfaces; the escalation verbs on the issue board.

### "Less Context, Better Agents" — context hygiene
- **The idea:** for long-horizon tool-using agents, retaining only recent tool
  interactions plus a summary of evicted ones *beats* full-history retention (91.6% vs
  71.0% task completion at −63% tokens in the paper's setting); accumulated history
  actively causes stale-state errors.
- **What we adopted:** the "refresh, don't remember" rule for standing desk loops — a
  desk acts only on state fetched this cycle; remembered state is narrative, never
  decision input — plus the rolling cycle summary as the sanctioned memory channel.
- **Source:** Lodha, Pahlavikhah Varnosfaderani, Chakraborty & Mithal, *Less Context,
  Better Agents: Efficient Context Engineering for Long-Horizon Tool-Using LLM Agents*
  — <https://arxiv.org/abs/2606.10209>
- **Where in Assay:** the loop discipline written into the
  [desk skills](plugins/assay/skills/) (act on freshly fetched board/PR state each
  cycle).

### CIGAR — machine-readable claim authority and honest release gates
- **The idea:** a release ships with a machine-readable claims manifest: fail-closed
  evidence gates marked "required-not-implied," a *prohibited claims* list the
  marketing prose may never exceed, and "machine-readable authorities take precedence
  over prose." Also: an external effect is `UNKNOWN` until non-execution is proven —
  ambiguity is a durable first-class state, never optimistically resolved.
- **What we adopted:** the release-claims manifest + prohibited-claims lint pattern for
  our own release gating; a standing-approvals register (pre-authorized action
  envelopes recorded as first-class artifacts); reinforcement of our three-state
  (`could-not-check` ≠ 0) instrument discipline.
- **Source:** CIGAR (Hashgraph Online) —
  <https://github.com/hashgraph-online/hol-cigar> (Apache-2.0), esp. its
  `release-requirements` manifest.
- **Where in Assay:** release gating and claims discipline
  (`deskrelease` and the release guard checks); the
  [three-state instrument rule](docs/three-state-instrument-rule.md) (independently
  derived; CIGAR is credited as the strongest parallel statement we found).

### Microsoft Agent Framework — the harness lens
- **The idea:** the "agent harness" as a named layer — the batteries-included runtime
  (tool loop, memory, persistence, safety switches) wrapped around a model, each
  capability individually declarable.
- **What we adopted:** the single-run harness vocabulary for auditing our own agent
  runs, and the prompt that led to recording standing approvals explicitly rather than
  leaving them implicit in skill prose.
- **Source:** Microsoft Agent Framework harness —
  <https://learn.microsoft.com/agent-framework/agents/harness>
- **Where in Assay:** methodology vocabulary; the standing-approvals practice noted
  under CIGAR above.

### Wikipedia — "Signs of AI writing"
- **The idea:** a concrete, maintained catalog of AI-writing tells (inflated
  symbolism, stock phrasing, formulaic structure, chatbot artifacts), built by
  WikiProject AI Cleanup for detecting machine prose in the wild.
- **What we adopted:** the pattern catalog as the normative checklist for de-slopping
  house prose before it ships.
- **Source:** <https://en.wikipedia.org/wiki/Wikipedia:Signs_of_AI_writing>
- **Where in Assay:** the house writing/editing gate (see blader/humanizer under
  Vendored code).

---

## GitOps & infrastructure

The hosted side of Assay stands on the CNCF/GitOps ecosystem rather than anything we
built: **Flux** (reconcile-cluster-from-git — the same derive-don't-assert instinct as
`statusgen`, applied to infrastructure; <https://fluxcd.io/>), **Kustomize**
(<https://kustomize.io/>), **SOPS** for platform secrets in git
(<https://github.com/getsops/sops>), **External Secrets Operator** + cloud KMS for
tenant-supplied material (<https://external-secrets.io/>), **cert-manager**
(<https://cert-manager.io/>), **actions-runner-controller (ARC)** for self-hosted CI
runners (<https://github.com/actions/actions-runner-controller>), **Kaniko** for
in-cluster image builds (<https://github.com/GoogleContainerTools/kaniko>), and
**sysbox** for unprivileged container-in-container builds
(<https://github.com/nestybox/sysbox>). **OpenBao** (<https://openbao.org/>) is on the
ledger as the evaluated licensing hedge for secret management.

**gitleaks** (<https://github.com/gitleaks/gitleaks>) does the secret scanning behind
the leak-sweep gate — see [.gitleaks.toml](.gitleaks.toml) at the repo root.

---

## Tooling & vendored code

### blader/humanizer (vendored)
- **What it is:** a skill that rewrites AI-sounding text so it reads naturally without
  changing what it says, built on the Wikipedia "Signs of AI writing" catalog.
- **What we adopted:** vendored wholesale (v2.11.2, MIT, license carried alongside) as
  the de-slop gate in the house article pipeline, run with a genre exemplar as the
  writing sample.
- **Source:** <https://github.com/blader/humanizer> (MIT).
- **Where in Assay:** the house writing/editing pass for articles and long-form docs.

### MakeHuman / MPFB — the character pipeline
- **What it is:** parametric 3D-human generation: the MakeHuman project's CC0 asset
  packs and the MPFB add-on (GPL) inside Blender.
- **What we adopted:** the house 3D-character pipeline builds on an MPFB parametric
  base with MakeHuman CC0 wardrobe/asset packs, rendered headless in **Blender**
  (<https://www.blender.org/>) with **Flamenco** render management
  (<https://flamenco.blender.org/>), and local image generation via **FLUX.1-schnell**
  through the mflux runner. OSS only, no services; a license ledger rides with every
  asset round.
- **Sources:** <http://www.makehumancommunity.org/> (CC0 assets),
  <https://github.com/makehumancommunity/mpfb2> (GPL).
- **Where in Assay:** the house brand/character asset pipeline (mascot and explainer
  imagery).

---

## Writing & craft

### Musical form and Bakhtin — the cadence pass
- **The idea:** long-form prose can carry deliberate musical structure — tempo,
  ostinato (a repeated syntactic frame pulsing under content), register shifts. The
  term for the last one, *heteroglossia* (many voices/registers inside one text), is
  Mikhail Bakhtin's.
- **What we adopted:** a revision-only "cadence pass" over already-verified drafts,
  using exactly that vocabulary; never a generator, never applied to operational text.
- **Source:** common-property musical form vocabulary; M. M. Bakhtin, *Discourse in the
  Novel* (heteroglossia).
- **Where in Assay:** the house long-form editing pipeline (runs before the humanizer
  gate).

---

## Naming & persona

### "Assay"
The name is the metallurgical/assayer's verb: to test a precious metal for what it
actually contains, as opposed to what the stamp claims. That is the whole methodology
in one word — evidence over assertion — and we did not coin the word, we picked it for
the metaphor.

### "Bob," the plumb bob
The coordinating desk persona in the house setup is named **Bob** — for the plumb bob,
the ancient tool that checks true vertical. A desk whose job is checking that reported
state hangs true to measured state gets named after the simplest instrument that ever
did that job.

---

## Evaluations on the record

Not everything reviewed was adopted. Sources we studied seriously and mined for lessons
without taking a mechanism wholesale: Steve Yegge's *The Shape of Things to Come*
(<https://yegge.ai/essays/the-shape-of-things-to-come/>, harness/fleet direction);
Tyler Jewell's multi-agent swarm-patterns corpus (InfoQ/Akka) together with Google
Research's agent-scaling work and Google's ADK docs (coordination-pattern taxonomy);
and the Singapore IMDA Model AI Governance Framework for Agentic AI (governance
checklist). Where a later mechanism traces to one of these, this file gains an entry.

---

*Maintained by the Assay desk. Additions should follow the entry shape above: the
idea — what we adopted — source (with link) — where in Assay.*

### Added
- **Design-approval gate** — a risk-gated brief (`gate: human`, or any `risk` answer
  `yes`) may no longer move to `in-progress` until it cites an approved
  **design-decision record**, so a wrong *design* is caught at authoring rather than only
  when the finished diff reaches the review gate. It is a precondition on the
  `todo → in-progress` edge, not a sixth lifecycle state. Specified in
  `spec/lifecycle-v1.md` §4.4; enforced by `statusgen --lint` (`designgate.go`),
  three-state (an unreadable register is `could-not-check`, never a silent pass).
- **DECISIONS register** — a fourth append-only register of design-decision records under
  `docs/streams/decisions/<slug>.md` with `DR-<slug>` ids: what was decided, the
  alternatives ruled out, the consequences accepted, an ordered `consequence` severity
  axis, and a `human:<name>` `decided-by` stamp (the design-approval authority). Specified
  in `spec/registers-v1.md` §7; a brief cites its record with the new `design:`
  `brief-v1` frontmatter key.
- **Validation named as the third activity** — `docs/validation.md` defines validation as
  distinct from review and verification (did the change achieve the purpose the
  requirement existed for, in its setting), anchored to the REQUIREMENTS register's
  acceptance criteria and to brief-rule 43's dereferencing row as its mechanical floor,
  honest about the intended-use part that remains the adopter's. This closes the gap
  `docs/iso9001-mapping.md` row 8.3.4 named in the repo's own words.
- **Threat model made mandatory for risk-gated briefs** — the `mistake-proofing.md` B5
  pre-mortem is now REQUIRED and RECORDED on a risk-gated brief, each failure mode mapped
  to the Verify row that catches it (`spec/brief-v1.md` §4.7, `docs/brief-rules.md`
  rule 49), wired into the brief's existing single-point-of-failure note rather than a
  second ceremony.

### Changed
- `spec/lifecycle-v1.md`, `spec/brief-v1.md` and `spec/registers-v1.md` carry the new
  gate, key and register with conformance clauses; `docs/iso9001-mapping.md` rows 8.3.2
  and 8.3.4 are updated to what is now true — all three of review, verification and
  validation are named — while still saying, in the same breath, that intended-use
  validation is the adopter's act.
- The design-approval gate is **scoped** (a `gate: model` all-risks-`no` brief is
  untouched) and **grandfathered by authoring date** (it binds only briefs authored after
  the cutover), so a pin bump reds nothing already in flight. The gate proves an approved
  record with a human approver exists; it does not mechanically prove that approver
  differs from the brief's author (the attribution-not-identity limit,
  `spec/lifecycle-v1.md` §7.1.2).

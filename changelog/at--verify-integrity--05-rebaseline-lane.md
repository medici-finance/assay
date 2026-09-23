### Added
- `deskrebaseline` — a verifier verb that turns a provably-intact-but-stale `## Verify` row into a
  one-row re-baseline draft PR instead of the Nth duplicate "stale Verify" issue. It classifies a
  failing row against a narrow safe set (`safe:rename` / `safe:count` / `safe:idiom`) and fails
  CLOSED: a row is re-baselined only when git history POSITIVELY proves the work intact, and
  everything else — a real behaviour change, a deliverable gone with no rename hop, any row of a
  risk-bearing brief, or anything unproven — is refused and filed as today. The verb never merges
  and never lands on `main`; the re-baseline is a PR, reviewed by the reviewer App and merged by a
  human. Dry-run by default (prints the classification and git evidence, creates no branch);
  `--open` pushes the `rebaseline/<stream>-<NN>-row-<K>` branch and opens the draft PR via
  `deskpr create`. The verify-desk skill now runs it on a stale-class FAIL before filing. See
  `docs/rebaseline.md` (verify-integrity/05).

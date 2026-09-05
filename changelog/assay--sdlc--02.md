### Added
- Requirement **traceability** is now checked. `statusgen --lint` runs three corpus-wide checks over the REQUIREMENTS register and the `satisfies:` brief citations: `orphan-requirement` (NOTICE — an `accepted` requirement no brief cites), `untraced-brief` (NOTICE — a forward brief in a `traced: true` stream that cites nothing), and `dangling-satisfies` (PROBLEM — a `satisfies:` naming a `REQ-<slug>` no register entry defines). The two NOTICEs never change the exit code, so a corpus authored before the register existed is never red-gated.
- `statusgen --requirements-rollup [--since <date>] [--json]` — the per-release ask→work→evidence rollup: each requirement, its acceptance criteria, the briefs that cite it with their board status and Evidence, and a three-state verdict (`satisfied` only when every backing brief is `done`, else `partial` or `could-not-check`). It reports what was authored, not re-measured — an input to `--export-evidence`, not a second bundler.
- Stream READMEs may opt into the `untraced-brief` check with a `traced: true` frontmatter key; absent it, the check never fires over that stream.

### Changed
- The `satisfies:` citation and the REQUIREMENTS traceability checks move from **reserved, not gating** to gating (spec `registers-v1.md` §6.5, `brief-v1.md` §3.3). Consumers pick the checks up on their next `.assay-versions` statusgen pin bump.

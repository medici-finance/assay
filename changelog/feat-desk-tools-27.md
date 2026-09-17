### Added
- `deskpr create --check` runs every LOCAL gate a real create would run — flags, branch
  state, the `Brief:`/`Issue:` trailer, the secret scan, the public-repo self-containment
  scan, the push-transport gate — and stops before minting a token or opening any
  connection; `update` and `edit` gain the same flag for the gates that do not require the
  forge-held PR body. `deskreply`'s `--dry-run` is widened from the `--workpad` path to the
  plain reply path.

### Changed
- The measured top body/schema refusal classes in `deskpr` and `deskreply` now name the
  offline check that would have caught them for free (`deskpr … --check`, `deskreply …
  --dry-run`), continuing the same hint `deskpost`'s refusals already carry.

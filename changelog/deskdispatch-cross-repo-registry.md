### Added
- `deskdispatch` resolves a cross-repo item's deliverable repo through the alias registry (`graph-repos.yaml`) — from the brief's `deliverable_repo`/`homed-in` or an `<alias>:` item-key prefix — and hard-fails before anything durable when that repo is not `--root`'s own; an unregistered alias is refused by name. The claim, the token and the prompt's `deskpr create --root <tracking checkout>` hint follow the resolved repo.
- `deskdispatch` resolves `--brief` once (absolute, else under `--root`, else under `--claim-root`) and feeds that one file to the human-gate detection, the decision script and the prompt, so a `gate: human` brief kept in the tracking checkout still fires its decision gate; a `--brief` that resolves nowhere is refused. Registry alias keys, `repo:` values and `self:` are grammar-checked before use.
- `deskdispatch --rework`: a rework row whose PR is already MERGED dispatches as a follow-up on a new branch instead of re-cutting the merged branch.

### Changed
- A MERGED PR found by the phantom check is refused as delivered (pointing at `--rework`) rather than with a "resume" hint; tests now pin that the phantom check precedes the admission gate, the token mint and the claim.

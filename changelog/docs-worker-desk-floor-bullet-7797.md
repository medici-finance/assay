### Changed
- `worker-desk`: the below-the-floor bullet now matches `pr-review-desk`. A released dispatch claim's stamp ages out only on `deskpost` (a review verdict and `deskpost ready`); `deskflip` cannot read claim liveness and leaves it standing. A ready-flip escalates by spawning a sub-agent on a named strong-tier model id to run `deskflip`, handing off only when none is configured (#7797).

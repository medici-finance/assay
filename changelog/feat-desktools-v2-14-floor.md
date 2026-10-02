### Added
- Register 26 inherited desk-tool and statusgen behaviors with parent-counterfactual evidence, a manifest guard, and fixture-only regression entry points.
- Stage the additive statusgen full-test CI case; its enforcement remains held until the maintainer applies the workflow patch.

- Check descendant files for directory readers on both workflow events, and retain rejecting controls. Give preserved shell fixtures bounded runtime headroom with a cancellation control.
- Run every floor fixture that starts git, directly or through a shell, without the caller's GIT_* variables, so a GIT_DIR exported by a git hook cannot redirect fixture writes; a hostile-GIT_DIR control and a structural guard pin it.
- Start every floor runner's go tool through one wrapper that clears the caller's GIT_* variables and global git config, so reused manifest rows cannot write to a repository an exported GIT_DIR names; a runner-level hostile-GIT_DIR control, a choke-point guard and a mutation mode pin it.

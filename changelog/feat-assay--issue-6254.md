### Changed

- `deskpost ready` now refuses a head that the post-flip check already returned to draft, the same way `deskflip` does. A reviewer reaching for `ready` no longer re-flips a head that would only be reverted again; a new head clears the refusal. The check reads the change's comment thread and can only refuse: no comment can admit a flip, and a thread that cannot be read, or that runs past 2000 comments, stops the flip as could-not-check.

### Fixed
- **`deskdispatch --kit review` now emits the reviewed head-fetch refspec in the target
  repo's own forge shape, so a GitLab reviewer can check out the MR head.** The review kit
  hard-coded the GitHub coordinate `git fetch origin pull/<N>/head`; on a GitLab-served repo
  the head is advertised at `merge-requests/<iid>/head` (the MR's own `pipeline.ref`), so a
  reviewer that followed the prompt verbatim fetched a ref that does not exist and reviewed
  the worktree's `origin/main` cut rather than the change. The refspec now follows the forge
  resolved for the target repo — GitHub `pull/<N>/head`, GitLab `merge-requests/<N>/head` —
  resolved before the dispatch claim is taken, so a repo whose forge cannot be determined
  refuses the review dispatch (could-not-check) rather than handing over a coordinate the
  reviewer cannot use. The worker/implementer dispatch path, which emits no forge-shaped ref,
  is unchanged. Part of the GitLab-adopter portability family.

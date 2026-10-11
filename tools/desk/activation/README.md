# Native Windows maintenance CI — staged activation

`git-maintenance-windows.yml` is the complete intended native Windows job for issue
#2513. It is inactive here. GitHub runs workflow files only from
`.github/workflows/`; the worker App deliberately lacks permission to write there.
The driver approved staging this file for a workflow-capable human to activate.

From this repository root, copy the reviewed file verbatim:

```sh
cp tools/desk/activation/git-maintenance-windows.yml .github/workflows/git-maintenance-windows.yml
git add .github/workflows/git-maintenance-windows.yml
git commit -m "ci: activate native Windows Git maintenance validation"
```

Publish that commit through the human's workflow-authorized transport. The job
checks out the PR's exact head, verifies the pinned official Git-for-Windows
archive checksum and native platform, and runs storage, child-custody and command
fixtures without forge credentials. Native Windows validation is **could-not-check**
until this activation is published and the job succeeds at the reviewed head.
Do not treat cross-compilation as native execution or silently waive that gate.

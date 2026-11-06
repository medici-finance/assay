---
id: DR-transition-netns
date: "2026-11-05"
title: "Run protected acceptance rows inside a closed container bracket with admission on the dispatch host"
consequence: major
decided-by: "human:<name>"
ruling: "https://github.com/medici-finance/assay/issues/5522#issuecomment-6672359311"
alternatives:
  - "Add a runner-runtime dispatch target under desktools-v2/81. The closed bracket allows the existing dispatch host to admit the run and is the approved scope for this brief."
  - "Require protected rows to execute on the dispatch host. A Darwin dispatch host cannot provide the Linux network namespace required to witness these rows."
accepted:
  - "The dispatch host opens and closes the bracket and checks approval and the exact execution plan at both steps. The network-isolated container executes the bound plan with the image digest recorded."
  - "The runtime recheck and the existing witness credit requirements continue to apply. Independent verification and human acceptance at done are separate gates."
---

The ruling selects reading 1 for desktools-v2/81. It applies the existing
[container witness decision](DR-container-witness.md) to protected acceptance rows.

---
id: I-fixture-intake-row
date: 2026-07-25
title: Fixture intake entry — belongs to the stub root's register only
disposition: watching
---

A synthetic intake entry in the SECOND root's own intake register, so the stub
root's intake-queue board line and untriaged-age alarm are computed from this
root's entries rather than the primary root's.

`disposition: watching` on purpose: `new` would age past the untriaged threshold
and fire a NOTICE that grows louder every day the fixture sits in the tree, which
would train readers to ignore the alarm the fixture is meant to exercise.

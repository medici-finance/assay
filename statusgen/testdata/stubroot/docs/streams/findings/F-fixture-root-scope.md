---
id: F-fixture-root-scope
date: 2026-07-25
title: Fixture finding — belongs to the stub root's register only
affects: ["multiroot-fixture/brief-01"]
resolved: false
---

A synthetic finding in the SECOND root's own findings register. Its `affects:`
names a stream that exists only under this root.

It is a cross-root bleed detector, and a self-enforcing one: a finding whose
`affects:` names a stream statusgen cannot resolve is a hard PROBLEM. So if the
stub root's register were ever fed into the primary root's check pass, the
primary root's `--lint` would go red with "Affects references unknown stream
multiroot-fixture" rather than failing quietly. The bleed cannot be silent.

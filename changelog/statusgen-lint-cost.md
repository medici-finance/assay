Issue: #4360

- Reduce repeated ordering-gate regex work in statusgen lint with conservative literal-prefix rejection. The original ordered matcher, Unicode behavior, negations, waivers, source-edit detection and every other lint control remain authoritative. Fixture parity, focused race/vet and a healthy five-caught local mutation sweep qualify the change.

The matched matcher benchmark improves 86.7%; five noisy full-command pairs do not establish a firm whole-command saving (paired median wall reduction 2.40%, process-tree CPU effectively neutral). Detailed limits are in docs/statusgen-lint-performance.md. No historical performance record is relabeled and no recording, consent or profile control changes here.

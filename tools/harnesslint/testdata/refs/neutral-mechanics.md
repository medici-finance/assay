# Fixture neutral reference — declared out of the bindings matrix

<!-- assay:harnesslint non-matrix-reference — fixture harness-neutral mechanics, not a per-harness capability binding -->

This file deliberately resolves NO capability and carries NO per-skill degradation
cell. It is in the fixture so the shipped-fixture-is-clean test exercises the
declared skip end to end: without the declaration these two absences are eight
violations, with it they are one announced skip.

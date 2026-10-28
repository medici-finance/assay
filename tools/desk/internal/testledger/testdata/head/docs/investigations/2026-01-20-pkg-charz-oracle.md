# Fixture oracle for pkg/charz

## 1. Intent

TestFixtureUntriaged is named here, outside the triage table: that is not a row.

## 2. Failure modes

| test | why it is here |
|---|---|
| TestFixtureUntriaged | a row outside section 3 is not a triage row |

## 3. Current behaviour

| test | verdict | reason | maps to |
|---|---|---|---|
| `TestFixtureTriaged` | keep | the split on a run of spaces is intent | S-fixture |

## 5. Acceptance

| # | Command | Expect |
|---|---------|--------|
| 2 | `go test ./pkg/charz/ -run '^TestFixtureUntriaged$'` | named in a cell, not the first |

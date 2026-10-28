# Cursor smoke run — <date> — cursor-agent <version>

<!-- Scaffold. Copy to docs/cursor-smoke-runs/<date>-<cursor-version>.md and fill every
     field. Every Result line below is an unfilled placeholder: replace it with exactly one
     of PASS, FAIL or BLOCKED, and only after pasting the step's transcript excerpt. -->

Runner: <the maintainer, or a session the maintainer sanctioned>
Cursor version: <output of `cursor-agent --version`; the planned build is 2026.09.02>
Surfaces exercised: headless `cursor-agent -p` (primary); IDE agent (Steps 2, 4, 5)
Model policy: `auto` — the model actually served is recorded per step below, read from
`--output-format stream-json`, or marked `model: unrecorded` where the stream does not report it
Credentials: API key held in a local credentials file on the runner only; not in this log
Bundle version under test: <plugins/assay/SOURCES.yaml bundle-version> (plugin manifest <plugins/assay/.claude-plugin/plugin.json version>; release tag installed in Step 1)
Adopter repo under test: <scratch-adopter-repo: not yet named>

## Run conditions and deviations (stated up front)

<Anything that differs from the protocol: flags used for each permission posture, config
isolation, steps run in a different order. "None" if none.>

Step 1: Fresh install per the adopt path
  Surface: headless
  Model served: <model from stream-json | model: unrecorded>
  Action taken: ...
  Transcript excerpt:
    <paste — the dry-run, the placement and the `--check` exit 0>
Result: <PASS | FAIL | BLOCKED>
  Issue (if FAIL): owner/repo#<n> (harness-portability/12)

Step 2: Resident rules present without manual pasting (both surfaces)
  Surface: headless AND IDE (one excerpt each)
  Model served: <per surface>
  Action taken: ...
  Transcript excerpt:
    <paste — each session stating rule 3 unprompted>
Result: <PASS | FAIL | BLOCKED>
  Issue (if FAIL): owner/repo#<n> (harness-portability/12)

Step 3: Invoke each skill by name (headless) -> body loads
  Surface: headless
  Model served: <model | model: unrecorded>
  Action taken: invoked every assay:* skill in Step 3's list by name
  Transcript excerpt:
    <paste one identifying line per loaded body>
Result: <PASS | FAIL | BLOCKED>
  Issue (if FAIL): owner/repo#<n> (harness-portability/12)

Step 4: Auto-trigger probe
  Surface: IDE (pass condition); headless (observation)
  Model served: <per surface>
  Action taken: ...
  Transcript excerpt:
    <paste — IDE auto-load, and the headless outcome either way>
  Headless auto-trigger posture: <fired | did not fire — state it>
Result: <PASS | FAIL | BLOCKED>
  Issue (if FAIL): owner/repo#<n> (harness-portability/12)

Step 5: Dispatch probe -> observed runs / degrades / refuses matches the ruled matrix
  Surface: IDE and headless
  Model served: <per surface>
  Action taken: ...
  Observed outcome per surface: <runs | degrades (serial, stated) | refuses>
  Transcript excerpt:
    <paste — N>=2 items, each completed with its own worktree / evidence / review, and the
    session's own statement of any degradation>
Result: <PASS | FAIL | BLOCKED>
  Issue (if FAIL; floor breach if an item was dropped or a guarantee weakened): owner/repo#<n> (harness-portability/12, or /04 for a floor breach)

Step 6: Isolation probe -> refuses where it cannot isolate, runs where it can
  Surface: headless
  Model served: <model | model: unrecorded>
  Flags / posture, blocking: <exact flags>
  Flags / posture, permitting: <exact flags>
  Action taken: ...
  Transcript excerpt:
    <paste — the refusal under the blocking posture, the worktree under the permitting one>
Result: <PASS | FAIL | BLOCKED>
  Issue (if FAIL, floor breach): owner/repo#<n> (harness-portability/04)

Step 7: Evidence-discipline probe -> real command evidence recorded
  Surface: headless
  Model served: <model | model: unrecorded>
  Action taken: ...
  Transcript excerpt:
    <paste — command + exit code + output>
Result: <PASS | FAIL | BLOCKED>
  Issue (if FAIL, floor breach): owner/repo#<n> (harness-portability/04)

Step 8: Full desk loop -> draft PR, then one pr-review-desk review cycle
  Surface: headless
  Model served: <model | model: unrecorded>
  Action taken: ...
  Draft PR: <URL and number on the adopter repo under test>
  Diff summary: <files and size — not empty or placeholder>
  Reviewer verdict: <verdict from the one pr-review-desk cycle>
  Transcript excerpt:
    <paste — dispatch, the draft PR URL, and the review verdict>
Result: <PASS | FAIL | BLOCKED>
  Issue (if FAIL): owner/repo#<n> (owning surface of the skill that stopped)

Human gate sign-off: <account> confirms — real Cursor install, version as listed, an excerpt
per step, a genuine draft-PR reference for Step 8, degradations observed match the ruled
matrix, failures routed to issues.

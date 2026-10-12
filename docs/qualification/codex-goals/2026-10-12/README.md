# Native Codex Goal qualification — 2026-10-12

Primary evidence for PR #2697, addressing reviews
[5493612049](https://github.com/medici-finance/assay/pull/2697#pullrequestreview-5493612049)
and [5493626958](https://github.com/medici-finance/assay/pull/2697#pullrequestreview-5493626958).
These are retained event and tool records, with the extraction limits below; the
worker's narrative and the probe driver's PASS line are not substitutes for them.

## What was exercised

The native `codex app-server --stdio --enable goals` engine executed the exact
[generated bootstrap](generated-bootstrap.txt) with a local harmless
`assay:worker-desk` skill. Its command output includes that complete stub. The stub
increments `counter.txt` once per authorized turn. Human steering instead writes
`STEERING_ACCEPTED`, pauses the Goal and requests a readback. No real queue,
reviewer, installed desk session, cockpit or service was started or restarted.

The native tools here are exposed through `exec` as `tools.get_goal`,
`tools.create_goal` and `tools.update_goal`. The retained native tool inputs show
those calls and their arguments; the RPC records independently show resulting
Goal state. This is more than a prompt asking the model to continue.

| Requirement | Primary artifact and source lines |
|---|---|
| Runtime and clean source revision | [bootstrap-tools-rpc.jsonl](bootstrap-tools-rpc.jsonl), 1, 10, 260: codex-cli 0.162.1, gpt-6-astra, reasoning effort high, same clean source before/after |
| Exact generated bootstrap submitted | Same RPC artifact, 16; byte-identical to `generated-bootstrap.txt` |
| No existing Goal; native creation with no budget; active readback | [bootstrap-tools-native.jsonl](bootstrap-tools-native.jsonl), 13–19: `get_goal({})`, exact `create_goal({objective:…})`, then `get_goal({})`, with actual outputs |
| Role admitted only after readback | Same native artifact, 23–31: complete skill read, then counter command and `count=1` output |
| Automatic second turn after final, with no second client start | RPC artifact, 172 first turn completed, 174 distinct second turn started, 245 second turn completed; only client `turn/start` is 16. Native artifact, 37–38 and 54 independently records both turns |
| Human steering accepted; native pause and readback | RPC artifact, 175 steering input, 207–208 paused Goal; native artifact, 45–48 actual command, `update_goal({status:"paused"})`, `get_goal({})` and outputs |
| Cleanup completed | RPC artifact, 249–257: supported pause, clear result, then `goal:null`; 259 records process/workspace removal |
| Goals disabled with role withheld | [goals-disabled.jsonl](goals-disabled.jsonl), 8–14, 104–116: feature-disabled RPC error, bootstrap refusal, no role marker. This is unavailable-tool refusal, not a synthetic creation-failure injection |
| Existing paused Goal preserved | [existing-paused.jsonl](existing-paused.jsonl), 8–16 and 135–145: before/after Goal objects equal, including objective, status, budget, usage and timestamps; no role marker; cleanup reads `goal:null` |

The final follow-up probe ran with a 180-second bound plus at most 15 seconds for
cleanup RPC, completed successfully, and retained the native rollout before
removing its own isolated workspace and authentication copy. Its native tool
records close a real gap in the earlier app-server-only logging. Drivers are
included as text snapshots for audit, not as a new automated CI qualification:
[follow-up](bootstrap-tools-driver.py.txt),
[original bootstrap](original-bootstrap-driver.py.txt),
[engine control](engine-control-driver.py.txt), and
[admission controls](admission-driver.py.txt). Authentication material itself is
not included. The work sandbox reports network access false; the model runtime
still used its ordinary authenticated model service.

## Source and runtime provenance

The follow-up probe ran at clean source revision
`4e269b710187624f34b987f8e88fd74442450a9e`, before and after. The resolved runtime
binary SHA-256 was
`74c6a6d263f40d65f99c27a815275f5b4ed0bcc2baecc396b4cf3255498bdfc0`.
The runtime reports service tier null; no stronger runtime tier is inferred.
The app-server labels its source `vscode`; this probe did not launch that editor.

The earlier original-bootstrap probe ran against the working implementation
before its first source commit, while HEAD was the scaffold
`2d5103f399fd195034d1ca4ff3997c7e53eb2a62`. It did **not** run at the later reviewed
head. The retained [prompt generator](prompt-generator.go.txt) contains the exact
`codexRolePrompt` and `codexStandingPrompt` functions. Both functions byte-match
implementation commit `eebba5f39`, reviewed source `4e269b710`, and the source after
the clean main merge used for this evidence publication. Re-running that retained
generator reproduced the bootstrap byte-for-byte (SHA-256
`4fc5ec0854e693f024d9fcbbf0f084aff57c12bc8833718234e2de03386239be`).
The disabled and paused controls reused that exact bootstrap. Original logs do not
contain a runtime binary hash or source before/after receipt; those stronger
receipts belong only to the follow-up probe.

## Original records and logging limits

[original-bootstrap.jsonl](original-bootstrap.jsonl) preserves the first actual
bootstrap probe: one client start, two native turn IDs, role admission and steering.
Its selector-based reader buffered the second completion until cleanup: the
`OBSERVED` counter at line 268 says one completed turn; the actual second
`turn/completed` is line 273. This is a driver accounting limitation, not evidence
of a missing second native turn. Lines 269–272 sent cleanup requests before that
buffered completion was drained. The server returned `cleared:true` at 277 but the
log missed the final get response. Final-null readback for that original run is
**could-not-check**. It is not reconstructed from another run.

[engine-control.jsonl](engine-control.jsonl) is an earlier, separate bespoke-prompt
engine control, with two native turns and final-null cleanup. It is not evidence
that the generated bootstrap ran. The follow-up actual-bootstrap probe separately
provides both native tool arguments and final-null cleanup in one retained run.

The Goals-disabled probe cannot inspect or clear Goal state through a disabled
RPC. Its cleanup records that same unavailable-feature error and terminates its
isolated process/workspace; it does not claim a successful null readback.
The paused control records actual RPC inputs and before/after objects, but its
original app-server log does not expose the model's internal Goal tool calls.

## Extraction and independent inspection

[manifest.json](manifest.json) gives each retained raw log's SHA-256, original
basename and line count, and the public artifact's SHA-256. Raw logs and the reverse
redaction map remain private. Hashes identify the retained inputs; they do not
independently certify an unshared input's authenticity.

Every source line maps to exactly one JSONL record at the same line number, with
`sourceLine`. No client request line is dropped. All relevant RPC parameters,
responses, Goal objects, final messages, command payloads/outputs and turn events
are retained. Stable `opaque-N` aliases replace opaque identifiers, including
identifiers nested in native tool output strings. The follow-up RPC and rollout
share the same aliases, preserving cross-file correlation. Each isolated absolute
workspace root becomes `<probe-root>`; relative role and marker paths are retained.
The incidental host-client name/version in the runtime user-agent is replaced by
`<host-client>`, while Codex version and OS/architecture remain.

Streaming deltas are replaced by omission records because completed payloads are
retained. Account/rate-limit, token-usage, remote-control, skill-change and MCP
startup notifications, plus stderr diagnostics, are individually marked omitted.
Reasoning items, including those nested in completed turns, retain only a type and
an explicit omission marker: no native reasoning text is published. The rollout
extract retains full native tool call/output records and task start/complete
events; every other rollout line is an explicit omission record. That excludes
session/account metadata, system/developer context, world state and reasoning.
Native tool arguments, results and their time/turn metadata are not paraphrased.
The manifest counts omitted lines by reason; nested reasoning omissions remain
visible in their original event records.

Native `input_text.text` values that themselves encode JSON are represented as
`{"encoding":"json","value":…}` objects. This removes one redundant serialization
layer; it is an extraction representation, not the native RPC schema. The
manifest lists every transformed source line and output index. In particular,
source line 26 now exposes the role-stub command output as a structured value.
No event, decoded field value, identifier alias or chronological position changes.

The validator reverses this transformation and requires the SHA-256 of the entire
reconstructed JSONL to equal the [previously published artifact at
f99f3e4f](https://github.com/medici-finance/assay/blob/f99f3e4f76388852e6f797ace830c167a6fc2111/docs/qualification/codex-goals/2026-10-12/bootstrap-tools-native.jsonl).
Thus the equivalence check covers all 57 records, not only the changed payloads,
and can be checked against an independently accessible prior public revision.
The original raw-log hashes and omission accounting remain unchanged.

Run `python3 validate.py` in this directory for hash, chronology, bootstrap,
readback, control and extraction-accounting assertions. It validates retained
artifacts only; it does not start Codex, contact services, or manufacture a new
native qualification result.

## Scope limits

This qualifies the isolated native engine executing the generated host bootstrap.
Fixture tests separately cover host argv, explicit ticks, launcher routing, Claude
invariance and the eight regression mutations. It does not qualify a deployed
interactive cockpit/TUI, image-owned container entrypoint, permanent service,
process restart, or timer. Native Goal continuation is not a durable scheduler.
Reviews [5494842031](https://github.com/medici-finance/assay/pull/2697#pullrequestreview-5494842031)
and [5494861116](https://github.com/medici-finance/assay/pull/2697#pullrequestreview-5494861116)
accepted this native evidence and resolved S1 and F-codex-goal-acceptance. Their
remaining validator-routing findings are separate from native qualification.

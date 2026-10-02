---
id: DR-cellctl-go-port
date: "2026-09-16"
title: "cellctl becomes a Go binary in tools/desk/cmd/cellctl, shipped in the desk-tools tarball, with the bash kept as the oracle until parity"
consequence: major
decided-by: "human:<name>"
ruling: "https://github.com/medici-finance/assay/issues/1193#issuecomment-5699633449"
alternatives:
  - "Keep bash and only fold the verbs in — the other answer the decision issue put to the operator (#1193 `## What is needed`, item 2: 'proceed, or keep bash and only fold the verbs in'). Ruled out by the ruling, for the reasons #1193 `## Why a Go port rather than more bash` gives: `tools/cellctl/cellctl` is ~2,100 lines of bash that one laptop already runs as three divergent copies; `deskkit` already carries the forge resolver, role-token custody, roster parsing and the writeguard that bash cellctl re-implements slices of, and the bridge exists because extending the bash was harder than wrapping it; and the windows-port stream needs a native-Windows cellctl that bash cannot reach."
  - "Keep the out-of-tree bridge — the ~170-line Python bridge plus a `.zshrc` shell function shadowing `cellctl` that #1193 `## What happened` describes, a fourth implementation in a fourth language. Ruled out: the operator disabled the `.zshrc` edit and directed that the bridge's functionality be folded into `cellctl` and that `cellctl` become a Go tool 'rather than a shell script with Python forks around it'."
accepted:
  - "The bash script stays in the tree as the oracle until parity: for the life of the port two implementations of the launcher coexist, and the Go side is held to byte-parity with the bash on a dry-run matrix (brief desk-containers/10 `## Context`, the oracle and parity-harness facts). Its removal is a later brief."
  - "The replacement is irreversible once shipped (brief desk-containers/10 `gate-why`): once a release ships the Go binary under the tarball's `cellctl` entry, every pinned install that upgrades runs the port, and a boot that goes wrong goes wrong on every cell at once. That cutover stays a separate human sign-off on the brief; this record does not decide it."
  - "The port is bound to `deskkit`: forge resolution, role-token custody, roster parsing and the writeguard are called from `deskkit`, never re-derived (brief desk-containers/10 `## Context`, 'deskkit reuse is a requirement, not a preference')."
  - "Building cellctl now needs a Go toolchain: `docs/cellctl.md` §Install's 'It is a shell script, not a Go build … It does not need a Go toolchain' retires (brief desk-containers/10 `sources:`)."
  - "Native Windows is a named consequence, not delivered here: `deskkit` does not compile for `GOOS=windows` until windows-port/00 lands, so a Go cellctl inherits that dependency and the windows-port stream owns building and proving a Windows cellctl (brief desk-containers/10 `## Context`)."
---

**Ruling recorded (2026-09-16): go port.** The operator ruled on the decision issue
[#1193](https://github.com/medici-finance/assay/issues/1193) under the operator's own login —
the [ruling comment](https://github.com/medici-finance/assay/issues/1193#issuecomment-5699633449)
(2026-09-16T15:00:55Z) reads `ratified - go port`. This record transcribes that ruling and the
text that issue put in front of the operator into the register for the lifecycle's
design-approval gate (`spec/lifecycle-v1.md` §4.4). It does not mint a new decision: the human
act is the operator's comment on #1193, not this file.

**The decision.** `cellctl` is ported from the bash script to a Go program at
`tools/desk/cmd/cellctl`, built and shipped in the desk-tools tarball and pinned by sha256 like
every other desk verb, with the bash kept as the oracle until the Go side reaches parity
(#1193 `## What is needed`, item 1(b); `docs/streams/desk-containers/brief-10-cellctl-go-port.md`).

**The constraint behind it.** One laptop already runs three divergent copies of the shell
launcher, and the first cell it could not express grew a fourth implementation in a fourth
language. Every other desk verb is a Go binary in one tarball, built on the same `deskkit` the
launcher re-implements slices of by hand (brief desk-containers/10 `why:`).

**What this record does not decide.** It scopes to the port only. It does not decide the
CUTOVER — when the tarball's `cellctl` entry switches from the script to the binary — which
stays the human gate on brief desk-containers/10 (its `## Human decision`). It does not decide
the fold-in of the host-local cell verbs or the bridge's retirement, which #1193 also asked for
and which are their own briefs. And, per the register's own limits, it records that the
alternatives were weighed and names a human approver; whether the port is correct is the review
gate's judgement and the parity matrix's.

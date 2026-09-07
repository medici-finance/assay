# claimguard

A heuristic check for one specific shape of unresolved outward claim: a
named third-party product asserted with **no citation at all** — not a dead
or wrong-host link (that is a link-resolution lint's job), a name with
nothing to dereference in the first place.

## Why

A model Review gate can read a DoD line like "named-competitor claims must
carry sources" and pass a document where the sources are simply absent for
one claim — nothing failed to resolve, because nothing was ever cited. A
confidently-worded invented name reads exactly like a real one until someone
tries to look it up.

## What it does

`claimguard` scans markdown for two candidate shapes:

- **CamelCase-shaped tokens** (`VerdictCI`, `OpenSky`) — an internal
  capital letter after at least one lowercase letter.
- **ALL-CAPS acronym-shaped tokens**, 3-8 letters (`SLAW`) — two-letter
  acronyms are excluded; the false-positive rate there is too high.

For each candidate not on the allowlist, it looks within a token window
(default 12 tokens either side) for a resolving URL: a markdown link whose
anchor text *is* the candidate, or a bare `http(s)://` URL nearby. No
resolver found within the window → flagged.

```
go run . [--allow-file words.txt] [--window N] <path>...
```

Each path is a markdown file or a directory (scanned recursively for
`*.md`). Exit 1 if anything is flagged, 0 otherwise. Nothing here touches
the network — it does not check that a cited URL is live or on the right
host; that is a separate, already-covered concern.

## Known limitations (declared, not hidden)

- **No entity recognition.** This is regex-shaped pattern matching, not NLP.
  It will miss a plain-English or single-Title-Case product name
  ("Verdict", "Acme") entirely — deliberately: distinguishing those from an
  ordinary sentence-initial or headline word without a prohibitive
  false-positive rate needs more than a lint can offer for free. It narrows
  to the shapes the triggering evidence actually showed.
- **Allowlist tuning is per-corpus.** The built-in list
  (`allowlist.go`) covers only extremely common tech acronyms and a handful
  of ubiquitous CamelCase product names. Running this over a real corpus for
  the first time will surface false positives (a house term, an internal
  codename) that belong in a `--allow-file`, not in a code change here.
- **Not wired into any CI gate.** This tool proves the mechanism; deciding
  which report-repo docs it should gate, at what stage (author-time,
  PR-time, review-gate time), and how findings surface to a human is a
  separate, judgment-heavier decision left to whoever adopts it for a given
  gate.

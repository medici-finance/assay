- Rewrite the Context `files:` block of 66 briefs whose wrapped or prose declarations the
declared-scope parser refused, so each now yields its declared scope. Only that block is
edited; Verify, Evidence and `version:` are untouched, and the parser is not loosened.
Refused briefs drop from 265 to 199 (not done: 176 to 110).

- Add an authoring-time ratchet over the brief corpus (`TestBriefScopeRatchet`): every
brief under `docs/streams` is parsed with `BriefDeclaredFiles`, and a refusal that is not
on `docs/streams/declared-scope-refused.txt` fails the desk test shard, as does a listed
brief that now parses or is gone, so no stale entry survives. Adding an entry is not
refused mechanically; review of the list's diff is the check on an addition.

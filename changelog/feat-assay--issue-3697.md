- Rewrite the Context `files:` block of 67 briefs whose wrapped or prose declarations the
declared-scope parser refused, so each now yields its declared scope. Only that block is
edited; Verify, Evidence and `version:` are untouched, and the parser is not loosened.
Refused briefs drop from 265 to 198 (not done: 176 to 109).

- Add an authoring-time ratchet over the brief corpus (`TestBriefScopeRatchet`): every
brief under `docs/streams` is parsed with `BriefDeclaredFiles`, and a refusal that is not
on `docs/streams/declared-scope-refused.txt` fails the desk test shard, as does a listed
brief that now parses or is gone, so the list only shrinks.

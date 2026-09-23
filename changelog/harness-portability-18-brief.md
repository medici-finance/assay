### Added
- harness-portability/18 (spec): a deterministic skillslint routing check. It scores every pair of
  skill descriptions with TF-IDF cosine and prints a NOTICE for any pair that competes for the
  same prompts. It also scores a checked-in fixture of positive and negative prompts per skill
  for rank-1 routing accuracy against a recorded baseline. That baseline starts as a NOTICE and
  becomes an exit-1 ratchet one release later. The check also runs over an adopter's own skills
  through `--skills-dir`.

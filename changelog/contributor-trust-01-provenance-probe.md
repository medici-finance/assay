### Added
- New `deskprovenance` verb and `internal/deskkit/provenance.go` gather a fixed set of
  mechanical signals about an unknown contributor's pull request (account age, fork-to-PR
  elapsed, cross-repository burst, prior merged/closed ratio, body-shape similarity, commit
  signature, build/dependency paths touched) and render them as a neutral, facts-only card —
  no score, no rating, no verdict. See `docs/contributor-provenance.md`. Posting the card
  publicly is pending the human ruling recorded on `docs/streams/decisions/DR-provenance-card.md`
  (contributor-trust/01).

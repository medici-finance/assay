### Fixed
- `deskclaim-ref` accepts the `--flag=value` spelling of its claim flags (`--branch=-` for a branchless run), and `progress` advances a plain physical claim key even when the repository publishes no owning root, instead of refusing with "no published owning root". A claim that carries a negotiated subject is still refused by the legacy writer.

### Fixed
- `deskclaim-ref` checks the checkout’s origin before choosing a forge host, including when `--repo` supplies the target repository. Forge host matching accepts ASCII case differences and refuses non-ASCII hostnames.

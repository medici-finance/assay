### Fixed
- `deskclaim-ref acquire` names the server's refusal when a claim create is rejected and the follow-up read finds no holder, instead of only "rejected but no claim exists" — distinguishing a lost compare-and-swap from the forge refusing the credential.

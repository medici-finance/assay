# Findings

Append-only log of knowledge that invalidates or updates existing briefs. Entry IDs are
slugs and carry no contiguity guarantee — there is no sequence to have a gap in; what
statusgen enforces is a tombstone check against history (an entry that has ever existed on
main but is absent from the working tree is a lint failure) plus duplicate-id detection.
Withdrawal is a tombstone entry, never deletion.

No findings yet.

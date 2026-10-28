# Outcome row identities and producer migration

`rows` is the exact held set for a wake receipt. `row_results` is the full covered
inventory, with `rows_total` and `rows_passed` derived from observed results.
Positive integer IDs retain JSON number form; suffix IDs use exact strings such
as `"6a"`. The identities `6`, `6a` and `6A` remain distinct. This is scheduling
metadata; it does not change the integer-only signed verdict-v1 execution schema
or grant Evidence, attestation or lifecycle authority.

The writer refuses case-variant row keys, duplicate/invalid identities, a held
row also claiming PASS, counts above the non-held inventory, held IDs absent
from exact results, and held IDs outside `1..rows_total` in count-only records.
Records without a reported inventory retain their legacy readability. Counts
cannot establish which exact identities passed.

## Producer migration: covered rows are not held rows

The `rows` field names held identities, never the list of all rows covered by a run.
Coverage belongs in `row_results` and `rows_total`; retain every original identity,
including suffixes, there. A `verified` producer omits `rows`. A failed or blocked
producer lists only the rows whose observed result remains held, and no held row
may also claim PASS. Whenever `rows` is non-empty, `row_results` is required for
ALL original Verify identities and their observed results, including non-contiguous
numeric and suffixed inventories. With an empty or omitted `rows`, exact results
remain optional. Count-only records are legacy-compatible only when held IDs fit
the declared contiguous `1..rows_total` inventory; new held-set producers always
supply exact results.
The writer's count, overlap, casing and inventory checks remain refusals.

This is a producer correction with an operational impact, not a transparent upgrade.
A read-only native-reader replay of 397 readable records at this source finds
53 records written since 2026-10-18 with rows and counts; 44 are refused by the
strict count rule and 9 are accepted. Correctness review 5593158920 independently
found the same 44 refusals at head 259f17ef, versus 3 before the count rule.
These are metadata-shape results, not verification verdicts or permission to
rewrite those records. Existing producers must adopt the held-only contract
before consuming this writer change.

For an existing unlanded payload that used `rows` as covered rows:

1. Preserve the original payload and its exact run, admission, Source, Evidence
   commit and witness artifacts. Never edit a committed outcome or historical
   Evidence to make the new checks pass.
2. Read that same run's actual witnessed row results. Put the complete original
   identity inventory and observed results in `row_results`; copy only the
   non-PASS identities actually held by its blocker into `rows`. A fully verified
   record omits `rows`. Derive counts from those observations, never from desired
   scheduling or the illustrative prompt example.
3. If those artifacts do not distinguish which exact rows were held, retain the
   payload unlanded as incomplete evidence. Report the missing row evidence and
   use the supported native verifier dispatch/admission flow for a fresh run;
   never assign PASS identities from aggregate counts or change admitted Source.
4. Submit a new outcome through `deskevidence --outcome-record`. Its ordinary
   completeness, actor, admission, source, join, receipt, target and immutable-record
   checks still apply. A corrected metadata payload does not waive a failed check,
   replace an old record in place, or advance lifecycle by itself.

The verifier skill and assembled native dispatch prompt carry this producer rule.
The prompt's metadata example is shape-tested against the writer and preserves
numeric/suffixed IDs. It is explicitly illustrative and cannot supply run Evidence.


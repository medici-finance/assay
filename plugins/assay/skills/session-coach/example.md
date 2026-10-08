Synthetic example — not drawn from a real session.

# Session coaching note (specimen)

Invented session: an agent was asked to add a retry limit to a fictional job-queue worker, "Parcel
Sorter", and open a draft PR. Models, step counts and file names below are made up to show the
shape of the note and nothing else. Written outside any repository; never committed.

### What did I work on

The closing summary said "added the retry limit and opened the PR". The history says otherwise.
Of 112 tool calls, 47 went to reading the test harness, 31 to re-running one test suite that
failed on a stale fixture, 19 to the change itself, and 15 to the PR text. The retry limit took
under a fifth of the session. The two threads that took the rest were both orientation: finding
out how the harness worked, then fighting the stale fixture. Models and routes used: the large
model for every step, including the 31 reruns.

### What did it cost

Measured in tool calls and in which model ran them; no spend figures were available in the record.

One swap: the 31 test reruns. Each was "run the suite, report pass or fail, quote the first
failing assertion", and the large model ran every one. Run that step on the small model. The
evidence it was enough: for the first six reruns the large model's output was a one-line verdict
plus a quoted assertion copied from the log, and the following step read the log directly anyway.
Nothing in those 31 steps needed judgement; the judgement came after, on the quoted line. The
large model stays on the step where the stale fixture was diagnosed (call 64).

### What would I do differently

One correction: the session wrote the same three-line shell sequence 9 times to reset the fixture
directory, rerun a single test file, and print the last 20 lines of the log. Each time cost about
two calls and an eyeballed copy of the path. That should have been a single verb, `run-one <file>`,
that resets the fixture and prints the tail. Failing that, the project's test README should carry
the sequence as a checklist item so the next session starts with it instead of rediscovering it.

Proposed memory edit (not made): in the project's agent notes, add "the Parcel Sorter fixture
directory goes stale between runs; reset it before rerunning a single test file". Evidence: the
failures at calls 38, 51 and 77 all cleared on reset. Accept or reject; this skill does not write it.

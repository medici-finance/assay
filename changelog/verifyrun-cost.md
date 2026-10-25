# verifyrun: cancel ordinary descendants at the row deadline

- A timed-out Verify row now uses statusgen's existing Unix process-group cancellation
helper, armed at the row deadline whether or not the row's shell is still running.
Killing only the shell left ordinary descendants running with the output pipe open, so
verifyrun could wait past the unchanged row deadline and the child could keep working
before the next row. That includes a shell that exits at once and leaves a background
child on the pipe. Timeout witnesses remain could-not-run. Exact row IDs, commands,
order, output hashes and all admission, source, fail-first, sandbox and Evidence checks
remain unchanged.
- Each Verify row now runs in its own process group, so a stop signal sent to
statusgen's group no longer reaches the row directly. statusgen therefore catches
SIGINT, SIGTERM and SIGHUP for the length of a run and cancels the running row with
the same group kill. The interrupted row is recorded could-not-run, never a verdict; no
later row starts; nothing is written to the brief or a witness file; and the run exits 2.
A signal that was ignored when statusgen started (under `nohup`, or as a script's
background job) stays ignored, so such a run still finishes normally. Only the first
signal is caught: a second one takes its default effect and ends statusgen at once,
even while an escaped descendant still holds the row's output pipe.
A row that runs `kill 0` now signals only its own group, not statusgen.

Normal background output is still read completely within the original deadline. No short
pipe cutoff, cached sandbox probe, command cache or skipped verification work was added.
Windows retains its existing direct-child cancellation limit, so there a descendant holding
the output pipe can still outlast the deadline. A descendant that deliberately leaves its
Unix process group can also outlive cancellation, and a SIGKILL of statusgen itself still
leaves a running row behind, as before. This is process cleanup, not a new sandbox or proof
of total process-tree containment.

Five alternating fresh-process fixture pairs, measured on the first revision of this change
(before the signal and shell-exit follow-up), kept the same 150 ms row deadline and command.
For the planted ordinary-child timeout, median executor time fell from 2,019 ms to 153.3 ms;
a successful background-output control stayed about 1.2 seconds before and after. These
controlled measurements show the deadline repair, not faster real verification checks or a
historical 30-minute-run speedup. CPU savings are not established.

The existing cancellation helper remains the implementation owner. The class sweep found
related statusgen Git-reader and desk hook/callout adapters; this change creates no new
execution library and moves no credential or admission authority.

Issue: #4360

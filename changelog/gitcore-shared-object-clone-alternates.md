### Fixed
- `deskpr` no longer refuses a clean shared-object checkout as
  "staged-but-uncommitted changes — commit them first". A checkout made with
  `git clone --shared` or `--reference` stores almost no objects of its own: it
  borrows them from the directory its `objects/info/alternates` names. The
  in-process git layer handed go-git a filesystem rooted at the checkout's own
  `.git`, which cannot see outside itself, so every borrowed object read as "not
  found" — and go-git's tree walk does not report that as an error. It turns a
  failed subtree read into an end-of-walk, so the walk stops early and the index
  entries whose HEAD-side counterparts vanished with it look like staged
  additions. A checkout `git status` called spotless was refused.
- Alternate object directories are now resolved through go-git's own
  `AlternatesFS` option. The filesystem it is given is rooted at the nearest
  common parent of the directories the repository itself declares it borrows
  from — not at the filesystem root — so the reach grows by exactly the subtree
  the repository names and no further.
- The staged-changes check now proves it can read the whole HEAD tree before it
  reports an answer, and returns an explicit could-not-check (`deskpr` exits
  unverifiable, naming `git repack -a` as the local repair) when it cannot. This
  closes the quieter half of the same defect: a truncated walk could also HIDE a
  genuinely staged deletion, so an unreadable object store could have produced a
  false CLEAN as easily as a false refusal. Real staged changes are still
  refused exactly as before.

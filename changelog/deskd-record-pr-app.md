### Fixed
- deskd files the cell-override record PR on its own record-PR App credential (`DESKD_RECORD_PR_TOKEN`, login on `DESKD_RECORD_PR_APP`) instead of the write-issues App's, which is specified to hold neither `Contents: write` nor `Pull requests: write`. `DESKD_WRITE_ISSUES_TOKEN` is no longer read; deskd prints a startup notice naming the replacement when it is still set.

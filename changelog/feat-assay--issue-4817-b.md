- Add `ASSAY_ASSERTED_HUMANS`, a pinned `login:id` roster key that lets a human whose username
ends in `-bot` or `-app` load into `ASSAY_TRUSTED_LOGINS` / `ASSAY_BLESS_LOGIN`. It waives only
the name-shape check, only for the exact pinned pair; a bare login, a `[bot]` / `app/`
rendering, a configured bot account, a duplicate or an unused assertion refuses the roster.
Every use is echoed (and so in the config digest), and an asserted login the forge marks as a
bot is still refused wherever the trust read carries the forge's bot flag. A refused roster
still loads nothing, and its echo now lists each dropped `ASSAY_` key and why.

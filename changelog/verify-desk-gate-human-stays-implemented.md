### Fixed

- The verify-desk skill no longer says a `gate: human` brief's verify PASS flips its row to `verified`. `statusgen verifyflip` refuses every human-gated brief, so the skill now says the PASS lands Evidence, the row stays `implemented`, the verify-outcome record reads blocked/human-action, and the human's close of the verify-gate card advances the row to `done`. It also says to refire the latest closed card with `deskclose verify-gate-refire` when every card is closed and a later PASS has landed (#7431).

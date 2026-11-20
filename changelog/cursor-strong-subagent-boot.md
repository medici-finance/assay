### Fixed

- A Cursor desk's standing boot prompt now names the cell's configured strong-tier model (`TIER_MODEL_TOP_CURSOR`) and says a `CURSOR_MODEL_default` of `auto` is a launch pin, not a reason to hold an `exec-tier: strong` item. The hold stays for an unconfirmed, weaker or inherited model.
- A Cursor verify-desk's boot prompt is never told to launch a strong subagent for routine work: the routine verify drain runs at its local session model and holds an `exec-tier: strong` brief that model cannot meet; the configured strong-tier id is named only for the floor-tier re-verify of a `gate: human` brief.

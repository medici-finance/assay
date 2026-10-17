Desk supervision now directs agents to wait on existing completion handles instead of
repeatedly polling unchanged work, and to start each new item with a fresh child context.
Compact checkpoints and file-backed logs reduce repeated context loading while preserving
review scope, independent lanes, prior findings and required freshness checks. Codex desk
launches include this guidance; already-running sessions require an explicit instruction
update or a later launch with the updated tools and plugin.

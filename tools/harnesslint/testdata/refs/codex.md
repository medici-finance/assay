# Fixture Codex binding

| Capability | Mechanism |
|---|---|
| `capability:dispatch-worker` | spawn_agent behind multi_agent |
| `capability:message-agent` | send_message |
| `capability:isolate-workspace` | git worktree (refuses under workspace-write) |
| `capability:invoke-skill` | the skill mechanism |
| `capability:session-notifications` | turn-complete notifications |

## Degradation

| Skill | Class |
|---|---|
| `alpha` | runs |
| `beta` | degrades: serial with an in-session statement |

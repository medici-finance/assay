- Document a draft celld session and execution-host design mapped onto the cell operator
  surface, the runtime contract alignment and the shared runner owners; celld adds no
  second Cell API or lifecycle vocabulary. Include six bounded qualification
  spikes for harness isolation, mixed-harness handoff, durable ownership, drain/stop,
  remote registration and Kubernetes lifecycle; no runtime activation is included.
  Map the Claude Agent SDK hooks, interrupt, background-task stop and session rebind onto
  the same capability contract as a separate candidate S4 qualification lane, with the
  drain gate enforced by a hook; the existing Claude CLI-profile adapter brief is unchanged.

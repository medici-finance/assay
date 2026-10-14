- Document a draft celld session and execution-host design aligned with the existing
  cell controller and shared runner architecture. Include six bounded qualification
  spikes for harness isolation, mixed-harness handoff, durable ownership, drain/stop,
  remote registration and Kubernetes lifecycle; no runtime activation is included.
  Map the Claude Agent SDK hooks, interrupt, background-task stop and resume onto the
  same capability contract as a candidate S4 lane, with the drain gate enforced by a hook.

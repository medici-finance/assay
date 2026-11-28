---
name: staging
description: >
  Stage a deck, talk, tutorial, or explainer video before detailed production — the
  genre → audience → arc layer. Decides the genre, reads the audience, chooses
  the arc, assigns a function and a headline claim to every slot, places the one
  bold moment, allocates motion roles for video, and reconciles it all against a
  time budget in minutes. Emits a staging.yaml contract that a downstream
  authoring skill (deck-author, video-author) consumes. Use when someone asks to
  "stage a deck", "stage a video", "storyboard the argument", "plan the
  presentation", "structure this talk/explainer", or asks for any deck, talk,
  workshop, walkthrough, pitch, or explainer video whose structure has not already been
  ruled. Do NOT use it to choose charts, layouts, palettes, fonts, or animation
  effects — that is design's job, and staging is forbidden to touch it.
---

# Staging

Stage an argument a reader can inspect before a slide or scene is built. Use the existing brief, audience and time limit. Ask what the audience should believe or be able to do afterwards. Choose a genre, sequence the claims, check every figure and give each term its first explanation.

Read [the staging workflow](workflow.md) for the genre routing table, complete staging.yaml schema, video review and handoff. Paths in that workflow start here. Save staging.yaml with the project and run tools/staging_lint.py on it. Use the sibling deck-author or video-author skill after the stage is ready.

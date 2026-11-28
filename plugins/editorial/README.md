# Editorial

Install `editorial@assay` in an article, deck or video project. Article Craft, Prose Cadence and Humanizer support writing. The six presentation skills share a staged argument across slides and film.

## Claude Code

Run these commands in your terminal, then start a fresh session in your writing project.

```sh
claude plugin marketplace add medici-finance/assay
claude plugin install editorial@assay
```

Try `/editorial:article-craft` with a draft and its source notes. Once you have checked the claims, use `/editorial:prose-cadence` on the paragraph you want to revise. Give `/editorial:humanizer` the revised passage and a sample of your writing.

## Codex

```sh
codex plugin marketplace add medici-finance/assay
codex plugin add editorial@assay
```

Start a fresh session in the writing project. Select `$editorial:article-craft`, `$editorial:prose-cadence`, or `$editorial:humanizer` and name your draft in the prompt.

## Cursor

On Teams or Enterprise, import `https://github.com/medici-finance/assay` through **Dashboard → Plugins & MCPs → Team Marketplaces → Add Marketplace → Import from Repo**. Track `main`, add **editorial**, and grant the team access. Members can enable **editorial** under **Customize** and ask for any installed Editorial skill by name in a new Agent chat. If the marketplace is already connected, refresh it to discover the new plugin.

## Produce a deck or video

Ask “Stage a five-minute technical talk from these notes” and give **staging** the source notes, audience and time limit. It produces a staging.yaml with the claims and their order. Use **deck-author** to turn that into an HTML presentation, **deck-typography** to check the reading size, and **slop-lint** to edit the slide copy. For film, **video-design** develops the storyboard and **video-author** builds a seekable preview and exports a native picture with your prepared soundtrack and captions.

In Claude Code invoke `/editorial:staging`. In Codex select `$editorial:staging`. In Cursor enable the Editorial plugin and select the staging skill or ask Agent to stage the talk. The other skills use their directory names in the same way. Keep source material and outputs in your project. Relative tool and reference paths in a skill start at its installed SKILL.md.

Python 3.11+ runs the staging and deck checks without third-party packages. Browser QA takes Node.js, Playwright, Chromium and ffmpeg. For an existing dependency installation, set `EDITORIAL_RENDER_DEPS` to the directory containing its `node_modules`. For a new one, run `npm install playwright` and `npx playwright install chromium ffmpeg` in a project dependency directory. The [video export instructions](skills/video-author/references/native-rendering.md) describe the fframes build, ffmpeg, ffprobe, audio and captions needed for final export. The voice provider, profiles, recordings and publishing credentials stay with the project.

## Try it

Give Article Craft a source note and a draft claim. For example, a benchmark measured 20 warm requests from a replay of 80, but the draft says performance improved for every request. Ask which words the evidence supports. Then give the verified paragraph to Prose Cadence and ask it to make the sampling limit easier to hear while preserving the measurements. For the final voice pass, give Humanizer that revision and a paragraph from a similar article you wrote. Keep “untimed” if the evidence says nothing about who ran the test.

Use the [article review rubric](skills/article-craft/references/editorial-calibration.md#article-review-rubric) to review a finished draft. Each criterion asks for a passage and its consequence for the reader.

## Updating this package

Article Craft and Prose Cadence are shared with the methodology plugin. Edit their canonical directories under `plugins/assay/skills/`, then copy the complete changed directories here, including references. Humanizer lives in this package. Its pattern guide adapts examples for technical articles and retains the MIT license from [Humanizer 4.0.0](https://github.com/blader/humanizer/tree/b48c832f0f809c1b5658edbb838c2521b72f2413). When updating that reference, compare the new upstream SKILL.md and LICENSE with the pinned source, carry forward the editorial examples, and check the changes against a writing sample.

When the published payload changes, increment the version in all three editorial plugin manifests. The umbrella release gate checks for a version increase when the editorial package changes between releases. The `tools/harnessgen` tests compare the complete skill trees, notices, manifests, and marketplace entries.

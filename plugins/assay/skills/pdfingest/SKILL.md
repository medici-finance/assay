---
name: pdfingest
description: >-
  First-pass ingestion of PDFs and office documents (DOCX/PPTX/XLSX/HTML/EPUB)
  into LLM-ready markdown via Docling — layout-aware reading order, real table
  structure, OCR for scans. Use when the task is to read, cite, or fold a
  document into research notes or a corpus: "ingest this paper", "what does
  this PDF say", "pull the tables out of this report", "add this to the
  research register". Handles the whole two-tier shape: deterministic Docling
  extraction first, vision pass only where Docling's output is insufficient
  (figures, dense math, mangled layout).
---

# pdfingest — document first pass

You are ingesting a document. The rule of this skill is the **two-tier shape**:
a cheap deterministic first pass (Docling — layout model, table-structure
model, OCR) over the whole document, then an expensive vision pass only on the
parts the first pass cannot carry. Never page an entire PDF through vision when
markdown extraction answers the question.

## 1. Convert

Run `plugins/assay/scripts/pdfingest.sh` (or any docling-serve client):

```bash
pdfingest.sh paper.pdf > paper.md       # markdown to stdout
pdfingest.sh --json paper.pdf           # page/bbox provenance — for citations
pdfingest.sh --pages 12-14 paper.pdf    # a slice, when hunting one claim
pdfingest.sh --health                   # is the endpoint up?
```

- Endpoint: `ASSAY_DOCLING_URL` (default `http://localhost:5001`). Point it at
  your deployment's FRONT (e.g. a scale-to-zero proxy in front of the pods) —
  `kubectl port-forward` the front service, not the docling service itself
  (a scaled-to-zero deployment has no endpoints). Field names are
  upstream's and may drift between versions — verify against the server's
  `/docs` if a request 400s. A cold start after idle legitimately takes tens
  of seconds to minutes (pod schedule + image pull + model load) — use a
  generous HTTP timeout, don't retry-storm.
- No endpoint and no local `docling` CLI: `pip install docling` (CPU-only,
  models download once, ~2 GB) — or tell the driver what's missing rather
  than silently downgrading to raw `pdftotext`.
- **Confidentiality**: the document body crosses the endpoint's wire WHOLE.
  Point `ASSAY_DOCLING_URL` only at an endpoint you or your driver control —
  never a third-party API for confidential material.

## 2. Cite with anchors

Extracted text is a claim about the source, so citations carry a locator:

- Prefer **section heading + page**: `Orchestra, §6.2, p.14` — page numbers
  come from `--json` provenance or a `--pages` slice, not from eyeballing.
- Record the **access date** and the artifact identity (URL, DOI, file hash
  for local files) alongside the citation, in whatever register the work
  keeps.
- If you quote the markdown rather than the page, say so — extraction can
  reorder two-column text or drop a figure caption.

## 3. Know the failure modes

Docling is text-and-structure, not understanding. Escalate to a vision pass
(render the page, read it as an image) exactly when:

- the claim lives in a **figure, chart, or diagram** — markdown carries only
  the caption;
- **dense math** — extraction mangles multi-line equations;
- the markdown looks **wrong**: a table whose columns collapsed, a reading
  order that zig-zags, headings inside paragraphs. Don't repair it by guessing
  — re-read that page as an image and reconcile.
- the file is a **scan** and OCR output is gibberish — vision beats damaged OCR.

The escalation is per-page, not per-document: extract the whole thing, vision
only the pages that earn it.

## 4. Discipline

- The extraction is **working material**, like transcripts: quote from it,
  cite from it, discard the intermediate — don't commit machine markdown to a
  research register as if it were authored prose.
- Never present an unrun conversion as done: a failed or skipped extraction is
  reported with its error, not papered over.
- Structured data (tables headed for analysis, not prose) should come from
  `--json`, which preserves cell structure — markdown tables round-trip badly.

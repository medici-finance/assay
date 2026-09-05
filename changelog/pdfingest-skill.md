### Added
- **`pdfingest` skill** — first-pass ingestion of PDFs and office documents
  (DOCX/PPTX/XLSX/HTML/EPUB) into LLM-ready markdown via Docling: layout-aware
  reading order, real table structure, and OCR for scans. Two-tier by design —
  deterministic Docling extraction first, a vision pass only where Docling's
  output is insufficient (figures, dense math, mangled layout). Ships a
  `docling-serve` client (`plugins/assay/scripts/pdfingest.sh`) and a
  self-contained `SETUP.md` covering all four install rungs.

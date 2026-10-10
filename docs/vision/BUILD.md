# Building the vision report

The narrative source is [README.md](README.md). The PDF uses that text and the seven committed SVG figures as sharp vectors; GitHub uses their matching PNG exports. It does not fetch network data or regenerate capability status.

## PDF

Use Python 3 with ReportLab installed, then run from the repository root:

```sh
python3 docs/vision/build_pdf.py
```

The output is `docs/vision/assay-vision.pdf`. The script uses PDF-standard Helvetica for body text and Times for chapter titles. A small renderer handles the static rectangles, paths and text used by these figures without rasterizing them. It fails on unsupported elements so future edits require an explicit renderer update. It creates a cover, linked contents, an installation quickstart, numbered chapters, source links and page numbers. The website and installation commands come from the same Markdown source; the cover also links to the website. Text streams remain uncompressed for straightforward publication inspection; no font files or pixel data are embedded. A fixed publication date and pinned evidence revision live in the README. Revise those only when the corresponding source review has been performed.

After a change, render every page with Poppler and inspect the result:

```sh
pdftoppm -r 120 -png docs/vision/assay-vision.pdf /tmp/assay-vision-page
```

Confirm that diagrams, tables, page breaks, links and non-ASCII glyphs remain readable. Markdown and PDF share the narrative source; the PDF contents page and typography are publication-specific.

## Graphics

Each figure in [assets](assets/) has three forms: a self-contained HTML source, an accessible SVG and a PNG embedded by GitHub. They use a white background, black text and a small blue accent. The images illustrate relationships and future direction; they do not encode numeric completion or claim operational deployment.

Edit the HTML's inline SVG and update the standalone SVG to match. Render the HTML's SVG element at a device scale factor of 2 with an existing browser automation runtime to regenerate its PNG. No remote fonts, external stylesheets or network requests are required. Keep the 1200-unit viewBox, accessible title/description and complete static meaning. Do not substitute external image hosting for the relative Markdown asset links.

## Publication review

Review status language against the cited public source revision. Validate local Markdown links and figure files. Read the diff for private information or unsupported availability, certification or leadership claims. The root README is the entry point; the long-form report remains in this directory so adoption instructions stay easy to find.

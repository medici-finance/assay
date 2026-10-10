#!/usr/bin/env python3
"""Build the public vision PDF from its Markdown and committed figures, offline."""

from html import escape
from pathlib import Path
import re
import math
import xml.etree.ElementTree as ET

import reportlab
from reportlab import rl_config
from reportlab.lib import colors
from reportlab.lib.enums import TA_LEFT
from reportlab.lib.pagesizes import A4
from reportlab.lib.styles import ParagraphStyle
from reportlab.pdfbase import pdfmetrics
from reportlab.pdfbase.pdfmetrics import Font
from reportlab.platypus import (
    BaseDocTemplate, CondPageBreak, Flowable, Frame, KeepTogether, PageBreak,
    PageTemplate, Paragraph, Preformatted, Spacer, Table, TableStyle,
)
from reportlab.platypus.tableofcontents import TableOfContents

HERE = Path(__file__).resolve().parent
# Keep image streams in ordinary Flate encoding for public artifact scanners.
# ReportLab's ASCII85 wrapper is unnecessary here and harder to inspect.
rl_config.useA85 = 0
ROOT = HERE.parent.parent
SNAPSHOT = "afa97c523701abdc65ce9afce1e73485c0b9ebc7"
INK = colors.HexColor("#171b21")
BLUE = colors.HexColor("#215ee8")
MUTED = colors.HexColor("#5d6470")
LINE = colors.HexColor("#dce0e6")
WIDTH, HEIGHT = A4
MARGIN = 54
CONTENT_WIDTH = WIDTH - 2 * MARGIN

for name, face in [
    ("Body", "Helvetica"), ("Body-Bold", "Helvetica-Bold"),
    ("Body-Italic", "Helvetica-Oblique"), ("Body-BoldItalic", "Helvetica-BoldOblique"),
]:
    pdfmetrics.registerFont(Font(name, face, "WinAnsiEncoding"))
pdfmetrics.registerFontFamily(
    "Body", normal="Body", bold="Body-Bold", italic="Body-Italic",
    boldItalic="Body-BoldItalic",
)

STYLES = {
    "body": ParagraphStyle("BodyText", fontName="Body", fontSize=10,
                           leading=15.5, textColor=INK, spaceAfter=11,
                           allowWidows=0, allowOrphans=0),
    "chapter": ParagraphStyle("Chapter", fontName="Times-Roman", fontSize=30,
                              leading=34, textColor=INK, spaceAfter=22,
                              keepWithNext=True),
    "eyebrow": ParagraphStyle("Eyebrow", fontName="Body-Bold", fontSize=8,
                              leading=12, textColor=BLUE, spaceAfter=12),
    "caption": ParagraphStyle("Caption", fontName="Body", fontSize=8,
                              leading=12, textColor=MUTED, spaceAfter=16),
    "table": ParagraphStyle("TableText", fontName="Body", fontSize=8.1,
                            leading=12, textColor=INK, spaceAfter=0),
    "tablehead": ParagraphStyle("TableHead", fontName="Body-Bold", fontSize=8.1,
                                leading=12, textColor=INK, spaceAfter=0),
    "code": ParagraphStyle("InstallCommand", fontName="Courier", fontSize=9,
                           leading=15, textColor=INK, spaceAfter=14,
                           backColor=colors.HexColor("#f3f5f8"), borderPadding=10),
    "toc": ParagraphStyle("ContentsEntry", fontName="Body", fontSize=10.2,
                          leading=16, textColor=INK, spaceBefore=8,
                          leftIndent=0, firstLineIndent=0),
}


def slug(text):
    return re.sub(r"[^a-z0-9 -]", "", text.lower()).replace(" ", "-")


def url_for(target):
    if target.startswith(("https://", "http://", "#")):
        return target
    local = (HERE / target).resolve()
    relative = local.relative_to(ROOT).as_posix()
    kind = "tree" if local.is_dir() else "blob"
    # Newly authored publication files live on the branch, rather than the
    # historical evidence snapshot. Existing source references stay pinned.
    ref = "main" if relative.startswith("docs/vision/") else SNAPSHOT
    return f"https://github.com/medici-finance/assay/{kind}/{ref}/{relative}"


def inline(text):
    # Parse just the inline forms used in this publication, leaving source text
    # escaped so Markdown cannot inject ReportLab markup.
    pattern = r"\[([^\]]+)\]\(([^)]+)\)|\*\*([^*]+)\*\*|`([^`]+)`"
    parts, end = [], 0
    for match in re.finditer(pattern, text):
        parts.append(escape(text[end:match.start()]))
        label, target, bold, code = match.groups()
        if target:
            parts.append(f'<link href="{escape(url_for(target), quote=True)}" color="#215ee8">{escape(label)}</link>')
        elif bold:
            parts.append(f"<b>{escape(bold)}</b>")
        else:
            parts.append(escape(code))
        end = match.end()
    parts.append(escape(text[end:]))
    # Use the PDF-standard Symbol font for the right-arrow glyph rather
    # than silently rendering a missing-glyph box in the brief's three labels.
    return "".join(parts).replace("→", '<font name="Symbol">→</font>')


class Cover(Flowable):
    def __init__(self):
        super().__init__()
        self.width = CONTENT_WIDTH
        self.height = HEIGHT - 2 * MARGIN - 22

    def draw(self):
        c = self.canv
        y = self.height
        c.setFillColor(BLUE)
        c.setFont("Body-Bold", 11)
        c.drawString(0, y - 22, "ASSAY / VISION REPORT")
        c.setStrokeColor(BLUE)
        c.setLineWidth(3.5)
        p = c.beginPath()
        p.moveTo(CONTENT_WIDTH - 45, y - 20)
        p.lineTo(CONTENT_WIDTH - 32, y - 33)
        p.lineTo(CONTENT_WIDTH - 6, y - 2)
        c.drawPath(p)
        c.setFillColor(INK)
        c.setFont("Times-Roman", 47)
        for i, line in enumerate(["Run engineering", "as a system."]):
            c.drawString(0, y - 134 - i * 54, line)
        c.setFont("Body", 15)
        c.drawString(0, y - 252, "From governed delivery")
        c.drawString(0, y - 276, "to engineering operations")
        phrase = Paragraph(
            "an agent-operated delivery pipeline<br/>with a human-governance layer",
            ParagraphStyle("CoverPhrase", fontName="Body", fontSize=11,
                           leading=17, textColor=MUTED),
        )
        phrase.wrap(CONTENT_WIDTH, 64)
        phrase.drawOn(c, 0, y - 344)
        SVGFigure(HERE / "assets/02-direction.svg").drawOn(c, 0, y - 594)
        c.setStrokeColor(LINE)
        c.setLineWidth(0.7)
        c.line(0, 65, CONTENT_WIDTH, 65)
        c.setFillColor(MUTED)
        c.setFont("Body", 9)
        c.drawString(0, 43, "9 October 2026 · For engineering leaders")
        c.drawString(0, 25, "Public strategic vision · current foundations and future direction")
        c.setFillColor(BLUE)
        c.drawString(0, 7, "assay.guide")
        c.linkURL("https://assay.guide/", (0, 4, 65, 17), relative=1)


class SVGFigure(Flowable):
    """Render this publication's small static SVG vocabulary as PDF vectors."""

    def __init__(self, path):
        super().__init__()
        self.svg = ET.parse(path).getroot()
        self.view_width, self.view_height = map(float, self.svg.attrib["viewBox"].split()[2:])
        self.width = CONTENT_WIDTH
        self.height = self.width * self.view_height / self.view_width

    def draw(self):
        c = self.canv
        c.saveState()
        c.translate(0, self.height)
        c.scale(self.width / self.view_width, -self.width / self.view_width)
        for element in self.svg:
            tag = element.tag.rsplit("}", 1)[-1]
            a = element.attrib
            if tag in {"title", "desc", "defs"}:
                continue
            stroke = a.get("stroke", "none")
            fill = a.get("fill", "none")
            c.setLineWidth(float(a.get("stroke-width", 1)))
            c.setDash([float(v) for v in a.get("stroke-dasharray", "").split(",") if v])
            if stroke != "none":
                c.setStrokeColor(colors.toColor(stroke))
            if fill != "none":
                c.setFillColor(colors.toColor(fill))
            if tag == "rect":
                x, y, w, h = [float(a.get(k, 0)) for k in ("x", "y", "width", "height")]
                c.roundRect(x, y, w, h, float(a.get("rx", 0)),
                            stroke=int(stroke != "none"), fill=int(fill != "none"))
            elif tag == "text":
                family = a.get("font-family", "Arial")
                face = "Times-Roman" if family.startswith("Georgia") else "Helvetica"
                if int(a.get("font-weight", 400)) >= 600:
                    face = "Times-Bold" if face == "Times-Roman" else "Helvetica-Bold"
                c.saveState()
                c.translate(float(a["x"]), float(a["y"]))
                c.scale(1, -1)
                c.setFont(face, float(a.get("font-size", 20)))
                c.drawString(0, 0, element.text or "")
                c.restoreState()
            elif tag == "path":
                tokens = re.findall(r"[MLHVQ]|-?\d+(?:\.\d+)?", a["d"])
                p = c.beginPath()
                i, x, y, dx, dy = 0, 0., 0., 1., 0.
                while i < len(tokens):
                    command = tokens[i]
                    i += 1
                    old_x, old_y = x, y
                    if command in {"M", "L"}:
                        x, y = float(tokens[i]), float(tokens[i + 1]); i += 2
                        p.moveTo(x, y) if command == "M" else p.lineTo(x, y)
                    elif command in {"H", "V"}:
                        value = float(tokens[i]); i += 1
                        if command == "H": x = value
                        else: y = value
                        p.lineTo(x, y)
                    elif command == "Q":
                        qx, qy, x, y = map(float, tokens[i:i + 4]); i += 4
                        p.curveTo(old_x + (qx - old_x) * 2 / 3,
                                  old_y + (qy - old_y) * 2 / 3,
                                  x + (qx - x) * 2 / 3, y + (qy - y) * 2 / 3, x, y)
                    else:
                        raise ValueError("Unsupported SVG path command: " + command)
                    if command != "M":
                        dx, dy = x - old_x, y - old_y
                c.drawPath(p, stroke=int(stroke != "none"), fill=int(fill != "none"))
                if a.get("marker-end"):
                    length = math.hypot(dx, dy)
                    ux, uy = dx / length, dy / length
                    head = c.beginPath()
                    head.moveTo(x - 12 * ux + 6 * uy, y - 12 * uy - 6 * ux)
                    head.lineTo(x, y)
                    head.lineTo(x - 12 * ux - 6 * uy, y - 12 * uy + 6 * ux)
                    c.setDash([])
                    c.drawPath(head)
            else:
                raise ValueError("Unsupported SVG element: " + tag)
        c.restoreState()


class VisionDoc(BaseDocTemplate):
    def __init__(self, path):
        super().__init__(str(path), pagesize=A4, leftMargin=MARGIN,
                         rightMargin=MARGIN, topMargin=58, bottomMargin=56,
                         title="Assay: from governed delivery to engineering operations",
                         author="Assay", subject="Public strategic vision",
                         creator="Assay vision publication builder", invariant=1,
                         pageCompression=0)
        self.section = "VISION REPORT"
        self.addPageTemplates(PageTemplate(
            id="publication", frames=[Frame(MARGIN, 56, CONTENT_WIDTH,
                                             HEIGHT - 114, leftPadding=0,
                                             rightPadding=0, topPadding=0,
                                             bottomPadding=0)],
            onPage=self.chrome,
        ))

    def beforeDocument(self):
        self.section = "VISION REPORT"

    def chrome(self, c, doc):
        if doc.page == 1:
            return
        c.saveState()
        c.setFillColor(MUTED)
        c.setFont("Body", 7.4)
        c.drawString(MARGIN, HEIGHT - 31, "ASSAY / PUBLIC STRATEGIC VISION")
        c.setStrokeColor(LINE)
        c.setLineWidth(0.6)
        c.line(MARGIN, 41, WIDTH - MARGIN, 41)
        c.setFont("Body", 7.5)
        c.drawString(MARGIN, 27, "Governed agent-led engineering · Vision edition 2026-10-09")
        c.drawRightString(WIDTH - MARGIN, 27, str(doc.page))
        c.restoreState()

    def afterFlowable(self, flowable):
        if getattr(flowable, "chapter_name", None):
            title = flowable.chapter_name
            self.section = re.sub(r"^\d+\. ", "", title)
            key = slug(title)
            self.canv.bookmarkPage(key)
            self.canv.addOutlineEntry(title, key, 0, False)
            self.notify("TOCEntry", (0, title, self.page, key))


def add_table(story, rows):
    columns = len(rows[0])
    widths = ([0.20, 0.40, 0.40] if columns == 3 else [1 / columns] * columns)
    data = [[Paragraph(inline(cell.strip()), STYLES["tablehead" if i == 0 else "table"])
             for cell in row] for i, row in enumerate(rows)]
    table = Table(data, colWidths=[CONTENT_WIDTH * w for w in widths], repeatRows=1,
                  hAlign=TA_LEFT)
    table.setStyle(TableStyle([
        ("VALIGN", (0, 0), (-1, -1), "TOP"),
        ("BACKGROUND", (0, 0), (-1, 0), colors.HexColor("#f3f5f8")),
        ("LINEBELOW", (0, 0), (-1, 0), 0.8, LINE),
        ("LINEBELOW", (0, 1), (-1, -1), 0.4, LINE),
        ("LEFTPADDING", (0, 0), (-1, -1), 8),
        ("RIGHTPADDING", (0, 0), (-1, -1), 8),
        ("TOPPADDING", (0, 0), (-1, -1), 9),
        ("BOTTOMPADDING", (0, 0), (-1, -1), 9),
    ]))
    story.extend([table, Spacer(1, 15)])


def build():
    source = (HERE / "README.md").read_text()
    # Intro, cover metadata and contents are laid out explicitly. All numbered
    # report chapters and their narrative are consumed directly from Markdown.
    boundary = source.index("## 1. What Assay is")
    introduction = source[source.index("This report describes"):source.index("## Get started")].strip()
    story = [Cover(), PageBreak(),
             Paragraph("READING THE REPORT", STYLES["eyebrow"]),
             Paragraph("The vision, and its evidence", STYLES["chapter"]),
             Paragraph(inline(introduction), STYLES["body"]), Spacer(1, 8)]
    toc = TableOfContents()
    toc.levelStyles = [STYLES["toc"]]
    story.append(toc)
    story.extend([PageBreak(), Paragraph("ADOPT ASSAY", STYLES["eyebrow"])])
    heading = Paragraph("Get started", STYLES["chapter"])
    heading.chapter_name = "Get started"
    story.append(heading)
    installation = source[source.index("## Get started") + len("## Get started"):source.index("## Read the report")].strip()
    for block in re.split(r"\n\s*\n", installation):
        if block.startswith("```text\n") and block.endswith("```"):
            story.append(Preformatted(block[len("```text\n"):-3].rstrip(), STYLES["code"]))
        else:
            story.append(Paragraph(inline(" ".join(block.splitlines())), STYLES["body"]))
    blocks = re.split(r"\n\s*\n", source[boundary:].strip())
    publication_notes = []
    first_chapter = True
    for block in blocks:
        if block.startswith("## "):
            title = block[3:].strip()
            number, name = title.split(". ", 1)
            new_page = first_chapter or number == "14"
            story.append(PageBreak() if new_page else CondPageBreak(330))
            if not new_page:
                story.append(Spacer(1, 16))
            first_chapter = False
            story.append(Paragraph("CHAPTER " + number.zfill(2), STYLES["eyebrow"]))
            paragraph = Paragraph(escape(name), STYLES["chapter"])
            paragraph.chapter_name = title
            story.append(paragraph)
        elif block.startswith("!["):
            match = re.fullmatch(r"!\[([^\]]+)\]\(([^)]+)\)", block.strip())
            if not match:
                raise ValueError("Unsupported figure syntax: " + block)
            caption, asset = match.groups()
            image = SVGFigure((HERE / asset).with_suffix(".svg"))
            story.append(KeepTogether([image, Spacer(1, 7),
                                      Paragraph(escape(caption), STYLES["caption"])]))
        elif block.startswith("|"):
            rows = [[cell.strip() for cell in line.strip().strip("|").split("|")]
                    for line in block.splitlines() if not re.match(r"^\|[ :|-]+\|$", line)]
            add_table(story, rows)
        else:
            paragraph = Paragraph(inline(" ".join(block.splitlines())), STYLES["body"])
            if block.startswith(("**Editorial proposals", "**Graphics.", "**Publication sources.")):
                publication_notes.append(paragraph)
            else:
                story.append(paragraph)
    story.append(KeepTogether(publication_notes))
    destination = HERE / "assay-vision.pdf"
    VisionDoc(destination).multiBuild(story)
    print(f"Built {destination.name} from README.md and seven committed figures.")


if __name__ == "__main__":
    build()

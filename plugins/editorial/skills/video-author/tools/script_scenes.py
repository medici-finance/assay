"""Read the author’s scene table for the native export manifest."""
from __future__ import annotations

import re
from dataclasses import dataclass
from pathlib import Path


@dataclass
class Scene:
    id: str
    narration: str
    subtitle: str  # falls back to narration when the script has no subtitle column


_BOLD = re.compile(r"\*\*(.+?)\*\*")


def _clean(cell: str) -> str:
    cell = cell.strip()
    cell = _BOLD.sub(r"\1", cell)
    if len(cell) >= 2 and cell[0] in "\"'" and cell[-1] == cell[0]:
        cell = cell[1:-1]
    return cell.strip()


def _split_row(line: str) -> list[str]:
    line = line.strip()
    if line.startswith("|"):
        line = line[1:]
    if line.endswith("|"):
        line = line[:-1]
    return [c.strip() for c in line.split("|")]


def parse_script(path: Path) -> list[Scene]:
    lines = [ln for ln in Path(path).read_text().splitlines()]
    header = None
    col = {}
    rows: list[list[str]] = []
    for ln in lines:
        if "|" not in ln:
            if header is not None and rows:
                break  # table ended
            continue
        cells = _split_row(ln)
        # separator row like |---|---|
        if all(set(c) <= set("-: ") and c for c in cells):
            continue
        if header is None:
            header = [c.lower() for c in cells]
            for i, h in enumerate(header):
                if "id" in h and "id" not in col:
                    col["id"] = i
                if "narration" in h:
                    col["narration"] = i
                if "subtitle" in h:
                    col["subtitle"] = i
            if "id" not in col or "narration" not in col:
                header = None  # not the scenes table; keep scanning
                col = {}
            continue
        rows.append(cells)

    if header is None:
        raise SystemExit(
            f"no scenes table found in {path}. Expected a Markdown table with "
            "'id' and 'narration' columns."
        )

    scenes: list[Scene] = []
    for cells in rows:
        try:
            sid = _clean(cells[col["id"]])
            narration = _clean(cells[col["narration"]])
        except IndexError:
            continue
        if not sid or not narration:
            continue
        subtitle = narration
        if "subtitle" in col and col["subtitle"] < len(cells):
            sub = _clean(cells[col["subtitle"]])
            if sub:
                subtitle = sub
        scenes.append(Scene(id=sid, narration=narration, subtitle=subtitle))

    if not scenes:
        raise SystemExit(f"scenes table in {path} parsed to zero rows.")
    return scenes

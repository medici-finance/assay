#!/usr/bin/env python3
"""Content-gate linter for a staged `staging.yaml` contract (presentation-skills/01).

Checks the REQUIRED `number_gate` and `glossary` blocks (SKILL.md §6 HANDOFF), the
`diagram:` design-leak exception (SKILL.md's composition contract §1), and the
`product-pitch` genre's three authored gates — `mechanism_by`, `problem_share`,
`cost_slot` (`references/genres/product-pitch.md`).

STDLIB-ONLY BY DECISION, not by accident. PyYAML resolves in this checkout's
interpreter, but these skills are advertised as self-contained
(`PRESENTATION-SKILLS.md`) and install into repos that never receive this
repo's environment — the sibling linter `deck-author/tools/check_deck.py`
makes the identical call for the identical reason.

Consequently this file does NOT parse general YAML. It parses the restricted
block/scalar subset `staging.yaml` actually uses, by construction of this
skill's own schema:

  - a bare scalar line:            `key: value` (or `key: "quoted value"`)
  - a single-line flow mapping:    `key: {a: 1, b: "two"}`
  - a block sequence of single-line flow mappings:
        key:
          - {a: 1, b: "two"}
          - {a: 3, b: "four"}

Every list-valued key this linter reads (`slots`, `number_gate`, `glossary`,
`arc`) MUST be written in the block-sequence form above — one flow mapping per
physical line; the sequence rows may sit indented or at column 0. The
inline-array form (`arc: [{...}, ...]`) and multi-line block mappings are valid
YAML but are NOT parsed here; unsupported list forms are rejected before
content checks, rather than being treated as empty or partially checked lists.
A trailing comment on a list key (`slots:  # rows`) is allowed. Nested flow values (a `{` inside a value) are
not supported either; the schema this skill emits never needs one.

Known limit, stated rather than hidden: a "figure" is recognised only as a
dollar amount (`$1,200`) or a percentage (`3.1%`) token — the two shapes every
dropped or unglossed number in the bake-off's verdicts actually took. A figure
written any other way (a bare count, a date used as a metric) is invisible to
the `number_gate` cross-check; the block itself is still required and its
per-row rules (reason/as_of/conflict/withhold) still apply.

Usage:
    python3 staging_lint.py <staging.yaml>

Exit: 0 clean (prints a line containing "PASS"), 1 one or more rule
violations, 2 usage/IO problem.
"""

from __future__ import annotations

import argparse
import re
import sys
from collections import defaultdict
from pathlib import Path

# ---------------------------------------------------------------- restricted parser


def parse_flow_value(raw: str):
    """Turn one flow-mapping VALUE token into a python scalar (str/bool/None)."""
    s = raw.strip()
    if len(s) >= 2 and s[0] == s[-1] == '"':
        return s[1:-1]
    if len(s) >= 2 and s[0] == s[-1] == "'":
        return s[1:-1]
    low = s.lower()
    if low == "true":
        return True
    if low == "false":
        return False
    if low in ("null", "~", ""):
        return None
    return s


def split_flow_items(body: str) -> list[str]:
    """Split a `{...}` interior on top-level commas, respecting quoted strings."""
    items: list[str] = []
    cur = ""
    quote = None
    for ch in body:
        if quote:
            cur += ch
            if ch == quote:
                quote = None
            continue
        if ch in ('"', "'"):
            quote = ch
            cur += ch
            continue
        if ch == ",":
            items.append(cur)
            cur = ""
            continue
        cur += ch
    if cur.strip():
        items.append(cur)
    return items


def parse_flow_dict(line: str) -> dict:
    line = line.strip()
    if not (line.startswith("{") and line.endswith("}")):
        raise ValueError(f"expected a single-line flow mapping, got: {line!r}")
    body = line[1:-1]
    out: dict[str, object] = {}
    for item in split_flow_items(body):
        if ":" not in item:
            continue
        k, v = item.split(":", 1)
        out[k.strip()] = parse_flow_value(v)
    return out


def scalar(text: str, key: str):
    """`key: value` at column 0 — used for top-level scalars like `genre:`."""
    m = re.search(rf"^{re.escape(key)}:[ \t]*(.+?)[ \t]*$", text, re.MULTILINE)
    if not m:
        return None
    return parse_flow_value(m.group(1))


def flow_dict_field(text: str, key: str):
    """`key: {...}` on one line at column 0 — used for e.g. `time_shape:`."""
    m = re.search(rf"^{re.escape(key)}:[ \t]*(\{{.*\}})[ \t]*$", text, re.MULTILINE)
    if not m:
        return None
    return parse_flow_dict(m.group(1))


def block_list(text: str, key: str) -> tuple[list[dict], bool]:
    """`key:` at column 0 followed by `  - {...}` lines. Returns (items, key_found)."""
    lines = text.splitlines()
    items: list[dict] = []
    found = False
    in_block = False
    for line in lines:
        if not in_block:
            if re.match(rf"^{re.escape(key)}:[ \t]*(#.*)?$", line):
                in_block = True
                found = True
            continue
        stripped = line.strip()
        if not stripped or stripped.startswith("#"):
            continue
        m = re.match(r"^-\s*(\{.*\})\s*$", stripped)
        if m:
            items.append(parse_flow_dict(m.group(1)))
            continue
        break  # first non-item, non-blank line ends the block
    return items, found


def check_list_syntax(text: str) -> list[tuple[str, str]]:
    """Reject rows the restricted parser would otherwise silently skip."""
    findings = []
    keys = {"slots", "number_gate", "glossary", "arc"}
    active = None
    for n, line in enumerate(text.splitlines(), 1):
        stripped = line.strip()
        if not stripped or stripped.startswith("#"):
            continue
        if line == line.lstrip():
            if active and stripped.startswith("-"):
                # A column-0 sequence row is valid YAML and still belongs to the
                # active list key; validate it like an indented row rather than
                # letting it end the block (and hide it and its continuations).
                if not re.fullmatch(r"-\s*\{.*\}", stripped):
                    findings.append(("list-syntax", f"line {n}: unsupported {active} row; use one complete '- {{key: value, ...}}' per line"))
                continue
            match = re.match(r"^([a-z_]+):[ \t]*(.*)$", line)
            active = match.group(1) if match and match.group(1) in keys else None
            value = match.group(2).strip() if match else ""
            if active and value and not value.startswith("#"):
                findings.append(("list-syntax", f"line {n}: {active} requires a block sequence of single-line flow mappings"))
            continue
        if active and not re.fullmatch(r"-\s*\{.*\}", stripped):
            findings.append(("list-syntax", f"line {n}: unsupported {active} row; use one complete '- {{key: value, ...}}' per line"))
    return findings


# ---------------------------------------------------------------- figure + design-leak scan

FIGURE_RE = re.compile(r"\$[0-9]{1,3}(?:,[0-9]{3})*(?:\.[0-9]+)?|[0-9]+(?:\.[0-9]+)?%")

BANNED_PHRASES = [
    # chart types
    "bar chart", "pie chart", "line chart", "donut chart", "area chart",
    "scatter plot", "gantt chart", "flowchart",
    # layout words
    "two-column", "three-column", "full-bleed", "hero image", "sidebar",
    "grid layout", "card layout", "split-screen",
    # font names
    "helvetica", "arial", "times new roman", "georgia", "roboto",
    "montserrat", "inter", "ibm plex",
]
HEX_RE = re.compile(r"#[0-9a-fA-F]{6}\b|#[0-9a-fA-F]{3}\b")


def find_banned_tokens(text: str) -> list[str]:
    if not text:
        return []
    found = []
    low = text.lower()
    for phrase in BANNED_PHRASES:
        if re.search(r"\b" + re.escape(phrase) + r"\b", low):
            found.append(phrase)
    found.extend(m.group(0) for m in HEX_RE.finditer(text))
    return found


def slot_text(slot: dict) -> str:
    return f"{slot.get('claim') or ''} {slot.get('supporting') or ''}"


# ---------------------------------------------------------------- checks

Finding = tuple[str, str]


def check_number_gate(slots, number_gate, ng_found) -> list[Finding]:
    findings: list[Finding] = []
    if not ng_found:
        findings.append(("number_gate-missing", "required block 'number_gate' is missing from staging.yaml"))
        return findings

    valid_verdicts = {"ship", "ship_with_as_of", "withhold"}
    for row in number_gate:
        figure = row.get("figure")
        verdict = row.get("verdict")
        if verdict not in valid_verdicts:
            findings.append((
                "number_gate-invalid-verdict",
                f"number_gate row for figure {figure!r} has invalid verdict {verdict!r} "
                f"(must be one of {sorted(valid_verdicts)})",
            ))
        if verdict == "withhold" and not row.get("reason"):
            findings.append((
                "number_gate-reason-required",
                f"number_gate row for figure {figure!r} (verdict=withhold) has an empty 'reason'",
            ))
        if verdict == "ship_with_as_of" and not row.get("as_of"):
            findings.append((
                "number_gate-asof-required",
                f"number_gate row for figure {figure!r} (verdict=ship_with_as_of) has an empty 'as_of'",
            ))

    # conflict: same figure + same source, disagreeing value ⇒ both must be withhold
    by_figure_source = defaultdict(list)
    for row in number_gate:
        by_figure_source[(row.get("figure"), row.get("source"))].append(row)
    for (figure, source), rows in by_figure_source.items():
        values = {r.get("value") for r in rows}
        if len(values) > 1 and not all(r.get("verdict") == "withhold" for r in rows):
            findings.append((
                "number_gate-conflict",
                f"figure {figure!r} has conflicting values from source {source!r} "
                f"({sorted(v for v in values if v)}) — both rows must be verdict: withhold",
            ))

    # every figure token in claim/supporting text needs a backing row
    for slot in slots:
        text = slot_text(slot)
        for tok in {m.group(0) for m in FIGURE_RE.finditer(text)}:
            if not any(r.get("value") == tok for r in number_gate):
                findings.append((
                    "number_gate-unbacked-figure",
                    f"figure {tok!r} appears on slot {slot.get('n')} with no matching number_gate row",
                ))

    # a withheld figure must never reach a slot
    for row in number_gate:
        if row.get("verdict") != "withhold":
            continue
        value = row.get("value")
        if not value:
            continue
        for slot in slots:
            text = slot_text(slot)
            if any(m.group(0) == value for m in FIGURE_RE.finditer(text)):
                findings.append((
                    "number_gate-withheld-on-slot",
                    f"withheld figure {value!r} ({row.get('figure')!r}) reached slot {slot.get('n')}",
                ))

    return findings


def check_glossary(slots, glossary, gl_found) -> list[Finding]:
    findings: list[Finding] = []
    if not gl_found:
        findings.append(("glossary-missing", "required block 'glossary' is missing from staging.yaml"))
        return findings

    valid_verdicts = {"keep_gloss_once", "rename", "drop", "internal_only", "banned"}
    for row in glossary:
        term = row.get("term")
        verdict = row.get("verdict")
        if not term:
            continue
        if verdict not in valid_verdicts:
            findings.append((
                "glossary-invalid-verdict",
                f"glossary row for {term!r} has invalid verdict {verdict!r} (must be one of {sorted(valid_verdicts)})",
            ))
            continue

        pattern = re.compile(r"\b" + re.escape(term) + r"\b", re.IGNORECASE)

        if verdict in ("rename", "drop", "banned"):
            for slot in slots:
                if pattern.search(slot_text(slot)):
                    findings.append((
                        "glossary-banned-term-present",
                        f"term {term!r} (verdict={verdict}) appears on slot {slot.get('n')} but must not appear anywhere",
                    ))
        elif verdict == "keep_gloss_once":
            first = next((s for s in slots if pattern.search(slot_text(s))), None)
            if first is not None:
                outbound = row.get("outbound") or ""
                if outbound and outbound not in slot_text(first):
                    findings.append((
                        "glossary-missing-gloss",
                        f"term {term!r} first occurs at slot {first.get('n')} without its outbound "
                        f"gloss present in that same slot",
                    ))
        # internal_only: no positive or negative appearance rule stated by the brief

    return findings


def check_diagram(slots) -> list[Finding]:
    findings: list[Finding] = []
    for slot in slots:
        diagram = slot.get("diagram")
        if not diagram:
            continue
        if slot.get("function") != "mechanism":
            findings.append((
                "diagram-illegal-function",
                f"slot {slot.get('n')}: 'diagram' is only legal on function: mechanism slots "
                f"(found function: {slot.get('function')!r})",
            ))
        tokens = find_banned_tokens(diagram)
        if tokens:
            findings.append((
                "diagram-banned-token",
                f"slot {slot.get('n')}: diagram prose names a banned design token {tokens[0]!r}",
            ))
    return findings


def check_product_pitch_gates(genre, slots, number_gate, arc, arc_found, time_shape) -> list[Finding]:
    findings: list[Finding] = []
    if genre != "product-pitch":
        return findings

    # mechanism_by: first mechanism-function slot ≤35% of content slots (appendix excluded)
    content_slots = [s for s in slots if s.get("function") != "appendix"]
    mech_positions = [i + 1 for i, s in enumerate(content_slots) if s.get("function") == "mechanism"]
    if mech_positions and content_slots:
        first = mech_positions[0]
        pct = 100.0 * first / len(content_slots)
        if pct > 35.0:
            findings.append((
                "mechanism_by",
                f"mechanism_by: first mechanism slot is {first} of {len(content_slots)} content "
                f"slots = {pct:.1f}% (ceiling is 35%)",
            ))

    # problem_share: the 'problem' beat holds 30-45% of the time budget
    if arc_found and time_shape and time_shape.get("minutes") is not None:
        try:
            total = float(time_shape["minutes"])
            problem_minutes = sum(
                float(b["minutes"]) for b in arc if b.get("beat") == "problem" and b.get("minutes") is not None
            )
        except (TypeError, ValueError):
            total = 0.0
            problem_minutes = 0.0
        if total > 0:
            share = problem_minutes / total
            if not (0.30 <= share <= 0.45):
                findings.append((
                    "problem_share",
                    f"problem_share: the problem beat holds {share * 100:.1f}% of the "
                    f"{total:g}-minute budget (must be 30-45%)",
                ))

    # cost_slot: exactly one `cost` slot, its figure backed by a ship/ship_with_as_of row
    cost_slots = [s for s in slots if s.get("function") == "cost"]
    if len(cost_slots) != 1:
        findings.append((
            "cost_slot",
            f"cost_slot: expected exactly one function: cost slot for product-pitch, found {len(cost_slots)}",
        ))
    else:
        cslot = cost_slots[0]
        toks = {m.group(0) for m in FIGURE_RE.finditer(slot_text(cslot))}
        backed = any(
            row.get("value") in toks and row.get("verdict") in ("ship", "ship_with_as_of")
            for row in number_gate
        )
        if not backed:
            findings.append((
                "cost_slot",
                f"cost_slot: the cost slot (n={cslot.get('n')}) carries no figure backed by a "
                f"ship/ship_with_as_of number_gate row",
            ))

    return findings


# ---------------------------------------------------------------- main


def lint(text: str) -> list[Finding]:
    findings: list[Finding] = check_list_syntax(text)
    if findings:
        return findings

    genre = scalar(text, "genre")
    slots, slots_found = block_list(text, "slots")
    number_gate, ng_found = block_list(text, "number_gate")
    glossary, gl_found = block_list(text, "glossary")
    arc, arc_found = block_list(text, "arc")
    time_shape = flow_dict_field(text, "time_shape")

    if slots_found and not slots:
        findings.append(("slots-empty", "required block slots contains no parsed rows"))
    if not slots_found:
        findings.append(("slots-missing", "required block 'slots' is missing from staging.yaml"))

    findings.extend(check_number_gate(slots, number_gate, ng_found))
    findings.extend(check_glossary(slots, glossary, gl_found))
    findings.extend(check_diagram(slots))
    findings.extend(check_product_pitch_gates(genre, slots, number_gate, arc, arc_found, time_shape))

    return findings


def main(argv: list[str] | None = None) -> int:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("contract", help="path to a staged staging.yaml")
    args = ap.parse_args(argv)

    path = Path(args.contract)
    try:
        text = path.read_text(encoding="utf-8")
    except OSError as exc:
        print(f"ERROR: could not read {path}: {exc}", file=sys.stderr)
        return 2

    findings = lint(text)

    if findings:
        for rule_id, message in findings:
            print(f"FAIL {rule_id}: {message}")
        print(f"{len(findings)} error(s) found in {path}")
        return 1

    slots, _ = block_list(text, "slots")
    number_gate, _ = block_list(text, "number_gate")
    glossary, _ = block_list(text, "glossary")
    print(
        f"PASS: {path} conforms — {len(slots)} slots, {len(number_gate)} number_gate rows, "
        f"{len(glossary)} glossary rows checked, 0 errors"
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())

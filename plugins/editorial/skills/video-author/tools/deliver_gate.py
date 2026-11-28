#!/usr/bin/env python3
"""The delivered-file gate — the hard fail the render Job runs on the ENCODED
mp4 before it is uploaded (brief explainer-video/03).

The single point of failure this whole brief guards against is the recorder
silently degrading under CPU contention: a throttled pod drops frames without
failing, so `record.mjs` exits 0 on a film that is too short or wrong. Guaranteed
QoS mitigates the cause; THIS is the delivered-artifact check that catches the
symptom before a degraded film is ever uploaded. It fails CLOSED — an
out-of-bounds duration or loudness aborts the job (no upload) rather than
shipping the bad take.

Two bounds, both derived from the render's own measured SCENE_DURATIONS (the
list `assemble_audio` emits and the pipeline injects into the slides):

  duration   the encoded file's length must equal the sum of the scene
             durations + the lead-in + a load-skew allowance, within a
             tolerance. Too SHORT => the recorder was truncated (the incident
             this brief exists for). Too LONG => a runaway/looping render.
  loudness   the integrated loudness (ffmpeg ebur128) must sit inside the house
             band around the -16 LUFS the audio was normalized to. Outside the
             band means the mux mangled the normalized track.

The decision functions are PURE (no I/O) so they are unit-tested offline; the
`probe_*` helpers wrap ffprobe/ffmpeg and are the only part that needs a real
file + binaries.
"""
from __future__ import annotations

import re
import subprocess
from dataclasses import dataclass
from pathlib import Path

# --- house bounds -----------------------------------------------------------
# The recording starts before scene 1 (page load + boot), so the file is a
# little longer than the pure take. The gate allows that as a one-sided skew on
# the SHORT side (never negative) and a symmetric tolerance on the long side.
LEAD_S = 0.35          # matches assemble_audio.LEAD_S / gen_subs.LEAD_S
DEFAULT_TAIL_S = 1.0   # matches assemble_audio.TAIL_S (trailing silence)
# How far the delivered length may sit under take+lead before it reads as a
# truncated (dropped-frame) render, and how far over before it reads as runaway.
DURATION_UNDER_TOL_S = 0.75
DURATION_OVER_TOL_S = 8.0     # generous: page-load skew varies pod to pod

# assemble_audio normalizes narration to I=-16 LUFS; the AAC mux preserves it.
TARGET_LUFS = -16.0
LOUDNESS_TOL_LU = 2.0         # -18.0 .. -14.0 LUFS is in-band


@dataclass
class GateResult:
    ok: bool
    detail: str


def expected_span(scene_durations: list[float], lead_s: float = LEAD_S) -> float:
    """The nominal delivered length: lead-in + the sum of every scene's duration.

    `scene_durations` already fold the inter-scene gaps and the final tail into
    each entry (see assemble_audio.assemble), so summing them + the lead-in is
    the whole timeline.
    """
    if not scene_durations:
        raise ValueError("scene_durations is empty — nothing to gate against")
    if any((not isinstance(d, (int, float)) or d <= 0) for d in scene_durations):
        raise ValueError(f"scene_durations must be positive numbers: {scene_durations!r}")
    return lead_s + sum(scene_durations)


def check_duration(video_len_s: float, scene_durations: list[float],
                   lead_s: float = LEAD_S,
                   under_tol_s: float = DURATION_UNDER_TOL_S,
                   over_tol_s: float = DURATION_OVER_TOL_S) -> GateResult:
    """Pure decision: is the delivered length within [span-under, span+over]?"""
    span = expected_span(scene_durations, lead_s)
    low = span - under_tol_s
    high = span + over_tol_s
    ok = low <= video_len_s <= high
    if ok:
        detail = (f"duration {video_len_s:.2f}s in band "
                  f"[{low:.2f}, {high:.2f}] (expected span {span:.2f}s)")
    elif video_len_s < low:
        detail = (f"duration {video_len_s:.2f}s is SHORT — below {low:.2f}s "
                  f"(expected span {span:.2f}s); likely a truncated/dropped-frame "
                  f"render under CPU contention")
    else:
        detail = (f"duration {video_len_s:.2f}s is LONG — above {high:.2f}s "
                  f"(expected span {span:.2f}s); likely a runaway render")
    return GateResult(ok, detail)


def check_loudness(integrated_lufs: float,
                   target_lufs: float = TARGET_LUFS,
                   tol_lu: float = LOUDNESS_TOL_LU) -> GateResult:
    """Pure decision: is the integrated loudness inside the house band?"""
    low = target_lufs - tol_lu
    high = target_lufs + tol_lu
    ok = low <= integrated_lufs <= high
    detail = (f"integrated loudness {integrated_lufs:.2f} LUFS "
              f"{'in' if ok else 'OUTSIDE'} band [{low:.1f}, {high:.1f}] "
              f"(target {target_lufs:.1f})")
    return GateResult(ok, detail)


# --- probes (the only I/O; need ffprobe/ffmpeg + a real file) ---------------

def probe_duration(mp4: Path) -> float:
    out = subprocess.run(
        ["ffprobe", "-v", "error", "-show_entries", "format=duration",
         "-of", "default=noprint_wrappers=1:nokey=1", str(mp4)],
        check=True, capture_output=True, text=True,
    )
    return float(out.stdout.strip())


_LUFS_RE = re.compile(r"I:\s*(-?\d+(?:\.\d+)?)\s*LUFS")


def probe_loudness(mp4: Path) -> float:
    """Integrated loudness (LUFS) of the mp4's audio via ffmpeg's ebur128 filter.

    ebur128 prints a Summary block to stderr ending in `I: -16.0 LUFS`; we parse
    the last `I:` line (the integrated value over the whole file).
    """
    proc = subprocess.run(
        ["ffmpeg", "-nostats", "-i", str(mp4),
         "-filter_complex", "ebur128", "-f", "null", "-"],
        capture_output=True, text=True,
    )
    matches = _LUFS_RE.findall(proc.stderr)
    if not matches:
        raise RuntimeError(
            f"could not parse integrated loudness from ffmpeg ebur128 output for {mp4}"
        )
    return float(matches[-1])


def run_gate(mp4: Path, scene_durations: list[float]) -> list[GateResult]:
    """Run every delivered-file check against a real mp4; returns all results.

    The caller (the render Job) fails the job if ANY result is not ok, AFTER
    logging each detail line, so the human sees exactly which bound was missed.
    """
    results = [
        check_duration(probe_duration(mp4), scene_durations),
        check_loudness(probe_loudness(mp4)),
    ]
    return results


if __name__ == "__main__":  # pragma: no cover - CLI wrapper for the render Job
    import sys
    if len(sys.argv) < 3:
        raise SystemExit(
            "usage: deliver_gate.py <video.mp4> <scene_durations csv, e.g. 10.3,16.6,...>")
    mp4 = Path(sys.argv[1])
    durations = [float(x) for x in sys.argv[2].split(",") if x.strip()]
    results = run_gate(mp4, durations)
    failed = False
    for r in results:
        print(("PASS " if r.ok else "FAIL ") + r.detail)
        failed = failed or not r.ok
    if failed:
        raise SystemExit("delivered-file gate FAILED — not uploading")
    print("delivered-file gate PASSED")

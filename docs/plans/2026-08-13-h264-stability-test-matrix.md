# H.264 Stability Test Matrix

Acceptance criteria from `docs/plans/2026-06-23-hardening-maintenance-plan.md`:

- `H.264 bitrate-skift staller ikke capture-loopet`
- `H.264 frame boundaries er robuste under load`

These are **runtime/quality** criteria that require a real encoder pipeline
(NVENC on Windows, VideoToolbox on macOS) and cannot be asserted in CI on
Linux. The pure-Go Annex-B frame-boundary parser is covered by unit tests in
`agent/internal/video/encoder/annexb/`. This document defines the manual
matrix that closes the two open acceptance criteria on a Windows/macOS runner.

## Prerequisites

- Agent host with the target encoder:
  - Windows: NVIDIA GPU + `h264_nvenc` (FFmpeg ≥ 4.x with NVENC).
  - macOS: Apple Silicon or Intel with HW H.264 block (VideoToolbox).
- Controller build matching the agent version under test.
- A device with a real desktop (not the 640×480 Basic Display Adapter).
- Optional: `WIN11DL` / `WIN-TEST` as the remote target.

## Mode under test

For each scenario run all three stream modes: `h264`, `hybrid`, `tiles`
(tiles is the control/baseline).

## T1 — Bitrate change does not stall the capture loop

**Closes:** `H.264 bitrate-skift staller ikke capture-loopet`

1. Connect controller → agent, switch to `h264`.
2. Confirm moving video (agent log: `H.264 tilstand aktiveret`; controller
   log: `H.264 decoded frame #…` with increasing frame numbers).
3. From the dashboard/controller cycle the quality preset
   (low → medium → high → low). Each change sends `set_stream_params` /
   `set_mode` with a new H.264 bitrate.
4. **Pass criteria** (all must hold):
   - Video keeps moving throughout the change (no ≥1s freeze).
   - No `capture-loop stalled` / `NVENC: dropping … pending bytes` spam in
     the agent log at the moment of the change.
   - `SetBitrate` on NVENC logs `effective on next encoder restart` (expected
     by-design message) and does NOT restart FFmpeg mid-session.
5. Repeat 5 bitrate cycles back-to-back.

## T2 — Frame boundaries robust under CPU pressure

**Closes:** `H.264 frame boundaries er robuste under load`

1. Connect in `h264` at 1080p, high preset.
2. On the agent host, pin CPU near 100% (e.g. start several `while true`
   loops / a CPU burner) leaving the agent's priority normal.
3. Drive rapid screen change on the remote desktop: open/close Total
   Commander fullscreen, drag large windows, scroll long pages.
4. **Pass criteria**:
   - No persistent smearing/corruption of the lower part of the frame.
   - Agent log shows `popAccessUnitLocked` draining complete access units
     (no `dropping … pending bytes without complete access unit` storm).
   - Within ~2s of stopping the change, video returns to clean state.
5. Back off CPU to ~50% and confirm immediate recovery.

## T3 — Rapid screen change (keyframe/burst path)

1. Connect in `h264`.
2. Trigger large full-screen white changes repeatedly (open/close a white
   fullscreen app, switch virtual desktops).
3. **Pass criteria**: after each change a keyframe restores a clean picture
   within ~1 GOP interval; decoder never latches onto a corrupt reference.

## T4 — H.264 → JPEG fallback

**Closes part of plan line 97 (`Test H.264 fallback til JPEG`).**

1. Connect in `h264`.
2. Force a decode stall: on the agent, kill the NVENC/FFmpeg encoder
   subprocess once, OR block the video track briefly.
3. **Pass criteria**:
   - Dashboard: after ~3.5s with no decoded frame, `scheduleH264Fallback`
     fires and sends `set_mode tiles`; toast `H.264 gav ingen video —
     skifter tilbage til JPEG` appears.
   - Controller: 3s decode watchdog restarts FFmpeg; if still no frames,
     `SetStreamingMode("tiles")` fallback is requested.
4. Confirm re-enabling H.264 later recovers cleanly.

## T5 — Resolution change mid-session

1. Connect in `h264` at one resolution.
2. Change remote display resolution (or dummy-plug behavior changes it).
3. **Pass criteria**: encoder reinitializes for the new frame size without a
   crash; video resumes after ≤1 keyframe.

## Evidence to capture per run

- Agent log excerpt (filter: `H.264`, `NVENC`, `popAccessUnit`, `drop`).
- Controller log excerpt (`H.264 decoded frame`, `RTP packets received`).
- For dashboard: browser devtools console around `scheduleH264Fallback`.
- A short screen recording or before/after screenshots for smearing checks.

## Status

| Test | Owner | Runner needed | Status |
|------|-------|---------------|--------|
| T1 bitrate stall | manual | Windows/macOS | pending |
| T2 frame boundaries under CPU | manual | Windows/macOS | pending |
| T3 rapid screen change | manual | Windows/macOS | pending |
| T4 H.264→JPEG fallback | manual | Windows/macOS | pending |
| T5 resolution change | manual | Windows/macOS | pending |

Automated runtime coverage of T1–T5 requires a Windows/macOS CI runner with
the target encoder; the Linux CI path can only assert the Annex-B parser unit
tests (already green).

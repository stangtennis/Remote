#!/usr/bin/env python3
"""Generate professional icons for Remote Desktop agent and controller.

Agent: Monitor with incoming signal waves (teal/blue) - this machine receives connections
Controller: Monitor with outgoing cursor arrow (indigo/purple) - controls another machine

Uses 4x supersampling for anti-aliased results at small sizes.
"""

from PIL import Image, ImageDraw
import math
import os

SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
SIZES = [16, 32, 48, 256]
SUPERSAMPLE = 4  # Draw at 4x, downscale for AA


def draw_monitor(draw, size, bezel_color, screen_color, stand_color):
    """Draw a modern flat monitor shape."""
    s = size
    pad = s * 0.06  # outer padding

    # Monitor body (rounded rect)
    mon_left = pad
    mon_top = pad
    mon_right = s - pad
    mon_bottom = s * 0.72
    radius = s * 0.06

    # Shadow (subtle)
    shadow_off = s * 0.015
    draw.rounded_rectangle(
        [mon_left + shadow_off, mon_top + shadow_off, mon_right + shadow_off, mon_bottom + shadow_off],
        radius=radius, fill=(0, 0, 0, 40)
    )

    # Monitor bezel
    draw.rounded_rectangle(
        [mon_left, mon_top, mon_right, mon_bottom],
        radius=radius, fill=bezel_color
    )

    # Screen area (inset)
    scr_inset = s * 0.08
    scr_left = mon_left + scr_inset
    scr_top = mon_top + scr_inset
    scr_right = mon_right - scr_inset
    scr_bottom = mon_bottom - scr_inset
    inner_radius = max(1, radius - s * 0.02)
    draw.rounded_rectangle(
        [scr_left, scr_top, scr_right, scr_bottom],
        radius=inner_radius, fill=screen_color
    )

    # Stand neck
    neck_w = s * 0.08
    neck_h = s * 0.10
    neck_left = s / 2 - neck_w / 2
    neck_top = mon_bottom
    draw.rectangle(
        [neck_left, neck_top, neck_left + neck_w, neck_top + neck_h],
        fill=stand_color
    )

    # Stand base (rounded)
    base_w = s * 0.30
    base_h = s * 0.05
    base_left = s / 2 - base_w / 2
    base_top = neck_top + neck_h
    draw.rounded_rectangle(
        [base_left, base_top, base_left + base_w, base_top + base_h],
        radius=max(1, s * 0.02), fill=stand_color
    )

    return scr_left, scr_top, scr_right, scr_bottom


def draw_signal_waves(draw, cx, cy, size, color_base):
    """Draw incoming WiFi/signal waves (concentric arcs)."""
    for i, (r_ratio, alpha) in enumerate([(0.14, 255), (0.22, 200), (0.30, 140)]):
        r = size * r_ratio
        c = (*color_base[:3], alpha)
        line_w = max(2, size * 0.03)

        # Draw arc (right-side quarter circle, like WiFi signal)
        bbox = [cx - r, cy - r, cx + r, cy + r]
        draw.arc(bbox, start=210, end=330, fill=c, width=int(line_w))

    # Center dot
    dot_r = size * 0.04
    draw.ellipse([cx - dot_r, cy - dot_r, cx + dot_r, cy + dot_r], fill=color_base)


def draw_cursor_arrow(draw, x, y, size, color):
    """Draw a classic mouse cursor/arrow pointing outward (upper-right)."""
    s = size * 0.28  # cursor total size

    # Arrow pointing upper-right
    # Main arrow body
    points = [
        (x, y + s),           # bottom-left (tail)
        (x, y),               # top-left (tip)
        (x + s * 0.7, y + s * 0.35),  # right point
    ]

    # Shadow
    shadow = [(px + size * 0.01, py + size * 0.01) for px, py in points]
    draw.polygon(shadow, fill=(0, 0, 0, 80))

    # Main arrow
    draw.polygon(points, fill=color, outline=(255, 255, 255, 200), width=max(1, int(size * 0.015)))


def generate_agent_icon(size):
    """Generate agent icon: monitor with incoming signal waves."""
    canvas_size = size * SUPERSAMPLE
    img = Image.new('RGBA', (canvas_size, canvas_size), (0, 0, 0, 0))
    draw = ImageDraw.Draw(img)

    # Colors - teal/blue theme
    bezel = (30, 60, 80, 255)        # dark teal-gray
    screen = (15, 35, 55, 255)       # very dark blue screen
    stand = (40, 75, 95, 255)        # slightly lighter teal
    signal_color = (0, 200, 220, 255)  # bright teal/cyan

    scr_l, scr_t, scr_r, scr_b = draw_monitor(draw, canvas_size, bezel, screen, stand)

    # Signal waves in center of screen
    scr_cx = (scr_l + scr_r) / 2
    scr_cy = (scr_t + scr_b) / 2
    draw_signal_waves(draw, scr_cx, scr_cy, canvas_size, signal_color)

    # Downscale with high-quality resampling
    img = img.resize((size, size), Image.LANCZOS)
    return img


def generate_controller_icon(size):
    """Generate controller icon: monitor with outgoing cursor."""
    canvas_size = size * SUPERSAMPLE
    img = Image.new('RGBA', (canvas_size, canvas_size), (0, 0, 0, 0))
    draw = ImageDraw.Draw(img)

    # Colors - indigo/purple theme
    bezel = (45, 30, 80, 255)          # dark indigo
    screen = (25, 15, 55, 255)         # very dark purple screen
    stand = (55, 40, 95, 255)          # slightly lighter indigo
    cursor_color = (180, 130, 255, 255)  # bright lavender/purple

    scr_l, scr_t, scr_r, scr_b = draw_monitor(draw, canvas_size, bezel, screen, stand)

    # Cursor in center of screen
    cursor_x = (scr_l + scr_r) / 2 - canvas_size * 0.06
    cursor_y = (scr_t + scr_b) / 2 - canvas_size * 0.05
    draw_cursor_arrow(draw, cursor_x, cursor_y, canvas_size, cursor_color)

    # Small connection lines emanating from cursor (showing "outgoing control")
    cx = cursor_x + canvas_size * 0.12
    cy = cursor_y + canvas_size * 0.04
    line_color = (180, 130, 255, 140)
    line_w = max(1, int(canvas_size * 0.015))
    for angle_deg in [20, 45, 70]:
        angle = math.radians(angle_deg)
        length = canvas_size * 0.10
        ex = cx + math.cos(angle) * length
        ey = cy - math.sin(angle) * length
        draw.line([(cx, cy), (ex, ey)], fill=line_color, width=line_w)
        # Small dot at end
        dot_r = canvas_size * 0.012
        draw.ellipse([ex - dot_r, ey - dot_r, ex + dot_r, ey + dot_r], fill=cursor_color)

    # Downscale
    img = img.resize((size, size), Image.LANCZOS)
    return img


def save_ico(images, path):
    """Save multiple PIL images as a multi-size ICO file."""
    # PIL can save ICO directly with multiple sizes
    images[0].save(path, format='ICO', sizes=[(img.size[0], img.size[1]) for img in images],
                   append_images=images[1:])


def main():
    print("Generating agent icons...")
    agent_images = [generate_agent_icon(s) for s in SIZES]
    agent_ico_path = os.path.join(SCRIPT_DIR, 'agent.ico')
    save_ico(agent_images, agent_ico_path)
    print(f"  -> {agent_ico_path}")

    agent_png_path = os.path.join(SCRIPT_DIR, 'agent.png')
    agent_images[-1].save(agent_png_path, format='PNG')  # 256x256
    print(f"  -> {agent_png_path}")

    print("Generating controller icons...")
    ctrl_images = [generate_controller_icon(s) for s in SIZES]
    ctrl_ico_path = os.path.join(SCRIPT_DIR, 'controller.ico')
    save_ico(ctrl_images, ctrl_ico_path)
    print(f"  -> {ctrl_ico_path}")

    ctrl_png_path = os.path.join(SCRIPT_DIR, 'controller.png')
    ctrl_images[-1].save(ctrl_png_path, format='PNG')  # 256x256
    print(f"  -> {ctrl_png_path}")

    print("Done!")


if __name__ == '__main__':
    main()

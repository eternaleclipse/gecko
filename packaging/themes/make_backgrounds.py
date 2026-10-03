#!/usr/bin/env python3
"""Draws Gecko's theme backgrounds as SVG (original artwork, no photos).

    python3 packaging/themes/make_backgrounds.py   # writes web/public/themes/*.svg
"""
import math
import os
import random

OUT = os.path.join(os.path.dirname(__file__), "..", "..", "web", "public", "themes")
W, H = 1920, 1080


def svg(body, defs=""):
    return (f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {W} {H}" '
            f'preserveAspectRatio="xMidYMid slice"><defs>{defs}</defs>{body}</svg>\n')


def fuji():
    rnd = random.Random(7)
    defs = """
<linearGradient id="sky" x1="0" y1="0" x2="0" y2="1">
  <stop offset="0" stop-color="#121731"/><stop offset=".42" stop-color="#352f5c"/>
  <stop offset=".66" stop-color="#8e5b7a"/><stop offset=".8" stop-color="#d98e7d"/>
  <stop offset=".86" stop-color="#f1b48c"/><stop offset="1" stop-color="#f6cf9f"/></linearGradient>
<radialGradient id="sunglow"><stop offset="0" stop-color="#ffe2b8" stop-opacity=".9"/>
  <stop offset=".25" stop-color="#ffcf9e" stop-opacity=".45"/><stop offset="1" stop-color="#f1a07f" stop-opacity="0"/></radialGradient>
<linearGradient id="mtn" x1="0" y1="0" x2="1" y2="1">
  <stop offset="0" stop-color="#4a4a80"/><stop offset=".55" stop-color="#2f3163"/><stop offset="1" stop-color="#1e2046"/></linearGradient>
<linearGradient id="snow" x1="0" y1="0" x2="1" y2="0">
  <stop offset="0" stop-color="#fbf3f1"/><stop offset=".55" stop-color="#ecdfe6"/><stop offset="1" stop-color="#b9b3d6"/></linearGradient>
<linearGradient id="lake" x1="0" y1="0" x2="0" y2="1">
  <stop offset="0" stop-color="#3a3360"/><stop offset=".25" stop-color="#232445"/><stop offset="1" stop-color="#0b0e22"/></linearGradient>
<linearGradient id="fade" x1="0" y1="0" x2="0" y2="1">
  <stop offset="0" stop-color="#fff" stop-opacity=".55"/><stop offset=".55" stop-color="#fff" stop-opacity="0"/></linearGradient>
<mask id="reflmask"><rect x="0" y="905" width="1920" height="175" fill="url(#fade)"/></mask>
<filter id="soft" x="-20%" y="-20%" width="140%" height="140%"><feGaussianBlur stdDeviation="2"/></filter>
<filter id="haze" x="-10%" y="-50%" width="120%" height="200%"><feGaussianBlur stdDeviation="14"/></filter>"""
    mountain = ("M430 905 C 700 760, 860 520, 924 392 L 948 372 L 972 382 L 990 368 L 1014 380 "
                "C 1080 520, 1240 760, 1500 905 Z")
    snow = ("M924 392 L 948 372 L 972 382 L 990 368 L 1014 380 C 1040 430, 1068 480, 1098 528 "
            "L 1076 520 L 1066 552 L 1044 528 L 1032 566 L 1012 536 L 996 574 L 980 540 L 962 580 "
            "L 948 542 L 930 568 L 918 534 L 896 556 L 884 530 L 862 546 C 880 500, 902 450, 924 392 Z")
    ridges = ""
    for layer, (y0, amp, col, op) in enumerate([(820, 46, "#5b4f87", .55), (860, 38, "#3a3567", .8), (900, 26, "#1d1e40", 1)]):
        pts, x = [], -20
        while x <= W + 40:
            pts.append(f"{x:.0f} {y0 - amp * (0.5 + 0.5 * math.sin(x / (170 + 60 * layer) + layer)) - rnd.uniform(0, amp * .5):.0f}")
            x += rnd.uniform(60, 140)
        ridges += f'<path d="M-20 {H} L {" L ".join(pts)} L {W + 40} {H} Z" fill="{col}" opacity="{op}"/>'
    clouds = "".join(
        f'<ellipse cx="{rnd.uniform(0, W):.0f}" cy="{rnd.uniform(560, 760):.0f}" rx="{rnd.uniform(160, 420):.0f}" '
        f'ry="{rnd.uniform(6, 14):.0f}" fill="#ffd8bd" opacity="{rnd.uniform(.08, .22):.2f}" filter="url(#haze)"/>'
        for _ in range(9))
    stars = "".join(
        f'<circle cx="{rnd.uniform(0, W):.0f}" cy="{rnd.uniform(0, 330):.0f}" r="{rnd.uniform(.6, 1.6):.1f}" '
        f'fill="#fff" opacity="{rnd.uniform(.25, .7):.2f}"/>' for _ in range(70))
    ripples = "".join(
        f'<rect x="{rnd.uniform(-100, W):.0f}" y="{rnd.uniform(915, 1075):.0f}" width="{rnd.uniform(60, 380):.0f}" height="1.4" '
        f'fill="#f6cf9f" opacity="{rnd.uniform(.05, .18):.2f}"/>' for _ in range(70))
    pines = ""
    for x0, x1 in ((-20, 360), (1580, 1940)):
        x = x0
        while x < x1:
            h = rnd.uniform(40, 95)
            w = h * .38
            pines += f'<path d="M{x:.0f} 908 L {x + w / 2:.0f} {908 - h:.0f} L {x + w:.0f} 908 Z" fill="#0b0d22"/>'
            x += rnd.uniform(14, 34)
    scene = (f'<path d="{mountain}" fill="url(#mtn)"/><path d="{snow}" fill="url(#snow)"/>'
             '<path d="M990 368 C 1060 470, 1180 690, 1500 905 L 1300 905 C 1150 760, 1050 560, 1014 380 Z" fill="#141633" opacity=".28"/>')
    body = (f'<rect width="{W}" height="{H}" fill="url(#sky)"/>{stars}'
            f'<circle cx="1385" cy="640" r="260" fill="url(#sunglow)"/><circle cx="1385" cy="640" r="74" fill="#ffe1b4" opacity=".92"/>'
            f'{clouds}<g>{scene}</g>{ridges}'
            f'<rect x="0" y="905" width="{W}" height="{H - 905}" fill="url(#lake)"/>'
            f'<g mask="url(#reflmask)" opacity=".75"><g transform="translate(0 1810) scale(1 -1)" filter="url(#soft)">{scene}'
            f'<circle cx="1385" cy="640" r="74" fill="#ffe1b4" opacity=".6"/></g></g>{ripples}{pines}'
            f'<rect x="0" y="903" width="{W}" height="3" fill="#0b0d22"/>')
    return svg(body, defs)


def jellyfish():
    rnd = random.Random(11)
    defs = """
<radialGradient id="sea" cx=".5" cy="-.1" r="1.25">
  <stop offset="0" stop-color="#14587a"/><stop offset=".35" stop-color="#0a2f4a"/>
  <stop offset=".7" stop-color="#041829"/><stop offset="1" stop-color="#010810"/></radialGradient>
<linearGradient id="ray" x1="0" y1="0" x2="0" y2="1">
  <stop offset="0" stop-color="#bff4ff" stop-opacity=".16"/><stop offset="1" stop-color="#bff4ff" stop-opacity="0"/></linearGradient>
<filter id="glow" x="-60%" y="-60%" width="220%" height="220%"><feGaussianBlur stdDeviation="18"/></filter>
<filter id="blur2" x="-20%" y="-20%" width="140%" height="140%"><feGaussianBlur stdDeviation="1.4"/></filter>
<filter id="far" x="-30%" y="-30%" width="160%" height="160%"><feGaussianBlur stdDeviation="3"/></filter>"""
    for name, (inner, mid) in {"pink": ("#ffe0f4", "#e06ad0"), "cyan": ("#e2fdff", "#3fc9de"),
                               "violet": ("#efe2ff", "#9a6cf0")}.items():
        defs += (f'<radialGradient id="bell-{name}" cx=".5" cy=".75" r=".75">'
                 f'<stop offset="0" stop-color="{inner}" stop-opacity=".9"/><stop offset=".45" stop-color="{mid}" stop-opacity=".55"/>'
                 f'<stop offset="1" stop-color="{mid}" stop-opacity=".08"/></radialGradient>')

    def jelly(cx, cy, s, color, tint, opacity=1.0, far=False):
        bell = ("M-60 0 C -62 -48, -36 -78, 0 -80 C 36 -78, 62 -48, 60 0 "
                "C 48 8, 40 2, 30 7 C 20 12, 10 4, 0 9 C -10 4, -20 12, -30 7 C -40 2, -48 8, -60 0 Z")
        parts = [f'<path d="{bell}" fill="{tint}" opacity=".55" filter="url(#glow)"/>',
                 f'<path d="{bell}" fill="url(#bell-{color})"/>',
                 '<path d="M-44 -6 C -40 -50, -16 -66, 0 -68 C 16 -66, 40 -50, 44 -6" fill="none" stroke="#fff" stroke-opacity=".35" stroke-width="1.5"/>']
        for i in range(9):  # tentacles
            x = -52 + i * 13
            length = rnd.uniform(220, 420)
            d, y = f"M{x:.0f} 6", 6
            while y < length:
                y2 = y + rnd.uniform(30, 60)
                d += f" Q {x + rnd.uniform(-22, 22):.0f} {(y + y2) / 2:.0f} {x + rnd.uniform(-10, 10):.0f} {y2:.0f}"
                y = y2
            parts.append(f'<path d="{d}" fill="none" stroke="{tint}" stroke-opacity="{rnd.uniform(.25, .5):.2f}" stroke-width="{rnd.uniform(1, 2.4):.1f}" stroke-linecap="round"/>')
        for i in range(4):  # frilly oral arms
            x = -18 + i * 12
            d = f"M{x} 8 " + " ".join(f"Q {x + (14 if k % 2 else -14)} {8 + k * 26 + 13} {x} {8 + (k + 1) * 26}" for k in range(rnd.randint(4, 7)))
            parts.append(f'<path d="{d}" fill="none" stroke="{tint}" stroke-opacity=".55" stroke-width="5" stroke-linecap="round"/>')
        filt = ' filter="url(#far)"' if far else ''
        return f'<g transform="translate({cx} {cy}) scale({s}) rotate({rnd.uniform(-12, 12):.0f})" opacity="{opacity}"{filt}>{"".join(parts)}</g>'

    rays = "".join(
        f'<path d="M{x:.0f} -10 L {x + w:.0f} -10 L {x + w * 3 + 180:.0f} {H} L {x + 120:.0f} {H} Z" fill="url(#ray)" opacity="{o:.2f}"/>'
        for x, w, o in [(rnd.uniform(200, 1700), rnd.uniform(40, 120), rnd.uniform(.4, 1)) for _ in range(7)])
    snow = "".join(
        f'<circle cx="{rnd.uniform(0, W):.0f}" cy="{rnd.uniform(0, H):.0f}" r="{rnd.uniform(.8, 2.6):.1f}" '
        f'fill="#d8fbff" opacity="{rnd.uniform(.08, .45):.2f}"/>' for _ in range(140))
    body = (f'<rect width="{W}" height="{H}" fill="url(#sea)"/>{rays}'
            + jelly(260, 260, .45, "violet", "#c9a8ff", .45, far=True)
            + jelly(1720, 180, .4, "cyan", "#9ff3ff", .4, far=True)
            + jelly(980, 140, .55, "violet", "#c9a8ff", .55, far=True)
            + jelly(560, 520, 1.05, "cyan", "#8ff2ff")
            + jelly(1380, 360, 1.6, "pink", "#ffb0e6")
            + jelly(1760, 760, .8, "violet", "#c9a8ff", .85)
            + jelly(180, 820, .7, "pink", "#ffb0e6", .8)
            + snow)
    return svg(body, defs)


if __name__ == "__main__":
    os.makedirs(OUT, exist_ok=True)
    for name, draw in (("fuji", fuji), ("jellyfish", jellyfish)):
        with open(os.path.join(OUT, name + ".svg"), "w") as f:
            f.write(draw())
        print("wrote", name + ".svg", os.path.getsize(os.path.join(OUT, name + ".svg")) // 1024, "KB")

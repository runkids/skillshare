#!/usr/bin/env python3
"""Export the dashboard's Clean tokens as a Design System tokens.json.

ui/src/components.css stays the source of truth. This prints the tokens.json
the "Skillshare Dashboard" design system artifact holds, so the artifact can be
re-synced after the CSS changes instead of being rebuilt by hand.

    python3 scripts/design/tokens.py [--out FILE]
"""

import argparse
import datetime
import json
from pathlib import Path
import re
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[2]
CSS_PATH = "ui/src/components.css"
LIGHT = ':root:not([data-theme="playful"])'
DARK = ':root:not([data-theme="playful"]).dark'

# Every Clean color variable needs a usage note here, or the export fails.
COLOR_USAGE = {
    "bg": "Page background behind every screen.",
    "side": "Sidebar background.",
    "surface": "Cards, lists, inputs, dialogs: anything framed on top of bg.",
    "sunken": "Recessed areas inside a surface: list headers, group headers, notes, tags, icon tiles.",
    "ink": "Primary text and headings on bg, surface or sunken.",
    "ink-2": "Secondary text: descriptions, inactive tabs, button labels.",
    "ink-3": "Tertiary text: captions, counts, paths, placeholders. Lightest text allowed.",
    "line": "Frames and dividers between sections and rows.",
    "line-2": "Control borders: buttons, inputs, switches when off, dashed add buttons.",
    "frame": "1px border around a card or list box (--frame in code).",
    "sep": "1px separator between rows inside a list (--sep in code).",
    "pri": "Primary button fill and switch-on track. Clean's primary is ink, never a hue.",
    "on-pri": "Text and icons on pri.",
    "accent": "Links, focus rings, 'pending' markers. The only hue Clean uses for interaction.",
    "accent-bg": "Selected row background; tint behind accent text.",
    "sel": "Selected nav item and menu hover; text on it stays ink.",
    "sel-ink": "Text on sel.",
    "ok": "Status dot and text: synced, loaded, on.",
    "ok-bg": "Tint behind ok text.",
    "warn": "Status: needs attention, not installed, drift.",
    "warn-bg": "Tint behind warn text; warning notes.",
    "bad": "Errors, destructive actions, removed diff lines.",
    "bad-bg": "Tint behind bad text; error notes.",
    "c-skill": "Kind tile icon for skills (.ss-cat.skill). Kind colors only appear on these small tiles.",
    "c-skill-bg": "Kind tile background for skills.",
    "c-agent": "Kind tile icon for agents.",
    "c-agent-bg": "Kind tile background for agents.",
    "c-extra": "Kind tile icon for extras.",
    "c-extra-bg": "Kind tile background for extras.",
    "c-mcp": "Kind tile icon for MCP servers.",
    "c-mcp-bg": "Kind tile background for MCP servers.",
    "c-target": "Kind tile icon for targets.",
    "c-target-bg": "Kind tile background for targets.",
    "c-plugin": "Kind tile icon for plugins and Pi packages.",
    "c-plugin-bg": "Kind tile background for plugins.",
}
# Color-valued variables deliberately left out of the design system.
COLOR_SKIP = {"line-soft", "dots"}
# Whole border shorthands ("1px solid #ECE9E3"); the design system records their color.
BORDER_SHORTHANDS = {"frame", "sep"}

RADIUS_USAGE = {
    "r-tag": "Tags and small chips.",
    "r-ctl": "Inputs, icon buttons, icon tiles.",
    "r-box": "Cards, lists, dialogs.",
    "r-btn": "Buttons are pills.",
}
SHADOW_USAGE = {
    "sh-box": "Cards.",
    "sh-float": "Menus, toasts, sticky action bars.",
    "sh-dialog": "Dialogs.",
}


def block(css, selector):
    match = re.search(r"^" + re.escape(selector) + r" \{", css, re.M)
    if not match:
        raise ValueError(f"{CSS_PATH}: no rule for {selector}")
    return css[match.start():css.index("}", match.start())]


def declarations(text, prefix=""):
    return {m.group(1): m.group(2).strip() for m in re.finditer(prefix + r"([a-z0-9-]+):\s*([^;]+);", text)}


def css_vars(css, selector):
    return declarations(block(css, selector), "--")


def is_color(value):
    return value.startswith("#") or value.startswith("rgb")


def font(shorthand):
    weight, size, line = re.match(r"(\d+) ([\d.]+px)/([\d.]+)", shorthand).groups()
    return {"fontSize": size, "lineHeight": float(line), "fontWeight": int(weight)}


def color_value(name, light, dark):
    if name in BORDER_SHORTHANDS:
        return {"light": light[name].split()[-1].lower(), "dark": dark[name].split()[-1].lower()}
    return {"light": light[name].lower(), "dark": dark.get(name, light[name]).lower()}


def colors(light, dark):
    unnoted = sorted(n for n, v in light.items() if is_color(v) and n not in COLOR_USAGE and n not in COLOR_SKIP)
    if unnoted:
        raise ValueError(f"{CSS_PATH}: add a usage note in scripts/design/tokens.py for --{', --'.join(unnoted)}")
    return [{"name": n, "value": color_value(n, light, dark), "usage": u} for n, u in COLOR_USAGE.items()]


def type_scale(css, light):
    body = declarations(block(css, "body"))
    btn = declarations(block(css, ".ss-btn"))
    note = declarations(block(css, ".ss-note"))
    cnt = declarations(block(css, ".ss-cnt"))
    name = declarations(block(css, ".ss-r .nm.m"))
    tag = declarations(block(css, ".ss-tag"))
    return {
        "fonts": [],
        "families": {"sans": light["f"], "mono": light["fm"]},
        "groups": [
            {"name": "Headings", "family": "sans", "styles": [
                {"name": "h1", **font(light["h1"]), "letterSpacing": light["h1-track"]},
                {"name": "h2", **font(light["h2"]), "letterSpacing": light["h2-track"]},
            ]},
            {"name": "Text", "family": "sans", "styles": [
                {"name": "body", **font(body["font"])},
                {"name": "small", "fontSize": note["font-size"], "lineHeight": float(note["line-height"]), "fontWeight": 400},
                {"name": "button", **font(btn["font"])},
                {"name": "caption", **font(cnt["font"])},
            ]},
            {"name": "Mono", "family": "mono", "styles": [
                {"name": "mono-name", "fontSize": name["font-size"], "lineHeight": 1.4, "fontWeight": int(name["font-weight"]), "letterSpacing": name["letter-spacing"]},
                {"name": "mono-tag", **font(tag["font"])},
            ]},
        ],
    }


def spacing(css):
    wrap = declarations(block(css, ".ss-wrap"))
    row = declarations(block(css, ".ss-r"))
    btn = declarations(block(css, ".ss-btn"))
    return [
        {"name": "page-max", "value": wrap["max-width"], "usage": "Max width of the page column (.ss-wrap)."},
        {"name": "page-gap", "value": wrap["gap"], "usage": "Gap between page sections; never add margins on top of it."},
        {"name": "row-height", "value": row["min-height"], "usage": "Minimum height of a list row (.ss-r)."},
        {"name": "row-pad", "value": row["padding"].split()[-1], "usage": "Horizontal padding inside list rows and headers."},
        {"name": "control-height", "value": btn["height"], "usage": "Buttons and inputs at default size."},
    ]


def shadows(light, dark):
    out = []
    for n, usage in SHADOW_USAGE.items():
        if dark.get(n, light[n]) != light[n]:
            usage += f" Dark: {dark[n]}."
        out.append({"name": n, "value": light[n], "usage": usage})
    return out


def export(css, ref=None, today=None):
    light, dark = css_vars(css, LIGHT), css_vars(css, DARK)
    return {
        "name": "Skillshare Dashboard",
        "version": 1,
        "meta": {
            "source": "github",
            "repo": "runkids/skillshare",
            "ref": ref,
            "package": "ui",
            "paths": {"tokens": [CSS_PATH]},
            "synced": today or datetime.date.today().isoformat(),
            "scope": "Clean style only (light + dark). Playful is not included.",
        },
        "color": {"themes": [{"id": "light", "name": "Clean Light"}, {"id": "dark", "name": "Clean Dark"}], "tokens": colors(light, dark)},
        "type": type_scale(css, light),
        "spacing": {"tokens": spacing(css)},
        "radius": {"tokens": [{"name": n, "value": light[n], "usage": u} for n, u in RADIUS_USAGE.items()]},
        "shadow": {"tokens": shadows(light, dark)},
    }


def git_ref():
    try:
        sha = subprocess.run(["git", "rev-parse", "--short", "HEAD"], cwd=ROOT, capture_output=True, text=True, check=True).stdout.strip()
        branch = subprocess.run(["git", "branch", "--show-current"], cwd=ROOT, capture_output=True, text=True, check=True).stdout.strip()
    except (OSError, subprocess.CalledProcessError):
        return None
    return f"{branch or 'HEAD'}@{sha}"


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--out", help="write here instead of stdout")
    args = parser.parse_args(argv)
    text = json.dumps(export((ROOT / CSS_PATH).read_text(), ref=git_ref()), indent=2, ensure_ascii=False) + "\n"
    if args.out:
        Path(args.out).write_text(text)
    else:
        sys.stdout.write(text)


if __name__ == "__main__":
    main()

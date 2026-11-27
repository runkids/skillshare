#!/usr/bin/env python3
"""Check that a CSS change keeps the cascade of an unminified build.

A built stylesheet is flattened into rules (at-rule chain, selector,
declarations), grouped by cascade layer. Blocks of one layer are merged,
because splitting a source file can split its @layer block in the output.

    python3 scripts/design/css_rules.py --same OLD NEW
    python3 scripts/design/css_rules.py --flips OLD NEW

--same passes only when the layer order and every layer's rule list are equal.
Use it after cutting a file apart without reordering.

--flips is for reordering. It lists rule pairs of one layer whose order
changed while both have the same specificity and set the same property to
different values (shorthands count as their longhands). It cannot tell whether
the two selectors can match one element: judge every pair by hand.

Both exit 0 when clean, 1 on a difference and 2 when a file cannot be checked.
Native CSS nesting is not supported; an unminified Vite build has none.
"""

import argparse
import difflib
from pathlib import Path
import re
import sys
from typing import NamedTuple


class Rule(NamedTuple):
    context: tuple  # enclosing at-rules, outermost first, without @layer
    selector: str  # empty for the declarations of an at-rule such as @property
    declarations: tuple  # (property, value) in source order


class Sheet(NamedTuple):
    layer_order: tuple
    layers: dict  # layer name ('' = unlayered) -> list of Rule


TOKEN = re.compile(
    r"""(?P<comment>/\*.*?\*/)
      | (?P<string>"(?:\\.|[^"\\\n])*"|'(?:\\.|[^'\\\n])*')
      | (?P<escape>\\.)
      | (?P<space>\s+)
      | (?P<punct>[{};()\[\]:,])
      | (?P<text>[^\s{};()\[\]:,"'\\/]+|.)""",
    re.S | re.X,
)


def squash(tokens):
    """Join tokens, collapsing whitespace outside strings."""
    return "".join(" " if kind == "space" else text for kind, text in tokens).strip()


def split_top(tokens, separator):
    parts, depth = [[]], 0
    for kind, text in tokens:
        if kind == "punct" and text in "([{":
            depth += 1
        elif kind == "punct" and text in ")]}":
            depth -= 1
        if depth == 0 and kind == "punct" and text == separator:
            parts.append([])
        else:
            parts[-1].append((kind, text))
    return parts


def parse(css):
    layer_order, layers = [], {}
    # One frame per open block: [kind, prelude, layer, context, declarations]
    stack = [["root", "", "", (), []]]
    buffer, depth, anonymous = [], 0, 0

    def declare(layer):
        if layer and layer not in layer_order:
            parent = layer.rpartition(".")[0]
            if parent:
                declare(parent)
            layer_order.append(layer)

    def emit(layer, context, selector, declarations):
        layers.setdefault(layer, []).append(Rule(context, selector, tuple(declarations)))

    def end_statement():
        kind, _, layer, context, declarations = stack[-1]
        text = squash(buffer)
        if not text:
            return
        if text.startswith("@layer "):
            for name in text[len("@layer ") :].split(","):
                declare(f"{layer}.{name.strip()}" if layer else name.strip())
        elif text.startswith("@"):
            emit(layer, context, text, ())
        elif kind == "root":
            raise ValueError(f"declaration outside a rule: {text[:80]}")
        else:
            parts = split_top(buffer, ":")
            if len(parts) < 2:
                raise ValueError(f"not a declaration: {text[:80]}")
            name = squash(parts[0])
            value = squash(buffer[len(parts[0]) + 1 :])
            declarations.append((name, value))

    for match in TOKEN.finditer(css):
        kind, text = match.lastgroup, match.group()
        if kind == "comment":
            continue
        if kind == "text" and text in "\"'":
            raise ValueError("unterminated string")
        if kind == "punct" and text in "([":
            depth += 1
        elif kind == "punct" and text in ")]":
            depth -= 1
        # A custom property may hold a {} block; keep it as part of the value.
        if kind == "punct" and text == "{" and (depth or squash(buffer).startswith("--")):
            depth += 1
        elif kind == "punct" and text == "}" and depth:
            depth -= 1
        elif kind == "punct" and text == "{":
            prelude = squash(buffer)
            parent_kind, _, layer, context, _ = stack[-1]
            if parent_kind == "style":
                raise ValueError(f"nested rule is not supported: {prelude[:80]}")
            if re.match(r"@layer\b", prelude):
                name = prelude[len("@layer") :].strip()
                if not name:
                    anonymous += 1
                    name = f"(anonymous {anonymous})"
                layer = f"{layer}.{name}" if layer else name
                declare(layer)
                stack.append(["group", prelude, layer, context, []])
            elif prelude.startswith("@"):
                stack.append(["at", prelude, layer, context + (prelude,), []])
            else:
                stack.append(["style", prelude, layer, context, []])
            buffer = []
            continue
        elif kind == "punct" and text == "}":
            if len(stack) == 1:
                raise ValueError("unmatched }")
            end_statement()
            buffer = []
            frame = stack.pop()
            block, prelude, layer, context, declarations = frame
            # Empty rules have no effect, so they are dropped.
            if declarations:
                emit(layer, context, prelude if block == "style" else "", declarations)
            continue
        elif kind == "punct" and text == ";" and not depth:
            end_statement()
            buffer = []
            continue
        if buffer or kind != "space":
            buffer.append((kind, text))

    if len(stack) != 1 or squash(buffer):
        raise ValueError("unexpected end of file")
    if not layers:
        raise ValueError("no rules found")
    return Sheet(tuple(layer_order), layers)


# ---------------------------------------------------------------- --same


def render(sheet):
    lines = ["layer order: " + ", ".join(sheet.layer_order)]
    for layer in sorted(sheet.layers):
        for rule in sheet.layers[layer]:
            body = " ".join(f"{name}: {value};" for name, value in rule.declarations)
            head = " ".join((*rule.context, rule.selector)).strip()
            lines.append(f"[{layer or 'unlayered'}] {head} {{ {body} }}")
    return lines


def same(old, new):
    """Return the unified diff between two sheets, empty when they are equal."""
    return list(difflib.unified_diff(render(old), render(new), "old", "new", n=2, lineterm=""))


# ---------------------------------------------------------------- --flips

SIDES = ("top", "right", "bottom", "left")
BORDER_PARTS = ("width", "style", "color")
ALIASES = {
    "inset": SIDES,
    "gap": ("row-gap", "column-gap"),
    "font": ("line-height",),
    "place-items": ("align-items", "justify-items"),
    "place-content": ("align-content", "justify-content"),
    "place-self": ("align-self", "justify-self"),
    "grid-area": ("grid-row", "grid-column"),
    "flex-flow": ("flex-direction", "flex-wrap"),
}
# Names that start like a shorthand but are not reset by it.
NOT_LONGHANDS = {
    "color": ("color-scheme",),
    "content": ("content-visibility",),
    "fill": ("fill-opacity", "fill-rule"),
    "flex": ("flex-direction", "flex-wrap", "flex-flow"),
    "outline": ("outline-offset",),
    "overflow": ("overflow-wrap", "overflow-anchor"),
    "position": ("position-anchor", "position-area", "position-try"),
    "stroke": ("stroke-width", "stroke-linecap", "stroke-linejoin", "stroke-dasharray", "stroke-dashoffset"),
    "transform": ("transform-origin", "transform-style", "transform-box"),
}


def border_atoms(name):
    """Expand a border property to side-part atoms, or None for other names."""
    match = re.fullmatch(
        r"border(?:-(top|right|bottom|left|block|inline)(?:-(start|end))?)?(?:-(width|style|color))?", name
    )
    if not match:
        return None
    side, end, part = match.groups()
    if side == "block":
        sides = {"start": ("top",), "end": ("bottom",), None: ("top", "bottom")}[end]
    elif side == "inline":
        sides = {"start": ("left",), "end": ("right",), None: ("left", "right")}[end]
    else:
        sides = (side,) if side else SIDES
    return {f"border-{s}-{p}" for s in sides for p in ((part,) if part else BORDER_PARTS)}


def overlaps(a, b):
    """Whether declaring property a and property b can set the same longhand."""
    if a == b:
        return True
    if a.startswith("--") or b.startswith("--"):
        return False
    if "all" in (a, b):
        return True

    def expand(name):
        # Logical sides are treated as their left-to-right physical sides.
        name = name.replace("-inline-start", "-left").replace("-inline-end", "-right")
        name = name.replace("-block-start", "-top").replace("-block-end", "-bottom")
        name = name.replace("inline-size", "width").replace("block-size", "height")
        name = re.sub(r"^inset-(?=top|right|bottom|left)", "", name)
        for axis, sides in (("-inline", ("-left", "-right")), ("-block", ("-top", "-bottom"))):
            if name.endswith(axis) and not name.startswith("border"):
                base = name[: -len(axis)]
                return [side[1:] if base == "inset" else base + side for side in sides]
        return [name]

    def covers(short, long):
        if short == long or long in ALIASES.get(short, ()):
            return True
        if any(covers(alias, long) for alias in ALIASES.get(short, ())):
            return True
        if short == "border-radius":
            return long.startswith("border-") and long.endswith("-radius")
        if short == "border":
            return long.startswith("border-image")
        return long.startswith(short + "-") and not long.startswith(NOT_LONGHANDS.get(short, ()))

    for x in expand(a):
        for y in expand(b):
            xb, yb = border_atoms(x), border_atoms(y)
            if xb and yb:
                if xb & yb:
                    return True
            elif covers(x, y) or covers(y, x):
                return True
    return False


def add(*specificities):
    return tuple(map(sum, zip((0, 0, 0), *specificities)))


def closing(text, start):
    """Index of the bracket that closes the one at start."""
    depth, quote, i = 0, "", start
    while i < len(text):
        char = text[i]
        if char == "\\":
            i += 1
        elif quote:
            quote = "" if char == quote else quote
        elif char in "\"'":
            quote = char
        elif char in "([":
            depth += 1
        elif char in ")]":
            depth -= 1
            if not depth:
                return i
        i += 1
    raise ValueError(f"unbalanced selector: {text}")


def split_list(selector):
    """Split a selector list on its top-level commas."""
    tokens = [(m.lastgroup, m.group()) for m in TOKEN.finditer(selector)]
    return [squash(part) for part in split_top(tokens, ",")]


IDENT = re.compile(r"(?:\\[0-9a-fA-F]{1,6}\s?|\\.|[-\w]|[^\x00-\x7f])+")
LEGACY_PSEUDO_ELEMENTS = {"before", "after", "first-line", "first-letter"}


def specificity(selector):
    total, i = (0, 0, 0), 0
    while i < len(selector):
        char = selector[i]
        if char == "[":
            total = add(total, (0, 1, 0))
            i = closing(selector, i) + 1
            continue
        if char in "#.":
            name = IDENT.match(selector, i + 1)
            total = add(total, (1, 0, 0) if char == "#" else (0, 1, 0))
            i = name.end() if name else i + 1
            continue
        if char == ":":
            element = selector.startswith("::", i)
            name = IDENT.match(selector, i + (2 if element else 1))
            if not name:
                raise ValueError(f"cannot read selector: {selector}")
            pseudo, i = name.group().lower(), name.end()
            argument = None
            if i < len(selector) and selector[i] == "(":
                end = closing(selector, i)
                argument, i = selector[i + 1 : end], end + 1
            if element or pseudo in LEGACY_PSEUDO_ELEMENTS:
                total = add(total, (0, 0, 1))
            elif pseudo in ("is", "not", "has"):
                total = add(total, max(specificity(part) for part in split_list(argument)))
            elif pseudo != "where":
                total = add(total, (0, 1, 0))
                of = re.search(r"\sof\s(.*)", argument or "")
                if of and pseudo.startswith("nth-"):
                    total = add(total, max(specificity(part) for part in split_list(of.group(1))))
            continue
        name = IDENT.match(selector, i)
        if name:
            total = add(total, (0, 0, 1))
            i = name.end()
        else:
            i += 1  # combinators, *, namespace bars
    return total


def selector_entries(rules):
    """One (identity, rule) per selector of every style rule, in cascade order."""
    seen, entries = {}, []
    for rule in rules:
        # Keyframe steps and at-rule bodies do not compete by specificity.
        if not rule.selector or rule.selector.startswith("@"):
            continue
        if any("keyframes" in at.split(" ")[0] for at in rule.context):
            continue
        for selector in split_list(rule.selector):
            key = (rule.context, selector, rule.declarations)
            seen[key] = seen.get(key, 0) + 1
            entries.append(((*key, seen[key]), Rule(rule.context, selector, rule.declarations)))
    return entries


def conflicts(first, second):
    """Property names for which the order of two rules can matter."""
    # The last declaration of a property wins inside a rule; !important competes only with !important.
    def winners(rule):
        return {(name, value.endswith("!important")): value for name, value in rule.declarations}

    found = []
    for (a, a_important), a_value in winners(first).items():
        for (b, b_important), b_value in winners(second).items():
            if a_important == b_important and overlaps(a, b) and not (a == b and a_value == b_value):
                found.append(a if a == b else f"{a} / {b}")
    return found


def flips(old, new):
    """Return (flipped pairs, rules without a counterpart)."""
    pairs, unmatched = [], []
    for layer in sorted(set(old.layers) | set(new.layers)):
        before = selector_entries(old.layers.get(layer, []))
        after = selector_entries(new.layers.get(layer, []))
        position = {key: index for index, (key, _) in enumerate(after)}
        old_keys = {key for key, _ in before}
        unmatched += [("old", layer, rule) for key, rule in before if key not in position]
        unmatched += [("new", layer, rule) for key, rule in after if key not in old_keys]
        by_specificity = {}
        for key, rule in before:
            if key in position:
                by_specificity.setdefault(specificity(rule.selector), []).append((position[key], rule))
        for level, entries in by_specificity.items():
            for i, (first_position, first) in enumerate(entries):
                for second_position, second in entries[i + 1 :]:
                    if second_position < first_position:
                        properties = conflicts(first, second)
                        if properties:
                            pairs.append((layer, level, properties, first, second))
    return pairs, unmatched


def describe(rule):
    return " ".join((*rule.context, rule.selector))


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    mode = parser.add_mutually_exclusive_group(required=True)
    mode.add_argument("--same", nargs=2, metavar=("OLD", "NEW"))
    mode.add_argument("--flips", nargs=2, metavar=("OLD", "NEW"))
    args = parser.parse_args(argv)

    try:
        old, new = (parse(Path(path).read_text()) for path in args.same or args.flips)
        if args.same:
            diff = same(old, new)
            if diff:
                print("\n".join(diff))
                return 1
            print(f"same: {sum(map(len, old.layers.values()))} rules")
            return 0
        pairs, unmatched = flips(old, new)
    except (OSError, ValueError) as error:
        print(f"error: {error}", file=sys.stderr)
        return 2

    if old.layer_order != new.layer_order:
        print(f"layer order changed: {', '.join(old.layer_order)} -> {', '.join(new.layer_order)}")
    for layer, level, properties, first, second in pairs:
        print(f"[{layer or 'unlayered'}] specificity {level}: {', '.join(properties)}")
        print(f"  was first: {describe(first)}")
        print(f"  now first: {describe(second)}")
    for side, layer, rule in unmatched:
        print(f"only in {side}, order not checked: [{layer or 'unlayered'}] {describe(rule)}")
    print(f"{len(pairs)} flipped pairs, {len(unmatched)} unmatched rules")
    return 1 if pairs or unmatched or old.layer_order != new.layer_order else 0


if __name__ == "__main__":
    sys.exit(main())

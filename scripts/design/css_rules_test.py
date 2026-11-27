import unittest

from css_rules import flips, parse, same, specificity

BASE = """
@layer theme, components;
:root { --bg: #fff; }
@layer components {
  .btn:hover { color: red; border: 1px solid; }
  @media (hover: hover) {
    .card:hover { border-top-color: blue; }
  }
  .note { content: "a  ;} b"; }
}
"""


def swapped(css, first, second):
    return css.replace(first, "\0").replace(second, first).replace("\0", second)


def flipped(old, new):
    pairs, _ = flips(parse(old), parse(new))
    return [(first.selector, second.selector) for _, _, _, first, second in pairs]


class SameTest(unittest.TestCase):
    def test_split_layer_blocks_and_formatting_are_equal(self):
        split = BASE.replace("  .note", "}\n/* cut */\n@layer components {\n  .note").replace("red;", "red ;\n\n")
        self.assertEqual(same(parse(BASE), parse(split)), [])

    def test_reordered_rules_differ(self):
        new = swapped(BASE, ".btn:hover { color: red; border: 1px solid; }", '.note { content: "a  ;} b"; }')
        self.assertIn(".note", "\n".join(same(parse(BASE), parse(new))))

    def test_whitespace_inside_a_string_is_significant(self):
        self.assertTrue(same(parse(BASE), parse(BASE.replace("a  ;} b", "a ;} b"))))

    def test_moving_a_rule_out_of_its_media_query_differs(self):
        new = BASE.replace("@media (hover: hover)", "@media (hover: none)")
        self.assertTrue(same(parse(BASE), parse(new)))

    def test_layer_order_change_differs(self):
        new = BASE.replace("theme, components", "components, theme")
        self.assertTrue(same(parse(BASE), parse(new)))

    def test_input_that_cannot_be_checked_fails(self):
        for css in ("", ".a { color: red;", ".a { .b { color: red; } }"):
            with self.subTest(css=css), self.assertRaises(ValueError):
                parse(css)


class FlipsTest(unittest.TestCase):
    def test_shorthand_pair_flips_across_a_media_query(self):
        rule = "  .btn:hover { color: red; border: 1px solid; }\n"
        new = BASE.replace(rule, "").replace("  .note", rule + "  .note")
        self.assertEqual(flipped(BASE, new), [(".btn:hover", ".card:hover")])

    def test_order_does_not_matter_without_a_real_conflict(self):
        cases = {
            "other specificity": (".a { color: red; }", ".a.b { color: blue; }"),
            "other property": (".a { color: red; }", ".b { margin: 0; }"),
            "same value": (".a { color: red; }", ".b { color: red; }"),
            "not a longhand": (".a { outline: none; }", ".b { outline-offset: 2px; }"),
            "other side": (".a { border-top: 0; }", ".b { border-bottom-color: red; }"),
            "importance decides": (".a { color: red !important; }", ".b { color: blue; }"),
            "other layer": ("@layer x { .a { color: red; } }", "@layer y { .b { color: blue; } }"),
        }
        for name, (first, second) in cases.items():
            with self.subTest(name):
                old = f"@layer x, y; {first} {second}"
                self.assertEqual(flipped(old, f"@layer x, y; {second} {first}"), [])

    def test_overlapping_shorthands_flip(self):
        for a, b in (
            ("padding", "padding-inline"),
            ("padding-inline", "padding-left"),
            ("inset", "top"),
            ("inset-inline", "left"),
            ("inset-block-end", "bottom"),
            ("inline-size", "width"),
            ("border-color", "border-left"),
            ("border-radius", "border-top-left-radius"),
            ("background", "background-color"),
        ):
            with self.subTest(a=a, b=b):
                first, second = f".a {{ {a}: 0; }}", f".b {{ {b}: 1px; }}"
                self.assertEqual(flipped(first + second, second + first), [(".a", ".b")])

    def test_each_selector_of_a_list_is_compared_by_itself(self):
        first, second = ".a, #id { color: red; }", ".b { color: blue; }"
        self.assertEqual(flipped(first + second, second + first), [(".a", ".b")])

    def test_changed_rule_is_reported_as_unchecked(self):
        _, unmatched = flips(parse(".a { color: red; }"), parse(".a { color: blue; }"))
        self.assertEqual([side for side, _, _ in unmatched], ["old", "new"])


class SpecificityTest(unittest.TestCase):
    def test_selectors(self):
        for selector, expected in {
            ":root:not([data-theme='playful']).dark": (0, 3, 0),
            ".ss-btn:hover:not(:disabled)": (0, 3, 0),
            ".a:where(.b, #c) > li::before": (0, 1, 2),
            ".a :is(button, #c):after": (1, 1, 1),
            ".hover\\:bg-\\[var\\(--x\\)\\]:hover": (0, 2, 0),
            "li:nth-child(2n + 1 of .a.b)": (0, 3, 1),
            "*": (0, 0, 0),
        }.items():
            with self.subTest(selector):
                self.assertEqual(specificity(selector), expected)


if __name__ == "__main__":
    unittest.main()

import unittest

from tokens import CSS_PATH, LIGHT, ROOT, export

CSS = (ROOT / CSS_PATH).read_text()


def tokens(data):
    return {t["name"]: t for family in ("color", "spacing", "radius", "shadow") for t in data[family]["tokens"]}


class TokensTest(unittest.TestCase):
    def test_exports_clean_light_and_dark_colors_from_css(self):
        ink = tokens(export(CSS))["ink"]["value"]
        self.assertEqual(ink, {"light": "#141312", "dark": "#edebe6"})

    def test_border_shorthand_is_exported_as_its_color(self):
        self.assertEqual(tokens(export(CSS))["frame"]["value"], {"light": "#e2dfd7", "dark": "#2c2a26"})

    def test_new_color_variable_without_usage_note_fails(self):
        css = CSS.replace(LIGHT + " {", LIGHT + " {\n  --brand-new: #123456;", 1)
        with self.assertRaisesRegex(ValueError, "--brand-new"):
            export(css)

    def test_token_names_are_unique_across_families(self):
        data = export(CSS)
        names = [t["name"] for family in ("color", "spacing", "radius", "shadow") for t in data[family]["tokens"]]
        self.assertEqual(len(names), len(set(names)))


if __name__ == "__main__":
    unittest.main()

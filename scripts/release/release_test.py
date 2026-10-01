import hashlib
import io
import json
from pathlib import Path
import subprocess
import tarfile
import tempfile
import unittest

from release import check, first_entry, prepare_assets, required_assets, sync, verify_assets


class ReleaseTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="skillshare-release-test-")
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.write(".github/release-please-manifest.json", json.dumps({".": "0.24.0"}))
        self.history = "## [0.23.1] - 2026-10-01\n\n- Previous release.\n"
        self.entry = "## [0.24.0] - 2026-10-02\n\n### New Features\n\n- **Example** — `skillshare check`.\n\n"
        self.prefix = "---\ntitle: Changelog\n---\n\n# Changelog\n\nKeep this intro.\n\n---\n\n"
        self.write("CHANGELOG.md", "# Changelog\n\n" + self.entry + self.history)
        self.write("website/src/pages/changelog.md", self.prefix + self.history)
        self.write("skills/skillshare/SKILL.md", "---\nname: skillshare\nmetadata:\n  version: v0.23.1\n---\n\nKeep this skill.\n")

    def write(self, path, text):
        target = self.root / path
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_text(text)

    def test_sync_normalizes_release_please_heading_and_preserves_history(self):
        generated = self.entry.replace("[0.24.0] - 2026-10-02", "[0.24.0](https://github.com/runkids/skillshare/compare/v0.23.1...v0.24.0) (2026-10-02)")
        self.write("CHANGELOG.md", "# Changelog\n\n" + generated + self.history)
        sync(self.root)
        self.assertEqual(check(self.root, "v0.24.0"), "0.24.0")
        self.assertEqual((self.root / "website/src/pages/changelog.md").read_text(), self.prefix + self.entry + self.history)
        self.assertEqual((self.root / "CHANGELOG.md").read_text(), "# Changelog\n\n" + self.entry + self.history)
        self.assertIn("Keep this skill.", (self.root / "skills/skillshare/SKILL.md").read_text())

    def test_sync_replaces_existing_entry_and_is_idempotent(self):
        sync(self.root)
        self.write("CHANGELOG.md", "# Changelog\n\n" + self.entry.replace("Example", "Reviewed example") + self.history)
        sync(self.root)
        first = (self.root / "website/src/pages/changelog.md").read_text()
        sync(self.root)
        self.assertEqual(first, (self.root / "website/src/pages/changelog.md").read_text())
        self.assertEqual(first.count("## [0.24.0]"), 1)
        self.assertIn("Reviewed example", first)

    def test_sync_normalizes_patch_release_heading(self):
        self.write(".github/release-please-manifest.json", json.dumps({".": "0.23.2"}))
        generated = self.entry.replace("## [0.24.0] - 2026-10-02", "### [0.23.2](https://example.test/compare) (2026-10-02)")
        self.write("CHANGELOG.md", "# Changelog\n\n" + generated + self.history)
        sync(self.root)
        self.assertEqual(check(self.root, "v0.23.2"), "0.23.2")

    def test_check_rejects_different_tag_skill_and_website(self):
        sync(self.root)
        with self.assertRaisesRegex(ValueError, "Tag differs"):
            check(self.root, "v0.24.1")
        self.write("skills/skillshare/SKILL.md", "metadata:\n  version: v0.23.1\n")
        with self.assertRaisesRegex(ValueError, "Built-in skill"):
            check(self.root)
        sync(self.root)
        self.write("website/src/pages/changelog.md", self.prefix + self.entry.replace("Example", "Different") + self.history)
        with self.assertRaisesRegex(ValueError, "changelog entries"):
            check(self.root)

    def test_sync_rejects_newer_website_without_writing_other_files(self):
        self.write("website/src/pages/changelog.md", self.prefix + self.entry.replace("0.24.0", "0.25.0") + self.history)
        before = (self.root / "skills/skillshare/SKILL.md").read_text()
        with self.assertRaisesRegex(ValueError, "newer website"):
            sync(self.root)
        self.assertEqual(before, (self.root / "skills/skillshare/SKILL.md").read_text())

    def test_check_rejects_a_different_release_commit(self):
        sync(self.root)
        subprocess.run(["git", "init", "-q"], cwd=self.root, check=True)
        subprocess.run(["git", "add", "."], cwd=self.root, check=True)
        subprocess.run(["git", "-c", "user.name=Release Test", "-c", "user.email=release@example.test", "commit", "-qm", "fixture"], cwd=self.root, check=True)
        sha = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=self.root, text=True).strip()
        self.assertEqual(check(self.root, sha=sha), "0.24.0")
        with self.assertRaisesRegex(ValueError, "approved release commit"):
            check(self.root, sha="0" * 40)

    def test_notes_include_only_current_release(self):
        _, _, entry = first_entry((self.root / "CHANGELOG.md").read_text())
        self.assertIn("Example", entry)
        self.assertNotIn("Previous release", entry)

    def assets(self, binary_version="0.24.0"):
        sync(self.root)
        directory = self.root / "dist"
        directory.mkdir()
        for name in required_assets("0.24.0") - {"skillshare.rb", "skillshare-ui-dist.tar.gz"}:
            (directory / name).write_bytes(b"release asset")
        (self.root / "skillshare-ui-dist.tar.gz").write_bytes(b"release asset")
        binary = f"#!/bin/sh\necho 'skillshare v{binary_version}'\n".encode()
        for arch in ("amd64", "arm64"):
            with tarfile.open(directory / f"skillshare_0.24.0_linux_{arch}.tar.gz", "w:gz") as archive:
                member = tarfile.TarInfo("skillshare")
                member.size = len(binary)
                archive.addfile(member, io.BytesIO(binary))
        self.write("dist/homebrew/Formula/skillshare.rb", 'class Skillshare < Formula\n  version "0.24.0"\n  url "https://github.com/runkids/skillshare/releases/download/v0.24.0/skillshare_0.24.0_darwin_arm64.tar.gz"\nend\n')
        lines = [f"{hashlib.sha256(path.read_bytes()).hexdigest()}  {path.name}" for path in directory.iterdir() if path.is_file()]
        lines.append(f"{hashlib.sha256((self.root / 'skillshare-ui-dist.tar.gz').read_bytes()).hexdigest()}  skillshare-ui-dist.tar.gz")
        (directory / "checksums.txt").write_text("\n".join(lines) + "\n")
        prepare_assets(directory)
        return directory

    def test_complete_assets_include_formula_and_match_binary_version(self):
        directory = self.assets()
        prepare_assets(directory)
        verify_assets(self.root, directory, "v0.24.0")
        self.assertEqual((directory / "checksums.txt").read_text().count("  skillshare.rb"), 1)

    def test_assets_reject_changed_bytes(self):
        directory = self.assets()
        (directory / "skillshare-ui-dist.tar.gz").write_bytes(b"tampered")
        with self.assertRaisesRegex(ValueError, "Checksum mismatch"):
            verify_assets(self.root, directory, "v0.24.0")

    def test_assets_reject_missing_platform_checksum(self):
        directory = self.assets()
        checksum_file = directory / "checksums.txt"
        checksum_file.write_text("\n".join(line for line in checksum_file.read_text().splitlines() if "windows_arm64" not in line) + "\n")
        with self.assertRaisesRegex(ValueError, "all six archives"):
            verify_assets(self.root, directory, "v0.24.0")

    def test_assets_reject_wrong_packaged_binary_version(self):
        directory = self.assets(binary_version="dev")
        with self.assertRaisesRegex(ValueError, "Packaged CLI version"):
            verify_assets(self.root, directory, "v0.24.0")

    def test_assets_reject_formula_pointing_at_another_release(self):
        directory = self.assets()
        formula = directory / "homebrew/Formula/skillshare.rb"
        formula.write_text(formula.read_text().replace("download/v0.24.0/", "download/v0.23.1/"))
        prepare_assets(directory)
        with self.assertRaisesRegex(ValueError, "this release's verified archives"):
            verify_assets(self.root, directory, "v0.24.0")


if __name__ == "__main__":
    unittest.main()

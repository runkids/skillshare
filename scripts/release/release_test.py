import base64
import hashlib
import io
import json
from pathlib import Path
import subprocess
import tarfile
import tempfile
import unittest
import warnings
import zipfile

from release import archive_binary, build_wheels, check, first_entry, prepare_assets, required_assets, sync, verify_assets


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
        (self.root / "skillshare-ui-dist.tar.gz").write_bytes(b"release asset")
        binary = f"#!/bin/sh\necho 'skillshare v{binary_version}'\n".encode()
        # Like GoReleaser's archives, each one also carries files besides the binary.
        for os_name in ("darwin", "linux"):
            for arch in ("amd64", "arm64"):
                with tarfile.open(directory / f"skillshare_0.24.0_{os_name}_{arch}.tar.gz", "w:gz") as archive:
                    for name, data in (("README.md", b"# skillshare\n"), ("skillshare", binary)):
                        member = tarfile.TarInfo(name)
                        member.size = len(data)
                        archive.addfile(member, io.BytesIO(data))
        for arch in ("amd64", "arm64"):
            with zipfile.ZipFile(directory / f"skillshare_0.24.0_windows_{arch}.zip", "w") as archive:
                archive.writestr("README.md", b"# skillshare\n")
                archive.writestr("skillshare.exe", binary)
        self.write("dist/homebrew/Formula/skillshare.rb", 'class Skillshare < Formula\n  version "0.24.0"\n  url "https://github.com/runkids/skillshare/releases/download/v0.24.0/skillshare_0.24.0_darwin_arm64.tar.gz"\nend\n')
        lines = [f"{hashlib.sha256(path.read_bytes()).hexdigest()}  {path.name}" for path in directory.iterdir() if path.is_file()]
        lines.append(f"{hashlib.sha256((self.root / 'skillshare-ui-dist.tar.gz').read_bytes()).hexdigest()}  skillshare-ui-dist.tar.gz")
        (directory / "checksums.txt").write_text("\n".join(lines) + "\n")
        prepare_assets(directory, "0.24.0")
        return directory

    def test_complete_assets_include_formula_and_match_binary_version(self):
        directory = self.assets()
        prepare_assets(directory, "0.24.0")
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
        prepare_assets(directory, "0.24.0")
        with self.assertRaisesRegex(ValueError, "this release's verified archives"):
            verify_assets(self.root, directory, "v0.24.0")

    def test_prepare_assets_builds_one_wheel_per_archive(self):
        directory = self.assets()
        wheels = sorted(path.name for path in directory.glob("*.whl"))
        self.assertEqual(len(wheels), 6)
        self.assertIn("skillshare_cli-0.24.0-py3-none-win_amd64.whl", wheels)
        self.assertIn("skillshare_cli-0.24.0-py3-none-manylinux_2_17_x86_64.manylinux2014_x86_64.musllinux_1_1_x86_64.whl", wheels)
        checksums = (directory / "checksums.txt").read_text()
        for name in wheels:
            self.assertEqual(checksums.count(f"  {name}\n"), 1)

    def test_prepare_assets_rerun_keeps_one_checksum_per_wheel(self):
        directory = self.assets()
        prepare_assets(directory, "0.24.0")
        checksums = (directory / "checksums.txt").read_text()
        self.assertEqual(checksums.count(".whl\n"), 6)

    def test_wheel_contains_only_the_binary_and_metadata(self):
        directory = self.assets()
        with zipfile.ZipFile(directory / "skillshare_cli-0.24.0-py3-none-win_amd64.whl") as wheel:
            self.assertEqual(sorted(wheel.namelist()), [
                "skillshare_cli-0.24.0.data/scripts/skillshare.exe",
                "skillshare_cli-0.24.0.dist-info/METADATA",
                "skillshare_cli-0.24.0.dist-info/RECORD",
                "skillshare_cli-0.24.0.dist-info/WHEEL",
            ])
            metadata = wheel.read("skillshare_cli-0.24.0.dist-info/METADATA").decode()
            self.assertIn("Name: skillshare-cli\n", metadata)
            self.assertIn("Version: 0.24.0\n", metadata)
            self.assertIn("Tag: py3-none-win_amd64\n", wheel.read("skillshare_cli-0.24.0.dist-info/WHEEL").decode())
            binary = wheel.getinfo("skillshare_cli-0.24.0.data/scripts/skillshare.exe")
            self.assertEqual(binary.external_attr >> 16, 0o100755)
            self.assertEqual(binary.create_system, 3)

    def test_wheel_record_hashes_every_other_entry(self):
        directory = self.assets()
        with zipfile.ZipFile(directory / "skillshare_cli-0.24.0-py3-none-macosx_12_0_arm64.whl") as wheel:
            record = wheel.read("skillshare_cli-0.24.0.dist-info/RECORD").decode().splitlines()
            self.assertEqual(record[-1], "skillshare_cli-0.24.0.dist-info/RECORD,,")
            for line in record[:-1]:
                path, digest, size = line.split(",")
                data = wheel.read(path)
                expected = base64.urlsafe_b64encode(hashlib.sha256(data).digest()).rstrip(b"=").decode()
                self.assertEqual((digest, int(size)), (f"sha256={expected}", len(data)))
            self.assertEqual(len(record), len(wheel.namelist()))

    def test_linux_wheel_lists_every_compressed_tag(self):
        directory = self.assets()
        name = "skillshare_cli-0.24.0-py3-none-manylinux_2_17_aarch64.manylinux2014_aarch64.musllinux_1_1_aarch64.whl"
        with zipfile.ZipFile(directory / name) as wheel:
            tags = [line for line in wheel.read("skillshare_cli-0.24.0.dist-info/WHEEL").decode().splitlines() if line.startswith("Tag: ")]
        self.assertEqual(tags, ["Tag: py3-none-manylinux_2_17_aarch64", "Tag: py3-none-manylinux2014_aarch64", "Tag: py3-none-musllinux_1_1_aarch64"])

    def test_wheels_are_reproducible(self):
        directory = self.assets()
        first = {path.name: path.read_bytes() for path in directory.glob("*.whl")}
        build_wheels(directory, "0.24.0")
        self.assertEqual(first, {path.name: path.read_bytes() for path in directory.glob("*.whl")})

    def test_archive_binary_rejects_a_symlink_member(self):
        path = self.root / "bad.tar.gz"
        with tarfile.open(path, "w:gz") as archive:
            member = tarfile.TarInfo("skillshare")
            member.type = tarfile.SYMTYPE
            member.linkname = "/bin/sh"
            archive.addfile(member)
        with self.assertRaisesRegex(ValueError, "regular skillshare binary in bad.tar.gz"):
            archive_binary(path)

    def test_archive_binary_rejects_a_zip_symlink_member(self):
        path = self.root / "bad.zip"
        with zipfile.ZipFile(path, "w") as archive:
            info = zipfile.ZipInfo("skillshare.exe")
            info.create_system = 3
            info.external_attr = 0o120777 << 16
            archive.writestr(info, "C:/Windows/System32/cmd.exe")
        with self.assertRaisesRegex(ValueError, "regular skillshare binary in bad.zip"):
            archive_binary(path)

    def test_archive_binary_rejects_a_missing_binary(self):
        path = self.root / "empty.zip"
        with zipfile.ZipFile(path, "w") as archive:
            archive.writestr("README.md", "# skillshare\n")
        with self.assertRaisesRegex(ValueError, "regular skillshare binary in empty.zip"):
            archive_binary(path)

    def rewrite_wheel(self, directory, name, change):
        """Pass every entry of one wheel through change(entry, data), keep its RECORD, and refresh its checksum."""
        path = directory / name
        with zipfile.ZipFile(path) as wheel:
            entries = [(info, change(info.filename, wheel.read(info))) for info in wheel.infolist()]
        with zipfile.ZipFile(path, "w") as wheel:
            for info, data in entries:
                wheel.writestr(info, data)
        checksum_file = directory / "checksums.txt"
        digest = hashlib.sha256(path.read_bytes()).hexdigest()
        checksum_file.write_text("".join(
            f"{digest}  {name}\n" if line.endswith(f"  {name}") else line + "\n"
            for line in checksum_file.read_text().splitlines()
        ))

    def test_assets_reject_a_missing_wheel(self):
        directory = self.assets()
        (directory / "skillshare_cli-0.24.0-py3-none-win_arm64.whl").unlink()
        checksum_file = directory / "checksums.txt"
        checksum_file.write_text("".join(line + "\n" for line in checksum_file.read_text().splitlines() if "win_arm64.whl" not in line))
        with self.assertRaisesRegex(ValueError, "all six archives"):
            verify_assets(self.root, directory, "v0.24.0")

    def test_assets_reject_a_wheel_whose_binary_differs_from_its_archive(self):
        directory = self.assets()
        name = "skillshare_cli-0.24.0-py3-none-macosx_12_0_arm64.whl"
        self.rewrite_wheel(directory, name, lambda entry, data: b"other binary" if entry.endswith("/skillshare") else data)
        with self.assertRaisesRegex(ValueError, f"Wheel binary differs from its archive: {name}"):
            verify_assets(self.root, directory, "v0.24.0")

    def test_assets_reject_a_wheel_with_the_wrong_version(self):
        directory = self.assets()
        name = "skillshare_cli-0.24.0-py3-none-win_amd64.whl"
        self.rewrite_wheel(directory, name, lambda entry, data: data.replace(b"Version: 0.24.0", b"Version: 0.23.1") if entry.endswith("METADATA") else data)
        with self.assertRaisesRegex(ValueError, f"Wheel metadata differs from the release: {name}"):
            verify_assets(self.root, directory, "v0.24.0")

    def test_assets_reject_a_wheel_with_an_extra_entry(self):
        directory = self.assets()
        name = "skillshare_cli-0.24.0-py3-none-win_amd64.whl"
        with zipfile.ZipFile(directory / name, "a") as wheel:
            wheel.writestr("skillshare_cli-0.24.0.data/scripts/helper.exe", b"extra")
        self.rewrite_wheel(directory, name, lambda entry, data: data)
        with self.assertRaisesRegex(ValueError, f"Wheel metadata differs from the release: {name}"):
            verify_assets(self.root, directory, "v0.24.0")

    def test_assets_reject_a_wheel_with_a_duplicate_entry(self):
        directory = self.assets()
        name = "skillshare_cli-0.24.0-py3-none-win_amd64.whl"
        script = "skillshare_cli-0.24.0.data/scripts/skillshare.exe"
        with zipfile.ZipFile(directory / name) as wheel:
            binary = wheel.read(script)
        with warnings.catch_warnings():
            warnings.simplefilter("ignore")  # zipfile warns about the duplicate name this test needs.
            with zipfile.ZipFile(directory / name, "a") as wheel:
                wheel.writestr(script, binary)
            self.rewrite_wheel(directory, name, lambda entry, data: data)
        with self.assertRaisesRegex(ValueError, f"Wheel metadata differs from the release: {name}"):
            verify_assets(self.root, directory, "v0.24.0")

    def test_assets_reject_a_wheel_whose_record_does_not_match(self):
        directory = self.assets()
        name = "skillshare_cli-0.24.0-py3-none-win_amd64.whl"
        self.rewrite_wheel(directory, name, lambda entry, data: data.replace(b"Generator: skillshare", b"Generator: tampered") if entry.endswith("/WHEEL") else data)
        with self.assertRaisesRegex(ValueError, f"Wheel RECORD does not match its contents: {name}"):
            verify_assets(self.root, directory, "v0.24.0")


if __name__ == "__main__":
    unittest.main()

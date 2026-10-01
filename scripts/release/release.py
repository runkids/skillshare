#!/usr/bin/env python3
"""Synchronize release metadata and verify the exact artifacts before publication."""

import argparse
import hashlib
import json
from pathlib import Path
import platform
import re
import shutil
import subprocess
import tarfile
import tempfile


VERSION = r"(?:0|[1-9]\d*)\.(?:0|[1-9]\d*)\.(?:0|[1-9]\d*)"
HEADING = re.compile(
    rf"^###? \[?v?({VERSION})\]?(?:\([^\n]*\))?"
    r"(?: - (\d{4}-\d{2}-\d{2})| \((\d{4}-\d{2}-\d{2})\))[ \t]*$",
    re.MULTILINE,
)
SKILL_VERSION = re.compile(rf"^(  version: )v({VERSION})$", re.MULTILINE)


def manifest_version(root):
    version = json.loads((root / ".github/release-please-manifest.json").read_text())["."]
    if not re.fullmatch(VERSION, version):
        raise ValueError("Only stable X.Y.Z release versions are supported")
    return version


def first_entry(text):
    headings = list(HEADING.finditer(text))
    if not headings:
        raise ValueError("No dated release heading found")
    first = headings[0]
    end = headings[1].start() if len(headings) > 1 else len(text)
    return first, end, text[first.start():end].rstrip() + "\n\n"


def sync(root):
    version = manifest_version(root)
    changelog = root / "CHANGELOG.md"
    text = changelog.read_text()
    heading, end, entry = first_entry(text)
    if heading[1] != version:
        raise ValueError("Changelog version differs from the release manifest")
    date = heading[2] or heading[3]
    entry = f"## [{version}] - {date}" + entry[entry.index("\n"):]
    website = root / "website/src/pages/changelog.md"
    web_text = website.read_text()
    web_heading, web_end, _ = first_entry(web_text)
    if tuple(map(int, web_heading[1].split("."))) > tuple(map(int, version.split("."))):
        raise ValueError("Refusing to replace a newer website release")
    insertion_end = web_end if web_heading[1] == version else web_heading.start()
    skill = root / "skills/skillshare/SKILL.md"
    skill_text, count = SKILL_VERSION.subn(rf"\g<1>v{version}", skill.read_text())
    if count != 1:
        raise ValueError("Expected exactly one built-in skill metadata version")
    changelog.write_text(text[:heading.start()] + entry + text[end:])
    website.write_text(web_text[:web_heading.start()] + entry + web_text[insertion_end:])
    skill.write_text(skill_text)


def check(root, tag=None, sha=None):
    version = manifest_version(root)
    if tag is not None and tag != f"v{version}":
        raise ValueError("Tag differs from the release manifest")
    if sha is not None:
        actual = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=root, text=True).strip()
        if not re.fullmatch(r"[0-9a-f]{40}", sha) or actual != sha:
            raise ValueError("Checkout differs from the approved release commit")
    _, _, entry = first_entry((root / "CHANGELOG.md").read_text())
    heading, _, web_entry = first_entry((root / "website/src/pages/changelog.md").read_text())
    if not entry.startswith(f"## [{version}] - ") or heading[1] != version or entry != web_entry:
        raise ValueError("The manifest and both changelog entries must match")
    matches = SKILL_VERSION.findall((root / "skills/skillshare/SKILL.md").read_text())
    if len(matches) != 1 or matches[0][1] != version:
        raise ValueError("Built-in skill version differs from the release manifest")
    return version


def required_assets(version):
    return {
        f"skillshare_{version}_{os_name}_{arch}.{('zip' if os_name == 'windows' else 'tar.gz')}"
        for os_name in ("darwin", "linux", "windows")
        for arch in ("amd64", "arm64")
    } | {"skillshare-ui-dist.tar.gz", "skillshare.rb"}


def prepare_assets(directory):
    shutil.copyfile(directory.parent / "skillshare-ui-dist.tar.gz", directory / "skillshare-ui-dist.tar.gz")
    formula = directory / "homebrew/Formula/skillshare.rb"
    shutil.copyfile(formula, directory / "skillshare.rb")
    checksum_file = directory / "checksums.txt"
    lines = [line for line in checksum_file.read_text().splitlines() if not line.endswith("  skillshare.rb")]
    digest = hashlib.sha256(formula.read_bytes()).hexdigest()
    checksum_file.write_text("\n".join(lines) + f"\n{digest}  skillshare.rb\n")


def verify_assets(root, directory, tag):
    version = check(root, tag)
    checksums = {}
    for line in (directory / "checksums.txt").read_text().splitlines():
        match = re.fullmatch(r"([0-9a-f]{64})  (.+)", line)
        if not match or match[2] in checksums:
            raise ValueError("Malformed or duplicate checksum entry")
        checksums[match[2]] = match[1]
    if set(checksums) != required_assets(version):
        raise ValueError("Checksums must contain all six archives, UI assets and the Homebrew formula")
    for name, digest in checksums.items():
        if hashlib.sha256((directory / name).read_bytes()).hexdigest() != digest:
            raise ValueError(f"Checksum mismatch: {name}")
    formula = (directory / "skillshare.rb").read_text()
    if f'version "{version}"' not in formula:
        raise ValueError("Homebrew formula version differs from the release manifest")
    urls = re.findall(r'url "(https://github.com/runkids/skillshare/releases/download/[^\"]+)"', formula)
    prefix = f"https://github.com/runkids/skillshare/releases/download/{tag}/"
    if not urls or any(not url.startswith(prefix) or url[len(prefix):] not in checksums for url in urls):
        raise ValueError("Homebrew formula must download this release's verified archives")
    # Read only the expected binary, never extract arbitrary archive paths.
    arch = {"x86_64": "amd64", "aarch64": "arm64"}[platform.machine()]
    with tarfile.open(directory / f"skillshare_{version}_linux_{arch}.tar.gz") as archive:
        member = archive.getmember("skillshare")
        if not member.isfile():
            raise ValueError("Expected a regular skillshare binary in the Linux archive")
        with tempfile.TemporaryDirectory(prefix="skillshare-release-binary-") as temp:
            binary = Path(temp) / "skillshare"
            binary.write_bytes(archive.extractfile(member).read())
            binary.chmod(0o700)
            output = subprocess.check_output([str(binary), "--version"], text=True).strip()
            if output != f"skillshare v{version}":
                raise ValueError("Packaged CLI version differs from the release manifest")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=("sync", "check", "notes", "prepare-assets", "verify-assets"))
    parser.add_argument("--root", type=Path, default=Path.cwd())
    parser.add_argument("--tag")
    parser.add_argument("--sha")
    parser.add_argument("--assets", type=Path)
    args = parser.parse_args()
    try:
        if args.command == "sync":
            sync(args.root)
            check(args.root)
        elif args.command == "check":
            print(check(args.root, args.tag, args.sha))
        elif args.command == "notes":
            check(args.root, args.tag)
            print(first_entry((args.root / "CHANGELOG.md").read_text())[2].split("\n", 1)[1].strip())
        else:
            if args.assets is None or (args.command == "verify-assets" and args.tag is None):
                parser.error("--assets is required; verify-assets also requires --tag")
            if args.command == "prepare-assets":
                prepare_assets(args.assets)
            else:
                verify_assets(args.root, args.assets, args.tag)
    except (ValueError, KeyError) as error:
        parser.exit(1, f"Release verification failed: {error}\n")


if __name__ == "__main__":
    main()

"""Build the standalone Herdr mod package from the layout's canonical mod."""

import argparse
import shutil
import subprocess
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
SOURCE = ROOT / "herdr" / "skillshare"
DEST = ROOT / "herdr" / "skillshare-mod"


def git_files(path):
    # Git excludes local editor files while including new, not-yet-staged assets.
    names = subprocess.check_output(
        ["git", "ls-files", "--cached", "--others", "--exclude-standard", "-z", "--",
         str(path.relative_to(ROOT)) + "/"], cwd=ROOT, text=True).split("\0")
    return [ROOT / name for name in names if name and (ROOT / name).is_file()]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--check", action="store_true", help="check without writing files")
    args = parser.parse_args()
    expected = {Path("preview.png"): SOURCE / "panels.png"}
    expected.update({path.relative_to(SOURCE): path for path in git_files(SOURCE / "mods")})
    actual = {path.relative_to(DEST) for path in git_files(DEST / "mods")}
    stale = actual - expected.keys()
    changed = [name for name, source in expected.items()
               if not (DEST / name).is_file() or (DEST / name).read_bytes() != source.read_bytes()]
    if args.check:
        if changed or stale:
            parser.exit(1, "Herdr mod package is stale; run python3 scripts/herdr/package-mod.py\n")
        print("Herdr mod package is up to date")
        return
    for name in stale:
        (DEST / name).unlink()
    for name in changed:
        target = DEST / name
        target.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(expected[name], target)
    print(f"Updated {len(changed)} Herdr mod package files")


if __name__ == "__main__":
    main()

#!/usr/bin/env python3
"""Convert a markdown subagent (YAML frontmatter + body) to a Codex CLI TOML agent.

Reads markdown from stdin, writes TOML to stdout.
Mapping:
  frontmatter `name`        -> TOML `name` (falls back to SS_REL_PATH filename)
  frontmatter `description` -> TOML `description`
  markdown body             -> TOML `developer_instructions` (triple-quoted)
Unknown frontmatter keys are dropped.
"""
import os
import sys
import re


def split_frontmatter(text):
    if text.startswith("---"):
        end = text.find("\n---", 3)
        if end != -1:
            return text[3:end].strip("\n"), text[end + 4:].lstrip("\n")
    return "", text


def get_key(fm, key):
    for line in fm.splitlines():
        m = re.match(r"\s*%s\s*:\s*(.+?)\s*$" % re.escape(key), line)
        if m:
            return m.group(1).strip().strip("\"'")
    return ""


def toml_escape_basic(s):
    return s.replace("\\", "\\\\").replace('"', '\\"')


def main():
    text = sys.stdin.read()
    fm, body = split_frontmatter(text)
    name = get_key(fm, "name")
    if not name:
        rel = os.environ.get("SS_REL_PATH", "agent.md")
        name = os.path.splitext(os.path.basename(rel))[0]
    description = get_key(fm, "description")

    out = ['name = "%s"' % toml_escape_basic(name)]
    if description:
        out.append('description = "%s"' % toml_escape_basic(description))
    fence = '"""'
    if fence in body:
        fence = "'''"
    out.append("developer_instructions = %s\n%s\n%s" % (fence, body.rstrip("\n"), fence))
    sys.stdout.write("\n".join(out) + "\n")


if __name__ == "__main__":
    main()

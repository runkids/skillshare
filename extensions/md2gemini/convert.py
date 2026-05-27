#!/usr/bin/env python3
"""Convert a markdown command (YAML frontmatter + body) to a Gemini CLI TOML command.

Reads markdown from stdin, writes TOML to stdout.
Mapping:
  frontmatter `description` -> TOML `description`
  markdown body            -> TOML `prompt` (triple-quoted)
  Claude placeholders $ARGUMENTS / $1.. -> Gemini {{args}}
Unknown frontmatter keys are dropped.
"""
import sys
import re


def split_frontmatter(text):
    if text.startswith("---"):
        end = text.find("\n---", 3)
        if end != -1:
            fm = text[3:end].strip("\n")
            body = text[end + 4:].lstrip("\n")
            return fm, body
    return "", text


def get_description(fm):
    for line in fm.splitlines():
        m = re.match(r"\s*description\s*:\s*(.+?)\s*$", line)
        if m:
            return m.group(1).strip().strip("\"'")
    return ""


def toml_escape_basic(s):
    return s.replace("\\", "\\\\").replace('"', '\\"')


def main():
    text = sys.stdin.read()
    fm, body = split_frontmatter(text)
    description = get_description(fm)
    prompt = re.sub(r"\$ARGUMENTS\b", "{{args}}", body)
    prompt = re.sub(r"\$\d+", "{{args}}", prompt)

    out = []
    if description:
        out.append('description = "%s"' % toml_escape_basic(description))
    fence = '"""'
    if fence in prompt:
        fence = "'''"
    out.append("prompt = %s\n%s\n%s" % (fence, prompt.rstrip("\n"), fence))
    sys.stdout.write("\n".join(out) + "\n")


if __name__ == "__main__":
    main()

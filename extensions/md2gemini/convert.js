#!/usr/bin/env node
// Convert a markdown command (YAML frontmatter + body) to a Gemini CLI TOML command.
//
// Reads markdown from stdin, writes TOML to stdout.
// Mapping:
//   frontmatter `description` -> TOML `description`
//   markdown body            -> TOML `prompt` (triple-quoted)
//   Claude placeholders $ARGUMENTS / $1.. -> Gemini {{args}}
// Unknown frontmatter keys are dropped.

function splitFrontmatter(text) {
  if (text.startsWith("---")) {
    const end = text.indexOf("\n---", 3);
    if (end !== -1) {
      const fm = text.slice(3, end).replace(/^\n+|\n+$/g, "");
      const body = text.slice(end + 4).replace(/^\n+/, "");
      return [fm, body];
    }
  }
  return ["", text];
}

function getDescription(fm) {
  for (const line of fm.split("\n")) {
    const m = line.match(/^\s*description\s*:\s*(.+?)\s*$/);
    if (m) return m[1].trim().replace(/^["']|["']$/g, "");
  }
  return "";
}

function tomlEscapeBasic(s) {
  return s.replace(/\\/g, "\\\\").replace(/"/g, '\\"');
}

function readStdin() {
  return new Promise((resolve) => {
    let data = "";
    process.stdin.setEncoding("utf8");
    process.stdin.on("data", (chunk) => (data += chunk));
    process.stdin.on("end", () => resolve(data));
  });
}

(async () => {
  const text = await readStdin();
  const [fm, body] = splitFrontmatter(text);
  const description = getDescription(fm);
  let prompt = body.replace(/\$ARGUMENTS\b/g, "{{args}}").replace(/\$\d+/g, "{{args}}");

  const out = [];
  if (description) out.push(`description = "${tomlEscapeBasic(description)}"`);
  let fence = '"""';
  if (prompt.includes(fence)) fence = "'''";
  out.push(`prompt = ${fence}\n${prompt.replace(/\n+$/, "")}\n${fence}`);
  process.stdout.write(out.join("\n") + "\n");
})();

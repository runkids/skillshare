#!/usr/bin/env node
// Convert a markdown subagent (YAML frontmatter + body) to a Codex CLI TOML agent.
//
// Reads markdown from stdin, writes TOML to stdout.
// Mapping:
//   frontmatter `name`        -> TOML `name` (falls back to SS_REL_PATH filename)
//   frontmatter `description` -> TOML `description`
//   markdown body             -> TOML `developer_instructions` (triple-quoted)
// Unknown frontmatter keys are dropped.

const path = require("path");

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

function getKey(fm, key) {
  const re = new RegExp("^\\s*" + key.replace(/[.*+?^${}()|[\]\\]/g, "\\$&") + "\\s*:\\s*(.+?)\\s*$");
  for (const line of fm.split("\n")) {
    const m = line.match(re);
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
  let name = getKey(fm, "name");
  if (!name) {
    const rel = process.env.SS_REL_PATH || "agent.md";
    name = path.basename(rel, path.extname(rel));
  }
  const description = getKey(fm, "description");

  const out = [`name = "${tomlEscapeBasic(name)}"`];
  if (description) out.push(`description = "${tomlEscapeBasic(description)}"`);
  let fence = '"""';
  if (body.includes(fence)) fence = "'''";
  out.push(`developer_instructions = ${fence}\n${body.replace(/\n+$/, "")}\n${fence}`);
  process.stdout.write(out.join("\n") + "\n");
})();

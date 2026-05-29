const path = require("path");

function block(value) {
  return { type: "block", value };
}

function convert(mapFields) {
  readStdin().then((input) => {
    const doc = parseMarkdown(input);
    const fields = mapFields(doc) || {};
    const lines = [];

    for (const [key, value] of Object.entries(fields)) {
      if (value == null || value === "") continue;
      if (value.type === "block") {
        lines.push(`${key} = ${tomlBlock(value.value)}`);
        continue;
      }
      lines.push(`${key} = ${tomlString(value)}`);
    }

    process.stdout.write(lines.join("\n") + (lines.length ? "\n" : ""));
  });
}

function parseMarkdown(input) {
  const { frontmatterText, body } = splitFrontmatter(input);
  const relPath = process.env.SS_REL_PATH || "input.md";

  return {
    body,
    frontmatter: parseFrontmatter(frontmatterText),
    relPath,
    stem: path.basename(relPath, path.extname(relPath)),
  };
}

function splitFrontmatter(input) {
  const match = input.match(/^---\r?\n([\s\S]*?)\r?\n---[ \t]*\r?\n?/);
  if (!match) return { frontmatterText: "", body: input };

  return {
    frontmatterText: match[1],
    body: input.slice(match[0].length),
  };
}

function parseFrontmatter(text) {
  const frontmatter = {};
  for (const line of text.split(/\r?\n/)) {
    const match = line.match(/^\s*([A-Za-z0-9_-]+)\s*:\s*(.*?)\s*$/);
    if (match) frontmatter[match[1]] = stripQuotes(match[2]);
  }
  return frontmatter;
}

function stripQuotes(value) {
  if (value.length < 2) return value;
  const first = value[0];
  const last = value[value.length - 1];
  if ((first === `"` && last === `"`) || (first === `'` && last === `'`)) {
    return value.slice(1, -1);
  }
  return value;
}

function tomlString(value) {
  // JSON basic-string escaping is a valid subset of TOML basic strings and,
  // unlike a hand-rolled escape of just \ and ", also handles control
  // characters (newlines, tabs, etc.) that would otherwise produce invalid TOML.
  return JSON.stringify(String(value));
}

function tomlBlock(value) {
  const text = String(value).replace(/\n+$/, "");
  if (!text.includes(`"""`)) return `"""\n${text}\n"""`;
  if (!text.includes(`'''`)) return `'''\n${text}\n'''`;
  return `"""\n${text.replace(/"""/g, `\\"""`)}\n"""`;
}

function readStdin() {
  return new Promise((resolve) => {
    let data = "";
    process.stdin.setEncoding("utf8");
    process.stdin.on("data", (chunk) => (data += chunk));
    process.stdin.on("end", () => resolve(data));
  });
}

module.exports = { block, convert };

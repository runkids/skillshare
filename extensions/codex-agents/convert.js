#!/usr/bin/env node
const { block, convert } = require("./md-toml");

convert(({ body, frontmatter, stem }) => {
  if (!frontmatter.description) {
    throw new Error(
      "codex-agents: missing required frontmatter 'description' (Codex custom agents require description)"
    );
  }

  return {
    name: frontmatter.name || stem,
    description: frontmatter.description,
    model: frontmatter.model,
    developer_instructions: block(body),
  };
});

#!/usr/bin/env node
const { block, convert } = require("./md-toml");

convert(({ body, frontmatter, stem }) => {
  const name = (frontmatter.name || stem).trim();
  const description = (frontmatter.description || "").trim();
  const developerInstructions = body.trim();

  if (!name) {
    throw new Error(
      "codex-agents: missing required field 'name' (Codex custom agents require name)"
    );
  }
  if (!description) {
    throw new Error(
      "codex-agents: missing required frontmatter 'description' (Codex custom agents require description)"
    );
  }
  if (!developerInstructions) {
    throw new Error(
      "codex-agents: missing required markdown body (Codex custom agents require developer_instructions)"
    );
  }

  return {
    name,
    description,
    model: frontmatter.model,
    developer_instructions: block(developerInstructions),
  };
});

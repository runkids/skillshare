#!/usr/bin/env node
const { block, convert } = require("./md-toml");

convert(({ body, frontmatter, stem }) => ({
  name: frontmatter.name || stem,
  description: frontmatter.description,
  model: frontmatter.model,
  developer_instructions: block(body),
}));

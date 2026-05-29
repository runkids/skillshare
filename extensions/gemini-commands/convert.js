#!/usr/bin/env node
const { block, convert } = require("./md-toml");

convert(({ body, frontmatter }) => ({
  description: frontmatter.description,
  prompt: block(body.replace(/\$ARGUMENTS\b|\$\d+/g, "{{args}}")),
}));

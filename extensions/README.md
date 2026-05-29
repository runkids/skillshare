# Extensions

This directory contains reference extensions for transforming Markdown extras into tool-specific files during `skillshare sync extras`.

The reference extensions use plain JavaScript with Node.js. Extensions are not limited to JavaScript, but plain JS is the most practical cross-platform default for directly editable examples:

- Windows, macOS, and Linux can run the same script with `node`.
- No TypeScript, npm install, bundling, or build step is required.
- Users can edit the script directly.
- Bash is fine for personal Unix-only scripts, but it is not a good default reference for Windows/macOS/Linux support.

## How Extensions Run

An extras target can set `extension: <name>`:

```yaml
extras:
  - name: commands
    targets:
      - path: ~/.gemini/commands
        extension: gemini-commands
```

During sync, Skillshare:

1. Reads each source Markdown file.
2. Sends the file content to the extension on `stdin`.
3. Writes the extension's `stdout` to the target file.
4. Uses `output_ext` from `extension.yaml` to choose the output extension, for example `.md` to `.toml`.

## Reference Layout

Each reference extension is self-contained. Copy the whole directory when you want to customize one.

```text
extensions/gemini-commands/
├── extension.yaml
├── convert.js
└── md-toml.js
```

`extension.yaml` tells Skillshare how to run the extension:

```yaml
run: ["node", "convert.js"]
output_ext: toml
description: "Markdown slash-command (Claude/Cursor style) → Gemini CLI TOML command"
```

`convert.js` is the file users are expected to edit. It only contains the field mapping:

```js
const { block, convert } = require("./md-toml");

convert(({ body, frontmatter }) => ({
  description: frontmatter.description,
  prompt: block(body.replace(/\$ARGUMENTS\b|\$\d+/g, "{{args}}")),
}));
```

`md-toml.js` is a helper for the repetitive parts that are easy to get wrong:

- reading `stdin`
- parsing simple Markdown frontmatter
- deriving `stem` from `SS_REL_PATH`
- escaping TOML strings
- writing TOML output

## Mapping API

`convert()` receives one object:

```js
{
  body,          // Markdown body after frontmatter
  frontmatter,  // simple key/value frontmatter
  relPath,      // SS_REL_PATH, relative to the extras source root
  stem          // filename without extension
}
```

Return an object whose keys become TOML fields:

```js
convert(({ body, frontmatter, stem }) => ({
  name: frontmatter.name || stem,
  description: frontmatter.description,
  developer_instructions: block(body),
}));
```

Plain values become TOML strings:

```toml
name = "review"
```

Values wrapped with `block()` become TOML multiline strings:

```toml
developer_instructions = """
Review carefully.
"""
```

Empty values are skipped.

## Test A Converter Locally

From the repository root:

```bash
printf '%s\n' '---' 'description: Run things' '---' 'Use $ARGUMENTS' \
  | SS_REL_PATH=run.md node extensions/gemini-commands/convert.js
```

Expected shape:

```toml
description = "Run things"
prompt = """
Use {{args}}
"""
```

## Create Your Own Extension

1. Copy one reference directory:

   ```bash
   cp -R extensions/gemini-commands ~/.config/skillshare/extensions/my-command-format
   ```

2. Edit `extension.yaml`:

   ```yaml
   run: ["node", "convert.js"]
   output_ext: toml
   description: "My Markdown command converter"
   ```

3. Edit `convert.js` to map Markdown into the target tool's format.

4. Use it from extras config:

   ```yaml
   extras:
     - name: commands
       targets:
         - path: ~/.my-tool/commands
           extension: my-command-format
   ```

5. Run:

   ```bash
   skillshare sync extras
   ```

For the full extension contract, see [Extension transforms](../website/docs/reference/commands/extras.md#extension-transforms).

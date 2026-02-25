---
sidebar_position: 3
---

# Skill Design

How to write skills that work reliably — choosing the right complexity level, maximizing determinism, and using progressive disclosure.

:::tip When does this matter?
If your skills work inconsistently, if cheap models fail at your skills, or if you're building skills for a team — this guide helps you write skills that are **reliable, secure, and efficient**.
:::

## The Skill Spectrum

Not all skills are created equal. Understanding where your skill falls on the complexity spectrum helps you make the right design choices:

| Level | Style | Determinism | Model Cost | Best For |
|-------|-------|-------------|------------|----------|
| **Passive** | Context only | N/A | Lowest | Background knowledge, coding standards |
| **Instructional** | Rules + guidelines | Medium | Low | Code review, style guides |
| **CLI Wrapper** | Calls a compiled binary | **High** | **Low** | Automation, integrations, data processing |
| **Workflow** | Multi-step with validation | Medium | Medium | Deploy pipelines, migrations |
| **Generative** | Asks agent to write code | Low | High | Scaffolding, code generation |

**The key insight: move left on this spectrum whenever possible.** Simpler skills are more reliable, cheaper to execute, and work across more models.

---

## Principle 1: Determinism First

The most important quality of a well-designed skill is **determinism** — the same input should produce the same output every time.

### Why determinism matters

- **Cheap models can run deterministic skills.** A skill that says "run `eslint --fix`" works on any model. A skill that says "analyze the code and suggest improvements" requires expensive reasoning.
- **Deterministic skills don't break.** CLI commands either succeed or fail with a clear error. Ambiguous instructions fail silently or produce inconsistent results.
- **Teams need predictability.** If a skill produces different results for different team members, it creates confusion.

### How to increase determinism

**Prefer commands over descriptions:**

```markdown
# ✅ Deterministic — any model can run this
Run the formatter:
`prettier --write "src/**/*.{ts,tsx}"`

# ❌ Non-deterministic — model must reason about formatting rules
Format the code following the project's style conventions.
Ensure consistent indentation, trailing commas, and import ordering.
```

**Prefer scripts over instructions:**

```markdown
# ✅ Deterministic — execute a script
Run `./scripts/deploy.sh staging` to deploy.

# ❌ Non-deterministic — model must reconstruct the deploy flow
Deploy to staging:
1. Build the project
2. Run tests
3. Push to the staging branch
4. Wait for CI
5. Verify the deployment
```

**Prefer explicit values over judgment:**

```markdown
# ✅ Deterministic
Block any file larger than 100KB.

# ❌ Non-deterministic
Block files that are too large.
```

---

## Principle 2: CLI Wrapper Pattern

The most powerful technique for reliable skills: **wrap logic in a compiled CLI binary, then have the skill call it.**

### The pattern

```
my-tool/                  # Compiled binary (Go, Rust, Swift, Bun)
├── main.go
└── ...

my-skill/                 # Skill just calls the binary
└── SKILL.md
```

```markdown title="SKILL.md"
---
name: my-tool
description: Processes data files with my-tool CLI
---

# My Tool

Use the `my-tool` CLI for data processing tasks.

## Commands

- `my-tool convert <input> <output>` — Convert between formats
- `my-tool validate <file>` — Check file integrity
- `my-tool analyze <file> --json` — Output analysis as JSON
```

### Why this works

1. **Zero runtime dependencies.** A Go or Rust binary has no `node_modules`, no `pip install`, no version conflicts.
2. **Binary behavior is fixed.** The same binary version produces the same results on every machine.
3. **Security.** No supply chain risk from transitive dependencies. The binary is self-contained.
4. **Works for cheap models.** Even the smallest model can execute `my-tool convert a.csv b.json`.

### Real-world examples

[Peter Steinberger](https://github.com/steipete) (PSPDFKit founder) builds compiled CLIs for everything his AI agents need:

| CLI | Language | Purpose |
|-----|----------|---------|
| `gogcli` | Go | Google Suite (Gmail, Calendar, Drive) |
| `peekaboo` | Swift | macOS screenshots for AI vision |
| `imsg` | Swift | Send/receive iMessages |
| `mcporter` | Bun | Convert MCP servers into CLI binaries |

His approach: **SKILL.md is a one-line instruction, the binary does all the work.**

> "Agents are really, really good at calling CLIs — actually much better than calling MCPs. You don't have to clutter up your context and you can use all the features on demand."
> — [Peekaboo 2.0](https://steipete.me/posts/2025/peekaboo-2-freeing-the-cli-from-its-mcp-shackles)

### When to use this pattern

- You have complex logic that shouldn't live in a prompt
- You need reproducible behavior across team members
- You're integrating with external services (APIs, databases, cloud)
- Security matters (no dependency supply chain)

### When NOT to use this pattern

- Simple knowledge or conventions (use instructional skills instead)
- The logic is genuinely different every time (use generative skills)
- You don't have time to build a CLI (start with instructions, refactor later)

---

## Principle 3: Progressive Disclosure

Don't dump everything into SKILL.md. Layer your content so the AI loads only what it needs.

### Three layers

```
my-skill/
├── SKILL.md           # Layer 1: Always loaded (~100 tokens in description)
├── references/        # Layer 2: Loaded on demand
│   ├── api-guide.md
│   └── patterns.md
├── scripts/           # Layer 3: Executed, not loaded into context
│   └── validate.sh
└── examples/          # Layer 3: Referenced by path
    └── sample.json
```

**Layer 1 — Metadata** (always in context):
Your `name` + `description` in frontmatter. Keep under 200 characters. This is what the AI uses to decide whether to activate the skill.

**Layer 2 — Body + References** (loaded when skill activates):
The SKILL.md body and any referenced files. Keep SKILL.md under 500 lines. Put detailed docs in `references/`.

**Layer 3 — Scripts + Assets** (executed or path-referenced, never loaded):
Scripts run via Bash, templates copied to output. These don't consume context tokens.

### Context window is a shared resource

Every token in your skill competes with the user's code, conversation history, and other skills. Ask yourself:

> "Is this line worth the context tokens it costs?"

**Before:**
```markdown
## Background

PDF (Portable Document Format) was developed by Adobe in 1993. It's widely used
for document exchange because it preserves formatting across platforms. PDFs can
contain text, images, forms, and multimedia. The PDF specification is maintained
by ISO as ISO 32000...

## Instructions

Use pdfplumber to extract text from PDF files.
```

**After:**
```markdown
Use `pdfplumber` for text extraction:

    import pdfplumber
    with pdfplumber.open("file.pdf") as pdf:
        text = pdf.pages[0].extract_text()
```

The AI already knows what PDF is. Only add what it doesn't know.

---

## Principle 4: Match Complexity to Risk

Use the "narrow bridge vs open field" heuristic:

| Scenario | Risk | Freedom | Approach |
|----------|------|---------|----------|
| Database migration | High | Low | Exact commands, validation steps, rollback plan |
| Code review | Low | High | General guidelines, let AI use judgment |
| Deploy to production | High | Low | Script with explicit steps and checks |
| Write documentation | Low | High | Style guide + examples |

**High-risk operations need low-freedom skills:**

```markdown
## Database Migration

⚠️ Follow these steps EXACTLY in order:

1. Create backup: `pg_dump -Fc mydb > backup_$(date +%Y%m%d).dump`
2. Run migration: `psql mydb < migrations/0042_add_index.sql`
3. Verify: `psql mydb -c "SELECT count(*) FROM pg_indexes WHERE indexname = 'idx_users_email'"`
4. If verification fails, rollback: `pg_restore -d mydb backup_*.dump`
```

**Low-risk operations can be high-freedom:**

```markdown
## Code Review Guidelines

When reviewing code, consider:
- Are there obvious bugs or edge cases?
- Is the code readable and well-structured?
- Are there performance concerns?

Adapt your review depth to the change size.
```

---

## Principle 5: Design the Interface First

Before writing a skill, define its contract — what triggers it, what it does, and what it produces.

### Five questions to answer

1. **When should this skill activate?** Write the `description` field as if teaching a new team member when to use this tool.
2. **What inputs does it need?** Arguments, files, environment state?
3. **What does success look like?** Specific output format, files created, commands run?
4. **What should it NOT do?** Explicit exclusions prevent scope creep.
5. **How do you verify it worked?** Include a validation step.

### Template

```markdown
---
name: {name}
description: {what it does}. Use when {trigger condition}.
---

# {Name}

{One sentence: what this does.}

## When to Use

{Specific trigger conditions — be precise}

## Instructions

{Steps — ordered, concrete, verifiable}

## Verify

{How to confirm it worked}

## When NOT to Use

{Explicit exclusions}
```

---

## Anti-Patterns

Common mistakes that make skills unreliable:

### 1. The kitchen sink

```markdown
# ❌ Too many responsibilities
This skill handles code review, testing, deployment,
documentation updates, and changelog generation.
```

**Fix:** One skill = one purpose. Split into separate skills.

### 2. Vague instructions

```markdown
# ❌ Agent must guess what "properly" means
Ensure the code is properly formatted and follows best practices.
```

**Fix:** Name the specific tools and rules.

```markdown
# ✅ Specific and actionable
Run `prettier --write .` to format. Run `eslint --fix .` to lint.
```

### 3. Explaining what the AI already knows

```markdown
# ❌ Wasting context tokens
React is a JavaScript library for building user interfaces.
Components are reusable pieces of UI. Props are passed from
parent to child components...
```

**Fix:** Only add what the AI doesn't know — your project's specific conventions, internal APIs, domain rules.

### 4. Too many options

```markdown
# ❌ Choice paralysis
You can use pdfplumber, PyMuPDF, pdfminer, tabula-py, or camelot
depending on the use case...
```

**Fix:** Give one default, mention alternatives only if needed.

```markdown
# ✅ Clear default
Use `pdfplumber` for text extraction. For scanned PDFs, fall back to `pytesseract`.
```

### 5. No verification step

```markdown
# ❌ No way to confirm success
Deploy the application to staging.
```

**Fix:** Always include how to verify.

```markdown
# ✅ Verifiable
Deploy to staging:
1. Run `make deploy-staging`
2. Verify: `curl -s https://staging.example.com/health | jq .status`
   Expected: `"ok"`
```

### 6. Hardcoded paths

```markdown
# ❌ Breaks on other machines
Edit the file at /Users/john/projects/my-app/src/config.ts
```

**Fix:** Use relative paths or environment variables.

---

## Testing Your Skills

### Cross-model testing

Test on multiple model tiers:
- **Cheap model** (e.g., Haiku): Can it follow the instructions? If not, simplify.
- **Mid-tier model** (e.g., Sonnet): Does it produce consistent results?
- **Top model** (e.g., Opus): Does it respect the boundaries, or does it "improve" beyond scope?

### The simplicity test

> If a cheap model can't execute your skill reliably, the skill is too complex.

This is the strongest signal that you need to:
- Extract logic into a script or CLI binary
- Reduce ambiguity in instructions
- Add explicit commands instead of descriptions

### Iteration loop

```
Write skill → Sync → Test in AI CLI → Observe behavior → Edit → Repeat
```

Use `skillshare sync` to deploy changes, then test in your AI CLI. Watch for:
- Does the AI activate the skill at the right time?
- Does it follow steps in order?
- Does it skip or improvise steps?
- Does the verification step catch failures?

---

## Summary

| Principle | One-liner |
|-----------|-----------|
| **Determinism First** | Commands over descriptions, scripts over instructions |
| **CLI Wrapper Pattern** | Complex logic → compiled binary, skill → thin wrapper |
| **Progressive Disclosure** | Layer content: metadata → body → references → scripts |
| **Match Complexity to Risk** | High risk = exact steps; low risk = guidelines |
| **Design Interface First** | Define trigger, inputs, outputs, exclusions before writing |

---

## See Also

- [Creating Skills](/docs/how-to/daily-tasks/creating-skills) — Step-by-step creation guide
- [Best Practices](/docs/how-to/daily-tasks/best-practices) — Naming, organization, version control
- [Skill Format](/docs/understand/skill-format) — SKILL.md structure and metadata
- [Securing Your Skills](/docs/how-to/advanced/security) — Security scanning and audit

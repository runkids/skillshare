# AGENTS.md

This guidance applies to the `website/` directory.

This is the **documentation website** for the skillshare CLI. See the parent `../AGENTS.md` for CLI/Go codebase details and the project-wide rules (run frontend tooling inside the devcontainer, never on the host).

## Commands

```bash
pnpm start          # Dev server with hot reload (localhost:3000)
pnpm run build      # Production build → ./build/ (fails on broken links)
pnpm run serve      # Serve production build locally
pnpm run typecheck  # TypeScript type checking (tsc)
pnpm run clear      # Clear Docusaurus cache (.docusaurus/)
```

Run these inside the devcontainer (`docker exec <container> bash -lc 'cd /workspace/website && ...'`). CI uses `npm ci && npm run build`.

## Stack

Docusaurus 3.9.2 with React 19, TypeScript, MDX. Builds with Rspack and SWC (`future.experimental_faster`, `@docusaurus/faster`); CSS order can differ from a webpack build, so compare screenshots after changing global styles. Themes: `@docusaurus/theme-mermaid` (diagrams), `@easyops-cn/docusaurus-search-local` (search). Icons from `lucide-react`. Prism languages: bash, powershell, yaml.

## Structure

```
docs/                        ~100 Markdown pages, English only
  intro.md                   /docs/ landing page
  getting-started/           Install → first sync
  learn/                     Quickstarts by scenario
  how-to/                    daily-tasks/, sharing/, advanced/, recipes/
  understand/                Concepts, design philosophy
  reference/
    commands/                One page per CLI command, plus index
    targets/                 Supported targets, target config
    appendix/                URL formats, env vars, file structure
    filtering.md
  troubleshooting/           Errors, FAQ, Windows
blog/                        Blog posts (enabled, /blog)
src/
  pages/index.tsx            Homepage: product demo video, desktop installation,
                             four-moves diagram, feature-map teaser, CTA
  pages/index.module.css     Homepage styles (hand-drawn tokens from custom.css)
  pages/features.tsx         /features — Feature Map: all commands grouped by job, live filter
  pages/changelog.md         /changelog, updated by hand at release time alongside CHANGELOG.md
  data/featureMap.ts         Command groups shared by the homepage teaser and /features
  components/                AsciinemaPlayer
  css/custom.css             Design system (tokens, typography, dark/light)
static/img/                  Screenshots, logo, video poster, social card
static/video/                Homepage demo MP4 and captions
plugins/llms-txt.ts          Local plugin: writes build/llms.txt and llms-full.txt
```

## Key Config

- `docusaurus.config.ts` — Site config, navbar (Desktop App, Learn, How-To, Reference, Feature map, Blog, Changelog), footer, redirects from old `/docs/commands/*` paths
- `sidebars.ts` — Learn / How-To / Understand / Reference / Troubleshooting, with nested command subcategories
- `plugins/llms-txt.ts` — On the English build, writes `/llms.txt` (sidebar-ordered link index with first-sentence descriptions) and `/llms-full.txt` (all sidebar docs concatenated). Docs missing from `sidebars.ts` are left out of both
- `onBrokenLinks: 'throw'` — a bad link fails the build
- Color mode: default **light**, `respectPrefersColorScheme: false`
- Mermaid config lives in `themeConfig.mermaid`; no per-diagram `%%{init}%%`

## Docs Conventions

- Each doc has YAML frontmatter with `sidebar_position` for ordering
- Command docs follow: description, usage, flags table, examples
- Cross-reference with relative markdown links: `[sync](../commands/sync.md)`
- Before documenting a flag, grep `cmd/skillshare/` to confirm it exists
- Screenshots go in `static/img/` named `<feature>-demo.png`
- Mermaid: use `<br/>` for line breaks in node labels, keep labels short

## Homepage / Feature Map Notes

- The hero places a floating framed logo beside the copy and a responsive MP4 below it; the logo stacks below the copy on mobile. The video has native controls and a poster; both floating motion and autoplay respect `prefers-reduced-motion`
- The four-moves board is laid out at a fixed design width (1168px) and scaled with a `ResizeObserver`; below 640px it becomes a list
- Interactive state is local React state only (no persistence)
- Counts (`COMMAND_COUNT`, `TARGET_COUNT`) live in `src/data/featureMap.ts`; update them when commands or targets change
- Every `href` in `featureMap.ts` must map to an existing page under `docs/` (the build's broken-link check covers them)

## Design System (custom.css)

- Hand-drawn "paper" look: dot-grid background, wobbly border radii (`--radius-wobbly*`), hard offset shadows (`--shadow-md`, `--shadow-lg`), post-it yellow highlights
- Palette tokens: `--color-paper`, `--color-pencil`, `--color-blue` (dark mode: amber), `--color-accent`, `--color-success`, `--color-danger`, `--color-postit`
- Fonts: Kalam (handwritten accents), Inter (headings), IBM Plex Sans (body), JetBrains Mono (code)
- Buttons: pill radius, `.button--primary` green, `.button--secondary` outlined
- Dark mode is a warm parchment palette; new components should use the tokens so they adapt automatically

## Deployment

Static site at `https://skillshare.runkids.cc`, built and deployed to Cloudflare Pages by `.github/workflows/website-pages.yml` from the release tag after **Publish Release** publishes it. Pushes to `main` do not deploy; dispatch **Website Pages** with a tag or branch to redeploy or ship a docs-only fix.

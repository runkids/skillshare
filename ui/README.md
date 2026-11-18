# Dashboard development

From `ui/` inside the devcontainer:

- `pnpm lint` checks TypeScript and React code with Oxlint.
- `pnpm lint:typed` runs the same configuration with type information from tsrs. CI runs this check and `pnpm format:check` for the dashboard.
- `pnpm format` applies Oxfmt; `pnpm format:check` checks formatting without writing.
- `pnpm typecheck` checks project references with tsrs. `pnpm build` runs it before Vite.
- `pnpm test` runs the dashboard tests with Vitest.

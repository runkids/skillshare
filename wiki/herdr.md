# Herdr Integration

Use when changing the native QuickJS panel, its Mod or Layout gallery package, or integration with a user's existing window. This is not a React frontend; the topic also loads `herdr/AGENTS.md` for its implementation rules.

Load `testing` before running package generation or Node tests in the devcontainer. Load `documentation` when changing public installation instructions, including the translated `learn/with-herdr.md` pages.

The complete Layout and standalone Mod share the canonical implementation in `herdr/skillshare/mods/skillshare/`. `herdr/skillshare/README.md` describes both installation paths, mod-only updates, gallery discovery and how to regenerate the standalone package. Each share folder has its own agent-facing recipe; keep installation and update boundaries consistent in both.

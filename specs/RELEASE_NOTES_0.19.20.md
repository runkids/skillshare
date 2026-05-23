# skillshare v0.19.20 Release Notes

Release date: 2026-05-24

## TL;DR

1. **Run the dashboard in the background** — `skillshare ui start` / `skillshare ui stop` give you a managed background server with remembered host and port
2. **Update without leaving the browser** — the Update dialog and the Doctor page's Version card now upgrade the CLI and reload the page for you
3. **Install the dashboard as a desktop app** — Skillshare is now a Progressive Web App with its own icon, and `--app` opens it in a chrome-less Chromium window
4. **Doctor Version card redesign** — clearer status, explicit version delta, no misleading Update button when nothing is pending

---

## Background Mode for the Web Dashboard

`skillshare ui` no longer has to hold a foreground shell. The new `start` and `stop` subcommands run the server as a managed background process, remember the host and port you used, and reuse the existing process when it is still healthy.

```bash
skillshare ui start                 # start in background, return to shell
skillshare ui start --clear-cache   # clear cached UI assets, then start
skillshare ui stop                  # stop the background server
```

The legacy foreground form (`skillshare ui`) and the older `--no-open &` shell-backgrounding workaround keep working — nothing changes if you don't use the new subcommands.

For a more app-like feel, `skillshare ui start --app` opens the dashboard in a Chromium app-mode window (Chrome, Edge, or Brave) on macOS, Windows, and Linux, giving it a separate Dock or taskbar entry without browser chrome.

## In-place Upgrade from the Dashboard

When the dashboard detects a newer release, the Update dialog and the Doctor page's Version card both show an **Update now** button. Clicking it:

1. Runs `skillshare upgrade` on your machine
2. Restarts the local UI server
3. Reloads the page automatically once the new server is healthy

If the auto-reload doesn't complete (for example, the server moved to a different port), the dialog tells you to run `skillshare ui start` to bring the background server back up.

## Progressive Web App Support

The dashboard now ships a web manifest, app icons (192px and 512px), and a minimal service worker. Browsers that support PWAs (Chrome, Edge, Safari, Brave) let you install Skillshare as a standalone desktop app directly from the address bar — useful for pinning it to the Dock/taskbar and keeping it out of your tab strip.

## Doctor Page — Cleaner Version Card

The Version section on the Doctor page got a small redesign:

- A state-coloured icon on the left makes the current status obvious at a glance (blue when an update is available, green when up to date)
- The version delta is shown explicitly as `current → latest` instead of being separated by a thin dot
- The **Update now** button only appears when there is actually something to upgrade, so the card no longer suggests an update when you're on the latest version

No behavioural changes — just easier to read.

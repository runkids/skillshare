<p align="center" style="margin-bottom: 0;">
  <img src=".github/assets/skillshare-logo-card.png" alt="skillshare" width="280">
</p>

<h1 align="center" style="margin-top: 0.5rem; margin-bottom: 0.5rem;">skillshare</h1>

<p align="center">
  <a href="README.md">English</a> · <a href="README-de.md">Deutsch</a> · <a href="README-es.md">Español</a> · <a href="README-fr.md">Français</a> · <a href="README-ja.md">日本語</a> · <a href="README-ko.md">한국어</a> · <a href="README-pt-BR.md">Português</a> · <a href="README-zh-CN.md">简体中文</a> · <a href="README-zh-TW.md">繁體中文</a>
</p>

<p align="center">
  <a href="https://skillshare.runkids.cc"><img src="https://img.shields.io/badge/Website-skillshare.runkids.cc-blue?logo=docusaurus" alt="Website"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-yellow.svg" alt="License: MIT"></a>
  <a href="https://github.com/runkids/skillshare/releases"><img src="https://img.shields.io/github/v/release/runkids/skillshare" alt="Release"></a>
  <a href="https://github.com/runkids/skillshare/releases"><img src="https://img.shields.io/github/downloads/runkids/skillshare/total" alt="Downloads"></a>
  <img src="https://img.shields.io/badge/platform-macOS%20%7C%20Linux%20%7C%20Windows-blue" alt="Platform">
  <a href="https://deepwiki.com/runkids/skillshare"><img src="https://deepwiki.com/badge.svg" alt="Ask DeepWiki"></a>
</p>

<p align="center">
  <a href="https://github.com/runkids/skillshare/stargazers"><img src="https://img.shields.io/github/stars/runkids/skillshare?style=social" alt="Star on GitHub"></a>
</p>

<p align="center">
  <a href="https://trendshift.io/repositories/21835" target="_blank"><img src="https://trendshift.io/api/badge/repositories/21835" alt="runkids%2Fskillshare | Trendshift" style="width: 250px; height: 55px;" width="250" height="55"/></a>
  <a href="https://trendshift.io/repositories/21835?utm_source=trendshift-badge&amp;utm_medium=badge&amp;utm_campaign=badge-trendshift-21835" target="_blank" rel="noopener noreferrer"><img src="https://trendshift.io/api/badge/trendshift/repositories/21835/daily?language=Go" alt="runkids%2Fskillshare | Trendshift" width="250" height="55"/></a>
</p>

<p align="center">
  <strong>Deine KI-Coding-Umgebung, überall.</strong><br>
  Verwalte Skills, Agents, Rules, MCP-Verbindungen und Hooks an einem Ort.<br>
  Für Claude Code, Codex, Pi, OpenCode und mehr.
</p>

<p align="center">
  <a href="https://skillshare.runkids.cc">Website</a> •
  <a href="#installation">Installation</a> •
  <a href="#schnellstart">Schnellstart</a> •
  <a href="#highlights">Highlights</a> •
  <a href="#cli--und-ui-vorschau">Screenshots</a> •
  <a href="#desktop-app">Desktop-App</a> •
  <a href="https://skillshare.runkids.cc/docs">Dokumentation</a>
</p>

<p align="center">
  <img src=".github/assets/demo.gif" alt="skillshare demo" width="960">
</p>

> [!NOTE]
> **Neueste Version**: v0.28.0 — Tracked Repos, die pro Agent eine eigene Kopie eines Skills mitliefern (`skills/claude/<skill>`, `skills/codex/<skill>`), synchronisieren im Merge- und Copy-Modus jede Kopie nur noch zum passenden Agent, und Übersichten zählen sie als einen Skill. `skillshare plugin update --all` aktualisiert alle getrackten Plugins auf einmal, und `update` funktioniert wieder mit flachen Partial Clones. [Alle Releases →](https://github.com/runkids/skillshare/releases)

## Warum skillshare

Ein Wechsel des KI-Tools sollte nicht bedeuten, die ganze Umgebung neu aufzubauen.
skillshare gibt deinen Skills und anderen KI-Ressourcen einen Ort, den du selbst kontrollierst.

- **Tool wechseln, Skills behalten** — einmal bearbeiten, dann zu Claude Code, Codex, Pi und den anderen Tools synchronisieren, die du nutzt.
- **Deine Umgebung kommt mit** — versioniere deine Quelle mit Git und hole sie auf einen anderen Rechner.
- **Mit dem Team teilen** — halte Projektressourcen bei deinem Code und verteile gemeinsame Skills über Tracked Repositories.

Eine Person im Team nutzt Claude Code, eine andere Codex. Haltet eure gemeinsame Code-Review-Checkliste in `.skillshare/` beim Projekt. Neue Teammitglieder installieren die deklarierten Skills und synchronisieren sie zu den konfigurierten Tools, statt Anweisungen aus dem Chat zu kopieren. [Team-Onboarding →](https://skillshare.runkids.cc/docs/how-to/recipes/team-onboarding-recipe)

Mit der Desktop-App oder dem CLI verwaltest du alles lokal, [prüfst Skills vor der Nutzung](https://skillshare.runkids.cc/docs/reference/commands/audit) und [legst fest, was jedes Tool bekommt](https://skillshare.runkids.cc/docs/how-to/daily-tasks/filtering-skills).

> Du kommst von einem anderen Tool? [Migrationsleitfaden](https://skillshare.runkids.cc/docs/how-to/advanced/migration) · [Vergleich](https://skillshare.runkids.cc/docs/understand/philosophy/comparison)

## CLI- und UI-Vorschau

| Skill-Details | Sicherheits-Audit |
|---|---|
| <img src=".github/assets/skill-detail-tui.png" alt="Skill-Details" width="480" height="300"> | <img src=".github/assets/audit-tui.png" alt="Sicherheits-Audit" width="480" height="300"> |

| Web-Dashboard | Skills-Seite im Web |
|---|---|
| <img src=".github/assets/ui/web-dashboard-demo.png" alt="Web-Dashboard" width="480"> | <img src=".github/assets/ui/web-skills-demo.png" alt="Skills-Seite im Web" width="480"> |

## Installation

> [!TIP]
> **Nutze skillshare vom Desktop aus.** [Lade Skillshare App herunter](https://github.com/runkids/skillshare-app/releases/latest) für macOS (Apple Silicon), Windows und Linux. Beim ersten Start hilft die App, das CLI zu installieren oder zu finden, deine KI-Tools auszuwählen und die erste Synchronisierung auszuführen. [Installationsanleitung](https://skillshare.runkids.cc/docs/getting-started/desktop-app).

<a id="desktop-app"></a>

### Desktop-App — visuelle Einrichtung und tägliche Verwaltung

[Skillshare App](https://github.com/runkids/skillshare-app) bringt Skills, Agents, MCP und Hooks in ein Desktop-Fenster. Installiere die App, öffne sie und folge der Einrichtung beim ersten Start.

macOS (Apple Silicon), mit Homebrew:

```bash
brew tap runkids/tap
brew install --cask skillshare-app
```

**Windows / Linux oder manuelle Installation unter macOS:** [Lade die neuesten App-Installer herunter](https://github.com/runkids/skillshare-app/releases/latest). Details zu den einzelnen Plattformen findest du im [Leitfaden zur Desktop-App](https://skillshare.runkids.cc/docs/getting-started/desktop-app).

### CLI: macOS / Linux

```bash
curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh
```

Das Skript installiert standardmäßig nach `~/.local/bin`, normale Installationen und Updates brauchen also kein `sudo`. Wenn der Installer Anweisungen zur PATH-Einrichtung ausgibt, folge ihnen, bevor du `skillshare` ausführst. Füge die vorgeschlagene Zeile deiner Shell-Konfiguration hinzu (etwa `~/.zshrc` oder `~/.bashrc`), damit sie auch in neuen Terminals gilt. Mit `INSTALL_DIR` wählst du einen anderen Ort.

### Windows PowerShell

```powershell
irm https://raw.githubusercontent.com/runkids/skillshare/main/install.ps1 | iex
```

### CLI: Homebrew

```bash
brew install skillshare
```

> **Tipp:** Mit `skillshare upgrade` aktualisierst du auf die neueste Version. Der Befehl erkennt deine Installationsmethode und erledigt den Rest.

### CLI: Nix

```bash
nix profile add github:runkids/skillshare
```

Aktualisiere mit `nix profile upgrade skillshare`. Erfordert Nix 2.30+ mit aktivierten Flakes; mit älterem Nix oder Lix verwende `nix profile install`. NixOS- und Home-Manager-Konfigurationen können das `overlays.default` des Flakes verwenden.

### GitHub Actions

```yaml
- uses: runkids/setup-skillshare@v1
  with:
    source: ./skills
- run: skillshare sync
```

Alle Optionen (Audit, Projektmodus, feste Version) findest du unter [`setup-skillshare`](https://github.com/marketplace/actions/setup-skillshare).

### Kurzbefehl (optional)

Füge deiner Shell-Konfiguration (`~/.zshrc` oder `~/.bashrc`) einen Alias hinzu:

```bash
alias ss='skillshare'
```

## Schnellstart

```bash
skillshare init            # Erstellt Config, Quelle und erkannte Targets
skillshare sync            # Synchronisiert Skills zu allen Targets
```

## So funktioniert es

- macOS / Linux: `~/.config/skillshare/`
- Windows: `%AppData%\skillshare\`

```
┌─────────────────────────────────────────────────────────────┐
│                    Source Directory                         │
│   ~/.config/skillshare/skills/    ← skills (SKILL.md)       │
│   ~/.config/skillshare/agents/    ← agents                  │
│   ~/.config/skillshare/extras/    ← rules, commands, etc.   │
└─────────────────────────────────────────────────────────────┘
                              │ sync
              ┌───────────────┼───────────────┐
              ▼               ▼               ▼
       ┌───────────┐   ┌───────────┐   ┌───────────┐
       │  Claude   │   │  OpenCode │   │ OpenClaw  │   ...
       └───────────┘   └───────────┘   └───────────┘
```

| Plattform | Skills-Quelle | Agents-Quelle | Extras-Quelle | Link-Typ |
|----------|---------------|---------------|---------------|-----------|
| macOS/Linux | `~/.config/skillshare/skills/` | `~/.config/skillshare/agents/` | `~/.config/skillshare/extras/` | Symlinks |
| Windows | `%AppData%\skillshare\skills\` | `%AppData%\skillshare\agents\` | `%AppData%\skillshare\extras\` | NTFS-Junctions für Ordner (keine Administratorrechte nötig); Datei-Symlinks brauchen den Developer Mode, sonst wird kopiert |

| | Imperativ (Installation pro Befehl) | Deklarativ (skillshare) |
|---|---|---|
| **Single Source of Truth** | Skills unabhängig kopiert | Eine Quelle → Symlinks (oder Kopien) |
| **Neuer Rechner** | Jede Installation von Hand wiederholen | Config per `git clone` + `sync` |
| **Sicherheits-Audit** | Keines | Eingebautes `audit` + automatischer Scan bei Installation und Update |
| **Web-Dashboard** | Keines | `skillshare ui` |
| **Laufzeitabhängigkeit** | Node.js + npm | Keine (eine einzige Go-Binary) |

> [Vollständiger Vergleich →](https://skillshare.runkids.cc/docs/understand/philosophy/comparison)

## Highlights

**Skills installieren und aktualisieren** — von GitHub, GitLab oder jedem Git-Host

```bash
skillshare install github.com/reponame/skills
skillshare update --all
skillshare target claude --mode copy  # wenn Symlinks nicht funktionieren
```

**Probleme mit Symlinks?** — wechsle pro Target in den Copy-Modus

```bash
skillshare target <name> --mode copy
skillshare sync
```

**Sicherheits-Audit** — prüfe Skills, bevor sie deinen Agent erreichen

```bash
skillshare audit
```

**Projekt-Skills** — pro Repository, gemeinsam mit deinem Code committet

```bash
skillshare init -p && skillshare sync
```

**Agents** — synchronisiere eigene Agents zu Targets, die Agents unterstützen

```bash
skillshare sync agents            # nur Agents synchronisieren
skillshare sync --all             # Skills + Agents + Extras + MCP + Hooks zusammen synchronisieren
```

**Extras** — verwalte Rules, Commands, Prompts und mehr

```bash
skillshare extras init rules          # ein Extra namens "rules" anlegen
skillshare sync --all                 # Skills + Agents + Extras + MCP + Hooks zusammen synchronisieren
skillshare extras collect rules       # lokale Dateien in die Quelle zurückholen
```

**MCP-Verbindungen** — einmal einrichten für Claude Code, Codex, Pi, VS Code, OpenCode und mehr

```bash
skillshare mcp add                    # geführte Einrichtung per URL oder JSON
skillshare sync mcp --dry-run         # Änderungen an den nativen Konfigurationen vorab ansehen
skillshare sync mcp                   # Verbindungseinstellungen anwenden
```

Halte die Definitionen in `config.yaml` oder verweise auf eine separate `mcp.yaml`.
Beispiele, Verweise auf Umgebungsvariablen und den Import bestehender Verbindungen findest du unter
[MCP einrichten](https://skillshare.runkids.cc/docs/how-to/daily-tasks/sharing-mcp).

Native Hooks verwalten, ohne sie auszuführen:

```bash
skillshare hooks add check --file ./check.yaml
skillshare hooks sync --dry-run
skillshare hooks sync
```

**Plugins** — installiere ein komplettes Plugin und wähle, welche Tools es bekommen

```bash
skillshare plugin add                 # geführt: Quelle, Plugin, Targets, Prüfung
skillshare plugin add owner/repo --target claude --target codex --no-tui
skillshare sync plugins --dry-run     # Plugins werden getrennt von sync --all synchronisiert
```

Bestehende native Installationen lassen sich mit `plugin import` übernehmen.
Siehe [Plugins über Tools hinweg verwalten](https://skillshare.runkids.cc/docs/how-to/daily-tasks/sharing-plugins).

**Shell-Vervollständigung** — Befehle, Flags und Unterbefehle per Tab vervollständigen

```bash
skillshare completion bash --install   # auch: zsh, fish, powershell, nushell
```

**Lokale Checkpoints** — Änderungen an der Quelle committen, ohne zu pushen

```bash
skillshare commit -m "Update review skill"
skillshare commit --dry-run
```

**Web-Dashboard** — ein visuelles Kontrollzentrum

```bash
skillshare ui
```

[Alle Befehle und Anleitungen →](https://skillshare.runkids.cc/docs/reference/commands)

## Mitwirken

Beiträge sind willkommen! Eröffne zuerst ein Issue und reiche dann einen Draft-PR mit Tests ein.
Details zur Einrichtung stehen in [CONTRIBUTING.md](CONTRIBUTING.md).

```bash
git clone https://github.com/runkids/skillshare.git && cd skillshare
make check  # format + lint + test
```

> [!TIP]
> Du weißt nicht, wo du anfangen sollst? Sieh dir die [offenen Issues](https://github.com/runkids/skillshare/issues) an oder probiere den [Playground](https://skillshare.runkids.cc/docs/learn/with-playground) aus, eine Entwicklungsumgebung ohne Einrichtung.

## Mitwirkende

Danke an alle, die skillshare mitgestaltet haben. Die Liste findest du in der [englischen README](README.md#contributors).

---

Wenn dir skillshare hilft, freuen wir uns über einen ⭐

## Star History

<a href="https://star-history.dera.page/runkids/skillshare">
 <picture>
   <source media="(prefers-color-scheme: dark)" srcset="https://star-history.dera.page/svg?repos=runkids/skillshare&theme=dark" />
   <source media="(prefers-color-scheme: light)" srcset="https://star-history.dera.page/svg?repos=runkids/skillshare" />
   <img alt="Star History Chart" src="https://star-history.dera.page/svg?repos=runkids/skillshare" />
 </picture>
</a>

---

## Lizenz

MIT

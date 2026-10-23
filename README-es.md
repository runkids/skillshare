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
  <strong>Tu entorno de programación con IA, en todas partes.</strong><br>
  Gestiona skills, agents, rules, conexiones MCP y hooks en un solo lugar.<br>
  Para Claude Code, Codex, Pi, OpenCode y más.
</p>

<p align="center">
  <a href="https://skillshare.runkids.cc">Sitio web</a> •
  <a href="#instalación">Instalación</a> •
  <a href="#inicio-rápido">Inicio rápido</a> •
  <a href="#funciones-destacadas">Funciones destacadas</a> •
  <a href="#vista-previa-del-cli-y-la-ui">Capturas</a> •
  <a href="#desktop-app">App de escritorio</a> •
  <a href="https://skillshare.runkids.cc/docs">Documentación</a>
</p>

<p align="center">
  <img src=".github/assets/demo.gif" alt="skillshare demo" width="960">
</p>

> [!NOTE]
> **Última versión**: v0.26.3 — Añade cuentas separadas de OpenCode con `OPENCODE_CONFIG_DIR`. Esta versión también actualiza dependencias de Go con correcciones de seguridad y dirige los conflictos de carpetas con targets de proyecto en el dashboard al editor del proyecto. [Todas las versiones →](https://github.com/runkids/skillshare/releases)

## Por qué skillshare

Cambiar de herramienta de IA no debería obligarte a reconstruir tu entorno.
skillshare da a tus skills y demás recursos de IA un lugar que tú controlas.

- **Cambia de herramienta, conserva tus skills** — edita una vez y sincroniza con Claude Code, Codex, Pi y las demás herramientas que uses.
- **Lleva tu entorno contigo** — versiona tu fuente con Git y tráela a otra máquina.
- **Comparte con tu equipo** — guarda los recursos del proyecto junto a tu código y distribuye skills compartidos mediante tracked repositories.

Una persona del equipo usa Claude Code y otra, Codex. Guardad vuestra checklist común de revisión de código en `.skillshare/`, junto al proyecto. Quien se incorpora instala los skills declarados y los sincroniza con las herramientas configuradas, en lugar de copiar instrucciones del chat. [Incorporación de equipos →](https://skillshare.runkids.cc/docs/how-to/recipes/team-onboarding-recipe)

Usa la app de escritorio o el CLI para gestionarlo todo en local, [auditar los skills antes de usarlos](https://skillshare.runkids.cc/docs/reference/commands/audit) y [elegir qué recibe cada herramienta](https://skillshare.runkids.cc/docs/how-to/daily-tasks/filtering-skills).

> ¿Vienes de otra herramienta? [Guía de migración](https://skillshare.runkids.cc/docs/how-to/advanced/migration) · [Comparación](https://skillshare.runkids.cc/docs/understand/philosophy/comparison)

## Vista previa del CLI y la UI

| Detalle de un skill | Auditoría de seguridad |
|---|---|
| <img src=".github/assets/skill-detail-tui.png" alt="Detalle de un skill" width="480" height="300"> | <img src=".github/assets/audit-tui.png" alt="Auditoría de seguridad" width="480" height="300"> |

| Panel web | Página Skills en la web |
|---|---|
| <img src=".github/assets/ui/web-dashboard-demo.png" alt="Panel web" width="480"> | <img src=".github/assets/ui/web-skills-demo.png" alt="Página Skills en la web" width="480"> |

## Instalación

> [!TIP]
> **Usa skillshare desde tu escritorio.** [Descarga Skillshare App](https://github.com/runkids/skillshare-app/releases/latest) para macOS (Apple Silicon), Windows y Linux. En el primer inicio te ayuda a instalar o localizar el CLI, elegir tus herramientas de IA y hacer tu primera sincronización. [Guía de instalación](https://skillshare.runkids.cc/docs/getting-started/desktop-app).

<a id="desktop-app"></a>

### App de escritorio — configuración visual y gestión diaria

[Skillshare App](https://github.com/runkids/skillshare-app) reúne skills, agents, MCP y hooks en una ventana de escritorio. Instala la app, ábrela y sigue la configuración del primer inicio.

macOS (Apple Silicon), con Homebrew:

```bash
brew tap runkids/tap
brew install --cask skillshare-app
```

**Windows / Linux, o instalación manual en macOS:** [Descarga los instaladores más recientes](https://github.com/runkids/skillshare-app/releases/latest). Consulta la [guía de la app de escritorio](https://skillshare.runkids.cc/docs/getting-started/desktop-app) para los detalles de cada plataforma.

### CLI: macOS / Linux

```bash
curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh
```

El script instala por defecto en `~/.local/bin`, así que las instalaciones y actualizaciones normales no necesitan `sudo`. Si el instalador muestra instrucciones para configurar el PATH, síguelas antes de ejecutar `skillshare`. Añade la línea sugerida a la configuración de tu shell (como `~/.zshrc` o `~/.bashrc`) para las próximas terminales. Define `INSTALL_DIR` para usar otra ubicación.

### Windows PowerShell

```powershell
irm https://raw.githubusercontent.com/runkids/skillshare/main/install.ps1 | iex
```

### CLI: Homebrew

```bash
brew install skillshare
```

> **Consejo:** ejecuta `skillshare upgrade` para actualizar a la última versión. Detecta tu método de instalación y se encarga del resto.

### GitHub Actions

```yaml
- uses: runkids/setup-skillshare@v1
  with:
    source: ./skills
- run: skillshare sync
```

Consulta [`setup-skillshare`](https://github.com/marketplace/actions/setup-skillshare) para ver todas las opciones (audit, modo proyecto, versión fija).

### Atajo (opcional)

Añade un alias a la configuración de tu shell (`~/.zshrc` o `~/.bashrc`):

```bash
alias ss='skillshare'
```

## Inicio rápido

```bash
skillshare init            # Crea la config, la fuente y los targets detectados
skillshare sync            # Sincroniza los skills con todos los targets
```

## Cómo funciona

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

| Plataforma | Fuente de skills | Fuente de agents | Fuente de extras | Tipo de enlace |
|----------|---------------|---------------|---------------|-----------|
| macOS/Linux | `~/.config/skillshare/skills/` | `~/.config/skillshare/agents/` | `~/.config/skillshare/extras/` | Symlinks |
| Windows | `%AppData%\skillshare\skills\` | `%AppData%\skillshare\agents\` | `%AppData%\skillshare\extras\` | Junctions NTFS para carpetas (sin permisos de administrador); los symlinks de archivos requieren el Developer Mode, si no se copian |

| | Imperativo (una instalación por comando) | Declarativo (skillshare) |
|---|---|---|
| **Fuente única** | Skills copiados por separado | Una sola fuente → symlinks (o copias) |
| **Máquina nueva** | Repetir cada instalación a mano | `git clone` de la config + `sync` |
| **Auditoría de seguridad** | Ninguna | `audit` integrado + análisis automático al instalar y actualizar |
| **Panel web** | Ninguno | `skillshare ui` |
| **Dependencia en tiempo de ejecución** | Node.js + npm | Ninguna (un único binario de Go) |

> [Comparación completa →](https://skillshare.runkids.cc/docs/understand/philosophy/comparison)

## Funciones destacadas

**Instalar y actualizar skills** — desde GitHub, GitLab o cualquier host Git

```bash
skillshare install github.com/reponame/skills
skillshare update --all
skillshare target claude --mode copy  # si los symlinks no funcionan
```

**¿Problemas con los symlinks?** — cambia a modo copy por target

```bash
skillshare target <name> --mode copy
skillshare sync
```

**Auditoría de seguridad** — analiza los skills antes de que lleguen a tu agent

```bash
skillshare audit
```

**Skills de proyecto** — por repositorio, con commit junto a tu código

```bash
skillshare init -p && skillshare sync
```

**Agents** — sincroniza agents personalizados con los targets que los admiten

```bash
skillshare sync agents            # sincroniza solo los agents
skillshare sync --all             # sincroniza skills + agents + extras + MCP + hooks a la vez
```

**Extras** — gestiona rules, commands, prompts y más

```bash
skillshare extras init rules          # crea un extra "rules"
skillshare sync --all                 # sincroniza skills + agents + extras + MCP + hooks a la vez
skillshare extras collect rules       # recupera los archivos locales en la fuente
```

**Conexiones MCP** — configura una vez para Claude Code, Codex, Pi, VS Code, OpenCode y más

```bash
skillshare mcp add                    # configuración guiada por URL o JSON
skillshare sync mcp --dry-run         # vista previa de los cambios en las configuraciones nativas
skillshare sync mcp                   # aplica la configuración de conexión
```

Guarda las definiciones en `config.yaml` o haz referencia a un `mcp.yaml` aparte.
Consulta [la configuración de MCP](https://skillshare.runkids.cc/docs/how-to/daily-tasks/sharing-mcp)
para ver ejemplos, referencias a variables de entorno y la importación de conexiones existentes.

Gestiona hooks nativos sin ejecutarlos:

```bash
skillshare hooks add check --file ./check.yaml
skillshare hooks sync --dry-run
skillshare hooks sync
```

**Plugins** — instala un plugin completo y elige qué herramientas lo reciben

```bash
skillshare plugin add                 # guiado: fuente, plugin, targets, revisión
skillshare plugin add owner/repo --target claude --target codex --no-tui
skillshare sync plugins --dry-run     # los plugins se sincronizan aparte de sync --all
```

Las instalaciones nativas existentes se pueden adoptar con `plugin import`.
Consulta [Gestionar plugins entre herramientas](https://skillshare.runkids.cc/docs/how-to/daily-tasks/sharing-plugins).

**Autocompletado del shell** — completa comandos, flags y subcomandos con Tab

```bash
skillshare completion bash --install   # también: zsh, fish, powershell, nushell
```

**Puntos de control locales** — haz commit de los cambios de la fuente sin hacer push

```bash
skillshare commit -m "Update review skill"
skillshare commit --dry-run
```

**Panel web** — un panel de control visual

```bash
skillshare ui
```

[Todos los comandos y guías →](https://skillshare.runkids.cc/docs/reference/commands)

## Contribuir

¡Las contribuciones son bienvenidas! Abre primero un issue y luego envía una draft PR con tests.
Consulta [CONTRIBUTING.md](CONTRIBUTING.md) para la configuración.

```bash
git clone https://github.com/runkids/skillshare.git && cd skillshare
make check  # format + lint + test
```

> [!TIP]
> ¿No sabes por dónde empezar? Revisa los [issues abiertos](https://github.com/runkids/skillshare/issues) o prueba el [Playground](https://skillshare.runkids.cc/docs/learn/with-playground), un entorno de desarrollo sin configuración.

## Colaboradores

Gracias a todas las personas que han ayudado a dar forma a skillshare. La lista está en el [README en inglés](README.md#contributors).

---

Si skillshare te resulta útil, regálale una ⭐

## Star History

<a href="https://star-history.dera.page/runkids/skillshare">
 <picture>
   <source media="(prefers-color-scheme: dark)" srcset="https://star-history.dera.page/svg?repos=runkids/skillshare&theme=dark" />
   <source media="(prefers-color-scheme: light)" srcset="https://star-history.dera.page/svg?repos=runkids/skillshare" />
   <img alt="Star History Chart" src="https://star-history.dera.page/svg?repos=runkids/skillshare" />
 </picture>
</a>

---

## Licencia

MIT

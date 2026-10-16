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
  <strong>Seu ambiente de programação com IA, em qualquer lugar.</strong><br>
  Gerencie skills, agents, rules, conexões MCP e hooks em um só lugar.<br>
  Para Claude Code, Codex, Pi, OpenCode e muito mais.
</p>

<p align="center">
  <a href="https://skillshare.runkids.cc">Site</a> •
  <a href="#instalação">Instalação</a> •
  <a href="#início-rápido">Início rápido</a> •
  <a href="#destaques">Destaques</a> •
  <a href="#prévia-do-cli-e-da-ui">Capturas de tela</a> •
  <a href="#desktop-app">App para desktop</a> •
  <a href="https://skillshare.runkids.cc/docs">Documentação</a>
</p>

<p align="center">
  <img src=".github/assets/demo.gif" alt="skillshare demo" width="960">
</p>

> [!NOTE]
> **Versão mais recente**: v0.25.13 — `mcp serve` agora pode editar os skills que você lista em `mcp.serve.edit.skills`: o cliente pré-visualiza o diff completo e o resultado da auditoria e depois salva exatamente essa prévia; se o arquivo mudou nesse meio-tempo, a edição é recusada. As ferramentas `list_skills` e `read_skill` agora são marcadas como somente leitura, então clientes que perguntam antes de cada chamada de ferramenta deixam de perguntar por elas. [Todas as versões →](https://github.com/runkids/skillshare/releases)

## Por que skillshare

Trocar de ferramenta de IA não deveria significar reconstruir todo o seu ambiente.
O skillshare dá às suas skills e a outros recursos de IA um lugar que você controla.

- **Troque de ferramenta, mantenha suas skills** — edite uma vez e sincronize com Claude Code, Codex, Pi e as outras ferramentas que você usa.
- **Leve seu ambiente com você** — versione sua fonte no Git e traga-a para outra máquina.
- **Compartilhe com seu time** — mantenha os recursos do projeto junto ao código e distribua skills compartilhadas por meio de tracked repositories.

Uma pessoa do time usa Claude Code, outra usa Codex. Mantenham o checklist de code review em comum em `.skillshare/`, junto com o projeto. Quem chega ao time instala as skills declaradas e sincroniza com as ferramentas configuradas, em vez de copiar instruções do chat. [Onboarding do time →](https://skillshare.runkids.cc/docs/how-to/recipes/team-onboarding-recipe)

Use o app para desktop ou o CLI para gerenciar tudo localmente, [auditar skills antes de usá-las](https://skillshare.runkids.cc/docs/reference/commands/audit) e [escolher o que cada ferramenta recebe](https://skillshare.runkids.cc/docs/how-to/daily-tasks/filtering-skills).

> Vindo de outra ferramenta? [Guia de migração](https://skillshare.runkids.cc/docs/how-to/advanced/migration) · [Comparação](https://skillshare.runkids.cc/docs/understand/philosophy/comparison)

## Prévia do CLI e da UI

| Detalhes de uma skill | Auditoria de segurança |
|---|---|
| <img src=".github/assets/skill-detail-tui.png" alt="Detalhes de uma skill" width="480" height="300"> | <img src=".github/assets/audit-tui.png" alt="Auditoria de segurança" width="480" height="300"> |

| Painel web | Página Skills na web |
|---|---|
| <img src=".github/assets/ui/web-dashboard-demo.png" alt="Painel web" width="480"> | <img src=".github/assets/ui/web-skills-demo.png" alt="Página Skills na web" width="480"> |

## Instalação

> [!TIP]
> **Use o skillshare pelo desktop.** [Baixe o Skillshare App](https://github.com/runkids/skillshare-app/releases/latest) para macOS (Apple Silicon), Windows e Linux. Na primeira abertura, ele ajuda a instalar ou localizar o CLI, escolher suas ferramentas de IA e fazer a primeira sincronização. [Guia de instalação](https://skillshare.runkids.cc/docs/getting-started/desktop-app).

<a id="desktop-app"></a>

### App para desktop — configuração visual e gerenciamento no dia a dia

O [Skillshare App](https://github.com/runkids/skillshare-app) reúne skills, agents, MCP e hooks em uma janela de desktop. Instale o app, abra-o e siga a configuração da primeira abertura.

macOS (Apple Silicon), com Homebrew:

```bash
brew tap runkids/tap
brew install --cask skillshare-app
```

**Windows / Linux, ou instalação manual no macOS:** [baixe os instaladores mais recentes](https://github.com/runkids/skillshare-app/releases/latest). Veja o [guia do app para desktop](https://skillshare.runkids.cc/docs/getting-started/desktop-app) para detalhes de cada plataforma.

### CLI: macOS / Linux

```bash
curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh
```

Por padrão, o script instala em `~/.local/bin`, então instalações e atualizações normais não precisam de `sudo`. Se o instalador mostrar instruções para configurar o PATH, siga-as antes de executar `skillshare`. Adicione a linha sugerida à configuração do seu shell (como `~/.zshrc` ou `~/.bashrc`) para os próximos terminais. Defina `INSTALL_DIR` para usar outro local.

### Windows PowerShell

```powershell
irm https://raw.githubusercontent.com/runkids/skillshare/main/install.ps1 | iex
```

### CLI: Homebrew

```bash
brew install skillshare
```

> **Dica:** execute `skillshare upgrade` para atualizar para a versão mais recente. Ele detecta seu método de instalação e cuida do resto.

### GitHub Actions

```yaml
- uses: runkids/setup-skillshare@v1
  with:
    source: ./skills
- run: skillshare sync
```

Veja [`setup-skillshare`](https://github.com/marketplace/actions/setup-skillshare) para todas as opções (audit, modo projeto, versão fixa).

### Atalho (opcional)

Adicione um alias à configuração do seu shell (`~/.zshrc` ou `~/.bashrc`):

```bash
alias ss='skillshare'
```

## Início rápido

```bash
skillshare init            # Cria a config, a fonte e os targets detectados
skillshare sync            # Sincroniza as skills com todos os targets
```

## Como funciona

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

| Plataforma | Fonte das skills | Fonte dos agents | Fonte dos extras | Tipo de link |
|----------|---------------|---------------|---------------|-----------|
| macOS/Linux | `~/.config/skillshare/skills/` | `~/.config/skillshare/agents/` | `~/.config/skillshare/extras/` | Symlinks |
| Windows | `%AppData%\skillshare\skills\` | `%AppData%\skillshare\agents\` | `%AppData%\skillshare\extras\` | Junctions NTFS para pastas (sem precisar de administrador); symlinks de arquivos exigem o Developer Mode, senão são copiados |

| | Imperativo (uma instalação por comando) | Declarativo (skillshare) |
|---|---|---|
| **Fonte da verdade** | Skills copiadas separadamente | Uma única fonte → symlinks (ou cópias) |
| **Máquina nova** | Repetir cada instalação manualmente | `git clone` da config + `sync` |
| **Auditoria de segurança** | Nenhuma | `audit` integrado + verificação automática ao instalar e atualizar |
| **Painel web** | Nenhum | `skillshare ui` |
| **Dependência em tempo de execução** | Node.js + npm | Nenhuma (um único binário Go) |

> [Comparação completa →](https://skillshare.runkids.cc/docs/understand/philosophy/comparison)

## Destaques

**Instale e atualize skills** — do GitHub, GitLab ou de qualquer host Git

```bash
skillshare install github.com/reponame/skills
skillshare update --all
skillshare target claude --mode copy  # se os symlinks não funcionarem
```

**Problemas com symlinks?** — mude para o modo copy por target

```bash
skillshare target <name> --mode copy
skillshare sync
```

**Auditoria de segurança** — verifique as skills antes que cheguem ao seu agent

```bash
skillshare audit
```

**Skills de projeto** — por repositório, com commit junto ao seu código

```bash
skillshare init -p && skillshare sync
```

**Agents** — sincronize agents personalizados com os targets que os suportam

```bash
skillshare sync agents            # sincroniza só os agents
skillshare sync --all             # sincroniza skills + agents + extras + MCP + hooks juntos
```

**Extras** — gerencie rules, commands, prompts e mais

```bash
skillshare extras init rules          # cria um extra "rules"
skillshare sync --all                 # sincroniza skills + agents + extras + MCP + hooks juntos
skillshare extras collect rules       # traz os arquivos locais de volta para a fonte
```

**Conexões MCP** — configure uma vez para Claude Code, Codex, Pi, VS Code, OpenCode e mais

```bash
skillshare mcp add                    # configuração guiada por URL ou JSON
skillshare sync mcp --dry-run         # prévia das mudanças nas configurações nativas
skillshare sync mcp                   # aplica as configurações de conexão
```

Mantenha as definições em `config.yaml` ou referencie um `mcp.yaml` separado.
Veja [a configuração de MCP](https://skillshare.runkids.cc/docs/how-to/daily-tasks/sharing-mcp)
para exemplos, referências a variáveis de ambiente e a importação de conexões existentes.

Gerencie hooks nativos sem executá-los:

```bash
skillshare hooks add check --file ./check.yaml
skillshare hooks sync --dry-run
skillshare hooks sync
```

**Plugins** — instale um plugin completo e escolha quais ferramentas o recebem

```bash
skillshare plugin add                 # guiado: fonte, plugin, targets, revisão
skillshare plugin add owner/repo --target claude --target codex --no-tui
skillshare sync plugins --dry-run     # plugins são sincronizados à parte do sync --all
```

Instalações nativas existentes podem ser adotadas com `plugin import`.
Veja [Gerenciar plugins entre ferramentas](https://skillshare.runkids.cc/docs/how-to/daily-tasks/sharing-plugins).

**Autocompletar no shell** — complete comandos, flags e subcomandos com Tab

```bash
skillshare completion bash --install   # também: zsh, fish, powershell, nushell
```

**Checkpoints locais** — faça commit das mudanças da fonte sem fazer push

```bash
skillshare commit -m "Update review skill"
skillshare commit --dry-run
```

**Painel web** — um painel de controle visual

```bash
skillshare ui
```

[Todos os comandos e guias →](https://skillshare.runkids.cc/docs/reference/commands)

## Contribuindo

Contribuições são bem-vindas! Abra uma issue primeiro e depois envie uma draft PR com testes.
Veja [CONTRIBUTING.md](CONTRIBUTING.md) para os detalhes de configuração.

```bash
git clone https://github.com/runkids/skillshare.git && cd skillshare
make check  # format + lint + test
```

> [!TIP]
> Não sabe por onde começar? Veja as [issues abertas](https://github.com/runkids/skillshare/issues) ou experimente o [Playground](https://skillshare.runkids.cc/docs/learn/with-playground), um ambiente de desenvolvimento sem configuração.

## Colaboradores

Obrigado a todas as pessoas que ajudaram a construir o skillshare. A lista está no [README em inglês](README.md#contributors).

---

Se o skillshare for útil para você, deixe uma ⭐

## Star History

[![Star History Chart](https://star-history.dera.page/svg?repos=runkids/skillshare&type=date&legend=top-left)](https://star-history.dera.page/#runkids/skillshare&type=date&legend=top-left)

---

## Licença

MIT

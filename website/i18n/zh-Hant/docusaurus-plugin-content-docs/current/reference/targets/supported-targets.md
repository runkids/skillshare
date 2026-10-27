---
sidebar_position: 2
---

# 支援的 Targets

skillshare 開箱即用支援的 AI CLI 完整清單。

## 總覽

skillshare 提供 **79 個內建 targets**，包含共用的 Universal target。當你執行 `skillshare init` 時，它會自動偵測並設定任何已安裝的工具。

---

## 內建 Targets

以下這些會在 `skillshare init` 時自動偵測：

<div className="target-grid">
  <a className="target-badge" href="#target-adal">AdaL</a>
  <a className="target-badge" href="#target-universal">Universal</a>
  <a className="target-badge" href="#target-amp">Amp</a>
  <a className="target-badge" href="#target-antigravity">Antigravity</a>
  <a className="target-badge" href="#target-antigravity-cli">Antigravity CLI</a>
  <a className="target-badge" href="#target-aiderdesk">AiderDesk</a>
  <a className="target-badge" href="#target-astrbot">AstrBot</a>
  <a className="target-badge" href="#target-augment">Augment</a>
  <a className="target-badge" href="#target-autohand-code">Autohand Code</a>
  <a className="target-badge" href="#target-bob">Bob</a>
  <a className="target-badge" href="#target-claude">Claude</a>
  <a className="target-badge" href="#target-codearts">CodeArts</a>
  <a className="target-badge" href="#target-cline">Cline</a>
  <a className="target-badge" href="#target-codebuddy">CodeBuddy</a>
  <a className="target-badge" href="#target-codestudio">Code Studio</a>
  <a className="target-badge" href="#target-comate">COMATE</a>
  <a className="target-badge" href="#target-codex">Codex</a>
  <a className="target-badge" href="#target-commandcode">Cmd Code</a>
  <a className="target-badge" href="#target-continue">Continue</a>
  <a className="target-badge" href="#target-copilot">Copilot</a>
  <a className="target-badge" href="#target-cortex">Cortex</a>
  <a className="target-badge" href="#target-crush">Crush</a>
  <a className="target-badge" href="#target-cursor">Cursor</a>
  <a className="target-badge" href="#target-deepagents">Deep Agents</a>
  <a className="target-badge" href="#target-deepseek-harness">DeepSeek Harness</a>
  <a className="target-badge" href="#target-devin">Devin</a>
  <a className="target-badge" href="#target-dexto">Dexto</a>
  <a className="target-badge" href="#target-droid">Droid</a>
  <a className="target-badge" href="#target-firebender">Firebender</a>
  <a className="target-badge" href="#target-forgecode">ForgeCode</a>
  <a className="target-badge" href="#target-fx">fx</a>
  <a className="target-badge" href="#target-gemini">Gemini CLI</a>
  <a className="target-badge" href="#target-gitlab-duo">GitLab Duo</a>
  <a className="target-badge" href="#target-goose">Goose</a>
  <a className="target-badge" href="#target-grok">Grok</a>
  <a className="target-badge" href="#target-hermes">Hermes</a>
  <a className="target-badge" href="#target-iflow">iFlow</a>
  <a className="target-badge" href="#target-jazz">Jazz</a>
  <a className="target-badge" href="#target-junie">Junie</a>
  <a className="target-badge" href="#target-kilocode">Kilocode</a>
  <a className="target-badge" href="#target-kimchi">Kimchi</a>
  <a className="target-badge" href="#target-kimi">Kimi</a>
  <a className="target-badge" href="#target-kimi-code">Kimi Code</a>
  <a className="target-badge" href="#target-kiro">Kiro</a>
  <a className="target-badge" href="#target-kode">Kode</a>
  <a className="target-badge" href="#target-letta">Letta</a>
  <a className="target-badge" href="#target-lingma">Lingma</a>
  <a className="target-badge" href="#target-mcpjam">MCPJam</a>
  <a className="target-badge" href="#target-mux">Mux</a>
  <a className="target-badge" href="#target-neovate">Neovate</a>
  <a className="target-badge" href="#target-omp">oh-my-pi</a>
  <a className="target-badge" href="#target-ona">Ona</a>
  <a className="target-badge" href="#target-openclaw">OpenClaw</a>
  <a className="target-badge" href="#target-opencode">OpenCode</a>
  <a className="target-badge" href="#target-openhands">OpenHands</a>
  <a className="target-badge" href="#target-pi">Pi</a>
  <a className="target-badge" href="#target-pochi">Pochi</a>
  <a className="target-badge" href="#target-posit-assistant">Posit Assistant</a>
  <a className="target-badge" href="#target-purecode">Purecode AI</a>
  <a className="target-badge" href="#target-qoder">Qoder</a>
  <a className="target-badge" href="#target-qoder-cn">Qoder CN</a>
  <a className="target-badge" href="#target-qwen">Qwen</a>
  <a className="target-badge" href="#target-replit">Replit</a>
  <a className="target-badge" href="#target-reasonix">Reasonix</a>
  <a className="target-badge" href="#target-roo">Roo</a>
  <a className="target-badge" href="#target-rovodev">Rovo Dev</a>
  <a className="target-badge" href="#target-tabnine">Tabnine</a>
  <a className="target-badge" href="#target-trae">Trae</a>
  <a className="target-badge" href="#target-trae-cn">Trae CN</a>
  <a className="target-badge" href="#target-vibe">Vibe</a>
  <a className="target-badge" href="#target-verdent">Verdent</a>
  <a className="target-badge" href="#target-warp">Warp</a>
  <a className="target-badge" href="#target-windsurf">Windsurf</a>
  <a className="target-badge" href="#target-witsy">Witsy</a>
  <a className="target-badge" href="#target-xcode-claude">Xcode Claude</a>
  <a className="target-badge" href="#target-xcode-codex">Xcode Codex</a>
  <a className="target-badge" href="#target-zcode">ZCode</a>
  <a className="target-badge" href="#target-zed">Zed</a>
  <a className="target-badge" href="#target-zencoder">Zencoder</a>
</div>

---

## Target 路徑

<table>
<thead>
<tr><th>Target</th><th>Global 路徑</th><th>Project 路徑</th></tr>
</thead>
<tbody>
<tr id="target-adal"><td>adal</td><td><code>&#126;/.adal/skills</code></td><td><code>.adal/skills</code></td></tr>
<tr id="target-universal"><td>universal</td><td><code>&#126;/.agents/skills</code></td><td><code>.agents/skills</code></td></tr>
<tr id="target-amp"><td>amp</td><td><code>&#126;/.config/agents/skills</code></td><td><code>.agents/skills</code></td></tr>
<tr id="target-antigravity"><td>antigravity</td><td><code>&#126;/.gemini/config/skills</code></td><td><code>.agents/skills</code></td></tr>
<tr id="target-antigravity-cli"><td>antigravity-cli</td><td><code>&#126;/.gemini/antigravity-cli/skills</code></td><td><code>.agents/skills</code></td></tr>
<tr id="target-aiderdesk"><td>aiderdesk</td><td><code>&#126;/.aider-desk/skills</code></td><td><code>.aider-desk/skills</code></td></tr>
<tr id="target-astrbot"><td>astrbot</td><td><code>&#126;/.astrbot/data/skills</code></td><td><code>data/skills</code></td></tr>
<tr id="target-augment"><td>augment</td><td><code>&#126;/.augment/skills</code></td><td><code>.augment/skills</code></td></tr>
<tr id="target-autohand-code"><td>autohand-code</td><td><code>&#126;/.autohand/skills</code></td><td><code>.autohand/skills</code></td></tr>
<tr id="target-bob"><td>bob</td><td><code>&#126;/.bob/skills</code></td><td><code>.bob/skills</code></td></tr>
<tr id="target-claude"><td>claude</td><td><code>&#126;/.claude/skills</code></td><td><code>.claude/skills</code></td></tr>
<tr id="target-codearts"><td>codearts</td><td><code>&#126;/.codeartsdoer/skills</code></td><td><code>.codeartsdoer/skills</code></td></tr>
<tr id="target-cline"><td>cline</td><td><code>&#126;/.cline/skills</code></td><td><code>.cline/skills</code></td></tr>
<tr id="target-codebuddy"><td>codebuddy</td><td><code>&#126;/.codebuddy/skills</code></td><td><code>.codebuddy/skills</code></td></tr>
<tr id="target-codestudio"><td>codestudio</td><td><code>&#126;/.codestudio/skills</code></td><td><code>.codestudio/skills</code></td></tr>
<tr id="target-comate"><td>comate</td><td><code>&#126;/.comate/skills</code></td><td><code>.comate/skills</code></td></tr>
<tr id="target-codex"><td>codex</td><td><code>&#126;/.agents/skills</code></td><td><code>.agents/skills</code></td></tr>
<tr id="target-commandcode"><td>commandcode</td><td><code>&#126;/.commandcode/skills</code></td><td><code>.commandcode/skills</code></td></tr>
<tr id="target-continue"><td>continue</td><td><code>&#126;/.continue/skills</code></td><td><code>.continue/skills</code></td></tr>
<tr id="target-cortex"><td>cortex</td><td><code>&#126;/.snowflake/cortex/skills</code></td><td><code>.cortex/skills</code></td></tr>
<tr id="target-copilot"><td>copilot</td><td><code>&#126;/.copilot/skills</code></td><td><code>.github/skills</code></td></tr>
<tr id="target-crush"><td>crush</td><td><code>&#126;/.config/crush/skills</code></td><td><code>.crush/skills</code></td></tr>
<tr id="target-cursor"><td>cursor</td><td><code>&#126;/.cursor/skills</code></td><td><code>.agents/skills</code></td></tr>
<tr id="target-deepagents"><td>deepagents</td><td><code>&#126;/.deepagents/agent/skills</code></td><td><code>.deepagents/skills</code></td></tr>
<tr id="target-deepseek-harness"><td>deepseek-harness</td><td><code>&#126;/.dsh/skills</code></td><td><code>.dsh/skills</code></td></tr>
<tr id="target-devin"><td>devin</td><td><code>&#126;/.config/devin/skills</code></td><td><code>.devin/skills</code></td></tr>
<tr id="target-dexto"><td>dexto</td><td><code>&#126;/.agents/skills</code></td><td><code>.agents/skills</code></td></tr>
<tr id="target-droid"><td>droid</td><td><code>&#126;/.factory/skills</code></td><td><code>.factory/skills</code></td></tr>
<tr id="target-firebender"><td>firebender</td><td><code>&#126;/.firebender/skills</code></td><td><code>.firebender/skills</code></td></tr>
<tr id="target-forgecode"><td>forgecode</td><td><code>&#126;/forge/skills</code></td><td><code>.forge/skills</code></td></tr>
<tr id="target-fx"><td>fx</td><td><code>&#126;/.fx/skills</code></td><td><code>.fx/skills</code></td></tr>

<tr id="target-gemini"><td>gemini</td><td><code>&#126;/.gemini/skills</code></td><td><code>.gemini/skills</code></td></tr>
<tr id="target-gitlab-duo"><td>gitlab-duo</td><td><code>&#126;/.gitlab/duo/skills</code></td><td><code>skills</code></td></tr>
<tr id="target-goose"><td>goose</td><td><code>&#126;/.agents/skills</code></td><td><code>.agents/skills</code></td></tr>
<tr id="target-grok"><td>grok</td><td><code>&#126;/.grok/skills</code></td><td><code>.grok/skills</code></td></tr>
<tr id="target-hermes"><td>hermes</td><td><code>&#126;/.hermes/skills</code></td><td><code>.hermes/skills</code></td></tr>
<tr id="target-iflow"><td>iflow</td><td><code>&#126;/.iflow/skills</code></td><td><code>.iflow/skills</code></td></tr>
<tr id="target-jazz"><td>jazz</td><td><code>&#126;/.jazz/skills</code></td><td><code>skills</code></td></tr>
<tr id="target-junie"><td>junie</td><td><code>&#126;/.junie/skills</code></td><td><code>.junie/skills</code></td></tr>
<tr id="target-kilocode"><td>kilocode</td><td><code>&#126;/.kilo/skills</code></td><td><code>.kilo/skills</code></td></tr>
<tr id="target-kimchi"><td>kimchi</td><td><code>&#126;/.config/kimchi/harness/skills</code></td><td><code>.kimchi/skills</code></td></tr>
<tr id="target-kimi"><td>kimi</td><td><code>&#126;/.config/agents/skills</code></td><td><code>.agents/skills</code></td></tr>
<tr id="target-kimi-code"><td>kimi-code</td><td><code>&#126;/.kimi-code/skills</code></td><td><code>.kimi-code/skills</code></td></tr>
<tr id="target-kiro"><td>kiro</td><td><code>&#126;/.kiro/skills</code></td><td><code>.kiro/skills</code></td></tr>
<tr id="target-kode"><td>kode</td><td><code>&#126;/.kode/skills</code></td><td><code>.kode/skills</code></td></tr>
<tr id="target-letta"><td>letta</td><td><code>&#126;/.letta/skills</code></td><td><code>.skills</code></td></tr>
<tr id="target-lingma"><td>lingma</td><td><code>&#126;/.lingma/skills</code></td><td><code>.lingma/skills</code></td></tr>
<tr id="target-mcpjam"><td>mcpjam</td><td><code>&#126;/.mcpjam/skills</code></td><td><code>.mcpjam/skills</code></td></tr>
<tr id="target-mux"><td>mux</td><td><code>&#126;/.mux/skills</code></td><td><code>.mux/skills</code></td></tr>
<tr id="target-neovate"><td>neovate</td><td><code>&#126;/.neovate/skills</code></td><td><code>.neovate/skills</code></td></tr>
<tr id="target-omp"><td>omp</td><td><code>&#126;/.omp/agent/skills</code></td><td><code>.omp/skills</code></td></tr>
<tr id="target-ona"><td>ona</td><td>—</td><td><code>.ona/skills</code></td></tr>
<tr id="target-openclaw"><td>openclaw</td><td><code>&#126;/.openclaw/skills</code></td><td><code>skills</code></td></tr>
<tr id="target-opencode"><td>opencode</td><td><code>&#126;/.config/opencode/skills</code></td><td><code>.opencode/skills</code></td></tr>
<tr id="target-openhands"><td>openhands</td><td><code>&#126;/.agents/skills</code></td><td><code>.agents/skills</code></td></tr>
<tr id="target-pi"><td>pi</td><td><code>&#126;/.pi/agent/skills</code></td><td><code>.pi/skills</code></td></tr>
<tr id="target-pochi"><td>pochi</td><td><code>&#126;/.pochi/skills</code></td><td><code>.pochi/skills</code></td></tr>
<tr id="target-posit-assistant"><td>posit-assistant</td><td><code>&#126;/.posit/assistant/skills</code></td><td><code>.posit/assistant/skills</code></td></tr>
<tr id="target-purecode"><td>purecode</td><td><code>&#126;/.purecode/skills</code></td><td><code>.agents/skills</code></td></tr>
<tr id="target-qoder"><td>qoder</td><td><code>&#126;/.qoder/skills</code></td><td><code>.qoder/skills</code></td></tr>
<tr id="target-qoder-cn"><td>qoder-cn</td><td><code>&#126;/.qoder-cn/skills</code></td><td><code>.qoder/skills</code></td></tr>
<tr id="target-qwen"><td>qwen</td><td><code>&#126;/.qwen/skills</code></td><td><code>.qwen/skills</code></td></tr>
<tr id="target-replit"><td>replit</td><td>—</td><td><code>.agents/skills</code></td></tr>
<tr id="target-reasonix"><td>reasonix</td><td><code>&#126;/.reasonix/skills</code></td><td><code>.reasonix/skills</code></td></tr>
<tr id="target-roo"><td>roo</td><td><code>&#126;/.roo/skills</code></td><td><code>.roo/skills</code></td></tr>
<tr id="target-rovodev"><td>rovodev</td><td><code>&#126;/.rovodev/skills</code></td><td><code>.rovodev/skills</code></td></tr>
<tr id="target-tabnine"><td>tabnine</td><td><code>&#126;/.tabnine/agent/skills</code></td><td><code>.tabnine/agent/skills</code></td></tr>
<tr id="target-trae"><td>trae</td><td><code>&#126;/.trae/skills</code></td><td><code>.trae/skills</code></td></tr>
<tr id="target-trae-cn"><td>trae-cn</td><td><code>&#126;/.trae-cn/skills</code></td><td><code>.trae/skills</code></td></tr>
<tr id="target-vibe"><td>vibe</td><td><code>&#126;/.vibe/skills</code></td><td><code>.vibe/skills</code></td></tr>
<tr id="target-verdent"><td>verdent</td><td><code>&#126;/.verdent/skills</code></td><td><code>.verdent/skills</code></td></tr>
<tr id="target-warp"><td>warp</td><td><code>&#126;/.agents/skills</code></td><td><code>.agents/skills</code></td></tr>
<tr id="target-windsurf"><td>windsurf</td><td><code>&#126;/.codeium/windsurf/skills</code></td><td><code>.windsurf/skills</code></td></tr>
<tr id="target-witsy"><td>witsy</td><td><code>&#126;/.agents/skills</code></td><td><code>.agents/skills</code></td></tr>
<tr id="target-xcode-claude"><td>xcode-claude</td><td><code>&#126;/Library/Developer/Xcode/CodingAssistant/ClaudeAgentConfig/skills</code></td><td><code>.claude/skills</code></td></tr>
<tr id="target-xcode-codex"><td>xcode-codex</td><td><code>&#126;/Library/Developer/Xcode/CodingAssistant/codex/skills</code></td><td><code>.codex/skills</code></td></tr>
<tr id="target-zcode"><td>zcode</td><td><code>&#126;/.zcode/skills</code></td><td><code>.zcode/skills</code></td></tr>
<tr id="target-zed"><td>zed</td><td><code>&#126;/.agents/skills</code></td><td><code>.agents/skills</code></td></tr>
<tr id="target-zencoder"><td>zencoder</td><td><code>&#126;/.agents/skills</code></td><td><code>.agents/skills</code></td></tr>
</tbody>
</table>

:::info Universal target
**universal** target（`&#126;/.agents/skills`）是一個共用的 Agent 目錄，多個 AI CLI 都能從中讀取。當偵測到任何其他 Agent 時，`skillshare init` 會自動偵測它。在 Project mode 中，`amp`、`codex`、`cursor`、`dexto`、`kimi`、`purecode`、`replit`、`warp`、`witsy`、`zed` 和 `zencoder` 共用同一個 `.agents/skills` 路徑，並會自動歸類在 `universal` 之下。

這與 [npx skills CLI](https://github.com/vercel-labs/skills) 使用的是同一個路徑。共存細節請參閱 [FAQ：搭配 npx skills 使用 universal](/docs/troubleshooting/faq#using-universal-alongside-npx-skills)。
:::

## Oh My Pi (OMP)

使用 `omp` 管理原生 skills、[MCP 設定](../commands/mcp.md#omp)與
[程式碼 hooks](../commands/hooks.md#omp)。
Skills 預設在全域會放到 `~/.omp/agent/skills`，在 project 中放到 `.omp/skills`。
只有省略 skill 路徑的全域 OMP 設定項目才遵循 `PI_CODING_AGENT_DIR`。
會去除前後空白；空值或相對路徑使用標準預設路徑。若變數指向已宣告的 OMP 帳號
`config_dir`（包含符號連結），一般 `omp` 仍使用 `~/.omp/agent/skills`，讓帳號篩選條件保持獨立。
因為 Pi 也使用相同變數，自動偵測 OMP 需要 `~/.omp` 存在。
`init` 和 `target add` 會儲存選定的目標路徑，已儲存的路徑保持固定。
明確的 skill 路徑和帳號 `config_dir` 優先。OMP 只掃描
一層 `<skill>/SKILL.md`，且要求必須有 description；Skillshare 預設的 flat target 命名
符合這種結構。`oh-my-pi` 是 skill target 的別名；MCP client ID 為 `omp`。

MCP 使用 `~/.omp/agent/mcp.json` 或 `.omp/mcp.json`，而不是 Pi 的檔案。同步會保留
OMP 的啟用／停用清單與原生 server 設定。若要使用具名 profile，請以 `agent: omp`
與明確的 `config_dir` [宣告帳號 target](./configuration.md#agent-config-dir)；
自動的 profile 路徑選擇則是另一回事。

Dashboard 的 **Extensions** 分頁會列出原生、已設定、hook 與 plugin 的 extension
檔案、它們的 scope，以及從檢查到的設定得出的選取狀態。已選取不代表已載入或正在執行。
支援的獨立 module 可以透過會檢查版本的預覽與套用來切換；不支援或不確定的列維持唯讀。
這份清單絕不會匯入 extension 程式碼，也不會遷移設定。只有在 Skillshare 的所有權紀錄與
檔案雜湊相符時，由 Hooks 擁有的檔案才會連回 Hooks。

[OMP plugin 管理](../commands/plugin.md#omp)支援在原生 user/project scope 中，以
marketplace 為基礎的本機/Git source。OMP 帳號不會改變 plugin 的儲存位置。extension
編輯對已驗證版本、所有權與原生鎖的要求，請參閱同一份參考文件。

OMP 沒有 project 信任提示。同步 hooks 前請先審閱程式碼：寫入的 extension 可能會在下次
OMP 啟動時執行。

## DeepSeek Harness and GitLab Duo

上方列出的是預設路徑。對於內建的全域路徑，skillshare 也會遵循：

- **DeepSeek Harness：** `DSH_HOME` 會選擇 `<DSH_HOME>/skills`。空值或只有空白的值會使用預設路徑。該工具也會讀取 `~/.agents/skills` 與 project 的 `.agents/skills`；它的 project target 仍為 `.dsh/skills`。參見[原生 skill 探索文件](https://github.com/deepseek-ai/deepseek-harness/blob/master/packages/skill/skill-filesystem/README.md)。
- **GitLab Duo：** `GLAB_CONFIG_DIR` 會選擇 `<GLAB_CONFIG_DIR>/skills`；否則由 `XDG_CONFIG_HOME` 選擇 `<XDG_CONFIG_HOME>/gitlab/duo/skills`。兩者都未設定時，Windows 使用 `%APPDATA%\GitLab\duo\skills`，macOS/Linux 則使用 `~/.gitlab/duo/skills`。它的 project target 是 `skills`，不是 `.gitlab/duo/skills`。

GitLab Duo 的使用者層級 skills 仍屬實驗性功能，需要 `glab duo cli --enable-global-skills true` 或 `GITLAB_ENABLE_GLOBAL_SKILLS=true`。啟用全域 skills 時，它也會讀取共用的 `~/.agents/skills` 目錄。光是同步檔案並不會啟用探索。參見 [GitLab 的 Agent Skills 文件](https://docs.gitlab.com/user/duo_agent_platform/customize/agent_skills/)。

請以與該工具相同的原生 home 覆寫設定來執行 skillshare。skillshare 設定中明確指定的 skills 路徑維持不變；工具專屬的設定以及其他共用根目錄（例如 `DSH_AGENTS_HOME`）需要明確指定 target 路徑。

## 別名

有些 Target 為了向後相容或方便使用，提供了替代名稱：

| 別名 | 對應到 | 備註 |
|-------|-------------|-------|
| `agents` | `universal` | 舊名稱 |
| `claude-code` | `claude` | 舊名稱 |
| `aider-desk` | `aiderdesk` | 帶連字號的變體 |
| `codearts-agent` | `codearts` | 帶 agent 後綴 |
| `code-studio` | `codestudio` | 帶連字號的變體 |
| `command-code` | `commandcode` | 帶連字號的變體 |
| `devin-terminal` | `devin` | 帶 terminal 後綴 |
| `deep-agents` | `deepagents` | 帶連字號的變體 |
| `factory` | `droid` | 品牌／設定目錄名稱 |
| `forge-code` | `forgecode` | 帶連字號的變體 |
| `gemini-cli` | `gemini` | 帶 CLI 後綴 |
| `github-copilot` | `copilot` | 完整產品名稱 |
| `iflow-cli` | `iflow` | 帶 CLI 後綴 |
| `kilo` | `kilocode` | 簡短名稱 |
| `kimi-cli` | `kimi` | 帶 CLI 後綴 |
| `kiro-cli` | `kiro` | 帶 CLI 後綴 |
| `mistral-vibe` | `vibe` | 完整產品名稱 |
| `oh-my-pi` | `omp` | 完整產品名稱 |
| `purecode-ai` | `purecode` | 帶連字號的變體 |
| `qwen-code` | `qwen` | 帶 code 後綴 |
| `rovo-dev` | `rovodev` | 帶連字號的變體 |
| `tabnine-cli` | `tabnine` | 帶 CLI 後綴 |
| `zenflow` | `zencoder` | 完整產品名稱 |

你可以在所有指令中使用別名或正式名稱：

```bash
skillshare target add claude           # 正式名稱
skillshare target add claude-code      # 別名 — 結果相同
```

別名會自動被解析。設定檔與狀態輸出中使用的是正式名稱。

---

## 檢查 Target 路徑

對任何 Target，執行：

```bash
skillshare target <name>
```

---

## 自訂 Targets

沒看到你的 AI CLI？手動新增它：

```bash
skillshare target add myapp ~/.myapp/skills
```

詳情請參閱[新增自訂 Targets](./adding-custom-targets.md)。

---

## 相關文件

- [新增自訂 Targets](./adding-custom-targets.md) — 新增不支援的工具
- [設定](./configuration.md) — 設定檔參考
- [指令：target](/docs/reference/commands/target) — Target 指令

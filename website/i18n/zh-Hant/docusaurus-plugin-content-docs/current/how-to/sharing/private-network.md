---
sidebar_position: 5
---

# 私有網路上的機器

當你自己的機器之間已經能透過 SSH 互通時（例如經由 Tailscale、WireGuard 或家用網路），就能在這些機器上使用 skillshare。不需要 GitHub，也不需要任何代管服務。

本頁涵蓋三件事：

- [透過放在你某台機器上的 repository 同步](#ssh-remote)，取代代管的 Git 服務
- [從眼前的這台機器開啟另一台機器的 dashboard](#remote-dashboard)
- [透過 MCP 把你的 skills 提供給另一台機器上的 Agents](#remote-skills)

範例中的機器分別叫 `mac-mini` 和 `laptop`。請換成你的網路給它們的名稱，例如 Tailscale MagicDNS 名稱。

## 開始之前 {#before-you-start}

確認 SSH 不會跳出密碼提示，且另一台機器找得到 skillshare：

```bash
ssh you@laptop skillshare version
```

透過 SSH 執行的指令不會載入互動式 shell 的 profile，因此可能找不到 skillshare 安裝的目錄。如果找不到指令，請使用在那台機器上執行 `which skillshare` 印出的完整路徑（例如在 Apple silicon 上用 Homebrew 安裝時為 `/opt/homebrew/bin/skillshare`），或把該目錄加入非互動式 shell 會讀取的檔案（例如 `~/.zshenv`）中的 `PATH`。

## 透過放在你某台機器上的 Repository 同步 {#ssh-remote}

`push` 和 `pull` 可搭配任何 Git remote，所以放在你某台機器上的 bare repository 可以取代 GitHub。

在保存 repository 的機器上（它也可以是你要同步的其中一台機器）：

```bash
git init --bare ~/skillshare.git
```

在第一台機器上，透過 SSH 把 skillshare 連到它，然後 push：

```bash
skillshare init --remote you@mac-mini:skillshare.git
skillshare push
```

`you@mac-mini:skillshare.git` 是相對於該使用者家目錄的路徑；使用完整路徑的 `ssh://you@mac-mini/home/you/skillshare.git` 也可以。在保存 repository 的那台機器上，可以改給本機路徑（`~/skillshare.git`）。如果 skillshare 已經設定了其他 remote（例如 GitHub），`init` 不會更動它；請用 `init` 印出的指令（例如 `git -C ~/.config/skillshare/skills remote set-url origin you@mac-mini:skillshare.git`）切換，再執行 `skillshare push`。

新的 bare repository 會以 `master`（或你的 `init.defaultBranch`）作為預設分支，這不一定是第一台機器 push 的分支。其他機器會從預設分支開始，所以請把它指向被 push 的那個分支：

```bash
git --git-dir ~/skillshare.git branch                            # 被 push 的分支，例如 main
git --git-dir ~/skillshare.git symbolic-ref HEAD refs/heads/main
```

沒有這個步驟的話，下一台機器上的 `init` 會找不到任何可 pull 的內容。

接著在其他每台機器上執行相同的 `init --remote`。之後就使用平常的指令：

```bash
skillshare push    # 在你變更了 skills 的機器上
skillshare pull    # 在另一台機器上；pull 也會執行 sync
```

要在不登入另一台機器的情況下更新它，可以透過 SSH 執行它的 `pull`：

```bash
skillshare push && ssh you@laptop skillshare pull
```

其餘一切都和[跨機器 Sync](/docs/how-to/sharing/cross-machine-sync) 相同：第一次 pull 的行為、衝突，以及 `push` 和 `pull` 會帶走什麼。

:::tip
這個 bare repository 現在是你的 skills 唯一一份不在你編輯用機器上的副本。請像備份其他資料一樣備份它，或再加一個 remote，例如私有的代管 repository。
:::

## 開啟另一台機器的 Dashboard {#remote-dashboard}

每台機器都有自己的 dashboard，而且只管理那台機器。從別處開啟它有兩種方式。

### 透過 SSH tunnel

另一台機器上沒有任何東西會監聽本機以外的位址。先在背景啟動它的 dashboard，再把本機連接埠轉送過去：

```bash
ssh you@laptop skillshare ui start
ssh -N -L 19421:127.0.0.1:19420 you@laptop
```

如果該 dashboard 已經在私有位址上執行（見下方），`ui start` 會回報它正在那個位址執行；請改轉送到那個位址，例如 `-L 19421:100.101.102.103:19420`。

開啟 `http://localhost:19421`。使用本機連接埠 `19421`，是為了避免和這台機器上已在 `19420` 執行的 dashboard 衝突。

### 使用私有網路位址

在你的私有網路給那台機器的位址上啟動 dashboard，例如它的 Tailscale IP（`tailscale ip -4`）：

```bash
skillshare ui start --host 100.101.102.103
```

從網路上的任何機器開啟 `http://100.101.102.103:19420`。dashboard 會直接在 IP 位址上回應。若想改用名稱開啟，請用該名稱啟動它，或信任你輸入的那個確切位址：

```bash
skillshare ui start --host laptop.example.ts.net
skillshare ui start --host 100.101.102.103 --trusted-origin http://laptop:19420
```

其他任何名稱都會得到 403，這是為了保護 dashboard 免受 DNS rebinding 攻擊。完整規則請見 [Reverse Proxy](/docs/reference/commands/ui#reverse-proxy)。

:::warning
dashboard 沒有登入機制。任何能連到它位址的人都可以使用它的全部功能，包括會寫入 Agent 所執行指令的 MCP、Hooks 與 Plugins 頁面。請只把它綁定在你控制的網路上的位址，限制誰能連到它（例如使用 Tailscale 存取規則），並且不要在會加入不受信任網路的機器上使用 `--host 0.0.0.0`。
:::

從 dashboard 開啟的編輯器會在 dashboard 所在的機器上啟動，而不是在你瀏覽的那台機器上。

## 把你的 Skills 提供給另一台機器上的 Agents {#remote-skills}

當 Agent 執行所在的機器沒有自己的 skillshare 設定時，它可以透過網路從 `skillshare mcp serve` 讀取你的 skills，而不必使用同步的資料夾：

```bash
SKILLSHARE_MCP_TOKEN=change-me skillshare mcp serve --http 100.101.102.103:8765 --http-allow-insecure
```

`--http-allow-insecure` 只能用在本身就會加密流量的網路上，例如 Tailscale 或 WireGuard；其他情況請改用 HTTPS 提供。把 Agent 的 MCP client 指向 `http://100.101.102.103:8765/`，並以該 token 作為 bearer token。傳輸規則、TLS 選項，以及提供哪些內容，請見[透過 MCP 提供 skills](/docs/reference/commands/mcp#serve-skills-over-mcp)。

## 限制 {#limits}

- `push` 和 `pull` 會帶走 skills、agents 和 extras，但不包括每台機器的 `config.yaml`。Plugins、MCP server 和 hooks 需要在每台機器上各自設定；請見 [Plugins、MCP 與 Hooks](/docs/how-to/sharing/cross-machine-sync#plugins-mcp-hooks)。
- 使用者名稱不同的機器會儲存不同的家目錄路徑。如果你用其他方式共用 `config.yaml`，請開啟 `preserve_tilde_on_save`（見 [Configuration](/docs/reference/targets/configuration)）。
- 在 macOS 上，透過 SSH 啟動的指令可能無法存取登入鑰匙圈。如果 `ssh you@laptop skillshare pull` 因 HTTPS remote 的憑證而失敗，請改用 SSH remote，或為那台機器使用 token 環境變數。

## 參見

- [跨機器 Sync](/docs/how-to/sharing/cross-machine-sync) — 透過代管的 Git 服務 Sync
- [ui](/docs/reference/commands/ui) — Dashboard 旗標與背景伺服器
- [mcp](/docs/reference/commands/mcp#serve-skills-over-mcp) — 透過 MCP 提供 skills
- [init](/docs/reference/commands/init) — 使用 `--remote` 進行 Init

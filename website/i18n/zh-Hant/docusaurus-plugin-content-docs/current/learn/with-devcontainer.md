---
sidebar_position: 5
---

# 在 Dev Containers 中使用 skillshare

> 在 VS Code 中開啟專案，skills 就已就緒 — 不需要本機安裝。

## 先決條件

- 已安裝 [VS Code](https://code.visualstudio.com/) 與 [Dev Containers extension](https://marketplace.visualstudio.com/items?itemName=ms-vscode-remote.remote-containers)

## 運作原理

VS Code Dev Containers 讓你能在 Docker 容器內開發。你在 `.devcontainer/` 中定義環境，其餘的交給 VS Code 處理 — 開啟專案、點擊「Reopen in Container」，一切就緒。

skillshare 能自然地融入這個工作流程。將它加入 `postCreateCommand`，容器啟動時 skills 就會自動安裝並 sync。

## 設定

在你的 `.devcontainer/devcontainer.json` 中加入以下內容：

```json
{
  "postCreateCommand": "curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh && export PATH=\"$HOME/.local/bin:$PATH\" && skillshare init --no-copy --all-targets --no-skill && skillshare sync"
}
```

這裡的 PATH 設定只適用於這次設定指令。請把 `export PATH="$HOME/.local/bin:$PATH"` 加入容器內的 shell 設定檔（例如 `~/.bashrc`），讓之後開啟的終端機也能使用。

就這樣。當團隊成員在 VS Code 中開啟專案並點擊「Reopen in Container」時：

1. skillshare 會自動安裝
2. `init` 會以非互動模式執行 — 加入所有偵測到的 AI CLI targets，並略過複製提示與內建 skill 安裝
3. `sync` 會將 skills 分發到所有 targets

## 新增 Project Skills

若要建立團隊共用的 skills，將 `.skillshare/` 設定 commit 到 repo 中：

```bash
# 在容器內
skillshare init -p
skillshare install your-org/team-skills -p
```

接著 commit，並更新 `postCreateCommand` 讓它也 sync project skills：

```json
{
  "postCreateCommand": "curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh && export PATH=\"$HOME/.local/bin:$PATH\" && skillshare init --no-copy --all-targets --no-skill && skillshare sync && skillshare sync -p"
}
```

現在每位團隊成員開啟容器時都會取得相同的 skills。

## GitHub Codespaces

相同的 `.devcontainer/` 設定在 Codespaces 中不需要任何修改即可運作。Codespaces 執行 `postCreateCommand` 的方式與 VS Code 相同。

## 使用 ssenv 進行隔離測試

在 devcontainer 內，`ssenv` 讓你能建立隔離的 skillshare 環境以進行平行測試。每個環境都有自己獨立的 `HOME` 目錄，擁有各自的 config、skills 與 targets。

| 指令 | 功能 |
|---------|-------------|
| `ssnew <name>` | 建立一個新的隔離環境 |
| `ssuse <name>` | 切換到某個環境 |
| `ssback` | 回到原本的環境 |
| `ssls` | 列出所有環境 |
| `ssrm <name>` | 刪除一個環境 |

```bash
ssnew demo && ssuse demo    # 建立並切換
ss init && ss sync          # 指令會在隔離環境中執行
ssback                      # 回到原本的環境
```

這對於測試設定變更或 skill 安裝、同時不影響你的主要環境設定非常有用。

## 接下來呢？

- [Project skill 設定 →](/docs/how-to/sharing/project-setup)
- [團隊分享 →](/docs/how-to/sharing/organization-sharing)
- [Sync modes 說明 →](/docs/understand/philosophy/sync-modes-explained)

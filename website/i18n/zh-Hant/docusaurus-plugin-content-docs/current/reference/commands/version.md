---
sidebar_position: 6
---

# version

顯示目前的 skillshare 版本。

## 何時使用

- 回報 bug 前先確認你正在使用的版本
- 驗證升級是否成功
- 比對你的版本與 [最新發行版](https://github.com/runkids/skillshare/releases)

## 語法

```bash
skillshare version
skillshare -v
skillshare --version
```

## 範例輸出

```
skillshare version 0.16.6
```

## 更新通知

當有新版本可用時，`skillshare` 會在支援的指令執行後顯示更新通知。此通知具備 **Homebrew 感知能力**：如果 skillshare 是透過 Homebrew 安裝的，它會查詢 `brew info` 取得最新的 formula 版本，並建議執行 `brew upgrade skillshare`；否則它會檢查 GitHub releases 並建議執行 `skillshare upgrade`。

偵測是自動的——skillshare 會解析自身的執行檔路徑，並檢查它是否位於 Homebrew Cellar 前綴目錄下（例如 `/opt/homebrew/Cellar/skillshare/`）。位於 `/nix/store/` 下的執行檔會檢查 GitHub releases。若執行檔位於你的 `nix profile` 中，則建議執行 `nix profile upgrade skillshare`，否則提示你在 Nix 設定中更新 skillshare 並重新建置。

版本檢查結果會快取 24 小時，存放於 `~/.cache/skillshare/version-check.json`。

## 另請參閱

- [upgrade](./upgrade.md) — 升級至最新版本
- [doctor](./doctor.md) — 完整環境診斷
- [status](./status.md) — 顯示同步狀態，包含版本資訊

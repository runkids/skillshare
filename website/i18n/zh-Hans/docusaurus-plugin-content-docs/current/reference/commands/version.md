---
sidebar_position: 6
---

# version

显示当前的 skillshare 版本。

## 何时使用

- 在报告 bug 之前先确认自己使用的版本
- 验证升级是否成功
- 将自己的版本与 [最新发布版本](https://github.com/runkids/skillshare/releases) 比较

## 语法

```bash
skillshare version
skillshare -v
skillshare --version
```

## 示例输出

```
skillshare version 0.16.6
```

## 更新通知

当有新版本可用时，`skillshare` 会在支持该功能的命令执行后显示更新通知。这个通知会**感知 Homebrew**：如果 skillshare 是通过 Homebrew 安装的，它会查询 `brew info` 获取最新的 formula 版本，并建议执行 `brew upgrade skillshare`；否则会检查 GitHub releases 并建议执行 `skillshare upgrade`。

检测是自动进行的——skillshare 会解析自身可执行文件的路径，并检查它是否位于 Homebrew Cellar 前缀下（例如 `/opt/homebrew/Cellar/skillshare/`）。位于 `/nix/store/` 下的二进制文件会检查 GitHub releases。若二进制文件位于你的 `nix profile` 中，则建议执行 `nix profile upgrade skillshare`，否则提示你在 Nix 配置中更新 skillshare 并重新构建。

版本检查结果会缓存 24 小时，缓存位置为 `~/.cache/skillshare/version-check.json`。

## 另请参阅

- [upgrade](./upgrade.md) — 升级到最新版本
- [doctor](./doctor.md) — 完整的环境诊断
- [status](./status.md) — 显示同步状态，包含版本信息

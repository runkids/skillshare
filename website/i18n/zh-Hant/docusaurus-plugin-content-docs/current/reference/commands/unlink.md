---
sidebar_position: 3
---

# unlink

移除由 [`skillshare link`](./link.md) 建立的連結，或 skills source 中任何第一層的連結。只有連結本身會移到 trash；它指向的資料夾會保留。

```bash
skillshare unlink _dev-skills              # 移除連結
skillshare unlink _team -p                 # 移除專案 source 中的連結
skillshare unlink -- -local                # 結束選項解析，允許名稱以 - 開頭
```

## 何時使用

- 不再使用某個已連結的 checkout，但不動到其中的檔案
- 先移除連結，再以另一個名稱重新連結

```text
$ skillshare unlink _dev-skills
✓ Unlinked _dev-skills → trash, kept 7 days; its target is untouched

Next
  skillshare sync                       remove its skills from your targets
  skillshare trash restore _dev-skills  undo
```

## 發生了什麼

`unlink` 只把連結項目移到 trash，連結資料夾本身包含 `SKILL.md` 時也適用。dashboard 的 **Unlink** 執行相同操作。trash 會把它列為 `link → <target>`，`skillshare trash restore <name>` 會重新建立連結（或 junction），絕不會變成目標的複本。之後執行 `skillshare sync`，就會把它的 skills 從 targets 移除。 在 global mode 中，`unlink` 也會移除為該連結名稱保留的追蹤儲存庫紀錄及其稽核接受紀錄；連結目標不受影響。

如果連結移至 trash 後中繼資料清理失敗，CLI 和 dashboard 會警告紀錄可能仍然存在。連結移除已完成：請在重新安裝前解決中繼資料錯誤，不要重試移除已不存在的連結。

不是第一層連結的名稱會被拒絕，所以它絕不會移除一般的 skill 或資料夾：

```text
$ skillshare unlink my-skill
✗ cannot unlink my-skill: my-skill is not a link
```

| 原因 | 成因 |
|--------|-------|
| `<name> not found in source` | source 底下沒有這個名稱的項目 |
| `<name> is not a link` | 該項目是一般目錄或檔案；請改用 [`uninstall`](./uninstall.md) |
| `"<name>" is not a first-level name` | 名稱含有路徑分隔符號，或是 `.` / `..` |

## 選項

| 旗標 | 說明 |
|------|-------------|
| `--project, -p` | 使用目前目錄的專案層級設定 |
| `--global, -g` | 使用全域設定（`~/.config/skillshare`） |
| `--` | 結束選項；允許以 `-` 開頭的名稱 |
| `--help, -h` | 顯示說明 |

## 另請參閱

- [link](./link.md) — 建立連結
- [trash](./trash.md) — 還原已移除的連結
- [設定 — `follow_source_links`](../targets/configuration.md#follow_source_links) — discovery 行為與透過連結寫入

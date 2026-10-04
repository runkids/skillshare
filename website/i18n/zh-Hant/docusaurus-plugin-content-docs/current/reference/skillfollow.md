---
sidebar_position: 4
---

# .skillfollow（實驗性）

明確宣告 skills source 第一層的 symlink 或 Windows junction，讓 skillshare 透過邏輯路徑探索外部群組或 repo。外部工作目錄可保留原位；宣告不授予 skillshare 對其檔案的寫入權。

## 設定

檔案放在**設定的 skills source 根目錄**：通常是 `~/.config/skillshare/skills/`、Windows 的 `%AppData%\skillshare\skills\`，或 project mode 的 `.skillshare/skills/`；自訂 `sources.skills` 也適用。不適用 agents/extras，也不從巢狀 repo 根目錄讀取。

外部 repo 含 `.git` 與 `review/SKILL.md`，但根目錄本身不含 `SKILL.md`：

```text
~/work/team-skills/
~/.config/skillshare/skills/
├── _team-skills -> ~/work/team-skills/
├── .skillfollow
└── .gitignore
```

自行建立第一層連結。macOS/Linux：

```bash
ln -s "$HOME/work/team-skills" "$HOME/.config/skillshare/skills/_team-skills"
```

Windows 可在 Command Prompt 建立不需 symlink 權限的 directory junction（替換外部路徑）：

```text
mklink /J "%AppData%\skillshare\skills\_team-skills" "C:\work\team-skills"
```

寫入**項目名稱**，不是外部路徑：

```text title=".skillfollow"
# One direct child of the skills source per line
_team-skills
```

在 skills source 的 `.gitignore` 加入有根目錄錨定、**無尾端斜線**的規則：

```text title=".gitignore"
/_team-skills
/.skillfollow.local
```

`/_team-skills/` 不夠：Git 把 symlink 儲存為檔案而不是目錄。提交 `.skillfollow`，不要追蹤連結與 `.skillfollow.local`。若連結已在 index，檢查路徑後從 skills source 執行下列指令；它只移除 index 項目，保留工作目錄連結：

```bash
git rm --cached -- '_team-skills'
```

執行 `skillshare doctor`、`skillshare list --no-tui`、`skillshare sync --dry-run`；用 `-g`/`-p` 選定範圍，預覽正確再 `skillshare sync`。宣告與 ignore 檔須手動編輯；discovery、status、doctor、dry run 不會自動建立或修復它們。目前沒有 `follow`/`unfollow` 指令。

`_` 前綴且含 `.git` 的項目視為 tracked repo，其他 followed 目錄視為群組。Skills 保留 `_team-skills/review` 等邏輯路徑（flat name：`_team-skills__review`）。Source-root/repo 的 `.skillignore` 仍適用（含 followed 群組內巢狀的 tracked repo）；巢狀的 tracked repo（含 `--track --into` 安裝的）擁有自己的 skills：`list` 顯示該 repo、`status` 與 Dashboard 計入該 repo、`.metadata.json` target override 生效、Dashboard 拒絕單獨解除安裝其中的 skill；未宣告第一層連結仍不可見。

## 格式

`.skillfollow.local` 與 `.skillfollow` 並列，加入本機名稱。兩檔為聯集，先 base 再 local，重複名稱合併；不同於 `.skillignore.local`，沒有否定或覆寫規則。

- 去除行首尾空白，忽略空白行及以 `#` 開始的整行註解；註解另起一行。
- 只能是直接子項目名稱；拒絕 `.`、`..`、絕對路徑、`C:` 等 volume/drive 名稱、UNC、`/`、`\`，及 path cleaning 會改變的名稱。
- 不接受 glob/否定：`*`、`?`、`[`、`]`、`{`、`}`、`!`、NUL 都拒絕。無效行產生警告，不成為 followed 項目。
- 只跟隨宣告的第一層連結，不遍歷 followed tree 內的巢狀連結。
- skillshare 絕不會建立已宣告的項目，即使其連結離線也一樣：`install`（一般、`--into`、`--track`、無參數重裝）、`new`、`collect`、`trash restore`、`init` 匯入、symlink 模式的 sync 遷移，以及 Dashboard 的 create/install/collect/restore 對宣告項目內的目的地以 `<source>/<entry> is a link; edit its target directly` 拒絕（Dashboard 為 409）；記錄在宣告項目內的 tracked repo 不會被列為 missing，也不會被 rehydrate。宣告檔存在但無法讀取時，這些寫入一律以讀取錯誤拒絕，因為讀不到的檔案可能正宣告了該目的地。

## 狀態與復原 {#states}

安全檢查使用 canonical paths；第一個符合的狀態優先。項目重疊會拒絕雙方，不由宣告順序決定。

| 狀態 | 意義與處理 |
|---|---|
| `missing` | 項目不存在、斷鏈、無法讀取，或安全邊界無法解析；遍歷讀取失敗也會標為 missing。恢復磁碟/連結/讀取權限，修正邊界，或移除廢棄宣告 |
| `not-link` | 真實目錄，照常探索；不改變一般擁有權，無須修復 |
| `invalid-target` | 目標不是目錄，或項目不是連結/目錄；改為指向目錄的連結或移除宣告 |
| `cycle` | 目標等於 source、在其內部或為其祖先；改指向獨立外部目錄 |
| `target-overlap` | 目標等於、包含或位於已啟用 skills target 內；分開輸入與輸出目錄 |
| `inside-git-root` | 目標在 skillshare 有效 Git staging tree 內；把外部樹移出，忽略連結無法隱藏實體檔案 |
| `entry-overlap` | 宣告目標相同或相互包含；移除或改指向，使宣告不重疊 |
| `single-skill` | 目標根目錄含 `SKILL.md`，目前不支援；改跟隨外層群組/repo 或移除宣告 |
| `followed` | 安全可讀的群組/tracked repo，可探索與同步 |
| `undeclared-link` | 未在兩檔宣告的第一層連結；可維持不可見，或宣告並加入 ignore |

`not-link`/`followed` 的 doctor 檢查為 pass；其他宣告狀態為 warning 並暫停清理。`undeclared-link` 僅 info，不暫停清理。Parser 警告另外呈現。

若 `.skillfollow` 或 `.skillfollow.local` 存在但無法讀取，discovery 會停止而不是以不完整的結果繼續：`sync` 拒絕並保留既有 target，`check` 與 `status` 回報讀取錯誤而非空計數，`doctor` 在 `skillfollow` 回報並把需要 discovered skills 的檢查（`skills_validity`、`skill_integrity`、`skill_targets_field`、`sync_drift`）標為 skipped 而不是當成空 source 判定，所有 `update`（CLI、Dashboard、`install --update`）即使 `--force` 也拒絕、Dashboard update-all 整體失敗，source 的 Git staging 也會拒絕。恢復該檔案的讀取權限或移除它。同一規則也適用於 followed 項目內部：選取其下某個群組的 `check` 與 `update`（`--group <name>` 或位置參數群組名）在群組內有目錄無法讀取時，以 `incomplete discovery of <entry>: <read error>` 拒絕，而不是只處理可讀的部分。

## 指令呈現 {#visibility}

- **Status**：`.skillfollow: N entries, M skipped`，local 啟用時加 `(.local active)`，另列 prune 暫停復原訊息。JSON 的 `source.skillfollow` 含 `active`、`local_active`、`entry_count`、`followed_count`、`skipped_count`、宣告 `entries`（`name`、`state`、選用 `resolved_target`、`reason`），以及選用 `warnings`/`prune_paused`。無宣告或宣告警告時省略此欄位。
- **Doctor**：`skillfollow` 列宣告狀態，`skillfollow_prune` 列清理阻擋。未宣告連結維持 `undeclared_source_links` info。Git repo 內另檢查 indexed/`not-ignored` 連結與不安全的 local 檔，不修改檔案。
- **`list --no-tui`**：followed tracked repo 加 `→ <resolved>`（家目錄可縮為 `~`）；skills 路徑仍為邏輯路徑，JSON 格式不變。
- **Diff**：以與 sync 相同的規則預覽。宣告項目無法使用時不回報任何移除，顯示 `<target>: prune paused; unavailable .skillfollow entry: <name> (<state>)`，sync 會保留的 standard naming managed copy 列為 **Kept**。`diff --json` 逐 target 加上 `prune_paused` 與 `keep` 項目。Dashboard diff 加上 `prune_paused`，保留的 copy 顯示為 `skip`。它也依 sync 的 prune 規則預覽 followed orphan link：指向 followed 項目 resolved 位置的 managed merge link，在其 skill 退出 discovery 後列為 `prune`；你自行建立的同目標 link 列為 `local`。
- **Dashboard**：Skills、Overview、Check、Update、Audit、Hub 可看到邏輯路徑（audit 透過 resolved root 掃描 followed skill）。內容編輯、解除安裝、啟停、target 覆寫、source URL 變更會拒絕。直接編輯外部樹，或在 **source-root `.skillignore`** 隱藏。尚無專用宣告編輯頁。Dashboard sync 與 CLI 共用 prune/copy 安全，逐 target 回報 `prune_paused`/`kept` 與警告；Targets 把 managed followed link 算為 linked 而非 local。

可辨認的原始診斷：

```text
_team-skills: not-ignored; add "/_team-skills" to <source>/.gitignore
_team-skills: indexed; run git rm --cached -- '_team-skills' and add "/_team-skills" to <source>/.gitignore
.skillfollow.local: tracked; run git rm --cached -- .skillfollow.local
.skillfollow.local: not-ignored; add "/.skillfollow.local" to <source>/.gitignore
```

## 清理安全 {#cleanup}

**任何宣告項目**不可用（除了 `followed`/`not-link`）時，merge/copy 的所有 skills target 暫停 prune，`sync --force` 也不能覆寫，pull source 後 init 的首次 sync 亦同。仍可建立新連結/副本。Standard naming 下，無法證明來源的既有 managed copy 不替換；flat naming 可繼續。Merge link 可替換，但會警告：項目恢復時可能名稱衝突。

Status/doctor 對每個阻擋顯示：

```text
prune paused: <name> is <state>; restore or fix <path>, or remove <name> from .skillfollow[.local], to resume cleanup
```

修復項目，或從**每個含該名稱的宣告檔**移除，然後再 sync。廢棄宣告會讓清理無限期暫停；移除宣告不刪除外部樹。透過邏輯 source 的 managed orphan link 可清理；直接指向已取消跟隨外部路徑的 managed link 則保留並警告 `managed link resolves outside the source after unfollow; remove it or re-run with --force`。

## 更新安全 {#updates}

CLI、Dashboard（含 all/streaming）、`install --update` 共用 followed tracked repo 策略：乾淨樹與 **fast-forward-only** pull（`--ff-only --no-rebase`）。明確 `--force` 在 dry run 也拒絕。Dirty、status-check error、fast-forward 失敗（含分歧）為各項失敗，提供 ``resolve in `<resolved path>` ``；其他 batch 項目繼續。請在外部 repo 解決，不要用 force 重試；一般 installed repo 策略不變。Agent repo 完全不在此策略內：`.skillfollow` 屬於 skills source，Dashboard 更新 repo-backed agent 時不會讀取它，即使 skills 的宣告檔無法讀取也一樣。

followed entry 之下的一般 skill 不會被重新安裝。`update` 在所有選擇方式（`--all`、名稱、glob、group、project mode、dry run）與 Dashboard 單項更新、update-all 中，把每一項標為失敗 `followed repository update refused: skill <path> is inside followed entry <name>`，其他項目繼續。只有 followed repository 本身會依上述策略更新。

**Audit 失敗仍 hard-reset 到 pull 前 commit。** Audit 掃描 resolved root、回報邏輯路徑，scan error 也阻擋。更新期間不要編輯 repo、重指連結或同時跑 Git：檢查是 snapshot，不是 lock，rollback 可丟失併發變更。Skillshare 外的 pull/編輯不會自動 audit；自行執行 `skillshare audit`。

## Source Git 安全 {#git-safety}

`commit`、`push`、其 dry run、Dashboard staging、init source commit 會拒絕 Git 可到達的 indexed/未忽略宣告連結。依 doctor 指示加入精確的無尾斜線 ignore 並 `git rm --cached`；不會自動取消追蹤。Guard 依實體 Git 可達性判定，不會僅因 skills 宣告而阻擋無關 agents/extras repo。

Source **pull/reset/checkout** 也拒絕 indexed 宣告（包括不存在但仍 indexed 的連結），或 incoming revision 觸碰含任何工作目錄連結元件的路徑，**宣告與否皆然**；宣告項目內的路徑即使其連結缺失也拒絕（錯誤會指出項目名；從 remote 移除該路徑，或從宣告移除該項目）。Ignore 不足以防止 Git 替換連結；錯誤列 commit/path，依指示取消追蹤並 ignore，或先修正 remote。Pull fetch 後檢查固定 revision；Dashboard checkout 檢查選定既有 local/remote-tracking revision，不增加隱含 fetch。Dashboard discard 保留 ignored followed link。

這些 guard 只保護 skillshare 操作，不保護自行執行的 Git。

## 限制

單 skill、`follow`/`unfollow` 與宣告編輯頁仍是未來工作；巢狀連結不跟隨。在關閉 Developer Mode 的 Windows 11 ARM64 上，已用跟隨的 junction（管理員與 basic-user token）和目錄 symlink（管理員 token）驗證 global mode 的 discovery、status、sync、prune 暫停與恢復、update 拒絕、unfollow、`.skillfollow.local` 與 `invalid-target`。project mode 的相對連結、Developer Mode 的相對 symlink、以連結形式存在的 source root 或 target 上層目錄，以及 dashboard 在 Windows 上**尚未驗證**。

## 另見

- [篩選](./filtering.md#skillignore)
- [Source 與 Targets](../understand/source-and-targets.md)
- [Update](./commands/update.md)
- [Sync](./commands/sync.md)

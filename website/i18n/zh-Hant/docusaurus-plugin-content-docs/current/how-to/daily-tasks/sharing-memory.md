---
sidebar_position: 12
---

# 跨 AI 工具共用記憶

將決策、經驗與專案背景集中在一個 Markdown 筆記資料夾。先為工具連接閱讀指引，再於新 session 中驗證 Agent 是否讀取相關筆記。

本教學使用 global mode，並已將 Claude 與 Codex 設為 targets：

```bash
skillshare ui -g
```

所有截圖的 UI、筆記與對話框皆為英文，使用 `/tmp/skillshare-memory-docs` 下的隔離示範 home。你的路徑會不同。

## 1. 建立記憶

開啟 **Extras → Memory**，點擊 **Create memory**。

![英文 Memory 分頁的初始狀態](/img/memory-empty-demo.png)

這會註冊名為 `memory` 的 source-only extra，並建立缺少的起始筆記：`INDEX.md` 是簡短入口；`LEARNED.md` 記錄經驗的日期、背景、結論與證據。既有筆記與設定會保留。

![包含 INDEX.md 與 LEARNED.md 的 Memory](/img/memory-starters-demo.png)

預設 global 資料夾為 `~/.config/skillshare/extras/memory/`，也會顯示在 **Extras → Folders & files**，起初沒有 targets。本機的 Agent 直接讀取此來源，不必把筆記副本 sync 到各工具。

## 2. 連接工具

在 **Use with agents** 點擊 **Connect to agents**。自行選取工具並為每個工具選擇更新模式，再點擊 **Review changes**。

- `passive`（預設）：Agent 會讀筆記，只在你要求時更新。
- `active`：Agent 也會保存之後的工作階段仍用得到的事實，例如你明確表示的偏好、附理由的決定或已確認的陷阱。它會略過一次性的細節與猜測；拿不準時先提議並等你同意；優先更新既有筆記而不是重複新增，並告訴你存了什麼。

讀取同一個檔案的工具共用一個區塊，所以切換其中一個會一起切換。要改已設定工具的模式，再開一次 **Connect to agents** 切換即可，變更同樣會經過預覽。

![英文連接對話框中的 instructions 檔案變更預覽](/img/memory-connect-demo.png)

檢查每個檔案的差異（刪除行標 `−`，新增行標 `+`），再點擊 **Apply changes**。Skillshare 將受管理的閱讀指引區塊加入工具既有的 instructions 檔案，或工具已讀取的共用來源；若檔案尚不存在，也可建立。區塊帶有 scope 與內容 hash 標記，其餘內容、既有指派與連接模式皆保留。修改既有檔案前會備份。若其他工具也讀取同一檔案，預覽會提示，也會警告已知的字元上限。

![英文 Memory 分頁顯示已設定的工具](/img/memory-connected-demo.png)

**Configured** 表示工具的讀取鏈已有目前的指引，不代表 Agent 已讀取。**Not configured**、**Outdated** 與 **Needs attention** 描述的是 instructions 檔案。完整但過期的區塊可經再次檢查後更新；手動修改或標記格式有誤的區塊會保留，需手動修復。若工具從不同檔案讀到兩種模式的區塊，也會顯示需要處理：請把讀這些檔案的工具設成同一種模式。從多個檔案讀到區塊的工具，要先移除多餘的區塊才能切換模式。未同步的共用 instructions 要先 sync；無法讀取的 instructions 檔案會略過。若預覽後檔案改變，必須重新檢查再套用。

使用 **Open AGENTS.md** 檢查或修復 instructions。連接預覽是 dashboard 流程，沒有新增 CLI 連接命令。

## 3. 新增筆記與索引連結

點擊 **New note**，在 **File name** 輸入 `wiki/architecture.md`，保留 **Link from INDEX.md** 勾選，再點擊 **Create**。當 `INDEX.md` 可讀取時才顯示此選項，且預設勾選。

![英文新增筆記對話框中的巢狀路徑與索引選項](/img/memory-folder-demo.png)

Skillshare 自動建立缺少的子資料夾，並在 `INDEX.md` 檔案末尾附加相對 Markdown 連結。索引更新會檢查 version 並備份。若加入連結失敗，筆記仍保留，並顯示部分完成的警告。選取未索引的筆記，點擊 **Add to INDEX** 可重試；也可自行編輯索引。保持索引簡短。失效連結會顯示警告，不會自動移除。

筆記須為 UTF-8 `.md` 檔案，大小不超過 1 MiB。不支援的筆記仍列出並標示 **Unsupported file**，其他正常筆記仍可使用。來源內的隱藏檔案、隱藏資料夾與符號連結不包含在內。

選取筆記，點擊 **Edit**，加入以下英文示範內容並 **Save**：

```markdown
# Architecture decisions

## Shared memory

Claude and Codex read the same Markdown notes from Skillshare.
Keep durable decisions here and verify facts that may have changed.

## Retrieval

Read INDEX.md first, then only the notes relevant to the current task.
Update notes when the user asks you to remember a decision.
```

![英文筆記的 Markdown 預覽](/img/memory-note-demo.png)

左側樹狀檢視支援巢狀資料夾，上方有搜尋框，旁邊的 **Refresh** 按鈕會在 Agent 從 dashboard 外寫入筆記後，重新載入清單與目前開啟的筆記；右側可切換 **Preview** / **Source**，較長的筆記會先收合，點擊 **Show all** 展開。筆記名稱旁是 **Edit** 與 **More actions** 選單，內含 **Copy file path**、**History**、**Move or rename** 與 **Delete note**；**Use with agents** 位於筆記下方。相對連結可在檢視器中開啟既有筆記。

![展開 wiki 的英文雙欄 Memory 檢視器](/img/memory-tree-demo.png)

## 4. 搜尋、編輯與還原

在 **Search names and content** 輸入 `Retrieval`。搜尋包含子資料夾中的筆記路徑與內容，不分大小寫。清除搜尋即可顯示全部筆記。

![英文搜尋顯示巢狀筆記](/img/memory-search-demo.png)

你也可使用文字編輯器。重新載入 dashboard 可查看外部變更。若編輯中的筆記被修改，過期的儲存會被拒絕，草稿會保留。編輯器顯示 **Latest saved version** 供比較，並提供 **Copy draft**。自行比較或合併後，點擊 **Save my draft** 並確認取代。儲存會使用更新後的 version 並備份已儲存內容；若再次發生外部修改，仍會產生衝突。

![英文編輯器保留草稿並顯示最新儲存版本](/img/memory-conflict-demo.png)

**History** 會開啟 **Backup Files**，以筆記的絕對路徑篩選。可預覽、還原版本，再重新載入 Memory。已刪除的筆記也能在同一頁還原。

![wiki/workflow-check.md 的英文 Backup Files 還原預覽](/img/memory-restore-demo.png)

## 5. 在新 Agent session 中驗證

在相關筆記加入臨時值，例如 `memory-check: demo-7429`，並儲存。在已連接的工具開啟新 session。**Copy verification prompt** 提供以下提示：

將滑鼠移到 **Copy verification prompt** 上，可在複製前預覽完整內容。

![English verification prompt tooltip](/img/memory-verification-demo.png)

> 請讀取 instructions 指定的共用記憶 INDEX.md，以及與本次任務相關的筆記。回報筆記的完整路徑及我加入的臨時驗證值。請使用檔案讀取工具，讓我能檢查讀取事件。

檢查實際 read tool event，核對完整路徑與臨時值。在另一個已連接工具重複驗證，再移除臨時值。這是手動驗證，Skillshare 沒有保證可用的讀取 telemetry。Agent 自稱讀過或顯示 **Configured** 都不足以證明讀取。

需要記錄經驗時，請要求 Agent 更新 `LEARNED.md` 的背景、結論與證據。兩種模式都會在每個 task 開始時讀 `INDEX.md`。筆記由使用者管理：`passive` 指引讓 Agent 提出值得記的事實、只在使用者要求時更新；`active` 指引則讓 Agent 依上述規則把這類事實存在這裡，而不是工具自己的記憶：一兩句話說得完的事實，直接作為 `INDEX.md` 中 `## Notes` 下的一個項目，較長的筆記才另建檔案。本功能不啟用 native automatic memory、自動學習或 Obsidian 整合。

## Project mode

先初始化專案的 Skillshare 設定，再執行：

```bash
skillshare extras memory init -p
skillshare ui -p
```

預設來源為 `.skillshare/extras/memory/`；使用可見設定目錄時為 `skillshare/extras/memory/`。既有 extras source overrides 仍適用。Project mode 提供相同的連接、索引、編輯與還原流程。Repo 內來源的指引使用相對於 **project root** 的路徑，即使 instructions 檔案在子資料夾中也一樣。Project 指引會告訴 agent 把這個專案的筆記放在這裡，關於你、你的工具或其他專案的筆記不屬於這裡，因此同時讀到 shared memory 指引的 agent 知道每個事實該存哪裡。Project 外的 override 使用絕對路徑。移動絕對來源或變更位置後，請重新產生並檢查指引。

## 進階替代方式：自行複製指引

打開 **Copy guidance**，選擇 `passive` 或 `active`，再把複製的區塊貼到 Agent 會讀取的 instructions 檔案。**Open AGENTS.md** 可開啟既有編輯器。

![英文 Copy guidance 預覽](/img/memory-guidance-demo.png)

也可在 **Extras → AGENTS.md** 建立共用 instructions，使用該頁既有的連接流程。請檢查 [跨工具共用一份 AGENTS.md](./sharing-instructions.md) 的連接模式與取代警告。

![英文共用 instructions 頁面](/img/memory-agents-demo.png)

保留 scope 與 hash 標記。手動修改產生區塊的內文會使其標為已修改，後續連接預覽將保留此內容。

## CLI 替代方式

```bash
skillshare extras memory init -g
printf '# Architecture decisions\n\nRead relevant notes on demand.\n' |
  skillshare extras memory write wiki/architecture.md --from - -g
skillshare extras memory list --search architecture -g
skillshare extras memory show wiki/architecture.md -g
skillshare extras memory instructions -g
skillshare extras memory instructions --update-mode active -g
```

CLI 只輸出閱讀指引（未指定 `--update-mode active` 時為 `passive`），需自行貼上；不會連接工具或新增索引連結。更新筆記須提供目前的 `--version`，詳見 [`extras memory` 參考](../../reference/commands/extras.md#extras-memory)。

## 重新命名或移動筆記

選取筆記，開啟 **More actions** 後點擊 **Move or rename**，輸入新的相對 `.md` 路徑。將 `wiki/architecture.md` 改為 `wiki/design.md` 是重新命名；改為 `projects/design.md` 則會移到其他資料夾。缺少的資料夾會自動建立，點擊 **Move** 套用。

![英文 Move or rename 對話框，輸入新的資料夾路徑](/img/memory-move-demo.png)

內容與權限會保留；目標已存在或版本過期時會拒絕操作。移動前會備份來源，可使用 **Restore in Backup Files** 查看舊路徑的歷史；在該處還原會重建舊筆記，移動後的筆記仍保留。

Markdown 連結不會自動更新，包含筆記內部的相對連結。請自行修正 `INDEX.md` 與其他筆記；失效的索引連結會顯示在瀏覽器上方。Agent 指引指向來源根目錄的 `INDEX.md`，請保留該位置。

## 刪除筆記

在 **More actions** 選單或編輯器點擊 **Delete note**，確認檔案名稱。未儲存編輯會捨棄。Skillshare 檢查已儲存版本並備份後，只刪除該筆記，保留資料夾與其他筆記。請自行更新 `INDEX.md` 中失效的連結。刪除後的 **Restore in Backup Files** 可開啟已篩選的歷史版本。

![英文刪除筆記確認](/img/memory-delete-demo.png)

CLI 也需要剛讀取的 version：

```bash
version=$(skillshare extras memory show wiki/architecture.md --json -g | jq -r '.version')
skillshare extras memory delete wiki/architecture.md --version "$version" -g
```

使用 [`backup files`](../../reference/commands/backup.md) 還原：先執行 `skillshare backup files show <absolute-note-path>`，再執行 `skillshare backup files restore <absolute-note-path> <id>`。

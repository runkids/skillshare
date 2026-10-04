---
sidebar_position: 4
---

# .skillfollow（实验性）

明确声明 skills source 第一层的 symlink 或 Windows junction，让 skillshare 通过逻辑路径发现外部组或 repo。外部工作目录可保留原位；声明不授予 skillshare 对其文件的写入权。

## 设置

文件放在**配置的 skills source 根目录**：通常为 `~/.config/skillshare/skills/`、Windows 的 `%AppData%\skillshare\skills\`，或 project mode 的 `.skillshare/skills/`；自定义 `sources.skills` 也适用。不适用 agents/extras，也不从嵌套 repo 根目录读取。

外部 repo 含 `.git` 与 `review/SKILL.md`，但根目录本身不含 `SKILL.md`：

```text
~/work/team-skills/
~/.config/skillshare/skills/
├── _team-skills -> ~/work/team-skills/
├── .skillfollow
└── .gitignore
```

运行 [`follow`](./commands/follow.md)，指定 entry 名称和外部目录：

```bash
skillshare follow _team-skills --to ~/work/team-skills
```

它会创建链接（Windows 上是 junction），把 `_team-skills` 加入 `.skillfollow`，source 位于 Git 工作区内时再把 `/_team-skills` 加入 `.gitignore`，最后打印该 entry 的 [状态](#states)。链接已存在时省略 `--to`。加上 `--local` 会改为声明在 `.skillfollow.local`，并把 `/.skillfollow.local` 加入 `.gitignore`。[`unfollow`](./commands/unfollow.md) 会撤销以上全部操作。

### 手动设置

自行创建第一层链接。macOS/Linux：

```bash
ln -s "$HOME/work/team-skills" "$HOME/.config/skillshare/skills/_team-skills"
```

Windows 可在 Command Prompt 创建无需 symlink 权限的 directory junction（替换外部路径）：

```text
mklink /J "%AppData%\skillshare\skills\_team-skills" "C:\work\team-skills"
```

写入**条目名称**，不是外部路径：

```text title=".skillfollow"
# One direct child of the skills source per line
_team-skills
```

在 skills source 的 `.gitignore` 加入根目录锚定、**无末尾斜杠**的规则：

```text title=".gitignore"
/_team-skills
/.skillfollow.local
```

`/_team-skills/` 不够：Git 把 symlink 存为文件而非目录。提交 `.skillfollow`，不要跟踪链接与 `.skillfollow.local`。若链接已在 index，检查路径后从 skills source 执行下列命令；它只移除 index 条目，保留工作目录链接：

```bash
git rm --cached -- '_team-skills'
```

### 检查与 sync

执行 `skillshare doctor`、`skillshare list --no-tui`、`skillshare sync --dry-run`；用 `-g`/`-p` 选范围，预览正确后 `skillshare sync`。只有 `follow` 和 `unfollow` 会写入声明与 ignore 文件；discovery、status、doctor、dry run 不会自动创建或修复。

`_` 前缀且含 `.git` 的条目视为 tracked repo，其他 followed 目录视为组。Skills 保留 `_team-skills/review` 等逻辑路径（flat name：`_team-skills__review`）。Source-root/repo `.skillignore` 仍适用（含 followed 组内嵌套的 tracked repo）；未声明第一层链接仍不可见。

## 格式

`.skillfollow.local` 与 `.skillfollow` 并列，加入本机名称。两文件为并集，先 base 再 local，重复名称合并；不同于 `.skillignore.local`，没有否定或覆盖规则。

- 去除行首尾空白，忽略空行及以 `#` 开始的整行注释；注释另起一行。
- 只能是直接子条目名称；拒绝 `.`、`..`、绝对路径、`C:` 等 volume/drive 名称、UNC、`/`、`\`，及 path cleaning 会改变的名称。
- 不接受 glob/否定：`*`、`?`、`[`、`]`、`{`、`}`、`!`、NUL 都拒绝。无效行产生警告，不成为 followed 条目。
- 只跟随声明的第一层链接，不遍历 followed tree 内的嵌套链接。

## 状态与恢复 {#states}

安全检查使用 canonical paths；第一个匹配的状态优先。条目重叠拒绝双方，不由声明顺序决定。

| 状态 | 含义与处理 |
|---|---|
| `missing` | 条目不存在、断链、不可读，或安全边界无法解析；遍历读取失败也标为 missing。恢复磁盘/链接/读取权限，修正边界，或移除废弃声明 |
| `not-link` | 真实目录，照常发现；不改变普通所有权，无需修复 |
| `invalid-target` | 目标不是目录，或条目不是链接/目录；改为指向目录的链接或移除声明 |
| `cycle` | 目标等于 source、在其内部或为其祖先；改指向独立外部目录 |
| `target-overlap` | 目标等于、包含或位于已启用 skills target 内；分开输入与输出目录 |
| `inside-git-root` | 目标在 skillshare 有效 Git staging tree 内；把外部树移出，忽略链接无法隐藏实际文件 |
| `entry-overlap` | 声明目标相同或互相包含；移除或改指向，使声明不重叠 |
| `single-skill` | 目标根目录含 `SKILL.md`，目前不支持；改跟随外层组/repo 或移除声明 |
| `followed` | 安全可读的组/tracked repo，可发现与同步 |
| `undeclared-link` | 未在两文件声明的第一层链接；可保持不可见，或运行 `skillshare follow <name>` |

`not-link`/`followed` 的 doctor 检查为 pass；其他声明状态为 warning 并暂停清理。`undeclared-link` 仅 info，不暂停清理。Parser 警告另列。

若 `.skillfollow` 或 `.skillfollow.local` 存在但无法读取，discovery 会停止而不是以不完整的结果继续：`sync` 拒绝并保留既有 target，`check` 与 `status` 回报读取错误而非空计数，所有 `update`（CLI、Dashboard、`install --update`）即使 `--force` 也拒绝、Dashboard update-all 整体失败，source 的 Git staging 也会拒绝。恢复该文件的读取权限或移除它。同一规则也适用于 followed 条目内部：选取其下某个组的 `check` 与 `update`（`--group <name>` 或位置参数组名）在组内有目录无法读取时，以 `incomplete discovery of <entry>: <read error>` 拒绝，而不是只处理可读的部分。

## 命令显示 {#visibility}

- **Status**：`.skillfollow: N entries, M skipped`，local 启用时加 `(.local active)`，另列 prune 暂停恢复消息。JSON 的 `source.skillfollow` 含 `active`、`local_active`、`entry_count`、`followed_count`、`skipped_count`、声明 `entries`（`name`、`state`、可选 `resolved_target`、`reason`），以及可选 `warnings`/`prune_paused`。无声明或声明警告时省略此字段。
- **Doctor**：`skillfollow` 列声明状态，`skillfollow_prune` 列清理阻挡。未声明链接保持 `undeclared_source_links` info。Git repo 内还检查 indexed/`not-ignored` 链接与不安全的 local 文件，不修改文件。
- **`list --no-tui`**：followed tracked repo 加 `→ <resolved>`（家目录可缩为 `~`）；skills 路径仍为逻辑路径，JSON 格式不变。
- **Diff**：以与 sync 相同的规则预览。声明条目不可用时不报告任何移除，显示 `<target>: prune paused; unavailable .skillfollow entry: <name> (<state>)`，sync 会保留的 standard naming managed copy 列为 **Kept**。`diff --json` 逐 target 加上 `prune_paused` 与 `keep` 条目。Dashboard diff 加上 `prune_paused`，保留的 copy 显示为 `skip`。它也按 sync 的 prune 规则预览 followed orphan link：指向 followed 条目 resolved 位置的 managed merge link，在其 skill 退出 discovery 后列为 `prune`；你自行建立的同目标 link 列为 `local`。
- **Dashboard**：Skills、Overview、Check、Update、Audit、Hub 可见逻辑路径（audit 通过 resolved root 扫描 followed skill）。内容编辑、卸载、启停、target 覆盖、source URL 更改会拒绝。直接编辑外部树，或在 **source-root `.skillignore`** 隐藏。尚无专用声明编辑页。Dashboard sync 与 CLI 共用 prune/copy 安全，逐 target 报告 `prune_paused`/`kept` 与警告；Targets 把 managed followed link 算为 linked 而非 local。

原始诊断便于识别：

```text
_team-skills: not-ignored; add "/_team-skills" to <source>/.gitignore
_team-skills: indexed; run git rm --cached -- '_team-skills' and add "/_team-skills" to <source>/.gitignore
.skillfollow.local: tracked; run git rm --cached -- .skillfollow.local
.skillfollow.local: not-ignored; add "/.skillfollow.local" to <source>/.gitignore
```

## 清理安全 {#cleanup}

**任意声明条目**不可用（除了 `followed`/`not-link`）时，merge/copy 的全部 skills target 暂停 prune，`sync --force` 也不能覆盖，pull source 后 init 的首次 sync 亦同。仍可创建新链接/副本。Standard naming 下，无法证明来源的既有 managed copy 不替换；flat naming 可继续。Merge link 可替换，但警告条目恢复时可能名称冲突。

Status/doctor 对每个阻挡显示：

```text
prune paused: <name> is <state>; restore or fix <path>, or remove <name> from .skillfollow[.local], to resume cleanup
```

修复条目，或从**每个含此名称的声明文件**移除（`skillshare unfollow <name>` 会完成这一步并同时移除链接），随后再 sync。废弃声明让清理无限期暂停；移除声明不删除外部树。通过逻辑 source 的 managed orphan link 可清理；直接指向已取消跟随外部路径的 managed link 则保留并警告 `managed link resolves outside the source after unfollow; remove it or re-run with --force`。

## 更新安全 {#updates}

CLI、Dashboard（含 all/streaming）、`install --update` 共用 followed tracked repo 策略：干净树与 **fast-forward-only** pull（`--ff-only --no-rebase`）。明确 `--force` 在 dry run 也拒绝。Dirty、status-check error、fast-forward 失败（含分歧）为逐项失败，提供 ``resolve in `<resolved path>` ``；其他 batch 条目继续。在外部 repo 解决，不要用 force 重试；普通 installed repo 策略不变。

followed entry 之下的普通 skill 不会被重新安装。`update` 在所有选择方式（`--all`、名称、glob、group、project mode、dry run）与 Dashboard 单项更新、update-all 中，把每一项标为失败 `followed repository update refused: skill <path> is inside followed entry <name>`，其他条目继续。只有 followed repository 本身会依上述策略更新。

**Audit 失败仍 hard-reset 到 pull 前 commit。** Audit 扫描 resolved root、报告逻辑路径，scan error 也阻挡。更新期间不要编辑 repo、重新指向链接或同时运行 Git：检查是 snapshot，不是 lock，rollback 可丢失并发更改。Skillshare 外的 pull/编辑不会自动 audit；自行运行 `skillshare audit`。

## Source Git 安全 {#git-safety}

`commit`、`push`、其 dry run、Dashboard staging、init source commit 拒绝 Git 可到达的 indexed/未忽略声明链接。按 doctor 指示加入精确无末尾斜杠 ignore 并 `git rm --cached`；不会自动取消跟踪。Guard 按物理 Git 可达性判断，不会仅因 skills 声明而阻挡无关 agents/extras repo。

Source **pull/reset/checkout** 也拒绝 indexed 声明（包括不存在但仍 indexed 的链接），或 incoming revision 触碰含任何工作目录链接组件的路径，**无论是否声明**。Ignore 不足以防止 Git 替换链接；错误列 commit/path，按指示取消跟踪并 ignore，或先修正 remote。Pull fetch 后检查固定 revision；Dashboard checkout 检查选定既有 local/remote-tracking revision，不增加隐式 fetch。Dashboard discard 保留 ignored followed link。

这些 guard 只保护 skillshare 操作，不保护自行执行的 Git。

## 限制

单 skill 与声明编辑页仍是未来工作；`follow --to` 在 Windows 上通过 sync 使用的同一个 helper 创建 junction，这条路径同样尚未在真实 Windows 上运行过；嵌套链接不跟随。Windows 有 junction 模拟与 cross-compilation，但**尚未验证真实 Windows junction/Developer Mode 的完整功能运行矩阵**；编译或早期 standalone probe 成功不等于 runtime 正确。

## 另见

- [过滤](./filtering.md#skillignore)
- [Source 与 Targets](../understand/source-and-targets.md)
- [Update](./commands/update.md)
- [Sync](./commands/sync.md)

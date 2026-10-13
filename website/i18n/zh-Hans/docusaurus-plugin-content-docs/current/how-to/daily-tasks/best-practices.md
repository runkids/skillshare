---
sidebar_position: 6
---

# 最佳实践

Skill 的命名规范、组织方式与版本控制。

## 命名

### Skill 名称

**推荐做法：**
- 使用小写加连字符：`code-review`、`pdf-tools`
- 具有描述性：使用 `react-component-generator` 而不是 `rcg`
- 为团队加上命名空间：`acme-code-review`

**避免做法：**
- 使用空格或特殊字符
- 使用通用名称：`helper`、`utils`、`tools`
- 与常见 Skill 名称冲突

### 仓库名称

**个人使用：**
```
my-skills
ai-skills
```

**团队使用：**
```
<team>-skills
<org>-skills
```

---

## 组织方式

### 个人 Skill

```
~/.config/skillshare/skills/
├── code-review/
├── pdf-tools/
├── git-workflow/
└── _team-skills/      # Tracked repo
```

### 团队仓库

```
team-skills/
├── frontend/
│   ├── react/
│   ├── vue/
│   └── testing/
├── backend/
│   ├── api/
│   └── database/
├── devops/
│   ├── deploy/
│   └── monitoring/
└── README.md
```

### Skill 目录结构

```
my-skill/
├── SKILL.md           # 必需
├── README.md          # 可选：给人类阅读
├── examples/          # 可选：示例文件
└── templates/         # 可选：代码模板
```

---

## 版本控制

### 提交信息

遵循 conventional commits 规范：
```
feat(code-review): add security check
fix(pdf-tools): handle empty files
docs(readme): update installation
```

### 分支策略

**个人使用：**
- 单一 `main` 分支即可
- 实验性内容使用分支

**团队使用：**
- `main` 分支用于稳定的 Skill
- 开发使用功能分支
- 合并前需经过 PR review

### 标签

为稳定版本打标签：
```bash
git tag v1.0.0
git push --tags
```

---

## 编写 Skill

### 结构

```markdown
---
name: skill-name
description: One-line description
---

# Skill Name

Brief overview.

## When to Use

Clear trigger conditions.

## Instructions

1. Step one
2. Step two

## Examples

Concrete input/output examples.

## When NOT to Use

Explicit exclusions.
```

### License

为发布的 Skill 添加 `license` 字段——这在企业环境中尤为重要：

```yaml
---
name: code-review
description: Reviews code for quality
license: MIT
---
```

这会在 `skillshare install` 时显示出来，方便用户做出合规决策。

### 内容

**推荐做法：**
- 编写清晰、可执行的指令
- 包含示例
- 说明边界情况
- 保持专注（一个 Skill = 一个用途）

**避免做法：**
- 编写含糊的指令
- 塞入过多职责
- 忽略错误处理
- 跳过测试

---

## 团队协作

### 为特定仓库的 Skill 使用 Project mode（`-p`）

当 Skill 与某个代码库紧密耦合（架构、领域规则、部署流程）时，优先使用 Project mode：

```bash
skillshare init -p
skillshare install <source> -p
skillshare sync
```

**为什么这有帮助：**
- **可复现的入职流程**：`.skillshare/config.yaml` 相当于一份可移植的 Skill 清单，方便任何 clone 该仓库的人使用。
- **范围清晰**：project 的 Skill 保存在 `.skillshare/skills/` 中，不会渗入全局的个人工作流。
- **更安全的协作**：变更通过与项目代码相同的常规 git PR 流程进行审查。
- **减少提交噪音**：`.skillshare/logs/` 在 project mode 下默认被忽略。

个人跨项目使用的 Skill 用 Global mode；仓库专属的团队上下文用 `-p`。

### 为内部工具使用 .skillignore

如果你的团队仓库包含内部工具或开发中的 Skill，添加一个 `.skillignore` 以防止被意外发现：

```text title=".skillignore"
# Hide from public discovery
_internal-scripts
test-*
wip-feature
```

这可确保外部贡献者或运行 `skillshare install <repo> --all` 的自动化流程不会拉取到内部 Skill。

**用 `.skillignore.local` 做本地覆盖**：如果共享仓库的 `.skillignore` 屏蔽了你本地需要的某个 Skill，可以在同一目录下创建 `.skillignore.local`，在不修改共享文件的情况下覆盖它：

```text title="_team-skills/.skillignore.local"
# Un-ignore my own private skill
!private-mine
```

将 `.skillignore.local` 加入你的 `.gitignore`——它本来就只应留在本地。

### 责任归属

- 为 Skill 分类指定负责人
- 在 README 中记录谁维护哪部分
- 合并前先审查 PR

### 文档

```
team-skills/
├── README.md           # Setup instructions
├── CONTRIBUTING.md     # How to add skills
├── CHANGELOG.md        # What changed
└── skills/
    └── ...
```

### 沟通

- 在团队聊天中宣布新的 Skill
- 记录破坏性变更
- 收集用户反馈

---

## 维护

### 常规任务

```bash
# Weekly
skillshare update --all     # Update tracked repos
skillshare doctor           # Check for issues
skillshare backup --cleanup # 移除旧快照和遗留的不完整目录

# Monthly
skillshare list             # Review installed skills
# Remove unused: skillshare uninstall <name>...
```

### 清理不再使用的 Skill

```bash
# List all skills
skillshare list

# Remove ones you don't use
skillshare uninstall unused-skill
skillshare sync
```

### 更新依赖

```bash
# Update CLI
skillshare upgrade --cli

# Update built-in skill
skillshare upgrade --skill

# Update tracked repos
skillshare update --all
```

---

## 安全

### 敏感信息

**绝不放入 Skill 的内容：**
- API 密钥
- 密码
- 个人信息
- 内部 URL

**替代做法：**
- 使用环境变量
- 引用外部配置
- 保持 Skill 内容通用

### 安装前先审查

安装第三方 Skill 之前：
- 检查来源
- 阅读 SKILL.md
- 先使用 `--dry-run`

关于完整的安全工作流程，参见 [保护你的 Skill](/docs/how-to/advanced/security) 指南。

---

## 检查清单

### 新建 Skill

- [ ] 名称具有描述性
- [ ] 描述清晰
- [ ] 指令可执行
- [ ] 包含示例
- [ ] 已在 AI CLI 中测试
- [ ] 无名称冲突

### 团队仓库

- [ ] 清晰的文件夹结构
- [ ] README 包含 setup 说明
- [ ] Skill 名称已加命名空间
- [ ] 内部工具已配置 `.skillignore`
- [ ] 有 PR 审查流程
- [ ] 维护 CHANGELOG

---

## 另请参阅

- [Creating Skills](./creating-skills.md) — Skill 创建指南
- [Skill Design](/docs/understand/philosophy/skill-design) — 复杂度分级、确定性、CLI 封装模式
- [Skill Format](/docs/understand/skill-format) — SKILL.md 参考
- [Organization-Wide Skills](/docs/how-to/sharing/organization-sharing) — 团队共享模式

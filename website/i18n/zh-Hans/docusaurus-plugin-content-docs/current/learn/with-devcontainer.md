---
sidebar_position: 5
---

# 在 Dev Containers 中使用 skillshare

> 在 VS Code 中打开，Skill 即已就绪——无需在本机安装。

## 前置条件

- 安装了 [Dev Containers extension](https://marketplace.visualstudio.com/items?itemName=ms-vscode-remote.remote-containers) 的 [VS Code](https://code.visualstudio.com/)

## 运作方式

VS Code Dev Containers 让你可以在 Docker 容器内进行开发。你在 `.devcontainer/` 中定义环境，其余交给 VS Code 处理——打开项目、点击「Reopen in Container」，一切就绪。

skillshare 能自然融入这个工作流。将它加入 `postCreateCommand`，容器启动时 Skill 就会自动安装并 Sync。

## 设置

在你的 `.devcontainer/devcontainer.json` 中加入两项内容：

```json
{
  "postCreateCommand": "curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh && export PATH=\"$HOME/.local/bin:$PATH\" && skillshare init --no-copy --all-targets --no-git --no-skill && skillshare sync"
}
```

这里的 PATH 设置仅适用于这次设置命令。请把 `export PATH="$HOME/.local/bin:$PATH"` 加入容器内的 shell 配置文件（例如 `~/.bashrc`），让之后打开的终端也能使用。

就这样。当团队成员在 VS Code 中打开项目并点击「Reopen in Container」时：

1. skillshare 会自动安装
2. `init` 不提问——加入所有检测到的 AI CLI Target，并跳过导入已有 Skill、git 与内置 Skill
3. `sync` 将 Skill 分发到所有 Target

## 添加 Project Skill

对于团队共享的 Skill，将 `.skillshare/` 配置提交到仓库：

```bash
# 在容器内
skillshare init -p
skillshare install your-org/team-skills -p
```

然后提交变更，并更新 `postCreateCommand`，让它同时 Sync project skills：

```json
{
  "postCreateCommand": "curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh && export PATH=\"$HOME/.local/bin:$PATH\" && skillshare init --no-copy --all-targets --no-git --no-skill && skillshare sync && skillshare sync -p"
}
```

现在每位团队成员打开容器时，都会获得相同的 Skill。

## GitHub Codespaces

同一份 `.devcontainer/` 配置在 Codespaces 中无需任何修改即可使用。Codespaces 运行 `postCreateCommand` 的方式与 VS Code 相同。

## 使用 ssenv 进行隔离测试

在 devcontainer 内，`ssenv` 让你可以建立隔离的 skillshare 环境以进行并行测试。每个环境都拥有独立的 `HOME` 目录，各自拥有独立的配置、Skill 与 Target。

| 命令 | 作用 |
|---------|-------------|
| `ssnew <name>` | 创建一个新的隔离环境 |
| `ssuse <name>` | 切换到某个环境 |
| `ssback` | 返回原始环境 |
| `ssls` | 列出所有环境 |
| `ssrm <name>` | 删除一个环境 |

```bash
ssnew demo && ssuse demo    # 创建并切换
ss init && ss sync          # 命令在隔离环境中运行
ssback                      # 返回原始环境
```

这对于在不影响主要设置的情况下测试配置变更或 Skill 安装非常有用。

## 接下来？

- [Project Skill 设置 →](/docs/how-to/sharing/project-setup)
- [团队共享 →](/docs/how-to/sharing/organization-sharing)
- [Sync 模式详解 →](/docs/understand/philosophy/sync-modes-explained)

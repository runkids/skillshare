---
sidebar_position: 2
---

# 데스크톱 앱

**[Skillshare App](https://github.com/runkids/skillshare-app)**으로 스킬, 에이전트, MCP, hooks를 데스크톱 창에서 관리하세요. macOS, Windows, Linux에서 skillshare 대시보드를 사용할 수 있으며, 첫 사용을 위한 설정 안내도 제공합니다.

**[Skillshare App 다운로드 →](https://github.com/runkids/skillshare-app/releases/latest)**

## macOS에 설치

Apple Silicon Mac에서는 Homebrew 설치를 권장합니다:

```bash
brew tap runkids/tap
brew install --cask skillshare-app
```

설치 후 응용 프로그램에서 **skillshare**를 여세요. [최신 릴리스](https://github.com/runkids/skillshare-app/releases/latest)의 `.dmg`로 직접 설치할 수도 있습니다.

## Windows 또는 Linux에 설치

[최신 앱 릴리스](https://github.com/runkids/skillshare-app/releases/latest)에서 설치 파일을 선택하세요:

| 플랫폼 | 설치 파일 |
|---|---|
| Windows(x64) | `.exe` 또는 `.msi` |
| Linux(x64) | `.deb`, `.AppImage` 또는 `.rpm` |

시스템에 맞는 패키지를 설치한 다음 Skillshare App을 여세요.

## 첫 실행

앱은 skillshare CLI를 사용해 작동합니다. 첫 설정에서 다음 과정을 안내합니다:

1. CLI를 설치하거나 기존 실행 파일을 선택합니다.
2. 동기화할 AI 도구를 선택합니다.
3. 첫 동기화를 실행한 다음 대시보드를 엽니다.

대시보드에서 스킬을 찾아 설치하고, 동기화 전에 변경 사항을 미리 보고, 에이전트, MCP, hooks를 관리할 수 있습니다. 데스크톱 앱과 [`skillshare ui`](../reference/commands/ui.md)는 같은 대시보드를 사용합니다.

## 터미널이 더 편하다면

CLI는 터미널 작업과 자동화에도 계속 사용할 수 있습니다. [CLI 첫 동기화 가이드](./first-sync.md)로 시작하거나 [빠른 참조](./quick-reference.md)에서 명령을 찾아보세요.

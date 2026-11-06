---
sidebar_position: 6
---

# version

현재 skillshare 버전을 표시합니다.

## 언제 사용하나요

- 버그를 리포트하기 전에 실행 중인 버전을 확인할 때
- 업그레이드가 성공했는지 확인할 때
- [최신 릴리스](https://github.com/runkids/skillshare/releases)와 버전을 비교할 때

## 사용법

```bash
skillshare version
skillshare -v
skillshare --version
```

## 출력 예시

```
skillshare version 0.16.6
```

## 업데이트 알림

새 버전을 사용할 수 있을 때, `skillshare`는 이를 지원하는 명령 실행 후 업데이트 알림을 표시합니다. 이 알림은 **Homebrew를 인식**합니다. skillshare가 Homebrew로 설치된 경우 `brew info`를 조회해 최신 formula 버전을 확인하고 `brew upgrade skillshare`를 제안하며, 그렇지 않으면 GitHub 릴리스를 확인하고 `skillshare upgrade`를 제안합니다.

감지는 자동으로 이루어집니다. skillshare는 자체 실행 파일 경로를 확인하여 Homebrew Cellar 접두사(예: `/opt/homebrew/Cellar/skillshare/`) 아래에 있는지 확인합니다. `/nix/store/` 아래의 바이너리는 GitHub 릴리스를 확인합니다. 바이너리가 `nix profile`에 있으면 `nix profile upgrade skillshare`를 제안하고, 그렇지 않으면 Nix 설정에서 skillshare를 업데이트하고 다시 빌드하라고 안내합니다.

버전 확인 결과는 `~/.cache/skillshare/version-check.json`에 24시간 동안 캐시됩니다.

## 참고 항목

- [upgrade](./upgrade.md) — 최신 버전으로 업그레이드
- [doctor](./doctor.md) — 전체 환경 진단
- [status](./status.md) — 버전 정보를 포함한 동기화 상태 표시

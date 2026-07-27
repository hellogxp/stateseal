# 시작하기

[English](../getting-started.md) | 한국어

일반적인 StateSeal 흐름은 컴퓨터에 한 번 설치하고, Agent를 한 번 통합하고,
프로젝트의 검증 계약을 확인한 뒤 작업 목표를 설명하는 것입니다.

GitHub Actions가 확정된 Git tag에서 빌드한 릴리스를 설치합니다. 설치 프로그램은
플랫폼을 선택하고 공개된 SHA-256 체크섬을 검증합니다.

```bash
curl --proto '=https' --tlsv1.2 -fsSL https://github.com/hellogxp/stateseal/releases/latest/download/install.sh | sh
export PATH="$HOME/.local/bin:$PATH"
seal version
codex plugin marketplace add /path/to/stateseal
codex plugin add stateseal@stateseal
```

Plugin에는 Skill과 MCP 등록이 포함되며 설치된 Core를 사용하므로 MCP를 별도로
설치할 필요가 없습니다. `marketplace add`는 로컬 checkout만 등록하고 게시하거나
업로드하지 않습니다. 새 Codex 작업의 일반 코드 변경 요청은 StateSeal을 자동으로,
그리고 보이게 활성화합니다.

Plugin을 사용할 수 없는 CLI/headless 환경에서는 저장소를 명시하세요.

```bash
seal run --repo /path/to/project "입력 검증을 추가하고 호환성을 유지하며 테스트 포함"
```

Git 저장소가 아닌 상위 Workspace에서는 하위 저장소를 탐색합니다. 여러 저장소가
있으면 `seal workspace list`로 확인하고 `--repo`로 명시적으로 선택합니다.

최초 프로젝트 계약과 최종 Apply는 명시적으로 확인합니다. StateSeal 자체의 확인,
MCP, Store 또는 격리 Worker를 사용할 수 없더라도 일반 Agent 개발은 중단되지
않으며 결과는 Receipt 없이 `UNVERIFIED`로 표시됩니다. 정상적인 enforce 정책의
실제 검증 거부는 계속 유효합니다.

익명 아티팩트 또는 소스 체크아웃에서는 Git과 Go 1.24 이상으로 `make install`을
실행합니다.

첫 실행에서 StateSeal은 감지된 admission, completion 명령과 보호 경로를 보여 줍니다.
해당 명령이 최소 전달 게이트로 적절할 때만 확인하세요.

StateSeal은 격리된 proposal을 만들고 후보를 독립적으로 평가하며 마지막 검증
체크포인트를 보존하고 종료 시 새 evaluator에서 재인증합니다. `ADMITTED` 이후에도
사용자가 명시적으로 적용하기 전까지 원본 브랜치는 변경되지 않습니다.

```bash
seal ui
```

Runs Console에서 모든 저장소의 상태 출처, 검증 증거, 이벤트 및 완료 영수증을
확인할 수 있습니다. 자세한 내용은 [Runs Console](runs-console.md)을 참조하세요.

`ADMITTED`는 `seal.yaml`에 구성된 검사만 포함합니다. 사양 완전성, 원격 CI 및
호스트 신뢰는 잔여 위험입니다.

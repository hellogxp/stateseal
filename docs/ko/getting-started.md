# 시작하기

[English](../getting-started.md) | 한국어

일반적인 StateSeal 흐름은 컴퓨터에 한 번 설치하고, Agent를 한 번 통합하고,
프로젝트의 검증 계약을 확인한 뒤 작업 목표를 설명하는 것입니다.

```bash
seal version
seal integrate codex-desktop
seal integrate status

cd /path/to/project
seal run "입력 검증을 추가하고 호환성을 유지하며 테스트 포함"
```

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

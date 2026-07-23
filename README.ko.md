# StateSeal

[English](README.md) · [简体中文](README.zh-CN.md) · [日本語](README.ja.md) ·
**한국어** · [Español](README.es.md) · [Português do Brasil](README.pt-BR.md) ·
[Deutsch](README.de.md) · [Français](README.fr.md)

StateSeal은 Coding Agent의 후보 코드를 정확한 코드 상태에 바인딩되고 독립적으로
검증되며 복구, 재인증, 감사가 가능한 전달 결과로 변환합니다. 검증 범위와 잔여 위험도
명확히 공개합니다.

> 모델이 변경을 제안하고, 증거가 판단을 지원하며, Broker가 승인을 결정하고, Git이 기록을 보존하고, 사용자가 적용합니다.

StateSeal은 또 다른 Coding Agent가 아니며 테스트를 대체하지 않습니다. 기존 Agent와
검증 명령을 감싸 최종 전달 코드가 실제로 검증된 코드인지 확인합니다.

## 필요한 이유

장시간 실행되는 Agent는 테스트 통과 후에도 편집을 계속해 회귀를 만들고 성공을
보고할 수 있습니다. StateSeal은 후보, 증거, 체크포인트, 재인증 및 승인을 명시적인
프로토콜 상태로 만듭니다.

```text
WORKING → CANDIDATE → VERIFYING → VERIFIED → RECERTIFYING → ADMITTED
                           ↘ REJECTED                 ↘ STALE / ABSTAINED
```

## 빠른 시작

```bash
go install github.com/hellogxp/stateseal/cmd/seal@latest
cd your-project
seal integrate codex-desktop
seal run "중복 콜백을 수정하고 호환성을 유지"
seal ui
```

Runs Console은 모든 저장소의 실행 목록, 상태 출처 DAG, 검증 증거, 신뢰할 수 있는
이벤트 타임라인, 체크포인트 복구 및 완료 영수증을 제공합니다. UI는 읽기 전용이며
루프백 주소에서만 수신합니다.

## 신뢰 경계

`ADMITTED`는 정확한 체크포인트가 `seal.yaml`의 검증 정책을 충족했다는 의미입니다.
요구사항이나 테스트의 완전성, 호스트의 무결성까지 증명하지는 않습니다.

## 문서

- [한국어 문서](docs/ko/index.md)
- [시작하기](docs/ko/getting-started.md)
- [Runs Console](docs/ko/runs-console.md)
- [영문 기술 문서](docs/index.md)

프로토콜 값, 명령, digest와 로그는 감사 의미를 보존하기 위해 영어 원문을 유지합니다.

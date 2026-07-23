# Runs Console

[English](../runs-console.md) | 한국어

`seal ui`는 외부 authority state를 읽기 전용으로 시각화하는 로컬 관측 화면입니다.

```bash
seal ui
seal ui --no-open
seal ui --address 127.0.0.1:9137
```

목록에서 모든 저장소의 실행을 검색하고 필터링할 수 있습니다. 상세 화면에는 다음이
포함됩니다.

- 후보, 검증 증거, 체크포인트, 복구, 결정, 적용의 상태 출처 DAG
- 정확한 코드 상태에 바인딩된 검증 실행
- 해시 체인이 검증된 이벤트 타임라인
- verdict, rule, disposition 및 checkpoint가 포함된 완료 영수증

단순한 실행은 빈 계층을 자동 압축합니다. 긴 노드 텍스트는 경계 안에서 생략되고
전체 내용은 툴팁과 Inspector에 표시됩니다.

UI는 8개 언어를 지원하며 브라우저 언어를 자동 감지합니다. Goal, 프로토콜 값,
ID, digest 및 로그는 감사 원문을 유지합니다.

서버는 loopback에만 바인딩되고 쓰기 API가 없습니다. 이벤트 해시 연속성이 실패하면
해당 타임라인은 신뢰할 수 있는 것으로 표시되지 않습니다.

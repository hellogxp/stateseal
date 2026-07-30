# 시작하기

```bash
make install
export PATH="$HOME/.local/bin:$PATH"
seal ui
```

StateSeal은 로컬 Codex 세션을 읽기 전용으로 탐색합니다. Hook, MCP, 승인 또는
Agent 제어가 필요하지 않습니다. `COMPLETED`는 Agent가 완료를 보고했다는 뜻일
뿐 코드의 정확성을 보장하지 않습니다.

분석은 `Observed`, `Derived`, `Inferred`로 구분됩니다. 자세한 내용은
[영문 가이드](../getting-started.md)를 참고하세요.

# StateSeal

[English](README.md) · [简体中文](README.zh-CN.md) · [日本語](README.ja.md) ·
**한국어** · [Español](README.es.md) ·
[Português do Brasil](README.pt-BR.md) · [Deutsch](README.de.md) ·
[Français](README.fr.md)

StateSeal은 Coding Agent를 위한 **읽기 전용 전달 인텔리전스 계층**입니다.
요청, 관측된 코드 상태, 검사, 산출물, 실패 및 Agent의 완료 보고를 실시간
파노라마로 재구성합니다.

Agent를 시작하거나 제어하거나 차단하거나 승인하거나 Apply하지 않습니다. Hook을
설치하지 않고 모델 트래픽을 프록시하지 않으며 Prompt나 브랜치를 변경하지 않습니다.

```bash
make install
export PATH="$HOME/.local/bin:$PATH"
seal ui
```

모든 분석은 `Observed`(직접 관측), `Derived`(결정적 연관),
`Inferred`(틀릴 수 있는 진단 가설)로 구분됩니다.

자세한 내용은 [English documentation](docs/index.md)을 참고하세요.

# StateSeal

[English](README.md) · [简体中文](README.zh-CN.md) · [日本語](README.ja.md) ·
[한국어](README.ko.md) · [Español](README.es.md) ·
**Português do Brasil** · [Deutsch](README.de.md) · [Français](README.fr.md)

StateSeal é uma **camada de inteligência de entrega somente leitura** para
agentes de programação. Ela reconstrói solicitações, estados de código
observados, verificações, artefatos, falhas e a declaração de conclusão do Agent.

Ela nunca inicia, orienta, bloqueia, aprova ou aplica o trabalho do Agent. Não
instala hooks, não atua como proxy do modelo e não altera prompts ou branches.

```bash
make install
export PATH="$HOME/.local/bin:$PATH"
seal ui
```

Cada afirmação é marcada como `Observed` (observada), `Derived` (correlação
determinística) ou `Inferred` (hipótese de diagnóstico que pode estar errada).

Consulte a [documentação em inglês](docs/index.md).

# StateSeal

[English](README.md) · [简体中文](README.zh-CN.md) · [日本語](README.ja.md) ·
[한국어](README.ko.md) · [Español](README.es.md) · **Português do Brasil** ·
[Deutsch](README.de.md) · [Français](README.fr.md)

O StateSeal transforma candidatos produzidos por Coding Agents em resultados de
entrega vinculados ao estado exato do código, verificados de forma independente,
recuperáveis, recertificáveis e auditáveis. A cobertura da verificação e os
riscos residuais também ficam explícitos.

> O modelo propõe. A evidência informa. O Broker decide. O Git registra. O usuário aplica.

O StateSeal não é outro Coding Agent e não substitui seus testes. Ele envolve o
Agent e os comandos de verificação existentes para garantir que o código
entregue seja exatamente o código verificado.

![Pipeline de entrega confiável do StateSeal](docs/assets/stateseal-trust-pipeline.svg)

## Capacidades principais

| Capacidade | Valor |
| --- | --- |
| Vínculo ao estado exato | Associa cada resultado à árvore de código que o produziu |
| Verificação independente | Executa a política em um evaluator limpo sem confiar no autorrelato do Agent |
| Recuperação de checkpoint | Preserva o último estado confiável quando um candidato posterior regride |
| Admissão externa | O Broker decide com base em evidência, cobertura e política |
| Recibos auditáveis | Registra proveniência, origem da evidência e risco residual |

## Por quê

Um Agent de longa duração pode passar nos testes, continuar editando, introduzir
uma regressão e ainda informar sucesso. O StateSeal torna candidatos, evidências,
checkpoints, recertificação e admissão estados explícitos do protocolo.

```text
WORKING → CANDIDATE → VERIFYING → VERIFIED → RECERTIFYING → ADMITTED
                           ↘ REJECTED                 ↘ STALE / ABSTAINED
```

## Início rápido

Instale a versão criada pelo GitHub Actions a partir de um Git tag determinado.
O instalador seleciona a plataforma e verifica o checksum SHA-256.

```bash
curl --proto '=https' --tlsv1.2 -fsSL https://github.com/hellogxp/stateseal/releases/latest/download/install.sh | sh
export PATH="$HOME/.local/bin:$PATH"
seal version
cd your-project
seal integrate codex-desktop
seal run "Corrigir callbacks duplicados e preservar a compatibilidade"
seal ui
```

Para avaliação anônima ou checkout do código-fonte, execute `make install` com
Git e Go 1.24 ou superior.

O Runs Console mostra execuções de todos os repositórios, o DAG de proveniência,
evidências do verificador, linha do tempo confiável, recuperação e recibos. A UI
é somente leitura e escuta apenas no endereço de loopback.

![Runs Console do StateSeal](docs/assets/stateseal-runs-console.svg)

## Integrações e automação

O StateSeal integra Codex Desktop, Codex CLI, Claude Code e Qoder. Outros agentes
de terminal usam o mesmo protocolo com `seal run -- <command>`. Saída JSON,
identificadores estáveis de regras e recibos apoiam CI e reprodução de pesquisa.

## Limite de confiança

`ADMITTED` significa apenas que o checkpoint exato atendeu à política em
`seal.yaml`. Não prova que a especificação ou os testes estejam completos nem
que o host seja íntegro.

## Documentação

- [Documentação em português](docs/pt-BR/index.md)
- [Primeiros passos](docs/pt-BR/getting-started.md)
- [Runs Console](docs/pt-BR/runs-console.md)
- [Referência técnica em inglês](docs/index.md)

Valores de protocolo, comandos, digest e logs permanecem em inglês para
preservar o significado de auditoria.

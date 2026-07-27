# Primeiros passos

[English](../getting-started.md) | Português do Brasil

O fluxo normal é instalar o StateSeal uma vez, integrar cada Agent, confirmar o
contrato de verificação do projeto e descrever o objetivo da tarefa.

Instale a versão criada pelo GitHub Actions a partir de um Git tag determinado.
O instalador seleciona a plataforma e verifica o checksum SHA-256 publicado.

```bash
curl --proto '=https' --tlsv1.2 -fsSL https://github.com/hellogxp/stateseal/releases/latest/download/install.sh | sh
export PATH="$HOME/.local/bin:$PATH"
seal version
seal integrate codex-desktop
seal integrate status

cd /path/to/project
seal run "Adicionar validação de entrada, preservar compatibilidade e incluir testes"
```

Em um artefato anônimo ou checkout do código-fonte, execute `make install` com
Git e Go 1.24 ou superior.

Na primeira execução, o StateSeal mostra os comandos de admission e completion
detectados e os caminhos protegidos. Confirme apenas se eles forem uma barreira
mínima de entrega apropriada.

O StateSeal cria uma proposal isolada, avalia candidatos independentemente,
preserva o último checkpoint verificado e o recertifica em um evaluator novo.
A branch original só muda quando o usuário aplica explicitamente um resultado
`ADMITTED`.

```bash
seal ui
```

O Runs Console permite inspecionar proveniência, evidências, eventos e recibos
de todos os repositórios. Consulte [Runs Console](runs-console.md).

`ADMITTED` cobre apenas as verificações de `seal.yaml`. A integridade do host, a
completude da especificação e a CI remota permanecem riscos residuais.

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
codex plugin marketplace add /path/to/stateseal
codex plugin add stateseal@stateseal
```

O Plugin inclui o Skill e o registro MCP e usa o Core instalado; não é
necessário instalar o MCP separadamente. `marketplace add` apenas registra o
checkout local: não publica nem envia código. Em uma nova tarefa do Codex,
pedidos comuns de alteração ativam o StateSeal de forma automática e visível.

Sem o Plugin, em CLI/headless selecione o repositório explicitamente:

```bash
seal run --repo /path/to/project "Adicionar validação de entrada, preservar compatibilidade e incluir testes"
```

Em um Workspace pai que não seja Git, o StateSeal descobre repositórios filhos.
Se houver vários, liste-os com `seal workspace list` e selecione com `--repo`.

O primeiro contrato do projeto e o Apply final exigem confirmação explícita.
Se a confirmação, o MCP, o Store ou o worker isolado do próprio StateSeal
falharem, o desenvolvimento normal do Agent continua, o resultado é marcado
como `UNVERIFIED` e nenhum recibo é emitido. Uma rejeição real de uma política
enforce saudável continua vinculante.

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

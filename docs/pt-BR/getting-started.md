# Primeiros passos

```bash
make install
export PATH="$HOME/.local/bin:$PATH"
seal ui
```

O StateSeal descobre sessões locais do Codex em modo somente leitura. Não
requer hooks, MCP, aprovações ou controle do Agent. `COMPLETED` significa apenas
que o Agent relatou o fim da sessão; não garante que o código esteja correto.
A visão mostra estados do código, alterações, atualidade das verificações,
artefatos, diagnósticos e a linha do tempo de auditoria.

A análise distingue `Observed`, `Derived` e `Inferred`. Consulte o
[guia em inglês](../getting-started.md).

# Runs Console

[English](../runs-console.md) | Português do Brasil

`seal ui` é uma visão local somente leitura do estado de autoridade externo.

```bash
seal ui
seal ui --no-open
seal ui --address 127.0.0.1:9137
```

A lista permite buscar e filtrar execuções de todos os repositórios. O detalhe
inclui:

- DAG de candidato, evidência, checkpoint, recuperação, decisão e aplicação;
- execuções do verificador vinculadas ao estado exato do código;
- linha do tempo com cadeia de hash validada;
- recibo com verdict, rule, disposition e checkpoint.

Fluxos simples comprimem camadas vazias. Textos longos ficam contidos no nó e
aparecem por completo no tooltip e no Inspector.

A UI suporta oito idiomas e detecta o idioma do navegador. Goal, valores do
protocolo, IDs, digest e logs preservam o texto original de auditoria.

O servidor escuta apenas em loopback e não oferece API de escrita. Se a
continuidade do hash falhar, a linha do tempo não é apresentada como confiável.

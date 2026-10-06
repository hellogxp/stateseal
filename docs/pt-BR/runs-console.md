# Panorama de entrega

`seal ui` é uma visão somente leitura das entregas locais do Agent. Ela mostra
o estado atual do código, arquivos alterados, verificações vinculadas a cada
estado, evidência desatualizada, artefatos, diagnósticos e a linha de auditoria.

```text
Solicitação → estados observados → verificações / artefatos → declaração do Agent
```

`Observed` é fato direto, `Derived` é correlação reproduzível e `Inferred` é
hipótese. Não existem ações para bloquear, aprovar ou aplicar trabalho.

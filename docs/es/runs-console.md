# Runs Console

[English](../runs-console.md) | Español

![Runs Console de StateSeal](../assets/stateseal-runs-console.svg)

`seal ui` es una vista local de solo lectura del estado de autoridad externo.

```bash
seal ui
seal ui --no-open
seal ui --address 127.0.0.1:9137
```

La lista permite buscar y filtrar ejecuciones de todos los repositorios. El
detalle incluye:

- DAG de candidato, evidencia, punto de control, recuperación, decisión y aplicación;
- ejecuciones del verificador vinculadas al estado exacto del código;
- cronología con cadena hash validada;
- recibo con verdict, rule, disposition y checkpoint.

Los flujos simples comprimen las capas vacías. El texto largo queda limitado al
nodo y se muestra completo en el tooltip y el Inspector.

La UI admite ocho idiomas y detecta el idioma del navegador. Goal, valores del
protocolo, ID, digest y logs conservan el texto original de auditoría.

El servidor solo se vincula a loopback y no expone API de escritura. Si falla
la continuidad hash, la cronología afectada no se presenta como confiable.

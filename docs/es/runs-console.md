# Panorama de entrega

`seal ui` es una vista de solo lectura de las entregas locales del Agent. Muestra
el estado actual del código, archivos cambiados, comprobaciones ligadas a cada
estado, evidencia obsoleta, artefactos, diagnósticos y la línea de auditoría.

```text
Solicitud → estados de código observados → comprobaciones / artefactos → declaración del Agent
```

`Observed` es un hecho directo, `Derived` una correlación reproducible e
`Inferred` una hipótesis. No hay controles para bloquear, aprobar o aplicar trabajo.

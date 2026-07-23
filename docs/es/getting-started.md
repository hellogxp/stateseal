# Primeros pasos

[English](../getting-started.md) | Español

El flujo normal consiste en instalar StateSeal una vez, integrar cada Agent,
confirmar el contrato de verificación del proyecto y describir el objetivo.

```bash
seal version
seal integrate codex-desktop
seal integrate status

cd /path/to/project
seal run "Añadir validación de entrada, mantener compatibilidad e incluir pruebas"
```

En la primera ejecución, StateSeal muestra los comandos de admission y
completion detectados y las rutas protegidas. Confírmalos solo si forman una
puerta mínima de entrega adecuada.

StateSeal crea una propuesta aislada, evalúa candidatos independientemente,
conserva el último punto verificado y lo recertifica en un evaluator nuevo.
La rama original no cambia hasta que el usuario aplica explícitamente un
resultado `ADMITTED`.

```bash
seal ui
```

Runs Console permite inspeccionar procedencia, evidencia, eventos y recibos de
todos los repositorios. Consulta [Runs Console](runs-console.md).

`ADMITTED` solo cubre las comprobaciones de `seal.yaml`. La integridad del host,
la integridad de la especificación y la CI remota siguen siendo riesgos
residuales.

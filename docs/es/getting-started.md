# Primeros pasos

[English](../getting-started.md) | Español

El flujo normal consiste en instalar StateSeal una vez, integrar cada Agent,
confirmar el contrato de verificación del proyecto y describir el objetivo.

Instala la versión construida por GitHub Actions desde un Git tag determinado.
El instalador selecciona la plataforma y verifica la suma SHA-256 publicada.

```bash
curl --proto '=https' --tlsv1.2 -fsSL https://github.com/hellogxp/stateseal/releases/latest/download/install.sh | sh
export PATH="$HOME/.local/bin:$PATH"
seal version
seal integrate codex-desktop
seal integrate status

cd /path/to/project
seal run "Añadir validación de entrada, mantener compatibilidad e incluir pruebas"
```

Desde un artefacto anónimo o checkout del código fuente, usa `make install` con
Git y Go 1.24 o posterior.

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

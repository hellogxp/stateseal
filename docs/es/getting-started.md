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
codex plugin marketplace add /path/to/stateseal
codex plugin add stateseal@stateseal
```

El Plugin incluye el Skill y el registro MCP y utiliza el Core instalado; no
hay que instalar MCP por separado. `marketplace add` solo registra el checkout
local: no publica ni sube código. En una tarea nueva de Codex, las solicitudes
normales de cambio activan StateSeal de forma automática y visible.

Sin Plugin, en CLI/headless selecciona el repositorio explícitamente:

```bash
seal run --repo /path/to/project "Añadir validación de entrada, mantener compatibilidad e incluir pruebas"
```

Desde un Workspace padre que no sea Git, StateSeal descubre repositorios hijos.
Si hay varios, revísalos con `seal workspace list` y selecciónalos con `--repo`.

El primer contrato del proyecto y el Apply final requieren confirmación
explícita. Si fallan la confirmación, MCP, Store o el worker aislado de
StateSeal, el desarrollo normal del Agent continúa, el resultado se marca
`UNVERIFIED` y no se emite un recibo. Un rechazo real de una política enforce
sana sigue siendo vinculante.

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

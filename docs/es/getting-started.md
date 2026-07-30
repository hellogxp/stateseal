# Primeros pasos

```bash
make install
export PATH="$HOME/.local/bin:$PATH"
seal ui
```

StateSeal descubre sesiones locales de Codex en modo de solo lectura. No
requiere hooks, MCP, aprobaciones ni control del Agent. `COMPLETED` solo indica
que el Agent comunicó el final de la sesión; no garantiza que el código sea
correcto.

El análisis distingue `Observed`, `Derived` e `Inferred`. Consulta la
[guía en inglés](../getting-started.md).

# StateSeal

[English](README.md) · [简体中文](README.zh-CN.md) · [日本語](README.ja.md) ·
[한국어](README.ko.md) · **Español** ·
[Português do Brasil](README.pt-BR.md) · [Deutsch](README.de.md) ·
[Français](README.fr.md)

StateSeal es una **capa de inteligencia de ejecución de solo lectura** para
agentes de programación. Reconstruye solicitudes, Skills, herramientas,
artefactos, fallos y resultados comunicados por el Agent como un panorama en
tiempo real.

Nunca inicia, dirige, bloquea, aprueba ni aplica trabajo del Agent. No instala
hooks, no actúa como proxy del modelo y no modifica prompts ni ramas.

```bash
make install
export PATH="$HOME/.local/bin:$PATH"
seal ui
```

Cada afirmación se marca como `Observed` (observada), `Derived` (correlación
determinista) o `Inferred` (hipótesis de diagnóstico que puede ser incorrecta).

Consulta la [documentación en inglés](docs/index.md).

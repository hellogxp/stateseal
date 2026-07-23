# StateSeal

[English](README.md) · [简体中文](README.zh-CN.md) · [日本語](README.ja.md) ·
[한국어](README.ko.md) · **Español** · [Português do Brasil](README.pt-BR.md) ·
[Deutsch](README.de.md) · [Français](README.fr.md)

StateSeal convierte los candidatos de un Coding Agent en resultados de entrega
vinculados al estado exacto del código, verificados de forma independiente,
recuperables, recertificables y auditables. También declara la cobertura de
verificación y los riesgos residuales.

> El modelo propone. La evidencia informa. El Broker decide. Git recuerda. El usuario aplica.

StateSeal no es otro Coding Agent y no sustituye tus pruebas. Envuelve el agente
y los comandos de verificación existentes para asegurar que el código entregado
sea exactamente el código verificado.

## Por qué

Un agente de larga duración puede aprobar las pruebas, continuar editando,
introducir una regresión y aun así informar de éxito. StateSeal convierte
candidatos, evidencia, puntos de control, recertificación y admisión en estados
explícitos del protocolo.

```text
WORKING → CANDIDATE → VERIFYING → VERIFIED → RECERTIFYING → ADMITTED
                           ↘ REJECTED                 ↘ STALE / ABSTAINED
```

## Inicio rápido

```bash
go install github.com/hellogxp/stateseal/cmd/seal@latest
cd your-project
seal integrate codex-desktop
seal run "Corregir callbacks duplicados y mantener la compatibilidad"
seal ui
```

Runs Console muestra las ejecuciones de todos los repositorios, el DAG de
procedencia, la evidencia del verificador, la cronología confiable, la
recuperación y los recibos. La UI es de solo lectura y solo escucha en loopback.

## Límite de confianza

`ADMITTED` solo significa que el punto de control exacto cumplió la política de
`seal.yaml`. No demuestra que la especificación o las pruebas sean completas ni
que el host sea íntegro.

## Documentación

- [Documentación en español](docs/es/index.md)
- [Primeros pasos](docs/es/getting-started.md)
- [Runs Console](docs/es/runs-console.md)
- [Referencia técnica en inglés](docs/index.md)

Los valores del protocolo, comandos, digest y registros conservan el inglés
original para mantener su significado de auditoría.

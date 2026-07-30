# Bien démarrer

```bash
make install
export PATH="$HOME/.local/bin:$PATH"
seal ui
```

StateSeal découvre les sessions Codex locales en lecture seule. Aucun hook,
MCP, accord ou contrôle de l’Agent n’est nécessaire. `COMPLETED` indique
uniquement que l’Agent a signalé la fin de la session et ne garantit pas la
justesse du code.

L’analyse distingue `Observed`, `Derived` et `Inferred`. Consultez le
[guide anglais](../getting-started.md).

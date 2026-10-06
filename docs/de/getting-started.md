# Erste Schritte

```bash
make install
export PATH="$HOME/.local/bin:$PATH"
seal ui
```

StateSeal erkennt lokale Codex-Sitzungen schreibgeschützt. Hooks, MCP,
Genehmigungen oder Agent-Steuerung sind nicht erforderlich. `COMPLETED`
bedeutet nur, dass der Agent das Sitzungsende gemeldet hat; die Korrektheit des
Codes wird nicht bestätigt. Die Ansicht zeigt Codezustände, Änderungen,
Evidenzaktualität, Artefakte, Diagnosen und die Audit-Zeitleiste.

Analysen werden als `Observed`, `Derived` oder `Inferred` gekennzeichnet. Siehe
die [englische Anleitung](../getting-started.md).

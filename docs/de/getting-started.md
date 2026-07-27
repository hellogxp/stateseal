# Erste Schritte

[English](../getting-started.md) | Deutsch

Der normale Ablauf besteht aus einer einmaligen Installation, der Integration
jedes Agenten, der Bestätigung des Prüfvertrags und der Beschreibung des Ziels.

Installieren Sie das von GitHub Actions aus einem bestimmten Git-Tag gebaute
Release. Das Installationsprogramm wählt die Plattform und prüft das
veröffentlichte SHA-256.

```bash
curl --proto '=https' --tlsv1.2 -fsSL https://github.com/hellogxp/stateseal/releases/latest/download/install.sh | sh
export PATH="$HOME/.local/bin:$PATH"
seal version
codex plugin marketplace add /path/to/stateseal
codex plugin add stateseal@stateseal
```

Das Plugin enthält Skill und MCP-Registrierung und verwendet den installierten
Core; MCP wird nicht separat installiert. `marketplace add` registriert nur den
lokalen Checkout und veröffentlicht oder lädt keinen Code hoch. In einer neuen
Codex-Aufgabe aktivieren normale Codeänderungsanfragen StateSeal automatisch
und sichtbar.

Ohne Plugin wählen Sie das Repository für CLI/headless ausdrücklich aus:

```bash
seal run --repo /path/to/project "Eingabevalidierung ergänzen, Kompatibilität erhalten und Tests hinzufügen"
```

In einem übergeordneten Nicht-Git-Workspace erkennt StateSeal untergeordnete
Repositories. Bei mehreren Ergebnissen verwenden Sie `seal workspace list` und
wählen mit `--repo` ausdrücklich aus.

Der erste Projektvertrag und das abschließende Apply erfordern eine
ausdrückliche Bestätigung. Wenn StateSeals eigene Bestätigung, MCP, Store oder
isolierter Worker ausfallen, läuft die normale Agent-Entwicklung weiter; das
Ergebnis wird ohne Beleg als `UNVERIFIED` markiert. Eine echte Ablehnung durch
eine intakte enforce-Richtlinie bleibt verbindlich.

Aus einem anonymen Artefakt oder Quellcode-Checkout kann `make install` mit Git
und Go 1.24 oder neuer ausgeführt werden.

Beim ersten Lauf zeigt StateSeal die erkannten admission- und
completion-Kommandos sowie geschützte Pfade. Bestätigen Sie sie nur, wenn sie
eine angemessene minimale Lieferbarriere bilden.

StateSeal erstellt einen isolierten Vorschlag, bewertet Kandidaten unabhängig,
bewahrt den letzten verifizierten Prüfpunkt und rezertifiziert ihn in einem
frischen Evaluator. Der ursprüngliche Branch ändert sich erst, wenn der Benutzer
ein `ADMITTED`-Ergebnis ausdrücklich anwendet.

```bash
seal ui
```

Die Runs Console zeigt Herkunft, Nachweise, Ereignisse und Belege über alle
Repositories. Siehe [Runs Console](runs-console.md).

`ADMITTED` umfasst nur die Prüfungen in `seal.yaml`. Hostintegrität,
Spezifikationsvollständigkeit und Remote-CI bleiben Restrisiken.

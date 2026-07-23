# Erste Schritte

[English](../getting-started.md) | Deutsch

Der normale Ablauf besteht aus einer einmaligen Installation, der Integration
jedes Agenten, der Bestätigung des Prüfvertrags und der Beschreibung des Ziels.

```bash
seal version
seal integrate codex-desktop
seal integrate status

cd /path/to/project
seal run "Eingabevalidierung ergänzen, Kompatibilität erhalten und Tests hinzufügen"
```

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

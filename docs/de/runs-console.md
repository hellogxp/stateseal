# Runs Console

[English](../runs-console.md) | Deutsch

![StateSeal Runs Console](../assets/stateseal-runs-console.svg)

`seal ui` ist eine schreibgeschützte lokale Sicht auf den externen
Autoritätszustand.

```bash
seal ui
seal ui --no-open
seal ui --address 127.0.0.1:9137
```

Die Liste durchsucht und filtert Läufe aller Repositories. Die Detailansicht
enthält:

- Herkunfts-DAG für Kandidat, Nachweis, Prüfpunkt, Wiederherstellung, Entscheidung und Anwendung;
- Prüfungen, die an den exakten Codezustand gebunden sind;
- Ereigniszeitleiste mit validierter Hash-Kette;
- Abschlussbeleg mit verdict, rule, disposition und checkpoint.

Einfache Läufe komprimieren leere Ebenen. Langer Text bleibt innerhalb des
Knotens; Tooltip und Inspector zeigen den vollständigen Inhalt.

Die UI unterstützt acht Sprachen und erkennt die Browsersprache. Goal,
Protokollwerte, IDs, Digests und Logs behalten den ursprünglichen Audittext.

Der Server bindet ausschließlich an Loopback und besitzt keine Schreib-API.
Bei fehlerhafter Hash-Kontinuität wird die Zeitleiste nicht als vertrauenswürdig
angezeigt.

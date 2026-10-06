# StateSeal

[English](README.md) · [简体中文](README.zh-CN.md) · [日本語](README.ja.md) ·
[한국어](README.ko.md) · [Español](README.es.md) ·
[Português do Brasil](README.pt-BR.md) · **Deutsch** · [Français](README.fr.md)

StateSeal ist eine **schreibgeschützte Delivery-Intelligence-Schicht** für
Coding Agents. Anfragen, beobachtete Codezustände, Prüfungen, Artefakte, Fehler
und Abschlussmeldungen des Agent werden als Live-Panorama rekonstruiert.

StateSeal startet, steuert, blockiert, genehmigt oder übernimmt keine
Agent-Arbeit. Es installiert keine Hooks, vermittelt keinen Modellverkehr und
ändert weder Prompts noch Branches.

```bash
make install
export PATH="$HOME/.local/bin:$PATH"
seal ui
```

Jede Aussage wird als `Observed` (beobachtet), `Derived` (deterministisch
abgeleitet) oder `Inferred` (möglicherweise falsche Diagnosehypothese) markiert.

Siehe die [englische Dokumentation](docs/index.md).

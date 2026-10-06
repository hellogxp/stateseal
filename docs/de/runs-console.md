# Delivery-Panorama

`seal ui` ist eine schreibgeschützte Ansicht lokaler Agent-Auslieferungen. Sie
zeigt den aktuellen Codezustand, geänderte Dateien, zustandsgebundene Prüfungen,
veraltete Evidenz, Artefakte, Diagnosen und die Audit-Zeitleiste.

```text
Anfrage → beobachtete Codezustände → Prüfungen / Artefakte → Agent-Abschlussmeldung
```

`Observed` ist direkt beobachtet, `Derived` reproduzierbar abgeleitet und
`Inferred` eine Hypothese. Es gibt keine Blockier-, Freigabe- oder Apply-Aktion.

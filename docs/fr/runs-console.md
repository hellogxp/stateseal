# Panorama de livraison

`seal ui` est une vue en lecture seule des livraisons locales de l’Agent. Elle
montre l’état du code, les fichiers modifiés, les contrôles liés à chaque état,
les preuves périmées, les artefacts, les diagnostics et la chronologie d’audit.

```text
Requête → états observés → contrôles / artefacts → déclaration de fin de l’Agent
```

`Observed` est un fait direct, `Derived` une corrélation reproductible et
`Inferred` une hypothèse. Aucune action ne bloque, n’approuve ou n’applique le travail.

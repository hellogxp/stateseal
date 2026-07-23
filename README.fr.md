# StateSeal

[English](README.md) · [简体中文](README.zh-CN.md) · [日本語](README.ja.md) ·
[한국어](README.ko.md) · [Español](README.es.md) ·
[Português do Brasil](README.pt-BR.md) · [Deutsch](README.de.md) · **Français**

StateSeal transforme les candidats produits par les Coding Agents en résultats
de livraison liés à l’état exact du code, vérifiés indépendamment,
récupérables, recertifiables et auditables. La couverture de vérification et
les risques résiduels sont également explicites.

> Le modèle propose. Les preuves informent. Le Broker décide. Git mémorise. L’utilisateur applique.

StateSeal n’est ni un autre Coding Agent ni un remplacement des tests. Il
encadre l’agent et les commandes de vérification existants afin que le code
livré corresponde exactement au code vérifié.

## Pourquoi

Un agent de longue durée peut réussir les tests, continuer à modifier le code,
introduire une régression et malgré tout annoncer un succès. StateSeal rend les
candidats, les preuves, les points de contrôle, la recertification et
l’admission explicites dans le protocole.

```text
WORKING → CANDIDATE → VERIFYING → VERIFIED → RECERTIFYING → ADMITTED
                           ↘ REJECTED                 ↘ STALE / ABSTAINED
```

## Démarrage rapide

```bash
go install github.com/hellogxp/stateseal/cmd/seal@latest
cd your-project
seal integrate codex-desktop
seal run "Corriger les callbacks dupliqués et préserver la compatibilité"
seal ui
```

Runs Console affiche les exécutions de tous les dépôts, le DAG de provenance,
les preuves du vérificateur, la chronologie fiable, la récupération et les
reçus. L’interface est en lecture seule et écoute uniquement sur loopback.

## Limite de confiance

`ADMITTED` signifie uniquement que le point de contrôle exact respecte la
politique de `seal.yaml`. Cela ne prouve ni que la spécification ou les tests
sont complets, ni que l’hôte est intègre.

## Documentation

- [Documentation française](docs/fr/index.md)
- [Bien démarrer](docs/fr/getting-started.md)
- [Runs Console](docs/fr/runs-console.md)
- [Référence technique en anglais](docs/index.md)

Les valeurs du protocole, commandes, digest et journaux restent en anglais pour
préserver leur sens d’audit.

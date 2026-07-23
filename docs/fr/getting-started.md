# Bien démarrer

[English](../getting-started.md) | Français

Le parcours normal consiste à installer StateSeal une fois, intégrer chaque
Agent, confirmer le contrat de vérification du projet et décrire l’objectif.

```bash
seal version
seal integrate codex-desktop
seal integrate status

cd /path/to/project
seal run "Ajouter la validation des entrées, préserver la compatibilité et inclure des tests"
```

Lors de la première exécution, StateSeal présente les commandes d’admission et
de completion détectées ainsi que les chemins protégés. Confirmez-les uniquement
si elles constituent une barrière minimale de livraison appropriée.

StateSeal crée une proposition isolée, évalue les candidats indépendamment,
préserve le dernier point vérifié et le recertifie dans un evaluator neuf. La
branche d’origine ne change que lorsque l’utilisateur applique explicitement un
résultat `ADMITTED`.

```bash
seal ui
```

Runs Console permet d’inspecter la provenance, les preuves, les événements et
les reçus de tous les dépôts. Voir [Runs Console](runs-console.md).

`ADMITTED` couvre uniquement les vérifications de `seal.yaml`. L’intégrité de
l’hôte, la complétude de la spécification et la CI distante restent des risques.

# Bien démarrer

[English](../getting-started.md) | Français

Le parcours normal consiste à installer StateSeal une fois, intégrer chaque
Agent, confirmer le contrat de vérification du projet et décrire l’objectif.

Installez la version construite par GitHub Actions à partir d’un Git tag
déterminé. Le programme sélectionne la plateforme et vérifie le SHA-256 publié.

```bash
curl --proto '=https' --tlsv1.2 -fsSL https://github.com/hellogxp/stateseal/releases/latest/download/install.sh | sh
export PATH="$HOME/.local/bin:$PATH"
seal version
codex plugin marketplace add /path/to/stateseal
codex plugin add stateseal@stateseal
```

Le Plugin regroupe le Skill et l’enregistrement MCP et utilise le Core installé ;
MCP ne s’installe pas séparément. `marketplace add` enregistre uniquement le
checkout local, sans publication ni téléversement. Dans une nouvelle tâche
Codex, les demandes ordinaires de modification activent StateSeal
automatiquement et de façon visible.

Sans Plugin, en CLI/headless, sélectionnez explicitement le dépôt :

```bash
seal run --repo /path/to/project "Ajouter la validation des entrées, préserver la compatibilité et inclure des tests"
```

Depuis un Workspace parent non Git, StateSeal découvre les dépôts enfants. S’il
y en a plusieurs, utilisez `seal workspace list` puis choisissez avec `--repo`.

Le premier contrat du projet et l’Apply final exigent une confirmation
explicite. Si la confirmation, MCP, Store ou le worker isolé de StateSeal
échoue, le développement normal de l’Agent continue, le résultat est marqué
`UNVERIFIED` et aucun reçu n’est émis. Un rejet réel par une politique enforce
saine reste contraignant.

Depuis un artefact anonyme ou un checkout des sources, exécutez `make install`
avec Git et Go 1.24 ou version ultérieure.

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

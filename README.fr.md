# StateSeal

[English](README.md) · [简体中文](README.zh-CN.md) · [日本語](README.ja.md) ·
[한국어](README.ko.md) · [Español](README.es.md) ·
[Português do Brasil](README.pt-BR.md) · [Deutsch](README.de.md) · **Français**

StateSeal est une **couche d’intelligence d’exécution en lecture seule** pour
les agents de programmation. Elle reconstruit les requêtes, Skills, outils,
artefacts, échecs et résultats signalés par l’Agent dans un panorama en temps
réel.

Elle ne démarre, ne dirige, ne bloque, n’approuve et n’applique jamais le
travail de l’Agent. Elle n’installe aucun hook, ne relaie pas le trafic du
modèle et ne modifie ni les prompts ni les branches.

```bash
make install
export PATH="$HOME/.local/bin:$PATH"
seal ui
```

Chaque affirmation est marquée `Observed` (observée), `Derived` (corrélation
déterministe) ou `Inferred` (hypothèse de diagnostic potentiellement erronée).

Consultez la [documentation anglaise](docs/index.md).

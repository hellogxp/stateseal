# Runs Console

[English](../runs-console.md) | Français

![Runs Console de StateSeal](../assets/stateseal-runs-console.svg)

`seal ui` est une vue locale en lecture seule de l’état d’autorité externe.

```bash
seal ui
seal ui --no-open
seal ui --address 127.0.0.1:9137
```

La liste recherche et filtre les exécutions de tous les dépôts. Le détail
contient :

- le DAG du candidat, des preuves, du point de contrôle, de la récupération, de la décision et de l’application ;
- les exécutions du vérificateur liées à l’état exact du code ;
- la chronologie dont la chaîne de hachage est validée ;
- le reçu avec verdict, rule, disposition et checkpoint.

Les exécutions simples compressent les couches vides. Les textes longs restent
dans le nœud ; le tooltip et l’Inspector affichent le contenu complet.

L’interface prend en charge huit langues et détecte celle du navigateur. Goal,
valeurs du protocole, ID, digest et logs conservent le texte d’audit original.

Le serveur se lie uniquement à loopback et n’expose aucune API d’écriture. Si
la continuité des hachages échoue, la chronologie n’est pas présentée comme
fiable.

# Release v11.5.0 - Des tests et une CI deux fois plus rapides

**Date** : 2026-11-09

## En bref

Cette version ne change rien pour les joueurs ni pour l'animateur : l'interface, les protocoles et le comportement du jeu sont identiques. Elle accélère la vérification de chaque modification et prépare un code plus facile à faire évoluer.

## Ce qui change dans les coulisses

- **Tests web plus rapides** : l'étape de tests de l'interface passe de 155 s à 30-50 s en CI, avec les mêmes 4 385 tests.
- **Serveur découpé en modules** : la logique des manches, de l'ambiance lumineuse, des LED des buzzers et des affichages joueurs vit dans des paquets Go indépendants. Le fichier principal passe de 8 701 à 6 217 lignes, sans test supprimé (2 595 à 2 612 tests de premier niveau).
- **Tests serveur parallélisés** : 27 tests de l'application s'exécutent maintenant en parallèle.
- **Firmware vérifié à chaque branche** : la compilation du firmware BuzzClick est contrôlée par la CI sur chaque branche, avec le même script que pour les releases.
- **Environnement de développement conteneurisé** : Go 1.26.10, Node 22 et PlatformIO dans un conteneur Linux.

## Mesures

| | Avant | Après |
|---|---|---|
| CI complète | 7 min 23 à 8 min 07 | 3 min 32 à 3 min 38 |
| Tests web en CI | 155 s | 30-50 s |
| Tests serveur en local (-race) | 266 s | 114-123 s |

## Ce qui reste à faire

Les tests du paquet serveur principal durent encore 87 à 91 s en CI (objectif 75 s). Trois chantiers de suivi sont ouverts : #296, #297 et #298.

## Comment mettre à jour

Aucune action requise. Remplacez l'exécutable du serveur comme d'habitude ; le firmware des buzzers n'a pas changé de comportement.

## Liens

- [Journal des modifications](https://github.com/CCoupel/BuzzMaster/blob/main/CHANGELOG.md)
- [Releases GitHub](https://github.com/CCoupel/BuzzMaster/releases)

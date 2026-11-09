# Posts v11.5.0

## Twitter / X

BuzzControl v11.5.0 : rien ne change pour vos soirées quiz, tout va plus vite pour nous.
CI divisée par 2 (8 min → 3,5 min), tests web 155 s → 30-50 s, serveur découpé en modules.
Les futures nouveautés arriveront plus vite.
#BuzzControl #opensource #quiz

## LinkedIn

BuzzControl v11.5.0 est disponible. C'est une version de maintenance : aucun changement pour les joueurs ni pour l'animateur.

Chaque modification du projet est vérifiée par plus de 4 000 tests web et plus de 2 600 tests Go. Cette vérification prenait plus de 7 minutes. Elle en prend maintenant moins de 4, grâce à des tests mieux isolés, exécutés en parallèle, et à un serveur découpé en modules indépendants.

Le firmware des buzzers est aussi compilé par la CI sur chaque branche, avec le même script que pour les releases.

Résultat : des retours plus rapides, et de nouvelles fonctionnalités qui arriveront plus souvent.

Détails et mesures dans le journal des modifications.

#BuzzControl #Go #ESP32 #CI

## Reddit / forum

**[Release] v11.5.0 - Optimisation des tests**

Pas de changement fonctionnel. Ce qui change :
- vitest en pool vmThreads, 33 fichiers sans DOM en environnement node : 155 s → 30-50 s en CI
- cmd/server découpé en paquets internal/ (vplayer, rounds, led, ambiance...) : main.go 8 701 → 6 217 lignes, aucun test perdu
- 27 tests cmd/server en t.Parallel()
- job compiling-firmware dans ci-branch.yml, partagé avec release.yml via scripts/build-firmware.sh
- job CI complet : ~8 min → ~3,5 min

Objectif non atteint : cmd/server à 87-91 s en CI (cible 75 s). Suivi : #296, #297, #298.

**Feedback bienvenu** : issues GitHub.

# BuzzControl v11.1.0 — « Question sonore »

**Date** : 25 septembre 2026

## Nouveautés

### 🎧 Un son pour chaque question
Vos questions SPEEDY, QCM et ARDOISE peuvent maintenant porter leur propre son : un fichier WAV (30 secondes et 6 Mio maximum) ajouté depuis l'éditeur Quiz. Il se joue au lancement de la question, sur l'enceinte du serveur, sans jamais ralentir le jeu.

**Chronomètre au choix, question par question** : simultané (le son et le chrono partent ensemble, par défaut) ou différé (le chrono attend la fin du son). En mode différé, la mention « ⏳ Le chrono démarre à la fin du son » s'affiche sur l'admin, l'animateur et la TV.

**L'animateur garde la main** : boutons ↻ Rejouer, ⏸ Pause/Reprise et ⏹ Stop, visibles dès qu'un son est attaché à la question en cours.

**Pas de mauvaise surprise** : avant le passage à « prêt », BuzzControl vérifie que le son pourra être joué (audio désactivé, enceinte indisponible, fichier introuvable) et vous l'indique. Une pastille « Audio indisponible » vous alerte dans l'administration si votre quiz contient des sons alors que l'audio n'est pas disponible. En cas de besoin, l'administrateur peut contourner cette vérification (Ctrl + clic sur la question).

### 🧭 Une barre de navigation admin repensée
- Nouveau groupe **🛠️ Préparation** : Joueurs, Quiz, Backstage et la section Interface (TV, Joueur, Animateur, ouverts dans un nouvel onglet).
- Le menu du logo propose désormais **« Réglages »** (ex-« Config »).
- Le bouton d'entracte devient **🍿 ENTRACTE** au repos et **🎬 REPRISE** quand l'entracte est en cours.
- Sur écran étroit, les libellés passent en icônes par paliers de largeur, et un badge 👥 regroupe les compteurs de connectés (survol ou clic pour les déplier).
- La pastille « Connecté » reste toujours visible.

### ⚡ Un nouveau logo
Le bouton du menu de configuration affiche le nouveau logo BuzzControl (« Buzz » en indigo, « Control » en rose, ⚡ en coin), à la place de l'abeille animée.

### 📺 La TV revient sur « Jeu » toute seule
Quand vous sélectionnez une question, lancez le START, reprenez après une pause ou tombez sur une carte MEMOTION ou un tirage RAFALE, l'affichage TV repasse automatiquement sur la vue « Jeu » — plus besoin d'y penser si vous étiez resté sur les classements. Vous pouvez toujours rebasculer manuellement ensuite.

## Comment mettre à jour
Depuis l'interface admin (menu du logo → Mises à jour) ou en téléchargeant la dernière version. Aucune migration nécessaire.

## Liens
- [Notes de version complètes (CHANGELOG)](https://github.com/CCoupel/BuzzMaster/blob/main/CHANGELOG.md)
- [Release GitHub](https://github.com/CCoupel/BuzzMaster/releases/tag/v11.1.0)

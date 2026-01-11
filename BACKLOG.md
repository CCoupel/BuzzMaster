# BuzzControl - Backlog

Fonctionnalités à implémenter.

---

## Gestion des scores

- [x] **Points d'équipe dissociés des points joueurs** *(v2.18.0)*
  - Champ `TEAM_POINTS` sur les équipes
  - Score total = TEAM_POINTS + sum(player scores)
  - Clic sur header équipe = points à l'équipe
  - Clic sur ligne joueur = points au joueur

---

## Catégories de questions

- [x] **Champ CATEGORY pour les questions** *(implémenté)*
  - Ajouter un champ `CATEGORY` au modèle Question
  - UI pour sélectionner/créer une catégorie lors de l'ajout de question
  - Filtrage des questions par catégorie dans QuestionsPage

- [ ] **Palmarès par catégorie**
  - Afficher un classement des équipes filtré par catégorie
  - Afficher un classement des joueurs filtré par catégorie
  - Nouveau mode d'affichage TV ou onglet dans ScoresPage

---

## Timer et gameplay

- [x] **Décompte de 3 secondes avant le timer** *(v2.29.0)*
  - Décompte visuel "3... 2... 1..." avant le timer principal
  - Phase COUNTDOWN distincte avec badge orange "DECOMPTE"
  - Les buzzers restent bloqués pendant le décompte
  - Le timer démarre automatiquement après le décompte

---

## QCM - Indices et pénalités

- [ ] **Invalidation automatique des mauvaises réponses QCM**
  - Si aucun joueur n'a buzzé, invalider une mauvaise réponse à certains seuils
  - La réponse invalidée est visuellement barrée/grisée sur l'affichage TV
  - L'invalidation est aléatoire parmi les mauvaises réponses restantes
  - **Seuils proportionnels au timer :**
    - Seuil 1 (1er indice) : 25% du temps restant
    - Seuil 2 (2ème indice) : 12.5% du temps restant
  - **Contraintes de sécurité :**
    - Seuil 2 ≥ 2s minimum (temps pour réagir)
    - Seuil 1 ≥ Seuil 2 + 2s (écart minimum entre indices)
    - Si timer < 6s → pas d'indices ni pénalités
  - **Exemples :**
    | Timer | Seuil 1 | Seuil 2 |
    |-------|---------|---------|
    | 30s   | 7.5s    | 3.75s   |
    | 20s   | 5s      | 2.5s    |
    | 15s   | 4s      | 2s      |
    | 10s   | 4s      | 2s      |
    | <6s   | —       | —       |

- [ ] **Pénalités de points selon le nombre de réponses restantes**
  - Si un joueur buzz après invalidation(s), ses points sont réduits
  - **Ratio de pénalité :**
    - 4 réponses (aucune invalidée) → 100% des points
    - 3 réponses (1 invalidée) → 67% des points (pénalité 1/3)
    - 2 réponses (2 invalidées) → 33% des points (pénalité 2/3)
  - Afficher la pénalité applicable sur l'interface admin
  - L'historique enregistre les points effectivement attribués

---

## Debug et tests

- [x] **Ctrl+Click sur joueur en PREPARE simule PONG** *(v2.28.0)*
  - En état PREPARE, Ctrl+Click sur un joueur simule la réponse au PING
  - Permet de tester sans buzzers physiques connectés
  - Le joueur passe de "en attente" à "prêt"

---

## Affichage TV

- [x] **Synchronisation des changements d'image de fond** *(v2.30.0)*
  - Le serveur centralise le timing et notifie tous les clients
  - `CurrentBackgroundIndex` dans GameState (backend)
  - Goroutine de cycling dans main.go
  - Action `BACKGROUND_CHANGE` dans le protocole WebSocket
  - Tous les clients TV reçoivent l'index synchronisé
  - Transitions simultanées sur tous les écrans

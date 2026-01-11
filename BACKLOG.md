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

- [ ] **Décompte de 3 secondes avant le timer**
  - Ajouter un décompte visuel "3... 2... 1..." avant de lancer le timer principal
  - Animation ou affichage distinct du timer normal
  - Les buzzers restent bloqués pendant le décompte
  - Le timer démarre automatiquement après "1"

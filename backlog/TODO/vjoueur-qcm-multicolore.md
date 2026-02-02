# VJoueur QCM Multicolore

**Statut** : 📋 Planifié

## Description

Améliorer le VJoueur (page `/player`) pour le mode QCM :
- Un VJoueur est valide pour **toutes les couleurs** (pas une seule)
- Afficher une pastille avec les 4 couleurs au lieu d'une seule
- Le VJoueur peut buzzer directement sur les couleurs/réponses depuis son écran
- Si un VJoueur est dans une équipe, les buzzers physiques de couleur unique de cette équipe deviennent invalides

## Objectifs

- [ ] VJoueur valide pour les 4 couleurs QCM
- [ ] Interface de buzz direct sur les réponses (écran tactile)
- [ ] Gestion de la priorité VJoueur vs buzzers physiques
- [ ] Pastille multicolore dans l'admin

## Tâches

### Phase 1 - Modèle et pastille multicolore

- [ ] **Modifier le modèle Bumper pour VJoueur**
  - Nouveau champ `IsVPlayer: bool` ou `BumperType: "physical" | "vplayer"`
  - Un VJoueur n'a pas de couleur unique, il peut répondre à toutes les couleurs

- [ ] **Pastille multicolore dans l'admin**
  - Remplacer la pastille de couleur unique par une pastille 4 couleurs
  - Design : cercle divisé en 4 quartiers (rouge, vert, jaune, bleu)
  - Ou : icône smartphone avec indicateur multicolore

### Phase 2 - Interface de buzz direct QCM

- [ ] **Affichage des réponses QCM sur VJoueur**
  - Pendant un jeu QCM, afficher les 4 réponses sur l'écran du VJoueur
  - Boutons tactiles colorés pour chaque réponse
  - Afficher uniquement pendant la phase de jeu (pas en IDLE)

- [ ] **Action WebSocket VPLAYER_QCM_ANSWER**
  - Payload : `{ANSWER_COLOR: string}` (RED, GREEN, YELLOW, BLUE)
  - Le serveur traite comme un buzz de cette couleur
  - Attribution des points à l'équipe du VJoueur

### Phase 3 - Priorité et invalidation des buzzers physiques

- [ ] **Logique d'invalidation des buzzers physiques**
  - Si une équipe a un VJoueur actif, les buzzers physiques de couleur unique de cette équipe sont ignorés
  - Raison : le VJoueur peut répondre pour toutes les couleurs, les buzzers physiques feraient doublon

- [ ] **Détection d'équipe avec VJoueur**
  - Ajouter un champ `HasVPlayer: bool` dans Team ou calculer dynamiquement
  - Lors d'un buzz physique, vérifier si l'équipe a un VJoueur actif

- [ ] **Feedback visuel**
  - Dans l'admin, indiquer quels buzzers sont invalidés (grisés, barrés)
  - Tooltip explicatif : "Invalidé : équipe a un VJoueur"

### Phase 4 - Tests et edge cases

- [ ] **Tests unitaires**
  - VJoueur peut répondre aux 4 couleurs
  - Buzz physique ignoré si équipe a VJoueur
  - Buzz physique accepté si équipe n'a pas VJoueur

- [ ] **Edge cases**
  - VJoueur se déconnecte → réactiver les buzzers physiques
  - VJoueur change d'équipe → mettre à jour les invalidations
  - Plusieurs VJoueurs dans une équipe → comportement identique

## Fichiers concernés

### Backend
- `server-go/internal/game/models.go` : Champ VPlayer dans Bumper
- `server-go/internal/game/engine.go` : Logique de buzz QCM + invalidation
- `server-go/internal/server/websocket.go` : Action VPLAYER_QCM_ANSWER

### Frontend
- `server-go/web/src/pages/PlayerPage.jsx` : Interface QCM tactile
- `server-go/web/src/components/TeamCard.jsx` : Pastille multicolore
- `server-go/web/src/pages/GamePage.jsx` : Indicateur buzzers invalidés

## Compatibilité

- ✅ Rétrocompatible : buzzers physiques fonctionnent normalement sans VJoueur
- ✅ VJoueur existants continuent de fonctionner (ajout de fonctionnalité)
- ⚠️ Changement de comportement : buzzers physiques invalidés si VJoueur présent

## Version cible

vX.Y.Z (à déterminer)

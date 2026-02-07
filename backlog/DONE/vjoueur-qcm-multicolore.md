# VJoueur QCM Multicolore

**Statut** : ✅ Terminé (v2.53.0)

## Description

Améliorer le VJoueur (page `/player`) pour le mode QCM :
- Un VJoueur est valide pour **toutes les couleurs** (pas une seule)
- Afficher une pastille avec les 4 couleurs au lieu d'une seule
- Le VJoueur peut buzzer directement sur les couleurs/réponses depuis son écran
- Si un VJoueur est dans une équipe, les buzzers physiques de couleur unique de cette équipe deviennent invalides

## Objectifs

- [x] VJoueur valide pour les 4 couleurs QCM
- [x] Interface de buzz direct sur les réponses (écran tactile)
- [x] Gestion de la priorité VJoueur vs buzzers physiques
- [x] Pastille multicolore dans l'admin

## Tâches

### Phase 1 - Modèle et pastille multicolore

- [x] **Modifier le modèle Bumper pour VJoueur**
  - Nouveau champ `IS_VPLAYER: bool` ajouté dans `models.go`
  - Un VJoueur n'a pas de couleur unique, il peut répondre à toutes les couleurs

- [x] **Pastille multicolore dans l'admin**
  - Badge SVG 4 quartiers (rouge, vert, jaune, bleu) dans TeamCard.jsx
  - Design : cercle divisé en 4 quartiers colorés

### Phase 2 - Interface de buzz direct QCM

- [x] **Affichage des réponses QCM sur VJoueur**
  - 4 boutons colorés sur VPlayerPage.jsx pendant les questions QCM STARTED
  - Boutons tactiles colorés pour chaque réponse (Rouge, Vert, Jaune, Bleu)
  - Affichage uniquement pendant la phase QCM active

- [x] **Action WebSocket VPLAYER_QCM_ANSWER**
  - Action implémentée dans `messages.go`
  - Payload : `{ANSWER_COLOR: string}` (RED, GREEN, YELLOW, BLUE)
  - Le serveur traite comme un buzz de cette couleur
  - Attribution des points à l'équipe du VJoueur

### Phase 3 - Priorité et invalidation des buzzers physiques

- [x] **Logique d'invalidation des buzzers physiques**
  - Si une équipe a un VJoueur actif, les buzzers physiques sont ignorés en QCM
  - Implémenté dans `engine.go` via fonction `teamHasVPlayer()`
  - Raison : le VJoueur peut répondre pour toutes les couleurs, les buzzers physiques feraient doublon

- [x] **Détection d'équipe avec VJoueur**
  - Fonction `teamHasVPlayer()` calcule dynamiquement
  - Lors d'un buzz physique en QCM, vérification si l'équipe a un VJoueur actif

- [x] **Feedback visuel**
  - Dans GamePage.jsx, buzzers physiques grisés avec tooltip "VJoueur actif"
  - Indicateur visuel clair de l'invalidation

### Phase 4 - Tests et edge cases

- [x] **Tests unitaires**
  - `TestVPlayerBumperCreation` : Création VJoueur avec IS_VPLAYER=true
  - `TestVPlayerQCMBuzzAllColors` : VJoueur peut répondre aux 4 couleurs
  - `TestPhysicalBuzzerInvalidatedForQCM` : Buzz physique ignoré si équipe a VJoueur en QCM
  - `TestPhysicalBuzzerNotInvalidatedForNonQCM` : Buzz physique accepté pour questions NORMAL

- [x] **Edge cases**
  - VJoueur se déconnecte → buzzers physiques redeviennent actifs automatiquement
  - VJoueur change d'équipe → invalidation recalculée dynamiquement
  - Plusieurs VJoueurs dans une équipe → comportement identique (un seul suffit)

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

## Version finale

v2.53.0 (2026-02-07)

# PLAN PHASE 6 - Memory Multi-Teams

**Version** : 2.51.0  
**Branche** : feature/memory-multi-teams  
**Date** : 2026-02-02  
**Cycle** : 1/3

---

## Résumé Exécutif

La Phase 6 ajoute le support des **modes de jeu multi-équipes** au Memory Game, permettant plusieurs équipes de jouer avec des règles de rotation différentes.

### 3 Modes de Jeu

1. **SOLO** (défaut, existant)
   - Une seule équipe joue
   - Comportement identique à Phase 5

2. **CHACUN_SON_TOUR** (nouveau)
   - Multi-équipes en rotation stricte
   - Après chaque tentative (2 cartes), on passe à l'équipe suivante
   - Que la paire soit trouvée ou non

3. **TANT_QUE_JE_GAGNE** (nouveau)
   - Multi-équipes avec garde de la main
   - L'équipe continue si elle trouve des paires
   - Passe à l'équipe suivante si erreur

---

## Implémentation Backend

### Fichiers modifiés

**server-go/internal/game/models.go**
- Ajouter champ `MEMORY_MODE` à `Question`
- Ajouter 3 champs à `GameState` : `MemoryCurrentTeam`, `MemoryTeamPairs`, `MemoryParticipatingTeams`

**server-go/internal/game/engine.go**
- Nouvelle fonction `rotateToNextTeam()` pour changer d'équipe
- Modifier `FlipMemoryCard()` pour supporter multi-équipes
- Modifier initialisation Memory pour setup équipes
- Modifier validation pour vérifier l'équipe courante en multi

**server-go/internal/server/websocket.go**
- Ajouter action `MEMORY_SET_TEAMS` (client → serveur)
- Ajouter broadcast `MEMORY_TEAM_CHANGED` (serveur → clients)
- Modifier broadcast `GAME_STATE` pour inclure champs Memory

**server-go/internal/game/history.go**
- Modifier `MEMORY_COMPLETED` pour inclure résultats par équipe

### Contrats API

**contracts/memory-phase6.md** (nouveau)
- `MEMORY_SET_TEAMS` : Définir équipes participantes
- `MEMORY_TEAM_CHANGED` : Notifier changement d'équipe
- Modification `FLIP_MEMORY_CARD` : validation équipe courante
- Modification `GAME_STATE` : ajouter champs Memory multi

**Dépendances** :
- Frontend attend les contrats finalisés du backend
- Backend peut modifier les contrats si contrainte technique

---

## Implémentation Frontend

### Fichiers modifiés

**web/src/pages/GamePage.jsx**
- Section "Équipes participantes" dépliable dans Prepare
- Checkboxes pour sélectionner équipes
- Validation : min 2 équipes en mode multi
- Envoyer `MEMORY_SET_TEAMS` avant `START_GAME`

**web/src/pages/QuestionsPage.jsx**
- Ajouter sélecteur mode Memory (radio buttons)
- Options : SOLO / CHACUN_SON_TOUR / TANT_QUE_JE_GAGNE
- Défaut : SOLO pour compatibilité

**web/src/pages/PlayerDisplay.jsx** (affichage TV)
- Badge équipe courante au-dessus de la grille
- Texte : "🎮 Au tour de : [Équipe]"
- Tableau des scores en temps réel
- Contrôle des clics : seule l'équipe courante peut cliquer
- Grisage cartes pour équipes en attente

---

## Points de Validation Critiques

### Backend ✅
1. Rotation correcte des équipes (modulo)
2. Scores par équipe calculés correctement
3. Historique inclut résultats multi-équipes
4. Rétrocompatibilité : Phase 5 (SOLO) continue de marcher
5. Questions sans MEMORY_MODE défautent à SOLO

### Frontend ✅
1. Sélection équipes fonctionnelle
2. Badge équipe affichée et mise à jour en temps réel
3. Cartes non-cliquables pour équipes en attente
4. Tableau scores en temps réel
5. Mode SOLO (défaut) affiche sans section équipes

### Intégration ✅
1. `MEMORY_SET_TEAMS` envoyée avant `START_GAME`
2. `MEMORY_TEAM_CHANGED` reçue et traitée correctement
3. `GAME_STATE` inclut champs Memory multi

---

## Timeline Estimée

| Phase | Durée | Agent |
|-------|-------|-------|
| Backend | 2h | dev-backend |
| Frontend | 1.5h | dev-frontend |
| Tests (écriture) | 1h | test-writer |
| Revue code | 1h | code-reviewer |
| Tests (exécution) | 1.5h | QA |
| Documentation | 30min | doc-updater |
| Déploiement | 30min | deploy |
| **TOTAL** | **8h** | |

---

## Dépendances & Ordonnancement

```
Phase 2 : DÉVELOPPEMENT
    │
    ├─→ dev-backend (2h)
    │   - Models, engine, WebSocket
    │   - Contrats API finalisés
    │   - Push vers branche
    │   │
    │   ▼
    └─→ dev-frontend (1.5h)
        - Consulte contrats finalisés
        - GamePage, QuestionsPage, PlayerDisplay
        - Push vers branche

Phase 3 : TESTS (1h)
    │
    └─→ test-writer
        - Tests unitaires Go
        - Scénarios E2E Chrome

Phase 4 : REVUE (1h)
    │
    └─→ code-reviewer
        - Analyse backend + frontend
        - Verdict : APPROVED / REJECTED

Phase 5 : QA (1.5h)
    │
    └─→ QA
        - go test ./...
        - Scénarios E2E Chrome
        - Verdict : VALIDATED / NOT_VALIDATED

Phase 6 : DOCUMENTATION (30min)
    │
    └─→ doc-updater

Phase 7 : DÉPLOIEMENT (30min)
    │
    └─→ deploy (QUALIF)
```

---

## Gestion des Erreurs

| Erreur | Action |
|--------|--------|
| Rotation incorrecte | Vérifier logique modulo dans `rotateToNextTeam()` |
| Scores non-mises à jour | Vérifier `MemoryTeamPairs` actualisé et broadcasté |
| Clics d'équipe incorrecte acceptés | Vérifier validation `MemoryCurrentTeam` |
| Historique incomplet | Vérifier `MEMORY_COMPLETED` inclut `TeamResults` |
| Questions anciennes cassées | Vérifier défaut SOLO pour absence `MEMORY_MODE` |

---

## Risques & Mitigations

| Risque | Impact | Mitigation |
|--------|--------|-----------|
| Oubli validation équipe courante | Équipes peuvent tricher | Tests unitaires + E2E exhaustifs |
| Perte compatibilité Phase 5 | Cassure production | Défaut SOLO testée dès Phase 3 |
| Race condition equipe/scores | Desynchronisation TV | Broadcast `GAME_STATE` complet après chaque flip |
| Historique malformé | Données perdues | Tests JSON historique en Phase 3 |

---

## Prochaines Étapes

1. **✅ Phase 1 (Planification) - Terminée**
   - Plan créé
   - Contrats définis
   - Branche créée
   - Version incrémentée

2. **▶ Phase 2 (Développement)**
   - Lancer dev-backend
   - Lancer dev-frontend (après backend)

3. **▶ Phases 3-7**
   - test-writer
   - code-reviewer
   - QA
   - doc-updater
   - deploy

---

## Validation Utilisateur

**Validez-vous ce plan ?**

- ✅ **OUI** : Procéder à Phase 2 (Développement)
- ❌ **NON** : Modifier le plan
- 🔄 **MODIFIER** : Préciser les changements


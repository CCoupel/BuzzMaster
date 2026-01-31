# Procédure de Tests - BuzzControl

> Document de référence pour les tests unitaires et bout en bout.
> Structure progressive : chaque phase ajoute des tests à la suite existante.

---

## 1. Structure des Tests

### Tests Unitaires Go

| Fichier | Couverture |
|---------|------------|
| `internal/game/models_test.go` | Modèles de données, sérialisation JSON |
| `internal/game/engine_test.go` | Machine d'état du jeu |
| `internal/server/http_test.go` | API REST, upload questions |
| `internal/server/tcp_test.go` | Protocole TCP buzzers |
| `internal/server/udp_test.go` | Broadcast UDP |
| `internal/server/e2e_test.go` | Tests d'intégration serveur |
| `internal/protocol/parser_test.go` | Parsing JSON null-terminated |
| `internal/protocol/messages_test.go` | Types de messages |

---

## 2. Exécution des Tests

### 2.1 Commandes

```bash
# Tests complets avec rapport
cd server-go
go test ./... -v -cover 2>&1 | tee test-report.txt

# Tests par package
go test ./internal/game/... -v
go test ./internal/server/... -v

# Couverture HTML
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html
```

### 2.2 Seuils de Qualité

| Métrique | Seuil Minimum |
|----------|---------------|
| Couverture globale | 60% |
| Couverture par package modifié | 70% |
| Tests en échec | 0 |

---

## 3. Tests Non-Régression (Existants)

### 3.1 Models (`models_test.go`)

| Test | Description |
|------|-------------|
| `TestGamePhase_Values` | Valeurs des phases de jeu |
| `TestQuestionStatus_Values` | Statuts des questions |
| `TestTeam_JSONSerialization` | Sérialisation équipes |
| `TestBumper_JSONSerialization` | Sérialisation buzzers |
| `TestQuestion_JSONSerialization` | Sérialisation questions NORMAL |
| `TestGameState_JSONSerialization` | Sérialisation état jeu |
| `TestTeamsAndBumpers_JSONSerialization` | Sérialisation collections |

### 3.2 HTTP (`http_test.go`)

| Test | Description |
|------|-------------|
| `TestHTTPServer_Version` | Endpoint /version |
| `TestHTTPServer_ListGame` | Endpoint /listGame |
| `TestHTTPServer_Questions_Empty` | Liste questions vide |
| `TestHTTPServer_Questions_WithData` | Liste avec questions |
| `TestHTTPServer_QuestionUpload` | Upload question NORMAL |
| `TestHTTPServer_CORS` | Headers CORS |

---

## 4. Tests Memory Game - Par Phase

### 4.1 PHASE 1 : Modèle de Données + Éditeur

**Objectif** : Créer et persister des questions Memory

#### Tests Unitaires Go

| Test | Fichier | Description | Statut |
|------|---------|-------------|--------|
| `TestQuestionType_MemoryConstant` | models_test.go | Constante MEMORY existe | ✅ Implémenté |
| `TestMemoryCard_TextSerialization` | models_test.go | Carte texte JSON | ✅ Implémenté |
| `TestMemoryCard_ImageSerialization` | models_test.go | Carte image JSON | ✅ Implémenté |
| `TestMemoryPair_JSONSerialization` | models_test.go | Paire de cartes JSON | ✅ Implémenté |
| `TestMemoryConfig_JSONSerialization` | models_test.go | Config Memory JSON | ✅ Implémenté |
| `TestMemoryConfig_UseTimerFalse` | models_test.go | Config sans timer | ✅ Implémenté |
| `TestQuestion_MemoryType_JSONSerialization` | models_test.go | Question Memory complète | ✅ Implémenté |
| `TestQuestion_MemoryOmitEmpty` | models_test.go | Omit fields pour NORMAL | ✅ Implémenté |
| `TestHTTPServer_MemoryQuestionUpload` | http_test.go | Upload question Memory | ✅ Implémenté |
| `TestHTTPServer_MemoryQuestionLoad` | http_test.go | Lecture question Memory | ✅ Implémenté |

#### Tests E2E Chrome - Phase 1

| ID | Scénario | Étapes | Résultat Attendu |
|----|----------|--------|------------------|
| E2E-M1-01 | Création Memory texte | 1. /quiz → Type Memory<br>2. Titre + 2 paires texte<br>3. Sauvegarder | Question créée avec badge MEMORY |
| E2E-M1-02 | Création Memory image | 1. Créer question Memory<br>2. Paire avec image<br>3. Sauvegarder | Image uploadée, preview visible |
| E2E-M1-03 | Création Memory mixte | 1. Paire 1: texte/texte<br>2. Paire 2: image/texte<br>3. Sauvegarder | Les 2 types coexistent |
| E2E-M1-04 | Édition Memory | 1. Cliquer question Memory<br>2. Modifier paire<br>3. Sauvegarder | Modifications persistées |
| E2E-M1-05 | Ajout/Suppression paires | 1. Ajouter 3ème paire<br>2. Supprimer 2ème paire<br>3. Sauvegarder | 2 paires restantes |
| E2E-M1-06 | Config Memory | 1. Modifier flipDelay=5000<br>2. Modifier pointsPerPair=20<br>3. Sauvegarder | Config mise à jour dans JSON |
| E2E-M1-07 | Timer désactivé | 1. Décocher "Utiliser timer"<br>2. Sauvegarder | USE_TIMER=false dans JSON |
| E2E-M1-08 | Validation min 2 paires | 1. Supprimer jusqu'à 1 paire<br>2. Essayer sauvegarder | Bouton désactivé ou erreur |

#### Vérification JSON - Phase 1

```json
{
  "ID": "X",
  "QUESTION": "Titre de la question",
  "ANSWER": "N paires",
  "TYPE": "MEMORY",
  "MEMORY_PAIRS": [
    {
      "ID": 1,
      "CARD1": { "TEXT": "...", "IS_IMAGE": false },
      "CARD2": { "TEXT": "...", "IS_IMAGE": false }
    }
  ],
  "MEMORY_CONFIG": {
    "FLIP_DELAY": 3000,
    "POINTS_PER_PAIR": 10,
    "ERROR_PENALTY": 0,
    "COMPLETION_BONUS": 0,
    "USE_TIMER": true
  }
}
```

---

### 4.2 PHASE 2 : Affichage TV (Grille + Cartes)

**Objectif** : Afficher la grille Memory sur l'écran TV (/tv)

#### Tests Unitaires Go

| Test | Fichier | Description | Statut |
|------|---------|-------------|--------|
| `TestEngine_MemoryGameState` | engine_test.go | État jeu Memory | À implémenter |
| `TestEngine_MemoryCardShuffle` | engine_test.go | Mélange des cartes | À implémenter |
| `TestWebSocket_MemoryBroadcast` | e2e_test.go | Broadcast état Memory | À implémenter |

#### Tests E2E Chrome - Phase 2

| ID | Scénario | Étapes | Résultat Attendu |
|----|----------|--------|------------------|
| E2E-M2-01 | Affichage grille | 1. Sélectionner question Memory<br>2. Passer en READY<br>3. Vérifier /tv | Grille de cartes face cachée |
| E2E-M2-02 | Layout responsive | 1. 4 paires → grille 4x2<br>2. 6 paires → grille 4x3 | Grille adaptée au nombre |
| E2E-M2-03 | Cartes face cachée | 1. Phase READY | Toutes cartes dos visible |
| E2E-M2-04 | Design cartes | 1. Vérifier style cartes | Coins arrondis, ombre, animation hover |

---

### 4.3 PHASE 3 : Gameplay (Sélection + Matching)

**Objectif** : Jouer au Memory avec buzzers/clics

#### Tests Unitaires Go

| Test | Fichier | Description | Statut |
|------|---------|-------------|--------|
| `TestEngine_MemoryCardSelect` | engine_test.go | Sélection carte | À implémenter |
| `TestEngine_MemoryPairMatch` | engine_test.go | Correspondance paire | À implémenter |
| `TestEngine_MemoryPairNoMatch` | engine_test.go | Non-correspondance | À implémenter |
| `TestEngine_MemoryFlipDelay` | engine_test.go | Délai retournement | À implémenter |
| `TestEngine_MemoryScoring` | engine_test.go | Calcul des points | À implémenter |
| `TestEngine_MemoryErrorPenalty` | engine_test.go | Pénalité erreur | À implémenter |
| `TestEngine_MemoryCompletion` | engine_test.go | Fin de partie | À implémenter |
| `TestEngine_MemoryCompletionBonus` | engine_test.go | Bonus completion | À implémenter |

#### Tests E2E Chrome - Phase 3

| ID | Scénario | Étapes | Résultat Attendu |
|----|----------|--------|------------------|
| E2E-M3-01 | Sélection 1ère carte | 1. START<br>2. Cliquer carte | Carte retournée, face visible |
| E2E-M3-02 | Sélection 2ème carte | 1. Cliquer 2ème carte | 2 cartes visibles |
| E2E-M3-03 | Paire trouvée | 1. Sélectionner 2 cartes qui matchent | Animation succès, cartes restent visibles |
| E2E-M3-04 | Paire non trouvée | 1. Sélectionner 2 cartes différentes | Attente flipDelay, puis retournement |
| E2E-M3-05 | Points attribués | 1. Trouver une paire | Score équipe +pointsPerPair |
| E2E-M3-06 | Pénalité erreur | 1. Erreur avec errorPenalty>0 | Score équipe -errorPenalty |
| E2E-M3-07 | Fin de partie | 1. Trouver toutes les paires | Phase STOPPED, message fin |
| E2E-M3-08 | Bonus completion | 1. Finir avec completionBonus>0 | Score +completionBonus |
| E2E-M3-09 | Timer épuisé | 1. Attendre fin timer | Phase STOPPED, paires restantes visibles |
| E2E-M3-10 | Sans timer | 1. USE_TIMER=false<br>2. Jouer | Pas de limite temps |

---

## 5. Checklist par Phase

### Phase 1 - Checklist

- [ ] Tests unitaires models_test.go passent
- [ ] Tests unitaires http_test.go passent
- [ ] E2E-M1-01 à E2E-M1-08 passent
- [ ] JSON généré conforme au format
- [ ] Non-régression : NORMAL et QCM fonctionnent

### Phase 2 - Checklist

- [ ] Tous tests Phase 1 passent
- [ ] Tests unitaires engine_test.go (Memory) passent
- [ ] E2E-M2-01 à E2E-M2-04 passent
- [ ] Affichage TV correct
- [ ] Non-régression : affichage NORMAL/QCM sur TV

### Phase 3 - Checklist

- [ ] Tous tests Phase 1 + 2 passent
- [ ] Tests unitaires gameplay passent
- [ ] E2E-M3-01 à E2E-M3-10 passent
- [ ] Scoring correct
- [ ] Non-régression : gameplay NORMAL/QCM

---

## 6. Format Rapport de Tests

```
========================================
RAPPORT DE TESTS - [DATE] - v[VERSION]
PHASE: [1|2|3]
========================================

TESTS UNITAIRES GO
------------------
Package: internal/game
  models_test.go: XX/XX PASS
  engine_test.go: XX/XX PASS
  Coverage: XX.X%

Package: internal/server
  http_test.go: XX/XX PASS
  Coverage: XX.X%

TOTAL UNITAIRES: XX/XX PASS
COUVERTURE GLOBALE: XX.X%

TESTS E2E CHROME
----------------
Phase 1:
  [x] E2E-M1-01 Création Memory texte - PASS
  [x] E2E-M1-02 Création Memory image - PASS
  ...

Phase 2: (si applicable)
  [x] E2E-M2-01 Affichage grille - PASS
  ...

Phase 3: (si applicable)
  [x] E2E-M3-01 Sélection carte - PASS
  ...

NON-RÉGRESSION
--------------
  [x] Questions NORMAL - PASS
  [x] Questions QCM - PASS
  [x] Upload images - PASS

========================================
RÉSULTAT GLOBAL: PASS / FAIL
========================================
```

---

## 7. Procédure de Test Complète

### Étape 1 : Préparer l'environnement

```bash
# 1. Arrêter serveur en cours
taskkill /IM server.exe /F 2>nul

# 2. Mettre à jour versions
# config.json et package.json

# 3. Rebuild (ORDRE IMPORTANT : frontend PUIS backend)
cd server-go

# 3a. Frontend d'abord (OBLIGATOIRE)
cd web
npm run build
cd ..

# 3b. Backend Go ensuite (embarque les fichiers web)
go build -o server.exe ./cmd/server
```

**⚠️ IMPORTANT** : Toujours rebuilder le frontend AVANT le Go build. Le serveur Go embarque les fichiers web compilés.

### Étape 2 : Tests Unitaires

```bash
# Exécuter tous les tests
go test ./... -v -cover 2>&1 | tee test-report-$(date +%Y%m%d).txt

# Vérifier 0 FAIL
grep -c "FAIL" test-report-*.txt
# Doit retourner 0
```

### Étape 3 : Démarrer Serveur

```bash
# Lancer en mode visible (CMD/PowerShell)
./server.exe

# Vérifier démarrage
# - "HTTP server started on :80"
# - "WebSocket hub started"
```

### Étape 4 : Tests E2E Chrome

1. Ouvrir Chrome : http://localhost/quiz
2. Exécuter les scénarios E2E de la phase courante
3. Noter les résultats (PASS/FAIL)

### Étape 5 : Vérification JSON

```bash
# Vérifier structure question Memory
cat data/files/questions/[ID]/question.json | jq .
```

### Étape 6 : Générer Rapport

Remplir le template de rapport avec les résultats.

---

## 8. Historique des Phases

| Version | Phase | Tests Ajoutés | Date |
|---------|-------|---------------|------|
| 2.33.0 | Phase 1 | TestMemory* (8 models, 2 http) | En cours |
| 2.34.0 | Phase 2 | TestEngine_Memory* (3) | Planifié |
| 2.35.0 | Phase 3 | TestEngine_Memory* (8) | Planifié |

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
| `internal/server/ai_validate_test.go` | `POST /api/ai/validate-key` — taxonomie valid/invalid_key/unreachable, cooldown, sécurité (§9) |
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

#### Arrêt du serveur de test

> **Pourquoi** : `go test ./...` démarre un serveur réel sur le port **8080** pendant les tests d'intégration (`e2e_test.go`). Ce serveur reste actif après la fin des tests et bloque le port, empêchant le lancement du serveur Windows (`build.ps1`).

```bash
# Libérer le port 8080 après les tests
curl -s http://localhost:8080/shutdown
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

### Étape 7 : Redémarrer le Serveur

**⚠️ IMPORTANT** : Ne jamais laisser le serveur arrêté après les tests.

```bash
# Vérifier si le serveur tourne
curl -s http://localhost/version

# Si pas de réponse, redémarrer le serveur
cd server-go
./server.exe &
# ou en mode visible :
# start ./server.exe
```

Le serveur doit toujours être opérationnel à la fin de la procédure de test.

---

## 8. Historique des Phases

| Version | Phase | Tests Ajoutés | Date |
|---------|-------|---------------|------|
| 2.33.0 | Phase 1 | TestMemory* (8 models, 2 http) | En cours |
| 2.34.0 | Phase 2 | TestEngine_Memory* (3) | Planifié |
| 2.35.0 | Phase 3 | TestEngine_Memory* (8) | Planifié |
| 6.0.3 | #9 — Validation clé API IA | `TestValidateAPIKey_*` (31, `ai_validate_test.go`) | 2026-08-09 |

---

## 9. Validation de clé API IA à l'enregistrement (v6.0.3, #9)

> **Contrat** : `contracts/ai-key-validation.md`
> **Plan** : `_work/reports/plan-20260809-104602.md`
> **Maquette** : `_work/reports/mockup-ai-key-validation-20260809-104602.html`
> **Tests Go** : `internal/server/ai_validate_test.go` (31 tests, dérivés du contrat)

Une clé API bien formée mais révoquée/tronquée/du mauvais compte passait aujourd'hui
le seul contrôle de préfixe (`sk-ant-`/`gsk_`) et s'enregistrait sans broncher. Le
bouton **Enregistrer** appelle désormais `POST /api/ai/validate-key` (coût nul en
tokens, `GET /models`) avant toute écriture, et distingue **clé refusée**
(`invalid_key`, 401/403 → « corrige ta clé ») de **fournisseur injoignable**
(`unreachable`, réseau/timeout/5xx/429 → « réessaie plus tard »). Dans les deux cas
l'opérateur peut forcer l'enregistrement ; la clé est alors marquée persistante non
vérifiée (`*_api_key_verified: false`).

### 9.1 Tests unitaires Go (`ai_validate_test.go`)

| Section | Tests | Couverture |
|---------|-------|------------|
| A — Requête | `MethodNotAllowed`, `MalformedJSON`, `UnknownProvider`, `ProviderAbsent`, `BodyTooLarge`, `InvalidPrefix_*_NoNetworkCall` (×2) | Rejets avant tout appel réseau (contrat §5) |
| B — Taxonomie Anthropic | `Valid200`, `401_InvalidKey`, `403_InvalidKey`, `429_IsUnreachable_NeverInvalidKey`, `500_Unreachable`, `NetworkDown_Unreachable`, `Timeout_Unreachable`, `SlowButWithinTimeout_StillValid` | Les 3 issues de la taxonomie (contrat §3), y compris la règle 429 → `unreachable` |
| C — Taxonomie Groq | `Valid200`, `401_InvalidKey`, `429_IsUnreachable` | Échantillon — le code de classification est partagé avec Anthropic |
| D — Non-régression | `Anthropic/Groq_HitsModelsEndpoint_NotGenerate` | Prouve que c'est `/v1/models`/`/openai/v1/models` qui est appelé, jamais l'endpoint de génération |
| E — Clé effective | `EmptyAPIKey_UsesStoredKey`, `EmptyAPIKey_EnvVarTakesPriorityOverStored`, `EmptyAPIKey_GroqEnvVar`, `NoSideEffect_DoesNotMutateStoredConfig` | Contrat §5/§7 — champ vide ⇒ clé stockée puis variable d'environnement prioritaire ; aucune écriture par cet endpoint |
| F — Sécurité | `ResponseNeverContainsTheKey`, `DetailIsSanitized_KeyShapedSubstringRedacted`, `LogsNeverContainTheKey`, `ValidResult_DetailFieldOmittedFromJSON` | Contrat §8 — la clé ne fuite ni en réponse ni en logs ; `sanitizeUpstreamMessage` réellement branché sur ce chemin |
| G — Cooldown | `SecondCallWithin2s_Gets429`, `IsGlobal_NotPerProvider`, `AfterWindowElapses_Succeeds` | Contrat §8 — 1 validation / 2 s, global au serveur |

> ⚠️ **`TestValidateAPIKey_Anthropic_Timeout_Unreachable` dure ~10 s** (le plafond de
> timeout de ce chemin est codé en dur à 10 s par le contrat §2, délibérément non
> dérivé de `ai.timeout_seconds`, donc non raccourcissable côté test). Skippé sous
> `go test -short` ; s'exécute en entier avec la commande standard `go test ./...`.

### 9.2 Procédure manuelle QA — les 11 scénarios de la maquette

**Prérequis** :
- [ ] Environnement : QUALIF ou LOCAL, serveur démarré, accès admin sur `/config`
- [ ] Une vraie clé API Groq de test valide (tier gratuit, aucun coût) pour les
      scénarios impliquant un fournisseur réel — voir 9.3
- [ ] Config IA vierge (aucune clé Claude ni Groq) au départ du Scénario 1

| # | Scénario | Étapes | Attendu à l'écran | Attendu en base (`config.json`) | OK ? |
|---|----------|--------|-------------------|----------------------------------|------|
| 1 | Clé valide, fournisseur joignable | Coller une clé Groq **valide**, cliquer "Enregistrer" | Bref état "Vérification…" puis badge **✅ Clé vérifiée**, toast succès — **aucun dialogue** | `groq_api_key` écrite, `groq_api_key_verified: true` | |
| 2 | Clé refusée, puis "Corriger" | Coller une clé au bon préfixe mais invalide (ex : révoquée ou tronquée), "Enregistrer", puis cliquer **"Corriger la clé"** dans le dialogue | Dialogue **refus** ("[Fournisseur] a refusé cette clé"), puis retour au champ marqué en erreur | **Rien n'est écrit** — clé précédente (ou absence de clé) intacte, `*_verified` inchangé | |
| 3 | Clé refusée, puis "Enregistrer quand même" | Même point de départ que #2, cliquer **"Enregistrer quand même"** | Badge **⚠️ Clé non vérifiée**, toast d'avertissement | Clé écrite, `*_api_key_verified: false` | |
| 4 | Fournisseur injoignable, "Réessayer" qui réussit | Couper temporairement l'accès réseau sortant (ou pointer un environnement de test sans accès Internet), "Enregistrer" → dialogue injoignable → rétablir le réseau → cliquer **"Réessayer"** | Dialogue se ferme, badge **✅ Clé vérifiée** | Clé écrite, `*_verified: true` | |
| 5 | Fournisseur répond 429 | (Test backend suffisant — `TestValidateAPIKey_*_429_IsUnreachable*`) Si testé manuellement : rafale de validations rapprochées côté fournisseur | Dialogue **injoignable**, **jamais** "refusée" | Rien tant que non forcé | |
| 6 | Champ vide, clé déjà enregistrée | Laisser le champ clé vide, cliquer "Enregistrer" (clé déjà présente en base) | La clé stockée est vérifiée, badge mis à jour en conséquence | Clé inchangée, `*_verified` actualisé | |
| 7 | Champ vide, clé issue de `BUZZCONTROL_*_API_KEY` | Démarrer le serveur avec `BUZZCONTROL_GROQ_API_KEY` positionnée, champ clé vide côté UI, cliquer "Enregistrer" | La clé d'environnement est vérifiée, badge mis à jour | **Aucun secret écrit sur disque** (`config.json` ne contient pas la clé d'environnement) | |
| 8 | Champ vide, aucune clé nulle part | Config vierge, champ vide, cliquer "Enregistrer" | Aucune vérification déclenchée (pas de "Vérification…", pas de dialogue) — comportement actuel inchangé | Autres réglages `ai.*` enregistrés normalement | |
| 9 | "Supprimer la clé" | Avec une clé enregistrée (vérifiée ou non), cliquer "Supprimer la clé" | Aucune vérification déclenchée (retour immédiat, pas d'appel réseau observable) | Clé effacée, `*_verified: false` | |
| 10 | Préfixe invalide | Saisir `foo-123` (ni `sk-ant-` ni `gsk_`), cliquer "Enregistrer" | Erreur de format immédiate (pas d'état "Vérification…") | Rien écrit, **aucun appel réseau** (vérifiable via les logs serveur — aucune ligne "AI key validation") | |
| 11 | Rechargement après enregistrement forcé | Suite du scénario #3, recharger `/config` (F5) | Badge **⚠️ Clé non vérifiée** persiste (ne redevient PAS "✅ Clé vérifiée") | `*_verified: false` relu du disque | |

**Verdict** : [ ] PASS  [ ] FAIL

### 9.3 Test avec une vraie clé Groq (tier gratuit, coût nul)

**Objectif** : valider le scénario #1/#2/#4 ci-dessus contre le **vrai** fournisseur
Groq (pas un mock), et confirmer par la console fournisseur qu'aucun token n'est
consommé par la validation (contrat §2 : `GET /models`, pas un appel de génération).

| Étape | Action | Résultat Attendu | OK ? |
|-------|--------|-------------------|------|
| 1 | Noter l'usage token actuel sur [console.groq.com](https://console.groq.com) (onglet Usage/Billing) | Valeur de référence notée | |
| 2 | Coller une clé Groq **valide** dans `/config`, cliquer "Enregistrer" | Badge **✅ Clé vérifiée** | |
| 3 | Rafraîchir la console Groq (Usage/Billing) | **Aucune variation** du compteur de tokens/requêtes de génération par rapport à l'étape 1 | |
| 4 | Supprimer la clé, coller la **même clé tronquée** (retirer les 4 derniers caractères), "Enregistrer" | Dialogue **refus** ("Groq a refusé cette clé") | |
| 5 | Rafraîchir la console Groq | Toujours **aucune variation** de tokens (le refus n'a pas consommé de quota) | |
| 6 | Couper l'accès réseau sortant du poste de test, "Enregistrer" avec la clé valide | Dialogue **injoignable** ("Impossible de joindre Groq"), dans un délai ≤ ~10 s | |
| 7 | Rétablir le réseau, cliquer "Réessayer" | Dialogue se ferme, badge **✅ Clé vérifiée** | |

**Verdict** : [ ] PASS  [ ] FAIL

### 9.4 Non-régression

- [ ] Le chemin de génération (`✨ Générer via IA`) fonctionne toujours à l'identique — aucun changement de comportement, de payload ni de latence perceptible
- [ ] `POST /config.json` sans les 2 nouveaux champs (`*_api_key_verified`) se comporte exactement comme avant (client qui ne les envoie pas)
- [ ] Le sélecteur de fournisseur Claude/Groq (bugfix/config-api-key-help, #7) n'est pas affecté — aucune validation déclenchée au changement de fournisseur
- [ ] `go test ./... -v -cover` — suite complète toujours verte (0 FAIL)

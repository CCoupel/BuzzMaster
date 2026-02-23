# Rapport QA - BuzzControl v3.1.5

**Date :** 2026-02-20
**Branche :** `claude/review-backlog-psIqt`
**Version :** 3.1.5
**Statut Global :** NOT VALIDATED
**Duree d'execution :** ~15 minutes

---

## 1. Resume Executif

La procedure QA complete a ete executee sur la version 3.1.5 de BuzzControl. Le build frontend et backend reussit sans erreur. Cependant, **9 tests echouent** sur 4 packages differents, dont des tests E2E critiques et des tests unitaires sur la machine d'etat du jeu. La couverture du package `internal/game` est de 33.1%, bien en dessous du seuil minimum de 60%. Le package `internal/server` est a 38.5%.

La cause racine principale est l'introduction d'un **countdown de 3 secondes** avant le demarrage du jeu (feature v3.1.x), sans mise a jour correspondante des tests existants qui supposent un demarrage immediat. D'autres echecs revelent des divergences entre les comportements attendus par les tests et l'implementation actuelle (ClearBumpers, Reveal, /questions API, /backup route).

---

## 2. Build

### 2.1 Frontend (npm run build)

| Element | Resultat |
|---------|---------|
| Commande | `cd server-go/web && npm run build` |
| Statut | PASS |
| Duree | 2.41s |
| Modules transformes | 492 |
| Taille index.js | 587 KB (gzip: 179 KB) |
| Avertissement | Chunk > 500 KB (non bloquant) |

### 2.2 Backend Go (go build)

| Element | Resultat |
|---------|---------|
| Commande | `go build -o server.exe ./cmd/server` |
| Statut | PASS |
| Taille du binaire | 19.9 MB |
| Avertissements | Aucun |

### 2.3 Verification serveur

| Page | HTTP Status |
|------|-------------|
| http://localhost/ | 200 OK |
| http://localhost/anim | 200 OK |
| http://localhost/tv | 200 OK |
| /version | "3.1.5" |

---

## 3. Tests Unitaires

### 3.1 Resultats par package

| Package | Tests PASS | Tests FAIL | Couverture | Seuil |
|---------|-----------|-----------|------------|-------|
| `internal/config` | 12 | 0 | 71.4% | 60% - OK |
| `internal/game` | 53 | 3 | 33.1% | 60% - **ECHEC** |
| `internal/protocol` | 12 | 0 | 90.7% | 60% - OK |
| `internal/server` | 33 | 6 | 38.5% | 60% - **ECHEC** |
| **TOTAL** | **110** | **9** | **~45%** | **60% - ECHEC** |

### 3.2 Tests en echec - Detail

#### FAIL: TestEngine_ClearBumpers (`engine_test.go:528`)

**Message :** `Team should be cleared`

**Analyse :** Le test appelle `ClearBumpers()` et attend que l'equipe soit supprimee (`GetTeam("red") == nil`). Mais l'implementation de `ClearBumpers()` ne supprime que les buzzers et dissocie les equipes de leurs buzzers (reset `Bumper`, `Time`, `Status`, `Ready`). Les equipes restent dans `e.data.Teams`. Le test suppose un comportement de "ClearAll" alors que le code fait intentionnellement un "clear buzzers only".

**Impact :** Moyen - Le comportement reel (conserver les equipes) est probablement voulu. Le test est mal aligne.

---

#### FAIL: TestEngine_Reveal (`engine_test.go:570`)

**Message :** `Expected answer 42, got ""`

**Log :** `[Engine] Cannot reveal from phase PREPARE (must be STOPPED or PAUSED)`

**Analyse :** Le test appelle `Ready("q1", &Question{Answer: "42"})` puis immediatement `Reveal()`. La phase apres `Ready()` est `PREPARE`, mais `Reveal()` requiert `STOPPED` ou `PAUSED`. Le test n'effectue pas de Start/Stop avant d'appeler Reveal.

**Impact :** Moyen - La restriction de phase pour `Reveal` est une contrainte metier; le test ne reproduit pas un scenario de jeu valide.

---

#### FAIL: TestFullGameState_ToJSON (`models_test.go:280`)

**Message :** `PHASE mismatch: STARTED`

**Analyse :** Le test cree un `FullGameState` avec `Phase: PhaseStarted` et attend que le JSON contienne `"PHASE": "START"`. Mais la constante `PhaseStarted` vaut `"STARTED"`, pas `"START"`. Le test attend une valeur incorrecte selon le modele de donnees existant.

**Impact :** Faible - Le test contient une erreur de valeur attendue.

---

#### FAIL: TestE2E_SingleBuzzerGameFlow (`e2e_test.go:194, 207`)

**Message :**
- `Game should be started`
- `Button press time should be recorded`

**Log :** `[Engine] Starting 3-second countdown before game (delay=30)`

**Analyse :** Le test appelle `engine.Start(30)` et s'attend a ce que le jeu soit immediatement en phase `STARTED`. Depuis l'introduction du countdown 3-2-1, `Start()` place d'abord le moteur en phase `COUNTDOWN` pendant 3 secondes avant de passer en `STARTED`. Le test ne prend pas en compte ce delai et echoue instantanement.

**Impact :** Critique - Ce test E2E couvre le flux complet d'un buzzer TCP.

---

#### FAIL: TestE2E_GameStateMachine (`e2e_test.go:337`)

**Message :** `Should be in START phase`

**Log :** `[Engine] Starting 3-second countdown before game (delay=10)`

**Analyse :** Meme cause que `TestE2E_SingleBuzzerGameFlow`. Le test de la machine d'etat appelle `Start(10)` et attend une transition immediate vers la phase `STARTED`. Le countdown bloque cette transition.

**Impact :** Critique - Ce test couvre la machine d'etat complete du jeu (PREPARE -> READY -> START -> PAUSE -> STOP).

---

#### FAIL: TestHTTPServer_Questions_Empty (`http_test.go:108`)

**Message :** `Response is not valid JSON: json: cannot unmarshal object into Go value of type []interface {}`

**Analyse :** Le test attend un tableau JSON (`[]interface{}`). L'API `/questions` retourne desormais un objet JSON (probablement `{"questions": [...], ...}`). L'endpoint a change de format de reponse depuis l'ecriture du test.

**Impact :** Important - Le changement de format de l'API `/questions` peut casser des clients existants.

---

#### FAIL: TestHTTPServer_Questions_WithData (`http_test.go:138, 142`)

**Message :**
- `Response is not valid JSON: json: cannot unmarshal object into Go value of type []map[string]interface {}`
- `Expected 1 question, got 0`

**Analyse :** Meme cause que `TestHTTPServer_Questions_Empty`. Le format de reponse de `/questions` a change.

**Impact :** Important - Meme probleme API que ci-dessus.

---

#### FAIL: TestHTTPServer_Backup (`http_test.go:439`)

**Message :** `Expected 501 Not Implemented, got 302`

**Analyse :** Le test attend un `501 Not Implemented` pour `GET /backup` (endpoint non implemente). L'endpoint retourne desormais un `302 Found` (redirection). La route `/backup` a ete implementee (redirection vers la page backup React) alors que le test supposait qu'elle n'existait pas.

**Impact :** Moyen - Le test est obsolete. L'implementation a avance.

---

#### FAIL: TestHTTPServer_Restore (`http_test.go:453`)

**Message :** `Expected 501 Not Implemented, got 400`

**Analyse :** Meme situation que `/backup`. L'endpoint `POST /restore` existait comme "non implemente" mais retourne maintenant un `400 Bad Request` (corps vide invalide), ce qui indique que l'endpoint a ete implemente.

**Impact :** Moyen - Le test est obsolete.

---

## 4. Tests E2E

| Test | Statut | Motif |
|------|--------|-------|
| `TestE2E_SingleBuzzerGameFlow` | FAIL | Countdown 3s non pris en compte |
| `TestE2E_WebSocketClient` | PASS | - |
| `TestE2E_GameStateMachine` | FAIL | Countdown 3s non pris en compte |
| `TestE2E_HTTPWithEngine` | PASS | - |

---

## 5. Couverture de Code

| Package | Couverture | Seuil | Verdict |
|---------|-----------|-------|---------|
| `internal/config` | 71.4% | 60% | OK |
| `internal/game` | 33.1% | 60% | ECHEC |
| `internal/protocol` | 90.7% | 60% | OK |
| `internal/server` | 38.5% | 60% | ECHEC |
| `cmd/server` | 0.0% | N/A | Normal (entrypoint) |

### 5.1 Fonctions les moins couvertes (server)

| Fichier | Fonction | Couverture |
|---------|---------|------------|
| `websocket.go:124` | `GetClientCounts` | 0.0% |
| `websocket.go:141` | `SetClientType` | 0.0% |
| `websocket.go:167` | `BroadcastRaw` | 0.0% |
| `websocket.go:172` | `SendToClient` | 0.0% |
| `websocket_buzzer.go:95` | `ConnectedCount` | 0.0% |
| `websocket_buzzer.go:151` | `IsClientConnected` | 0.0% |
| `config.go:153` | `Get` | 0.0% |
| `config.go:201` | `SetInstance` | 0.0% |

---

## 6. Linting et Formatage

### 6.1 go vet

| Resultat | Detail |
|---------|--------|
| Statut | PASS |
| Erreurs | 0 |
| Avertissements | 0 |

### 6.2 gofmt

| Resultat | Detail |
|---------|--------|
| Statut | 23 fichiers non formates |
| Fichiers | `config.go`, `engine.go`, `models.go`, `messages.go`, `http.go`, etc. |

Note : `golangci-lint` n'est pas installe sur l'environnement. Seul `go vet` a ete utilise.

---

## 7. Regression

### 7.1 Fonctionnalites existantes

| Fonctionnalite | Statut | Notes |
|----------------|--------|-------|
| Build et demarrage serveur | OK | v3.1.5 repond correctement |
| WebSocket client (HELLO) | OK | TestE2E_WebSocketClient passe |
| API /version | OK | Retourne "3.1.5" |
| API /config GET/POST | OK | Tests passent |
| Upload questions | OK | TestHTTPServer_QuestionUpload passe |
| Questions Memory | OK | TestHTTPServer_MemoryQuestionUpload/Load passent |
| API buzzers | OK | Tests APIBuzzers passent |
| TCP protocol | PARTIEL | Tests TCP passent mais E2E echoue a cause du countdown |
| Machine d'etat PAUSE/CONTINUE/STOP | PARTIEL | La sequence de transitions echoue au START a cause du countdown |

### 7.2 Regressions detectees

| Regression | Gravite | Description |
|-----------|---------|-------------|
| Format API /questions | Important | L'API retourne un objet au lieu d'un tableau |
| Demarrage immediat du jeu | Important | `Start()` lance un countdown 3s, bloquant les tests E2E |
| Routes /backup et /restore | Mineur | Endpoints implementes, tests attendent encore 501 |

---

## 8. Problemes Bloquants

### 8.1 Niveau Critique

| ID | Type | Description | Impact |
|----|------|-------------|--------|
| B1 | Regression test | Countdown 3s non couvert par TestE2E_SingleBuzzerGameFlow et TestE2E_GameStateMachine | Les tests E2E du flux principal echouent |

### 8.2 Niveau Important

| ID | Type | Description | Impact |
|----|------|-------------|--------|
| B2 | Regression API | Format de reponse /questions modifie (objet vs tableau) | Clients potentiellement casses |
| B3 | Couverture | internal/game: 33.1% (seuil: 60%) | Zone de risque non testee |
| B4 | Couverture | internal/server: 38.5% (seuil: 60%) | Zone de risque non testee |

### 8.3 Niveau Mineur

| ID | Type | Description | Impact |
|----|------|-------------|--------|
| B5 | Test obsolete | TestEngine_ClearBumpers: comportement different de l'attendu | Test a corriger |
| B6 | Test obsolete | TestEngine_Reveal: scenario de test invalide | Test a corriger |
| B7 | Test obsolete | TestFullGameState_ToJSON: valeur attendue incorrecte | Test a corriger |
| B8 | Test obsolete | TestHTTPServer_Backup/Restore: endpoints maintenant implementes | Tests a mettre a jour |
| B9 | Formatage | 23 fichiers non formates (gofmt) | Qualite code |

---

## 9. Recommandations

### Actions obligatoires avant QUALIF

1. **[B1] Mettre a jour les tests E2E pour le countdown**
   - `TestE2E_SingleBuzzerGameFlow` : ajouter `time.Sleep(4 * time.Second)` apres `Start()`, ou utiliser un moteur de test avec countdown desactive (delay=0)
   - `TestE2E_GameStateMachine` : meme correction

2. **[B2] Verifier le changement de format /questions**
   - Si intentionnel, mettre a jour les tests `TestHTTPServer_Questions_Empty` et `TestHTTPServer_Questions_WithData`
   - Si non intentionnel, restaurer le format tableau

3. **[B3, B4] Augmenter la couverture de tests**
   - Ecrire des tests pour les fonctions WebSocket non couvertes (`GetClientCounts`, `SetClientType`, `BroadcastRaw`, `SendToClient`)
   - Cibler minimum 60% par package

4. **[B5-B8] Corriger les tests obsoletes**
   - `TestEngine_ClearBumpers` : adapter l'assertion ou clarifier le comportement attendu
   - `TestEngine_Reveal` : ajouter les transitions Start/Stop avant Reveal
   - `TestFullGameState_ToJSON` : corriger "START" en "STARTED"
   - `TestHTTPServer_Backup/Restore` : mettre a jour selon le comportement actuel

### Ameliorations suggerees

- Installer `golangci-lint` pour un linting plus complet
- Executer `gofmt -w .` sur l'ensemble du codebase
- Ajouter un flag de configuration pour desactiver le countdown en mode test

---

## 10. Decision Finale

### NOT VALIDATED

**Motifs :**

1. 9 tests echouent sur 4 packages (seuil: 0 echec)
2. Couverture `internal/game`: 33.1% (seuil minimum: 60%)
3. Couverture `internal/server`: 38.5% (seuil minimum: 60%)
4. 2 tests E2E critiques echouent (flux principal TCP et machine d'etat)
5. Regression probable sur le format de l'API `/questions`

Bien que le build reussisse et que le serveur fonctionne correctement en production, les tests revelent des divergences significatives entre l'implementation et les specifications attendues, notamment autour de la feature de countdown introduite en v3.1.x.

---

## 11. Serveur

| Element | Detail |
|---------|--------|
| Statut | Actif |
| Version | 3.1.5 |
| URL Player | http://localhost/ |
| URL Admin | http://localhost/anim |
| URL TV | http://localhost/tv |

---

## 12. Synthese pour Validation Utilisateur

### Ce qui a ete implemente (v3.1.5)

Ajout d'un systeme de configuration WiFi fallback (SSID2/PASS2) pour les buzzers, avec synchronisation automatique via l'API `/api/buzzer/wifi-config` et diffusion broadcast depuis la page Config.

### Tests de Non-Regression

| Fonctionnalite existante | Statut |
|--------------------------|--------|
| Build et demarrage serveur | OK |
| Page / (Player) | OK |
| Page /anim (Admin) | OK |
| Page /tv (TV Display) | OK |
| API /version | OK |
| Upload questions NORMAL | OK |
| Questions Memory (upload/lecture) | OK |
| API /config GET/POST | OK |
| WebSocket client HELLO | OK |
| API /api/buzzers | OK |

### Tests de la Nouvelle Fonctionnalite

| Test | Statut |
|------|--------|
| Build avec la feature WiFi2 | OK (compile) |
| Serveur demarre correctement | OK |
| API /api/buzzer/wifi-config accessible | Non verifie (test manuel requis) |

### Comment Tester Manuellement (3 etapes)

1. **Acceder a la Config** : Aller sur http://localhost/anim, cliquer sur le menu abeille, puis "Config"
2. **Verifier les champs WiFi2** : Dans la section WiFi, les champs SSID2 et Mot de passe 2 doivent etre visibles
3. **Tester le broadcast** : Cliquer sur le bouton "Diffuser config WiFi" et verifier qu'aucune erreur n'apparait

### Resultat Attendu

Les champs WiFi de secours (SSID2/PASS2) s'affichent dans la page Config. Le bouton de broadcast envoie la configuration a tous les buzzers connectes sans erreur dans les logs.

---

## Annexe : Logs Complets

### Tests unitaires game (echecs)

```
=== RUN   TestEngine_ClearBumpers
[Engine] Updated bumper b1: team=red, name=, protocol=
[Engine] All bumpers cleared and dissociated from teams
    engine_test.go:528: Team should be cleared
--- FAIL: TestEngine_ClearBumpers (0.00s)

=== RUN   TestEngine_Reveal
[Engine] Game ready with question: q1
[Engine] Cannot reveal from phase PREPARE (must be STOPPED or PAUSED)
    engine_test.go:570: Expected answer 42, got
--- FAIL: TestEngine_Reveal (0.00s)

=== RUN   TestFullGameState_ToJSON
    models_test.go:280: PHASE mismatch: STARTED
--- FAIL: TestFullGameState_ToJSON (0.00s)
```

### Tests E2E (echecs)

```
=== RUN   TestE2E_SingleBuzzerGameFlow
[Engine] Starting 3-second countdown before game (delay=30)
    e2e_test.go:194: Game should be started
    e2e_test.go:207: Button press time should be recorded
--- FAIL: TestE2E_SingleBuzzerGameFlow (0.63s)

=== RUN   TestE2E_GameStateMachine
[Engine] Starting 3-second countdown before game (delay=10)
    e2e_test.go:337: Should be in START phase
--- FAIL: TestE2E_GameStateMachine (0.00s)
```

### Tests HTTP (echecs)

```
=== RUN   TestHTTPServer_Questions_Empty
    http_test.go:108: Response is not valid JSON: json: cannot unmarshal object into Go value of type []interface {}
--- FAIL: TestHTTPServer_Questions_Empty (0.00s)

=== RUN   TestHTTPServer_Questions_WithData
    http_test.go:138: Response is not valid JSON: json: cannot unmarshal object into Go value of type []map[string]interface {}
    http_test.go:142: Expected 1 question, got 0
--- FAIL: TestHTTPServer_Questions_WithData (0.00s)

=== RUN   TestHTTPServer_Backup
    http_test.go:439: Expected 501 Not Implemented, got 302
--- FAIL: TestHTTPServer_Backup (0.00s)

=== RUN   TestHTTPServer_Restore
    http_test.go:453: Expected 501 Not Implemented, got 400
--- FAIL: TestHTTPServer_Restore (0.00s)
```

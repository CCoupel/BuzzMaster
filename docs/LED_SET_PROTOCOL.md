# LED_SET Protocol — Server-Driven LED Control (v3.4.0)

## 1. Motivation

Avant v3.4.0, le firmware calculait l'état LED depuis l'état de jeu reçu dans chaque message `UPDATE`. Cette approche posait plusieurs problèmes :

- **Bugs de timing** : `UPDATE_TIMER` était émis ~10 fois par seconde. À chaque UPDATE, le firmware recalculait l'état LED, causant des flashs parasites (ex: retour bref à la couleur équipe pendant un QCM actif entre deux UPDATE).
- **Logique dupliquée** : La logique LED était implémentée à la fois dans le firmware (calcul depuis UPDATE) et dans le serveur (envoi des actions spécialisées `QCM_COLOR`, `QCM_DIM`, etc.), créant des divergences.
- **Couplage fort** : Tout ajout d'effet ou de phase de jeu nécessitait un OTA firmware, rendant les évolutions lentes et risquées.

## 2. Principe

Le serveur est l'unique source de vérité pour l'état LED pendant le jeu.

- **Firmware** : applique l'action `LED_SET` reçue du serveur. Conserve uniquement les animations autonomes qui ne dépendent pas de l'état de jeu : boot sequence (phases 1-6) et grey rotation (buzzer sans équipe assignée).
- **Serveur** : calcule et envoie `LED_SET` à chaque changement d'état pertinent (transition de phase, buzz, révélation, etc.). N'envoie plus de `UPDATE_TIMER` comme déclencheur LED.

## 3. Action LED_SET

```
Server → Buzzer (WebSocket /ws/buzzer, per-buzzer ou broadcast)
```

Payload JSON :
```json
{
  "COLOR": [255, 0, 0],
  "INTENSITY": 100,
  "EFFECT": "SOLID"
}
```

Champs :

- `COLOR` : `[R, G, B]` entiers 0-255
- `INTENSITY` : entier 0-255 (255 = pleine intensité)
- `EFFECT` : `"SOLID"` | `"BLINK"` | `"DIM"`
  - `SOLID` : steady, intensité exacte
  - `BLINK` : 100%↔25% à 400ms (ex: bonne réponse QCM)
  - `DIM` : steady à l'intensité donnée (alias sémantique de SOLID, utilisé pour indiquer une atténuation intentionnelle)

## 4. Suppression des actions spécialisées

Les actions LED dédiées sont supprimées et remplacées par `LED_SET` :

| Action supprimée | Remplacement LED_SET |
|------------------|----------------------|
| `QCM_COLOR` | `LED_SET` SOLID 100% couleur réponse |
| `QCM_DIM` | `LED_SET` DIM 25% couleur réponse |
| `QCM_REVEAL` correct | `LED_SET` BLINK couleur réponse |
| `QCM_REVEAL` wrong buzzed | `LED_SET` SOLID 100% couleur réponse |
| `QCM_REVEAL` not buzzed | `LED_SET` DIM 10% couleur réponse |
| `QCM_RESET` | `LED_SET` SOLID 100% couleur équipe |

## 5. Règle d'acceptation des buzz

Un appui buzzer n'est pris en compte par le serveur **que si la phase est `STARTED`** (timer en cours).

En phase `READY`, `COUNTDOWN`, `PAUSED`, `REVEALED` ou `STOPPED`, le serveur ignore les messages `BUTTON` entrants. Le firmware ne filtre pas — le filtrage est exclusivement serveur-side.

## 6. Tables des états LED

> Voir **[BUZZER_LED_STATE_MACHINE.md](BUZZER_LED_STATE_MACHINE.md)** pour les tables complètes par type de jeu (NORMAL, QCM, MEMORY) et état de buzz.

## 7. Persistance d'état et reconnexion

Le serveur maintient un `bumperLEDState map[string]LEDSetPayload` (mac → état courant).

Mise à jour : à chaque appel à `sendLEDSet(mac, payload)` ou `broadcastLEDSet(payload)`, l'état est enregistré dans la map.

Reconnexion : à la réception d'un message `HELLO` d'un buzzer en cours de partie, le serveur renvoie immédiatement le dernier état LED connu pour ce buzzer via `resendLEDOnReconnect(mac)`. Si aucun état n'est connu (premier HELLO de la session), le serveur calcule l'état courant depuis la phase de jeu active.

## 8. Architecture Go

```go
// protocol/messages.go
ActionLEDSet = "LED_SET"

type LEDSetPayload struct {
    Color     [3]int `json:"COLOR"`
    Intensity int    `json:"INTENSITY"`
    Effect    string `json:"EFFECT"`  // "SOLID", "BLINK", "DIM"
}

// app (main.go ou engine_leds.go)
type App struct {
    // ...
    bumperLEDState map[string]protocol.LEDSetPayload  // mac → current LED state
}

func (a *App) sendLEDSet(mac string, payload protocol.LEDSetPayload)
func (a *App) broadcastLEDSet(payload protocol.LEDSetPayload)  // tous les buzzers
func (a *App) resendLEDOnReconnect(mac string)                  // appelé sur HELLO
```

## 9. Architecture Firmware (BuzzClick)

### Supprimé

Variables d'état QCM :
- `qcmActive`, `qcmR`, `qcmG`, `qcmB`, `qcmDimmed`, `qcmBuzzed`, `qcmCorrect`, `qcmBlinking`

Fonctions :
- `manageLeds()` — calcul LED depuis état UPDATE
- `manageLedQCMBlink()` — animation blink spécifique QCM

Handlers de messages :
- `QCM_COLOR`, `QCM_DIM`, `QCM_REVEAL`, `QCM_RESET`

### Ajouté dans `parseJSON()`

```cpp
case hash("LED_SET"):
{
    JsonArray colorArr = message["COLOR"].as<JsonArray>();
    int intensity = message["INTENSITY"] | 255;
    const char* effect = message["EFFECT"] | "SOLID";

    int r = colorArr[0]; int g = colorArr[1]; int b = colorArr[2];
    setLedColor(r, g, b);
    setLedIntensity(intensity);

    ledBlinking = (strcmp(effect, "BLINK") == 0);
    if (ledBlinking) { ledBlinkOn = true; ledLastBlink = millis(); }
}
```

### Ajouté dans `loop()`

`manageLedBlink()` — remplace `manageLedQCMBlink()`, générique (déclenché si `ledBlinking == true`)

### Conservé firmware-side

- Boot sequence phases 1-6 (autonome, avant connexion serveur)
- Grey rotation (buzzer sans équipe assignée, calculé localement)
- Feedback LED connexion/déconnexion WebSocket

## 10. Impact réseau

| Métrique | Avant | Après |
|----------|-------|-------|
| Déclencheur LED | UPDATE_TIMER ~10/s | Event-driven (changement d'état) |
| Fréquence LED msgs | ~10/s | ~5-10/partie |
| Payload LED | ~800 bytes (UPDATE complet) | ~50 bytes (LED_SET) |
| Charge totale | Élevée | Réduite |

---

## 11. Résolution LED exacte via COLOR_NAME (v5.7.25, #113)

### Contexte

Avant v5.7.25, la couleur LED d'un buzzer était résolue par approximation de teinte (`nearestPaletteColorByHue`) en l'absence de champ `COLOR_NAME`. Ceci causait une divergence :
- Affichage écran (UI admin/TV) : couleur exacte de palette (16 valeurs)
- LED buzzer : approximation (seulement 8 ancres de teinte)

### Solution : COLOR_NAME

Nouveau champ optionnel `Team.COLOR_NAME` (string, ex: `"rouge"`, `"bleu-profond"`). Le frontend l'écrit à chaque sélection manuelle ou attribution automatique. Permet résolution LED exacte au lieu d'approximation.

### Implémentation backend

**Fonction `teamColorToRGB(colorName string, fallbackRGB [3]int) [3]int`** :
1. Si `colorName` présent dans table `teamColorPalette` → retourne RGB exact
2. Sinon → utilise `fallbackRGB` (brute-force exact, ou recalc par teinte si absent)

**Table `teamColorPalette`** : 16 entrées {clé, RGB}, rangs 1→16 (8 tons vifs + 8 tons profonds).

```go
// Exemple d'entrée
{"rouge", [3]int{255, 26, 26}},
{"bleu-profond", [3]int{0, 54, 179}},
```

**Rétrocompatibilité** : équipes sans `COLOR_NAME` (pré-v5.7.25) continuent à fonctionner via `nearestPaletteColorByHue(rgb)` — fallback par teinte, pas idéal mais fonctionnel.

### Atténuation relative au ton — `dimIntensityFor()` (v5.7.25, #113)

Nouvelle fonction `dimIntensityFor(rgb [3]int) int` calcule intensité depuis luminosité HSL :

- **Tons vifs** (L≈55%, ex: rouge/bleu) → Intensity ~64 (comportement préexistant NORMAL)
- **Tons profonds** (L≈35%, ex: bleu-profond) → Intensity ~100 (plus clair, sinon quasi-éteint)
- **Borné** : [64, 128]

Formule : `Intensity = 163 - 180×L` (borné)

### Sites d'application de `dimIntensityFor()`

1. **`sendLEDSetForBuzzerNormal`** : mode NORMAL, équipes non-buzzées (état STARTED/PAUSED/REVEALED). Cible la couleur d'équipe seulement.

2. **`sendLEDSetForBuzzerMemory`** : mode MEMORY
   - Branche `PAUSED` (tous buzzers) → `dimIntensityFor(team.COLOR)`
   - Branche `STARTED+SOLO` inactive → `dimIntensityFor(team.COLOR)`
   - Branche multi-équipes, « autres équipes participantes » (DIM 25%) → `dimIntensityFor(team.COLOR)`

### Sites **NON** modifiés — QCM conserve intensité fixe

- **`sendLEDSetForBuzzerQCM`** : intensité fixe 64 sur `answerRGB` (couleur de réponse, jamais couleur d'équipe)
- **`sendLEDSetForBuzzerQCMReveal`** : idem

Rationale : l'atténuation en ton profond ne doit s'appliquer qu'à la **couleur d'équipe**, jamais à la couleur de réponse QCM (2 feedback distincts : intensité atténuée = position inactive en mode jeu, blink = réponse juste).

---

## 12. Effet COMET (v3.7.0)

Bande lumineuse rotative sur 23 LEDs, 2 tours ~3.3 s. Déclenché sur attribution de points (`TEAM_POINTS`/`BUMPER_POINTS`).

```json
{ "ACTION": "LED_SET", "MSG": { "COLOR": [255, 0, 0], "INTENSITY": 255, "EFFECT": "COMET", "COMET_COLOR": [255, 215, 0] } }
```

- `COMET_COLOR` : optionnel — or `[255,215,0]` ou blanc `[255,255,255]` selon contraste avec couleur équipe (dist² euclidien RGB < 8000 → blanc)
- Sélection couleur LED par teinte : `nearestPaletteColorByHue()` — distance HSL pour cohérence palette
- Firmware : `manageLedComet()` dans `loop()` (`click_serverConnection.h`)

**Logique serveur par phase** :
| Phase | LED envoyée |
|-------|-------------|
| READY/START QCM | couleur réponse SOLID 100% (per-buzzer) |
| READY/START NORMAL/MEMORY | couleur équipe SOLID 100% |
| PAUSE (buzz QCM) | buzzer actif DIM 64, autres inchangés |
| PAUSE ALL | couleur équipe DIM 64 pour tous |
| STOP | couleur équipe SOLID 100% pour tous |
| REVEALED QCM | correct=BLINK, wrong-buzzed=SOLID, non-buzzé=DIM 25 |
| Attribution points | COMET avec COMET_COLOR dynamique sur buzzers de l'équipe |

**Patterns d'erreur LED locaux** (firmware, serveur injoignable) :
| Pattern | Visuel | Déclencheur |
|---------|--------|-------------|
| `WIFI_FAILED` | Rouge clignotant 1 Hz | WiFi non associé |
| `WS_TIMEOUT` | Rouge pulsant ~0.5 Hz | Timeout connexion boot |
| `WS_RECONNECTING` | 1 pixel blanc tournant 100ms/step | Reconnexion en cours |
| `OTA_ERROR` | Rouge fixe + flash blanc 2s | Échec OTA |

`WS_RECONNECTING` : delta-update 2 pixels/tick, préserve ring couleur équipe. Teardown non-bloquant via `ws_destroy_task` FreeRTOS (v3.6.4).

---

## 12. Protocole ACK pour LED_SET (v3.8.0)

Le serveur inclut un `MSG_ID` dans les `LED_SET` critiques. Le buzzer ACK avant d'appliquer.

```json
// Server → Buzzer
{ "ACTION": "LED_SET", "MSG_ID": "a1b2c3d4e5f6", "MSG": { "COLOR": [255,0,0], "INTENSITY": 255, "EFFECT": "SOLID" } }

// Buzzer → Server
{ "ACTION": "ACK", "MSG": { "ack_action": "LED_SET", "ack_id": "a1b2c3d4e5f6" } }
```

- `AckManager` : retry auto + expiry après 3 tentatives (timeout 2000ms par défaut)
- `MSG_ID` avec `omitempty` → rétrocompatible anciens firmwares
- `Bumper.ACK_PENDING` : `true` pendant attente, `false` sur ACK ou expiry

Fichier : `internal/server/ack_manager.go`, `src/BuzzClick/click_websocket_espidf.h` (`ws_sendAck()`)

---

## 13. LED État ENTRACTE — Extinction (v6.5.2, #119)

### Transition

**ENTRACTE → OFF (Extinction)**
- À l'activation (`ENTRACTE_SET {ACTIVE: true}`) : tous les buzzers reçoivent un 
  payload LED OFF immédiatement.
- Payload : `{R: 0, G: 0, B: 0, intensity: 0, mode: SOLID, duration: -1}`.
- Pendant l'entracte : LEDs restent OFF.
- À la désactivation (`ENTRACTE_SET {ACTIVE: false}`) : LEDs restaurées à l'état 
  correspondant à la phase courante (voir table des états LED ci-dessus).

### Reconnexion pendant l'ENTRACTE

Quand un buzzer se reconnecte (ou établit sa première connexion) pendant une pause 
en cours, il reçoit immédiatement un payload LED OFF — garantit que les LEDs ne 
révèlent jamais la configuration des équipes pendant une pause.

### Non-régression

Le buzz physique est **déjà inerte** pendant l'entracte : `handleButton` retourne si 
la phase n'est pas `STARTED`, et l'entracte n'est atteignable qu'en dehors de `STARTED` 
(phases autorisées : `STOPPED`, `PREPARE`, `READY`, `NEW_GAME`, `REVEALED`). Aucun 
code n'a besoin d'être ajouté pour ça — c'est une propriété héritée, maintenant 
verrouillée par test.

---

## 14. LED — Mode RAFALE (v8.0.0, #16)

### Grille LED multi-équipes RAFALE

Réutilise le patron de `sendLEDSetMemoryMultiTeam`, factorisation **byte-for-byte identical** :

| Buzzer | État | Effet LED |
|--------|------|-----------|
| Équipe **active** | Jouant (QUESTION) | `SOLID`, couleur d'équipe, `INTENSITY = 255` |
| Équipe **suivante** | Attente | `SOLID`, couleur d'équipe, `INTENSITY = 128` |
| Autres **participantes** | Attente | `DIM`, `INTENSITY = dimIntensityFor(rgb)` |
| **Non participantes** | Absent | Éteint, `{0,0,0}`, `INTENSITY = 0` |
| **Mode SOLO** | N/A | Tous éteints (pas de rotation) |

**Résolution couleur** : via `COLOR_NAME` (ex: `"rouge"`, `"bleu-profond"`), jamais RGB codée en dur.

**Rafraîchissement** :
- À chaque rotation d'équipe (VALIDATE/INVALIDATE → équipe suivante)
- À chaque changement de question (pioche, avance)
- À chaque modification sélection équipes (RAFALE_SET_TEAMS)

**Protocole ACK** : inchangé (v3.8.0) — le serveur attend l'ACK LED comme pour MEMORY.

**Factorisation** : helper partagé `sendLEDSetMultiTeam(currentTeam, nextTeam, participatingTeams, teamsData)` utilisé par MEMORY et RAFALE — **non-régression MEMORY obligatoire** (suite test identique avant/après refactor).

**Exemple** :
```
Équipes : Red, Blue, Green
Mode : CHACUN_SON_TOUR
State courant : Red joue

LED Buzzers Red : SOLID rouge 255
LED Buzzers Blue : SOLID bleu 128
LED Buzzers Green : DIM (atténué) 0-100
```

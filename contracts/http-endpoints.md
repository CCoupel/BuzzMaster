# Endpoints HTTP

> **Base URL** : `http://localhost` (port 80)
> **Dernière mise à jour** : 2026-06-07

---

## Questions

### GET /questions

Liste toutes les questions.

| Propriété | Valeur |
|-----------|--------|
| Auth      | Aucune |
| Response  | application/json |

#### Response 200

```json
{
  "/files/questions/1": {
    "ID": "1",
    "QUESTION": "Question text",
    "ANSWER": "Answer",
    "TYPE": "NORMAL",
    "POINTS": "10",
    "TIME": "30",
    "MEDIA": "/question/1/media_1234.jpg",
    "ORDER": 1,
    "STATUS": "AVAILABLE"
  },
  "FSINFO": {
    "USED": "1234567",
    "FREE": "98765432",
    "TOTAL": "100000000",
    "P_USED": "1.2"
  }
}
```

---

### POST /questions

Crée ou met à jour une question.

| Propriété | Valeur |
|-----------|--------|
| Auth      | Aucune |
| Content-Type | multipart/form-data |

#### Request (form fields)

| Champ | Type | Obligatoire | Description |
|-------|------|-------------|-------------|
| id | string | ❌ | ID (auto-généré si absent) |
| question | string | ✅ | Texte de la question |
| answer | string | ✅ | Réponse |
| type | string | ❌ | `NORMAL`, `QCM`, `MEMORY` (défaut: NORMAL) |
| points | int | ❌ | Points (défaut: 10) |
| time | int | ❌ | Durée en secondes (défaut: 30) |
| category | string | ❌ | Catégorie |
| points_target | string | ❌ | `PLAYER` ou `TEAM` |
| explanation | string | ❌ | **v6.4.x (#168)** — note d'explication/justification, visible de l'animateur seul. Texte libre, longueur non bornée. Écrit dans `EXPLANATION` ; **champ absent ou vide = note effacée** (voir note ci-dessous) |
| file | file | ❌ | Image question |
| file_answer | file | ❌ | Image réponse |

> ⚠️ **`explanation` doit être lu explicitement par `handleUploadQuestion`.** Ce handler
> **reconstruit la question de zéro** à chaque enregistrement et ne recopie depuis le fichier
> existant que `MEDIA`, `MEDIA_ANSWER` et `ORDER` : un champ non relu est perdu à la première
> édition. C'est aussi ce qui donne gratuitement la sémantique d'effacement — un `explanation`
> vide n'est simplement pas réécrit, donc la clé `EXPLANATION` disparaît du `question.json`.

> ℹ️ Cette table est **incomplète et antérieure** aux types ARDOISE et MEMOTION : les champs
> `ardoise_keyboard_type`, `memory_*`, `motion_*` et les extras QCM au-delà de ceux listés
> ci-dessous sont acceptés par le handler sans figurer ici. Constat signalé lors de #168, **non
> corrigé par ce lot** (hors périmètre) — à traiter dans une passe de remise à niveau des contrats.

**Champs QCM additionnels :**

| Champ | Type | Description |
|-------|------|-------------|
| qcm_red | string | Réponse rouge (A) |
| qcm_green | string | Réponse verte (B) |
| qcm_yellow | string | Réponse jaune (C) |
| qcm_blue | string | Réponse bleue (D) |
| qcm_correct | string | Couleur correcte |
| qcm_hints_enabled | bool | Activer indices |
| qcm_hint_threshold_1 | float | Seuil indice 1 (défaut: 0.25) |
| qcm_hint_threshold_2 | float | Seuil indice 2 (défaut: 0.125) |
| qcm_penalty_1 | float | Pénalité après 1 indice |
| qcm_penalty_2 | float | Pénalité après 2 indices |

**Champs MEMORY additionnels :**

| Champ | Type | Description |
|-------|------|-------------|
| memory_pairs | JSON | Tableau de paires |
| memory_config | JSON | Configuration Memory |

#### Response 200

```json
{
  "success": true,
  "id": "5"
}
```

---

## Configuration

### GET /config.json

Récupère la configuration **système** (serveur, WiFi, stockage, clés API IA).

**⚠️ BREAKING (v6.0.x, #150)** : les sections `game` et `neon_effect` ne sont
**plus** exposées ici — voir [GET /game-config.json](#get-game-configjson).

#### Response 200

```json
{
  "version": "2.45.0",
  "server": {
    "http_port": 80,
    "tcp_port": 1234
  }
}
```

---

### POST /config.json

Met à jour la configuration **système**. Fusion additive par section (une
section présente dans le body remplace intégralement cette section ; une
section absente n'est pas touchée).

| Propriété | Valeur |
|-----------|--------|
| Content-Type | application/json |

**⚠️ BREAKING (v6.0.x, #150)** : une requête contenant encore une section
`game` ou `neon_effect` est **rejetée en 400**, avec un message nommant le
nouvel endpoint (`POST /game-config.json`). Migration côté serveur
automatique et idempotente au démarrage — voir §Migration ci-dessous.

---

### GET /game-config.json

**Nouveau (v6.0.x, #150).** Récupère la configuration **de jeu** (délai par
défaut, effet néon) — séparée de la configuration système pour pouvoir être
sauvegardée/restaurée avec une partie (voir §Backup / Restore), indépendamment
des clés API et identifiants WiFi.

#### Response 200

```json
{
  "game": { "default_delay": 30 },
  "neon_effect": {
    "enabled": false,
    "mode": "bar",
    "arc_width": 60,
    "intensity_gap": 80,
    "rotation_speed": 4,
    "bar_offset": 20,
    "bar_thickness": 4,
    "arc_blur": 100,
    "glow_pulse_speed": 2,
    "glow_pulse_min": 30,
    "glow_pulse_max": 50
  }
}
```

---

### POST /game-config.json

**Nouveau (v6.0.x, #150).** Met à jour la configuration de jeu. Même
sémantique de fusion additive par section que `POST /config.json` (`game` et
`neon_effect` sont les deux seules sections). Les valeurs de `neon_effect`
sont validées/clampées aux mêmes bornes qu'avant la scission.

| Propriété | Valeur |
|-----------|--------|
| Content-Type | application/json |

Le payload WebSocket `neon_effect` (`CONFIG_UPDATE`, voir
`websocket-actions.md`) **n'est pas modifié** — seule sa source de lecture
change côté serveur.

#### Migration (v6.0.x, #150)

Au démarrage, si `config.json` porte encore une section `game` ou
`neon_effect` :
- si `data/config/game-config.json` n'existe pas encore → ses valeurs sont
  extraites vers ce nouveau fichier, puis retirées de `config.json` ;
- si `data/config/game-config.json` existe déjà → il fait autorité, les
  valeurs résiduelles de `config.json` sont **ignorées** (avec avertissement
  dans les logs) et retirées de `config.json`.

La migration est **idempotente** : un second démarrage sur des fichiers déjà
migrés ne réécrit rien.

---

## Backup / Restore

### GET /backup

Redirige vers /fs-backup (sauvegarde complète).

---

### GET /fs-backup

Télécharge une sauvegarde complète du système de fichiers (TAR).

| Propriété | Valeur |
|-----------|--------|
| Response  | application/x-tar |

---

### GET /game-backup

**Correction (divergence contrat/code constatée en v6.0.x, #150)** :
télécharge `dataDir/files` — c'est-à-dire les **questions et médias**
(backgrounds, catégories), **pas** la configuration. La description
précédente de cet endpoint était inversée par rapport au code
(`http.go:handleGameBackup`). Pour une sauvegarde incluant la configuration
de jeu (`game-config.json`), voir `GET /fs-backup` (complète, tout `dataDir`)
ou `GET /backup-select?history=true` (sélective).

| Propriété | Valeur |
|-----------|--------|
| Response  | application/x-tar |

---

### GET /backup-select

Sauvegarde sélective.

#### Query Parameters

| Param | Type | Défaut | Description |
|-------|------|--------|-------------|
| questions | bool | true | Inclure questions |
| teams | bool | true | Inclure équipes |
| bumpers | bool | true | Inclure joueurs |
| history | bool | true | Inclure historique, **et** `game_state.json` (métadonnées quiz + plafond de joueurs virtuels + config d'entracte enregistrée, v6.0.x #141, v6.5.2 #119) — pas de case dédiée pour ce petit fichier, rattaché à `history` par défaut de conception (identité/réglages d'une session, plus proche de l'historique qu'un préréglage visuel) |
| ambiance | bool | true | Inclure `game-config.json` (délai par défaut + effet néon, v6.0.x #150). **Flag dédié depuis #152** (2026-08-21) — n'était auparavant rattaché à aucun flag propre, **piggybacké sur `history`** ; `code-reviewer` a relevé ce rattachement comme un contresens sémantique (un réglage visuel/ambiance n'est pas de l'historique) lors de la revue de #150. Correspond à la case « Configuration Ambiance » de `BackupPage.jsx` |
| medias | bool | true | Inclure fonds & catégories (**renommé depuis `backgrounds` en v5.7.1**) |

#### Exemple

```
GET /backup-select?questions=true&history=true&ambiance=true&medias=true
```

---

### POST /restore

Restaure depuis un fichier TAR. Détection automatique du contenu (pas de
paramètre de sélection) : chaque type de donnée présent dans l'archive est
restauré indépendamment.

| Propriété | Valeur |
|-----------|--------|
| Content-Type | multipart/form-data |

#### Request

| Champ | Type | Description |
|-------|------|-------------|
| file | file | Fichier TAR |

**v6.0.x (#150, #141)** : une entrée `config/game-config.json` et/ou
`config/game_state.json` dans l'archive est détectée et restaurée
indépendamment de tout paramètre (contrairement à `/backup-select`, la
détection ici se fait sur le contenu réel de l'archive, pas sur un flag) —
l'état en mémoire (singleton config, ou `GameState` du moteur pour
`game_state.json`) est rafraîchi immédiatement, sans redémarrage.

---

### GET /reset-select

Reset sélectif.

#### Query Parameters

| Param | Type | Description |
|-------|------|-------------|
| all | bool | Reset tout |
| questions | bool | Supprimer questions |
| teams | bool | Vider équipes |
| bumpers | bool | Vider joueurs |
| history | bool | Vider historique **et vider `game_state.json`** (métadonnées quiz — fichier supprimé, même convention que `history.json`, v6.0.x #141) — même rattachement que `/backup-select` |
| ambiance | bool | **Réinitialiser `game-config.json` aux valeurs par défaut** (v6.0.x, #150). **Flag dédié depuis #152** — n'était auparavant réinitialisé qu'avec `history=true` ; voir `/backup-select` ci-dessus pour le détail du rattachement corrigé |
| medias | bool | Supprimer fonds & catégories (**renommé depuis `backgrounds` en v5.7.1**) |

---

## Catégories

### GET /api/categories

Liste les catégories disponibles (hardcodées + custom images).

| Propriété | Valeur |
|-----------|--------|
| Auth      | Aucune |
| Response  | application/json |

#### Response 200

```json
[
  { "key": "GEOGRAPHY",    "name": "Geographie",     "imageURL": "",                                  "color": "#3b82f6", "isCustom": false },
  { "key": "ENTERTAINMENT","name": "Divertissement",  "imageURL": "",                                  "color": "#f59e0b", "isCustom": false },
  { "key": "MA_CATEGORIE", "name": "Ma Catégorie",    "imageURL": "/files/categories/MA_CATEGORIE.png","color": "#6b7280", "isCustom": true }
]
```

**Champs** :
- `key` : identifiant unique (MAJUSCULES_UNDERSCORES)
- `name` : nom affiché (pour custom : nom original saisi lors de la création depuis v5.7.7, sinon stem du fichier)
- `imageURL` : URL image (vide pour hardcodées sans image, URL pour custom)
- `color` : couleur accent hex (couleur prédéfinie pour hardcodées, `#6b7280` fallback pour custom)
- `isCustom` : `true` si catégorie custom uploadée, `false` si hardcodée

> Les catégories custom sont des fichiers image dans `data/files/categories/`. 
> Le champ `color` est ajouté en v5.7.6.
> Depuis v5.7.7, le nom original est persiste en sidecar JSON (`<KEY>.json` aux côtés de `<KEY>.<ext>`). Les catégories créées avant v5.7.7 affichent leur nom technique (stem du fichier).

---

### POST /api/categories

Crée une nouvelle catégorie custom avec image. (v5.7.2 — #100)

**⚠️ BREAKING depuis v5.7.1** : remplace le body JSON par multipart/form-data (nom + image obligatoire).

| Propriété | Valeur |
|-----------|--------|
| Auth      | Aucune |
| Content-Type | multipart/form-data |

#### Request fields

| Champ | Type | Obligatoire | Description |
|-------|------|-------------|-------------|
| name  | string (form field) | ✅ | Nom de la catégorie (max 50 chars, alphanumérique/espace/tiret/underscore) |
| file  | file (image)        | ✅ | Image PNG, JPG, JPEG ou WebP |

> La clé est calculée automatiquement : `toUpperSnakeCase(name)` (espaces et tirets → underscore, MAJUSCULES).
> L'image est sauvegardée comme `<KEY>.<ext>` dans `data/files/categories/`.
> Depuis v5.7.7, le nom original est persiste en sidecar JSON : `<KEY>.json` contient `{ "name": "Ma Catégorie" }`.

#### Exemple fetch (ne pas définir Content-Type manuellement)

```js
const fd = new FormData()
fd.append('name', 'Ma Categorie')
fd.append('file', imageFile)
fetch('/api/categories', { method: 'POST', body: fd })
```

#### Response 200

```json
{
  "key": "MA_CATEGORIE",
  "name": "Ma Categorie",
  "imageURL": "/files/categories/MA_CATEGORIE.png",
  "isCustom": true
}
```

#### Errors

| Code | Raison |
|------|--------|
| 400  | `name` vide, trop long (>50), caractères invalides, `file` manquant, ou extension non autorisée |
| 405  | Méthode non autorisée |
| 409  | Clé déjà existante (catégorie hardcodée ou custom) |

---

## Historique

### GET /history

Récupère l'historique des événements.

#### Response 200

```json
[
  {
    "Timestamp": 1706380800000000,
    "QuestionID": "1",
    "QuestionText": "Question text",
    "QuestionCategory": "GEOGRAPHY",
    "EventType": "POINTS_AWARDED",
    "WinnerType": "PLAYER",
    "TeamName": "Les Rouges",
    "TeamColor": [239, 68, 68],
    "PlayerName": "Alice",
    "PlayerColor": "GREEN",
    "Points": 10,
    "CATEGORY_NAME": "Géographie",
    "CATEGORY_IMAGE_URL": "/files/categories/GEOGRAPHY.png",
    "CATEGORY_COLOR": "#3b82f6"
  }
]
```

**Champs catégorie (v5.7.9, `omitempty`)** :

| Champ | Type | Description |
|-------|------|-------------|
| `CATEGORY_NAME` | string | Nom affiché de la catégorie (ex: `"Sciences & Nature"`) |
| `CATEGORY_IMAGE_URL` | string | URL de l'image (ex: `"/files/categories/MON_JEU.png"`) — absent si aucune image |
| `CATEGORY_COLOR` | string | Couleur accent (ex: `"#22c55e"`) — absent pour catégories custom |

> Ces champs sont absents des anciens événements (avant v5.7.9). Le frontend doit gérer le fallback via `/api/categories`.

---

### GET /palmares

Retourne le palmarès de la partie en cours, pré-assemblé côté serveur. (v5.7.10)

**Réponse** : `PalmaresEntry[]` triée par `totalPoints` décroissant. `[]` si aucun événement.

#### Response 200

```json
[
  {
    "category": "GEOGRAPHY",
    "name": "Géographie",
    "imageURL": "/files/categories/GEOGRAPHY.png",
    "color": "#3b82f6",
    "totalPoints": 150,
    "teams": [
      { "name": "Les Rouges", "color": [239, 68, 68], "points": 100 },
      { "name": "Les Bleus", "color": [59, 130, 246], "points": 50 }
    ],
    "players": [
      { "name": "Alice", "team": "Les Rouges", "points": 60 },
      { "name": "Bob", "team": "Les Rouges", "points": 40 },
      { "name": "Charlie", "team": "Les Bleus", "points": 50 }
    ]
  }
]
```

**Champs `PalmaresEntry`** :

| Champ | Type | Description |
|-------|------|-------------|
| `category` | string | Clé catégorie (ex: `"SCIENCE"`, `"MON_JEU"`) |
| `name` | string | Nom affiché résolu (ex: `"Sciences & Nature"`) |
| `imageURL` | string | URL image (ex: `"/files/categories/MON_JEU.png"`) — `""` si aucune |
| `color` | string | Couleur accent hex (ex: `"#22c55e"`) — `""` pour catégories custom |
| `totalPoints` | int | Total des points attribués pour cette catégorie |
| `teams` | `TeamScore[]` | Scores par équipe, triés desc |
| `players` | `PlayerScore[]` | Scores par joueur, triés desc |

**`TeamScore`** : `{ name: string, color: [int, int, int], points: int }`  
**`PlayerScore`** : `{ name: string, team: string, points: int }`

> Cet endpoint agrège automatiquement `/history` sans double-comptage (clé composite `team|player`). Aucune race condition possible.

### [CHANGED] v9.0.0, Lot A+1 — agrégation par catégorie d'un événement RAFALE ventilé

Un événement `history.json` portant `CATEGORY_BREAKDOWN` (`contracts/models.md` — manche RAFALE
classique uniquement, retour QUALIF v9.0.0.4) est agrégé **différemment** d'un événement
ordinaire : au lieu de créditer la totalité de `POINTS` à la seule catégorie
`QUESTION_CATEGORY`, chaque paire `{catégorie: part}` de `CATEGORY_BREAKDOWN` crédite **sa propre**
entrée `PalmaresEntry` de sa **propre** part — équipes et joueurs inclus, même logique
d'attribution que pour un événement ordinaire, juste répétée une fois par catégorie nommée.

- La somme des parts créditées à travers toutes les catégories reste exactement égale à `POINTS`
  (garantie déjà portée par `CATEGORY_BREAKDOWN` lui-même) — **aucun double-comptage**, un événement
  ventilé ne fait qu'ajouter des ENTRÉES DE CATÉGORIE, jamais des points.
- Un événement **sans** `CATEGORY_BREAKDOWN` (tout événement non-RAFALE, et une manche RAFALE dont
  la répartition n'a pas pu être calculée — aucune catégorie configurée) garde le comportement
  **actuel, inchangé** : un seul crédit, sur `QUESTION_CATEGORY` (ou `"UNKNOWN"` si vide) — non-
  régression explicite.
- Implémentation : `handlePalmares` (`internal/server/http.go`) factorise l'accumulation
  bucket/équipe/joueur dans une closure `creditCategory`, appelée une fois par événement ordinaire
  ou une fois par entrée de `CATEGORY_BREAKDOWN` pour un événement ventilé — pas de logique
  dupliquée entre les deux cas.

---

## Système

### GET /version

Version du serveur.

#### Response 200

```json
{
  "version": "2.45.0"
}
```

---

### GET /listGame

État brut du jeu (JSON).

#### Response 200

Retourne l'état complet du jeu avec équipes et joueurs.

```json
{
  "GAME": { /* GameState */ },
  "teams": { /* Map des équipes */ },
  "bumpers": { /* Map des joueurs */ }
}
```

---

### GET /listFiles

Liste tous les fichiers média.

#### Response 200

```json
{
  "questions": ["1", "2", "3"],
  "backgrounds": ["bg1.jpg", "bg2.jpg"]
}
```

---

### GET /clearGame

Réinitialise la partie en cours.

---

### GET /clearBuzzers

Supprime tous les buzzers.

---

### GET /reboot

Redémarre le serveur.

---

### GET /shutdown

Arrête le serveur proprement.

---

### GET /reset

Reset usine complet.

---

## Demo

### POST /load-demo

Charge les données de démonstration.

| Propriété | Valeur |
|-----------|--------|
| Auth      | Aucune |

#### Response 200

```json
{
  "success": true,
  "message": "Demo loaded"
}
```

#### Données créées

| Type | Quantité |
|------|----------|
| Équipes | 6 |
| Joueurs | 24 |
| Questions | 10 |
| Historique | 10 événements |
| Fonds | 3 |

---

## Media

### GET /question/{id}/media_{suffix}.{ext}

Récupère l'image d'une question.

#### Path Parameters

| Param | Description |
|-------|-------------|
| id | ID de la question |
| suffix | Suffixe aléatoire |
| ext | Extension (jpg, png, gif) |

---

### GET /backgrounds/{filename}

Récupère une image de fond.

---

## Captive Portal

### GET /connecttest.txt

Endpoint pour détection de captive portal Windows.

#### Response 200

```
Microsoft Connect Test
```

---

### GET /ncsi.txt

Endpoint NCSI (Network Connectivity Status Indicator) Windows.

#### Response 200

```
Microsoft NCSI
```

---

## WebSocket

### GET /ws

WebSocket principal pour le jeu.

| Propriété | Valeur |
|-----------|--------|
| Protocol  | WebSocket |
| Usage     | Admin, TV, Buzzers web |

---

### GET /ws/logs

WebSocket dédiée aux logs temps réel.

| Propriété | Valeur |
|-----------|--------|
| Protocol  | WebSocket |
| Usage     | Page Logs uniquement |

Actions: `LOG_HISTORY`, `LOG_ENTRY`

---

## Génération IA (v6.0.0, #8)

### POST /api/generate-questions

Génère des questions via l'API Claude et les écrit directement en base (additif uniquement).

| Propriété | Valeur |
|-----------|--------|
| Auth      | Aucune |
| Content-Type | `application/json` |
| Durée     | Longue (1 à 3 min) — réponse synchrone |

> **Contrat détaillé** : `contracts/ai-generation.md` §3 (request, response, codes d'erreur
> stables, effet de bord `OnQuestionUpload` obligatoire).

Codes : `200` (avec `created` / `skipped_count`), `400` invalide, `405`, `409` pas de clé,
`502` erreur amont Anthropic, `504` timeout, `507` plus d'ID libre.

---

### POST /config.json — comportement modifié (correctif)

Le handler devient **additif** : il part de `config.Get()` et n'écrase que les sections
présentes dans le corps. Auparavant il désérialisait dans un struct vide puis réécrivait tout
le fichier, ce qui remettait à zéro chaque section absente du payload.

> **Contrat détaillé** : `contracts/ai-generation.md` §0 et §2.

### GET /config.json — nouvelle section `ai`

```json
"ai": {
  "anthropic_api_key": "",
  "api_key_configured": true,
  "model": "claude-opus-5",
  "timeout_seconds": 300,
  "max_questions": 200
}
```

`anthropic_api_key` est **toujours vide en réponse** — le secret n'est jamais renvoyé. Le
frontend s'appuie sur `api_key_configured` seul. Voir `contracts/ai-generation.md` §2.

---

### POST /api/generate-questions — devient asynchrone (v6.1.0, #137)

**[BREAKING]** L'endpoint ne renvoie plus le résultat de la génération.

| Propriété | Valeur |
|-----------|--------|
| Réponse   | `202 Accepted` — `{"status":"accepted","job_id":"…","batches_total":10}` |
| Progression | Action WebSocket `AI_GENERATION_PROGRESS` sur `/ws/admin` |
| Concurrence | `409 generation_in_progress` — un seul job à la fois |

Les codes `502` / `504` ne sont plus renvoyés par cet endpoint : ces erreurs surviennent
pendant le job et transitent par la progression.

> **Contrat détaillé** : `contracts/ai-multi-provider.md` §9 à §12.

---

## Mode ENTRACTE (v6.5.2, #119)

### ~~Section `entracte` de `POST /game-config.json`~~ — **supprimée (2026-08-20)**

La configuration du panneau d'entracte **ne vit plus dans `game-config.json`**. C'est une propriété
de la **partie**, pas un réglage du serveur : elle est désormais persistée dans `game_state.json`
et éditée depuis la page Quiz via l'action WebSocket `UPDATE_ENTRACTE_CONFIG`
(voir `contracts/game-state.md` §ENTRACTE_CONFIG et `contracts/websocket-actions.md`).

`POST /game-config.json` **n'accepte plus** de section `entracte` ; une clé résiduelle est ignorée
sans erreur. Aucune migration n'est fournie — cette section n'a existé qu'en QUALIF, jamais en
production.

### `GET` / `POST` / `DELETE /api/game/entracte-image`

Image de fond **unique et optionnelle** du panneau. Calqué à l'identique sur
`/api/config/default-image` (v3.2.2), le seul patron d'image unique du projet.

| Méthode | Effet |
|---|---|
| `GET` | Sert l'image téléversée ; `404` si aucune |
| `POST` | `multipart/form-data`, champ `file`, 10 Mo max, extensions image validées. **Remplace** l'image précédente quelle que soit son extension |
| `DELETE` | Supprime l'image — le panneau retombe sur son fond par défaut |

> **Renommé le 2026-08-20** — anciennement `/api/config/entracte-image`. Le préfixe `/api/config/`
> est devenu trompeur dès lors que l'image appartient à la partie et non aux réglages serveur.
> Renommage effectué pendant que c'était encore gratuit : l'endpoint n'a jamais atteint la
> production et son unique consommateur frontend était de toute façon réécrit par le déplacement de
> la section vers la page Quiz.

L'URL est **stable** : aucun chemin de fichier ne transite par le WebSocket, seul le booléen
`ENTRACTE_CONFIG.IMAGE_IS_CUSTOM` — **dérivé de la présence du fichier, jamais persisté** —
indique si une image existe. Le client ajoute un cache-buster
(`?t=…`), comme `ConfigPage` le fait déjà pour l'image de question par défaut.

**Stockage : `data/files/entracte/`**, un répertoire dédié — et **ajouté explicitement** à l'archive
du flag `medias` ainsi qu'à la remise à zéro des médias. Cette liste ne couvre aujourd'hui que
`backgrounds/` et `categories/` : l'image de question par défaut (écrite à la racine `data/files/`)
et `new-game-backgrounds/` en sont déjà absentes et ne survivent qu'à une sauvegarde intégrale
`/fs-backup`. Le répertoire dédié évite de reproduire ce trou.

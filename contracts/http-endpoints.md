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

---

## Sound (v11.0, #227/#229/#230)

> Contrat complet du vocabulaire de cues, du format canonique et du moteur :
> `contracts/sound.md` — en particulier §3 (format), §6.3 (`CuesDisabled`, amendement #230) et
> l'amendement §4 (accesseur `IsNeutral`, requis par `GET /api/sound/status` ci-dessous).
>
> **Terminologie normative de l'interface** (maquette `docs/mockups/sound-config-230.html`,
> révision 4) : un son est **« défaut »** ou **« personnalisé »** — jamais « livré ». Le mot
> « livré » ne désigne qu'une issue terminée dans les rapports techniques, jamais un état affiché
> à l'utilisateur.
>
> **Aucun endpoint pour l'activation générale ni pour l'interrupteur par cue.** Les deux
> s'écrivent par le patch partiel additif déjà existant `POST /config.json` avec un corps
> `{ "sound": { "enabled": ..., "cues_disabled": {...} } }` — exactement comme `{ "lighting": {...} }`
> le fait pour l'éclairage. Un lecteur qui chercherait un endpoint `PATCH`/`PUT` dédié à ces deux
> réglages n'en trouvera jamais : ce n'est pas un oubli.

### Sécurité — le nom de cue vient de l'URL, normatif pour tous les endpoints `{cue}` ci-dessous

Le segment `{cue}` transite par l'URL et sert à construire un chemin de fichier
(`data/files/sounds/<cue>.wav`). **Il doit être validé contre le catalogue fermé des 7 cues**
(`contracts/sound.md` §2.1) **avant tout usage** — jamais passé tel quel à une jointure de chemin.
Une valeur hors catalogue rend `404`, jamais une tentative de résolution disque. Même discipline
que la garde SSRF de #206 et que `filepath.Base` à la suppression d'un fond d'écran.

### `GET /api/sounds`

Les sept cues du catalogue, dans l'ordre normatif de `contracts/sound.md` §2.1, avec leur état
actuel — source de vérité : le manifeste réconcilié de #229
(`internal/audio/synth.ReconcileManifest`, comparaison d'octets contre une synthèse fraîche,
**aucun état supplémentaire à maintenir**) et `SoundConfig.CuesDisabled`.

| Propriété | Valeur |
|-----------|--------|
| Auth | Aucune |
| Méthode | `GET` uniquement |

#### Response 200

```json
{
  "cues": [
    {
      "cue": "depart",
      "enabled": true,
      "custom": false,
      "duration_seconds": 0.42,
      "path": "/files/sounds/depart.wav"
    }
  ]
}
```

| Champ | Type | Description |
|---|---|---|
| `cue` | string | Identifiant de la cue (`contracts/sound.md` §2.1) |
| `enabled` | bool | `!CuesDisabled[cue]` — interrupteur de ligne. **Indépendant** de l'interrupteur général (`sound.enabled`, voir `GET /api/sound/status` ci-dessous) |
| `custom` | bool | `true` si l'octet-à-octet diffère d'une synthèse fraîche (manifeste #229) — « personnalisé » à l'affichage, jamais « défaut » |
| `duration_seconds` | number | Calculée depuis l'en-tête WAV, sans décoder : `taille(data) / (fréquence × canaux × octets_par_échantillon)` |
| `path` | string | Servi tel quel par `handleFiles` (`/files/**`, sans restriction de sous-dossier — gratuit, aucune route dédiée) ; usage : lecture **locale** dans le navigateur (`<audio>`), jamais un test sur l'enceinte |

### `POST /api/sounds/{cue}`

Remplace le son d'une cue — `multipart/form-data`, champ `file`. Calqué sur le patron
`/api/game/entracte-image` (nom de fichier régénéré côté serveur, jamais celui envoyé par le
client), **avec une allowlist et des règles de validation volontairement plus strictes** :

| Règle | Valeur | Motif |
|---|---|---|
| Extension | **`.wav` uniquement** | Décision utilisateur actée au cadrage — aucune conversion, ni serveur ni navigateur |
| Contenu | **RIFF/WAVE valide**, PCM, format canonique strict (§3 : 16 bits, 44 100 Hz, stéréo) | Un seul contexte audio pour tout le processus (`contracts/sound.md` §1/§3) — accepter un fichier non conforme produirait soit un plantage silencieux à la lecture, soit une conversion à la volée jamais voulue. Le validateur s'appuie sur `extractCanonicalPCM` (`internal/audio/bank.go:60`, déjà écrit pour #229 et dont le commentaire anticipe explicitement cet upload) — **jamais un second validateur réécrit pour #230** |
| **Durée** | **refus au-delà de 5 s** | Le moteur lit **strictement séquentiellement** et `Play` bloque réellement (`contracts/sound.md` §4, amendement #228) — un son long retarde tous les suivants dans la file |
| **Durée (avertissement)** | accepté, mais signalé au-delà de ~2 s | Au-delà de deux secondes environ, un bruitage risque de déborder sur le moment de jeu suivant — accepté tel quel, l'utilisateur en est informé |
| Taille | plafond explicite **~2 Mo** | Un son canonique de 5 s pèse ~880 Ko — 2 Mo laisse une marge très généreuse sans autoriser un fichier abusif |

Durée calculée **sans décoder** : `taille(data) / (fréquence_échantillonnage × canaux × octets_par_échantillon)`.

#### Response 200

```json
{
  "status": "ok",
  "cue": "temps-ecoule",
  "custom": true,
  "duration_seconds": 1.8,
  "warning": null
}
```

`warning` porte le message d'avertissement de durée (> ~2 s) quand applicable, `null` sinon —
jamais un champ absent : le frontend n'a pas à distinguer « absent » de « vide ».

#### Errors

| Code | Description |
|------|-------------|
| 400 | Fichier absent, extension refusée, contenu non-WAV, format non conforme au canon (fréquence/canaux/bits), ou durée > 5 s — message lisible distinguant explicitement ces cas (maquette révision 4, section « Les refus, et ce qu'ils disent ») |
| 404 | `{cue}` hors du catalogue fermé |
| 405 | Méthode autre que `POST` |
| 413 | Fichier au-delà du plafond de taille |

Après acceptation : le fichier est écrit, `sounds.json` réconcilié (`synth.ReconcileManifest`),
la cue devient immédiatement active — **aucun redémarrage nécessaire** (`FileBank` ne met jamais
en cache, `contracts/sound.md` §"disque fait foi").

### `POST /api/sounds/{cue}/restore`

Régénère **une seule** cue à son son par défaut — jamais les six autres. Différence avec
`POST /api/sounds/restore-defaults` (ci-dessous) : portée unitaire, pas globale. **Idempotent par
construction**, même raison que le restore global : le générateur (`internal/audio/synth`) est
déterministe.

| Propriété | Valeur |
|-----------|--------|
| Auth | Aucune |
| Méthode | `POST` uniquement |

#### Response 200

```json
{"status": "ok", "cue": "temps-ecoule", "custom": false, "duration_seconds": 0.65}
```

#### Errors

| Code | Description |
|------|-------------|
| 404 | `{cue}` hors du catalogue fermé |
| 405 | Méthode autre que `POST` |
| 500 | Échec d'écriture disque |

N'apparaît dans l'interface **que sur une cue déjà personnalisée** (maquette révision 4, « "Restaurer"
n'apparaît que sur les sons personnalisés ») — un son déjà par défaut n'a rien à restaurer, mais
l'endpoint lui-même reste appelable sans condition (pas de `409` sur une cue déjà par défaut :
réécrire les mêmes octets par-dessus eux-mêmes est un no-op sans risque).

### `POST /api/sounds/{cue}/test`

Joue réellement la cue **sur l'enceinte reliée au serveur** — jamais dans le navigateur (voir
`path` de `GET /api/sounds` pour l'écoute locale). Appelle le moteur **directement**
(`a.sound().PlayCue(cue)`), **jamais via `notifySound`** : une cue individuellement désactivée
(`CuesDisabled`) reste testable, seul le déroulé réel de la partie est muet pour elle
(`contracts/sound.md` §6.3) — tester est un geste explicite de l'utilisateur.

**Trois résultats distincts, normatifs** (maquette révision 4, « Jamais de silence inexpliqué ») —
tous en `200`, le corps de la réponse porte la distinction, jamais le code HTTP : aucun des trois
n'est une erreur au sens HTTP, ce sont trois issues métier également valides.

| `result` | Condition | Libellé interface |
|---|---|---|
| `played` | `sound.enabled == true` **et** une sortie réelle est attachée (`!audio.IsNeutral(...)`, contract §4) **et** `PlayCue` a accepté la cue en file | « Son envoyé à l'enceinte » |
| `disabled` | `sound.enabled == false` — l'interrupteur général est éteint, le moteur n'est pas démarré | « Bruitages désactivés — rien n'a été joué » |
| `unavailable` | `sound.enabled == true` mais aucune sortie réelle n'a pu être ouverte au démarrage (`audio.IsNeutral(...)` vrai), **ou** la file était saturée (cas résiduel, même résultat affiché : aucun son n'est parti) | « Enceinte indisponible — rien n'a été joué » |

`disabled` est vérifié **avant** tout accès au moteur (lecture directe de `config.Get().Sound.Enabled`),
`unavailable` distingue un pilote neutre d'un pilote réel via l'accesseur du contract §4 —
`Stats.PlayErrors` ne permet PAS cette distinction : un `noopOutput` ne produit jamais d'erreur
(dégradation silencieuse jusqu'au bout, `contracts/sound.md` §5.5).

#### Response 200

```json
{"result": "played"}
```

#### Errors

| Code | Description |
|------|-------------|
| 404 | `{cue}` hors du catalogue fermé |
| 405 | Méthode autre que `POST` |

> **⚠️ Ce que `result` ne dit PAS — normatif, à ne jamais laisser l'interface déduire.** `played`
> signifie que la cue a été **confiée au moteur pour l'enceinte**, jamais qu'un son a été
> **entendu** : `Play` renvoie `nil` que l'enceinte soit présente ou non
> (`contracts/sound.md` §4) — **aucune** des trois valeurs de `result` ne constate une émission
> sonore réelle, `played` y compris. L'interface (#230, modale de test à verdict manuel — maquette
> révision 4) porte à côté de ces trois libellés un contrôle à trois positions (pas testé / ok /
> ko) que **seul l'utilisateur peut poser, après avoir écouté**. Câbler ce verdict
> automatiquement depuis `result` (par exemple `played` ⇒ verdict « ok ») **détruirait la raison
> d'être du contrôle manuel** — c'est le défaut le plus probable d'une implémentation qui ne lit
> que cette section sans lire aussi `GET /api/sound/status` ci-dessous, où la même limite est
> détaillée.

### `GET /api/sound/status`

État de la sortie audio, pour la pastille de l'interface — **deux états seulement, fusionnés
délibérément** (maquette révision 4, section 06 « deux états, et c'est tout »).

| Propriété | Valeur |
|-----------|--------|
| Auth | Aucune |
| Méthode | `GET` uniquement |

#### Response 200

```json
{"active": true}
```

`active = sound.enabled == true ET !audio.IsNeutral(sortie attachée)`. **Tout le reste fusionne en
`false`** — que le son soit désactivé ou que la sortie soit indisponible : la conséquence pour qui
regarde la page est identique (aucun son ne sortira), et distinguer la cause dans la pastille
demanderait une lecture qui ne changerait rien à l'action à mener. La cause reste connaissable —
par `POST /api/sounds/{cue}/test`, qui la nomme, et par les journaux du serveur.

> **Ce que cet état NE signifie PAS — à ne jamais laisser croire à l'interface.** Il est établi
> **une seule fois, à la construction du pilote** (`newOtoOutput`,
> `internal/audio/output_oto.go:71`) : il n'existe **aucune** boucle de reconnexion ni de
> surveillance après coup. Il ne détecte donc :
> - **ni** une enceinte qui s'éteint, s'endort ou s'éloigne en cours de partie — la pastille
>   resterait au vert ;
> - **ni** le périphérique réellement utilisé — si l'enceinte était absente au démarrage, le
>   serveur a très bien pu ouvrir la sortie par défaut du système (prise jack comprise) et se
>   déclarer actif ;
> - **ni** qu'un son a été **entendu** — `Play` renvoie `nil` que l'enceinte soit présente ou non
>   (`contracts/sound.md` §4).
>
> Le seul remède à un état inactif d'origine matérielle est un **redémarrage du serveur** — la
> tentative d'ouverture est unique. L'interface doit le dire explicitement, jamais laisser
> supposer une action corrective côté configuration seule.

### `POST /api/sounds/restore-defaults` — déjà livré en #229

Régénère les **sept** sons du catalogue dans `data/files/sounds/`, **en écrasant
inconditionnellement** tout fichier déjà présent — défaut ou personnalisé par l'utilisateur.
Calqué sur `POST /api/firmware/buzzclick/restore-embedded` : action explicite, toujours
destructrice, jamais le comportement du démarrage ordinaire (qui ne touche **jamais** un fichier
existant — `createDefaultSounds`, `cmd/server/sound.go`).

**Idempotent par construction** : le générateur (`internal/audio/synth`) est déterministe (aucun
aléa, aucune horodatation), donc deux appels successifs produisent des octets strictement
identiques.

| Propriété | Valeur |
|-----------|--------|
| Auth | Aucune |
| Méthode | `POST` uniquement — `405` sinon |

#### Response 200

```json
{
  "status": "ok",
  "written": ["depart", "temps-ecoule", "gagne", "perdu", "reveal", "entracte-debut", "entracte-fin"]
}
```

#### Errors

| Code | Description |
|------|-------------|
| 405 | Méthode autre que `POST` |
| 500 | Échec d'écriture disque (répertoire non accessible en écriture, etc.) |

Réconcilie aussi `data/files/sounds/sounds.json` (manifeste, `internal/audio/synth.ReconcileManifest`)
après l'écriture — voir `contracts/sound.md` §7. Dans l'interface (maquette révision 4), une
confirmation nomme explicitement combien de sons personnalisés seront écrasés avant l'action —
comportement d'interface, hors périmètre de ce contrat HTTP.

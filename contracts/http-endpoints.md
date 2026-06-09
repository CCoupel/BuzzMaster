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
| file | file | ❌ | Image question |
| file_answer | file | ❌ | Image réponse |

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

Récupère la configuration.

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

Met à jour la configuration.

| Propriété | Valeur |
|-----------|--------|
| Content-Type | application/json |

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

Télécharge uniquement les données de jeu (config, pas les questions).

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
| history | bool | true | Inclure historique |
| medias | bool | true | Inclure fonds & catégories (**renommé depuis `backgrounds` en v5.7.1**) |

#### Exemple

```
GET /backup-select?questions=true&history=true&medias=true
```

---

### POST /restore

Restaure depuis un fichier TAR.

| Propriété | Valeur |
|-----------|--------|
| Content-Type | multipart/form-data |

#### Request

| Champ | Type | Description |
|-------|------|-------------|
| file | file | Fichier TAR |

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
| history | bool | Vider historique |
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

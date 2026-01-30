# Endpoints HTTP

> **Base URL** : `http://localhost` (port 80)
> **Dernière mise à jour** : 2026-01-27

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
| backgrounds | bool | true | Inclure fonds |

#### Exemple

```
GET /backup-select?questions=true&history=true
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
| backgrounds | bool | Supprimer fonds |

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
    "Points": 10
  }
]
```

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

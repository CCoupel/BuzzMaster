# Journalisation persistante et événements de diagnostic

> **Créé** : 2026-10-10 (#246, v11.2)
> **Pourquoi** : pendant la partie de la semaine bleue 2026, aucun log n'a pu être récupéré. Le
> serveur ne gardait que **1000 entrées en mémoire** (`server.InitLogger(1000)`) et journalisait
> chaque frappe ARDOISE en `INFO` : à 10 équipes, environ 20 secondes d'historique, rien sur disque.
> Ce contrat rend les avertissements **persistants** et fixe la liste des événements nécessaires
> pour prouver la cause d'un incident à la prochaine partie.

---

## 1. Configuration — `config.json`, section `logging` (`[NEW]`, rétrocompatible)

```json
{
  "logging": {
    "file_enabled": true,
    "file_path": "./data/logs/buzzcontrol.log",
    "file_min_level": "WARN",
    "file_max_size_mb": 10,
    "file_max_files": 5
  }
}
```

| Clé | Type | Défaut si absente | Règle |
|-----|------|-------------------|-------|
| `file_enabled` | bool | `true` | `false` = comportement antérieur (mémoire + sortie standard) |
| `file_path` | string | `./data/logs/buzzcontrol.log` | Répertoire créé au démarrage s'il manque. Chemin relatif = relatif au répertoire de travail du serveur, comme `storage.data_dir` |
| `file_min_level` | string | `WARN` | `DEBUG`, `INFO`, `WARN` ou `ERROR`. Seules les entrées de niveau ≥ sont écrites sur disque. Le tampon mémoire et `/ws/logs` restent inchangés |
| `file_max_size_mb` | int | `10` | Rotation à cette taille : `buzzcontrol.log` → `buzzcontrol.log.1` → … |
| `file_max_files` | int | `5` | Nombre de fichiers conservés (courant compris) ; les plus anciens sont supprimés |

- Une section `logging` absente (fichier de configuration antérieur) = valeurs par défaut ci-dessus.
- Une erreur d'écriture disque n'arrête jamais le serveur : un `ERROR` en sortie standard au plus une fois par minute, puis on continue en mémoire.
- Format d'une ligne : `2026-10-10T14:03:21.123+02:00 [WARN] [App] ARDOISE_CLEARED team="Les Verts" bumper=… client=…` (horodatage RFC 3339 en millisecondes, niveau, composant, message ; valeurs texte entre guillemets, échappées).

## 2. Changement de niveau (`[CHANGED]`)

- La ligne par frappe `ARDOISE_INPUT: team=… text=…` (`cmd/server/main.go`, `handleArdoiseInput`) passe de `INFO` à **`DEBUG`**. Elle noyait le tampon mémoire ; l'information utile est portée par les événements §3.

## 3. Catalogue des événements (`[NEW]`)

Tous écrits sur disque avec la configuration par défaut (`WARN`/`ERROR`), sauf mention contraire.

### 3.1 ARDOISE

| Événement | Niveau | Quand | Champs |
|-----------|--------|-------|--------|
| `ARDOISE_CLEARED` | WARN | Le texte d'une équipe passe de non vide à `""` par `ARDOISE_INPUT` | `team`, `bumper`, `client`, `conn_age_ms`, `prev_len`, `seq`, `reason`, `question`, `phase` |
| `ARDOISE_SEQ_GAP` | WARN | `SEQ` non consécutif sur une même connexion | `bumper`, `client`, `expected`, `got` |
| `ARDOISE_SEQ_STALE` | WARN | `SEQ` ≤ dernier appliqué sur la même connexion → message ignoré. Le dernier `SEQ` d'une connexion est effacé à chaque (re)liaison (`PLAYER_CONNECT` accepté) et à la fermeture de la connexion | `bumper`, `client`, `last`, `got` |
| `ARDOISE_STATE_DIVERGED` | **ERROR** | Invariant : `ARDOISE_ANSWERS` diffère de la copie de contrôle tenue **uniquement** par `ARDOISE_INPUT`/`ARDOISE_FINAL`/`Ready()`/NEW_GAME. Vérifié à chaque diffusion d'état complet et à chaque changement de phase | `team`, `expected`, `actual`, `site` (appelant), `phase`, `question` |
| `ARDOISE_FINAL_STALE` | WARN | `ARDOISE_FINAL` d'une autre question, ou hors `STOPPED`/`PAUSED` | `bumper`, `question_id`, `current_question`, `phase` |
| `ARDOISE_INPUT_TOO_LONG` | WARN | `ARDOISE_INPUT` de plus de 2 000 caractères → ignoré en entier, sans troncature | `bumper`, `client`, `len` (caractères) |
| `ARDOISE_FINAL_TOO_LONG` | WARN | `ARDOISE_FINAL` de plus de 2 000 caractères → refusé, sans accusé | `bumper`, `client`, `len` (caractères), `question_id` |
| `ARDOISE_FINAL_HASH_MISMATCH` | WARN | Hash reçu ≠ hash recalculé | `bumper`, `got`, `computed`, `len` |
| `ARDOISE_FINAL_RECEIVED` | INFO | Final accepté (non écrit sur disque par défaut) | `team`, `bumper`, `hash`, `len` |
| `ARDOISE_REVEAL_REFUSED` | WARN | REVEAL refusé par la garde | `client`, `type` (admin/anim), `pending` (équipes + hors ligne) |
| `ARDOISE_REVEAL_FORCED` | WARN | REVEAL avec `FORCE: true` alors que des équipes manquent | `client`, `type`, `pending` |

### 3.2 Identité des joueurs virtuels et connexions

| Événement | Niveau | Quand | Champs |
|-----------|--------|-------|--------|
| `PLAYER_ACTION_UNBOUND` | WARN | Action joueur d'une connexion `vplayer` non liée → refusée | `action`, `client`, `payload_id` |
| `PLAYER_FOREIGN_ID` | WARN | Action joueur dont `ID` ≠ bumper lié → refusée | `action`, `client`, `bound_id`, `payload_id` |
| `VPLAYER_NAME_MISMATCH` | WARN | `PLAYER_CONNECT` cas 1 avec un `NAME` différent du nom du bumper (le nom n'est plus modifié) | `id`, `server_name`, `client_name`, `client` |
| `VPLAYER_DUPLICATE_SESSION` | WARN | Nouvelle connexion liée à un bumper déjà lié → l'ancienne est évincée | `id`, `old_client`, `new_client` |
| `VPLAYER_DUPLICATE_NAME` | WARN | Liste admin (`UPDATE`/`FULL`) : renommage d'un VJoueur existant vers un nom déjà porté → ignoré ; nouvelle entrée de VJoueur au nom déjà pris → retirée | `id`, `name`, `holder_id` |
| `VPLAYER_CONN` | INFO | Connexion/déconnexion d'un VJoueur pendant une question `STARTED`/`PAUSED` (non écrit sur disque par défaut ; les coupures sont tracées en WARN par `CLIENT_TRACE WS_GAP` à la reconnexion) | `id`, `event`, `client`, `question`, `phase` |

### 3.2 bis Équipes (v11.2, #249)

| Événement | Niveau | Quand | Champs |
|-----------|--------|-------|--------|
| `TEAM_NAME_NORMALIZED` | WARN | `NAME` d'une équipe différent de sa clé, corrigé (enregistrement ou chargement de `teams.json`) | `key`, `old_name`, `source` (`load`/`update`) |
| `TEAMS_CHANGE_REFUSED` | WARN | `UPDATE`/`FULL` qui renomme ou supprime une équipe pendant une question → refusé | `client`, `teams` (clés retirées), `phase`, `question` |
| `TEAM_RENAMED` | INFO | Renommage appliqué hors question, états reportés (non écrit sur disque par défaut) | `old`, `new`, `migrated` (liste des états reportés) |

### 3.2 ter Nouvelle partie (v11.2, #243)

| Événement | Niveau | Quand | Champs |
|-----------|--------|-------|--------|
| `NEW_GAME_PURGE` | WARN | Chaque `NEW_GAME` appliqué, quelles que soient les options | `client`, `options` (`scores`/`players`/`teams`/`questions` vrais), `players_removed`, `teams_removed`, `scores_reset`, `questions_reset`, `phase_before`, `question_before` |
| `NEW_GAME_REFUSED` | WARN | `NEW_GAME` refusé (joueurs sans scores, ou entracte active) | `client`, `options`, `reason` (`PLAYERS_REQUIRE_SCORES` / `ENTRACTE_ACTIVE`) |

### 3.2 quater Attribution groupée (v11.2, #244/#251)

| Événement | Niveau | Quand | Champs |
|-----------|--------|-------|--------|
| `TEAM_POINTS_BATCH` | INFO | Lot appliqué (non écrit sur disque par défaut) | `client`, `question`, `applied` (équipe:montant), `skipped` (équipe:motif) |
| `TEAM_POINTS_BATCH_REFUSED` | WARN | Lot refusé | `client`, `question`, `reason`, `teams` |

### 3.3 Diffusion

| Événement | Niveau | Quand | Champs |
|-----------|--------|-------|--------|
| `CLIENT_EVICTED_SLOW` | WARN | Client fermé parce que sa file d'envoi (256) est pleine (`internal/server/websocket.go`, envoi direct et boucle `Run`) — **aujourd'hui silencieux** | `client`, `type`, `queue` |
| `CLIENT_WRITE_TIMEOUT` | WARN | Écriture vers un client au-delà de son délai (3 s) | `client`, `type` |
| `INCOMING_DROPPED` | WARN | File d'entrée pleine (100) → message ignoré (remplace le WARN actuel, sans contexte) | `client`, `type`, `action` |

### 3.4 Traces clients (`CLIENT_TRACE`, `contracts/websocket-actions.md`)

| Événement | Niveau | Champs |
|-----------|--------|--------|
| `CLIENT_TRACE` | WARN | `client`, `type`, `player_id` (si lié), `kind`, `at` (horodatage client), puis les champs du `KIND` (`team`, `prev_len`, `cause_action`, `detail.*`). **Une trace = une seule ligne** : textes (`kind`, `cause_action`, `player_id`, clés et valeurs de `detail`) nus s'ils ne contiennent que `[A-Za-z0-9_.:-]`, sinon entre guillemets échappés ; valeur de `detail` non scalaire → JSON entre guillemets ; retours à la ligne neutralisés à l'écriture disque |
| `CLIENT_TRACE_THROTTLED` | WARN | `client=… type=…` : au plus une fois par minute et par connexion qui dépasse **10 traces/s** ; `scope=global limit=50/s` : au plus une fois par minute quand l'ensemble des connexions dépasse **50 traces/s** (traces au-delà ignorées) |

## 4. Lecture en QUALIF — quelle trace prouve quoi

| Trace observée | Conclusion |
|----------------|-----------|
| `ARDOISE_STATE_DIVERGED` | Réécriture serveur des réponses sans saisie : le champ `site` désigne le code fautif |
| `PLAYER_FOREIGN_ID` / `PLAYER_ACTION_UNBOUND` | Une tablette a tenté d'écrire pour un autre joueur ou sans liaison (désormais bloqué) |
| `VPLAYER_NAME_MISMATCH`, `VPLAYER_DUPLICATE_SESSION`, `CLIENT_TRACE STALE_TAB` | Onglet périmé ou session en double sur une tablette (mécanisme du changement de nom spontané) |
| `ARDOISE_CLEARED reason=clear` / `backspace_empty` | Effacement fait sur la tablette |
| `CLIENT_TRACE ARDOISE_CLEARED_SEEN` sans `ARDOISE_CLEARED` serveur | Problème côté écran (admin/anim) |
| `CLIENT_TRACE WS_GAP` avec `DROPPED_SENDS` > 0 | Frappes perdues pendant une coupure de la tablette |
| `CLIENT_TRACE ARDOISE_FINAL_GIVEUP` | Une tablette a renoncé à transmettre son texte final (30 envois sans accusé) : l'équipe reste « en attente » ; à rapprocher des `ARDOISE_FINAL_*` serveur (refus, hash, trop long) ou d'une coupure |
| `CLIENT_EVICTED_SLOW` | Un écran (admin, TV, anim) trop lent a été coupé par le serveur |

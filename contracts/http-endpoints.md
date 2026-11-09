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
| round_id | string | ❌ | *(v11.2, #245)* Manche d'une **nouvelle** question (bouton « + Question » d'une manche). Ignoré si l'ID est inconnu. **À la modification d'une question existante, `ROUND_ID` est conservé tel quel** (même piège de reconstruction que `ORDER`, `MEDIA`, `SOUND`) : seul `ROUNDS_UPDATE` change la manche d'une question existante |
| explanation | string | ❌ | **v6.4.x (#168)** — note d'explication/justification, visible de l'animateur seul. Texte libre, longueur non bornée. Écrit dans `EXPLANATION` ; **champ absent ou vide = note effacée** (voir note ci-dessous) |
| file | file | ❌ | Image question |
| file_answer | file | ❌ | Image réponse |
| sound | file | ❌ | **v11.1 (#219)** — média sonore de question, `.wav` canonique (44 100 Hz/16 bits, **mono ou stéréo** — un mono est suréchantillonné et stocké stéréo, arbitrage QUALIF 2026-09-22, `contracts/sound.md` §10.4), ≤ 30 s, ≤ 6 Mio. Écrit dans `SOUND`. Refusé en `400` (cause nommée : pas WAV / fréquence-bits / nombre de voies invalide / > 30 s) ou `413` (> 6 Mio) — voir §Réponses d'erreur ci-dessous |
| sound_cleared | bool (`"true"`/absent) | ❌ | **v11.1 (#219)** — `"true"` supprime le son existant (fichier disque compris, `os.Remove`) et empêche sa recopie. Sans effet si `sound` est également fourni dans la même requête (le nouveau fichier prévaut) |
| sound_timer_delayed | bool (`"true"`/absent) | ❌ | **v11.1 (#219)** — `"true"` = chronomètre de réponse différé jusqu'à la fin du son (`contracts/sound.md` §10.7). Écrit dans `SOUND_TIMER_DELAYED` **uniquement si un son est effectivement attaché** (préservé ou fraîchement téléversé) — jamais laissé actif sans son |
| sound_loop | bool (`"true"`/absent) | ❌ | **v11.4.0 (#288)** — `"true"` = média rejoué en boucle jusqu'à l'arrêt du chronomètre (`contracts/sound.md` §13). Écrit dans `SOUND_LOOP` **uniquement si un son est effectivement attaché** (même règle que `sound_timer_delayed`). Combinable avec `sound_timer_delayed=true` : le chronomètre démarre à la fin du **premier passage**, le média continue en boucle (`sound.md` §13.2) |

> ⚠️ **`explanation` doit être lu explicitement par `handleUploadQuestion`.** Ce handler
> **reconstruit la question de zéro** à chaque enregistrement et ne recopie depuis le fichier
> existant que `MEDIA`, `MEDIA_ANSWER` et `ORDER` : un champ non relu est perdu à la première
> édition. C'est aussi ce qui donne gratuitement la sémantique d'effacement — un `explanation`
> vide n'est simplement pas réécrit, donc la clé `EXPLANATION` disparaît du `question.json`.

> ⚠️ **`sound` suit la même règle de recopie que `explanation`, avec un piège aggravé** (contract
> `sound.md` §10, `contracts/models.md` §SOUND) : sans recopie explicite de `SOUND` depuis la
> question existante, le son serait **détruit silencieusement à chaque ré-édition sans nouveau
> fichier**. `sound_cleared=true` est l'opt-out explicite, et va plus loin que `MEDIA`/
> `MEDIA_ANSWER` (qui n'ont aujourd'hui aucun moyen réel de suppression) : le fichier disque est
> effectivement supprimé, pas seulement dé-référencé.
>
> **Refus vs avertissement — deux mécanismes distincts.** Un fichier non conforme (mauvais format,
> > 30 s, > 6 Mio) est **refusé** (la requête entière échoue, `400`/`413`, rien n'est enregistré).
> Un fichier **conforme** mais plus long que `time` **en mode simultané** (`sound_timer_delayed`
> absent/`false`) déclenche un **avertissement non bloquant** — voir §Réponse ci-dessous — jamais en
> mode différé, où la situation ne peut pas se produire (le chronomètre attend la fin du son quelle
> que soit `time`).

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
| qcm_wrong_penalty | int | *(v11.2, #254)* Pénalité de mauvaise réponse, entier de −100 à 0 (0 = aucune). Enregistrée en `QCM_WRONG_PENALTY` seulement pour `type = QCM` et si valide (0 compris, écrit tel quel) ; absente ou invalide → champ **non écrit** (valeur effective 0), sans erreur (même tolérance que `qcm_penalty_1/2`) |

**Champs MEMORY additionnels :**

| Champ | Type | Description |
|-------|------|-------------|
| memory_pairs | JSON | Tableau de paires |
| memory_config | JSON | Configuration Memory |

#### Response 200

> ⚠️ **Le corps ci-dessous (`success`/`id`) est un exemple historique déjà signalé stale en #168
> (non corrigé, hors périmètre de ce lot) — la réponse **réellement** renvoyée par
> `handleUploadQuestion` (`internal/server/http.go`) est celle documentée juste après, **normative
> depuis v11.1 (#219)** : c'est celle-ci que le frontend doit lire.

```json
{
  "success": true,
  "id": "5"
}
```

**Réponse réelle, normative depuis v11.1 (#219)** :

```json
{
  "status": "ok",
  "warning": null
}
```

- `status` : toujours `"ok"` sur un `200` (aucune autre valeur possible — un échec ne produit jamais
  ce corps, voir §Réponses d'erreur ci-dessous).
- `warning` : `null` quand aucun avertissement (**jamais une clé absente**, même convention que
  `POST /api/sounds/{cue}` ci-dessous), sinon une **chaîne** nommant la cause — aujourd'hui la seule
  source possible est le son de question en mode simultané plus long que `time` (voir la note
  ci-dessus). Non bloquant : la question est enregistrée normalement.

#### Réponses d'erreur (v11.1, #219 — champ `sound`)

Un `sound` rejeté (format non conforme, > 30 s, > 6 Mio, échec d'écriture disque) répond en **texte
brut** (`http.Error` standard Go — `Content-Type: text/plain`, le message tel quel comme corps, sans
enveloppe JSON) :

| Cause | Status |
|---|---|
| Extension autre que `.wav` | `400` |
| Pas un WAV canonique / > 30 s (cause nommée dans le corps) | `400` |
| Lecture du fichier échouée | `400` |
| > 6 Mio | `413` |
| Échec d'écriture disque | `500` |

Une erreur sur `sound` **rejette toute la requête** — rien n'est enregistré, y compris les autres
champs de la question (contrairement à `MEDIA`/`MEDIA_ANSWER`, qui échouent silencieusement sans
jamais renvoyer d'erreur).

#### Atomicité du refus — aucune trace sur disque (v11.2.3, #278)

Tout refus survenant **après l'allocation de l'identifiant** (`400` de validation des cartes MEMOTION
— `CARD_TYPE_CONTENT_MISMATCH`, `POINTS_RULE_*` —, `400`/`413` sur `sound`, `500` d'écriture) ne
laisse **aucune trace sur disque** :

- **question nouvelle** (identifiant absent ou libre) : le dossier `data/files/questions/<id>/` créé
  par l'allocation est **supprimé** et l'identifiant **redevient disponible** pour l'allocation
  automatique suivante (`resolveQuestionDir` alloue par `mkdir`, l'identifiant libéré est donc le
  premier candidat réutilisé) ;
- **question existante** : le dossier et le `question.json` antérieurs **ne sont jamais supprimés**
  (un refus survient avant toute réécriture de `question.json`).

Avant v11.2.3, un refus de validation des cartes laissait un dossier vide (constat QA #190).

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

> **v11.4.0 (#285, LOT-4) — section réécrite.** Gestion complète des catégories : renommage, couleur (pilote les badges
> **et** l'éclairage d'ambiance, `contracts/lighting.md` §8), icône (personnalisées seulement), suppression avec
> réaffectation ou suppression des questions et **fusion de l'historique dans « Sans catégorie »**. Décisions utilisateur
> (GATE 2, commentaire de l'issue #285 ; `docs/mockup/DECISIONS.md` « admin_category_manager ») ; maquette normative
> `docs/mockup/v11.4.0/ui/admin_category_manager__rename_color_delete.html` — **sauf** son état « Icône envoyée sur une
> catégorie intégrée » et le bouton « ↺ emoji », **écartés** par la décision (icône réservée aux personnalisées).
> Historique : `GET /api/categories` v5.7.6 (`color`), v5.7.7 (sidecar `name`), `POST` v5.7.2 (#100).

### Modèle de stockage — `data/files/categories/`

| Élément | Catégorie **intégrée** (8 clés de `hardcodedCategories`) | Catégorie **personnalisée** |
|---|---|---|
| Identité | clé fixe (`GEOGRAPHY`…), **jamais supprimable** | clé = `toUpperSnakeCase(name)` à la création, **immuable** ensuite |
| Icône | emoji (table frontend `CATEGORIES`), **jamais d'image** | image `<KEY>.<ext>` (`png`/`jpg`/`jpeg`/`webp`), **obligatoire** (c'est sa présence qui fait exister la catégorie — inchangé) |
| Sidecar `<KEY>.json` | **surcharge** optionnelle `{ "name"?: string, "color"?: string }` — absent = valeurs de la table (`name` ASCII historique, couleur intégrée) | `{ "name": string, "color"?: string }` — `name` écrit à la création (v5.7.7), `color` optionnelle |
| Couleur | table (`#3b82f6`…) sauf surcharge `color` du sidecar ; `""` dans le sidecar = **aucune** (explicite) | `color` du sidecar ; absente ou `""` = **aucune** |

- **Valeur zéro / rétrocompatibilité** : aucun fichier existant n'est modifié ; une catégorie personnalisée sans
  `color` se comporte comme aujourd'hui (badge gris `#6b7280` côté frontend, salle **blanche**) ; une intégrée sans
  sidecar garde nom et couleur d'origine. `handleGetCategories` continue d'**ignorer** une image dont la clé est
  intégrée (conflit journalisé) : le sidecar d'une intégrée **n'a jamais d'image à côté**.
- **Résolution** (`ResolveCategoryMeta(key)`, source unique pour le palmarès, l'historique **et** l'ambiance) :
  intégrée ⇒ table **puis** surcharge sidecar (`name`, `color`) ; personnalisée ⇒ image + sidecar. **Aucune seconde
  table** côté serveur ; le frontend devient **API-first** (voir « Frontend » ci-dessous).
- **« Sans catégorie »** : `CATEGORY == ""` sur une question/carte/réponse du réservoir ; dans l'historique
  `QUESTION_CATEGORY == ""` ⇒ bucket palmarès `UNKNOWN` (comportement existant de `handlePalmares`), affiché
  **« Sans catégorie »** (le libellé « Inconnue » de `CategoryPalmaresPage.jsx` est renommé).
- **Partie en cours ⇒ tout est bloqué** (décision utilisateur 2026-11-05) : `POST /api/categories`, `PUT`, `POST …/icon`
  et `DELETE` répondent **`409 {"error":"game in progress"}`** quand la phase du moteur ∉ {`STOPPED`, `NEW_GAME`,
  `REVEALED`} (question vivante `PREPARE`…`PAUSED`, ou `ENROLL`). `GET` reste libre. Garde évaluée **en premier**, avant
  toute validation de corps ou de fichier (fonction commune `categoriesLocked()`), testée sur les quatre routes.
- **Sécurité des chemins** : toute clé reçue en URL (`/api/categories/{key}…`) est validée par
  `^[\p{L}\p{N}_]{1,50}$` (Unicode : `CINÉMA` passe — révision 2, aligné sur le code) **avant** toute jointure de chemin
  (`400` sinon) ; les **noms réservés Windows** (`CON`, `PRN`, `AUX`, `NUL`, `COM1`–`COM9`, `LPT1`–`LPT9`, comparaison
  insensible à la casse, sur la clé entière) sont refusés `400` à la création (`handlePostCategory`) — serveur Windows
  supporté, `CON.png` serait un périphérique ; un nom disque est **toujours** dérivé de la
  clé validée + extension de l'allowlist, **jamais** de `header.Filename` (même discipline que `SafeQuestionPath`).

### GET /api/categories

| Propriété | Valeur |
|-----------|--------|
| Auth      | Aucune |
| Response  | application/json |

#### Response 200

```json
[
  { "key": "GEOGRAPHY",    "name": "Géographie",  "imageURL": "",                                   "color": "#3b82f6", "isCustom": false, "questionCount": 24, "rafaleCount": 110 },
  { "key": "CINEMA",       "name": "Cinéma",      "imageURL": "/files/categories/CINEMA.png",       "color": "#0ea5e9", "isCustom": true,  "questionCount": 12, "rafaleCount": 30 },
  { "key": "MA_CATEGORIE", "name": "Ma Catégorie","imageURL": "/files/categories/MA_CATEGORIE.png", "color": "",        "isCustom": true,  "questionCount": 0,  "rafaleCount": 0 }
]
```

| Champ | Type | Description |
|---|---|---|
| `key` | string | identifiant unique (`^[\p{L}\p{N}_]{1,50}$`, majuscules par `toUpperSnakeCase`), immuable |
| `name` | string | nom affiché — **résolu** (surcharge sidecar pour une intégrée, v11.4.0) |
| `imageURL` | string | URL de l'icône (`""` pour une intégrée — toujours) |
| `color` | string | `#rrggbb` **stockée/résolue**, ou `""` = aucune (v11.4.0 : les personnalisées peuvent en avoir une ; le repli `#6b7280` est **retiré** de la réponse — il reste un repli d'affichage frontend) |
| `isCustom` | bool | `false` pour les 8 intégrées |
| `questionCount` | int | **v11.4.0** — questions `question.json` dont `CATEGORY == key` **+** cartes MEMOTION dont `CATEGORY == key` |
| `rafaleCount` | int | **v11.4.0** — réponses du réservoir RAFALE dont `CATEGORY == key` (utilisées ou non) |

> Ordre : 8 intégrées dans l'ordre de la table, puis personnalisées par ordre de lecture du dossier (inchangé).
> `questionCount`/`rafaleCount` sont calculés à l'appel (lecture `ReadQuestionMaps` + `SnapshotRafaleReservoir`), jamais mis en cache.

---

### POST /api/categories

Création d'une catégorie personnalisée (multipart, inchangé) — **v11.4.0** : champ `color` optionnel.

| Champ | Type | Obligatoire | Description |
|-------|------|-------------|-------------|
| name  | string | ✅ | nom (max 50, lettres/chiffres/espace/tiret/underscore — **inchangé**) ; clé dérivée refusée `400 {"error":"reserved name"}` si nom réservé Windows (révision 2) |
| file  | file | ✅ | icône PNG/JPG/JPEG/WebP, **≤ 2 Mio**, validée par `http.DetectContentType` (voir « Validation d'icône ») |
| color | string | ❌ | `#rrggbb` ou `""`/absent = aucune. Autre valeur ⇒ `400` |

Réponse `200` : `CategoryInfo` complet (avec `color`, `questionCount: 0`, `rafaleCount: 0`). Sidecar écrit
`{ "name", "color" }` (`color` omise si vide). `OnCategoriesChanged({created, key})` ⇒ `CATEGORIES_CHANGED`. Erreurs : `409 game in progress` (v11.4.0, évaluée en premier), `400`
(nom, fichier, type, `color`), `409` (clé existante), `413` (> 2 Mio — nouveau, avant c'était 10 Mio sans contrôle de type). Le frontend propose par défaut la **première
teinte libre** de la palette (maquette §01) — décision d'interface, le serveur n'impose rien.

#### Validation d'icône (commune à POST /api/categories et POST /api/categories/{key}/icon)

1. `r.Body = http.MaxBytesReader(w, r.Body, 2<<20 + 64<<10)` puis `ParseMultipartForm` ⇒ dépassement ⇒ `413`.
2. Extension de `header.Filename` dans l'allowlist `.png .jpg .jpeg .webp` (insensible à la casse) ⇒ sinon `400`.
3. Les 512 premiers octets passés à `http.DetectContentType` doivent donner `image/png`, `image/jpeg` ou `image/webp`
   **cohérent avec l'extension** (`.jpg`/`.jpeg` ⇔ `image/jpeg`) ⇒ sinon `400 {"error":"file content does not match its extension"}`.
4. Nom disque = `<KEY>.<ext normalisée>` (`.jpeg` ⇒ `.jpg`), écrit en **`<KEY>.<ext>.tmp` puis `os.Rename`** ; toute
   autre `<KEY>.*` image préexistante est supprimée **après** le rename (jamais deux images pour une clé).
5. Jamais de chemin construit depuis `Filename` ; la clé a été validée `^[\p{L}\p{N}_]{1,50}$` + refus des noms réservés Windows en amont.

Tests exigés : `../x.png` en nom de fichier, clé `../../etc` en URL, PNG renommé `.webp`, HTML renommé `.png`,
2 Mio + 1 octet, remplacement `.png` → `.webp` (l'ancien `.png` disparaît), image envoyée sur une intégrée ⇒ `400`.

---

### PUT /api/categories/{key} (v11.4.0, #285)

Renommage et/ou couleur. Intégrées **et** personnalisées. Clé immuable.

| Propriété | Valeur |
|-----------|--------|
| Content-Type | application/json |
| Body | `{ "name"?: string, "color"?: string }` — au moins un champ |

- `name` : mêmes règles que `POST` (max 50, caractères autorisés) ; ne change **que** le nom affiché — les
  `question.json`, le réservoir, les `rounds.json` et l'historique **ne sont pas réécrits** (ils portent la clé ;
  l'historique garde ses snapshots `CATEGORY_NAME`, le palmarès résout en direct via `ResolveCategoryMeta` — comportement
  existant).
- `color` : `^#[0-9a-fA-F]{6}$` ou `""` (= aucune) ; stockée telle quelle (le serveur ne restreint pas à la palette de
  12 teintes de la maquette — c'est l'interface qui la propose).
- Écriture sidecar `<KEY>.json` (fusion : un champ absent du body est conservé), tmp + rename.
- Effet immédiat : `OnCategoriesChanged(change)` (rappel `HTTPServer` → `App`) ⇒ **`CATEGORIES_CHANGED`** diffusé à
  admin/tv/vplayer/anim (`websocket-actions.md` §CATEGORIES_CHANGED — révision 2, 2026-11-06 : avant, seul `QUESTIONS`
  admin-only était émis et la TV ne voyait rien avant rechargement), puis `broadcastQuestions()` **et**
  `a.ambiance().NotifyState()` (`lighting.md` §10.5 : tout ce qui change la salle notifie — une question de cette
  catégorie peut être en cours). `CHANGE.TYPE` = `renamed` si `name` présent dans le body, sinon `colored`.
- Réponse `200` : `CategoryInfo` résolu. Erreurs : `409 game in progress` (en premier), `400` (clé invalide, body
  vide, `name`/`color` invalides), `404` (clé inconnue), `405`.

---

### POST /api/categories/{key}/icon (v11.4.0, #285)

Remplace l'icône d'une catégorie **personnalisée** (multipart, champ `file`, « Validation d'icône » ci-dessus).

- Intégrée ⇒ **`400 {"error":"built-in categories keep their emoji"}`** (décision utilisateur ; pas de `DELETE …/icon`).
- `409 game in progress` (en premier) ; `404` clé inconnue ; `413` > 2 Mio ; `400` type/contenu.
- Réponse `200` : `CategoryInfo` résolu (nouvelle `imageURL`, l'extension peut changer). `OnCategoriesChanged({icon, key})`
  ⇒ `CATEGORIES_CHANGED` (`CHANGE.TYPE: "icon"`).
- Le navigateur doit contourner son cache : chaque client ajoute `?v=<TS>` à `imageURL` de la clé touchée à la réception
  de `CATEGORIES_CHANGED` (`TS` du message) — la régie qui a fait l'envoi comme la TV.

---

### DELETE /api/categories/{key} (v11.4.0, #285)

| Propriété | Valeur |
|-----------|--------|
| Content-Type | application/json |
| Body | `{ "mode": "reassign" \| "delete" }` — **obligatoire dès que la catégorie est utilisée** |

**Gardes, dans l'ordre** :
1. **Partie en cours** ⇒ `409 {"error":"game in progress"}` (règle commune ci-dessus).
2. clé invalide ⇒ `400` ; intégrée ⇒ `400 {"error":"built-in categories cannot be deleted"}` ; inconnue ⇒ `404`.
3. Utilisation = `questionCount + rafaleCount > 0` (même calcul que `GET`) **ou `history.events > 0`**
   (`CountHistoryCategory(key)` — révision 2, revue LOT-4 MAJEUR : une catégorie sans question mais avec de
   l'historique déclenche aussi la confirmation, puisque sa suppression **réécrit** `history.json`) **et** `mode`
   absent/invalide ⇒
   `409 {"error":"category in use","questions":12,"rafale":30,"history":{"events":3,"points":1240}}` — c'est le
   payload qui alimente le dialogue (maquette §02 « Supprimer “Cinéma” — 12 questions et 30 … »). Historique seul ⇒
   `{"questions":0,"rafale":0,"history":{"events":3,"points":1240}}` : le frontend affiche le dialogue avec la **case
   obligatoire** (règle existante `history.events > 0`), les deux modes étant équivalents. `history.events`
   = événements dont `QUESTION_CATEGORY == key` **ou** dont `CATEGORY_BREAKDOWN` contient `key` ; `history.points` =
   somme des points concernés. Vraiment inutilisée (0/0/0) ⇒ `200` direct, inchangé.

**Exécution** (après les gardes), **dans cet ordre** — l'historique d'abord parce que c'est l'étape la plus risquée
et la seule à exiger une sauvegarde : si elle échoue, **rien d'autre n'est touché** (`500`, état inchangé) :

| Étape | Action |
|---|---|
| H. Historique | `Engine.MergeHistoryCategory(key) (events, points int, err)` — sous `e.mu` : pour chaque événement, `QUESTION_CATEGORY == key` ⇒ `""` et `CATEGORY_NAME`/`CATEGORY_IMAGE_URL`/`CATEGORY_COLOR` vidés (snapshot d'une catégorie qui n'existe plus) ; `CATEGORY_BREAKDOWN[key]` ⇒ sa part est **ajoutée** à `CATEGORY_BREAKDOWN[""]` (créée si besoin) et la clé retirée — la somme des parts reste égale à `POINTS`. **Sauvegarde** `history.json.bak-<YYYYMMDD-HHmmss>` (copie octet pour octet du fichier courant, même dossier `config/`, **5 plus récentes conservées**, les autres supprimées ; jamais incluses dans le `.tar` de sauvegarde — `addFileToTAR` n'ajoute que `history.json`) **puis** écriture **atomique** (`history.json.tmp` + `os.Rename`, patron `atomicWriteFile` de `rafale_store.go`). Aucun événement concerné ⇒ pas de `.bak`, pas de réécriture. |
| Q. Questions | `mode: "reassign"` ⇒ chaque `question.json` (et chaque carte MEMOTION) dont `CATEGORY == key` est réécrit avec `CATEGORY: ""` (tmp + rename) ; `mode: "delete"` ⇒ le dossier `questions/<id>/` de chaque question dont `CATEGORY == key` est supprimé (`os.RemoveAll`, médias et son compris), `ForgetQuestionStatuses(ids)`, retrait de `rounds.json` par le **chemin existant** de `DELETE /question/{id}` (factorisé, jamais dupliqué) ; une **carte MEMOTION** dont `CATEGORY == key` est **supprimée de sa question hôte** (liste de cartes réécrite, ses médias de carte supprimés ; la question hôte est conservée même sans carte restante, et comptée dans `questions.cardsDeleted`) — décision utilisateur 2026-11-05 ; en mode `reassign` la carte passe à `""` comme le reste (`cardsReassigned`). Dans les **deux** modes, `key` est **retiré** de toute liste `RAFALE_CATEGORIES` (questions RAFALE et cartes RAFALE) ; une liste devenue vide reste vide (= toutes catégories, sémantique `EffectiveRafaleCategories` inchangée). |
| R. Réservoir | `reassign` ⇒ `Engine.ReassignRafaleCategory(key, "")` (réponses non tirables tant qu'elles n'ont pas de catégorie — maquette : « à recatégoriser ») ; `delete` ⇒ `Engine.DeleteRafaleQuestionsByCategory(key)` (+ purge de `rafale_used` pour ces IDs). Sauvegarde `SaveRafale`/`SaveRafaleUsed` existantes. |
| F. Fichiers | suppression de `<KEY>.<ext>` et `<KEY>.json`. |
| N. Notification | `OnCategoriesChanged({deleted, key})` ⇒ `CATEGORIES_CHANGED` (liste sans la clé) + `broadcastQuestions()` + `NotifyState()` ; `OnQuestionUpload()` si des questions ont changé. |

#### Response 200

```json
{ "deleted": "CINEMA", "mode": "reassign",
  "questions": { "reassigned": 12, "deleted": 0, "cardsReassigned": 2, "cardsDeleted": 0, "rafaleFiltersCleaned": 3 },
  "rafale":    { "reassigned": 30, "deleted": 0 },
  "history":   { "events": 3, "points": 1240, "backup": "history.json.bak-20261105-173306" } }
```

`backup` est `""` quand aucun événement n'était concerné. Erreurs : `400`, `404`, `405`, `409` (deux formes ci-dessus),
`500` (échec d'écriture — état inchangé si l'échec est à l'étape H ; sinon la réponse `500` porte `"partial": true` et
les compteurs déjà appliqués).

---

### Frontend (normatif pour dev-frontend — maquette §01/§02)

- **API-first** : `categoryMeta(key, categories)` (`utils/categoryUtils.js`) résout **d'abord** l'entrée `GET
  /api/categories` (nom, `color` — `""` ⇒ `#6b7280` pour les badges, `imageURL`), la table JS `CATEGORIES` ne fournit
  plus que l'**emoji** des intégrées et un repli hors ligne ; `getCategoryColor` (`constants/colors.js`, néon TV) prend
  la liste API en second argument et retombe sur la table. La table `CATEGORIES` de `constants/colors.js` (doublon de
  `categoryUtils.js`) est **supprimée** au profit d'un import unique. `CategoryBalance` compte aussi les personnalisées.
- `useCategories` : **mis à jour en direct** par `CATEGORIES_CHANGED` (liste appliquée telle quelle, sans refetch ;
  `bustCategoryIcon(KEY)` avec `TS` sur `icon`/`created`) — `websocket-actions.md` §CATEGORIES_CHANGED. Le contournement
  « évènement `window` sur `QUESTIONS` » (révision 1) est **retiré**. `GET /api/categories` reste l'état initial au montage.
- `CategoryPicker` : chip « ⚙ Gérer… » à côté de « + Créer » (mode `single` avec `allowCreate`, et mode `filter`
  de la page Questions) ; création enrichie : nom + icône (obligatoire) + couleur (pastille, défaut = 1ʳᵉ teinte libre).
- `CategoryManagerDialog` (nouveau, `role="dialog"`, maquette §02) : lignes = pastille-bouton (`aria-haspopup`, palette
  12 teintes + « aucune », `PUT {color}`), icône (bouton d'envoi **sur les personnalisées seulement** ; emoji statique +
  mention « emoji » sur les intégrées), nom + ✎ renommage en ligne (`PUT {name}`), compteurs `questionCount` (+
  `rafaleCount` en infobulle), aperçu 💡 (couleur ou « blanc »), 🗑 (désactivé + 🔒 « intégrée » sur les intégrées).
  Suppression : `DELETE` sans `mode` ⇒ `200` (inutilisée) ou `409 category in use` ⇒ dialogue de confirmation
  (radio « Garder les questions, sans catégorie » présélectionné / « Supprimer aussi les questions », encart 📊
  depuis `history`, **case de confirmation obligatoire dès que `history.events > 0` ou mode `delete`**, textes de la
  maquette — en mode `delete`, le texte compte aussi les cartes MEMOTION), puis `DELETE {mode}`. `409 game in
  progress` sur **toute** action (créer, renommer, colorer, icône, supprimer) ⇒ message « Impossible pendant une
  partie » ; le dialogue reste consultable (lecture seule).
- Palmarès : libellé du bucket `UNKNOWN` = « Sans catégorie ».

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

> **v11.4.0 (#285, LOT-4) — fusion d'une catégorie supprimée.** `DELETE /api/categories/{key}` réécrit `history.json`
> (§Catégories, étape H) : `QUESTION_CATEGORY == key` ⇒ `""` (champs `CATEGORY_*` vidés), `CATEGORY_BREAKDOWN[key]`
> fusionné dans `CATEGORY_BREAKDOWN[""]`. Le palmarès crédite alors le bucket `UNKNOWN` (« Sans catégorie ») — aucune
> perte de points (somme conservée). Sauvegarde `config/history.json.bak-<horodatage>` (5 conservées), écriture atomique.

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
(`data/files/sounds/<cue>.wav`). **Il doit être validé contre le catalogue fermé des cues** (7 en v11.0 ; **15 depuis v11.6.0**, `sound.md` §2.1 amendement #231)
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
  ],
  "long": [
    {
      "name": "decompte",
      "kind": "countdown",
      "enabled": true,
      "loop": true,
      "custom": false,
      "duration_seconds": 2.0,
      "path": "/files/sounds/decompte.wav"
    }
  ]
}
```

> **v11.4.0 (#287/#288, LOT-3)** — tableau **`long`** (additif ; `cues` byte-identique) : les sons longs (≤ 30 s,
> `sound.md` §10.4), aujourd'hui le seul `decompte` (`kind: "countdown"`, `sound.md` §12). `enabled` =
> `!SoundConfig.CountdownDisabled`, `loop` = `!SoundConfig.CountdownOnce`, `custom` = manifeste (`sounds.json`, entrée
> `decompte`), `duration_seconds` calculée comme pour les cues, `path` servi par `handleFiles`. Toujours présent
> (`[]` impossible aujourd'hui : l'entrée `decompte` est toujours listée, `custom:false`/`duration_seconds:null` si le
> fichier manque — `present` n'est pas exposé, comme pour les cues).

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
{"active": true, "muted": false}
```

> **v11.4.0 (#286, LOT-1)** : champ `muted` ajouté (additif) — son coupé persisté (`sound.muted`), `contracts/sound.md` §11.
> `muted` est indépendant d'`active` : `active=true, muted=true` = sortie disponible mais régie en silence.

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

### `POST /config.json {"sound": {"muted": bool}}` — son coupé (v11.4.0, #286, LOT-1)

**Aucun endpoint dédié** (même règle que `enabled` et `cues_disabled`, `contracts/sound.md` §6.3/§11) : le réglage
ponctuel du popover « Ambiance » s'écrit par le patch partiel additif existant, fusion champ par champ (clé absente
⇒ valeur préservée). Persisté. Effets de bord d'un passage à `true` (dans `handleConfig`, après écriture) : arrêt net
du média de question en cours (`QUESTION_SOUND_STATE` → `IDLE`, chronomètre différé libéré) et du son de décompte.
Un passage à `false` ne reprend rien. `POST /api/sounds/{cue}/test` reste joué même coupé (geste explicite).
`GET /api/sound/status.muted` le reflète. Même mécanisme pour l'interrupteur général de l'éclairage d'ambiance :
`POST /config.json {"lighting": {"enabled": bool}}` (`sound.md` §11.4).

### `POST /api/sounds/decompte` · `POST /api/sounds/decompte/restore` · `POST /api/sounds/decompte/test` (v11.4.0, #287, LOT-3)

Branche **dédiée** de `handleAPISoundsRouter` : le segment `decompte` est reconnu **avant** `soundCueFromString`
(qui reste inchangé et refuse toujours `decompte` : ce n'est pas une cue, `sound.md` §12.1). Contrat `sound.md` §12.

| Route | Comportement | Différences avec `/api/sounds/{cue}…` |
|---|---|---|
| `POST /api/sounds/decompte` (multipart `file`) | remplace `data/files/sounds/decompte.wav` ; validation **`audio.ValidateQuestionSound`** (≤ 30 s, ≤ 6 Mio, mono accepté ⇒ stocké stéréo, `sound.md` §10.4) ; écriture tmp + rename ; manifeste réconcilié ; réponse `{status:"ok", custom:true, duration_seconds}` | limite **30 s** (cues : 5 s) ; `413` au-delà de 6 Mio ; erreurs `writeSoundError` (`{status:"error", message}`) |
| `POST /api/sounds/decompte/restore` | réécrit `synth.GenerateCountdown()` ; idempotent ; `{status:"ok", custom:false, duration_seconds:2.0}` | — |
| `POST /api/sounds/decompte/test` | `countdownSound().Test()` : joue le fichier **une fois** sur l'enceinte, **ignore** `muted`, `countdown_disabled` et `sound.enabled` éteint au même titre que `/{cue}/test` (`sound.md` §11.2/§12.4) ; remplace un décompte en cours ; réponse `{played|disabled|unavailable}` avec la **même** sémantique que `/{cue}/test` (`disabled` = `sound.enabled == false`) | lecteur n°2 (`MediaPlayer`), jamais `PlayCue` |

`POST /api/sounds/restore-defaults` restaure désormais **7 cues + `decompte`** : `written` **inchangé** (les 7 cues,
forme #229 conservée pour les clients existants) et **nouveau** `restored` = 8 noms (7 cues + `"decompte"`), additif.

**Réglages** (`sound.md` §12.3) — `POST /config.json {"sound": {"countdown_disabled": bool, "countdown_once": bool}}`,
fusion champ par champ, aucun endpoint dédié (même règle que `muted`/`cues_disabled`). Effet de bord serveur : passage de
`countdown_disabled` à `true` ⇒ arrêt immédiat d'un décompte en cours.

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
  "written": ["depart", "temps-ecoule", "gagne", "perdu", "reveal", "entracte-debut", "entracte-fin"],
  "restored": ["depart", "temps-ecoule", "gagne", "perdu", "reveal", "entracte-debut", "entracte-fin", "decompte"]
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


---

### Manches et sauvegarde (v11.2, #245)
- `GET /questions` : chaque question porte son `ROUND_ID` éventuel. La liste des manches est dans `GameState.ROUNDS`.
- **Sauvegarde / restauration** : `data/files/questions/rounds.json` fait partie de `files/questions`, donc il est sauvegardé et restauré **avec les questions**. Une restauration remplace les manches comme elle remplace les questions. Une question qui cite une manche absente tombe dans « Sans manche ».
- **Génération IA** (`ai-generation.md` §5.2) : les questions générées n'ont pas de `ROUND_ID`. Elles arrivent **à la fin du groupe « Sans manche »**, qui est **en tête** de la liste *(décision utilisateur)*. `ORDER` = plus grand `ORDER` de ce groupe + 1, comme aujourd'hui.

---

## Sound & Ambiance — amendements v11.6.0 (#283, #232, #235, #231, #290)

> Plan `_work/reports/planner-plan-v11.6.0.md` (Révision 2). Tous additifs ; aucun BREAKING.

### `GET /api/sound/status` — diagnostic (#283, LOT-1)

```json
{"active": true, "muted": false,
 "enabled": true, "output": "ok", "reason": null, "restart_required": false, "muted_source": "global"}
```

| Champ | Type | Description |
|---|---|---|
| `active`, `muted` | bool | **Inchangés** (v11.0 / v11.4.0). `muted` = valeur **résolue** (`ambiance-settings.md`) |
| `enabled` | bool | `sound.enabled` lu **au démarrage** (valeur qui a ouvert ou non la sortie) |
| `output` | `"ok" \| "unavailable" \| "disabled" \| "restart_required"` | `ok` : sortie ouverte ; `unavailable` : `enabled` mais contexte audio refusé (`IsNeutral`) ; `disabled` : `enabled=false` au démarrage ; `restart_required` : `sound.enabled` **courant ≠ valeur au démarrage** (interrupteur changé sans redémarrage, dans un sens ou l'autre) |
| `reason` | string \| null | Cause lisible, **même texte que le journal de démarrage** (ex. `oto: no output device`, `sound.enabled=false`, `enabled flipped false→true after startup`) ; `null` si `ok` |
| `restart_required` | bool | `output == "restart_required"` (commodité frontend) |
| `muted_source` | `"global" \| "round" \| "direct"` | Source du `muted` résolu (#290, LOT-4 ; `"global"` tant que LOT-4 n'est pas livré) |

Journal de démarrage (P1) : une ligne `INFO`/`WARN`/`ERROR` composant `AUDIO`
`sound: enabled=<b> output=<ok|unavailable|disabled> reason="…" device=<…> format=44100Hz/2ch/16bit (config: <chemin>)`
routée vers le logger serveur (`/ws/logs`, fichier `logging.md`) ; les `log.Printf` d'`internal/audio` passent par un
logger injecté (`audio.SetLogger`). Maquette `docs/mockup/v11.6.0/ui/admin_sounds_manager__output_diagnostic.html`.

### Uploads sonores — conversion automatique (#232, LOT-1)

`POST /api/sounds/{cue}`, `POST /api/sounds/decompte`, `POST /questions` (`sound`, `motion_card_<id>_sound`) : la règle
« format canonique strict » de la table de `POST /api/sounds/{cue}` devient **« WAV PCM, converti en canonique »**
(`sound.md` §3 amendement #232 : tags 1/3/extensible, 8–32 bits ou float32, 1–2 canaux, 8–192 kHz ; `.mp3` toujours
refusé). Réponses `200` : champs additifs `converted: bool` et `source_format: string|null` (ex. `"48000Hz/1ch/24bit"`,
`null` si déjà canonique). `POST /questions` : mêmes champs dans la réponse JSON. Erreurs : `400` « pas WAV / format
PCM inconnu / compressé » (texte nommant le cas), `413` au-delà de **4 ×** le plafond canonique du chemin (entrée brute)
ou du plafond canonique après conversion ; limites de **durée** inchangées (5 s cue, 30 s média, calculées **après**
conversion). Le message frontend indique « converti depuis <source_format> » (toast existant).

### `GET /api/sounds` — cues par type (#231, LOT-3)

`cues[]` gagne `type: "" | "QCM" | "MEMORY" | "MEMOTION" | "RAFALE"` et contient les **15** cues dans l'ordre normatif
(global puis par type — `sound.md` §2.1 amendement). `long[]` inchangé. Les endpoints `POST /api/sounds/{cue}`,
`/restore`, `/test` acceptent les 8 nouvelles cues (validation URL contre `audio.AllCues()`), sans autre changement.
`POST /api/sounds/restore-defaults` restaure les 15 + `decompte`.

### `POST /questions` — sons de cartes MEMOTION (#235, LOT-2)

| Champ | Type | Description |
|---|---|---|
| `motion_card_<cardID>_sound` | file | Média sonore de la **carte** (`sound.md` §14.4) — mêmes règles que `sound` (`ValidateQuestionSound` après conversion §232, ≤ 30 s, ≤ 6 Mio) ; écrit `MOTION_CARDS[].SOUND = "/question/<id>/motion_<cardID>_sound_<rand4>.wav"` |
| `motion_card_<cardID>_sound_cleared` | `"true"`/absent | Supprime le son de la carte (fichier compris) |
| `motion_card_<cardID>_sound_timer_delayed` | `"true"`/absent | Différé du **chrono de la carte** ; écrit seulement si un son est attaché |
| `motion_card_<cardID>_sound_loop` | `"true"`/absent | Boucle ; écrit seulement si un son est attaché |

**Préservation serveur par `ID` de carte** (normatif, `sound.md` §14.4) : une carte existante sans nouveau fichier ni
`_sound_cleared` **conserve** ses trois champs depuis le `question.json` existant. `sound` / `sound_loop` restent
acceptés pour **MEMORY**, **ENTRACTE** et l'**hôte MEMOTION** (champ commun, §10.4bis) ; `sound_timer_delayed` est ignoré
(écrit `false`) pour ENTRACTE et hôte MEMOTION (pas de chrono global). Réponse : `warning` de durée (v11.1) étendu aux
cartes sous la forme `card_warnings: [{card_id, warning}]` (additif, `[]` sinon).

### `POST /config.json {"lighting": {"default_mode": "ON"|"AUTO"|"OFF"}}` (#290, LOT-4)

Nouveau réglage **persisté** (D8), fusion champ par champ existante ; `400` hors valeurs. Valeur **globale** du mode
d'éclairage (`ambiance-settings.md` §2) ; `POST /api/lighting/mode` reste le **geste direct** et n'écrit jamais
`default_mode` (§4.2 point 4). `GET /config.json` l'expose (absent ⇒ `AUTO`).

### `GET /api/ambiance/settings` ✨ (#290, LOT-4)

| Propriété | Valeur |
|---|---|
| Auth | Aucune |
| Méthode | `GET` uniquement |

```json
{
  "global":   { "sound": { "muted": false, "countdown": {"enabled": true, "loop": true}, "cues": { "depart": true, "…": true } }, "light": { "mode": "AUTO" } },
  "effective":{ "sound": { "muted": false, "countdown": {"enabled": false, "loop": true}, "cues": { "…": true, "qcm-indice": false } }, "light": { "mode": "ON" } },
  "sources":  { "sound": { "muted": "global", "countdown": {"enabled": "round", "loop": "global"}, "cues": { "…": "global", "qcm-indice": "round" } }, "light": { "mode": "round" } },
  "round_id": "r2",
  "round_override": { "sound": { "countdown": {"enabled": false}, "cues": {"qcm-indice": false} }, "light": {"mode": "ON"} }
}
```

Formes : `ambiance-settings.md` §3. `round_id = ""` et `round_override = {}` hors manche. Lu par la fenêtre « Ambiance de
la manche » (`base = global`) et par le popover en secours de `GameState.AMBIANCE_EFFECTIVE`.

### Écritures de réglages existantes — effet « geste direct » (#290, LOT-4)

`POST /config.json {"sound": {"muted"|"cues_disabled"|"countdown_disabled"|"countdown_once"}}` et `POST /api/lighting/mode`
sont des **gestes directs** (`ambiance-settings.md` §4.2) : ils alimentent la couche directe et, **si la manche effective
ne surcharge pas le champ**, le global persisté (comportement v11.4.0 conservé). Sinon le global n'est pas écrit et la
réponse `200` porte `{"status":"ok","scope":"round"}` (additif ; `"scope":"global"` sinon) pour que la page Ambiance
puisse l'indiquer.


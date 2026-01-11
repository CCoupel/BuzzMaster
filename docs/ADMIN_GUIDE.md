# Guide d'Administration - BuzzControl

Ce document decrit les fonctionnalites d'administration du systeme BuzzControl.

## Table des matieres

- [Persistance des donnees](#persistance-des-donnees)
- [Sauvegarde et restauration](#sauvegarde-et-restauration)
- [Reinitialisation selective](#reinitialisation-selective)
- [Gestion des scores](#gestion-des-scores)
- [Historique des evenements](#historique-des-evenements)

---

## Persistance des donnees

### Fichiers de configuration

Les donnees sont automatiquement sauvegardees dans le dossier `data/config/` :

| Fichier | Contenu |
|---------|---------|
| `teams.json` | Equipes avec scores et TeamPoints |
| `bumpers.json` | Joueurs avec scores et assignations |
| `history.json` | Historique des evenements (source de verite) |

### Auto-save

Chaque modification declenche une sauvegarde asynchrone :
- Modification d'equipe ou joueur
- Attribution de points
- Remise a zero des scores

### Chargement au demarrage

Au demarrage du serveur :
1. Les fichiers `teams.json` et `bumpers.json` sont charges
2. Si les fichiers existent et contiennent des donnees, les donnees de test ne sont pas initialisees
3. Les scores peuvent etre recalcules depuis l'historique

---

## Sauvegarde et restauration

### Page Configuration (`/settings`)

La section **Sauvegarde** permet de choisir quoi inclure dans l'archive :

| Option | Description |
|--------|-------------|
| Questions | Dossiers questions avec medias |
| Equipes | Fichier `teams.json` |
| Joueurs | Fichier `bumpers.json` |
| Historique | Fichier `history.json` |
| Fonds | Images de fond d'ecran |

### Endpoints API

#### Sauvegarde selective

```
GET /backup-select?questions=true&teams=true&bumpers=true&history=true&backgrounds=true
```

Retourne un fichier TAR contenant uniquement les elements selectionnes.

#### Sauvegarde complete (legacy)

```
GET /backup
```

Retourne un fichier TAR complet (toutes les donnees).

#### Restauration intelligente

```
POST /restore
Content-Type: multipart/form-data
Body: file=<archive.tar>
```

Le serveur detecte automatiquement le contenu de l'archive et restaure uniquement les elements presents :
- Detecte les dossiers `questions/`
- Detecte les fichiers `teams.json`, `bumpers.json`, `history.json`
- Detecte les fichiers `backgrounds/`

Apres restauration, les donnees sont rechargees en memoire.

---

## Reinitialisation selective

### Page Configuration (`/settings`)

La section **Reinitialisation** permet de choisir quoi remettre a zero :

| Option | Action |
|--------|--------|
| Questions | Supprime tous les dossiers questions |
| Equipes | Vide la liste des equipes |
| Joueurs | Vide la liste des joueurs |
| Historique | Efface l'historique des evenements |
| Fonds | Supprime les images de fond |

### Endpoint API

```
POST /reset-select?questions=true&teams=true&bumpers=true&history=true&backgrounds=true
```

Reinitialise uniquement les elements selectionnes.

### Remise a zero des scores uniquement

Action WebSocket `RAZ` : Remet tous les scores a zero sans supprimer les equipes/joueurs.

---

## Gestion des scores

### Architecture des scores

```
Score total equipe = TeamPoints + sum(scores joueurs)
```

| Champ | Description |
|-------|-------------|
| `Team.TeamPoints` | Points attribues directement a l'equipe |
| `Team.Score` | Score total calcule (TeamPoints + joueurs) |
| `Bumper.Score` | Points individuels du joueur |

### Attribution des points

#### Via l'interface admin (GamePage)

- **Clic sur l'en-tete d'equipe** : Ajoute des points a l'equipe (TeamPoints)
- **Clic sur la ligne joueur** : Ajoute des points au joueur

#### Via WebSocket

```json
// Points equipe
{ "ACTION": "TEAM_POINTS", "MSG": { "TEAM": "Les Rouges", "POINTS": 10 } }

// Points joueur
{ "ACTION": "BUMPER_POINTS", "MSG": { "ID": "AA:BB:CC:DD:EE:FF", "POINTS": 5 } }
```

### POINTS_TARGET sur les questions

Chaque question definit a qui les points sont attribues :

| Valeur | Description | Defaut pour |
|--------|-------------|-------------|
| `PLAYER` | Points au joueur qui repond | Questions NORMAL |
| `TEAM` | Points a l'equipe | Questions QCM |

---

## Historique des evenements

### Modele GameEvent

```json
{
  "TIMESTAMP": 1234567890123,
  "QUESTION_ID": "1",
  "EVENT_TYPE": "POINTS_AWARDED",
  "TEAM_NAME": "Les Rouges",
  "TEAM_COLOR": "#ef4444",
  "PLAYER_NAME": "Alice",
  "PLAYER_COLOR": "#22c55e",
  "WINNER_TYPE": "PLAYER",
  "POINTS": 10,
  "REACTION_TIME": 1234567
}
```

### Page Historique (`/history-page`)

Affiche les evenements groupes par question :
- Vue reduite : Badges resume par equipe/joueur
- Vue detaillee : Tableau avec heure, equipe, joueur, temps, points
- Boutons "Tout ouvrir" / "Tout fermer"

### Endpoint API

```
GET /history
```

Retourne la liste des `GameEvent` en JSON.

### Event Sourcing

L'historique est la **source de verite** pour les scores :
- Fonction `RecalculateScoresFromHistory()` recalcule tous les scores
- Permet de reconstruire l'etat a tout moment
- Garantit la coherence des donnees

---

## Resume des endpoints admin

| Methode | Endpoint | Description |
|---------|----------|-------------|
| GET | `/backup` | Sauvegarde complete |
| GET | `/backup-select?...` | Sauvegarde selective |
| POST | `/restore` | Restauration intelligente |
| POST | `/reset-select?...` | Reinitialisation selective |
| GET | `/history` | Liste des evenements |
| GET | `/version` | Version du serveur |
| GET | `/config.json` | Configuration |
| POST | `/config.json` | Modifier configuration |

---

## Bonnes pratiques

1. **Sauvegardes regulieres** : Effectuer une sauvegarde avant chaque session de jeu
2. **Historique intact** : Ne pas reinitialiser l'historique sauf si necessaire
3. **Restauration** : Toujours verifier le contenu de l'archive avant restauration
4. **Scores** : Utiliser POINTS_TARGET adapte au type de question

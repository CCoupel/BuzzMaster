# Guide d'Administration - BuzzControl

Ce document decrit les fonctionnalites d'administration du systeme BuzzControl.

## Table des matieres

- [Persistance des donnees](#persistance-des-donnees)
- [Sauvegarde et restauration](#sauvegarde-et-restauration)
- [Reinitialisation selective](#reinitialisation-selective)
- [Gestion des scores](#gestion-des-scores)
- [Historique des evenements](#historique-des-evenements)
- [Joueurs Virtuels (VPlayer)](#joueurs-virtuels-vplayer)
- [Configuration Effet Neon](#configuration-effet-neon)

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

## Joueurs Virtuels (VPlayer)

### Presentation

La fonctionnalite VPlayer permet aux joueurs de buzzer depuis leur smartphone en scannant un QR Code. Les joueurs virtuels fonctionnent exactement comme des buzzers physiques une fois inscrits.

### Workflow d'inscription

1. **Ouvrir les inscriptions**
   - Aller sur la page Joueurs (`/admin/teams` ou `/anim/teams`)
   - Section "Inscriptions" en haut de la page
   - Cliquer sur "Lancer Inscriptions"

2. **Affichage du QR Code**
   - Le QR Code s'affiche automatiquement sur l'affichage TV (`/tv`)
   - Les joueurs scannent le QR Code avec leur smartphone
   - Barre de progression : Inscrits X/Y (Y = limite max)

3. **Inscription joueur**
   - Le joueur arrive sur la page d'inscription (`/`)
   - Il saisit un pseudo (2-20 caracteres)
   - Clic sur "Rejoindre" → redirection automatique vers `/player`

4. **Fermer les inscriptions**
   - Retour sur `/admin/teams`
   - Cliquer sur "Fin Inscriptions"
   - Le QR Code disparait de l'affichage TV

### Page joueur (`/player`)

L'interface joueur affiche :
- **Badges permanents** : Nom du joueur (gauche), Équipe (droite)
- **Affichage TV** : Vue synchronisee du jeu en cours
- **Zone media cliquable** : Cliquer sur l'image pour buzzer (76% de largeur)
- **Bouton BUZZ** : États visuels selon la phase du jeu

#### États du bouton BUZZ

| Phase du jeu | Texte affiché | État | Couleur |
|--------------|---------------|------|---------|
| Pas d'équipe | "En attente..." | Désactivé | Gris |
| STOPPED | "En attente de question" | Désactivé | Gris |
| PREPARE | "Préparation..." | Désactivé | Orange |
| READY / COUNTDOWN | "Prêt !" | Désactivé | Cyan |
| STARTED | "BUZZ !" | Actif | Vert pulsant |
| PAUSED | "Déjà buzzé" | Désactivé | Bleu |

#### Feedback au buzz

Quand le joueur clique pour buzzer :
- **Vibration haptique** (100ms, si supporté)
- **Overlay vert** avec checkmark géant
- **Texte "BUZZÉ !"** avec glow vert
- Disparition automatique après 1.5 secondes

### Gestion des joueurs virtuels

#### Assignation à une équipe

Les joueurs virtuels apparaissent dans la colonne "Joueurs non assignés" :
- Utiliser le drag & drop pour assigner à une équipe
- Les joueurs assignés peuvent buzzer normalement

#### Suppression d'un joueur

Pour supprimer un joueur virtuel :
1. S'assurer qu'il n'est **pas assigné** à une équipe (drag vers colonne droite si besoin)
2. Cliquer sur le bouton × en haut à droite de la carte joueur
3. Confirmer la suppression

**Important :** Le joueur est automatiquement déconnecté et redirigé vers la page d'inscription.

#### Compteurs

Dans la section "Inscriptions" :
- **Places max** : Nombre maximum de joueurs virtuels (configurable)
- **Inscrits** : X/Y avec distinction 🎮 physiques / 📱 virtuels

### Restrictions

#### Questions MEMORY

Les joueurs virtuels **ne peuvent pas** buzzer sur les questions de type MEMORY :
- Le contrôle du jeu reste exclusif à l'admin
- Le buzz est bloqué côté serveur
- Aucun feedback visuel n'est affiché

#### Session et reconnexion

- Session stockée dans localStorage (24h)
- Si le joueur ferme puis rouvre `/player` → reconnexion automatique
- Si l'admin supprime le joueur → redirection automatique vers `/`

### Bonnes pratiques

1. **Limite de joueurs** : Définir une limite réaliste selon le nombre d'équipes
2. **Fermer les inscriptions** : Toujours fermer avant de commencer le jeu
3. **Assignation rapide** : Assigner les joueurs aux équipes dès leur inscription
4. **Suppression prudente** : Ne supprimer un joueur que s'il ne participe plus

---

## Configuration Effet Neon

### Presentation

L'effet néon affiche une bordure lumineuse animée autour de l'écran TV (`/tv`) et de l'interface VPlayer (`/player`). La couleur s'adapte automatiquement à la catégorie de la question en cours.

### Accès à la configuration

1. Ouvrir la page Configuration (`/admin/settings` ou `/anim/settings`)
2. Section "Effet Néon" en bas de la page
3. Quatre sliders pour ajuster les paramètres

### Paramètres disponibles

| Paramètre | Plage | Défaut | Description |
|-----------|-------|--------|-------------|
| **Activé** | On/Off | Off | Active ou désactive l'effet néon |
| **Largeur arc** | 30-180° | 60° | Largeur de l'arc lumineux en degrés |
| **Écart intensité** | 0-100% | 80% | Écart d'intensité (opacité du dégradé) |
| **Vitesse rotation** | 1-10s | 4s | Vitesse de rotation de l'arc (en secondes) |

### Comment utiliser

#### Activer l'effet

1. Cocher la case "Activer l'effet néon"
2. Ajuster les sliders selon vos préférences
3. Cliquer sur "Enregistrer la configuration"
4. L'effet s'applique **immédiatement** sur tous les écrans connectés

#### Phases actives

L'effet néon s'affiche uniquement pendant les phases de jeu suivantes :
- **READY** : Question prête à démarrer
- **COUNTDOWN** : Décompte avant le timer
- **STARTED** : Question en cours
- **PAUSED** : Jeu en pause après un buzz

L'effet **disparaît** pendant les phases :
- **STOPPED** : Jeu arrêté
- **REVEALED** : Réponse affichée

#### Couleur automatique

La couleur de la bordure correspond à la catégorie de la question active :
- **GEOGRAPHY** : Vert (#22c55e)
- **ENTERTAINMENT** : Magenta (#d946ef)
- **HISTORY** : Orange (#f59e0b)
- **SCIENCE** : Bleu (#3b82f6)
- **SPORTS** : Rouge (#ef4444)
- **ARTS** : Violet (#a855f7)
- **CULTURE** : Cyan (#06b6d4)
- **OTHER** : Gris (#6b7280)

### Conseils d'utilisation

#### Largeur de l'arc (30-180°)

- **30-60°** : Arc fin et discret
- **90°** : Arc moyen (un quart de cercle)
- **120-180°** : Arc large et spectaculaire

**Recommandation** : Commencer à 60° pour un équilibre entre visibilité et discrétion.

#### Écart d'intensité (0-100%)

Contrôle la différence d'opacité entre le point le plus lumineux et le point le moins lumineux :
- **0%** : Pas de dégradé (arc uniforme)
- **50%** : Dégradé modéré
- **80%** : Dégradé marqué (recommandé)
- **100%** : Dégradé maximal (fade complet)

**Recommandation** : 80% pour un effet néon réaliste avec dégradé visible.

#### Vitesse de rotation (1-10s)

- **1-2s** : Rotation rapide (dynamique)
- **4s** : Vitesse modérée (recommandé)
- **8-10s** : Rotation lente (ambiance)

**Recommandation** : 4s pour un effet fluide sans distraction.

### Diffusion en temps réel

Quand vous modifiez la configuration :
- Les changements sont **broadcastés instantanément** via WebSocket
- Tous les écrans connectés reçoivent la mise à jour (ACTION: CONFIG_UPDATE)
- Pas besoin de rafraîchir les pages

### Désactivation

Pour désactiver complètement l'effet :
1. Décocher "Activer l'effet néon"
2. Cliquer sur "Enregistrer la configuration"
3. La bordure disparaît immédiatement de tous les écrans

### Fichier de configuration

Les paramètres sont sauvegardés dans `server-go/config.json` :

```json
{
  "neon_effect": {
    "enabled": false,
    "arc_width": 60,
    "intensity_gap": 80,
    "rotation_speed": 4
  }
}
```

Ces paramètres sont chargés au démarrage du serveur et peuvent être modifiés via l'interface admin.

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
| GET | `/config.json` | Configuration (incluant effet néon) |
| POST | `/config.json` | Modifier configuration (incluant effet néon) |

---

## Bonnes pratiques

1. **Sauvegardes regulieres** : Effectuer une sauvegarde avant chaque session de jeu
2. **Historique intact** : Ne pas reinitialiser l'historique sauf si necessaire
3. **Restauration** : Toujours verifier le contenu de l'archive avant restauration
4. **Scores** : Utiliser POINTS_TARGET adapte au type de question

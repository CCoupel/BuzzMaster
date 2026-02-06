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
- [Mises a jour automatiques](#mises-a-jour-automatiques)

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
3. Toggle pour activer/désactiver
4. Deux onglets : **Structure** et **Glow**

### Modes d'affichage

L'effet néon propose 2 modes visuels :

| Mode | Description | Cas d'usage |
|------|-------------|-------------|
| **bar** (défaut) | Tube lumineux fin avec centre blanc et arc rotatif | Effet moderne et subtil |
| **halo** | Bordure lumineuse large type néon classique | Effet spectaculaire et immersif |

**Mode "bar" - Composition visuelle** :
- Tube fixe avec 3 couches (externe floutée, centrale précise, centre blanc)
- Arc rotatif au centre du tube avec hotspot blanc brillant
- Proportions équilibrées : 1/3 par couche

### Paramètres disponibles

**Onglet Structure** :

| Paramètre | Plage | Défaut | Description |
|-----------|-------|--------|-------------|
| **Activé** | On/Off | Off | Active ou désactive l'effet néon |
| **Mode** | bar / halo | bar | Type d'effet visuel |
| **Largeur arc** | 30-180° | 60° | Largeur de l'arc lumineux en degrés |
| **Écart intensité** | 0-100% | 80% | Écart d'intensité (opacité zone sombre) |
| **Vitesse rotation** | 1-10s | 4s | Vitesse de rotation de l'arc (en secondes) |
| **Distance bord** | 10-100px | 20px | Distance du tube par rapport au bord (mode bar) |
| **Épaisseur tube** | 2-20px | 4px | Épaisseur du tube lumineux (mode bar) |
| **Flou arc** | 0-200% | 100% | Flou de l'arc (% de épaisseur tube, mode bar) |

**Onglet Glow** :

| Paramètre | Plage | Défaut | Description |
|-----------|-------|--------|-------------|
| **Vitesse pulsation** | 0.5-5s | 2s | Vitesse de pulsation du glow |
| **Opacité min** | 0-100% | 30% | Opacité minimale du glow pulsant |
| **Opacité max** | 0-100% | 50% | Opacité maximale du glow pulsant |

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

#### Choix du mode

- **Mode "bar"** : Préférer pour un affichage discret et moderne. Le tube fin ne prend pas trop d'espace à l'écran.
- **Mode "halo"** : Préférer pour un affichage spectaculaire et immersif. La bordure large est très visible.

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

#### Distance du bord (10-100px, mode bar)

Contrôle la distance entre le tube et le bord de l'écran :
- **10-20px** : Tube proche du bord (recommandé)
- **30-50px** : Tube plus éloigné
- **60-100px** : Tube très éloigné (pour grands écrans)

**Recommandation** : 20px pour un effet bien cadré sans chevaucher le contenu.

#### Épaisseur du tube (2-20px, mode bar)

- **2-4px** : Tube très fin (discret, recommandé)
- **6-10px** : Tube moyen
- **12-20px** : Tube épais (très visible)

**Recommandation** : 4px pour un bon équilibre entre visibilité et discrétion.

#### Flou de l'arc (0-200%, mode bar)

Contrôle le flou de l'arc rotatif (en % de l'épaisseur du tube) :
- **0-50%** : Arc net et précis
- **100%** : Arc flou modéré (recommandé)
- **150-200%** : Arc très flou (effet glow intense)

**Recommandation** : 100% pour un effet lumineux réaliste.

#### Vitesse de pulsation (0.5-5s)

- **0.5-1s** : Pulsation rapide
- **2s** : Pulsation modérée (recommandé)
- **3-5s** : Pulsation lente (ambiance)

**Recommandation** : 2s pour une pulsation visible mais pas trop rapide.

#### Opacité du glow pulsant (0-100%)

Définit la plage d'opacité du glow pulsant :
- **Min 30% / Max 50%** : Pulsation subtile (recommandé)
- **Min 10% / Max 90%** : Pulsation très marquée
- **Min 40% / Max 60%** : Pulsation discrète

**Recommandation** : Min 30% / Max 50% pour une pulsation visible sans distraction.

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

Ces paramètres sont chargés au démarrage du serveur et peuvent être modifiés via l'interface admin.

---

## Mises a jour automatiques

### Presentation

BuzzControl peut vérifier et installer automatiquement les nouvelles versions depuis GitHub Releases. Cette fonctionnalité permet de garder votre installation à jour sans intervention manuelle complexe.

### Accès à la page de mise à jour

1. Cliquer sur le logo abeille (🐝) dans la navbar
2. Sélectionner "Mises à jour" dans le menu déroulant
3. La page `/admin/updates` s'affiche

**Notification automatique** :
- Un badge rouge avec icône apparaît sur le logo abeille si une mise à jour est disponible
- La vérification se fait automatiquement au démarrage du serveur (si `auto_check_updates: true`)

### Vérifier les mises à jour

Sur la page de mise à jour, vous verrez :
- **Version actuelle** : Version installée de BuzzControl
- **Dernière version** : Dernière version disponible sur GitHub
- **Statut** : À jour, mise à jour disponible, vérification en cours, etc.

Pour vérifier manuellement :
1. Cliquer sur "Vérifier maintenant"
2. Le serveur interroge GitHub Releases API
3. Le résultat s'affiche en quelques secondes

### Télécharger une mise à jour

Si une mise à jour est disponible :
1. Le bouton "Télécharger" devient actif
2. Le changelog de la nouvelle version s'affiche automatiquement
3. Cliquer sur "Télécharger"
4. La progression du téléchargement s'affiche (statut "Téléchargement...")
5. Une fois terminé, le bouton "Installer et redémarrer" devient actif

**Vérifications automatiques** :
- Détection automatique de la plateforme (Windows, Linux x64, Linux ARM64)
- Vérification de la taille du fichier (minimum 40 MB)
- Téléchargement dans le dossier du serveur

### Installer la mise à jour

Après téléchargement :
1. Cliquer sur "Installer et redémarrer"
2. Le serveur effectue les opérations suivantes :
   - Sauvegarde de l'ancien exécutable (suffixe `.backup`)
   - Remplacement par la nouvelle version
   - Redémarrage automatique du serveur
3. L'interface affiche "Installation en cours..."
4. Reconnexion automatique après redémarrage (polling toutes les 2 secondes)

**Durée estimée** :
- Installation : 2-5 secondes
- Redémarrage : 5-10 secondes
- Reconnexion : 2-4 secondes

### Que se passe-t-il pendant le redémarrage

Pendant la mise à jour :
1. **Backup** : L'ancien exécutable est sauvegardé avec l'extension `.backup`
2. **Remplacement** : Le nouveau fichier remplace l'ancien
3. **Redémarrage** : Le serveur redémarre automatiquement
4. **Reconnexion** : L'interface tente de se reconnecter toutes les 2 secondes (10 tentatives max)

**Important** :
- Ne fermez pas la page pendant la mise à jour
- Les connexions WebSocket seront temporairement coupées
- Les joueurs virtuels devront se reconnecter après le redémarrage

### Rollback automatique

En cas d'échec du redémarrage :
- Le serveur détecte que la nouvelle version ne démarre pas correctement
- Il restaure automatiquement l'ancien exécutable depuis `.backup`
- Un message d'erreur s'affiche dans l'interface

**Rollback manuel** :
Si le serveur ne redémarre pas du tout :
1. Arrêter le processus serveur (Ctrl+C ou kill)
2. Renommer `server.exe.backup` en `server.exe` (Windows)
3. Ou renommer `server.backup` en `server` (Linux)
4. Relancer le serveur normalement

### Vérifier que la mise à jour a réussi

Après reconnexion :
1. La page de mise à jour affiche la nouvelle version actuelle
2. Le statut passe à "À jour"
3. Le badge de notification disparaît de la navbar
4. Vérifier les logs pour confirmer le bon démarrage

### Configuration automatique

Dans `config.json` :

```json
{
  "server": {
    "auto_check_updates": true
  }
}
```

| Paramètre | Défaut | Description |
|-----------|--------|-------------|
| `auto_check_updates` | true | Vérifier les mises à jour au démarrage |

**Recommandation** : Laisser à `true` pour être notifié des nouvelles versions.

### Cache GitHub API

Pour éviter le rate limiting GitHub (60 requêtes/heure pour IP publique) :
- Le serveur met en cache la réponse pendant 1 heure
- Les vérifications fréquentes utilisent le cache
- Le cache se rafraîchit automatiquement après expiration

### Bonnes pratiques

1. **Sauvegarder avant mise à jour** : Effectuer une sauvegarde complète via `/admin/backup`
2. **Lire le changelog** : Vérifier les changements de la nouvelle version
3. **Tester après mise à jour** : Vérifier que toutes les fonctionnalités marchent
4. **Garder le .backup** : Ne supprimez pas le fichier `.backup` immédiatement après mise à jour

### Dépannage

#### La vérification échoue

**Erreur : "Failed to check for updates"**
- Vérifier la connexion Internet
- GitHub API peut être temporairement indisponible
- Rate limit GitHub atteint (attendre 1 heure)

#### Le téléchargement échoue

**Erreur : "Failed to download update"**
- Vérifier l'espace disque disponible (minimum 100 MB)
- Vérifier les permissions d'écriture dans le dossier du serveur
- Le fichier téléchargé est trop petit (< 40 MB) - relancer le téléchargement

#### Le redémarrage échoue

**Erreur : "Server did not restart"**
- Le serveur tente un rollback automatique
- Vérifier les logs pour identifier le problème
- Effectuer un rollback manuel si nécessaire (voir section Rollback)

#### L'interface ne se reconnecte pas

**Après 10 tentatives, toujours "Reconnexion..."**
- Rafraîchir la page manuellement (F5)
- Vérifier que le serveur est bien démarré (logs)
- Vérifier le port 80 disponible

### Limites connues

- **Plateforme** : Mise à jour disponible uniquement pour Windows, Linux x64 et Linux ARM64
- **Permissions** : Le serveur doit avoir les droits d'écriture sur son propre exécutable
- **Session** : Les joueurs virtuels doivent se reconnecter après mise à jour
- **État du jeu** : La partie en cours est interrompue pendant la mise à jour

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
| GET | `/api/updates/check` | Vérifier les mises à jour disponibles |
| POST | `/api/updates/download` | Télécharger une mise à jour |
| POST | `/api/updates/apply` | Installer et redémarrer |

---

## Jeu Memory - Modes multi-équipes

### Sélection du mode de jeu

Dans l'éditeur de question Memory (page Questions), vous pouvez choisir parmi 3 modes :

1. **SOLO** (par défaut) : Une seule équipe joue, comportement classique
2. **CHACUN SON TOUR** : Les équipes jouent à tour de rôle, rotation après chaque paire
3. **TANT QUE JE GAGNE** : L'équipe continue tant qu'elle trouve des paires valides

### Sélection des équipes participantes

Pour les modes multi-équipes :
1. En phase PREPARE, une interface de sélection apparaît
2. Cochez les équipes qui participent (minimum 2)
3. Cliquez sur "Valider équipes" avant de démarrer

### Affichage TV

- Badge "Au tour de : [Équipe]" en haut de la grille
- Tableau des scores par équipe sous le badge
- L'équipe active est mise en évidence (bordure dorée)

### Modes de jeu détaillés

#### Mode SOLO
- Une seule équipe joue
- Tous les joueurs de l'équipe peuvent retourner les cartes
- Points attribués à l'équipe à la fin du jeu

#### Mode CHACUN SON TOUR
- Multi-équipes en rotation stricte
- On change d'équipe à chaque retournement de paire (2 cartes)
- Que la paire soit valide ou non, on passe à l'équipe suivante
- Rotation : Équipe 1 → Équipe 2 → Équipe 3 → ... → Équipe 1
- Chaque équipe accumule ses propres paires trouvées
- Points par paire attribués à l'équipe qui la trouve

#### Mode TANT QUE JE GAGNE
- Multi-équipes avec garde de la main
- Une équipe continue de jouer tant qu'elle trouve des paires valides
- Dès qu'une paire n'est pas valide (non-match), on passe à l'équipe suivante
- Chaque équipe accumule ses propres paires trouvées
- Mode "hot potato" : celui qui se trompe perd la main

---

## Bonnes pratiques

1. **Sauvegardes regulieres** : Effectuer une sauvegarde avant chaque session de jeu
2. **Historique intact** : Ne pas reinitialiser l'historique sauf si necessaire
3. **Restauration** : Toujours verifier le contenu de l'archive avant restauration
4. **Scores** : Utiliser POINTS_TARGET adapte au type de question

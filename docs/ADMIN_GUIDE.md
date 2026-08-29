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
- [Mise a jour firmware OTA des buzzers](#mise-a-jour-firmware-ota-des-buzzers-v310)
- [Configuration WiFi des buzzers](#configuration-wifi-des-buzzers-v310)
- [Jeu Memory - Modes multi-equipes](#jeu-memory---modes-multi-equipes)
- [Filtres categories dans GamePage](#filtres-categories-dans-gamepage-v370)
- [Double QR code enrollment TV](#double-qr-code-enrollment-tv-v370)
- [Catégories personnalisées](#catégories-personnalisées-v570)
- [Palette de 16 couleurs d'équipes](#palette-de-16-couleurs-déquipes-v5725)
- [Générateur de questions via IA](#générateur-de-questions-via-ia-v600)

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

### Persistance des metadata de quiz (v6.0.x, #141)

**Nouveau depuis v6.0.x** : les métadonnées du quiz (nom, thème, notes, publics, difficultés, objectifs, langue, champs masqués TV) sont automatiquement sauvegardées dans `data/config/game_state.json` et **survivent au redémarrage du serveur**.

Au redémarrage :
1. Les métadonnées sont restaurées — le quiz retrouve son nom, thème, notes, etc.
2. Le nouvel état du jeu (phase, question en cours, scores) commence vide (comportement inchangé)
3. Les réglages TV masqués (ex: réponses cachées) sont **oubliés entre deux sessions** — c'est intentionnel pour éviter les fuites accidentelles

**Impact utilisateur** : plus besoin de re-remplir la fiche "Quiz" après un redémarrage. Les joueurs qui reviennent jouent sur la même partie avec les mêmes métadonnées de contexte.

---

### Migration configuration système / jeu (v6.0.x, #150)

**Changement structurel** : les réglages de jeu (délai par défaut, effet néon) sont désormais sauvegardés dans un fichier séparé (`data/config/game-config.json`), indépendant de la configuration système (clés API, identifiants WiFi, etc.).

**Migration automatique au démarrage** (une seule fois) :

- Si vous aviez un `config.json` au format ancien (sections `game` et `neon_effect` présentes)
- Au premier démarrage de v6.0.x, ces sections sont **extraites** vers `game-config.json`
- Aucune action de votre part — tout est transparent
- La migration est **idempotente** : démarrage suivant = pas de re-migration

**Impact utilisateur** : aucun ! Les réglages de jeu (délai, effet néon) restent disponibles au même endroit dans l'interface admin.

**Avantage interne** : les réglages de jeu sont maintenant inclus dans les sauvegardes/restaurations de partie (via `/backup-select` ou `/fs-backup`), séparé des clés API qui ne doivent jamais être sauvegardées.

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
GET /backup-select?questions=true&teams=true&bumpers=true&history=true&medias=true&ambiance=true
```

Retourne un fichier TAR contenant uniquement les elements selectionnes.

**Parametres disponibles** :
- `questions` — questions du quiz
- `teams` — configuration des equipes
- `bumpers` — configuration des joueurs
- `history` — historique des parties
- `medias` — images de fond / ressources
- `ambiance` — configuration ambiance (fonds d'ecran, musique, effets) — independant de `history`

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
- Detecte les fichiers `medias/`
- Detecte les fichiers `ambiance/` (configuration ambiance — independante de `history`)

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
| Configuration Ambiance | Reinitialise les reglages d'ambiance (fonds d'ecran, musique, effets) — independant de l'Historique |

### Endpoint API

```
POST /reset-select?questions=true&teams=true&bumpers=true&history=true&medias=true&ambiance=true
```

Reinitialise uniquement les elements selectionnes.

**Parametres disponibles** : memes que `/backup-select` ci-dessus.

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
| `PLAYER` | Points au joueur qui repond | Questions SPEEDY |
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
   - Aller sur la page Joueurs (`/admin/teams`)
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

**Comportement après suppression** : Le joueur est automatiquement déconnecté et redirigé vers la page d'inscription avec un bandeau explicite « Joueur supprimé ». Il peut immédiatement se réinscrire avec le même pseudo (v5.9.0+, #120).

#### Réinitialisation d'une partie (NEW_GAME)

Lors d'une réinitialisation du jeu (`NEW_GAME`), tous les joueurs virtuels actuellement inscrits sont purgés du roster, même s'ils n'ont pas d'équipe assignée. Ils sont redirigés avec le motif « Partie réinitialisée » et restent libres de se réinscrire après l'ouverture d'une nouvelle phase d'inscriptions.

#### Retrait d'un VJoueur — Réinscription vs. Suppression totale (v5.9.3+, étendu en v5.10.x #134)

Chaque fiche VJoueur (y compris dans la liste des équipiers) affiche un bouton **×** qui ouvre un dialogue de confirmation. Ce dialogue propose deux actions selon la situation du joueur :

**Le bouton × propose par défaut** :
- Si le joueur a une **équipe** → **Réinscription** (conserve score + équipe)
- Si le joueur est **sans équipe** → **Suppression totale** (libère la place)

**L'autre action reste toujours accessible** dans le même dialogue (juste en dessous, pas besoin de fermer et de cliquer ailleurs).

**Détail des deux actions** :

| Action | Joueur déconnecté | Joueur connecté (v5.10.x #134) | Score | Équipe | Reprise possible |
|---|---|---|---|---|---|
| **Réinscription** | Accorde autorisation ~5 min | Libère place immédiatement + invalide session | Conservé | Conservée | Oui, même pseudo autorisé |
| **Suppression totale** | Supprime définitivement | Supprime définitivement | Perdu | Perdue | Oui (nouvel enregistrement à zéro si inscriptions ouvertes) |

**Réinscription** :
- Accorde une autorisation **valide ~5 minutes** au joueur pour reprendre sa place
- Valable que le joueur soit déconnecté (perdu son ID local) ou connecté (session active)
- Pour un joueur connecté : place libérée et session invalidée immédiatement ; le joueur peut se réinscrire avec le même pseudo dans la fenêtre d'autorisation
- Pour un joueur déconnecté : autorisation temporaire accordée ; il dispose de cette fenêtre pour se reconnecter sans son ancien ID (même s'il l'a perdu)
- Son score et son équipe sont **intégralement conservés** lors de la reprise
- L'autorisation est à **usage unique** (une seule reconnexion/reprise réussit)
- Utilité : joueur bloqué temporairement (changement d'appareil, cache vidé, connexion perte), ou jugeur connecté dont on veut faire la place mais avec possibilité de reprise

**Suppression totale** :
- Libère complètement la place — pseudonyme, score, assignation équipe
- Pour un joueur connecté : suppression immédiate, session fermée, redirection avec motif « Joueur supprimé »
- Pour un joueur déconnecté : suppression du compte, inaccessible
- **⚠️ Piège important** : cette suppression ne libère vraiment la place **que si les inscriptions sont ouvertes**
- Si les inscriptions sont **fermées**, le joueur ne pourra pas revenir du tout (ni se réinscrire, ni reprendre sa place)
- L'avertissement « Inscriptions fermées : {nom} ne pourra pas revenir après une suppression totale » s'affiche dans le dialogue pour vous rappeler ce piège

**Conseil d'utilisation** :
- **Joueur bloqué après changement d'appareil ou perte de connexion ?** Cliquez sur le bouton ×, sélectionnez Réinscription — il conservera son score et pourra revenir dans les 5 minutes avec le même pseudo.
- **Joueur connecté dont vous voulez faire la place pour un autre, mais lui conserver son score ?** Cliquez sur le bouton ×, sélectionnez Réinscription (c'est l'action par défaut si assigné à une équipe). Sa place est libérée immédiatement, mais il peut revenir avec son score intégré.
- **Joueur qui abandonne la partie définitivement ?** Cliquez sur le bouton ×, vérifiez d'abord que les inscriptions sont ouvertes, puis sélectionnez Suppression totale. S'ils veulent revenir plus tard, elles devront être réouvertes et ils s'enregistreront à zéro.
- **Joueur sans équipe dont vous ne voulez plus ?** L'action par défaut est Suppression totale — c'est généralement ce qu'il faut pour libérer la place rapidement.

#### Compteurs

Dans la section "Inscriptions" :
- **Places max** : Nombre maximum de joueurs virtuels (configurable)
- **Inscrits** : X/Y avec distinction 🎮 physiques / 📱 virtuels

### Interface QCM tactile multicolore (v2.53.0)

#### Présentation

Les VJoueurs disposent d'une interface spéciale pour répondre aux questions QCM : ils peuvent toucher directement une des 4 couleurs (Rouge, Vert, Jaune, Bleu) sur leur écran au lieu d'utiliser un buzzer physique.

#### Identification visuelle

Les VJoueurs sont identifiés par un **badge multicolore** (4 quartiers colorés) affiché dans leur carte joueur sur l'interface admin :
- Ce badge remplace le point de couleur unique des buzzers physiques
- Il apparaît dans la colonne "Joueurs non assignés" et dans les cartes d'équipe

#### Comportement en QCM

Quand une question QCM est active (phase STARTED) :

**Pour le VJoueur** :
- 4 gros boutons colorés apparaissent sur son écran (`/player`)
- Chaque bouton correspond à une réponse (Rouge, Vert, Jaune, Bleu)
- Un seul clic suffit pour répondre (pas besoin de buzzer d'abord)
- Feedback haptique + overlay vert "BUZZÉ !"

**Pour les buzzers physiques de l'équipe** :
- Si l'équipe possède un VJoueur actif, les buzzers physiques sont **automatiquement invalidés** pour cette question QCM
- Les autres joueurs de l'équipe ne peuvent plus buzzer avec leur buzzer physique
- Sur l'interface admin, les buzzers physiques apparaissent **grisés** avec l'indication "VJoueur actif"

#### Pourquoi cette invalidation

Cette limitation évite les conflits entre :
- Un joueur qui buzz avec son buzzer physique (sans couleur)
- Un VJoueur qui répond avec une couleur précise

Seul le VJoueur peut répondre pour l'équipe en mode QCM, garantissant une réponse colorée valide.

#### Comportement normal

L'invalidation des buzzers physiques est **limitée au mode QCM** :
- Pour les questions SPEEDY, les buzzers physiques fonctionnent normalement
- Le VJoueur peut aussi buzzer sur les questions SPEEDY via son bouton "BUZZ !"
- Pour les questions MEMORY, le VJoueur et les buzzers physiques sont tous deux bloqués (contrôle admin uniquement)

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

1. Ouvrir la page Configuration (`/admin/settings`)
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

## Mise a jour firmware OTA des buzzers (v3.1.0)

### Presentation

La fonctionnalite OTA (Over-The-Air) permet de mettre a jour le firmware des buzzers BuzzClick directement depuis l'interface admin, sans cable USB. Le serveur stocke un firmware de reference et peut le deployer sur tous les buzzers connectes en WiFi.

### Acces a la gestion firmware

1. Ouvrir la page Configuration (`/admin/config`)
2. Section "Gestion Firmware" en bas de la page
3. Le serveur affiche la version de reference actuellement stockee

### Etape 1 : Uploader le firmware de reference

1. Dans la section "Gestion Firmware", cliquer sur "Choisir un fichier"
2. Selectionner le fichier `.bin` (ex : `buzzclick-v3.1.0.bin`)
3. Cliquer sur "Uploader"
4. Le serveur valide le fichier (taille 200 KB - 2 MB, extension .bin)
5. La version de reference s'affiche apres upload

**Nommage recommande** : `buzzclick-vX.Y.Z.bin` — la version est extraite automatiquement du nom de fichier.

### Etape 2 : Identifier les buzzers obsoletes

Sur la page Jeu (`/admin/game`), les buzzers dont le firmware est plus ancien que la reference affichent un badge rouge :

```
fw: 3.0.8   ← rouge = obsolete, clic pour mettre a jour
fw: 3.1.0   ← gris = a jour
```

### Etape 3a : Mettre a jour un buzzer individuel

1. Cliquer sur le badge rouge du buzzer
2. Une modal s'ouvre avec la version actuelle et la version de reference
3. Cliquer sur "Lancer la mise a jour OTA"
4. Le statut s'affiche en direct : `downloading → flashing → done`
5. Le buzzer redemarrera automatiquement apres le flash

### Etape 3b : Mettre a jour tous les buzzers obsoletes

1. Dans la section "Gestion Firmware" de ConfigPage
2. Cliquer sur "Mettre a jour tous"
3. Confirmer la mise a jour en masse
4. L'interface affiche le nombre de buzzers declenches / ignores

**Important** : Seuls les buzzers connectes via WebSocket peuvent recevoir l'OTA. Les buzzers TCP (ancien firmware) ne sont pas eligibles.

### Duree et indicateurs

| Phase | Duree estimee | Indicateur |
|-------|---------------|------------|
| Telechargement | 10-30 secondes | `downloading` (pourcentage) |
| Flash | 10-20 secondes | `flashing` |
| Redemarrage | 5-10 secondes | Deconnexion puis reconnexion |
| Succes | - | Badge `fw: X.Y.Z` gris |
| Echec | - | Badge `error` rouge |

### Flash firmware via USB (v3.1.2)

En alternative a l'OTA WiFi, vous pouvez flasher le firmware via cable USB depuis la modale USB unifiee. Cette modale regroupe la configuration WiFi AT et le flash firmware dans une seule interface.

**Acces depuis ConfigPage** :
1. Aller dans la section "Gestion Firmware" de ConfigPage (`/admin/config`)
2. Cliquer sur "Flash via USB" (bouton au meme niveau que "Uploader le firmware" et "Mettre a jour les buzzers")
3. La modale USB s'ouvre — selectionner le port serie du buzzer
4. Aller dans l'onglet "Flash Firmware"
5. Cliquer sur "Flash"
6. La barre de progression et les logs s'affichent en temps reel

**Acces depuis l'icone USB** :
1. Cliquer sur l'icone USB dans l'interface
2. Selectionner le port serie du buzzer
3. L'onglet "Config WiFi" ou "Flash Firmware" selon l'operation souhaitee

**Avantage v3.1.2** : Le port USB est selectionne une seule fois dans la modale, quel que soit l'onglet utilise (config AT ou flash). Plus besoin de gerer deux points d'entree distincts.

**Prerequis** : Chrome ou Edge 89+, navigateur sur localhost, firmware uploade au prealable sur le serveur.

### Rollback automatique

Le firmware ESP32-C3 dispose d'un mecanisme de rollback automatique :
- Si le nouveau firmware ne demarre pas correctement, l'ESP32 restaure automatiquement l'ancienne version
- En cas d'erreur OTA, le buzzer reste operationnel avec son ancien firmware

---

## Configuration WiFi des buzzers (v3.1.0)

### Presentation

La fonctionnalite de diffusion WiFi permet d'envoyer la configuration reseau (SSID, mot de passe, IP serveur) a tous les buzzers connectes simultanement, sans repasser par la configuration USB individuelle.

### Acces a la configuration WiFi

1. Ouvrir la page Configuration (`/admin/config`)
2. Section "Configuration WiFi par defaut"
3. Les champs affichent la configuration actuellement sauvegardee

### Parametres disponibles

| Champ | Description |
|-------|-------------|
| **SSID** | Nom du reseau WiFi principal |
| **Mot de passe** | Mot de passe du reseau WiFi principal |
| **SSID 2** | Nom du reseau WiFi de secours (optionnel) |
| **Mot de passe 2** | Mot de passe du reseau WiFi de secours (optionnel) |
| **IP du serveur** | Adresse IP du serveur BuzzControl |
| **Port** | Port du serveur (defaut : 80) |

### Diffuser la configuration

1. Remplir les champs de configuration WiFi
2. Cliquer sur "Enregistrer la configuration" pour sauvegarder
3. Cliquer sur "Diffuser config WiFi" pour envoyer aux buzzers connectes

**Comportement sur le buzzer** :
- Si la configuration a change : sauvegarde en NVS + redemarrage automatique dans 3 secondes
- Si la configuration est identique : aucune action (pas de redemarrage inutile)

### Auto-sync a la connexion

Chaque fois qu'un buzzer se connecte via WebSocket, le serveur lui envoie automatiquement la configuration WiFi de reference. Cela garantit que les buzzers sont toujours synchronises avec la configuration serveur apres un redemarrage.

### Reseau de secours (SSID2)

Le champ SSID2/Mot de passe 2 permet de configurer un reseau WiFi de secours :
- Le buzzer tentera de se connecter au SSID principal en priorite
- En cas d'echec, il basculera automatiquement sur le SSID2
- Utile pour les installations avec deux points d'acces WiFi

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
| GET | `/api/firmware/buzzclick/version` | Info firmware de reference (v3.1.0) |
| GET | `/api/firmware/buzzclick/latest.bin` | Telechargement firmware (v3.1.0) |
| POST | `/api/firmware/buzzclick/upload` | Upload firmware .bin (v3.1.0) |
| POST | `/api/buzzer/{mac}/update` | OTA individuelle par MAC (v3.1.0) |
| POST | `/api/buzzer/update-all` | OTA en masse buzzers obsoletes (v3.1.0) |
| POST | `/api/buzzer/wifi-config` | Broadcast config WiFi aux buzzers (v3.1.0) |

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

---

## Filtres categories dans GamePage (v3.7.0)

### Presentation

Sur la page de jeu (`/admin/game`), la barre d'equilibre des categories affiche les categories disponibles dans votre banque de questions. Depuis la v3.7.0, ces categories sont **cliquables** pour filtrer les questions affichees.

### Utilisation

1. Dans `GamePage`, reperer la barre d'equilibre des categories en haut
2. Cliquer sur une categorie pour l'activer comme filtre — seules les questions de cette categorie s'affichent
3. Cliquer sur plusieurs categories pour cumuler les filtres (multi-selection)
4. Cliquer sur une categorie active pour la desactiver

### Indicateurs visuels

- **Categorie inactive** : apparence normale
- **Categorie active (filtre)** : surlignage visuel (badge colore)
- **Aucun filtre actif** : toutes les questions sont affichees (comportement par defaut)

### Cas d'usage

- Preparer une manche thematique (ex : uniquement les questions Geographie)
- Equilibrer le jeu en alternant les categories manuellement
- Masquer temporairement des categories deja jouees

---

## Catégories personnalisées (v5.7.0)

### Présentation

Depuis la v5.7.0, vous pouvez créer vos propres catégories de questions en déposant des images dans le dossier `data/files/categories/`. Ces catégories apparaissent dans le sélecteur de l'éditeur de questions et sur les badges des QuestionCard.

### Workflow

1. Préparer une image (PNG, JPG, JPEG ou WEBP) représentant la catégorie
2. La nommer de manière descriptive : ex. `Sport Extreme.png`
3. La déposer dans `data/files/categories/` (créer le dossier si absent)
4. La catégorie apparaît automatiquement dans l'interface (pas de redémarrage requis)

### Convention de nommage

Le nom du fichier devient la clé de la catégorie :

| Fichier | Clé générée |
|---------|-------------|
| `Sport Extreme.png` | `SPORT_EXTREME` |
| `Cinéma-Classique.jpg` | `CINÉMA-CLASSIQUE` |
| `sciences.webp` | `SCIENCES` |

La transformation appliquée : espaces → `_`, tout en majuscules, extension supprimée.

### Catégories hardcodées

Les catégories standard restent disponibles même sans fichier image :
`GEOGRAPHY`, `ENTERTAINMENT`, `HISTORY`, `SCIENCE`, `SPORTS`, `ARTS`, `CULTURE`, `OTHER`

### Endpoint API

```
GET /api/categories
```

Retourne la liste fusionnée (hardcodées + custom) avec `"custom": true` pour les catégories personnalisées.

### Backup / Restore

Les catégories personnalisées sont incluses dans le backup/restore via le flag `medias` :
```
GET /backup-select?medias=true
POST /restore  (archive contenant files/categories/)
```

---

## Double QR code enrollment TV (v3.7.0)

### Presentation

Lors de la phase d'inscription VJoueur, l'affichage TV (`/tv`) montre desormais **deux QR codes cote a cote** pour faciliter la connexion des joueurs depuis leur smartphone.

### Les deux QR codes

| QR code | Contenu | Usage |
|---------|---------|-------|
| **WiFi** | SSID + mot de passe du reseau | Le joueur se connecte d'abord au bon reseau WiFi |
| **VJoueur** | URL d'inscription (`http://[host]/`) | Le joueur scanne pour rejoindre la partie |

### Workflow recommande

1. Lancer les inscriptions depuis `/admin/teams` (bouton "Lancer Inscriptions")
2. L'affichage TV passe en vue enrollment avec les deux QR codes
3. Les joueurs commencent par scanner le **QR WiFi** pour rejoindre le bon reseau
4. Puis ils scannent le **QR VJoueur** pour s'inscrire avec leur pseudo
5. Une fois tous inscrits, fermer les inscriptions depuis `/admin/teams`

### Note

Le QR code WiFi utilise la configuration WiFi enregistree dans `config.json` (SSID principal). Si aucun SSID n'est configure, seul le QR code VJoueur s'affiche.

---

## Palette de 16 couleurs d'équipes (v5.7.25, #113)

### Présentation

Depuis v5.7.25, la palette d'équipe passe de 8 à 16 couleurs pour une meilleure distinctivité. Les 16 couleurs combinent 8 teintes déclinées en deux tons : **ton vif** (saturation 100%, luminosité ~55%) et **ton profond** (saturation 100%, luminosité ~35%).

### Les 16 couleurs de palette

| Rang | Nom (Vif) | Nom (Profond) | Teinte |
|------|-----------|---------------|--------|
| 1/9 | Rouge | Grenat | 0° (Rouge) |
| 2/10 | Orange | Ambre | 28° |
| 3/11 | Jaune | Or | 50° |
| 4/12 | Vert | Émeraude | 135° |
| 5/13 | Cyan | Turquoise | 185° |
| 6/14 | Bleu | Marine | 222° |
| 7/15 | Violet | Indigo | 275° |
| 8/16 | Rose | Magenta | 325° |

Voir `contracts/models.md` § « Palette d'équipes » pour les valeurs RGB exactes.

### Attribution automatique des couleurs

La création d'une nouvelle équipe attribue automatiquement :
1. **Rangs 1-8** : tons vifs (si disponibles) — couleurs claires, visibles sur toutes les surfaces
2. **Rangs 9-16** : tons profonds (si rangs 1-8 épuisés) — couleurs sombres, moins consommateur d'énergie

### Au-delà de 16 équipes

Une fois les 16 couleurs épuisées, l'attribution **recycle au rang 1** (Rouge vif). Cet exemple montre un comportement de deux équipes "Les Rouges" (rang 1) et "Les Rouges 2" (rang 17 → rang 1 après recyclage) avec la même couleur.

**Note** : en pratique, les quiz dépassent rarement 16 équipes. Pour les cas hors norme, la page Équipes affiche explicitement le marquage « déjà prise » sur les couleurs en conflit.

### Sélecteur de couleurs — page Équipes

#### Grille 8×2

La sélection manuelle de couleur s'effectue via une grille de 16 pastilles :
- **2 rangées** : rang 1-8 (première rangée, tons vifs), rang 9-16 (deuxième rangée, tons profonds)
- **8 colonnes** : une teinte par colonne (rouge, orange, jaune, vert, cyan, bleu, violet, rose)

#### Marquage « déjà prise »

Une couleur est marquée « **déjà prise** » avec des hachures si elle est **déjà assignée à une autre équipe**. Cette équipe peut néanmoins réassigner la couleur d'une autre équipe (décision volontaire). La pastille reste cliquable.

#### Navigation clavier

Focus visible sur chaque pastille via `:focus-visible` (cadre/bordure explicite). Tabulation entre pastilles avec direction Left/Right/Up/Down supportée.

### Persistance et LED buzzer

Le champ `Team.COLOR_NAME` (ex: `"rouge"`, `"bleu-profond"`) est écrit à chaque:
- **Attribution automatique** : création d'équipe
- **Changement manuel** : sélection via grille

**Impact LED** : le buzzer physique d'une équipe affiche désormais la **couleur exacte** (via `COLOR_NAME`) au lieu d'une approximation par teinte. Rétrocompatibilité garantie : équipes antérieures à v5.7.25 restent jouables (fallback par teinte).

### Invariant d'affichage

Les 16 couleurs de palette sont **invariantes par le filtre CSS `boostTeamColor()`** (déjà à 100% de saturation, luminosité déjà dans [35%, 65%]). Ainsi :
- RGB stocké en base = RGB affiché à l'écran
- Aucune distorsion ou arrondissement lors du rendu

**Important** : ne jamais ajouter de nouvelle couleur à la palette sans vérifier cette propriété d'invariance.

---

## Générateur de questions via IA (v6.0.0)

### Présentation

Le **Générateur de questions via IA** permet de créer automatiquement des questions pour compléter votre quiz en envoyant une demande à l'API Claude (Anthropic). Contrairement à la création manuelle, vous définissez les paramètres (population cible, difficulté, thème, catégories) et l'IA génère les questions directement.

**Important** : Cette fonctionnalité nécessite une **clé API Claude personnelle**. Vous devez la configurer dans les Paramètres avant de pouvoir utiliser le générateur. Les coûts d'utilisation de l'API sont **facturés sur votre compte Anthropic** — ce n'est pas inclus dans BuzzControl.

### Configuration préalable : Choisir un provider et configurer la clé API

Deux providers LLM sont disponibles. Choisissez celui qui correspond à vos besoins :

| Provider | Coût | Format clé | Configuration | Vitesse | Notes |
|----------|------|-----------|----------------|---------|-------|
| **Claude (Anthropic)** | Payant (BYOK) | `sk-ant-...` | Console Anthropic | 1–3 min (200q) | Qualité et vitesse optimales |
| **Groq** | **Gratuit** | `gsk_...` | Console Groq | ~10 min (200q) | Gratuit, plus lent, quota journalier limité |

#### Accès à la configuration

1. Ouvrir la page **Configuration** (`/settings` ou lien "⚙️ Paramètres" en haut à droite)
2. Aller à la section **IA** (nouvelle section en bas de la page)
3. **Sélectionner le Provider** : Cliquer sur le bouton "Claude (Anthropic)" ou "Groq" pour basculer entre les deux. Seule la carte du fournisseur sélectionné s'affiche — l'autre est masquée.
4. **Remplir la clé API** du provider sélectionné (voir ci-dessous)

**Popup d'aide "?" sur la carte du provider** : Un bouton "?" en haut à droite de la carte vous guide pas à pas pour créer un compte et générer une clé auprès du fournisseur sélectionné. Valable uniquement pour les nouveaux comptes ou si vous avez oublié comment accéder à votre clé.

#### Option 1 : Claude (Payant, Anthropic)

**Format attendu** : `sk-ant-...` (commence par `sk-ant-`, suivie de caractères alphanumériques)

**Où trouver votre clé ?**
- Console Anthropic : https://console.anthropic.com/account/keys
- Créer une nouvelle clé si nécessaire (remplacer une clé exposée publiquement)
- Nécessite une **méthode de paiement** (carte de crédit, débit mensuel)

**Modèle utilisé** : Claude Opus 5 (le plus capable, recommandé pour la qualité)

#### Option 2 : Groq (Gratuit, Tier gratuit)

**Format attendu** : `gsk_...` (commence par `gsk_`, suivie de caractères)

**Où trouver votre clé ?**
- Console Groq : https://console.groq.com/keys
- Créer un compte **gratuit** (aucune CB requise)
- Nouvelle clé générée automatiquement

**Limitations du tier gratuit** :
- **Quota journalier** : 1 000 requêtes, 200 000 tokens/jour
- **Débit** : 8 000 tokens/minute
- **Modèle** : `openai/gpt-oss-120b` (open-source, qualité variable)

**Temps de génération** :
- 200 questions → ~10 minutes (contre 1–3 min avec Claude)
- Les questions **apparaissent au fur et à mesure** dans QuestionsPage (non pas tout à la fin)
- Vous pouvez **arrêter la génération entre les lots** et conserver les questions générées jusqu'à présent

**Important** : La qualité du français de `gpt-oss-120b` n'est pas garantie. Après génération, **relisez et corrigez** les questions avant de les utiliser en partie publique.

#### Validation et Sauvegarde à l'enregistrement

Quand vous cliquez sur "Enregistrer" (après avoir changé la clé ou laissé le champ vide si une clé est stockée), le serveur **valide la clé auprès du fournisseur réel** en effectuant un appel authentifié. Trois cas :

**✅ Clé valide** :
- La clé a été vérifiée auprès du fournisseur
- Badge affiché : **"✅ Clé vérifiée"** (vert)
- Clé sauvegardée avec statut `verified: true`
- Toast de confirmation : "Clé vérifiée auprès de Claude et enregistrée."

**⚠️ Clé refusée** (le fournisseur rejette la clé) :
- Dialogue modal apparaît : **"Claude a refusé cette clé"** ou **"Groq a refusé cette clé"**
- Message : *"La clé a bien été transmise, mais le fournisseur ne la reconnaît pas. Vérifiez que vous l'avez copiée en entier et qu'elle n'a pas été révoquée."*
- Deux options :
  - **[Corriger]** — rejette la sauvegarde, le champ reste en bordure rouge. Vous pouvez modifier la clé et réessayer.
  - **[Enregistrer quand même]** — enregistre la clé sans vérification (rare, utile si la clé n'est pas encore active côté fournisseur). Badge : **"⚠️ Clé non vérifiée"** (orange). Toast : "Clé enregistrée sans vérification — testez-la plus tard."

**⚠️ Fournisseur injoignable** (réseau, timeout, ou autre erreur) :
- Dialogue modal : **"Impossible de joindre Claude"** ou **"Impossible de joindre Groq"**
- Message : *"La clé n'a pas pu être vérifiée — elle n'est ni confirmée ni refusée. Si ce serveur est hors ligne, vous pouvez l'enregistrer telle quelle et vérifier plus tard."*
- Trois options :
  - **[Réessayer]** — relance la validation après quelques secondes (utile si dérangement temporaire)
  - **[Corriger]** — rejette la sauvegarde comme ci-dessus
  - **[Enregistrer quand même]** — enregistre la clé. Badge : **"⚠️ Clé non vérifiée"** (orange). Toast : "Clé enregistrée sans vérification — testez-la plus tard."

**Champ vide + clé stockée** :
- Si vous laissez le champ vide mais qu'une clé est déjà enregistrée pour ce fournisseur, cliquer "Enregistrer" valide la clé stockée (utile pour mettre à jour le badge de vérification après un rechargement de page, ou pour vérifier une clé issue d'une variable d'environnement PROD).

**Affichage des clés** :
- Les clés valides sont **toujours masquées** en affichage (jamais exposées en clair dans la page ou les logs), qu'elles soient vérifiées ou non.

### Configurer les clés API IA en production (recommandé)

> **Contexte** : `server-go/config.json` est un fichier de configuration classique, potentiellement
> suivi par git (comme dans ce dépôt) — une clé saisie via la page Paramètres y est écrite en
> clair sur disque. Sur un poste personnel ou un dépôt privé, ce n'est pas un problème. Sur un
> déploiement dont le dépôt est **public** ou partagé, une clé écrite dans `config.json` finit
> tôt ou tard commitée et exposée (c'est exactement ce qui s'est produit lors du développement de
> cette fonctionnalité, cf. `contracts/CHANGELOG.md` [20260807]).

Pour un déploiement en production, configurez la clé via **variable d'environnement** plutôt que
via la page Paramètres — elle n'est alors **jamais écrite sur disque**.

| Provider | Variable d'environnement |
|---|---|
| Claude (Anthropic) | `BUZZCONTROL_ANTHROPIC_API_KEY` |
| Groq | `BUZZCONTROL_GROQ_API_KEY` |

**Exemple (Linux/systemd)** :
```ini
# /etc/systemd/system/buzzcontrol.service
[Service]
Environment="BUZZCONTROL_GROQ_API_KEY=gsk_votre_cle_ici"
ExecStart=/opt/buzzcontrol/server
```

**Exemple (lancement manuel)** :
```bash
BUZZCONTROL_GROQ_API_KEY=gsk_votre_cle_ici ./server
```

**Règles de priorité** :
- La variable d'environnement, si définie, est **toujours prioritaire** sur la valeur de
  `config.json` — y compris si une clé (différente ou identique) est déjà enregistrée dans le
  fichier.
- Si la variable n'est **pas** définie, le comportement est inchangé : la clé enregistrée via la
  page Paramètres (`config.json`) continue de fonctionner normalement — pratique pour un usage
  local/dev qui accepte ce risque en connaissance de cause.
- La page Paramètres affiche **"clé configurée"** dès qu'une clé est disponible **par l'une ou
  l'autre voie** — vous n'avez pas besoin de laisser un champ vide dans l'UI pour que la variable
  d'environnement soit prise en compte, mais dans ce cas laissez-le vide justement pour ne rien
  écrire sur disque.
- La clé fournie par variable d'environnement n'est **jamais** écrite dans `config.json` par le
  serveur, quelle que soit l'action effectuée par la suite dans la page Paramètres (modifier un
  autre réglage IA, changer de provider, etc.) — les deux mécanismes sont indépendants.

**Recommandation** : sur un déploiement PROD, laissez `anthropic_api_key`/`groq_api_key` **vides**
dans `config.json` et utilisez exclusivement les variables d'environnement ci-dessus.

### Utilisation : Générer des questions (Workflow asynchrone)

#### Accès au formulaire

1. Aller à la page **Questions** (`/admin/questions` ou lien "❓ Questions")
2. Localiser le bouton **"✨ Générer via IA"** dans le bloc "Nouvelle Question" (carte grise à gauche)
3. Cliquer sur le bouton → ouverture d'une modale de génération

**Note** : Si le bouton est grisé, cela signifie que **aucune clé API n'est configurée**. Configurez-la d'abord dans les Paramètres (voir section ci-dessus).

**Important (v6.1.0+)** : **Une seule génération à la fois** est possible. Si une génération est en cours, le bouton affiche un message `"Génération en cours..."` et est désactivé. Une autre tentative renvoie une erreur `409 Génération déjà en cours`.

#### Formulaire de génération

La modale affiche deux blocs :

**Bloc 1 — Paramètres du Quiz (pré-remplis, informatif)** :
- Thème global de votre quiz
- Population cible
- Langue
- Difficulté globale

Ces valeurs viennent de la section "Quiz" en haut de QuestionsPage. Elles sont **affichées à titre informatif** et ne sont **pas modifiables** dans le formulaire de génération.

**Bloc 2 — Cette génération (éditable)** :
- **Difficulté** : Cocher une ou plusieurs cases (Facile, Moyen, Difficile, Expert). Vous pouvez mixer les niveaux pour un même lot — l'IA répartira les questions équitablement.
- **Objectifs / Consignes** (optionnel) : Texte libre décrivant le contexte (ex: "révision scolaire", "ambiance conviviale") ou des contraintes spéciales (ex: "éviter les questions sur la politique").
- **Catégories cibles** : Sélectionner une ou plusieurs catégories existantes. L'IA génère des questions **uniquement** dans ces catégories (elle ne crée jamais de nouvelle catégorie).
- **Volume** : 
  - **Mode "Nombre"** : indiquer combien de questions générer (ex: 20)
  - **Mode "Durée"** : indiquer la durée cible de la partie en minutes (ex: 30 min) — l'IA calcule le nombre de questions et le temps de réponse pour approcher cette durée
- **Répartition par type** : Quatre sliders (SPEEDY, QCM, MEMORY, MEMOTION) fixant le pourcentage de chaque type. Les sliders se **rééquilibrent automatiquement** quand vous en bougez un. Vous pouvez **désactiver complètement** un type en cliquant sur le toggle (remis à 0%, exclu du rééquilibrage).

#### Lancer la génération

1. Remplir les champs du bloc "Cette génération"
2. Cliquer sur le bouton **"Générer"** en bas à droite de la modale
3. La génération démarre **en arrière-plan** :
   - La modale **ne se ferme pas** et affiche une **barre de progression**
   - Les questions apparaissent **par lots** au fur et à mesure dans QuestionsPage (refresh temps réel)
   - Le temps total dépend du provider (Claude 1–3 min, Groq ~10 min pour 200 questions)

**Pendant la génération** :
- **Modale non-bloquante** : Vous pouvez continuer à éditer d'autres questions, naviguer, etc.
- **Progression visible** : Barre montrant le nombre de lots générés (ex: "Lot 3/5")
- **Questions en direct** : Chaque lot de questions terminé s'affiche immédiatement dans QuestionsPage sans attendre la fin

#### Résultat et gestion des erreurs

**✅ Succès complet** :
- Modale affiche "Génération terminée"
- Bouton "Fermer" pour quitter
- Les **nouvelles questions sont visibles dans QuestionsPage**
- Vous pouvez les **éditer, supprimer ou télécharger des images** comme des questions manuelles

**⚠️ Succès partiel** (certains lots échouent, d'autres réussissent) :
- Les questions **déjà générées sont conservées**
- Message indiquant combien de questions ont été créées et combien d'erreurs rencontrées
- Possibilité de **relancer une génération** pour compléter (sans duplication)

**❌ Erreur (avant ou pendant la génération)** :
- Message d'erreur explicite :
  - `Clé API invalide` — vérifiez le format dans Paramètres
  - `Clé API refusée` — la clé a été révoquée ou n'est pas reconnue par le provider
  - `Quota dépassé` — quota journalier épuisé (surtout Groq : 1 000 requêtes/jour)
  - `Erreur réseau/provider` — réponse HTTP 500, timeout, ou erreur structurelle du provider
- Bouton **"Réessayer"** conservant votre saisie — corrigez la clé (si besoin) et relancez
- **Panneau "Détail technique"** (repliable, nouveau v6.1.1) : Message d'erreur réel du provider
  - Affiché en complément du message générique
  - Exemple Groq #142 (avant correction) : `"discriminator: multiple candidate properties CATEGORY, DIFFICULTY, TYPE [discriminator_multiple_candidates]"` — permet un diagnostic immédiat
  - Masque les clés API par sécurité (filtre automatique)
  - Admin-only — jamais visible en TV ou interface VPlayer

**Aucune question n'a été créée en cas d'erreur initiale** — la génération est atomique jusqu'au premier lot réussi.

#### Après une génération terminée : relancer

Après que une génération soit **terminée avec succès (DONE)** ou **arrêtée (CANCELLED)** :

1. La modale affiche un **panneau résumé** :
   - DONE : nombre de questions créées, aucun message d'erreur
   - CANCELLED : nombre de questions créées avant l'arrêt

2. Deux boutons sont présents :
   - **"Fermer"** (variant secondaire) → Ferme la modale, conserve les questions générées
   - **"Nouvelle génération"** (variant primary, nouveau v6.1.1) → Réinitialise le formulaire à l'intérieur de la modale (Bloc 2 "Cette génération" — catégories, volume, distribution reprennent les dernières valeurs) et remet le curseur dans le formulaire. Permet de relancer une génération sans fermer/rouvrir la modale.

**Bon à savoir** :
- Fermer la modale puis recliquer sur "✨ Générer via IA" affiche le même résultat (modale conserve l'état du dernier job).
- Cliquer "Nouvelle génération" sans fermer reste dans la modale — gain de temps pour un deuxième lot.
- Les questions de la première génération restent intactes dans QuestionsPage, quelle que soit votre choix (Fermer ou Nouvelle génération).

### ⚠️ Note de sécurité

#### Génération de questions (`POST /api/generate-questions`)

L'endpoint de génération **n'a aucune authentification**, exactement comme le reste du serveur BuzzControl. Cela signifie :

- **Toute personne sur le réseau LAN** peut déclencher une génération
- **Chaque génération est facturée** sur le compte Anthropic de l'opérateur (celui dont la clé API est configurée)
- Si votre serveur est exposé sur un réseau non maîtrisé (WiFi public, réseau d'entreprise ouvert), des tiers peuvent vous facturer involontairement des appels API

**Recommandation** :
- **Ne pas exposer ce serveur** sur un réseau public ou non maîtrisé tant qu'il n'y a pas d'authentification globale
- **Limiter l'accès** via un pare-feu réseau ou un reverse proxy authentifiant
- **Surveiller votre compte Anthropic** pour détecter toute utilisation anormale

Cette limitation a été **acceptée explicitement** au GATE 2 du chantier comme un risque connu et managé par l'opérateur (voir contrats/CHANGELOG.md pour le détail).

#### Validation de clé API (`POST /api/ai/validate-key`)

Depuis v6.0.2 (bugfix/config-api-key-help), une **nouvelle vérification de clé** valide votre clé API auprès du fournisseur au moment de l'enregistrement. Deux points de sécurité supplémentaires à connaître :

**1. Coût d'abus gratuit pour tester une clé déjà obtenue**

Auparavant, tester une clé valait via `POST /api/generate-questions` consommait des tokens facturés sur votre compte Anthropic — un frein naturel aux abus. Désormais, `POST /api/ai/validate-key` est **gratuit et illimité** (juste un cooldown de 2 secondes entre deux validations). Cela signifie qu'un utilisateur du LAN peut tester une clé capturée ou trouvée sans coût financier.

**C'est la même classe de risque que `/api/generate-questions`** (accès LAN non authentifié) — pas une nouvelle vulnérabilité — mais la nature du frein change (financier → technique seulement). Si votre serveur est exposé en production à un réseau non contrôlé, surveillez votre compte Anthropic/Groq pour détecter des taux anormalement élevés d'authentifications échouées.

**2. Le badge "✅ Clé vérifiée" est indicatif, pas une garantie cryptographique**

Le badge marque que **la clé a été acceptée par le fournisseur une fois dans le passé** — généralement juste après l'enregistrement. Il ne garantit **jamais** que :
- La clé est encore valide à ce moment (elle peut avoir été révoquée entre-temps)
- La génération de questions réelle va aboutir (une clé valide peut échouer en génération pour une autre raison — modèle indisponible, quota dépassé, problème de schéma)
- Quelqu'un d'autre n'a pas découvert/utilisé votre clé depuis

Le badge est un **rappel visuel** (pour l'opérateur : "cette clé a été testée") et une **aide au diagnostic** (certaines erreurs de génération viennent d'une clé bien formée mais invalide — ce badge vous dit qu'on l'a vérifiée au moins une fois). C'est tout.

**Recommandation** :
- Gardez votre clé API **confidentielle** — ne la partagez jamais
- Si vous pensez qu'une clé a été exposée, **révoquez-la immédiatement** dans la console du fournisseur et générez-en une nouvelle
- Testez une clé neuve via "Enregistrer" dans les Paramètres (qui valide), pas en la laissant dans un historique ou un fichier de configuration checké en git

### Arrêt et reprise de la génération

**Pendant une génération en cours** :
- Bouton **"Arrêter"** dans la modale (visible pendant la progression)
- L'arrêt prend effet **entre deux lots** — jamais au milieu d'un appel au fournisseur
- Les questions **déjà générées sont conservées** (vous n'avez rien perdu)
- État sauvegardé : vous pouvez relancer une nouvelle génération après, ou éditer les questions existantes

**Exemple** :
- Génération de 200 questions prévue en 5 lots de 40
- Vous arrêtez après le lot 2 (80 questions générées)
- 80 questions restent dans QuestionsPage
- Vous pouvez les éditer, ou relancer une génération pour compléter

### Assurance qualité : Relecture obligatoire

**Important** : Le contenu généré par l'IA est une **aide à la création**, pas une finalité. Avant d'utiliser les questions en partie publique :

1. **Relisez chaque question** — même Claude peut contenir des typos ou imprécisions
2. **Vérifiez la pertinence** — l'IA peut générer des questions hors contexte ou trop évidentes
3. **Corrigez les défauts** — changez les valeurs de points, ajoutez une image, reformulez si besoin
4. **Testez en partie réelle** — voir comment les questions jouent en situation réelle

**Surtout avec Groq (gratuit)** : La qualité du français n'est pas garantie, expect corriger plus de typos et reformulations que Claude.

### Régénération

Si vous êtes insatisfait des questions générées :
1. **Supprimer** les questions du lot (sélectionner et cliquer le bouton ×)
2. **Relancer** la génération avec des paramètres ajustés (autre difficulté, autres catégories, autre thème, etc.)
3. L'IA tiendra compte du nouvel état (les questions supprimées ne seront **pas régénérées identiques**)

**Note** : Il n'existe **pas de bouton "Annuler"** pour une génération passée — la suppression manuelle sert de mécanisme de correction. Pour arrêter une génération **en cours**, utilisez le bouton "Arrêter" (voir section ci-dessus).

---

## Workflow Admin v6.1.0 — Publics/Difficultés multiples, Objectifs, Filtres TV (Batch 2b #137)

### Sélection multiple populations et difficultés

Depuis v6.1.0, l'admin peut spécifier **plusieurs populations cibles et difficultés** pour une même partie.

#### Accès et interface

1. Ouvrir la page **Questions** (`/admin/questions`)
2. Localiser la section **"Quiz"** en haut de page (bloc gris)
3. **Populations cibles** : Passage de champ texte unique à **checkboxes multiples**
   - Cochez une ou plusieurs : Junior (6-12), Ado (13-17), Adulte (18-64), Senior (65+), Famille
   - Aucune sélection → tableau vide `[]` (défaut)
4. **Difficultés visées** : Passage de champ texte unique à **checkboxes multiples**
   - Cochez une ou plusieurs : Facile, Moyen, Difficile, Expert
   - Aucune sélection → tableau vide `[]` (défaut)

#### Impact sur la génération IA

Les sélections globales Population/Difficulté pré-remplissent automatiquement la modale "Générer via IA" :
- **Bloc informatif** (non-modifiable) : Affiche les valeurs globales du Quiz
- **Bloc "Cette génération"** (éditable) : Les checkboxes héritent les sélections globales, mais restent modifiables (permet une génération différente sans affecter le global)

**Exemple** :
- Quiz global : Populations = `["Adulte", "Senior"]`, Difficultés = `["Moyen"]`
- Modale génération : Les checkboxes affichent "Adulte" ✓ "Senior" ✓ et "Moyen" ✓
- Vous pouvez décocher "Senior" et cocher "Facile" pour générer un lot "Adulte + Facile" sans changer le global

#### Affichage sur l'écran TV NEW_GAME

Les populations et difficultés sélectionnées s'affichent sur la TV en **ligne compacte de badges** :
- Badges "Junior", "Ado", "Adulte", "Senior", "Famille" (si sélectionnés)
- Badges "Facile", "Moyen", "Difficile", "Expert" (si sélectionnés)
- **Uniquement si non-vides** — tableau vide `[]` n'affiche rien

**Contraintes TV statique** : `overflow: hidden`, unités viewport (`vh`, `vw`), `flex` avec `min-height: 0` — les badges s'affichent sans déborder.

---

### Objectif pédagogique de partie (QUIZ_OBJECTIVES)

Nouveau champ texte libre permettant de documenter l'objectif ou le thème pédagogique de la partie.

#### Accès et interface

1. Page **Questions**, section **"Quiz"**
2. Nouveau champ **"Objectif de partie"** (texte libre, multi-ligne)
   - Exemple : "Révision scolaire sur la Seconde Guerre mondiale"
   - Exemple : "Team-building ambiance conviviale sans enjeu"
   - Optionnel — laisser vide si non applicable

#### Confidentialité — Jamais visible aux joueurs

- **`/ws/admin`** : Reçoit `QUIZ_OBJECTIVES` dans le GameState complet
- **`/ws/tv` (écran TV)** : N'reçoit **pas** `QUIZ_OBJECTIVES` (masqué par le serveur)
- **`/ws/player` (interface VPlayer)** : N'reçoit **pas** `QUIZ_OBJECTIVES` (masqué par le serveur)

**Cas d'usage** : L'admin peut documenter que la partie vise la "révision" ou "team-building", sans que les joueurs voient cet objectif (garantit l'impartialité pédagogique).

---

### Filtres affichage TV par champ (QUIZ_HIDDEN_FIELDS)

Interface **interrupteurs "Afficher sur la TV"** permettant de masquer certains champs question lors de l'affichage sur l'écran TV.

#### Accès et interface

1. Page **Questions**, section **"Quiz"**
2. Nouvelle zone **"Visibilité TV"** avec liste de toggles :
   - ☑️ **Réponse** — affiche/masque le champ `ANSWER` en phase REVEALED
   - ☑️ **Image question** — affiche/masque `MEDIA` (image de la question)
   - ☑️ **Image réponse** — affiche/masque `MEDIA_ANSWER` (image de la réponse)
   - (Autres champs selon évolution future)

#### Comportement et diffusion

- **Stockage** : Les toggles coché/décochés sont sauvegardés dans `QUIZ_HIDDEN_FIELDS` (tableau de strings, ex: `["ANSWER"]`)
- **Diffusion WebSocket** :
  - **Tous endpoints web** (`/ws/admin`, `/ws/tv`, `/ws/player`) reçoivent `QUIZ_HIDDEN_FIELDS` dans le GameState
  - Le **serveur ne filtre pas** — il envoie la liste complète des champs masqués
  - **Le client applique le filtrage côté rendu** (frontend)

#### Exemple concret

**Scénario** : Vous voulez masquer la réponse sur la TV avant qu'elle ne soit révélée.

1. Cochez le toggle **"Réponse"** → `QUIZ_HIDDEN_FIELDS = ["ANSWER"]`
2. En phase STARTED : La réponse n'apparaît pas à l'écran TV
3. En phase REVEALED : La réponse s'affiche (le frontend applique le masquage intelligemment selon la phase)

**Cas d'usage courant** : Masquer l'image réponse pendant le jeu pour plus de suspense — l'image apparaît seulement après la révélation.

#### Rétrocompatibilité

- Ancien quiz sans toggles → `QUIZ_HIDDEN_FIELDS = []` (affiche tout par défaut)
- Aucune modification retroactive sur les questions existantes

---

## Mode ENTRACTE — Pause Globale

### Qu'est-ce que c'est ?

L'entracte est une **pause globale** de la partie (pause déjeuner, changement de salle, 
annonce spéciale). Pendant une pause :

- La **TV et l'écran joueur** affichent un panneau estompé au-dessus du contenu actuel.
- L'**admin et l'animateur** voient leur interface estompée (l'admin garde un bouton 
  de contrôle visible).
- **Aucune action de jeu n'est possible** — pas de lancement, arrêt, scoring, rien.
- Les **LEDs des buzzers s'éteignent**.
- La question sélectionnée avant la pause **reste intacte** — à la reprise, on continue 
  où on s'était arrêté.

### Déclencher une pause

1. Arrêtez la question courante (bouton **STOP** ou attendez la révélation).
2. Attendez une phase sans manche active (`STOPPED`, `PREPARE`, `READY`, `NEW_GAME`, 
   `REVEALED`).
3. Cliquez le bouton **ENTRACTE** (navbar ou panel de contrôle).
4. Le bouton devient **FIN D'ENTRACTE** (rouge avec halo).

### Mettre fin à la pause

Cliquez le bouton **FIN D'ENTRACTE** — tout revient à la normale, la question 
précédente est prête à continuer.

> **Note** : vous pouvez sortir de l'entracte **à n'importe quel moment**, même si le 
> serveur avait refusé l'entrée (cas rare : une manche a démarré pendant que vous 
> remplissiez le formulaire de config).

### Configuration du panneau

Dans la **Page Quiz** (`/admin/quiz`), section **ENTRACTE** (ou bouton dédié dans la zone Quiz) :

| Champ | Défaut | Portée |
|-------|--------|--------|
| **Titre** | `ENTRACTE` | Texte grand affiché sur TV et écran joueur |
| **Sous-titre** | `Retour dans 20mn` | Texte secondaire (ex. horaire de reprise) |
| **Image de fond** | (aucune) | Image optionnelle (ex. logo, photo, pause café) |
| **Taille du panneau** | 65% | Pourcentage de l'écran (20–100) — affecte TV et écran joueur identiquement |
| **Vitesse du mouvement** | 10s | Durée d'un cycle d'animation (2–30 secondes) |
| **Amplitude du mouvement** | 20 | Force de l'animation (0–100 ; **0 = animation désactivée**) |

### Gel de la configuration à l'activation

**Point important** : dès que vous cliquez le bouton **ENTRACTE**, la configuration courante 
(titre, sous-titre, image, taille, animation) est **sauvegardée et gelée** — elle ne change 
plus pendant toute la durée de la pause, même si vous modifiez les réglages.

Les modifications apportées pendant un entracte actif **prennent effet au prochain cycle** 
(après sortie puis nouvelle entrée en entracte). C'est un mécanisme de sécurité pour que le 
panneau affiché reste stable pendant une pause prolongée.

**Exemple** : vous lancez une pause avec le titre « DÉJEUNER » et l'animation 
à 20. Pendant la pause, vous changez le titre en « RETOUR IMMINENT » et l'animation en 0. 
Quand vous cliquez « FIN D'ENTRACTE » et que vous relancez une pause 10 minutes plus tard, 
c'est le nouveau titre et la nouvelle animation qui s'afficheront.

### Animation du panneau

Le panneau anime un **respiration douce** : zoom oscillant combiné avec balancement 
(oscillation de rotation). Cet effet subtil rend le panneau plus vivant pendant une 
pause prolongée.

- **Augmentez l'amplitude** (ex. 50) pour une respiration plus prononcée — utile pour 
  une pause brève et attirer l'attention.
- **Diminuez l'amplitude** (ex. 5) ou mettez à **0** pour un panneau fixe — préférable 
  pour une pause longue (déjeuner).
- **Respecte `prefers-reduced-motion`** : les utilisateurs qui ont activé ce réglage 
  système voient un panneau fixe, quel que soit votre paramétrage — c'est une 
  accessibilité, pas une préférence esthétique.

### Image de fond

1. Cliquez **Choisir une image** dans la section Entracte de la page Quiz.
2. Sélectionnez un fichier depuis votre ordinateur (PNG, JPG, etc.).
3. L'image s'affiche immédiatement derrière le titre et le sous-titre (le changement prend 
   effet au prochain entracte si un est en cours).

Pour **supprimer l'image**, cliquez **Supprimer image** — le panneau reste lisible 
sans image de fond.

> **Note** : l'image est stockée côté serveur et persiste après un redémarrage (elle 
> est sauvegardée dans votre sauvegarde de partie si vous cochez « historique » lors 
> de la sauvegarde).

### Exigences de sécurité

- Seule l'**interface admin** peut déclencher / arrêter une pause.
- L'**interface animateur** voit quand une pause est active, mais ne peut pas la 
  commander (bouton grisé).
- Le **serveur bloque tout clic malveillant** — même si quelqu'un essaie de scorer ou 
  lancer une question depuis un navigateur hostile, le serveur refuse.

### Cas d'usage

1. **Pause déjeuner** : titre `"DÉJEUNER"`, sous-titre `"Retour à 14h"`, animation 
   très douce ou désactivée, durée 60 minutes.
2. **Changement de salle** : titre `"EN DÉPLACEMENT"`, animation rapide (vitesse 5s) 
   pour signaler du changement, image de logo.
3. **Annonce spéciale** : titre customisé (ex. `"TIRAGE AU SORT"`), image pertinente, 
   animation moyenne pour garder l'attention.

### Workflow complet — étapes recommandées

1. **Ouvrir la page Questions** (`/admin/questions`)
2. **Bloc Quiz** :
   - Saisir Nom, Thème, Notes (existant)
   - ✨ **Nouveau** : Sélectionner Populations (checkboxes multiples)
   - ✨ **Nouveau** : Sélectionner Difficultés (checkboxes multiples)
   - ✨ **Nouveau** : Remplir Objectif de partie (texte, optionnel)
   - ✨ **Nouveau** : Cocher les toggles Visibilité TV (ex: masquer Réponse)
   - Choisir Langue
3. **Générer via IA** (si applicable) :
   - Les selections Population/Difficulté pré-remplissent la modale
   - Ajuster si besoin pour cette génération
4. **Sauvegarder** → Les paramètres sont persistés
5. **Affichage TV** :
   - Les badges Population/Difficulté s'affichent (s'il non-vides)
   - L'Objectif n'est pas visible (masqué au serveur)
   - Les champs cochés "Afficher TV" s'affichent selon la phase

---

## Mode RAFALE — Guide Animateur (v8.0.0, #16)

### Vue d'ensemble

RAFALE est un mode de jeu « questions en rafale » : sur une manche de ~2 minutes, des questions s'enchaînent automatiquement avec ~3 secondes par question. **L'animateur valide ou invalide chaque réponse via deux gros boutons tactiles** sur sa tablette (`/anim`). Les points sont attribués manuellement en fin de manche.

### Configuration de manche

Sur l'écran admin (`/admin`) :

1. **Créer/éditer une question de type RAFALE**
   - Cliquer « Question de type RAFALE »
   - Réglages : durée manche (~120 s), barème de manche (points par bonne réponse)

2. **Filtrer le réservoir**
   - Sélectionner une ou plusieurs **catégories**
   - Choisir une **difficulté unique** (1–3)
   - Voir le nombre de questions **disponibles** et **utilisées**

3. **Pré-validations automatiques**
   - 🔴 **Bloquant** : aucune question disponible → démarrage refusé
   - 🟠 **Avertissement** : trop peu de questions → risque fin anticipée
   - ✅ **OK** : assez de questions → aucun avertissement

4. **Sélectionner les équipes** (si mode multi)
   - Ordre = ordre de rotation

### Pendant la manche

Sur la tablette animateur (`/anim`, en grand écran) :

```
┌─────────────────────────────────┐
│  Timer manche: 01:45             │
│  Timer question: 2.3s            │
│                                   │
│  Question: Capitale de l'Italie? │
│  Réponse: Rome                   │
│  Équipe active: Red              │
│                                   │
│  ┌────────────────────────────────┐
│  │  ✓ RÉPONSE VALIDE (vert)      │
│  ├────────────────────────────────┤
│  │  ✗ RÉPONSE INVALIDE (rouge)   │
│  └────────────────────────────────┘
└─────────────────────────────────┘
```

**Actions** :
- **Appuyer RÉPONSE VALIDE** → l'équipe gagne un point de compteur (règle mode-dependent)
- **Appuyer RÉPONSE INVALIDE** → l'équipe perd la main (rotation) ou remet compteur à 0
- **Timer question expire (0 s)** → équivalent à RÉPONSE INVALIDE
- **Attendre fin manche** (timer global à 0 ou pool épuisée)

### Modes de jeu

| Mode | Règle | Animation Utile |
|------|-------|-----------------|
| **SOLO** | Une équipe joue seule, aucune rotation | Compteur équipe visible |
| **CHACUN SON TOUR** | Bonne OU mauvaise → équipe suivante | Nom équipe change tout le temps |
| **TANT QUE JE GAGNE** | Bonne → garde la main ; Mauvaise → suivante | Équipe peut rester longtemps |
| **MAILLON FAIBLE** | Idem CHACUN, mais compteur → 0 sur mauvaise (meilleur mémorisé) | Affichage meilleur compteur, pas compteur courant |

### Fin de manche

Sous-phase `ROUND_END` : écran affiche les compteurs finaux et un tableau d'attribution :

```
Compteurs finaux :
  Red:   12 points
  Blue:  8  points
  Green: 10 points

Attribution des points (cliquer une équipe) :
  [ Red (12 × 5 = 60 pts suggérés) ]
  [ Blue (8 × 5 = 40 pts) ]
  [ Green (10 × 5 = 50 pts) ]
```

**À l'admin (ou l'animateur s'il a accès) :
1. Cliquer une équipe → valeur pré-remplie = compteur × barème
2. Ajuster si souhaité (ex: attribuer au seul gagnant, ou répartir)
3. Valider → fin manche, retour à STOPPED

**Important** : aucun point n'est attribué pendant la manche, seuls les compteurs visibles. L'attribution manuelle en fin de manche est **l'étape décisive**.

### Gestion du réservoir (`/admin/rafale`)

**Avant les premières manches** :

1. Créer des questions en RAFALE (`/admin/rafale`)
2. Chaque question = énoncé + réponse + catégorie + difficulté
3. Sauvegarder

**Entre les manches** :

- Les questions utilisées sont marquées **"Utilisée"**
- Elles ne réapparaissent plus dans la même manche (même après arrêt/redémarrage serveur)
- Ajouter de nouvelles questions ou réinitialiser le flag (commande admin `/reset` sélectif) pour relancer

**Réinitialisation sélective** (`/admin/backup`) :

- Checkbox « Questions RAFALE » permet de réinitialiser le flag uniquement (garde le réservoir intact)
- Permet de rejouer les mêmes questions sans les re-créer

### Affichages

**Sur la TV (`/tv`)** :
- Question courante (gros texte)
- Double timer (manche + question)
- Équipe active (si mode multi)
- **Jamais la réponse** (seulement l'animateur la voit)

**Sur les VJoueurs (`/player`)** :
- Indicateur « C'est ton tour ! » (si équipe active, mode multi)
- Aucun bouton tactile pendant RAFALE (tous les contrôles sont sur l'interface animateur)

**Sur les LED buzzers** :
- Équipe active : couleur pleine
- Équipe suivante : couleur atténuée
- Autres : très atténuées ou éteintes



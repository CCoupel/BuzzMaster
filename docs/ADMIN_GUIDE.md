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
- Pour les questions NORMAL, les buzzers physiques fonctionnent normalement
- Le VJoueur peut aussi buzzer sur les questions NORMAL via son bouton "BUZZ !"
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

Les catégories personnalisées sont incluses dans le backup/restore via le flag `backgrounds` :
```
GET /backup-select?backgrounds=true
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

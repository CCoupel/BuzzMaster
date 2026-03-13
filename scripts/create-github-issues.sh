#!/bin/bash
# =============================================================================
# Script de création des issues GitHub depuis le backlog BuzzMaster
#
# Prérequis : gh auth login (GitHub CLI authentifié)
# Usage : ./scripts/create-github-issues.sh
# =============================================================================

set -euo pipefail

REPO="CCoupel/BuzzMaster"

echo "🐝 Création des issues GitHub depuis le backlog BuzzMaster"
echo "==========================================================="
echo ""

# Vérifier l'authentification gh
if ! gh auth status &>/dev/null; then
    echo "❌ Erreur : gh n'est pas authentifié. Exécutez 'gh auth login' d'abord."
    exit 1
fi

# Créer les labels nécessaires (ignore si déjà existants)
echo "📌 Création des labels..."
gh label create "backlog" --description "Item du backlog BuzzMaster" --color "0075ca" --repo "$REPO" 2>/dev/null || true
gh label create "enhancement" --description "New feature or request" --color "a2eeef" --repo "$REPO" 2>/dev/null || true
gh label create "TODO" --description "Planifié, pas encore démarré" --color "fbca04" --repo "$REPO" 2>/dev/null || true
gh label create "En-Cours" --description "Implémentation en cours" --color "0e8a16" --repo "$REPO" 2>/dev/null || true
gh label create "DONE" --description "Fonctionnalité complétée et livrée" --color "5319e7" --repo "$REPO" 2>/dev/null || true
gh label create "REMOVED" --description "Fonctionnalité abandonnée" --color "e4e669" --repo "$REPO" 2>/dev/null || true
gh label create "backend" --description "Serveur Go" --color "d73a4a" --repo "$REPO" 2>/dev/null || true
gh label create "frontend" --description "Interface React" --color "7057ff" --repo "$REPO" 2>/dev/null || true
gh label create "firmware" --description "Firmware BuzzClick ESP32" --color "006b75" --repo "$REPO" 2>/dev/null || true
gh label create "ci-cd" --description "CI/CD et workflows" --color "e99695" --repo "$REPO" 2>/dev/null || true
gh label create "memory-game" --description "Jeu Memory" --color "1d76db" --repo "$REPO" 2>/dev/null || true
gh label create "ai" --description "Intelligence artificielle" --color "b60205" --repo "$REPO" 2>/dev/null || true
echo "✅ Labels créés"
echo ""

# ============================================================================
# ISSUE 1 : Générateur de jeu via IA (TODO)
# ============================================================================
echo "📝 Création issue : Générateur de jeu via IA..."
gh issue create --repo "$REPO" \
    --title "Générateur de jeu complet via IA" \
    --label "enhancement,backlog,TODO,ai" \
    --body "$(cat <<'ISSUE_BODY'
## Description

Outil/site web pour générer automatiquement un jeu complet BuzzMaster via une IA générative.
L'utilisateur fournit des paramètres de jeu, et l'IA génère un fichier de backup (.tar) prêt à être importé dans BuzzMaster, contenant questions, médias, équipes, et configuration.

📄 **Spécification complète** : [`backlog/TODO/generateur-ia.md`](https://github.com/CCoupel/BuzzMaster/blob/main/backlog/TODO/generateur-ia.md)

## Phase 1 - Core Generator (MVP)

### Interface de configuration
- [ ] Formulaire web avec paramètres : population cible, difficulté, thème, catégories, volume, répartition types (QCM/Normal/Memory), langue

### Backend générateur
- [ ] Intégration API LLM (Claude API recommandé)
- [ ] Génération structurée des questions avec validation JSON
- [ ] Prompt engineering pour qualité et cohérence
- [ ] Gestion de la génération par lots

### Génération de contenu
- [ ] Questions normales, QCM, Memory
- [ ] Attribution automatique des catégories
- [ ] Équilibrage entre catégories (CategoryBalance)

### Export backup BuzzMaster
- [ ] Génération structure TAR compatible (`config/teams.json`, `files/questions/`)
- [ ] Téléchargement du fichier .tar

## Phase 2 - Améliorations UX
- [ ] Preview et édition avant export
- [ ] Templates de jeu prédéfinis (Quiz TV, Trivia Pub, Formation, Révision scolaire, Animation)
- [ ] Métadonnées du jeu

## Phase 3 - Génération de médias
- [ ] Génération d'images via IA (DALL-E 3, Stable Diffusion)
- [ ] Recherche d'images libres de droits (Unsplash, Pexels, Pixabay)
- [ ] Images pour Memory (paires cohérentes)

## Phase 4 - Architecture et déploiement
- [ ] Choix architecture : site externe / intégré BuzzMaster / CLI / SaaS
- [ ] Gestion des coûts API

## Phase 5 - Qualité et personnalisation
- [ ] Validation automatique des questions (cohérence, doublons, orthographe)
- [ ] Import de contexte spécifique (PDF, texte) pour questions sur-mesure

## Phase 6 - Analytics
- [ ] Feedback utilisateur et ratings
- [ ] Statistiques d'utilisation

## Cas d'usage identifiés

| Cas d'usage | Paramètres suggérés |
|-------------|---------------------|
| Anniversaire enfant | Junior, Facile, Cinéma, 20 questions, 70% QCM |
| Soirée entre amis | Adulte, Moyen, Culture générale, 50 questions |
| Formation entreprise | Adulte, Difficile, IT, 30 questions |
| Révision scolaire | Junior, Moyen, Histoire, 40 questions |
| Résidence seniors | Senior, Facile, Musique/Cinéma, 30 questions |

## Technologies suggérées
- **Backend** : Go (cohérence) ou Node.js
- **LLM** : Claude API (recommandé), OpenAI GPT-4
- **Images** : DALL-E 3, Unsplash API
ISSUE_BODY
)"
echo "✅ Issue créée"

# ============================================================================
# ISSUE 2 : Métadonnées dans les binaires (TODO)
# ============================================================================
echo "📝 Création issue : Métadonnées dans les binaires..."
gh issue create --repo "$REPO" \
    --title "Métadonnées dans les binaires (Windows PE + ldflags)" \
    --label "enhancement,backlog,TODO,backend,ci-cd" \
    --body "$(cat <<'ISSUE_BODY'
## Description

Ajouter des métadonnées (nom du produit, version, description) dans les binaires exécutables Windows (.exe) et Linux pour une meilleure identification et traçabilité.

📄 **Spécification complète** : [`backlog/TODO/metadata-binaires.md`](https://github.com/CCoupel/BuzzMaster/blob/main/backlog/TODO/metadata-binaires.md)

## Phase 1 - Windows PE Metadata

Utiliser `goversioninfo` pour générer un fichier `.syso` avec les infos Windows.

- [ ] Installer `goversioninfo` dans le workflow CI
- [ ] Créer `cmd/server/versioninfo.json` avec template
- [ ] Créer/trouver une icône `assets/icon.ico`
- [ ] Modifier le script de build pour générer le `.syso`
- [ ] Vérifier les métadonnées dans Propriétés Windows

**Résultat attendu (Propriétés Windows)** :
```
Nom du produit : BuzzControl
Version du fichier : X.Y.0.0
Description : Wireless Buzzer System for Quiz Games
Copyright : 2026 CCoupel
```

## Phase 2 - Version embarquée (ldflags)

- [ ] Ajouter variables `Version`, `ProductName`, `BuildTime`, `GitCommit` dans `cmd/server/main.go`
- [ ] Modifier scripts de build (local + CI) avec `-ldflags`
- [ ] Afficher version au démarrage du serveur
- [ ] Endpoint `/version` retourne les infos complètes

## Phase 3 - Intégration CI

- [ ] Modifier `.github/workflows/release.yml`
- [ ] Installer `goversioninfo` dans le job Windows
- [ ] Générer `versioninfo.json` dynamiquement depuis le tag
- [ ] Ajouter ldflags au build Linux ARM64

## Fichiers concernés

| Fichier | Modification |
|---------|--------------|
| `cmd/server/main.go` | Variables Version, ProductName, BuildTime, GitCommit |
| `cmd/server/versioninfo.json` | Nouveau - Template métadonnées Windows |
| `assets/icon.ico` | Nouveau - Icône Windows |
| `.github/workflows/release.yml` | Ajout goversioninfo + ldflags |
| `build.ps1` / `build.sh` | Ajout ldflags pour build local |
ISSUE_BODY
)"
echo "✅ Issue créée"

# ============================================================================
# ISSUE 3 : Réorganisation layout modale USB (TODO)
# ============================================================================
echo "📝 Création issue : Layout modale USB compact..."
gh issue create --repo "$REPO" \
    --title "Réorganisation layout modale USB (compact, sans scroll)" \
    --label "enhancement,backlog,TODO,frontend" \
    --body "$(cat <<'ISSUE_BODY'
## Description

Revoir l'organisation visuelle de la modale USB (`USBConfigModal`) pour que tout son contenu tienne dans la modale sans nécessiter de scroll.

Le bouton **"Flash USB"** doit être repositionné sur la même ligne que le bouton **"Envoyer et configurer"**, côte à côte.

📄 **Spécification complète** : [`backlog/TODO/usb-modal-layout-compact.md`](https://github.com/CCoupel/BuzzMaster/blob/main/backlog/TODO/usb-modal-layout-compact.md)

## Objectifs

- [ ] Modale entièrement visible sans scroll, quelle que soit la résolution standard
- [ ] Bouton "Flash USB" aligné horizontalement avec "Envoyer et configurer" (même ligne)

## Tâches

- [ ] Analyser le layout actuel de `USBConfigModal.jsx` et identifier ce qui dépasse
- [ ] Réduire les marges/paddings excessifs, compresser les sections si nécessaire
- [ ] Déplacer le bouton "Flash USB" sur la même ligne que "Envoyer et configurer"
- [ ] Vérifier que la modale tient sans scroll sur 1280×720 et 1920×1080
- [ ] Tester visuellement

## Fichier concerné

- `web/src/components/USBConfigModal.jsx`
ISSUE_BODY
)"
echo "✅ Issue créée"

# ============================================================================
# ISSUE 4 : Filtrage broadcasts WebSocket (TODO)
# ============================================================================
echo "📝 Création issue : Filtrage broadcasts WebSocket..."
gh issue create --repo "$REPO" \
    --title "Filtrage des broadcasts WebSocket par type de client" \
    --label "enhancement,backlog,TODO,backend" \
    --body "$(cat <<'ISSUE_BODY'
## Description

Actuellement, tous les messages WebSocket sont envoyés à tous les clients connectés (admin, TV, VJoueur) sans distinction. Cette amélioration ajoute un filtrage intelligent pour n'envoyer que les messages pertinents à chaque type de client.

📄 **Spécification complète** : [`backlog/TODO/websocket-broadcast-filtre.md`](https://github.com/CCoupel/BuzzMaster/blob/main/backlog/TODO/websocket-broadcast-filtre.md)

## Problème actuel

| Message | Admin | TV | VJoueur | Actuellement |
|---------|:-----:|:--:|:-------:|:------------:|
| UPDATE | ✅ | ✅ | ✅ | Tous ✅ |
| QUESTIONS | ✅ | ❌ | ❌ | Tous ❌ |
| CLIENTS | ✅ | ❌ | ❌ | Tous ❌ |
| BACKGROUND_CHANGE | ❌ | ✅ | ✅ | Tous ❌ |
| ENROLLMENT_UPDATE | ✅ | ✅ | ❌ | Tous ❌ |

Les clients reçoivent des messages inutiles → bande passante gaspillée.

## Phase 1 - Backend

- [ ] Ajouter `ClientType` dans `internal/server/websocket.go`
- [ ] Modifier `WSClient` pour inclure le type
- [ ] Parser `SET_CLIENT_TYPE` pour définir le type
- [ ] Créer `broadcastFiltered(msg, targets...)` dans `main.go`
- [ ] Remplacer les appels `broadcast()` par `broadcastFiltered()` avec les bons filtres

## Phase 2 - Frontend

- [ ] Ajouter `ClientPlayer` pour les VJoueurs
- [ ] Envoyer `SET_CLIENT_TYPE` depuis VPlayerPage

## Phase 3 - Tests

- [ ] Test unitaire : broadcast sans filtre → tous reçoivent
- [ ] Test unitaire : broadcast avec filtre → seuls les ciblés reçoivent
- [ ] Test E2E : vérifier qu'un VJoueur ne reçoit pas QUESTIONS

## Fichiers concernés

| Fichier | Modification |
|---------|--------------|
| `internal/server/websocket.go` | Ajouter `ClientType`, modifier `WSClient` |
| `cmd/server/main.go` | `broadcastFiltered()`, mise à jour des appels |
| `web/src/pages/VPlayerPage.jsx` | Envoyer `SET_CLIENT_TYPE: "player"` |

## Avantages

- Réduction du trafic WebSocket inutile
- Code plus explicite (on sait qui reçoit quoi)
- Préparation pour restrictions de sécurité
- Pas de refactoring majeur (amélioration incrémentale)
ISSUE_BODY
)"
echo "✅ Issue créée"

# ============================================================================
# ISSUE 5 : Memory Game Phase 7 - Modes de scoring (En-Cours)
# ============================================================================
echo "📝 Création issue : Memory Game - Phase 7 Modes de scoring..."
gh issue create --repo "$REPO" \
    --title "Memory Game - Phase 7 : Modes de scoring avancés" \
    --label "enhancement,backlog,En-Cours,backend,frontend,memory-game" \
    --body "$(cat <<'ISSUE_BODY'
## Description

Ajout de modes de scoring (points) avancés pour le jeu Memory. Les Phases 1-6 sont complétées (v2.51.0). La Phase 7 définit **comment les points sont calculés** et est **combinable** avec les modes de jeu existants (SOLO, CHACUN_SON_TOUR, TANT_QUE_JE_GAGNE).

📄 **Spécification complète** : [`backlog/En-Cours/memory-game.md`](https://github.com/CCoupel/BuzzMaster/blob/main/backlog/En-Cours/memory-game.md) (section Phase 7)

## Modes de scoring

- [ ] **TO_THE_END** (défaut, déjà implémenté) — Paires restent visibles, scoring classique
- [ ] **MORT_SUBITE** — Erreur = RESET complet (cartes cachées, scores à zéro), high score conservé
- [ ] **PERFECT** — Identique à TO_THE_END + gros bonus si aucune erreur
- [ ] **CASCADE** — Multiplicateur progressif (×1 → ×5) sur paires consécutives sans erreur
- [ ] **TIME_BONUS** — Bonus proportionnel au temps restant
- [ ] **ZERO_SUM** — Score peut devenir négatif, pénalités élevées

## Modes de jeu supplémentaires

- [ ] **MAILLON_FAIBLE** — Hybride : garde main si match + reset global si erreur + options cascade/élimination
- [ ] **ELIMINATION** — Battle royale, quota d'erreurs par équipe
- [ ] **SPEED_RUN** — Timer court par tour
- [ ] **BLITZ** — Cartes se cachent plus vite (1.5s au lieu de 3s)

## Tâches backend

- [ ] Ajouter champ `MEMORY_SCORING_MODE` dans le modèle Question
- [ ] Étendre `GameState` : `MemoryHighScore`, `MemoryResetCount`, `MemoryMultiplier`, `MemoryStreak`
- [ ] Implémenter logique reset MORT_SUBITE dans `engine.go`
- [ ] Implémenter multiplicateur CASCADE dans `engine.go`
- [ ] Implémenter logique MAILLON_FAIBLE (reset + élimination optionnelle)
- [ ] Étendre `GameState` pour MAILLON_FAIBLE : `MemoryTeamErrors`, `MemoryEliminatedTeams`

## Tâches frontend

- [ ] Sélecteur mode scoring dans QuestionsPage
- [ ] Affichages spécifiques MORT_SUBITE (badge, high score, animation reset)
- [ ] Affichages CASCADE (multiplicateur dynamique, streak)
- [ ] Affichages TIME_BONUS (projection bonus temps)
- [ ] Affichages MAILLON_FAIBLE (vies, équipes éliminées, animations)

## Combinaisons pertinentes

| Mode de jeu | Mode scoring | Difficulté |
|-------------|-------------|------------|
| SOLO + TO_THE_END | Classique | ⭐ |
| SOLO + CASCADE | Multiplicateur | ⭐⭐⭐ |
| SOLO + MORT_SUBITE | Hardcore | ⭐⭐⭐⭐⭐ |
| CHACUN_SON_TOUR + CASCADE | Multi compétitif | ⭐⭐⭐⭐ |
| TANT_QUE_JE_GAGNE + MORT_SUBITE | Tension max | ⭐⭐⭐⭐⭐ |
| MAILLON_FAIBLE + chaîne + élim | Extrême | ⭐⭐⭐⭐⭐ |

## Compatibilité

- ✅ Rétrocompatible : Questions sans `MEMORY_SCORING_MODE` → "TO_THE_END"
- ✅ 4 modes jeu × 6 modes points = 24+ variantes
ISSUE_BODY
)"
echo "✅ Issue créée"

# ============================================================================
# ISSUES DONE (fermées)
# ============================================================================
echo ""
echo "📦 Création des issues DONE (seront fermées automatiquement)..."
echo ""

# Helper function for DONE issues
create_done_issue() {
    local title="$1"
    local version="$2"
    local labels="$3"
    local body="$4"

    echo "📝 [DONE] $title ($version)..."
    local url
    url=$(gh issue create --repo "$REPO" \
        --title "$title" \
        --label "$labels" \
        --body "$body")

    # Extract issue number and close it
    local issue_num
    issue_num=$(echo "$url" | grep -oP '\d+$')
    if [ -n "$issue_num" ]; then
        gh issue close "$issue_num" --repo "$REPO" --comment "Complété en $version" 2>/dev/null || true
    fi
    echo "✅ Issue créée et fermée"
}

create_done_issue \
    "Découverte serveur via UDP broadcast heartbeat" \
    "v3.2.0" \
    "enhancement,backlog,DONE,backend,firmware" \
    "$(cat <<'BODY'
## Complété en v3.2.0

Découverte automatique de l'IP serveur via UDP heartbeat (DHCP friendly, LED boot sequence enrichie).

📄 Spec : [`backlog/DONE/udp-broadcast-server-discovery.md`](https://github.com/CCoupel/BuzzMaster/blob/main/backlog/DONE/udp-broadcast-server-discovery.md)

- Format : `BUZZ_SERVER|IP1|IP2|...|PORT\0`
- Intervalle : 5s normal, 1s en enrollment
- Chaîne de fallback firmware : UDP → NVS → mDNS → retry
BODY
)"

create_done_issue \
    "Mise à jour OTA firmware buzzers" \
    "v3.1.0" \
    "enhancement,backlog,DONE,backend,frontend,firmware" \
    "$(cat <<'BODY'
## Complété en v3.1.0

Mise à jour OTA du firmware des buzzers BuzzClick (détection version, pastille obsolète, upload + restore embarqué).

📄 Spec : [`backlog/DONE/buzzer-firmware-ota-update.md`](https://github.com/CCoupel/BuzzMaster/blob/main/backlog/DONE/buzzer-firmware-ota-update.md)

- Endpoints : `/api/firmware/buzzclick/*`, `/api/buzzer/*/update`
- Actions WS : OTA_UPDATE, OTA_PROGRESS, FIRMWARE_VERSION
- Firmware embarqué dans le binaire serveur (go:embed)
BODY
)"

create_done_issue \
    "Migration protocole buzzers TCP → WebSocket" \
    "v3.0.0" \
    "enhancement,backlog,DONE,backend,firmware" \
    "$(cat <<'BODY'
## Complété en v3.0.0

Migration du protocole buzzers de TCP vers WebSocket avec hub dédié `/ws/buzzer`.

📄 Spec : [`backlog/DONE/buzzer-protocol-websocket.md`](https://github.com/CCoupel/BuzzMaster/blob/main/backlog/DONE/buzzer-protocol-websocket.md)

- Hub WebSocket dédié : BuzzerWebSocketHub
- Mode hybride TCP+WS pour rétrocompatibilité
- Endpoint : `/ws/buzzer`
BODY
)"

create_done_issue \
    "VJoueur valide pour les 4 couleurs QCM + buzz direct" \
    "v2.53.0" \
    "enhancement,backlog,DONE,frontend" \
    "$(cat <<'BODY'
## Complété en v2.53.0

VJoueur valide pour les 4 couleurs QCM + buzz direct sur écran.

📄 Spec : [`backlog/DONE/vjoueur-qcm-multicolore.md`](https://github.com/CCoupel/BuzzMaster/blob/main/backlog/DONE/vjoueur-qcm-multicolore.md)
BODY
)"

create_done_issue \
    "Memory Game - Phases 1-6 (modèle, TV, gameplay, admin, scoring, multi-équipes)" \
    "v2.51.0" \
    "enhancement,backlog,DONE,backend,frontend,memory-game" \
    "$(cat <<'BODY'
## Complété en v2.51.0

Jeu de mémoire complet avec paires de cartes à retrouver, 3 modes de jeu multi-équipes.

📄 Spec : [`backlog/En-Cours/memory-game.md`](https://github.com/CCoupel/BuzzMaster/blob/main/backlog/En-Cours/memory-game.md) (Phases 1-6)

### Phases complétées
- ✅ Phase 1 : Modèle et création de question MEMORY
- ✅ Phase 2 : État du jeu Memory et Affichage TV (grille responsive, flip 3D CSS)
- ✅ Phase 3 : Gameplay interactif (FLIP_MEMORY_CARD, logique révélation, fin de partie)
- ✅ Phase 4 : Interface Admin (indicateurs temps réel, révéler tout)
- ✅ Phase 5 : Scoring (points par paire, bonus complétion, pénalité erreur)
- ✅ Phase 6 : Modes multi-équipes (SOLO, CHACUN_SON_TOUR, TANT_QUE_JE_GAGNE)
BODY
)"

create_done_issue \
    "Style neutre gris pour cartes joueurs (admin)" \
    "v2.49.0" \
    "enhancement,backlog,DONE,frontend" \
    "$(cat <<'BODY'
## Complété en v2.49.0

Style neutre gris pour cartes joueurs sur la page admin Jeu.

📄 Spec : [`backlog/DONE/admin-joueur-card-style.md`](https://github.com/CCoupel/BuzzMaster/blob/main/backlog/DONE/admin-joueur-card-style.md)
BODY
)"

create_done_issue \
    "Menu déroulant abeille dans la navbar" \
    "v2.49.0" \
    "enhancement,backlog,DONE,frontend" \
    "$(cat <<'BODY'
## Complété en v2.49.0

Menu déroulant abeille (🐝) dans la navbar : Config, Logs, Backup, MAJ.

📄 Spec : [`backlog/DONE/navbar-menu-connexion.md`](https://github.com/CCoupel/BuzzMaster/blob/main/backlog/DONE/navbar-menu-connexion.md)
BODY
)"

create_done_issue \
    "Identification WebSocket des VJoueurs" \
    "v2.47.0" \
    "enhancement,backlog,DONE,backend,frontend" \
    "$(cat <<'BODY'
## Complété en v2.47.0

Identification WebSocket des VJoueurs avec type `vplayer` distinct.

📄 Spec : [`backlog/DONE/vjoueur-websocket-identification.md`](https://github.com/CCoupel/BuzzMaster/blob/main/backlog/DONE/vjoueur-websocket-identification.md)
BODY
)"

create_done_issue \
    "Effet néon couleur catégorie sur TV et VJoueur" \
    "v2.46.0" \
    "enhancement,backlog,DONE,frontend" \
    "$(cat <<'BODY'
## Complété en v2.46.0

Effet néon avec couleur dynamique par catégorie sur TV et VJoueur.

📄 Spec : [`backlog/DONE/effet-neon-categorie.md`](https://github.com/CCoupel/BuzzMaster/blob/main/backlog/DONE/effet-neon-categorie.md)
BODY
)"

create_done_issue \
    "Interface personnalisée joueur smartphone (Phase 1)" \
    "v2.45.0" \
    "enhancement,backlog,DONE,frontend" \
    "$(cat <<'BODY'
## Complété en v2.45.0

Interface personnalisée pour jouer depuis smartphone (/player).

📄 Spec : [`backlog/DONE/page-joueur.md`](https://github.com/CCoupel/BuzzMaster/blob/main/backlog/DONE/page-joueur.md)
BODY
)"

create_done_issue \
    "Tri équipes/joueurs par rapidité de buzz" \
    "v2.44.1" \
    "enhancement,backlog,DONE,backend,frontend" \
    "$(cat <<'BODY'
## Complété en v2.44.1

Tri des équipes et joueurs par rapidité de buzz.

📄 Spec : [`backlog/DONE/tri-rapidite-reponse.md`](https://github.com/CCoupel/BuzzMaster/blob/main/backlog/DONE/tri-rapidite-reponse.md)
BODY
)"

create_done_issue \
    "Affichage logs serveur en temps réel (WebSocket)" \
    "v2.43.0" \
    "enhancement,backlog,DONE,backend,frontend" \
    "$(cat <<'BODY'
## Complété en v2.43.0

Affichage des logs serveur en temps réel via WebSocket dédiée.

📄 Spec : [`backlog/DONE/page-logs.md`](https://github.com/CCoupel/BuzzMaster/blob/main/backlog/DONE/page-logs.md)
BODY
)"

create_done_issue \
    "Mode démonstration avec données complètes" \
    "v2.40.0" \
    "enhancement,backlog,DONE,backend" \
    "$(cat <<'BODY'
## Complété en v2.40.0

Mode démonstration avec données complètes pour les présentations.

📄 Spec : [`backlog/DONE/mode-demo.md`](https://github.com/CCoupel/BuzzMaster/blob/main/backlog/DONE/mode-demo.md)
BODY
)"

create_done_issue \
    "Indices automatiques pour QCM avec pénalités" \
    "v2.38.0" \
    "enhancement,backlog,DONE,backend,frontend" \
    "$(cat <<'BODY'
## Complété en v2.38.0

Indices automatiques pour QCM avec pénalités progressives.

📄 Spec : [`backlog/DONE/qcm-indices-penalites.md`](https://github.com/CCoupel/BuzzMaster/blob/main/backlog/DONE/qcm-indices-penalites.md)
BODY
)"

create_done_issue \
    "Système de catégorisation et palmarès" \
    "v2.34.0" \
    "enhancement,backlog,DONE,backend,frontend" \
    "$(cat <<'BODY'
## Complété en v2.34.0

Système de catégorisation des questions et palmarès.

📄 Spec : [`backlog/DONE/categories-questions.md`](https://github.com/CCoupel/BuzzMaster/blob/main/backlog/DONE/categories-questions.md)
BODY
)"

create_done_issue \
    "Synchronisation des fonds d'écran (TV)" \
    "v2.30.0" \
    "enhancement,backlog,DONE,frontend" \
    "$(cat <<'BODY'
## Complété en v2.30.0

Synchronisation des fonds d'écran sur l'affichage TV.

📄 Spec : [`backlog/DONE/affichage-tv.md`](https://github.com/CCoupel/BuzzMaster/blob/main/backlog/DONE/affichage-tv.md)
BODY
)"

create_done_issue \
    "Décompte de préparation avant timer" \
    "v2.29.0" \
    "enhancement,backlog,DONE,backend,frontend" \
    "$(cat <<'BODY'
## Complété en v2.29.0

Décompte de préparation avant le timer du jeu.

📄 Spec : [`backlog/DONE/timer-gameplay.md`](https://github.com/CCoupel/BuzzMaster/blob/main/backlog/DONE/timer-gameplay.md)
BODY
)"

create_done_issue \
    "Fonctionnalités de test sans buzzers" \
    "v2.28.0" \
    "enhancement,backlog,DONE,backend" \
    "$(cat <<'BODY'
## Complété en v2.28.0

Fonctionnalités de debug/test sans buzzers physiques.

📄 Spec : [`backlog/DONE/debug-tests.md`](https://github.com/CCoupel/BuzzMaster/blob/main/backlog/DONE/debug-tests.md)
BODY
)"

create_done_issue \
    "Points d'équipe dissociés des points joueurs" \
    "v2.18.0" \
    "enhancement,backlog,DONE,backend" \
    "$(cat <<'BODY'
## Complété en v2.18.0

Points d'équipe dissociés des points joueurs individuels.

📄 Spec : [`backlog/DONE/gestion-scores.md`](https://github.com/CCoupel/BuzzMaster/blob/main/backlog/DONE/gestion-scores.md)
BODY
)"

# ============================================================================
# ISSUE REMOVED
# ============================================================================
echo ""
echo "🗑️ Création de l'issue REMOVED..."

url=$(gh issue create --repo "$REPO" \
    --title "[Abandonné] Provisionnement WiFi buzzers via SmartConfig" \
    --label "backlog,REMOVED" \
    --body "$(cat <<'BODY'
## Abandonné

Provisionnement WiFi des buzzers via ESP-Touch SmartConfig intégré au mode ENROLL.

📄 Spec : [`backlog/REMOVED/buzzer-wifi-provisioning-smartconfig.md`](https://github.com/CCoupel/BuzzMaster/blob/main/backlog/REMOVED/buzzer-wifi-provisioning-smartconfig.md)

**Raison de l'abandon** : Remplacé par la configuration WiFi directe USB depuis l'interface Admin (v3.0.x). La solution USB est plus fiable et plus simple à utiliser.
BODY
)")
issue_num=$(echo "$url" | grep -oP '\d+$')
if [ -n "$issue_num" ]; then
    gh issue close "$issue_num" --repo "$REPO" --reason "not planned" --comment "Abandonné : remplacé par config WiFi USB directe (v3.0.x)" 2>/dev/null || true
fi
echo "✅ Issue créée et fermée (not planned)"

echo ""
echo "==========================================================="
echo "🎉 Toutes les issues ont été créées avec succès !"
echo ""
echo "Résumé :"
echo "  - 4 issues TODO (ouvertes)"
echo "  - 1 issue En-Cours (ouverte)"
echo "  - 18 issues DONE (fermées)"
echo "  - 1 issue REMOVED (fermée, not planned)"
echo ""
echo "📋 Voir : https://github.com/$REPO/issues"

# BuzzControl - Backlog

Fonctionnalités à implémenter.

---

## Gestion des scores

- [x] **Points d'équipe dissociés des points joueurs** *(v2.18.0)*
  - Champ `TEAM_POINTS` sur les équipes
  - Score total = TEAM_POINTS + sum(player scores)
  - Clic sur header équipe = points à l'équipe
  - Clic sur ligne joueur = points au joueur

---

## Catégories de questions

- [x] **Champ CATEGORY pour les questions** *(implémenté)*
  - Ajouter un champ `CATEGORY` au modèle Question
  - UI pour sélectionner/créer une catégorie lors de l'ajout de question
  - Filtrage des questions par catégorie dans QuestionsPage

- [x] **Palmarès par catégorie** *(v2.34.0)*
  - Page admin `/palmares` avec classement par catégorie
  - Vue TV Palmares avec grille 3x2 des catégories
  - Classement séparé équipes/joueurs avec médailles

---

## Timer et gameplay

- [x] **Décompte de 3 secondes avant le timer** *(v2.29.0)*
  - Décompte visuel "3... 2... 1..." avant le timer principal
  - Phase COUNTDOWN distincte avec badge orange "DECOMPTE"
  - Les buzzers restent bloqués pendant le décompte
  - Le timer démarre automatiquement après le décompte

---

## QCM - Indices et pénalités

> **En cours d'implémentation** - v2.38.0

### Configuration

- [ ] **Option activable par question QCM**
  - Champ `QCM_HINTS_ENABLED` (boolean, défaut: false)
  - Visible uniquement pour les questions de type QCM
  - Toggle dans le formulaire de création/édition de question

### Invalidation automatique des mauvaises réponses

- [ ] **Logique d'invalidation (Backend)**
  - Si aucun joueur n'a buzzé, invalider une mauvaise réponse aux seuils configurés
  - L'invalidation est aléatoire parmi les mauvaises réponses restantes
  - **Seuils par défaut (proportionnels au timer) :**
    - Seuil 1 (1er indice) : 25% du temps restant
    - Seuil 2 (2ème indice) : 12.5% du temps restant
  - **Contraintes de sécurité :**
    - Minimum 1s entre les deux indices
    - Seuil 2 ≥ 1s avant la fin du jeu
    - Si ces contraintes ne peuvent être respectées → pas d'indices
  - **Exemples :**
    | Timer | Seuil 1 | Seuil 2 | Notes |
    |-------|---------|---------|-------|
    | 30s   | 7.5s    | 3.75s   | OK |
    | 20s   | 5s      | 2.5s    | OK |
    | 10s   | 2.5s    | 1.25s   | OK |
    | 4s    | 1s      | —       | 1 seul indice possible |
    | 2s    | —       | —       | Pas d'indices |

- [ ] **Affichage TV (Frontend)**
  - Réponse invalidée : visuellement barrée/grisée
  - Animation de transition lors de l'invalidation
  - État `QCM_INVALIDATED` dans GameState : liste des couleurs invalidées

- [ ] **Broadcast WebSocket**
  - Action `QCM_HINT` : notifie les clients d'une invalidation
  - Payload : `{COLOR: "RED|GREEN|YELLOW|BLUE"}`

### Pénalités de points

- [ ] **Calcul des pénalités (Backend)**
  - Si un joueur buzz après invalidation(s), ses points sont réduits
  - **Ratio de pénalité :**
    - 4 réponses (aucune invalidée) → 100% des points
    - 3 réponses (1 invalidée) → 67% des points
    - 2 réponses (2 invalidées) → 33% des points
  - Calcul : `points_effectifs = points_base × (réponses_restantes / 4)`

- [ ] **Affichage admin (Frontend)**
  - Indicateur de pénalité applicable sur GamePage
  - Badge "67%" ou "33%" à côté des points si pénalité active

- [ ] **Historique**
  - L'historique enregistre les points effectivement attribués (après pénalité)
  - Champ optionnel : `PenaltyApplied` (pourcentage de réduction)

---

## Debug et tests

- [x] **Ctrl+Click sur joueur en PREPARE simule PONG** *(v2.28.0)*
  - En état PREPARE, Ctrl+Click sur un joueur simule la réponse au PING
  - Permet de tester sans buzzers physiques connectés
  - Le joueur passe de "en attente" à "prêt"

---

## Affichage TV

- [x] **Synchronisation des changements d'image de fond** *(v2.30.0)*
  - Le serveur centralise le timing et notifie tous les clients
  - `CurrentBackgroundIndex` dans GameState (backend)
  - Goroutine de cycling dans main.go
  - Action `BACKGROUND_CHANGE` dans le protocole WebSocket
  - Tous les clients TV reçoivent l'index synchronisé
  - Transitions simultanées sur tous les écrans

---

## Type de jeu : Memory

Jeu de mémoire avec paires de cartes à retrouver.

### Phase 1 - Modèle et création de question ✅

- [x] **Nouveau type de question `MEMORY`**
  - Champ `TYPE: "MEMORY"` dans le modèle Question
  - Structure `MEMORY_PAIRS` : tableau de paires `[{id, card1, card2}]`
  - Chaque carte peut être : texte OU image (chemin)
  - Paramètres configurables :
    - `MEMORY_FLIP_DELAY` : délai avant retournement si non-match (défaut: 3s)
    - `MEMORY_POINTS_PER_PAIR` : points par paire trouvée (défaut: 10)
    - `MEMORY_ERROR_PENALTY` : pénalité par erreur (défaut: 0)
    - `MEMORY_COMPLETION_BONUS` : bonus si toutes les paires trouvées (défaut: 0)

- [x] **Interface de création de paires (QuestionsPage)**
  - Sélecteur type "MEMORY" affiche l'éditeur de paires
  - Liste des paires avec boutons +/- pour ajouter/supprimer
  - Chaque paire : 2 inputs (texte ou upload image)
  - Preview de la grille générée automatiquement
  - Validation : minimum 2 paires, maximum 12 paires

### Phase 2 - État du jeu Memory et Affichage TV ✅

- [x] **Structure Memory dans GameState**
  - `MemoryFlippedCards []string` : IDs des cartes retournées (max 2)
  - `MemoryMatchedPairs []int` : IDs des paires trouvées
  - `MemoryErrors int` : compteur d'erreurs (non-matches)

- [x] **Affichage TV (PlayerDisplay)**
  - Grille responsive avec Container Queries (cqw, cqh, cqmin)
  - Animation flip 3D CSS sur les cartes
  - Colonnes automatiques selon nombre de cartes (2-6 colonnes)
  - États visuels : dos (violet), révélée, matched (vert)
  - Mélange Fisher-Yates avec seed basé sur question ID

### Phase 3 - Gameplay interactif ✅

- [x] **Action `FLIP_MEMORY_CARD` (Admin/TV → Serveur)**
  - Payload : `{CARD_ID: string}` (format "pairID-cardNum")
  - Le serveur valide et met à jour l'état
  - Broadcast de l'état aux clients TV

- [x] **Logique de révélation (engine.go:FlipMemoryCard)**
  - Si 0 carte révélée → révéler la carte, attendre la 2ème
  - Si 1 carte révélée → révéler la 2ème, vérifier le match
  - Si match → marquer les 2 cartes comme MATCHED, incrémenter compteur
  - Si non-match → incrémenter erreurs, démarrer timer (FLIP_DELAY), puis cacher

- [x] **Détection de fin de partie**
  - Toutes les paires trouvées → auto-stop game, transition vers STOPPED
  - Timer global épuisé (si configuré) → fin avec points partiels

- [x] **Affichage statistiques pendant le jeu**
  - Paires trouvées X/Y
  - Erreurs Z (si penalty ou erreurs > 0)

### Phase 4 - Interface Admin (GamePage)

- [x] **Indicateurs Memory en temps réel dans GamePage** *(implémenté)*
  - Paires trouvées X/Y, compteur d'erreurs
  - Badge de succès si toutes les paires sont trouvées

- [x] **Bouton "Révéler tout" pour Memory** *(implémenté)*
  - Le bouton "REPONSE" passe en phase REVEALED
  - Révèle toutes les cartes en cascade avec REVEAL_DELAY

### Phase 5 - Scoring et historique

- [x] **Calcul des points Memory** *(implémenté dans GamePage.jsx + engine.go)*
  ```
  Score = (paires_trouvées × POINTS_PER_PAIR)
        + (COMPLETION_BONUS si toutes trouvées)
        - (erreurs × ERROR_PENALTY)
  ```

- [ ] **Enregistrement spécifique dans l'historique**
  - EventType: "MEMORY_COMPLETED" (actuellement "POINTS_AWARDED")
  - Détails: paires trouvées, erreurs, temps total

### Améliorations futures (hors scope initial)

- [ ] **Mode Équipes** : les équipes buzzent pour désigner les cartes
- [ ] **Mode Chrono** : temps limité, max de paires en un temps donné
- [ ] **Thèmes de cartes** : dos de carte personnalisable
- [ ] **Types de paires mixtes** : Image ↔ Texte (association)
- [ ] **Niveaux de difficulté** : délai de retournement variable

---

## Générateur de jeu via IA

Outil/site web pour générer automatiquement un jeu complet BuzzMaster via une IA générative.

### Concept

L'utilisateur fournit des paramètres de jeu, et l'IA génère un fichier de backup (.tar) prêt à être importé dans BuzzMaster, contenant questions, médias, équipes, et configuration.

### Phase 1 - Core Generator

- [ ] **Interface de configuration**
  - Formulaire web avec les paramètres de génération :
    - **Population cible** : Junior (6-12 ans), Adolescent (13-17 ans), Adulte (18-64 ans), Senior (65+), Famille (multi-générationnel)
    - **Niveau de difficulté** : Facile, Moyen, Difficile, Expert
    - **Thème général** : Cinéma, Sport, Histoire, Sciences, Géographie, Culture générale, Musique, Jeux vidéo, Entreprise, Éducation, etc.
    - **Objectifs pédagogiques** (optionnel) : Formation professionnelle, révision scolaire, team building, animation événementielle, découverte culturelle
    - **Catégories souhaitées** : Sélection multiple avec suggestion auto basée sur le thème
    - **Volume de contenu** :
      - Nombre de questions (10, 20, 30, 50, 100)
      - OU durée estimée du jeu (30 min, 1h, 2h)
    - **Répartition des types de questions** :
      - Pourcentage QCM (0-100%)
      - Pourcentage Normal (0-100%)
      - Pourcentage Memory (0-100%)
      - Validation : total = 100%
    - **Langue** : Français (défaut), Anglais, Espagnol, Allemand, etc.

- [ ] **Backend générateur (Go ou Node.js)**
  - Intégration API LLM (Claude API via Anthropic, GPT-4, ou autre)
  - Génération structurée des questions avec validation JSON
  - Prompt engineering pour garantir la qualité et la cohérence
  - Gestion de la génération par lots (éviter timeouts)
  - Logging des générations pour debug et amélioration

- [ ] **Génération de contenu**
  - Questions normales : question + réponse + points + temps suggérés
  - Questions QCM : question + 4 réponses + bonne réponse
  - Questions Memory : paires de cartes textuelles pertinentes au thème
  - Attribution automatique des catégories
  - Équilibrage automatique entre catégories (via CategoryBalance)
  - Validation de la pertinence au thème et à la population cible

- [ ] **Export vers backup BuzzMaster**
  - Génération de la structure TAR compatible :
    - `config/teams.json` : 4-6 équipes prédéfinies avec couleurs
    - `files/questions/` : Dossiers de questions avec question.json
    - `config/history.json` : Vide ou avec données de démo
  - Téléchargement du fichier .tar
  - Instructions d'import dans BuzzMaster

### Phase 2 - Améliorations UX

- [ ] **Preview et édition avant export**
  - Affichage de toutes les questions générées dans une interface similaire à QuestionsPage
  - Possibilité de modifier/supprimer/réordonner les questions
  - Ajout manuel de questions supplémentaires
  - Régénération individuelle d'une question si insatisfaisante

- [ ] **Templates de jeu prédéfinis**
  - Quiz TV (style Questions pour un Champion)
  - Trivia Pub (atmosphère conviviale, questions variées)
  - Formation entreprise (questions métier spécifiques)
  - Révision scolaire (programmes scolaires par niveau)
  - Animation événementielle (questions légères et amusantes)
  - Chaque template pré-remplit certains paramètres

- [ ] **Métadonnées du jeu**
  - Nom du jeu (ex: "Quiz Cinéma 80s")
  - Auteur/créateur
  - Description courte
  - Tags pour recherche future
  - Date de création
  - Stockées dans un fichier `game_metadata.json` dans le backup

### Phase 3 - Génération de médias

- [ ] **Génération d'images via IA**
  - Intégration DALL-E 3, Stable Diffusion, ou Midjourney API
  - Génération automatique d'images pour les questions pertinentes
  - Génération d'images de réponse pour les révélations visuelles
  - Preview des images avant export
  - Possibilité de régénérer une image spécifique

- [ ] **Recherche d'images libres de droits**
  - Intégration API Unsplash, Pexels, Pixabay
  - Recherche automatique basée sur les mots-clés de la question
  - Sélection semi-automatique (IA choisit, utilisateur valide)
  - Attribution automatique des crédits si nécessaire

- [ ] **Images pour Memory**
  - Génération de paires d'images cohérentes pour les jeux Memory
  - Styles visuels adaptés à la population cible (cartoon pour juniors, photos pour adultes)

### Phase 4 - Architecture et déploiement

- [ ] **Options d'architecture**
  - **Option A - Site web externe** :
    - Frontend React/Vue.js hébergé séparément
    - Backend API (Go/Node.js) avec workers pour génération longue
    - Stockage temporaire des générations (S3, local disk)
    - Pas de dépendance avec BuzzMaster (génère juste le TAR)
  - **Option B - Intégré dans BuzzMaster** :
    - Nouvelle route `/generator` dans l'interface admin
    - Backend Go existant étendu avec endpoints de génération
    - Avantage : un seul outil, import direct sans téléchargement
    - Inconvénient : alourdit l'application principale
  - **Option C - CLI/Script** :
    - Outil en ligne de commande (Go binary)
    - Fichier de config YAML/JSON pour les paramètres
    - Génération locale, pas de serveur nécessaire
    - Idéal pour génération en masse ou scripting
  - **Option D - Service cloud SaaS** :
    - Plateforme hébergée avec comptes utilisateurs
    - Bibliothèque de jeux générés et partageables
    - Modèle freemium (X générations gratuites/mois)
    - Marketplace de jeux créés par la communauté

- [ ] **Gestion des coûts API**
  - Estimation du coût par génération (tokens LLM + images)
  - Système de crédits ou quotas si service payant
  - Cache des questions similaires pour réduire les appels API
  - Fallback sur modèles moins coûteux si possible

### Phase 5 - Qualité et personnalisation avancée

- [ ] **Validation de la qualité**
  - Vérification automatique des questions générées :
    - Cohérence question/réponse
    - Niveau de difficulté conforme à la cible
    - Pas de doublons
    - Orthographe et grammaire (API LanguageTool)
  - Score de qualité par question (0-100%)
  - Régénération automatique si score < seuil

- [ ] **Personnalisation avancée**
  - Import de contexte spécifique (PDF, texte) pour questions sur-mesure
  - Exemple : "Générer un quiz sur notre produit X à partir de ce manuel"
  - Extraction automatique des points clés du document
  - Génération de questions basées sur le contenu fourni

- [ ] **Historique et bibliothèque**
  - Sauvegarde des jeux générés (si compte utilisateur)
  - Possibilité de re-télécharger un jeu précédent
  - Partage de jeux entre utilisateurs (si mode collaboratif)
  - Import/fusion de jeux existants

### Phase 6 - Analytics et amélioration continue

- [ ] **Feedback utilisateur**
  - Rating des questions générées (1-5 étoiles)
  - Signalement de questions inappropriées ou incorrectes
  - Commentaires pour amélioration du prompt

- [ ] **Analytics des générations**
  - Thèmes les plus demandés
  - Taux de satisfaction par type de question
  - Durée moyenne de génération
  - Taux de régénération par question (indicateur de qualité)

- [ ] **Fine-tuning du modèle**
  - Si volume suffisant, entraîner un modèle spécialisé
  - Apprentissage des préférences utilisateurs
  - Amélioration continue des prompts

### Cas d'usage identifiés

| Cas d'usage | Exemple | Paramètres suggérés |
|-------------|---------|---------------------|
| **Anniversaire enfant** | Quiz Disney pour 10 ans | Junior, Facile, Cinéma/Dessins animés, 20 questions, 70% QCM |
| **Soirée entre amis** | Trivia années 90 | Adulte, Moyen, Culture générale, 50 questions, 60% QCM |
| **Formation entreprise** | Quiz sécurité informatique | Adulte, Difficile, Entreprise/IT, 30 questions, 50% QCM + 30% Normal |
| **Révision scolaire** | Histoire CM2 | Junior, Moyen, Histoire, 40 questions, 80% QCM |
| **Team building** | Quiz inter-services | Adulte, Facile, Entreprise/Culture, 25 questions, 50% QCM + 30% Memory |
| **Résidence seniors** | Nostalgie années 50-60 | Senior, Facile, Musique/Cinéma/Histoire, 30 questions, 40% QCM + 40% Memory |

### Technologies suggérées

| Composant | Technologies possibles |
|-----------|------------------------|
| **Frontend** | React + Vite, TailwindCSS, Framer Motion |
| **Backend** | Go (cohérence avec BuzzMaster), Node.js + Express (alternative) |
| **LLM API** | Anthropic Claude API (recommandé), OpenAI GPT-4, Mistral API |
| **Génération images** | DALL-E 3, Stable Diffusion XL, Midjourney (via proxy) |
| **Recherche images** | Unsplash API, Pexels API, Pixabay API |
| **Hosting** | Vercel/Netlify (frontend), Railway/Fly.io (backend), AWS/GCP (production) |
| **Storage** | S3-compatible (Backblaze B2, Cloudflare R2) pour backups temporaires |

### Exemple de prompt pour Claude API

```
Vous êtes un expert en création de quiz éducatifs et divertissants.

Contexte :
- Population cible : {population}
- Niveau de difficulté : {difficulty}
- Thème principal : {theme}
- Objectif : {objective}
- Langue : {language}

Consignes :
1. Générez {count} questions de type {type}
2. Répartissez équitablement entre les catégories : {categories}
3. Adaptez le vocabulaire et la complexité à la population cible
4. Pour les QCM, assurez-vous que les mauvaises réponses soient plausibles
5. Proposez des temps de réponse et points adaptés au niveau

Format de sortie JSON :
{
  "questions": [
    {
      "TYPE": "QCM",
      "CATEGORY": "HISTORY",
      "QUESTION": "En quelle année a eu lieu la Révolution française ?",
      "QCM_ANSWERS": {
        "RED": "1789",
        "GREEN": "1799",
        "YELLOW": "1776",
        "BLUE": "1804"
      },
      "QCM_CORRECT": "RED",
      "ANSWER": "1789",
      "POINTS": 10,
      "TIME": 20
    },
    ...
  ]
}
```

### Priorités de développement

**Court terme (MVP)** :
- Phase 1 : Core Generator (formulaire + génération basique + export TAR)
- Option d'architecture : Site web externe (indépendant)

**Moyen terme** :
- Phase 2 : Preview et templates
- Phase 3 : Génération d'images (recherche Unsplash d'abord)

**Long terme** :
- Phase 4 : SaaS avec comptes utilisateurs
- Phase 5 : Personnalisation avancée avec import de documents
- Phase 6 : Analytics et fine-tuning

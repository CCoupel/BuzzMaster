# Générateur de jeu via IA

**Statut** : 📋 Planifié

## Concept

Outil/site web pour générer automatiquement un jeu complet BuzzMaster via une IA générative.

L'utilisateur fournit des paramètres de jeu, et l'IA génère un fichier de backup (.tar) prêt à être importé dans BuzzMaster, contenant questions, médias, équipes, et configuration.

---

## Phase 1 - Core Generator

### Interface de configuration

- [ ] **Formulaire web avec les paramètres de génération :**
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

### Backend générateur (Go ou Node.js)

- [ ] **Intégration API LLM**
  - Intégration API LLM (Claude API via Anthropic, GPT-4, ou autre)
  - Génération structurée des questions avec validation JSON
  - Prompt engineering pour garantir la qualité et la cohérence
  - Gestion de la génération par lots (éviter timeouts)
  - Logging des générations pour debug et amélioration

### Génération de contenu

- [ ] **Questions et équilibrage**
  - Questions normales : question + réponse + points + temps suggérés
  - Questions QCM : question + 4 réponses + bonne réponse
  - Questions Memory : paires de cartes textuelles pertinentes au thème
  - Attribution automatique des catégories
  - Équilibrage automatique entre catégories (via CategoryBalance)
  - Validation de la pertinence au thème et à la population cible

### Export vers backup BuzzMaster

- [ ] **Génération de la structure TAR compatible**
  - Génération de la structure TAR compatible :
    - `config/teams.json` : 4-6 équipes prédéfinies avec couleurs
    - `files/questions/` : Dossiers de questions avec question.json
    - `config/history.json` : Vide ou avec données de démo
  - Téléchargement du fichier .tar
  - Instructions d'import dans BuzzMaster

---

## Phase 2 - Améliorations UX

### Preview et édition avant export

- [ ] **Interface d'édition**
  - Affichage de toutes les questions générées dans une interface similaire à QuestionsPage
  - Possibilité de modifier/supprimer/réordonner les questions
  - Ajout manuel de questions supplémentaires
  - Régénération individuelle d'une question si insatisfaisante

### Templates de jeu prédéfinis

- [ ] **Bibliothèque de templates**
  - Quiz TV (style Questions pour un Champion)
  - Trivia Pub (atmosphère conviviale, questions variées)
  - Formation entreprise (questions métier spécifiques)
  - Révision scolaire (programmes scolaires par niveau)
  - Animation événementielle (questions légères et amusantes)
  - Chaque template pré-remplit certains paramètres

### Métadonnées du jeu

- [ ] **Informations descriptives**
  - Nom du jeu (ex: "Quiz Cinéma 80s")
  - Auteur/créateur
  - Description courte
  - Tags pour recherche future
  - Date de création
  - Stockées dans un fichier `game_metadata.json` dans le backup

---

## Phase 3 - Génération de médias

### Génération d'images via IA

- [ ] **API de génération d'images**
  - Intégration DALL-E 3, Stable Diffusion, ou Midjourney API
  - Génération automatique d'images pour les questions pertinentes
  - Génération d'images de réponse pour les révélations visuelles
  - Preview des images avant export
  - Possibilité de régénérer une image spécifique

### Recherche d'images libres de droits

- [ ] **API d'images stock**
  - Intégration API Unsplash, Pexels, Pixabay
  - Recherche automatique basée sur les mots-clés de la question
  - Sélection semi-automatique (IA choisit, utilisateur valide)
  - Attribution automatique des crédits si nécessaire

### Images pour Memory

- [ ] **Paires d'images cohérentes**
  - Génération de paires d'images cohérentes pour les jeux Memory
  - Styles visuels adaptés à la population cible (cartoon pour juniors, photos pour adultes)

---

## Phase 4 - Architecture et déploiement

### Options d'architecture

- [ ] **Option A - Site web externe**
  - Frontend React/Vue.js hébergé séparément
  - Backend API (Go/Node.js) avec workers pour génération longue
  - Stockage temporaire des générations (S3, local disk)
  - Pas de dépendance avec BuzzMaster (génère juste le TAR)

- [ ] **Option B - Intégré dans BuzzMaster**
  - Nouvelle route `/generator` dans l'interface admin
  - Backend Go existant étendu avec endpoints de génération
  - Avantage : un seul outil, import direct sans téléchargement
  - Inconvénient : alourdit l'application principale

- [ ] **Option C - CLI/Script**
  - Outil en ligne de commande (Go binary)
  - Fichier de config YAML/JSON pour les paramètres
  - Génération locale, pas de serveur nécessaire
  - Idéal pour génération en masse ou scripting

- [ ] **Option D - Service cloud SaaS**
  - Plateforme hébergée avec comptes utilisateurs
  - Bibliothèque de jeux générés et partageables
  - Modèle freemium (X générations gratuites/mois)
  - Marketplace de jeux créés par la communauté

### Gestion des coûts API

- [ ] **Optimisation des coûts**
  - Estimation du coût par génération (tokens LLM + images)
  - Système de crédits ou quotas si service payant
  - Cache des questions similaires pour réduire les appels API
  - Fallback sur modèles moins coûteux si possible

---

## Phase 5 - Qualité et personnalisation avancée

### Validation de la qualité

- [ ] **Vérification automatique des questions générées**
  - Cohérence question/réponse
  - Niveau de difficulté conforme à la cible
  - Pas de doublons
  - Orthographe et grammaire (API LanguageTool)
  - Score de qualité par question (0-100%)
  - Régénération automatique si score < seuil

### Personnalisation avancée

- [ ] **Import de contexte spécifique**
  - Import de contexte spécifique (PDF, texte) pour questions sur-mesure
  - Exemple : "Générer un quiz sur notre produit X à partir de ce manuel"
  - Extraction automatique des points clés du document
  - Génération de questions basées sur le contenu fourni

### Historique et bibliothèque

- [ ] **Gestion des générations**
  - Sauvegarde des jeux générés (si compte utilisateur)
  - Possibilité de re-télécharger un jeu précédent
  - Partage de jeux entre utilisateurs (si mode collaboratif)
  - Import/fusion de jeux existants

---

## Phase 6 - Analytics et amélioration continue

### Feedback utilisateur

- [ ] **Système de notation**
  - Rating des questions générées (1-5 étoiles)
  - Signalement de questions inappropriées ou incorrectes
  - Commentaires pour amélioration du prompt

### Analytics des générations

- [ ] **Statistiques d'utilisation**
  - Thèmes les plus demandés
  - Taux de satisfaction par type de question
  - Durée moyenne de génération
  - Taux de régénération par question (indicateur de qualité)

### Fine-tuning du modèle

- [ ] **Amélioration continue**
  - Si volume suffisant, entraîner un modèle spécialisé
  - Apprentissage des préférences utilisateurs
  - Amélioration continue des prompts

---

## Cas d'usage identifiés

| Cas d'usage | Exemple | Paramètres suggérés |
|-------------|---------|---------------------|
| **Anniversaire enfant** | Quiz Disney pour 10 ans | Junior, Facile, Cinéma/Dessins animés, 20 questions, 70% QCM |
| **Soirée entre amis** | Trivia années 90 | Adulte, Moyen, Culture générale, 50 questions, 60% QCM |
| **Formation entreprise** | Quiz sécurité informatique | Adulte, Difficile, Entreprise/IT, 30 questions, 50% QCM + 30% Normal |
| **Révision scolaire** | Histoire CM2 | Junior, Moyen, Histoire, 40 questions, 80% QCM |
| **Team building** | Quiz inter-services | Adulte, Facile, Entreprise/Culture, 25 questions, 50% QCM + 30% Memory |
| **Résidence seniors** | Nostalgie années 50-60 | Senior, Facile, Musique/Cinéma/Histoire, 30 questions, 40% QCM + 40% Memory |

---

## Technologies suggérées

| Composant | Technologies possibles |
|-----------|------------------------|
| **Frontend** | React + Vite, TailwindCSS, Framer Motion |
| **Backend** | Go (cohérence avec BuzzMaster), Node.js + Express (alternative) |
| **LLM API** | Anthropic Claude API (recommandé), OpenAI GPT-4, Mistral API |
| **Génération images** | DALL-E 3, Stable Diffusion XL, Midjourney (via proxy) |
| **Recherche images** | Unsplash API, Pexels API, Pixabay API |
| **Hosting** | Vercel/Netlify (frontend), Railway/Fly.io (backend), AWS/GCP (production) |
| **Storage** | S3-compatible (Backblaze B2, Cloudflare R2) pour backups temporaires |

---

## Exemple de prompt pour Claude API

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

---

## Priorités de développement

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

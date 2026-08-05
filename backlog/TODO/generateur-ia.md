# Générateur de questions via IA

**Statut** : 📋 Planifié

## Concept

Outil intégré dans BuzzControl pour générer automatiquement des **questions** (pas les équipes) via l'API Claude (Anthropic), permettant de compléter/enrichir une partie existante avant l'événement.

---

## Architecture

### Intégration
- **Bouton "Générer via IA"** dans la page admin Quiz (QuestionsPage)
- **Backend Go** appelle directement l'API Claude
- **Phase de préparation** (avant l'événement, pas en direct pendant une partie)
- Nécessite un accès réseau externe
- Option grisée/désactivée si :
  - Accès externe non disponible
  - Aucune clé API configurée

### Authentification & Coûts
- **BYOK (Bring Your Own Key)** : chaque opérateur configure sa propre clé API Claude dans une page Paramètres
- **Provider unique** : Claude API uniquement (pas de multi-provider, pas de LLM gratuit)
- **Abonnement Claude Pro/Max non utilisable** : l'accès API programmatique est distinct de claude.ai
- Pas de système de quota interne — géré côté compte Anthropic de l'opérateur
- **Gestion d'erreurs minimale** :
  - Clé absente/vide → option grisée + message "Configurer une clé API dans Paramètres"
  - Clé invalide/quota dépassé → erreur affichée, pas de crash

---

## Formulaire de génération

| Champ | Type | Détail |
|---|---|---|
| **Population cible** | Select | Junior (6-12) / Ado (13-17) / Adulte (18-64) / Senior (65+) / Famille |
| **Difficulté** | Multi-select | Facile / Moyen / Difficile / Expert — mix possible. Chaque question a `POINTS` cohérent : Facile=10, Moyen=20, Difficile=30, Expert=50 |
| **Thème général** | Texte libre | ex. "Cinéma français des années 80" — pas de liste prédéfinie |
| **Objectifs pédagogiques** | Texte libre, optionnel | ex. "révision scolaire", "team building" |
| **Catégories cibles** | Multi-select | **Uniquement parmi les catégories existantes** du jeu — le LLM ne crée jamais de catégorie |
| **Volume** | Nombre OU durée cible | Nombre de questions OU durée de partie cible. Si durée : le LLM décide du nombre de questions ET du `TIME` (temps de réponse) par question pour coller approximativement à la durée demandée |
| **Répartition par type** | 4 sliders % | SPEEDY / QCM / MEMORY / MEMOTION (types réels de `server-go/internal/game/models.go`). Rebalance auto quand on bouge un slider. Toggle par type pour le désactiver complètement (exclu du rebalance, verrouillé à 0%) |
| **Langue** | Select | Français par défaut |

**Note** : `ARDOISE` n'est pas un type generable (mode d'affichage). `PRESENTATION` (issue #119) à ajouter si #119 livré avant ce chantier.

---

## Génération et injection

### Contexte injecté automatiquement
La liste des questions déjà existantes dans les catégories ciblées est transmise au LLM dans le prompt — uniquement à titre informationnel, pour éviter les doublons de thème/formulation et permettre un affinage itératif (l'admin peut relancer une génération après correction/suppression de questions).

### Comportement — additif uniquement
- **Pas d'écran de relecture** avant injection — l'édition/suppression existante de QuestionsPage sert de mécanisme de correction
- **Sortie structurée** : le LLM répond en JSON structuré (structured outputs / `output_config.format`), directement dans le schéma question de BuzzControl
- **Pas de multipart** : ne passe pas par l'endpoint `POST /questions` existant (conçu pour un formulaire humain avec upload média), un nouveau chemin d'écriture interne consomme directement ce JSON
- **Garantie de non-modification** : le chemin d'import crée **seulement** de nouveaux fichiers `question.json` avec des IDs fraîchement alloués (réutiliser la logique d'allocation d'ID de `handleUploadQuestion` dans `server-go/internal/server/http.go`)
  - **Aucun moyen structurel** pour que la réponse du LLM référence un ID existant en update/delete — l'architecture rend cela impossible par construction
  - Même si le LLM hallucine une intention de modifier l'existant, le code l'empêche
- **Régénération** = nouvel appel avec mêmes paramètres ou ajustés, remplace un système d'undo/batch dédié

---

## Contenu généré — types de questions

Questions **texte-only**. Claude ne génère pas d'images (pas un modèle de diffusion). Seules options réalistes (recherche web, génération DALL-E) explicitement écartées pour le MVP — ajoutent une complexité/dépendance externe disproportionnée. L'admin peut ajouter une image manuellement après coup via l'upload existant de QuestionsPage.

### Types supportés

| Type | Format de sortie LLM |
|------|---------------------|
| **SPEEDY** | `{"TYPE": "SPEEDY", "QUESTION": "...", "ANSWER": "...", "POINTS": N, "TIME": N}` |
| **QCM** | `{"TYPE": "QCM", "QUESTION": "...", "QCM_ANSWERS": {"RED": "...", "GREEN": "...", "YELLOW": "...", "BLUE": "..."}, "QCM_CORRECT": "RED", "POINTS": N, "TIME": N}` |
| **MEMORY** | `{"TYPE": "MEMORY", "PAIRS": [{"TEXT": "...", "CORRECT": "..."}, ...], "POINTS": N, "TIME": N}` |
| **MEMOTION** | `{"TYPE": "MEMOTION", "PAIRS": [{"EMOTION": "happy", "TEXT": "..."}, ...], "POINTS": N, "TIME": N}` |

---

## Cas d'usage identifiés

| Cas d'usage | Exemple | Paramètres suggérés |
|-------------|---------|---------------------|
| **Anniversaire enfant** | Quiz Disney pour 10 ans | Junior, Facile, Cinéma/Dessins animés, 20 questions, 70% QCM |
| **Soirée entre amis** | Trivia années 90 | Adulte, Moyen, Culture générale, 50 questions, 60% QCM |
| **Formation entreprise** | Quiz sécurité informatique | Adulte, Difficile, Entreprise/IT, 30 questions, 50% QCM + 30% SPEEDY |
| **Révision scolaire** | Histoire CM2 | Junior, Moyen, Histoire, 40 questions, 80% QCM |
| **Team building** | Quiz inter-services | Adulte, Facile, Entreprise/Culture, 25 questions, 50% QCM + 30% MEMORY |
| **Résidence seniors** | Nostalgie années 50-60 | Senior, Facile, Musique/Cinéma/Histoire, 30 questions, 40% QCM + 40% MEMORY |

---

## Explicitement hors scope MVP

- ❌ Génération/gestion des équipes
- ❌ Images (génération ou recherche)
- ❌ Multi-provider LLM, abonnement Claude Pro/Max
- ❌ Export TAR / réimport (remplacé par injection directe)
- ❌ Templates de jeu prédéfinis, personnalisation avancée
- ❌ Import de contexte PDF/document
- ❌ Système de rating/feedback
- ❌ Analytics d'usage
- ❌ Fine-tuning

---

## Implémentation

### Backend Go

1. **Nouvelle route HTTP** : `POST /api/generate-questions`
   - Accepte les paramètres du formulaire
   - Valide la clé API Claude existante
   - Appelle l'API Claude avec structured outputs
   - Crée de nouveaux fichiers `question.json` dans `data/files/questions/<category>/`
   - Retourne la liste des IDs créés

2. **Page Paramètres / ConfigPage**
   - À vérifier si existe déjà
   - Champ de saisie : "Clé API Claude"
   - Stockage sécurisé (config.json chiffré ou env var)
   - Indication visuelle : clé configurée/manquante

3. **Allocation d'IDs** : réutiliser la logique de `handleUploadQuestion`

### Frontend React

1. **Bouton dans QuestionsPage**
   - Grisé si pas de clé API configurée
   - Tooltip : "Configurer une clé API dans Paramètres"

2. **Modale/formulaire de génération**
   - Champs décrits ci-dessus
   - Validation côté client
   - Affichage du statut en cours de génération (spinner + "Génération en cours...")

3. **Résultat**
   - Liste des questions créées
   - Redirection automatique vers la liste refresh (ou scroll vers les nouvelles questions)

---

## Configuration

Configuration API Claude à définir dans `.claude/project-config.json` ou docs spécifiques si détails d'implémentation requis.


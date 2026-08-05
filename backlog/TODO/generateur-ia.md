# Générateur de questions via IA

**Statut** : 📋 Planifié — MVP v6.0.x

## Concept

Bouton **"✨ Générer via IA"** intégré à BuzzControl pour générer automatiquement des **questions** via l'API Claude (Anthropic), permettant de compléter/enrichir une partie existante avant l'événement.

---

## Architecture

### Intégration UI
- **Placement** : Bouton "✨ Générer via IA" dans QuestionsPage, intégré dans la carte sidebar "Nouvelle Question" existante, à côté du titre
- **Popup de génération** : modale avec 2 blocs visuellement distincts
  - Bloc "Paramètres du Quiz" (haut) : champs globaux pré-remplis
  - Bloc "Cette génération" (bas) : paramètres éditables pour la génération courante uniquement
  - Footer : Annuler / Générer

**Maquette de référence** : https://claude.ai/code/artifact/0f2bfd0b-a6b6-4780-abf6-127f3d9e827a

### Backend & API
- Backend Go appelle directement l'API Claude
- Phase de préparation uniquement (avant l'événement, pas en direct pendant une partie)
- Nécessite accès réseau externe
- Bouton grisé/désactivé si :
  - Pas de clé API configurée
  - Pas d'accès réseau externe
  - Avec message explicite : "Configurer une clé API dans Paramètres"

### Prérequis technique à traiter AVANT le dev IA

**⚠️ Bug critique dans `server-go/internal/server/http.go:handleConfig` (~L1056-1102)** :

Le handler POST `/config.json` fait :
```go
var cfg config.Config      // struct vide !
json.Unmarshal(body, &cfg) // décode le payload
// sauvegarde cfg partiel → config.json écrase tout au disque
```

Or ConfigPage envoie des payloads partiels (ConfigSaveNeonConfig → `{neon_effect: ...}` uniquement, ConfigSaveServerParams → `{server: ...}` uniquement). Résultat : sauvegarder un réglage écrase toutes les autres sections à zéro sur disque.

**Fix avant d'ajouter la section `ai` à config.json** : charger `config.Get()` existant dans `cfg` **avant** l'`Unmarshal`, au lieu de partir d'un struct vide. Sinon la clé API sera effacée par n'importe quelle autre sauvegarde de Paramètres (et vice-versa).

---

## Authentification & Coûts

### Configuration
- **BYOK (Bring Your Own Key)** : clé API Claude configurée dans **ConfigPage** (`server-go/web/src/pages/ConfigPage.jsx`)
- **Stockage** : nouvelle section dans `config.json` sous clé top-level `"ai"` (ex. `{"ai": {"anthropic_api_key": "..."}}`), suivant le pattern existant de `NeonEffectConfig` dans `server-go/internal/config/config.go`
- Premier champ de type "secret" applicatif dans ce fichier (le mot de passe WiFi y est déjà en clair)

### Provider & quotas
- **Provider unique** : Claude API uniquement
- **Pas d'abonnement Pro/Max** : accès API programmatique ≠ abonnement claude.ai (contrainte technique confirmée) — ne pas explorer cette piste
- Pas de multi-provider, pas de LLM gratuit pour le MVP
- Pas de gestion de quota interne — géré côté compte Anthropic de l'opérateur

### Gestion d'erreurs
- Clé absente/vide → bouton grisé + message "Configurer une clé API dans Paramètres"
- Clé invalide/quota dépassé au moment de l'appel → erreur affichée, pas de crash

---

## Champs globaux au jeu — Étendre QUIZ_META

`server-go/internal/game/models.go` a déjà `QuizName`/`QuizTheme`/`QuizNotes` (v4.0.0, champs `QUIZ_NAME`/`QUIZ_THEME`/`QUIZ_NOTES` sur `GameState`, sans `omitempty`). Éditables dans la section "Quiz" en haut de QuestionsPage, affichés sur TV à l'écran NEW_GAME. Action WS `UPDATE_QUIZ_META`, payload `QuizMetaPayload` (`protocol/messages.go:216-220`).

### À étendre (même mécanisme : GameState + WS action + affichage TV)

| Champ | Type | Valeur par défaut | Affichage TV |
|---|---|---|---|
| `QUIZ_POPULATION` | Select | — | Écran NEW_GAME |
| `QUIZ_DIFFICULTY` | Select | — | Écran NEW_GAME |
| `QUIZ_LANGUAGE` | Select | Français | Écran NEW_GAME |

**Valeurs** :
- `QUIZ_POPULATION` : Junior (6-12) / Ado (13-17) / Adulte (18-64) / Senior (65+) / Famille
- `QUIZ_DIFFICULTY` : Facile / Moyen / Difficile / Expert (valeur **unique** au niveau global, cohérent avec Thème/Population/Langue qui sont des chaînes uniques)
- `QUIZ_LANGUAGE` : Français par défaut, autres à prévoir

### Utilisation
Ces 4 champs globaux (Thème inclus) sont **pré-remplis dans le formulaire de génération IA**, éditables/override pour une génération précise **sans modifier le global** (sauf action explicite).

---

## Formulaire de génération

**Comportement** : Jamais persisté, remis à zéro à chaque ouverture de modale.

### Bloc "Paramètres du Quiz" (pré-rempli depuis les globaux)

| Champ | Type | Source |
|---|---|---|
| Thème | Texte | `QUIZ_THEME` global |
| Population | Select | `QUIZ_POPULATION` global |
| Langue | Select | `QUIZ_LANGUAGE` global |
| Difficulté globale (info) | Select | `QUIZ_DIFFICULTY` global (affichage uniquement) |

### Bloc "Cette génération" (paramètres éditables)

| Champ | Type | Détail |
|---|---|---|
| **Difficulté** | Multi-select (cases) | **Pré-coché sur la valeur globale**, mais multi-select possible ici pour mixer les niveaux dans un même lot. Chaque question générée reçoit `POINTS` cohérent (ex. Facile=10/Moyen=20/Difficile=30/Expert=50) |
| **Objectifs—consignes** | Texte libre, optionnel | Champ unique fusionnant objectif d'usage (ex. "révision scolaire") ET consignes libres (ex. "éviter tel sujet", "ton humoristique") — pas deux champs séparés |
| **Catégories cibles** | Multi-select | Uniquement catégories existantes — le LLM ne crée jamais de catégorie |
| **Volume** | Toggle : "Nombre" OU "Durée" | **Mode "Nombre"** : admin fixe le compte. **Mode "Durée"** : LLM décide le nombre de questions pour approcher la durée cible (pas de calcul côté backend). **Dans TOUS les cas**, le `TIME` (temps de réponse) de chaque question est déterminé par le LLM selon type/difficulté/population |
| **Répartition par type** | 4 sliders % | SPEEDY / QCM / MEMORY / MEMOTION (types réels de `models.go:157-160`). Rebalance auto au déplacement d'un slider. Toggle par type pour désactiver complètement (verrouillé à 0%, exclu du rebalance) |

**Notes** :
- `ARDOISE` n'est pas un type generable (mode d'affichage)
- `PRESENTATION` (issue #119) à ajouter à cette liste si #119 livré avant ce chantier

### Contexte injecté automatiquement (pas un champ du formulaire)

Liste des questions déjà existantes dans les catégories ciblées, transmise au LLM à titre informationnel :
- Anti-doublon de thème/formulation
- Affinage itératif : l'admin peut relancer une génération après correction/suppression de questions, le LLM tient compte du nouvel état

---

## Génération & injection

### Comportement — Additif uniquement, garanti par construction

- **Pas d'écran de relecture** avant injection — l'édition/suppression existante de QuestionsPage sert de mécanisme de correction
- **Régénération** = nouvel appel avec mêmes paramètres ou ajustés, remplace le besoin d'un système d'undo/batch dédié
- **Sortie structurée** : le LLM répond en JSON structuré (structured outputs / `output_config.format`), directement dans le schéma question de BuzzControl
- **Pas de multipart** : ne passe pas par l'endpoint `POST /questions` existant (conçu pour un formulaire humain avec upload média). Un nouveau chemin d'écriture interne consomme directement ce JSON structuré

### Garantie de non-modification imposée par le code

Le chemin d'import ne fait **QUE créer** de nouveaux fichiers `question.json` avec des IDs fraîchement alloués. **Aucun moyen structurel ne doit permettre à la réponse du LLM de modifier un ID existant** :
- Réutiliser la logique d'allocation d'ID de `handleUploadQuestion` (`http.go:682+`)
- Le schéma de sortie JSON du LLM ne doit contenir AUCUN champ d'ID — le backend en alloue après réception
- Même si le LLM hallucine une intention de modifier l'existant, l'architecture rend cela impossible par construction

---

## Contenu généré

### Questions texte-only
- Claude ne génère pas d'images (pas un modèle de diffusion)
- L'admin ajoute une image manuellement après coup via l'upload existant de QuestionsPage si besoin

### Types supportés

| Type | Champs JSON (schéma sortie LLM) |
|---|---|
| **SPEEDY** | `TYPE`, `CATEGORY`, `QUESTION`, `ANSWER`, `POINTS`, `TIME` |
| **QCM** | `TYPE`, `CATEGORY`, `QUESTION`, `QCM_ANSWERS` (dict RED/GREEN/YELLOW/BLUE), `QCM_CORRECT`, `POINTS`, `TIME` |
| **MEMORY** | `TYPE`, `CATEGORY`, `PAIRS` (array de `{TEXT, CORRECT}`), `POINTS`, `TIME` |
| **MEMOTION** | `TYPE`, `CATEGORY`, `PAIRS` (array de `{EMOTION, TEXT}`), `POINTS`, `TIME` |

**Détermination du `TIME` par le LLM** : selon type/difficulté/population cible, permet un affinage itératif si régénération.

---

## Implémentation — Points clés

### Backend Go (`server-go/internal/`)

1. **Fix config.json handler** (préalable) : charger existant avant Unmarshal
2. **Modèle config** (`config/config.go`) : ajouter struct `AIConfig` avec `anthropic_api_key`
3. **GameState & engine** (`game/models.go`, `game/engine.go`) :
   - Ajouter champs `QUIZ_POPULATION`, `QUIZ_DIFFICULTY`, `QUIZ_LANGUAGE` à `GameState`
   - Ajouter setter `SetQuizMeta` pour ces champs (ou étendre le setter existant)
4. **Protocol** (`protocol/messages.go`) : étendre `QuizMetaPayload` avec ces 3 champs
5. **Route HTTP** : `POST /api/generate-questions`
   - Accepte paramètres du formulaire
   - Valide clé API
   - Appelle API Claude avec structured outputs
   - Crée nouveaux fichiers `question.json` dans `data/files/questions/<category>/`
   - Retourne liste des IDs créés + erreur si applicable
6. **WebSocket action** : `UPDATE_QUIZ_META` étend à ces 3 champs

### Frontend React (`server-go/web/src/`)

1. **ConfigPage** : ajouter section "IA" avec champ "Clé API Claude" (type password)
2. **QuestionsPage** : bouton "✨ Générer via IA" dans la carte sidebar "Nouvelle Question"
3. **Modale de génération** :
   - Bloc "Paramètres du Quiz" : affichage Thème/Population/Langue/Difficulté (globaux)
   - Bloc "Cette génération" : formulaire avec champs décrits ci-dessus
   - Validation côté client
   - Spinner + message "Génération en cours..." pendant l'appel API
4. **Résultat** : liste des questions créées, redirection/scroll vers les nouvelles questions dans QuestionsPage

### Champs GameState à ajouter/modifier

```go
// Dans GameState struct
QUIZ_NAME     string  // existant
QUIZ_THEME    string  // existant
QUIZ_NOTES    string  // existant
QUIZ_POPULATION string  // ← nouveau
QUIZ_DIFFICULTY string  // ← nouveau
QUIZ_LANGUAGE string    // ← nouveau
```

Tous **sans `omitempty`** (évite réinitialisations manquées côté frontend).

---

## Explicitement hors scope MVP

- ❌ Génération/gestion des équipes
- ❌ Génération d'images (Claude, DALL-E, Unsplash)
- ❌ Multi-provider LLM, abonnement Claude Pro/Max
- ❌ Export TAR / réimport
- ❌ Templates de jeu prédéfinis, personnalisation avancée
- ❌ Import de contexte PDF/document
- ❌ Système de rating/feedback
- ❌ Analytics d'usage
- ❌ Fine-tuning

---

## Cas d'usage identifiés

| Cas d'usage | Exemple | Paramètres suggérés |
|-------------|---------|---------------------|
| **Anniversaire enfant** | Quiz Disney | Junior, Facile, Cinéma, 20q, 70% QCM |
| **Soirée entre amis** | Trivia années 90 | Adulte, Moyen, Culture générale, 50q, 60% QCM |
| **Formation entreprise** | Quiz sécurité IT | Adulte, Difficile, Entreprise/IT, 30q, 50% QCM + 30% SPEEDY |
| **Révision scolaire** | Histoire CM2 | Junior, Moyen, Histoire, 40q, 80% QCM |
| **Team building** | Quiz inter-services | Adulte, Facile, Entreprise/Culture, 25q, 50% QCM + 30% MEMORY |
| **Résidence seniors** | Nostalgie années 50-60 | Senior, Facile, Musique/Cinéma/Histoire, 30q, 40% QCM + 40% MEMORY |

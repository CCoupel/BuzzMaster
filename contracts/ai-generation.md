# Contrat — Génération de questions via IA (#8, v6.0.0)

> **Statut** : contrat créé par `planner` avant développement (contract-first).
> **Plan** : `_work/reports/planner-20260805-121900.md`
> **Maquette** : `_work/mockups/8-generateur-ia.md`
> **Spec fonctionnelle** : `backlog/TODO/generateur-ia.md`
>
> `dev-backend` implémente et peut ajuster (en documentant la raison).
> `dev-frontend` consomme, ne modifie pas.

---

## 0. Prérequis bloquant — `POST /config.json` devient additif

**Comportement actuel (bug)** — `internal/server/http.go:1056-1115` :

```go
var cfg config.Config          // struct ZÉRO
json.Unmarshal(body, &cfg)     // seules les sections présentes sont peuplées
os.WriteFile(configPath, json.MarshalIndent(&cfg))  // écrit TOUT le struct
config.SetInstance(&cfg)
```

Aucun champ de `config.Config` ne porte `omitempty` (`internal/config/config.go:11-19`), donc
**toutes** les sections absentes du payload sont réécrites à zéro. ConfigPage n'envoie que des
payloads partiels (`{neon_effect: …}` ou `{server: {…}}`) → chaque sauvegarde d'un réglage
détruit les autres sections sur disque.

> Le dégât est déjà visible en production : `server-go/config.json` contient
> `"questions_dir": ""` et `"files_dir": ""` alors que les valeurs par défaut sont non vides
> (`config.go:184-187`). `main.go:4036-4039` compense avec un chemin codé en dur.

**Comportement contractuel après correction** :

| Règle | Détail |
|---|---|
| **Merge** | Le handler part de `config.Get()` (singleton courant) et n'écrase **que** les sections présentes dans le corps de la requête. |
| **Sections absentes** | Inchangées sur disque et en mémoire. |
| **Sections présentes** | Remplacées **intégralement** (pas de merge champ-à-champ à l'intérieur d'une section) — **sauf la section `ai`, cf. §0bis**. |
| **Défauts** | Après merge, les valeurs par défaut de `config.Load` sont ré-appliquées aux champs restés à zéro (sinon un client qui poste `{"server":{"debug":true}}` remet `http_port` à 0). |
| **Persistance** | Écriture atomique (fichier temporaire + `os.Rename`) — une coupure en cours d'écriture ne doit pas laisser un `config.json` tronqué. |
| **Concurrence** | Le paquet `config` expose désormais `Save(*Config) error` et protège `instance` par un `sync.RWMutex` (`Get`/`SetInstance` sont actuellement sans verrou). |

**Précédent à reprendre** : `handleAPIWiFiDefaults` (`http.go:2457-2520`) applique déjà exactement
ce motif (`cfg := config.Get()` → mutation ciblée → marshal complet → écriture → `SetInstance`).

> ⚠️ **Ceci est un correctif de bug, pas une évolution.** Il doit être livré et testé
> **avant** l'ajout de la section `ai`, sinon la clé API est effacée au premier
> « Enregistrer » d'une autre section de Paramètres — et réciproquement.

---

## 0bis. Amendement — la section `ai` est mergée champ-à-champ, pas remplacée intégralement

> **Statut** : amendement `dev-backend` suite à un bug réel trouvé en QA sur #137
> (`_work/handoff/task-dev-backend-20260806-103004.md`), pas une simple lacune de test.
> La règle « sections présentes remplacées intégralement » de §0 reste valable pour
> `server`/`wifi`/`game`/`storage`/`neon_effect`/`wifi_defaults`/`version` — **`ai` en est
> désormais exclue**, pour la raison ci-dessous.

**Constat** : `ConfigPage.jsx` sauvegarde les réglages `ai.*` par bouton séparé (sélecteur de
provider, champ clé API, curseurs de batching — `ConfigPage.jsx:343-352` pour la sauvegarde de
la clé seule) : chaque bouton poste un payload `{"ai": {...}}` qui ne porte que le(s) champ(s)
qu'il possède. Avec la règle §0 (section présente = remplacement intégral), sauvegarder la clé
Groq seule réinitialisait silencieusement `provider` à `"anthropic"` et tous les réglages de
batching à leurs valeurs par défaut — cassant le parcours documenté de configuration Groq
(procédure QA Scénario 1, étape 2).

**Règle corrigée** : pour la section `ai` uniquement, un champ JSON **absent** du payload
préserve la valeur actuellement stockée ; un champ **présent** (même à sa valeur zéro Go,
ex. `batch_size: 0`) écrase la valeur stockée puis, comme pour toute section, se fait
ré-appliquer les défauts de `config.ApplyDefaults` s'il est resté à zéro après merge. C'est
exactement la sémantique déjà en place pour les deux secrets (`anthropic_api_key`,
`groq_api_key` — cf. §2 et §8 de `ai-multi-provider.md`), généralisée à tous les autres champs
de `AIConfig` plutôt que d'en rester l'unique exception.

**Implémentation** : `internal/server/http.go` (`handleConfig`) re-décode la section `ai` en
`map[string]json.RawMessage` pour connaître les clés JSON réellement envoyées (présence, pas
valeur), puis ne réaffecte que les champs de `cfg.AI` dont la clé est présente — les autres
gardent la valeur du singleton courant. **Un champ `AIConfig` ajouté après cet amendement doit
être énuméré explicitement dans ce bloc de merge** (contrairement au remplacement intégral
précédent, l'ajout n'est plus automatique).

**Test de non-régression** : `TestHTTPServer_Config_POST_APIKeyPreservation/batching_and_provider_fields_survive_a_key-only_POST`
(`internal/server/http_test.go`) reproduit exactement les étapes du bug QA.

---

## 1. Section `ai` de `config.json`

Nouvelle clé top-level, suivant le motif de `NeonEffectConfig` (valeur, pas pointeur).

```go
// internal/config/config.go
type AIConfig struct {
    AnthropicAPIKey string `json:"anthropic_api_key"`
    Model           string `json:"model"`
    TimeoutSeconds  int    `json:"timeout_seconds"`
    MaxQuestions    int    `json:"max_questions"`
    // Champ dérivé, jamais persisté : positionné uniquement dans la réponse GET.
    APIKeyConfigured bool  `json:"api_key_configured,omitempty"`
}
```

Ajouté à `Config` : `AI AIConfig \`json:"ai"\`` .

| Champ | Défaut | Rôle |
|---|---|---|
| `anthropic_api_key` | `""` | Clé BYOK (`sk-ant-…`) |
| `model` | `"claude-opus-5"` | Modèle Claude utilisé |
| `timeout_seconds` | `300` | Timeout de l'appel API côté serveur |
| `max_questions` | `200` | Plafond dur du nombre de questions générées par appel |

Les défauts sont appliqués dans `config.Load` **et** dans le littéral de repli de `config.Get`
(les deux emplacements existants — `config.go:88-155` et `:170-202`).

---

## 2. `GET` / `POST /config.json` — traitement du secret

### GET — la clé n'est **jamais** renvoyée

| Propriété | Valeur |
|---|---|
| Auth | Aucune (comme tout le reste du serveur) |
| Content-Type | `application/json` |

La réponse est construite sur une **copie** du singleton :

```json
{
  "ai": {
    "anthropic_api_key": "",
    "api_key_configured": true,
    "model": "claude-opus-5",
    "timeout_seconds": 300,
    "max_questions": 200
  },
  "server": { … }, "wifi": { … }, "neon_effect": { … }
}
```

- `anthropic_api_key` est **toujours** la chaîne vide en réponse, quelle que soit la valeur stockée.
- `api_key_configured` vaut `true` si et seulement si la clé stockée est non vide.
- C'est **l'unique** signal dont dispose le frontend pour activer/désactiver le bouton
  « ✨ Générer via IA ».

> **Motif** : `/config.json` est servi sans authentification sur le LAN. Le mot de passe WiFi y
> est déjà en clair (dette existante, hors périmètre), mais une clé API facturable ne doit pas
> s'y ajouter. Voir §8 (sécurité).

### POST — préservation du secret

| Corps envoyé | Effet sur `anthropic_api_key` |
|---|---|
| Section `ai` **absente** | Inchangée (règle de merge générale, §0) |
| `"anthropic_api_key"` **absent** de la section `ai` | **Inchangée** |
| `"anthropic_api_key": ""` | **Inchangée** — une chaîne vide n'efface pas |
| `"anthropic_api_key": "sk-ant-…"` | Remplacée |
| `"clear_api_key": true` | **Effacée** (`""`), quelle que soit la valeur de `anthropic_api_key` |

`clear_api_key` est un champ de requête uniquement (`json:"clear_api_key,omitempty"` sur
`AIConfig`, jamais persisté, jamais renvoyé en GET).

**Validation** : une clé non vide doit commencer par `sk-ant-` — sinon `400`, message
« Format de clé API invalide (attendu : sk-ant-…) ». La validité réelle n'est vérifiée qu'au
premier appel de génération.

> **Alternative écartée** : un endpoint dédié `POST/DELETE /api/ai/key` (motif
> `/api/wifi/defaults`). Rejetée pour rester sur le chemin ConfigPage existant ; le correctif
> §0 est de toute façon requis dans les deux cas.

---

## 3. `POST /api/generate-questions`

| Propriété | Valeur |
|---|---|
| Auth | Aucune |
| Content-Type | `application/json` |
| Méthode | `POST` uniquement — le mux est un `http.ServeMux` sans motif de méthode, le handler doit renvoyer `405` lui-même (idiome établi : `http.go:2459`) |
| Durée | **Longue** (1 à 3 min). Réponse synchrone, pas de job asynchrone en MVP. |

### Request

```json
{
  "theme": "Cinéma français des années 80",
  "populations": ["Ado (13-17 ans)", "Adulte (18-64 ans)"],
  "language": "Français",
  "difficulties": ["Moyen", "Difficile"],
  "objectives": "Soirée de fin d'année du club, on veut que tout le monde marque au moins une fois",
  "instructions": "Éviter les questions sur le sport, insister sur les comédies",
  "categories": ["ENTERTAINMENT", "ma-categorie-custom"],
  "volume": { "mode": "count", "value": 20 },
  "distribution": { "SPEEDY": 40, "QCM": 40, "MEMORY": 20, "MEMOTION": 0, "MEMOTION_PLUS": 0, "ARDOISE": 0 }
}
```

| Champ | Type | Obligatoire | Règle de validation |
|---|---|---|---|
| `theme` | string | ✅ | non vide, ≤ 200 car. |
| `populations` | string[] | ✅ | **v6.1.0 — remplace `population` (string), cf. §3bis.** ≥ 1 élément, chacun ∈ énumération §6, sans doublon |
| `language` | string | ✅ | ∈ énumération §6 |
| `difficulties` | string[] | ✅ | ≥ 1 élément, chacun ∈ énumération §6, sans doublon. **v6.1.0 — seule source des difficultés** : le frontend n'enveloppe plus une valeur globale unique dans un tableau à un élément (cf. §3bis) |
| `objectives` | string | ❌ | **v6.1.0 — nouveau.** ≤ 2000 car. Objectif **global de la partie** (`QUIZ_OBJECTIVES`, `game-state.md`), distinct de `instructions` |
| `instructions` | string | ❌ | ≤ 2000 car. Précisions **propres à cette génération** — jamais persisté (cf. §9) |
| `categories` | string[] | ✅ | ≥ 1 élément ; chaque clé doit exister dans les catégories connues (dur + custom). Une clé inconnue → `400`. |
| `volume.mode` | `"count"` \| `"duration"` | ✅ | |
| `volume.value` | int | ✅ | mode `count` : 1..`ai.max_questions`. mode `duration` : 5..240 (minutes). |
| `distribution` | objet | ✅ | clés ⊂ {`SPEEDY`,`QCM`,`MEMORY`,`MEMOTION`,**`MEMOTION_PLUS`**,`ARDOISE`} ; valeurs entières ≥ 0 ; **somme = 100** ; au moins une valeur > 0. `MEMOTION_PLUS` — **v7.1.0, #196, cf. §3ter** : pseudo-type de génération, absent ⇒ 0 ⇒ comportement d'avant #196 inchangé. |

> Les libellés `population` / `language` / `difficulties` sont transmis **tels quels** au LLM
> (ils servent de contexte de rédaction, pas de clés techniques). Seules `categories` et
> `distribution` portent des clés techniques.

### 3ter. `MEMOTION_PLUS` — pseudo-type de génération (v7.1.0, #196)

**Aucun champ n'est ajouté à la Request.** Le choix passe par une **clé de distribution
supplémentaire**, au même niveau que les cinq autres — et non par un mode niché sous `MEMOTION`.

| Clé de distribution | Cartes générées |
|---|---|
| `MEMOTION` | **Cartes SPEEDY uniquement** — aucune carte ne porte de `TYPE`. Comportement d'avant #196, strictement inchangé |
| **`MEMOTION_PLUS`** (affiché « MEMOTION+ ») | Mélange **SPEEDY / QCM**, le type étant choisi carte par carte par le modèle selon ce qui convient au contenu |

#### 🔴 `MEMOTION_PLUS` n'est pas un `QuestionType` — invariant central

> **Une question générée depuis `MEMOTION_PLUS` est persistée avec `TYPE: "MEMOTION"`.**
> La chaîne `MEMOTION_PLUS` ne doit **jamais** apparaître dans un `question.json`.

Le pseudo-type n'existe que pendant la génération : il exprime *comment générer*, pas *ce qui est
généré*. La normalisation `MEMOTION_PLUS → MEMOTION` a lieu à la construction de la question, avant
toute écriture.

**Ce que cela implique, et qui est délibérément hors de portée de #196 :**

| Élément | Touché ? |
|---|---|
| `questionTypeRegistry` (Go), `AllQuestionTypes()` | ❌ **jamais** — MEMOTION+ n'est pas un type du moteur |
| `QUESTION_TYPES` (`web/src/utils/questionTypeMeta.js`) | ❌ **jamais** — table faisant autorité pour les **types réels** depuis #183, consommée par l'éditeur de questions, les badges `QuestionCard` et `PlayerDisplay`. Y ajouter le pseudo-type ferait apparaître un « MEMOTION+ » fantôme dans l'éditeur et recréerait la divergence de tables que #183 a supprimée |
| `generableQuestionTypes` (`internal/server/ai_generator.go`) | ✅ **oui** — cette liste est **déjà** propre à la génération et distincte du registre. C'est la maison naturelle du pseudo-type |
| `GENERABLE_TYPES` (nouvel export, `questionTypeMeta.js`) | ✅ **oui** — export **séparé** (= les 5 types réels + `MEMOTION_PLUS`), consommé **uniquement** par la modale de génération. Miroir JS exact de `generableQuestionTypes` |

> **Symétrie voulue** : Go distingue déjà « types réels » (`questionTypeRegistry`) de « ce que l'IA
> sait produire » (`generableQuestionTypes`). Le JS reproduit la même séparation plutôt que d'élargir
> la table des types réels.

#### Non-régression de `MEMOTION`, garantie par construction

La variante de schéma `MEMOTION` conserve ses items de `MOTION_CARDS` à **4 propriétés**
(`RECTO_THEME`, `QUESTION_TEXT`, `ANSWER_TEXT`, `DIFFICULTY`) avec `additionalProperties: false` :
le modèle est donc **structurellement incapable** d'y typer une carte. La non-régression n'est pas
seulement testée, elle est **interdite par le schéma**.

C'est la reconduction de l'exigence de #184 (« une carte sans `TYPE` continue d'être générée comme
aujourd'hui »). Une carte sans `TYPE` valant `SPEEDY` (`contracts/question-types.md` §3), la
génération `MEMOTION` **ne doit pas** se mettre à écrire `TYPE: "SPEEDY"` explicitement : cela
changerait la forme des fichiers produits pour un résultat sémantiquement identique.

#### Portée du mode `MEMOTION_PLUS`

- Types de carte autorisés : **`SPEEDY` et `QCM` uniquement** — les deux seuls dont le registre
  déclare `NestableInMotionCard: true` (`contracts/question-types.md` §7).
- Une carte `TYPE=QCM` porte `QCM_ANSWERS` (4 réponses) et `QCM_CORRECT`, ses `OwnedFields` (§3.1).
- **Aucun contrôle de ratio** SPEEDY/QCM n'est exposé : le modèle arbitre carte par carte.
- Aucun média n'est généré — inchangé.

> **Aucun élargissement de la validation d'imbrication n'est nécessaire.**
> `IsNestableInMotionCard` et `ValidateCardTypeContent` sont déjà en place et refusent
> respectivement un type non imbricable et un contenu orphelin d'un autre type. Une carte produite
> par le LLM emprunte **le même chemin de validation** qu'une carte saisie dans l'éditeur : un type
> inventé ou un contenu incohérent est rejeté sans une ligne de code supplémentaire.

### Response 200

```json
{
  "status": "ok",
  "created": [
    { "id": "12", "type": "QCM",    "category": "ENTERTAINMENT" },
    { "id": "13", "type": "SPEEDY", "category": "ENTERTAINMENT" }
  ],
  "created_count": 2,
  "skipped_count": 1,
  "skipped_reasons": ["MEMORY: moins de 2 paires valides"],
  "model": "claude-opus-5"
}
```

- `created` est ordonné selon l'ordre d'écriture (= ordre `ORDER` attribué).
- `created[].type` reporte le **type réellement persisté**. Une question générée depuis la clé de
  distribution `MEMOTION_PLUS` y apparaît donc en **`"MEMOTION"`** (§3ter) — la réponse décrit ce
  qui est sur disque, jamais le pseudo-type qui a servi à le demander.
- `skipped_count` > 0 signale des questions renvoyées par le LLM mais **rejetées** par la
  validation serveur (§5). Ce n'est pas une erreur : la réponse reste `200`.
- `created_count == 0` → renvoyer **`502`** (§4), pas un `200` vide : la modale ne doit jamais
  afficher un faux succès.

### Errors

| Code | Cas | Corps |
|---|---|---|
| `400` | Payload invalide (champ manquant, somme ≠ 100, catégorie inconnue, volume hors bornes) | `{"status":"error","code":"invalid_request","message":"…"}` |
| `405` | Méthode ≠ POST | texte brut |
| `409` | Aucune clé API configurée | `{"status":"error","code":"no_api_key","message":"Aucune clé API Claude configurée."}` |
| `502` | Erreur amont Anthropic : clé invalide (401), quota (429), surcharge (529), réseau injoignable, réponse non conforme au schéma, 0 question exploitable | `{"status":"error","code":"upstream_error","upstream_status":401,"message":"…"}` |
| `504` | Dépassement de `ai.timeout_seconds` | `{"status":"error","code":"timeout","message":"…"}` |
| `507` | Plus d'ID disponible (§5.1) | `{"status":"error","code":"id_exhausted","message":"…"}` |

Le champ `code` est **stable** : c'est lui que le frontend mappe vers les messages de la
maquette §6.3, pas le texte de `message`.

### Effet de bord obligatoire

Après écriture des questions, le handler **doit** appeler `h.OnQuestionUpload()` (le même
callback que `handleUploadQuestion`, `http.go:997-999`, câblé sur `a.broadcastQuestions()`).
Sans cet appel, les questions créées n'apparaissent dans QuestionsPage **qu'après un
rechargement de page** — la liste est alimentée par le broadcast WebSocket, jamais par un
refetch client.

---

## 3bis. Amendement v6.1.0 — publics et difficultés multiples, objectif global

> **Statut** : amendement `planner` avant développement (contract-first), #137 Batch 2b.
> **Origine** : retour QUALIF utilisateur, audit `_work/reports/planner-20260806-143248.md`.
> **Arbitrages utilisateur** : `_work/handoff/task-planner-contracts-20260806-144240.md`.
> **⚠️ BREAKING** — `population` (string) disparaît au profit de `populations` (string[]).

### Ce qui change et pourquoi

| Avant (v6.0.0) | Après (v6.1.0) | Motif |
|---|---|---|
| `population: string` | `populations: string[]` | Une partie s'adresse souvent à plusieurs publics à la fois (soirée famille = enfants + adultes). Un choix unique forçait l'animateur à en sacrifier un. |
| `difficulties: string[]` alimenté par **une** valeur globale unique enveloppée côté client | `difficulties: string[]` alimenté par une **sélection multiple** globale | Le champ était déjà un tableau côté contrat et backend depuis v6.0.0 ; seule l'UI le bridait à une valeur. **Aucun changement de forme ici** — uniquement la disparition de l'enveloppe artificielle `[valeurUnique]` côté frontend. |
| — | `objectives: string` (optionnel) | L'objectif de la partie (« révision du chapitre 3 », « team building ») est une propriété **de la partie**, pas de chaque génération : il était jusqu'ici resaisi à chaque ouverture de la modale, ou pas saisi du tout. |

### `objectives` vs `instructions` — deux champs, jamais un seul

| | `objectives` | `instructions` |
|---|---|---|
| Portée | La **partie** — global, persisté dans `GameState` (`QUIZ_OBJECTIVES`) | **Cette génération** — local à la modale |
| Saisi dans | Section « Quiz » de QuestionsPage | Popup de génération IA |
| Survit à la fermeture de la modale | ✅ | ❌ (jamais persisté, §9) |
| Visible des joueurs | ❌ **jamais** (cf. `game-state.md`) | ❌ (n'existe que le temps de la requête) |

> **Règle de non-duplication** : le popup de génération ne doit proposer **aucun** champ
> éditable qui reproduise `theme` / `populations` / `difficulties` / `language` / `objectives`.
> Ces cinq valeurs viennent du `GameState` et sont affichées **en lecture seule** dans le popup
> (rappel + lien « modifier » vers la section Quiz). Le seul champ de saisie du popup relevant
> de cette famille est `instructions`, qui **précise** l'objectif global sans le remplacer, et
> dont le libellé doit refléter cette portée (« Précisions pour cette génération », et non
> « Objectifs » — qui rejouerait exactement la confusion global/local corrigée ici).

### Validation

| Champ | Erreur `400 invalid_request` si |
|---|---|
| `populations` | absent, tableau vide, élément hors énumération §6, ou doublon |
| `difficulties` | inchangé — absent, tableau vide, élément hors énumération §6, ou doublon |
| `objectives` | > 2000 caractères (absent ou vide = accepté, le champ est optionnel) |

> Un appelant de v6.0.0 qui poste encore `population` (singulier) reçoit un `400` :
> le champ n'est pas reconnu et `populations` est manquant. **C'est le comportement attendu** —
> pas de repli silencieux sur l'ancien nom, qui masquerait un client non déployé (cf. §3ter).

### §3ter. Fenêtre de déploiement — clients non rechargés

`GameState` **n'est pas persisté sur disque** (vérifié : `cmd/server/main.go:205-211` ne
déclare de chemin de persistance que pour `history.json`, `teams.json`, `bumpers.json` et
`question_statuses.json` ; aucun `SetStatePath`/`SaveState` n'existe). Il n'y a donc **aucune
migration de fichier à écrire** — voir `game-state.md` § « Migration » pour le raisonnement
complet et les deux risques résiduels.

Le seul risque réel est une **fenêtre de désynchronisation** : après déploiement, un onglet
resté ouvert sert encore le JS de v6.0.0 et posterait `population`. Le `400` ci-dessus est le
comportement voulu : il est visible (message d'erreur dans la modale) plutôt que silencieux.
**Mode opératoire de déploiement** : rechargement des interfaces admin/TV après mise à jour,
comme pour toute montée de version côté client.

---

## 4. Appel à l'API Claude

| Point | Décision |
|---|---|
| Client | SDK officiel `github.com/anthropics/anthropic-sdk-go` (nouvelle dépendance — cf. plan, risque R7) |
| Modèle | `config.Get().AI.Model`, défaut `claude-opus-5` |
| Sortie structurée | `output_config.format` = `{"type":"json_schema","schema": …}` (§5) |
| Streaming | **Obligatoire** — le volume de sortie (jusqu'à 200 questions) dépasse largement le seuil au-delà duquel une requête non streamée expire. Accumuler puis décoder le message final. |
| `max_tokens` | Dimensionné large (≥ 64000). ⚠️ `max_tokens` plafonne **thinking + texte** : un plafond trop bas tronque la réponse au milieu du JSON. |
| Effort | `output_config.effort` = `"medium"` par défaut (génération de contenu, pas de raisonnement long-horizon) |
| Timeout | `option.WithRequestTimeout(ai.timeout_seconds)` |
| Erreurs | Dépaqueter via `errors.As(err, &apierr)` sur `*anthropic.Error`, puis brancher sur `apierr.StatusCode` (401 / 429 / 5xx) → mapping §3 |
| Clé | Lue à chaque appel depuis `config.Get().AI.AnthropicAPIKey`. **Jamais** journalisée, jamais renvoyée dans un message d'erreur. |

### Ordre d'injection des consignes dans le prompt (v6.1.0)

`buildGenerationPrompt` (`internal/server/ai_generator.go:678-691`) compose les paramètres de
l'admin dans un ordre **normatif** — deux consignes de portées différentes doivent être
distinguées explicitement, sinon le modèle reçoit deux instructions concurrentes sans savoir
laquelle prime :

| Rang | Ligne émise | Source | Condition |
|---|---|---|---|
| 1 | `Thème : …` | `theme` | toujours |
| 2 | `Publics cibles : …` (valeurs jointes par `, `) | `populations[]` | toujours — **pluriel**, remplace `Public cible : %s` |
| 3 | `Langue de rédaction : …` | `language` | toujours |
| 4 | `Niveaux de difficulté autorisés : …` | `difficulties[]` | toujours |
| 5 | `Objectif de la partie : …` | `objectives` | si non vide après trim |
| 6 | `Précisions pour cette génération : …` | `instructions` | si non vide après trim |
| 7 | `Catégories autorisées …` | `categories[]` | toujours |

**Règles** :
- L'objectif global (rang 5) précède **toujours** les précisions de génération (rang 6) : le
  cadre avant l'ajustement.
- Le libellé du rang 6 remplace « Instructions additionnelles de l'animateur » de v6.0.0 — les
  deux lignes doivent nommer leur portée, faute de quoi elles sont indiscernables pour le modèle.
- Un champ optionnel vide n'émet **aucune** ligne (pas de ligne à valeur vide, qui inviterait le
  modèle à combler un blanc).

### Contexte injecté (non éditable par l'admin)

Le prompt inclut la liste des questions **déjà existantes dans les catégories ciblées**, pour
l'anti-doublon et l'affinage itératif (spec §« Contexte injecté automatiquement »).

**Plafond obligatoire** : au plus **150 questions**, réduites à `{TYPE, CATEGORY, QUESTION}` et
chaque énoncé tronqué à 200 caractères. Au-delà, ne transmettre que les plus récentes.
Sans ce plafond, une base de 900 questions ferait exploser le coût d'entrée à chaque génération.

---

## 5. Schéma de sortie du LLM

**Règle d'or — garantie d'additivité par construction** : le schéma ne contient **aucun champ
d'identifiant**. `ID`, `ORDER`, `MEDIA`, `MEDIA_ANSWER`, `STATUS` sont absents du schéma et
seront **rejetés** s'ils apparaissent (`additionalProperties: false`). Le LLM est donc
structurellement incapable de désigner — et a fortiori de modifier — une question existante.

Racine :

```json
{
  "type": "object",
  "additionalProperties": false,
  "required": ["questions"],
  "properties": {
    "questions": { "type": "array", "items": { "anyOf": [ /* 5 variantes ci-dessous */ ] } }
  }
}
```

Champs communs à toutes les variantes : `TYPE`, `CATEGORY`, `QUESTION`, `TIME`, `DIFFICULTY`.

| Champ | Type schéma | Note |
|---|---|---|
| `TYPE` | `enum` | `SPEEDY` \| `QCM` \| `MEMORY` \| `MEMOTION` \| `ARDOISE` (discriminant) |
| `CATEGORY` | `enum` | restreint aux clés de `categories` de la requête |
| `QUESTION` | string | énoncé |
| `TIME` | integer | secondes ; **le LLM le détermine** selon type/difficulté/population |
| `DIFFICULTY` | `enum` | restreint aux valeurs de `difficulties` de la requête ; sert au calcul de `POINTS` (§5.2) |

Variantes :

| `TYPE` | Champs additionnels |
|---|---|
| `SPEEDY` | `ANSWER` (string) |
| `QCM` | `QCM_ANSWERS` (objet `{RED,GREEN,YELLOW,BLUE}`, 4 strings, tous requis), `QCM_CORRECT` (`enum` RED/GREEN/YELLOW/BLUE) |
| `MEMORY` | `MEMORY_PAIRS` : array de `{ "LEFT": string, "RIGHT": string }` |
| `MEMOTION` | `MOTION_CARDS` : array de `{ "RECTO_THEME": string, "QUESTION_TEXT": string, "ANSWER_TEXT": string, "DIFFICULTY": integer(enum 1,2,3) }` |
| `ARDOISE` | `ANSWER` (string), `ARDOISE_KEYBOARD_TYPE` (`enum` `AZERTY`\|`NUMPAD`) |

> **Amendement 2026-08-06** (#137 Batch 2a, `_work/reports/planner-20260806-121743-qualif-137.md`
> §2, arbitrage utilisateur) : `ARDOISE` était décrit par la spec d'origine de #8
> (`backlog/TODO/generateur-ia.md`) comme « pas un type generable (mode d'affichage) », et ce
> contrat reprenait cette caractérisation (ancien §6). **Elle était inexacte** :
> `game/models.go:161` en fait un `QuestionType` au même rang que SPEEDY/QCM/MEMORY/MEMOTION,
> avec son propre contenu persisté (énoncé, réponse) — le mode de *saisie* est particulier
> (clavier virtuel côté joueur), pas le *contenu*, qui doit être généré comme pour SPEEDY.
> Structurellement `ARDOISE = SPEEDY + ARDOISE_KEYBOARD_TYPE`. Arbitrage retenu sur le choix du
> clavier (Q2.1) : **le LLM le choisit lui-même** — `NUMPAD` si la réponse est purement numérique
> (nombre, date, année), `AZERTY` sinon — consigne en prose dans le prompt de génération
> (`ai_generator.go`, `buildGenerationPrompt`), le schéma ne pouvant pas exprimer une règle
> dépendant du contenu d'un autre champ. Activation par défaut
> dans la répartition (Q2.2) : **désactivé à 0 %**, comme `MEMOTION` — ajouter un 5ᵉ type actif
> par défaut aurait silencieusement redistribué les pourcentages des quatre types existants.

> ⚠️ **Écart avec `backlog/TODO/generateur-ia.md`.** La spec décrit MEMORY et MEMOTION comme
> partageant un champ `PAIRS` (`{TEXT, CORRECT}` pour MEMORY, `{EMOTION, TEXT}` pour MEMOTION).
> **Le modèle réel est différent** (`internal/game/models.go:211-243`) :
> - MEMORY = `MEMORY_PAIRS[]` de `{ID:int, CARD1:{TEXT,IMAGE,IS_IMAGE}, CARD2:{…}}` — un jeu
>   d'appariement (ex. pays ↔ capitale) ;
> - MEMOTION = `MOTION_CARDS[]` de `{ID:string "mc-N", RECTO_THEME, DIFFICULTY:1|2|3,
>   QUESTION_TEXT, ANSWER_TEXT, + images}` — une grille de cartes à 3 faces thématiques, **sans
>   notion d'émotion** (cf. la question MEMOTION réelle `data/files/questions/2/`, thème Sport).
>
> **Le code fait foi.** Le schéma ci-dessus est aligné sur le modèle réel. Les noms `LEFT` /
> `RIGHT` du schéma LLM sont volontairement neutres et mappés par le backend vers
> `CARD1.TEXT` / `CARD2.TEXT`. **Point à confirmer auprès de l'utilisateur au GATE 2.**

**Limites des structured outputs à connaître** : `minItems`, `maxItems`, `minLength`,
`minimum`/`maximum` ne sont **pas** supportés. Les contraintes de cardinalité (≥ 2 paires,
≥ 4 cartes, `TIME` dans 5..300) sont donc **inapplicables au niveau du schéma** et doivent être
vérifiées côté serveur (§5.1).

### 5.1 Validation serveur — chaque question est acceptée ou écartée

Une question invalide est **écartée** (comptée dans `skipped_count`), elle n'interrompt pas la
génération.

| Contrôle | Règle |
|---|---|
| `CATEGORY` | doit appartenir aux `categories` de la requête — sinon écartée |
| `DIFFICULTY` (racine, hors `MOTION_CARDS`) | **v6.1.1, issue #142** — doit appartenir aux `difficulties` de la requête, sinon écartée. Ajoutée en compensation : le schéma envoyé à Groq ne contraint plus ce champ par `enum` (`ai-multi-provider.md` §7 amendé), cette vérification est donc la seule garantie restante pour ce provider ; appliquée aux deux providers par cohérence, sans effet observable sur Anthropic (son schéma garde l'`enum`, le LLM ne peut structurellement pas en sortir). |
| `QUESTION` | non vide après trim |
| `TIME` | clampé dans `[5, 300]` |
| `SPEEDY` | `ANSWER` non vide |
| `QCM` | 4 réponses non vides et distinctes ; `QCM_CORRECT` ∈ {RED,GREEN,YELLOW,BLUE} |
| `MEMORY` | `2 ≤ len(MEMORY_PAIRS) ≤ 12` ; chaque `LEFT`/`RIGHT` non vide |
| `MEMOTION` | `4 ≤ len(MOTION_CARDS) ≤ 12` ; `RECTO_THEME` + `ANSWER_TEXT` non vides ; `DIFFICULTY ∈ {1,2,3}` |
| `ARDOISE` | `ANSWER` non vide ; `ARDOISE_KEYBOARD_TYPE ∈ {AZERTY,NUMPAD}` |
| Volume | si `created_count` dépasserait `ai.max_questions`, les questions excédentaires sont écartées |

**Allocation d'ID — sérialisée et exclusive.** `findFreeQuestionID` (`http.go:1008-1018`) est un
balayage de répertoire `1..999` **sans aucun verrou** ; deux écritures concurrentes peuvent
obtenir le même ID. Pour une génération par lot :

1. Prendre un **mutex** au niveau du `HTTPServer` (à ajouter — la struct n'en a aucun) couvrant
   allocation **et** création du répertoire ;
2. Réserver chaque ID avec `os.Mkdir` (échec si existe — **exclusif**), pas `os.MkdirAll`
   (idempotent, donc non réservant) ;
3. Si le balayage atteint 999 sans trouver d'ID libre → `507 id_exhausted` (le repli actuel
   `return "999"` écraserait la question 999).

Le même mutex doit être pris par `handleUploadQuestion` pour fermer la course pré-existante.

### 5.2 Mapping vers `question.json`

Le fichier écrit est `<dataDir>/files/questions/<ID>/question.json`, indenté 2 espaces
(`json.MarshalIndent`), **exactement au format produit par `handleUploadQuestion`**.

> ⚠️ `POINTS` et `TIME` sont des **chaînes** dans le modèle (`models.go:288-289`), pas des
> entiers. Émettre `"10"`, jamais `10`.

Champs dérivés par le backend (jamais par le LLM) :

| Champ | Règle |
|---|---|
| `ID` | alloué §5.1 |
| `ORDER` | `max(ORDER existants) + 1 + i` → les questions générées se placent **à la fin** de la liste, dans l'ordre de création |
| `POINTS_TARGET` | `TEAM` pour QCM / MEMORY / MEMOTION / ARDOISE, `PLAYER` pour SPEEDY (règle existante `http.go:742-752` ; `TEAM` pour ARDOISE cohérent avec la donnée réelle existante, ardoise collective) |
| `POINTS` (SPEEDY, QCM, ARDOISE) | depuis `DIFFICULTY` : Facile `"10"`, Moyen `"20"`, Difficile `"30"`, Expert `"50"` |
| `POINTS` (MEMORY) | `"0"` — le score vient de `MEMORY_CONFIG.POINTS_PER_PAIR` |
| `POINTS` (MEMOTION) | `"1"` — le score vient de `MOTION_CONFIG` (1/3/5 étoiles) |
| `ANSWER` (MEMORY) | `"<N> paires"` (aligné sur QuestionsPage) |
| `ANSWER` (MEMOTION) | `"<N> cartes"` |
| `ANSWER` (ARDOISE) | valeur du LLM telle quelle (comme SPEEDY) |
| `ARDOISE_KEYBOARD_TYPE` | valeur du LLM telle quelle |
| `MEMORY_PAIRS[i].ID` | entier, `1..N` |
| `MEMORY_PAIRS[i].CARD{1,2}` | `{TEXT: <LEFT\|RIGHT>, IS_IMAGE: false}` |
| `MOTION_CARDS[i].ID` | chaîne `"mc-<i+1>"` |
| `MEMORY_MODE` | `"SOLO"` |
| `MEMORY_CONFIG` | `{FLIP_DELAY:3, POINTS_PER_PAIR:10, ERROR_PENALTY:0, COMPLETION_BONUS:20, USE_TIMER:true, MEMORIZE_TIME:5, SHOW_DURING_MEMORIZE:true, REVEAL_DELAY:0.5}` |
| `MOTION_MODE` | `"CHACUN_SON_TOUR"` |
| `MOTION_MEMORIZE_DURATION` | `0` |
| `MOTION_CONFIG` | `{POINTS_1_STAR:1, POINTS_2_STAR:3, POINTS_3_STAR:5}` |
| `QCM_HINTS_ENABLED` | `false` — les indices restent une option que l'admin active question par question |
| `MEDIA`, `MEDIA_ANSWER` | absents (questions texte uniquement) |

> `ORDER` : `handleUploadQuestion` ne positionne pas `ORDER` sur une création manuelle (il le
> préserve seulement s'il existe). Le calcul ci-dessus est un **écart assumé** : il rend
> déterministe le « défiler jusqu'aux nouvelles questions » de la maquette §6.2.

---

## 6. Énumérations partagées

Ces listes doivent être **identiques** côté backend (validation) et frontend (selects). Source
de vérité : ce contrat.

| Énumération | Valeurs | Cardinalité (v6.1.0) |
|---|---|---|
| `QUIZ_POPULATIONS` | `Junior (6-12 ans)`, `Ado (13-17 ans)`, `Adulte (18-64 ans)`, `Senior (65+ ans)`, `Famille` | **≥ 1**, sans doublon |
| `QUIZ_DIFFICULTIES` | `Facile`, `Moyen`, `Difficile`, `Expert` | **≥ 1**, sans doublon |
| `QUIZ_LANGUAGE` | `Français` (défaut), `Anglais`, `Espagnol` | **exactement 1** — inchangé |
| Types générables | `SPEEDY`, `QCM`, `MEMORY`, `MEMOTION`, `ARDOISE` | ≥ 1 actif |

> **Amendement v6.1.0 (§3bis)** : les listes de valeurs sont **inchangées** — seule la
> cardinalité de `QUIZ_POPULATIONS` et `QUIZ_DIFFICULTIES` passe de 1 à N. Les noms au singulier
> (`QUIZ_POPULATION`, `QUIZ_DIFFICULTY`) n'existent plus, ni dans `GameState`, ni dans
> `UPDATE_QUIZ_META`, ni dans la requête de génération.
>
> **Rendu UI imposé (arbitrage utilisateur Q3)** : chips multi-sélection, reprenant exactement
> le motif déjà en place pour les catégories du popup (`AIGenerateModal.jsx:684-702`) — pas de
> `<select multiple>`, dont l'ergonomie (Ctrl+clic) est inadaptée à un usage tablette/tactile
> et incohérente avec le reste de l'application.

> **Amendement 2026-08-06** (#137 Batch 2a) : `ARDOISE` **est** générable depuis ce chantier —
> voir l'amendement détaillé au §5. La ligne précédente de ce contrat (« `ARDOISE` n'est pas
> générable, c'est un mode de saisie ») reprenait une caractérisation erronée de la spec
> d'origine de #8 ; le code montre que c'est un type de contenu à part entière (`models.go:161`).

`PRESENTATION` **n'existe pas** dans le code (`models.go:156-162`) — l'issue #119 n'est pas
livrée ; à rouvrir si elle l'est avant ce chantier.

---

## 7. `UPDATE_QUIZ_META` — extension

Action WebSocket existante (`protocol/messages.go:24`), payload étendu de 3 à 6 champs en
v6.0.0, puis à 7 champs en v6.1.0 (dont 2 changent de type — cf. §3bis).

**Avant** (`messages.go:216-221`) :
```go
type QuizMetaPayload struct {
    Name  string `json:"NAME"`
    Theme string `json:"THEME"`
    Notes string `json:"NOTES"`
}
```

**Après (v6.0.0)** :
```go
type QuizMetaPayload struct {
    Name       string `json:"NAME"`
    Theme      string `json:"THEME"`
    Notes      string `json:"NOTES"`
    Population string `json:"POPULATION"`
    Difficulty string `json:"DIFFICULTY"`
    Language   string `json:"LANGUAGE"`
}
```

**Après (v6.1.0 — §3bis, ⚠️ BREAKING)** — les pointeurs portent déjà la sémantique
« absent = inchangé » (`messages.go:221-233`), elle s'applique **à l'identique** aux nouveaux
champs :
```go
type QuizMetaPayload struct {
    Name         string    `json:"NAME"`
    Theme        string    `json:"THEME"`
    Notes        string    `json:"NOTES"`
    Populations  *[]string `json:"POPULATIONS,omitempty"`  // remplace POPULATION (string)
    Difficulties *[]string `json:"DIFFICULTIES,omitempty"` // remplace DIFFICULTY (string)
    Language     *string   `json:"LANGUAGE,omitempty"`
    Objectives   *string   `json:"OBJECTIVES,omitempty"`   // nouveau
}
```

| Cas | Effet sur le champ |
|---|---|
| Clé JSON **absente** | Valeur courante **inchangée** (pointeur `nil`) |
| Clé présente, tableau **vide** `[]` | Écrase avec une liste vide — **effacement explicite assumé** |
| Clé présente, chaîne vide `""` (`OBJECTIVES`) | Écrase avec `""` — effacement explicite assumé |

> La distinction « absent » / « présent mais vide » est **le point à ne pas rater** : c'est elle
> qui permet à un client d'effacer un champ volontairement sans qu'un autre client, qui n'envoie
> qu'une partie du formulaire, ne l'efface par accident. Elle existe déjà pour les trois champs
> de v6.0.0 ; l'étendre aux nouveaux n'est pas optionnel.

`SetQuizMeta` (`internal/game/engine.go:1592`) change de signature : `population string` →
`populations []string`, `difficulty string` → `difficulties []string`, + `objectives string`.
**Seul appelant existant** : `cmd/server/main.go:1064-1076`.

Message client → serveur (v6.1.0) :
```json
{ "ACTION": "UPDATE_QUIZ_META",
  "MSG": { "NAME": "Quiz ciné", "THEME": "Cinéma", "NOTES": "",
           "POPULATIONS": ["Ado (13-17 ans)", "Adulte (18-64 ans)"],
           "DIFFICULTIES": ["Moyen", "Difficile"],
           "LANGUAGE": "Français",
           "OBJECTIVES": "Révision du chapitre 3 avant le contrôle" } }
```

**Rétrocompatibilité** : ⚠️ **rompue en v6.1.0** — `POPULATION` et `DIFFICULTY` (singuliers)
ne sont plus reconnus. Un client de v6.0.0 qui les envoie voit ses deux champs **ignorés**
(clés inconnues du décodage sélectif) : les valeurs courantes restent inchangées, aucune
corruption, mais l'enregistrement est partiellement sans effet. **Aucun repli** vers les anciens
noms n'est implémenté — un client non redéployé doit être visible, pas silencieusement rattrapé
(cf. §3ter).

> **Règle de contrat, toujours en vigueur** : le handler `ActionUpdateQuizMeta`
> (`cmd/server/main.go:1055-1076`) applique une sémantique **« absent = inchangé »** en
> distinguant champ absent et valeur vide (pointeurs, cf. `messages.go:221-233`). Sans elle,
> toute source d'`UPDATE_QUIZ_META` n'envoyant qu'une partie du formulaire efface le reste.

---

## 8. Sécurité — points imposés par ce contrat

| # | Règle |
|---|---|
| S1 | La clé API n'est **jamais** renvoyée par `GET /config.json` (§2). Seul `api_key_configured` sort. |
| S2 | La clé n'apparaît dans **aucun** log (`LogInfo`/`LogWarn`), ni dans un message d'erreur renvoyé au client. Les erreurs amont sont remappées (§3). **Amendé (v6.1.1, issue #142)** : le corps d'erreur du provider **peut** être relayé — assaini (`server.sanitizeUpstreamMessage` : toute sous-chaîne au format d'une clé API connue remplacée par `[redacted]`, message tronqué à 500 caractères) et **uniquement** via `AI_GENERATION_PROGRESS.ERROR_MESSAGE` (`/ws/admin` seul, jamais une réponse HTTP synchrone) — plus la relecture aveugle interdite par la version précédente de cette règle. Motif : un message générique fixe rendait un échec de génération indiagnosticable sans ajouter une trace de debug temporaire (#142, schéma JSON rejeté par Groq visible uniquement ainsi) — un admin sans accès au code ne pouvait pas savoir pourquoi une génération échouait. La clé elle-même reste couverte à l'identique : elle n'est de toute façon jamais présente dans un corps de réponse provider (transmise uniquement via l'en-tête `Authorization`) ; le filtre de `sanitizeUpstreamMessage` est une défense en profondeur, pas le mécanisme qui garantit son absence. |
| S3 | `CATEGORY` renvoyée par le LLM est validée contre une liste blanche (§5.1) et **jamais** utilisée pour construire un chemin de fichier (le répertoire est nommé par l'`ID` alloué serveur). |
| S4 | Le contenu généré est du texte inséré dans `question.json` puis rendu par React (échappement par défaut). Aucun `dangerouslySetInnerHTML` ne doit être introduit sur ce chemin. |
| S5 | L'endpoint est **non authentifié** comme tout le serveur : n'importe qui sur le LAN peut déclencher une génération facturée sur le compte Anthropic de l'opérateur. Dette assumée en MVP, à signaler à l'utilisateur (cf. plan, risque R6). |
| S6 | Le plafond `ai.max_questions` et le timeout sont des garde-fous de coût, pas des détails d'implémentation : ils doivent être appliqués côté serveur, jamais seulement côté client. |

---

## 9. Ce que ce contrat ne couvre pas

- Génération d'images (hors scope MVP).
- Génération/gestion d'équipes, templates, export TAR (hors scope MVP).
- Multi-provider LLM (Claude uniquement).
- Reprise/annulation d'une génération en cours (pas d'undo : la correction passe par
  l'édition/suppression existante de QuestionsPage, cf. spec).
- Persistance du formulaire de génération (jamais persisté, cf. maquette §7).
